// Package corerpc implements the data collection abstractions in package
// collect by using the Bitcoin Core JSON-RPC API.
package corerpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/sim"
)

func Getters(timeNow UnixNow, cfg Config) (col.MempoolStateGetter, col.BlockGetter, error) {
	c := newClient(cfg)
	relayfee, err := c.getRelayFee()
	if err != nil {
		return nil, nil, err
	}
	getState := func() (*col.MempoolState, error) {
		height, rawEntries, err := c.pollMempool()
		if err != nil {
			return nil, err
		}
		entries := make(map[string]col.MempoolEntry)
		for txid, rawEntry := range rawEntries {
			entries[txid] = rawEntry
		}
		col.PruneLowFee(entries, relayfee)
		s := &col.MempoolState{
			Height:     height,
			Entries:    entries,
			Time:       timeNow(),
			MinFeeRate: relayfee,
		}
		return s, nil
	}
	getBlock := func(height int64) (col.Block, error) {
		return c.getBlock(height)
	}
	return getState, getBlock, nil
}

// Unix time in seconds
type UnixNow func() int64

type Config struct {
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`

	// HTTP timeout in seconds
	Timeout int `json:"timeout" yaml:"timeout"`
}

type request struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Id      int64       `json:"id"`
}

type response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   interface{}     `json:"error"`
	Id      int64           `json:"id"`
}

type client struct {
	currid     int64
	httpclient *http.Client
	cfg        Config
}

func newClient(cfg Config) *client {
	c := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	return &client{cfg: cfg, httpclient: c}
}

func (r *client) newRequest(method string, params interface{}) *request {
	return &request{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		Id:      atomic.AddInt64(&r.currid, 1),
	}
}

func (c *client) getNetworkInfo() (map[string]interface{}, error) {
	var info map[string]interface{}
	req := c.newRequest("getnetworkinfo", nil)
	res, err := c.send(req)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(res, &info)
	return info, err
}

// Return a hex-encoded blockhash, used by GetBlock()
func (c *client) getBlockHash(height int64) (hash string, err error) {
	req := c.newRequest("getblockhash", []int64{height})
	resp, err := c.send(req)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp, &hash)
	return
}

// Get a Block by height
func (c *client) getBlock(height int64) (*block, error) {
	hash, err := c.getBlockHash(height)
	if err != nil {
		return nil, err
	}

	req := c.newRequest("getblock", []interface{}{hash, true})
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}

	b := new(block)
	err = json.Unmarshal(resp, b)
	return b, err
}

// Batch request for getrawmempool and getblockcount
func (c *client) pollMempool() (height int64, entries map[string]*MempoolEntry, err error) {
	reqs := []*request{
		c.newRequest("getrawmempool", []bool{true}),
		c.newRequest("getblockcount", nil),
	}
	resp, err := c.sendbatch(reqs)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp[0], &entries)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp[1], &height)
	return
}

func (c *client) getRelayFee() (sim.FeeRate, error) {
	info, err := c.getNetworkInfo()
	if err != nil {
		return 0, err
	}
	relayfee := sim.FeeRate(info["relayfee"].(float64) * coin)
	return relayfee, nil
}

// Send an RPC req.
func (c *client) send(rpcreq *request) (json.RawMessage, error) {
	reqbody, err := json.Marshal(rpcreq)
	if err != nil {
		return nil, err
	}
	respbody, err := c.sendhttp(reqbody)
	if err != nil {
		return nil, err
	}
	var rpcresp response
	if err := json.Unmarshal(respbody, &rpcresp); err != nil {
		return nil, err
	}
	// Error on mismatched Id field
	if rpcresp.Id != rpcreq.Id {
		return nil, fmt.Errorf("mismatched RPC id")
	}
	if rpcresp.Error != nil {
		return nil, fmt.Errorf("%v", rpcresp.Error)
	}
	return rpcresp.Result, nil
}

// Send batch RPC request
func (c *client) sendbatch(rpcreqs []*request) ([]json.RawMessage, error) {
	// Take note of ids
	idlist := make([]int64, len(rpcreqs))
	for i, rpcreq := range rpcreqs {
		idlist[i] = rpcreq.Id
	}

	reqbody, err := json.Marshal(rpcreqs)
	if err != nil {
		return nil, err
	}

	respbody, err := c.sendhttp(reqbody)
	if err != nil {
		return nil, err
	}

	// Match the Ids; return in the same order as the request
	rpcresps := make([]response, len(rpcreqs))
	if err := json.Unmarshal(respbody, &rpcresps); err != nil {
		return nil, err
	}

	result := make([]json.RawMessage, len(rpcreqs))

IDLoop:
	for i, reqid := range idlist {
		for _, rpcresp := range rpcresps {
			if reqid == rpcresp.Id {
				if rpcresp.Error != nil {
					// Return an error if even one rpc request failed
					return nil, fmt.Errorf("%v", rpcresp.Error)
				}
				result[i] = rpcresp.Result
				continue IDLoop
			}
		}
		// No ID match was found
		return nil, fmt.Errorf("unmatched req/resp IDs")
	}

	return result, nil
}

// Send the HTTP request
func (c *client) sendhttp(body []byte) ([]byte, error) {
	url := "http://" + net.JoinHostPort(c.cfg.Host, c.cfg.Port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	resp, err := c.httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%v: %s", resp.Status, b)
	}

	return b, nil
}

// Package api provides a client for accessing the feesim services through its
// JSON-RPC API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	jsonrpc "github.com/gorilla/rpc/json"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/collect/corerpc"
	"github.com/bitcoinfees/feesim/sim"
)

type Config struct {
	Host    string
	Port    string
	Timeout int
}

type Client struct {
	httpclient *http.Client
	cfg        Config
}

func NewClient(cfg Config) *Client {
	httpclient := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	return &Client{httpclient: httpclient, cfg: cfg}
}

func (c *Client) Stop() error {
	_, err := c.doRPC("stop", nil)
	return err
}

func (c *Client) Status() (map[string]string, error) {
	r, err := c.doRPC("status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]string
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) EstimateFee(n int) (interface{}, error) {
	r, err := c.doRPC("estimatefee", n)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Scores() (map[string][]float64, error) {
	r, err := c.doRPC("predictscores", nil)
	if err != nil {
		return nil, err
	}

	var result map[string][]float64
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) TxRate(n int) (map[string][]float64, error) {
	r, err := c.doRPC("txrate", n)
	if err != nil {
		return nil, err
	}

	var result map[string][]float64
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CapRate(n int) (map[string][]float64, error) {
	r, err := c.doRPC("caprate", n)
	if err != nil {
		return nil, err
	}

	var result map[string][]float64
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) MempoolSize(n int) (map[string][]float64, error) {
	r, err := c.doRPC("mempoolsize", n)
	if err != nil {
		return nil, err
	}

	var result map[string][]float64
	if err := json.Unmarshal(r, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Pause() error {
	_, err := c.doRPC("pause", nil)
	return err
}

func (c *Client) Unpause() error {
	_, err := c.doRPC("unpause", nil)
	return err
}

func (c *Client) SetDebug(d bool) error {
	_, err := c.doRPC("setdebug", d)
	return err
}

func (c *Client) Config() (map[string]interface{}, error) {
	r, err := c.doRPC("config", nil)
	if err != nil {
		return nil, err
	}

	v := make(map[string]interface{})
	if err := json.Unmarshal(r, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) Metrics() (map[string]interface{}, error) {
	r, err := c.doRPC("metrics", nil)
	if err != nil {
		return nil, err
	}

	v := make(map[string]interface{})
	if err := json.Unmarshal(r, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) BlockSource() (map[string]interface{}, error) {
	r, err := c.doRPC("blocksource", nil)
	if err != nil {
		return nil, err
	}

	v := make(map[string]interface{})
	if err := json.Unmarshal(r, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) MempoolState() (*col.MempoolState, error) {
	// This depends on the corerpc implementation of col.MempoolEntry
	r, err := c.doRPC("mempoolstate", nil)
	if err != nil {
		return nil, fmt.Errorf("error doRPC: %v", err)
	}

	var (
		height, t  int64
		minfeerate sim.FeeRate
		entries    map[string]*corerpc.MempoolEntry
	)

	var v map[string]json.RawMessage
	if err := json.Unmarshal(r, &v); err != nil {
		return nil, fmt.Errorf("error unmarshaling map: %v", err)
	}

	if err := json.Unmarshal(v["entries"], &entries); err != nil {
		return nil, fmt.Errorf("error unmarshaling entries: %v", err)
	}
	if err := json.Unmarshal(v["height"], &height); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(v["time"], &t); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(v["minfeerate"], &minfeerate); err != nil {
		return nil, err
	}

	stateEntries := make(map[string]col.MempoolEntry)
	for txid, entry := range entries {
		stateEntries[txid] = entry
	}

	s := &col.MempoolState{
		Height:     height,
		Entries:    stateEntries,
		Time:       t,
		MinFeeRate: minfeerate,
	}
	return s, nil
}

func (c *Client) doRPC(method string, args interface{}) (json.RawMessage, error) {
	b, err := jsonrpc.EncodeClientRequest(method, args)
	if err != nil {
		return nil, fmt.Errorf("jsonrpc.EncodeClientRequest: %v", err)
	}

	url := "http://" + net.JoinHostPort(c.cfg.Host, c.cfg.Port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var m json.RawMessage
	if err := jsonrpc.DecodeClientResponse(resp.Body, &m); err != nil {
		return nil, fmt.Errorf("jsonrpc.DecodeClientRequest: %v", err)
	}
	return m, nil
}

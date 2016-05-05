package corerpc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/bitcoinfees/feesim/testutil"
)

var (
	cfg        Config
	testclient *client
)

// To run tests, you need to put Config in ./config.json.
func TestMain(m *testing.M) {
	const configFile = "config.json"

	cfg = Config{
		Host:    "localhost",
		Port:    "8332",
		Timeout: 15,
	}
	if f, err := ioutil.ReadFile(configFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else if err := json.Unmarshal(f, &cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	testclient = newClient(cfg)

	testutil.LoadData("../../testutil/testdata/")
	os.Exit(m.Run())
}

func TestGetters(t *testing.T) {
	const tm int64 = 11
	timeNow := func() int64 { return tm }
	getState, getBlock, err := Getters(timeNow, cfg)
	if err != nil {
		t.Fatal(err)
	}
	state, err := getState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Height < 400000 {
		t.Error("Bad state height.")
	}
	if err := testutil.CheckEqual(state.Time, tm); err != nil {
		t.Error(err)
	}
	if len(state.Entries) == 0 {
		t.Error("Something wrong with entries.")
	}
	t.Log("relayfee:", state.MinFeeRate)
	// Test that entries are pruned
	for _, entry := range state.Entries {
		if entry.FeeRate() < state.MinFeeRate {
			t.Fatal("Mempool entries were not pruned.")
		}
	}

	if _, err := getBlock(state.Height); err != nil {
		t.Error(err)
	}
}

func TestRPC(t *testing.T) {
	// Test getInfo
	if info, err := testclient.getInfo(); err != nil {
		t.Error(err)
	} else {
		t.Logf("%+v", info)
	}

	// Test getRelayFee
	if fee, err := testclient.getRelayFee(); err != nil {
		t.Error(err)
	} else {
		t.Logf("relayfee: %+v sats/kB", fee)
	}

	// Test pollMempool
	height, txs, err := testclient.pollMempool()
	if err != nil {
		t.Fatal("Error polling mempool:", err)
	}
	if height < 400000 {
		t.Fatal("Height is wrong.")
	}
	if len(txs) == 0 {
		t.Fatal("No txs")
	}
	// For each MempoolEntry field, check that at least one tx has a non-zero
	// value; most likely case is that either they were all unmarshaled
	// correctly, or all wrongly.
	var maxtx MempoolEntry
	for txid, tx := range txs {
		if txid == "" {
			t.Fatal("Empty txid.")
		}
		if tx.Size() > maxtx.Size() {
			maxtx.Size_ = tx.Size_
		}
		if tx.Fee > maxtx.Fee {
			maxtx.Fee = tx.Fee
		}
		if tx.Time() > maxtx.Time() {
			maxtx.Time_ = tx.Time_
		}
		if tx.CurrentPriority > maxtx.CurrentPriority {
			maxtx.CurrentPriority = tx.CurrentPriority
		}
		if len(tx.Depends()) > len(maxtx.Depends()) {
			maxtx.Depends_ = tx.Depends_
		}
	}
	// Unlikely to happen
	if maxtx.Size() == 0 || maxtx.Fee == 0 || maxtx.Time() == 0 ||
		maxtx.CurrentPriority == 0 || len(maxtx.Depends()) == 0 {
		t.Error("Empty fields.")
	}

	// Test getBlock
	block, err := testclient.getBlock(height)
	if err != nil {
		t.Fatal(err)
	}
	blockBytes, err := block.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(block.Height(), height); err != nil {
		t.Error(err)
	}
	t.Log("Blocksize:", len(blockBytes))
	if len(blockBytes) == 0 {
		t.Error("zero size block")
	}
}

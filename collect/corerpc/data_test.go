package corerpc

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

func TestMempoolEntry(t *testing.T) {
	entry := &MempoolEntry{
		Size_:           999,
		Fee:             0.0001,
		Depends_:        []string{"0", "1"},
		Time_:           300,
		CurrentPriority: 57600001,
	}

	f := entry.FeeRate()
	if err := testutil.CheckEqual(f, sim.FeeRate(10010)); err != nil {
		t.Error(err)
	}
	p := entry.IsHighPriority()
	if err := testutil.CheckEqual(p, true); err != nil {
		t.Error(err)
	}
	tm := entry.Time()
	if err := testutil.CheckEqual(tm, entry.Time_); err != nil {
		t.Error(err)
	}
	d := entry.Depends()
	if err := testutil.CheckEqual(d, entry.Depends_); err != nil {
		t.Error(err)
	}
	// Test that d is a copy
	d[1] = "100"
	d2 := entry.Depends()
	if err := testutil.CheckEqual(d, d2); err == nil {
		t.Error("Depends was mutated")
	}
}

// Test Block methods
func TestBlock(t *testing.T) {
	heights := []int64{333931, 334656}
	numhashes_ref := []float64{1.718333983803829e+20, 1.6947199377788292e+20}
	size_ref := []int64{153669, 443571}
	tag_ref := []string{
		"036b180504548a53678f9ef26000000d7c",
		"03401b050ce9f2cd2079cf5a17e3fba656",
	}

	var blockTxidData map[string][]string
	// Load reference txid data
	f, err := os.Open("testdata/blocktxids.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&blockTxidData)
	if err != nil {
		panic(err)
	}

	for i, height := range heights {
		blockbytes := testutil.GetBlockBytes(height)
		block, err := newBlockFromBytes(blockbytes, height)
		if err != nil {
			t.Fatal(err)
		}

		if err := testutil.CheckEqual(block.Height(), height); err != nil {
			t.Error(err)
		}

		size := block.Size()
		if err := testutil.CheckEqual(size, size_ref[i]); err != nil {
			t.Error(err)
		}

		txidRef := blockTxidData[strconv.Itoa(int(height))]
		txids := block.Txids()
		sort.Strings(txids)
		if err := testutil.CheckEqual(txids, txidRef); err != nil {
			t.Error(err)
		}

		numhashes := block.NumHashes()
		if err := testutil.CheckEqual(numhashes, numhashes_ref[i]); err != nil {
			t.Error(err)
		}

		tag := block.Tag()
		if err != nil {
			t.Fatal(err)
		}
		tag_hex := hex.EncodeToString(tag)
		if err := testutil.CheckEqual(tag_hex, tag_ref[i]); err != nil {
			t.Error(err)
		}
	}
}

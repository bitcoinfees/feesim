package collect

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

const (
	minrelaytxfee = 5000
)

var (
	newTxs []est.Tx
)

func TestMain(m *testing.M) {
	testutil.LoadData("../testutil/testdata")

	f, err := os.Open("testdata/newtxs.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&newTxs)
	if err != nil {
		panic(err)
	}
	sort.Sort(newTxSlice(newTxs))

	os.Exit(m.Run())
}

func TestCollect(t *testing.T) {
	tdb := &MockTxDB{t: t}
	bdb := &MockBlockStatDB{t: t}
	blockidx := 0
	getState := func() (*MempoolState, error) {
		defer func() { blockidx++ }()
		if blockidx > 300000 {
			return statedata(blockidx)
		} else if blockidx < 2 {
			return statedata(333931)
		}
		return statedata(333932)
	}
	cfg := Config{
		GetState:   getState,
		GetBlock:   getBlock,
		PollPeriod: 1,
	}

	c := NewCollector(tdb, bdb, cfg)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}

	// Stop after 3 seconds
	go func() {
		<-time.After(time.Second * 3)
		t.Log("Stopping collector")
		c.Stop()
		t.Log("Collector stopped")
	}()
	seenBlocks := false
	sidx := 0

LoopA:
	for {
		select {
		case state := <-c.S:
			if state == nil {
				continue
			}
			var height int64
			if sidx < 1 {
				height = 333930
			} else {
				height = 333931
			}
			t.Log("Testing state...")
			if err := testutil.CheckEqual(height, state.Height); err != nil {
				t.Error(err)
			}
			sidx++
		case blocks := <-c.B:
			if blocks == nil {
				continue
			}
			if seenBlocks {
				t.Fatal("Should only receive blocks once.")
			}
			t.Log("Testing blocks...")
			if err := testutil.CheckEqual(len(blocks), 1); err != nil {
				t.Error(err)
			}
			if err := testutil.CheckEqual(blocks[0].Height(), int64(333931)); err != nil {
				t.Error(err)
			}
			seenBlocks = true
		case err := <-c.E:
			if err != nil {
				t.Error(err)
			}
			break LoopA
		}
	}

	// Test processBlock error return path
	blockidx = 333932
	c = NewCollector(tdb, bdb, cfg)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
LoopB:
	for {
		select {
		case <-c.S:
		case <-c.B:
		case errCol := <-c.E:
			expected := "GetState: " + errBadHeight.Error()
			if err := testutil.CheckEqual(errCol.Error(), expected); err != nil {
				t.Error(err)
			}
			break LoopB
		}
	}
	c.Stop()

	// Test txdb error return path
	blockidx = 0
	tdb = &MockTxDB{t: t, err: fmt.Errorf("TxDB error")}
	bdb = &MockBlockStatDB{t: t}
	c = NewCollector(tdb, bdb, cfg)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
LoopC:
	for {
		select {
		case <-c.S:
		case <-c.B:
		case errCol := <-c.E:
			expected := "TxDB.Put: " + tdb.err.Error()
			if err := testutil.CheckEqual(errCol.Error(), expected); err != nil {
				t.Error(err)
			}
			break LoopC
		}
	}
	c.Stop()

	// Test blockstatdb error return path
	blockidx = 0
	tdb = &MockTxDB{t: t}
	bdb = &MockBlockStatDB{t: t, err: fmt.Errorf("BlockStatDB error")}
	c = NewCollector(tdb, bdb, cfg)
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
LoopD:
	for {
		select {
		case <-c.S:
		case <-c.B:
		case errCol := <-c.E:
			expected := "BlockStatDB.Put: " + bdb.err.Error()
			if err := testutil.CheckEqual(errCol.Error(), expected); err != nil {
				t.Error(err)
			}
			break LoopD
		}
	}
	c.Stop()
}

type MockBlockStatDB struct {
	t   *testing.T
	err error
}

func (d *MockBlockStatDB) Put(b []*est.BlockStat) error {
	if d.err != nil {
		return d.err
	}
	b_ref := []*est.BlockStat{
		{
			Height:            333931,
			NumHashes:         1.718333983803829e+20,
			Size:              153669,
			Time:              b[0].Time,
			MempoolSize:       681121,
			MempoolSizeRemain: 535628,
			SFRStat: est.SFRStat{
				SFR: 23310,
				AK:  305,
				AN:  308,
				BK:  204,
				BN:  204,
			},
		},
	}
	err := testutil.CheckEqual(b, b_ref)
	if err != nil {
		d.t.Log(err)
	} else {
		d.t.Log("BlockStatDB Put OK")
	}
	return err
}

type MockTxDB struct {
	t   *testing.T
	err error
}

func (d *MockTxDB) Put(txs []est.Tx) error {
	if d.err != nil {
		return d.err
	}
	if len(txs) == 0 {
		return nil
	}

	// Check the txs against a stored reference
	sort.Sort(newTxSlice(txs))
	if err := testutil.CheckEqual(txs, newTxs); err != nil {
		return err
	}

	d.t.Logf("%d txs put.", len(txs))
	return nil
}

func getBlock(height int64) (Block, error) {
	return testutil.GetBlock(height)
}

var errBadHeight = errors.New("invalid mempool height")

func statedata(height int) (*MempoolState, error) {
	heightstr := strconv.Itoa(height)
	rawEntries, ok := testutil.MempoolData[heightstr]
	if !ok {
		return nil, errBadHeight
	}

	entries := toMempoolEntry(rawEntries)
	PruneLowFee(entries, minrelaytxfee)

	t := int64(0)
	for _, entry := range entries {
		if entry.Time() > t {
			t = entry.Time()
		}
	}

	state := &MempoolState{
		Height:     int64(height - 1),
		Entries:    entries,
		Time:       t,
		MinFeeRate: minrelaytxfee,
	}
	return state, nil
}

func toMempoolEntry(rawEntries map[string]*testutil.MempoolEntry) map[string]MempoolEntry {
	entries := make(map[string]MempoolEntry)
	for txid, rawEntry := range rawEntries {
		entries[txid] = &testMempoolEntry{rawEntry}
	}
	return entries
}

type testMempoolEntry struct {
	*testutil.MempoolEntry
}

func (e *testMempoolEntry) Size() sim.TxSize {
	return sim.TxSize(e.MempoolEntry.Size)
}

func (e *testMempoolEntry) FeeRate() sim.FeeRate {
	return sim.FeeRate(e.MempoolEntry.FeeRate())
}

func (e *testMempoolEntry) Time() int64 {
	return e.MempoolEntry.Time
}

func (e *testMempoolEntry) Depends() []string {
	return e.MempoolEntry.Depends
}

// Implements sort.Interface to get a canonical ordering, for test comparisons
type newTxSlice []est.Tx

func (s newTxSlice) Len() int {
	return len(s)
}

func (s newTxSlice) Less(i, j int) bool {
	if s[i].FeeRate != s[j].FeeRate {
		return s[i].FeeRate < s[j].FeeRate
	}
	if s[i].Size != s[j].Size {
		return s[i].Size < s[j].Size
	}
	return s[i].Time < s[j].Time
}

func (s newTxSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

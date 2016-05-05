package collect

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

func BenchmarkPruneLowFee(b *testing.B) {
	entries := toMempoolEntry(testutil.MempoolData["333931"])
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pruned := make(map[string]MempoolEntry)
		for txid, entry := range entries {
			pruned[txid] = entry
		}
		PruneLowFee(pruned, 5000)
	}
}

func TestPruneLowFee(t *testing.T) {
	const thresh = 5000
	for height := 333931; height < 333954; height++ {
		heightStr := strconv.Itoa(height)
		rawEntries, ok := testutil.MempoolData[heightStr]
		if !ok {
			continue
		}
		entries := toMempoolEntry(rawEntries)

		// Make a copy of entries
		pruned := make(map[string]MempoolEntry)
		for txid, entry := range entries {
			pruned[txid] = entry
		}
		PruneLowFee(pruned, thresh)
		t.Logf("Height %d: %d pruned", height, len(entries)-len(pruned))

		// Check that all pruned entries are correctly pruned
		checked := make(map[string]error)
		for txid := range entries {
			if _, ok := pruned[txid]; !ok {
				if err := pruneAssert(txid, entries, thresh, checked); err != nil {
					t.Fatal(err)
				}
			}
		}

		// Check that none of the unpruned entries deserve to be pruned
		checked = make(map[string]error)
		for txid := range pruned {
			if err := unpruneAssert(txid, entries, thresh, checked); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// pruneAssert asserts that at least one of the ancestors of the tx specified by
// txid has a feerate < thresh
func pruneAssert(txid string, entries map[string]MempoolEntry, thresh sim.FeeRate, checked map[string]error) error {
	if err, ok := checked[txid]; ok {
		return err
	}
	var err error
	defer func() { checked[txid] = err }()

	if entries[txid].FeeRate() < thresh {
		return nil
	}
	for _, parent := range entries[txid].Depends() {
		if err := pruneAssert(parent, entries, thresh, checked); err == nil {
			return nil
		}
	}
	err = fmt.Errorf("all ancestors have feerate >= thresh.")
	return err
}

// unpruneAssert asserts that all ancestors of the tx specified by txid have a
// feerate >= thresh
func unpruneAssert(txid string, entries map[string]MempoolEntry, thresh sim.FeeRate, checked map[string]error) error {
	if err, ok := checked[txid]; ok {
		return err
	}
	var err error
	defer func() { checked[txid] = err }()

	if entries[txid].FeeRate() < thresh {
		err = fmt.Errorf("ancestor has feerate < thresh")
		return err
	}
	for _, parent := range entries[txid].Depends() {
		if err = unpruneAssert(parent, entries, thresh, checked); err != nil {
			return err
		}
	}
	return nil
}

func TestSimifyMempool(t *testing.T) {
	// This is copied from sim.TestSimSFR
	const (
		mfr sim.FeeRate = 10000 // min fee rate
		mbs sim.TxSize  = 50000 // max block size
	)
	var entries map[string]MempoolEntry
	if s, err := statedata(333931); err != nil {
		t.Fatal(err)
	} else {
		entries = s.Entries
	}
	initmempool, err := SimifyMempool(entries)
	if err != nil {
		t.Fatal(err)
	}

	blocksource := sim.NewIndBlockSource([]sim.FeeRate{mfr}, []sim.TxSize{mbs}, 1./600)
	txsource := sim.NewMultiTxSource(nil, nil, nil, 0)
	s := sim.NewSim(txsource, blocksource, initmempool)

	// This ref data obtained from an earlier implementation
	sfrsRef := []sim.FeeRate{
		44248,
		38462,
		22884,
		21187,
		16156,
		13840,
		11423,
		11199,
		11249,
		10961,
		10194,
		10000,
	}
	for _, sfrRef := range sfrsRef {
		sfr, blocksize := s.NextBlock()
		t.Logf("%d/%d", sfr, blocksize)
		if err := testutil.CheckEqual(sfr, sfrRef); err != nil {
			t.Error(err)
		}
	}
}

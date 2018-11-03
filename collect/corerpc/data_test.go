package corerpc

import (
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
	}

	f := entry.FeeRate()
	if err := testutil.CheckEqual(f, sim.FeeRate(10010)); err != nil {
		t.Error(err)
	}
	p := entry.IsHighPriority()
	if err := testutil.CheckEqual(p, false); err != nil {
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

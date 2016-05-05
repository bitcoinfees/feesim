package sim

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestUniTxSource(t *testing.T) {
	const (
		N = 100000 // 100000 txs
	)
	T := time.Duration(600) * time.Second // 10 min interblock times
	f := []FeeRate{20000, 20000, 20000, 20000, 20000, 10000, 10000, 10000, 5000, 5000}
	s := []TxSize{250, 250, 250, 250, 250, 500, 500, 500, 1000, 1000}
	prop := []float64{0.5, 0.3, 0.2}
	txrate := 1.5
	txsrc := NewUniTxSource(f, s, txrate)

	// Assert that UniTxSource implements TxSource
	var _ TxSource = txsrc

	n := 0
	tm := time.Duration(0)
	count := make([]float64, 3)
	numarrives := []float64{}
	for n < N {
		newtxs := txsrc.Generate(T)
		n += len(newtxs)
		tm += T
		numarrives = append(numarrives, float64(len(newtxs)))
		for _, tx := range newtxs {
			switch tx.FeeRate {
			case 20000:
				count[0]++
			case 10000:
				count[1]++
			case 5000:
				count[2]++
			}
		}
	}
	for i := range count {
		count[i] /= float64(n)
	}

	testrate := float64(n) / tm.Seconds()
	t.Logf("rate/expected: %f/%f", testrate, txrate)
	if err := testutil.CheckPctDiff(testrate, txrate, 0.01); err != nil {
		t.Error(err)
	}

	t.Logf("Proportions/expected: %v/%v", count, prop)
	for i := range count {
		if err := testutil.CheckPctDiff(count[i], prop[i], 0.014); err != nil {
			t.Error(err)
		}
	}

	// Test moments of tx arrival count
	mean, variance := moments(numarrives)
	l := T.Seconds() * txrate
	t.Logf("numarrives mean/variance/expected: %f/%f/%f", mean, variance, l)
	if err := testutil.CheckPctDiff(mean, l, 0.01); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckPctDiff(variance, l, 0.1); err != nil {
		t.Error(err)
	}

	// Test MarshalJON
	srcJSON, err := json.MarshalIndent(txsrc, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(srcJSON))

	// Test RateFn
	ratefn := txsrc.RateFn()
	xref := []float64{-1, 4999, 5000, 5001, 9999, 10000, 10001, 19999, 20000, 20001}
	yref := []float64{712.5, 712.5, 712.5, 412.5, 412.5, 412.5, 187.5, 187.5, 187.5, 0}
	for i, x := range xref {
		if err := testutil.CheckEqual(ratefn.Eval(x), yref[i]); err != nil {
			t.Error(err)
		}
	}
	yref = []float64{-1, 0, 187.4, 187.5, 187.6, 412.4, 412.5, 412.6, 712.4, 712.5, 712.6, 1000}
	xref = []float64{20001, 20001, 20001, 10001, 10001, 10001, 5001, 5001, 5001, 0, 0, 0}
	for i, y := range yref {
		if err := testutil.CheckEqual(ratefn.Inverse(y), xref[i]); err != nil {
			t.Error("y is", y, err)
		}
	}
	// Test RateFn approx
	approx := ratefn.Approx(20)
	t.Log("TxRateFn approx:", approx)

	// Test null source
	txsrc = NewUniTxSource([]FeeRate{}, []TxSize{}, 0)
	txs := txsrc.Generate(time.Second * 10)
	if err := testutil.CheckEqual(len(txs), 0); err != nil {
		t.Error(err)
	}
	ratefn = txsrc.RateFn()
	if err := testutil.CheckEqual(ratefn.Eval(1000), float64(0)); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(ratefn.Inverse(1000), float64(0)); err != nil {
		t.Error(err)
	}
}

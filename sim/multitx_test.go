package sim

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestMultiTxSource(t *testing.T) {
	const (
		N = 100000 // 100000 txs
	)
	T := time.Duration(600) * time.Second // 10 min interblock times
	f := []FeeRate{20000, 10000, 5000}
	s := []TxSize{250, 500, 1000}
	prop := []float64{0.5, 0.3, 0.2}
	w := make([]float64, 3)
	for i := range w {
		// Weights doesn't need to sum to 1, so we test the general case by
		// multiplying with an arbitrary factor.
		w[i] = prop[i] * 2
	}
	txrate := 1.5
	m := NewMultiTxSource(f, s, w, txrate)
	n := 0
	tm := time.Duration(0)
	count := make([]float64, 3)
	numarrives := []float64{}
	for n < N {
		newtxs := m.Generate(T)
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
		if err := testutil.CheckPctDiff(count[i], prop[i], 0.01); err != nil {
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
	mJSON, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(mJSON))

	// Test RateFn
	ratefn := m.RateFn()
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

	// Test RateFn for null source
	m = NewMultiTxSource([]FeeRate{}, []TxSize{}, []float64{}, 0)
	ratefn = m.RateFn()
	if err := testutil.CheckEqual(ratefn.Eval(1000), float64(0)); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(ratefn.Inverse(1000), float64(0)); err != nil {
		t.Error(err)
	}

	// TODO: test ratefn for 1 tx?
}

func TestPoissonVariate(t *testing.T) {
	r := getrand(1)[0]
	const n = 10000
	x := make([]float64, n)

	// Test for l < 30
	var l float64 = 25
	for i := range x {
		x[i] = float64(poissonvariate(l, r))
	}
	mean, variance := moments(x)
	t.Logf("Mean/variance/expected: %f/%f/%f", mean, variance, l)

	// Too lazy to do a proper statistical test, so just see if it's "close"
	if err := testutil.CheckPctDiff(mean, l, 0.01); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckPctDiff(variance, l, 0.01); err != nil {
		t.Error(err)
	}

	// Test for l > 30 (normal approx)
	l = 1000
	for i := range x {
		x[i] = float64(poissonvariate(l, r))
	}
	mean, variance = moments(x)
	t.Logf("Mean/variance/expected: %f/%f/%f", mean, variance, l)
	if err := testutil.CheckPctDiff(mean, l, 0.01); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckPctDiff(variance, l, 0.01); err != nil {
		t.Error(err)
	}

	// Test for l == 0
	p := poissonvariate(0, r)
	if err := testutil.CheckEqual(p, int64(0)); err != nil {
		t.Error(err)
	}
}

// Estimate mean + var
func moments(x []float64) (mean float64, variance float64) {
	n := float64(len(x))
	var total float64
	for _, v := range x {
		total += v
	}
	mean = total / n

	var s float64
	for _, v := range x {
		s += math.Pow(v-mean, 2)
	}
	variance = s / (n - 1)
	return
}

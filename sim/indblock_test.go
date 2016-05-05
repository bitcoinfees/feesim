package sim

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestIndBlockSource(t *testing.T) {
	const N = 10000 // 10000 blocks
	minfeerates := []FeeRate{5000, 10000, 20000}
	maxblocksizes := []TxSize{2000000, 1000000, 950000, 750000}
	blockrate := 1.0 / 600.0 // once per 10 minutes
	b := NewIndBlockSource(minfeerates, maxblocksizes, blockrate)
	T := time.Duration(0)
	count := BlockPolicy{}
	for i := 0; i < N; i++ {
		tm, p := b.Next()
		T += tm
		count.MaxBlockSize += p.MaxBlockSize
		count.MinFeeRate += p.MinFeeRate
		if i < 10 {
			t.Logf("%+v", p)
		}
	}
	avgMinFeeRate := count.MinFeeRate / N
	avgMaxBlockSize := count.MaxBlockSize / N
	t.Logf("Avg mbs/mfr: %d/%d", avgMaxBlockSize, avgMinFeeRate)
	if err := testutil.CheckPctDiff(float64(avgMinFeeRate), 11666, 0.01); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckPctDiff(float64(avgMaxBlockSize), 1.175e6, 0.01); err != nil {
		t.Error(err)
	}

	simblockrate := float64(N) / T.Seconds()
	t.Logf("simblockrate: %f", simblockrate)
	if err := testutil.CheckPctDiff(simblockrate, blockrate, 0.01); err != nil {
		t.Error(err)
	}

	// Test MarshalJSON
	bJSON, err := json.MarshalIndent(b, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bJSON))

	// Test RateFn
	minfeerates = append(minfeerates, 5000)
	b = NewIndBlockSource(minfeerates, maxblocksizes, blockrate)
	ratefn := b.RateFn()
	xref := []float64{-1, 4999, 5000, 5001, 9999, 10000, 10001, 19999, 20000, 20001}
	yref := []float64{0, 0, 979, 979, 979, 1468, 1468, 1468, 1958, 1958}
	for i, x := range xref {
		if err := testutil.CheckEqual(int(ratefn.Eval(x)), int(yref[i])); err != nil {
			t.Error(err)
		}
	}
	yref = []float64{-1, 0, 979, 980, 1468, 1468.76, 1469, 1958, 10000}
	xref = []float64{0, 0, 5000, 10000, 10000, 20000, 20000, 20000, float64(MaxFeeRate)}
	for i, y := range yref {
		if err := testutil.CheckEqual(ratefn.Inverse(y), xref[i]); err != nil {
			t.Error(err)
		}
	}
	// Test RateFn approx
	approx := ratefn.Approx(20)
	t.Log("CapRateFn approx:", approx)

	// Test null source
	b = NewIndBlockSource([]FeeRate{MaxFeeRate}, []TxSize{1e6}, blockrate)
	ratefn = b.RateFn()
	if err := testutil.CheckEqual(ratefn.Eval(1000), float64(0)); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(ratefn.Inverse(1), float64(MaxFeeRate)); err != nil {
		t.Error(err)
	}
	b = NewIndBlockSource([]FeeRate{1000}, []TxSize{0}, blockrate)
	ratefn = b.RateFn()
	if err := testutil.CheckEqual(ratefn.Eval(1000), float64(0)); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(ratefn.Inverse(1), float64(MaxFeeRate)); err != nil {
		t.Error(err)
	}
}

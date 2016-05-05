package sim

import (
	"encoding/json"
	"math/rand"
	"sort"
	"time"
)

// Implements BlockSource; models max block size and min fee rate as independent
// random variables. Not concurrent safe.
type IndBlockSource struct {
	minfeerates   []FeeRate
	maxblocksizes []TxSize
	blockrate     float64 // blocks per second
	rand          *rand.Rand
}

func NewIndBlockSource(minfeerates []FeeRate, maxblocksizes []TxSize, blockrate float64) *IndBlockSource {
	if blockrate <= 0 {
		panic("blockrate must be > 0")
	}
	if len(minfeerates) == 0 || len(maxblocksizes) == 0 {
		panic("minfeerates and maxblocksizes must have len > 0.")
	}
	return &IndBlockSource{
		minfeerates:   minfeerates,
		maxblocksizes: maxblocksizes,
		blockrate:     blockrate,
		rand:          getrand(1)[0],
	}
}

func (b *IndBlockSource) Next() (t time.Duration, p BlockPolicy) {
	t = time.Duration(b.rand.ExpFloat64() / b.blockrate * float64(time.Second))
	p.MinFeeRate = b.minfeerates[b.rand.Intn(len(b.minfeerates))]
	p.MaxBlockSize = b.maxblocksizes[b.rand.Intn(len(b.maxblocksizes))]
	return
}

func (b *IndBlockSource) Copy(n int) []BlockSource {
	bb := make([]BlockSource, n)
	r := getrand(n + 1)
	for i := range bb {
		bb[i] = &IndBlockSource{
			minfeerates:   b.minfeerates,
			maxblocksizes: b.maxblocksizes,
			blockrate:     b.blockrate,
			rand:          r[i+1],
		}
	}
	return bb
}

func (b *IndBlockSource) RateFn() MonotonicFn {
	// Calculate the average max block size
	sizesum := TxSize(0)
	for _, s := range b.maxblocksizes {
		sizesum += s
	}
	avgmbs := float64(sizesum) / float64(len(b.maxblocksizes))

	m := make(map[float64]float64)
	for _, f := range b.minfeerates {
		if f < MaxFeeRate {
			m[float64(f)] += 1 / float64(len(b.minfeerates))
		}
	}
	x := make([]float64, len(m))
	for k := range m {
		x = append(x, float64(k))
	}
	sort.Float64s(x)
	ratesum := float64(0)
	y := make([]float64, len(x))
	for i, f := range x {
		ratesum += m[f] * avgmbs * b.blockrate
		y[i] = ratesum
	}
	return NewCapRateFn(x, y)
}

func (b *IndBlockSource) MarshalJSON() ([]byte, error) {
	minfeerates := make([]float64, len(b.minfeerates))
	for i, feerate := range b.minfeerates {
		if feerate == MaxFeeRate {
			minfeerates[i] = -1
		} else {
			minfeerates[i] = float64(feerate)
		}
	}
	sort.Float64s(minfeerates)

	maxblocksizes := make([]float64, len(b.maxblocksizes))
	for i, size := range b.maxblocksizes {
		maxblocksizes[i] = float64(size)
	}
	sort.Float64s(maxblocksizes)

	v := make(map[string]interface{})
	v["minfeerates"] = minfeerates
	v["maxblocksizes"] = maxblocksizes
	v["blockrate"] = b.blockrate
	v["type"] = "IndBlockSource"
	return json.Marshal(v)
}

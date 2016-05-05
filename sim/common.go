package sim

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

const (
	MaxFeeRate FeeRate = math.MaxInt64
	MaxTxSize  TxSize  = math.MaxInt64
)

type (
	FeeRate int64 // satoshis per kB
	TxSize  int64 // in bytes
)

type Tx struct {
	FeeRate FeeRate `json:"feerate"`
	Size    TxSize  `json:"size"`
	Parents []*Tx

	children       []*Tx
	removedparents int
}

// If a block won't include any txs regardless of fee, set
// MinFeeRate = MaxFeeRate.
type BlockPolicy struct {
	MaxBlockSize TxSize
	MinFeeRate   FeeRate
}

// Priority queue with a heap, see heap.go
type txqueue []*Tx

// A simulation tx source. For a given time interval t, Generate returns a slice
// of transactions generated in that interval. For example, if tx arrivals are
// modeled as a Poisson process, then len(txs) is Poisson distributed with
// expected value txrate*t.
//
// t is typically an inter-block time and hence an exponential random variable.
// Since Generate is not a function of time, we are modeling the tx arrival
// process as homogeneous.
//
// A null source is permitted (i.e. a source which always Generates length-zero
// txs.
// TODO: Add unmarshalers
type TxSource interface {
	Generate(t time.Duration) (txs []*Tx)

	// Return n copies of this source, which must have isolated random states.
	// This is so that:
	//     1. The source will be concurrent-safe.
	//     2. The sources' randomness are not coupled; this allows tests to be
	//        made deterministic.
	Copy(n int) []TxSource

	// Returns the minimum tx size that this source will generate.
	// It's used to optimize Sim.
	MinSize() TxSize

	// Returns the reverse cumulative transaction byte rate in bytes/s with
	// respect to fee rate.
	RateFn() MonotonicFn

	MarshalJSON() ([]byte, error)
}

// A simulation block source.
type BlockSource interface {
	Next() (t time.Duration, b BlockPolicy)

	// Return n copies of this source, which must have isolated random states.
	// This is so that:
	//     1. The source will be concurrent-safe.
	//     2. The sources' randomness are not coupled; this allows tests to be
	//        made deterministic.
	Copy(n int) []BlockSource

	// Returns the cumulative capacity byte rate in bytes/s with
	// respect to fee rate.
	RateFn() MonotonicFn

	MarshalJSON() ([]byte, error)
}

// MonotonicFn represents a (non-strict) monotonic function.
type MonotonicFn interface {
	Eval(x float64) float64
	Inverse(y float64) float64
	Approx(n int) MonotonicFn
	MarshalJSON() ([]byte, error)
}

type TxRateFn struct {
	x, y []float64 // x is sorted; y is reverse sorted.
}

func NewTxRateFn(x, y []float64) TxRateFn {
	if len(x) != len(y) {
		panic("x and y must have same len")
	}
	if !sort.Float64sAreSorted(x) {
		panic("x must be sorted.")
	}
	if !sort.IsSorted(sort.Reverse(sort.Float64Slice(y))) {
		panic("y must be reverse sorted.")
	}
	return TxRateFn{x: x, y: y}
}

// What is the total byterate of all transactions with fee rate >= x.
func (f TxRateFn) Eval(x float64) (y float64) {
	i := sort.SearchFloat64s(f.x, x)
	if i < len(f.x) {
		return f.y[i]
	}
	return 0
}

// For a given byterate y, what is the lowest fee rate x such that the total
// byterate of transactions with fee rate >= x is <= y.
func (f TxRateFn) Inverse(y float64) (x float64) {
	idx := sort.Search(len(f.y), func(i int) bool { return f.y[i] <= y })
	if idx == 0 {
		return 0
	}
	return f.x[idx-1] + 1
}

func (f TxRateFn) Approx(n int) MonotonicFn {
	if len(f.x) == 0 {
		return NewTxRateFn(nil, nil)
	}
	max := f.Eval(0)
	x := make([]float64, n)
	for i := range x {
		y := float64(n-i) * max / float64(n)
		x[i] = f.Inverse(y)
	}

	// Remove duplicate x entries
	xd := []float64{x[0]}
	yd := []float64{f.Eval(x[0])}
	xprev := x[0]
	for _, xi := range x {
		if xi != xprev {
			xd = append(xd, xi)
			yd = append(yd, f.Eval(xi))
		}
		xprev = xi
	}

	return NewTxRateFn(xd, yd)
}

func (f TxRateFn) MarshalJSON() ([]byte, error) {
	v := make(map[string][]float64)
	v["x"] = f.x
	v["y"] = f.y
	return json.Marshal(v)
}

type CapRateFn struct {
	x, y []float64 // x and y are sorted.
}

func NewCapRateFn(x, y []float64) CapRateFn {
	if len(x) != len(y) {
		panic("x and y must have same len")
	}
	if !(sort.Float64sAreSorted(x) && sort.Float64sAreSorted(y)) {
		panic("x and y must be sorted.")
	}
	return CapRateFn{x: x, y: y}
}

// The avg max block size * block rate for all miners with minfeerate <= x
func (f CapRateFn) Eval(x float64) (y float64) {
	i := sort.SearchFloat64s(f.x, x)
	switch {
	case i < len(f.y) && f.x[i] == x:
		return f.y[i]
	case i > 0:
		return f.y[i-1]
	default:
		return 0
	}
}

// For a given y, what is the min fee rate x such that the cumulative
// caprate is >= y. If y >= max cap, return MaxFeeRate
func (f CapRateFn) Inverse(y float64) (x float64) {
	if y <= 0 {
		return 0
	}
	idx := sort.SearchFloat64s(f.y, y)
	if idx == len(f.y) {
		return float64(MaxFeeRate)
	}
	return f.x[idx]
}

func (f CapRateFn) Approx(n int) MonotonicFn {
	max := f.Eval(math.MaxFloat64) - 1
	x := make([]float64, n)
	for i := range x {
		y := float64(n-i) * max / float64(n)
		x[i] = f.Inverse(y)
	}

	// Remove duplicate x entries
	xd := []float64{x[len(x)-1]}
	yd := []float64{f.Eval(xd[0])}
	xprev := xd[0]
	for i := len(x) - 1; i >= 0; i-- {
		if x[i] != xprev {
			xd = append(xd, x[i])
			yd = append(yd, f.Eval(x[i]))
		}
		xprev = x[i]
	}

	return NewCapRateFn(xd, yd)
}

func (f CapRateFn) MarshalJSON() ([]byte, error) {
	v := make(map[string][]float64)
	v["x"] = f.x
	v["y"] = f.y
	return json.Marshal(v)
}

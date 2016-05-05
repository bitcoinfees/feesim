package sim

import (
	"encoding/json"
	"math/rand"
	"sort"
	"time"
)

// The typical tx source: poisson arrivals with independent (feerate, size).
// Implements TxSource. Not concurrent-safe.
type MultiTxSource struct {
	txs     []Tx
	weights []float64
	index   []float64
	txrate  float64 // txs per second
	minSize TxSize
	rand    *rand.Rand
}

// txs and weights must be nonzero.
func NewMultiTxSource(feerates []FeeRate, sizes []TxSize, weights []float64, txrate float64) *MultiTxSource {
	if len(feerates) != len(weights) || len(sizes) != len(weights) {
		panic("feerates / sizes / weights must have same len")
	}
	if len(feerates) == 0 {
		txrate = 0
	}
	var weightsTotal float64
	minSize := MaxTxSize
	index := make([]float64, len(feerates))
	txs := make([]Tx, len(feerates))
	for i, wi := range weights {
		if wi <= 0 {
			panic("weights must be positive")
		}
		weightsTotal += wi
		index[i] = weightsTotal

		txs[i].FeeRate, txs[i].Size = feerates[i], sizes[i]
		if sizes[i] < minSize {
			minSize = sizes[i]
		}
	}
	for i := range index {
		index[i] /= weightsTotal
		weights[i] /= weightsTotal
	}
	return &MultiTxSource{
		txs:     txs,
		weights: weights,
		index:   index,
		txrate:  txrate,
		minSize: minSize,
		rand:    getrand(1)[0],
	}
}

func (s *MultiTxSource) Generate(t time.Duration) (txs []*Tx) {
	l := t.Seconds() * s.txrate    // Expected number of txs
	n := poissonvariate(l, s.rand) // Actual number of txs
	txs = make([]*Tx, n)
	for i := range txs {
		x := s.rand.Float64()
		pos := searchFloat64s(s.index, x)
		txs[i] = &s.txs[pos]
	}
	return
}

func (s *MultiTxSource) Copy(n int) []TxSource {
	ss := make([]TxSource, n)
	r := getrand(n + 1)
	for i := range ss {
		ss[i] = &MultiTxSource{
			txs:     s.txs,
			weights: s.weights,
			index:   s.index,
			txrate:  s.txrate,
			minSize: s.minSize,
			rand:    r[i+1],
		}
	}
	return ss
}

func (s *MultiTxSource) MinSize() TxSize {
	return s.minSize
}

func (s *MultiTxSource) RateFn() MonotonicFn {
	m := make(map[float64]float64)
	for i, tx := range s.txs {
		m[float64(tx.FeeRate)] += float64(tx.Size) * s.weights[i]
	}
	x := make([]float64, len(m))
	for k := range m {
		x = append(x, float64(k))
	}
	sort.Float64s(x)
	sum := float64(0)
	y := make([]float64, len(x))
	for i := len(x) - 1; i >= 0; i-- {
		sum += m[x[i]] * s.txrate
		y[i] = sum
	}
	return NewTxRateFn(x, y)
}

func (s *MultiTxSource) MarshalJSON() ([]byte, error) {
	feerates := make([]int64, len(s.txs))
	sizes := make([]int64, len(s.txs))
	for i, tx := range s.txs {
		feerates[i] = int64(tx.FeeRate)
		sizes[i] = int64(tx.Size)
	}
	v := make(map[string]interface{})
	v["feerates"] = feerates
	v["sizes"] = sizes
	v["weights"] = s.weights
	v["txrate"] = s.txrate
	v["type"] = "MultiTxSource"
	return json.Marshal(v)
}

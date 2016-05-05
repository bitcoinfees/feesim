package sim

import (
	"encoding/json"
	"math/rand"
	"sort"
	"time"
)

type UniTxSource struct {
	txs     []Tx
	txrate  float64 // txs per second
	minSize TxSize
	rand    *rand.Rand
}

func NewUniTxSource(feerates []FeeRate, sizes []TxSize, txrate float64) *UniTxSource {
	if len(feerates) != len(sizes) {
		panic("feerates and sizes must have same len")
	}
	if len(feerates) == 0 {
		txrate = 0
	}
	txs := make([]Tx, len(sizes))
	minSize := MaxTxSize
	for i, size := range sizes {
		txs[i].FeeRate, txs[i].Size = feerates[i], size
		if size < minSize {
			minSize = size
		}
	}
	return &UniTxSource{
		txs:     txs,
		txrate:  txrate,
		minSize: minSize,
		rand:    getrand(1)[0],
	}
}

func (s *UniTxSource) Generate(t time.Duration) []*Tx {
	l := t.Seconds() * s.txrate    // Expected number of txs
	n := poissonvariate(l, s.rand) // Actual number of txs
	txs := make([]*Tx, n)
	for i := range txs {
		j := s.rand.Intn(len(s.txs))
		txs[i] = &s.txs[j]
	}
	return txs
}

func (s *UniTxSource) Copy(n int) []TxSource {
	ss := make([]TxSource, n)
	r := getrand(n + 1)
	for i := range ss {
		ss[i] = &UniTxSource{
			txs:     s.txs,
			txrate:  s.txrate,
			minSize: s.minSize,
			rand:    r[i+1],
		}
	}
	return ss
}

func (s *UniTxSource) MinSize() TxSize {
	return s.minSize
}

func (s *UniTxSource) RateFn() MonotonicFn {
	m := make(map[float64]float64)
	for _, tx := range s.txs {
		m[float64(tx.FeeRate)] += float64(tx.Size)
	}
	x := make([]float64, 0, len(m))
	for k := range m {
		x = append(x, float64(k))
	}
	sort.Float64s(x)
	sum := float64(0)
	y := make([]float64, len(x))
	for i := len(x) - 1; i >= 0; i-- {
		sum += m[x[i]] * s.txrate / float64(len(s.txs))
		y[i] = sum
	}
	return NewTxRateFn(x, y)
}

func (s *UniTxSource) MarshalJSON() ([]byte, error) {
	feerates := make([]int64, len(s.txs))
	sizes := make([]int64, len(s.txs))
	for i, tx := range s.txs {
		feerates[i] = int64(tx.FeeRate)
		sizes[i] = int64(tx.Size)
	}
	v := make(map[string]interface{})
	v["feerates"] = feerates
	v["sizes"] = sizes
	v["txrate"] = s.txrate
	v["type"] = "UniTxSource"
	return json.Marshal(v)
}

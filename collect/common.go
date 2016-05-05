package collect

import (
	"fmt"
	"sort"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/sim"
)

type Block interface {
	Height() int64
	Size() int64
	Txids() []string
	NumHashes() float64
	Tag() []byte
}

type MempoolEntry interface {
	Size() sim.TxSize
	FeeRate() sim.FeeRate
	Time() int64
	Depends() []string
	IsHighPriority() bool
}

type BlockGetter func(height int64) (Block, error)
type MempoolStateGetter func() (*MempoolState, error)

type TxDB interface {
	Put([]est.Tx) error
}

type BlockStatDB interface {
	Put([]*est.BlockStat) error
}

type MempoolState struct {
	Height     int64                   `json:"height"`
	Entries    map[string]MempoolEntry `json:"entries"`
	Time       int64                   `json:"time"`
	MinFeeRate sim.FeeRate             `json:"minfeerate"`
}

func (s *MempoolState) Copy() *MempoolState {
	entries := make(map[string]MempoolEntry)
	for txid, entry := range s.Entries {
		entries[txid] = entry
	}
	return &MempoolState{
		Height:     s.Height,
		Entries:    entries,
		Time:       s.Time,
		MinFeeRate: s.MinFeeRate,
	}
}

func (s *MempoolState) Sub(t *MempoolState) *MempoolStateDiff {
	entries := make(map[string]MempoolEntry)
	for txid, entry := range s.Entries {
		if _, ok := t.Entries[txid]; !ok {
			entries[txid] = entry
		}
	}
	return &MempoolStateDiff{
		Height:  s.Height - t.Height,
		Entries: entries,
		Time:    s.Time - t.Time,
	}
}

func (s *MempoolState) String() string {
	return fmt.Sprintf("MempoolState{height: %d, entries: %d, minfeerate: %d}",
		s.Height, len(s.Entries), s.MinFeeRate)
}

func (s *MempoolState) SizeFn() sim.MonotonicFn {
	m := make(map[float64]float64)
	for _, entry := range s.Entries {
		m[float64(entry.FeeRate())] += float64(entry.Size())
	}
	x := make([]float64, 0, len(m))
	for k := range m {
		x = append(x, float64(k))
	}
	sort.Float64s(x)
	sum := float64(0)
	y := make([]float64, len(x))
	for i := len(x) - 1; i >= 0; i-- {
		sum += m[x[i]]
		y[i] = sum
	}
	return sim.NewTxRateFn(x, y)
}

type MempoolStateDiff struct {
	Height  int64
	Entries map[string]MempoolEntry
	Time    int64
}

// SimifyMempool converts the mempool data in mempoolState into the form
// expected by Sim.
// We check here that the mempool is closed; i.e. that all parents listed in
// a tx's "depends" field are also in the mempool.
func SimifyMempool(entries map[string]MempoolEntry) ([]*sim.Tx, error) {
	var txids []string
	m := make(map[string]*sim.Tx)
	for txid, entry := range entries {
		txids = append(txids, txid)
		mtx := m[txid]
		if mtx == nil {
			mtx = &sim.Tx{}
		}
		mtx.FeeRate, mtx.Size = entry.FeeRate(), entry.Size()
		m[txid] = mtx
		for _, parent := range entry.Depends() {
			if _, ok := entries[parent]; !ok {
				return nil, fmt.Errorf("mempool not closed")
			}
			ptx := m[parent]
			if ptx == nil {
				ptx = &sim.Tx{}
			}
			mtx.Parents = append(mtx.Parents, ptx)
			m[parent] = ptx
		}
	}
	// Establish an arbitrary canonical ordering for the sim mempool, to make
	// test results deterministic.
	sort.Strings(txids)
	s := make([]*sim.Tx, len(entries))
	for i, txid := range txids {
		s[i] = m[txid]
	}
	return s, nil
}

// Remove mempool entries with a feerate lower than thresh, along with its descendants.
func PruneLowFee(entries map[string]MempoolEntry, thresh sim.FeeRate) {
	// Maps txids to child txids
	childMap := make(map[string][]string)
	for txid, entry := range entries {
		for _, d := range entry.Depends() {
			childMap[d] = append(childMap[d], txid)
		}
	}

	var r []string // the "remove" stack
	for txid, entry := range entries {
		if entry.FeeRate() >= thresh {
			continue
		}
		r = append(r, txid)
		for len(r) > 0 {
			// Pop from r
			newlen := len(r) - 1
			rtxid := r[newlen]
			r = r[:newlen]
			// Push onto r the descendant txids
			for _, dtxid := range childMap[rtxid] {
				r = append(r, dtxid)
			}
			// Remove the popped tx
			delete(entries, rtxid)
			delete(childMap, rtxid)
		}
	}
}

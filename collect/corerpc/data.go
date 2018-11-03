package corerpc

import (
	"github.com/bitcoinfees/feesim/sim"
)

const (
	priorityThresh = 57600000
	coin           = 100000000
)

// There is no longer any concept of priority in Bitcoin Core
// So any mention of priority here is vestigial
type MempoolEntry struct {
	Size_           int64    `json:"size"`
	Time_           int64    `json:"time"`
	Depends_        []string `json:"depends"`
	Fee             float64  `json:"fee"`
	CurrentPriority float64  `json:"currentpriority"`
}

func (m *MempoolEntry) Size() sim.TxSize {
	return sim.TxSize(m.Size_)
}

// Panics if called with a zero-value receiver
func (m *MempoolEntry) FeeRate() sim.FeeRate {
	return sim.FeeRate(m.Fee*coin*1000) / sim.FeeRate(m.Size_)
}

func (m *MempoolEntry) Time() int64 {
	return m.Time_
}

func (m *MempoolEntry) Depends() []string {
	d := make([]string, len(m.Depends_))
	copy(d, m.Depends_)
	return d
}

// Whether or not the tx is "high priority". We don't want to use these txs to
// estimate miner's min fee rate policies.
func (m *MempoolEntry) IsHighPriority() bool {
	// Vestigial since there is no longer any concept of priority in Bitcoin Core
	return false
}

type block struct {
	Height_    int64    `json:"height"`
	Size_      int64    `json:"weight"`
	Txids_     []string `json:"tx"`
	Difficulty float64  `json:"difficulty"`
}

// Height returns the block height.
func (b *block) Height() int64 {
	return b.Height_
}

// Size returns the virtual block size (i.e. weight / 4)
func (b *block) Size() int64 {
	return b.Size_ / 4
}

// Txids returns a slice of the block txids. User is free to do whatever with it,
// as a copy is made each time.
func (b *block) Txids() []string {
	txids := make([]string, len(b.Txids_))
	copy(txids, b.Txids_)
	return txids
}

// Calculate expected number of hashes needed to solve this block
func (b *block) NumHashes() float64 {
	return b.Difficulty * 4295032833.000015
}

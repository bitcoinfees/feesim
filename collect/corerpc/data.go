package corerpc

import (
	"math"

	"github.com/bitcoinfees/feesim/sim"
	"github.com/btcsuite/btcutil"
)

const (
	priorityThresh = 57600000
	coin           = 100000000
)

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
	return m.CurrentPriority > priorityThresh
}

type block struct {
	*btcutil.Block
}

// Wrap btcutil.NewBlockFromBytes
func newBlockFromBytes(blockbytes []byte, height int64) (*block, error) {
	b, err := btcutil.NewBlockFromBytes(blockbytes)
	if err != nil {
		return nil, err
	}
	b.SetHeight(int32(height))
	return &block{b}, nil
}

func (b *block) Height() int64 {
	return int64(b.Block.Height())
}

// Return the block size
func (b *block) Size() int64 {
	return int64(b.MsgBlock().SerializeSize())
}

// txids returns a sorted slice of block txids
func (b *block) Txids() []string {
	txs := b.Transactions()
	txids := make([]string, len(txs))
	for i, tx := range txs {
		txids[i] = tx.Sha().String()
	}
	return txids
}

// Calculate expected number of hashes needed to solve this block
func (b *block) NumHashes() float64 {
	nbits := b.MsgBlock().Header.Bits
	significand := 0xffffff & nbits
	exponent := (nbits>>24 - 3) * 8
	logtarget := math.Log2(float64(significand)) + float64(exponent)

	return math.Pow(2, 256-logtarget)
}

// Get the scriptsig of the coinbase transaction of a block
func (b *block) Tag() []byte {
	tx, err := b.Tx(0)
	if err != nil {
		panic("Shouldn't happen, because the only possible error is index out-of-range.")
	}
	return tx.MsgTx().TxIn[0].SignatureScript
}

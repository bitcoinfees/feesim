/*
Package estimate produces estimates of the models defined in package sim
(TxSource / BlockSource).
*/
package estimate

import (
	"github.com/bitcoinfees/feesim/sim"
)

const diffAdjInterval = 2016 // Difficulty adjustment interval in blocks

// Successive calls have non-decreasing t.
type TxSourceEstimator func(t int64) (sim.TxSource, error)

// Successive calls have non-decreasing h.
type BlockSourceEstimator func(h int64) (sim.BlockSource, error)

type TxDB interface {
	// Get returns all transactions with entry time within [start, end].
	// Txs are to be sorted by increasing time.
	Get(start, end int64) ([]Tx, error)
}

type BlockStatDB interface {
	Get(start, end int64) ([]*BlockStat, error) // Result must be height-sorted
}

type Tx struct {
	FeeRate sim.FeeRate `json:"feerate"`
	Size    sim.TxSize  `json:"size"`
	Time    int64       `json:"time"` // Unix time in seconds
	Type    int64       // Reserved; in the future we might want to model RBF txs
}

type BlockStat struct {
	// Block height
	Height int64 `json:"height"`

	// Block size
	Size int64 `json:"size"`

	// Stranding fee rate stats
	SFRStat SFRStat `json:"sfrstat"`

	// Mempool size just prior to block discovery
	MempoolSize int64 `json:"mempoolsize"`

	// Mempool size just after block discovery
	MempoolSizeRemain int64 `json:"mempoolsizeremain"`

	// Block time (as measured locally; not the block timestamp)
	// Unit is Unix time in seconds.
	Time int64 `json:"time"`

	// Expected number of hashes used to solve this block (function of nBits)
	NumHashes float64 `json:"numhashes"`
}

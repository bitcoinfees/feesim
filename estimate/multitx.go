package estimate

import (
	"fmt"
	"math"

	"github.com/bitcoinfees/feesim/sim"
)

type MultiTxSourceConfig struct {
	// All in seconds
	MinWindow int64 `yaml:"minwindow" json:"minwindow"`
	MaxWindow int64 `yaml:"maxwindow" json:"maxwindow"`
	Halflife  int64 `yaml:"halflife" json:"halflife"`
	MaxTxs    int   `yaml:"maxtxs" json:"maxtxs"`
}

func MultiTxSource(t int64, c *MultiTxSourceConfig, db TxDB) (*sim.MultiTxSource, error) {
	txs, err := db.Get(t-c.MaxWindow, t)
	if err != nil {
		return nil, err
	} else if len(txs) == 0 {
		return nil, TxWindowError{MinWindow: c.MinWindow}
	}

	// Check window size
	earliest := txs[0]
	window := t - earliest.Time
	if window < c.MinWindow {
		return nil, TxWindowError{Window: window, MinWindow: c.MinWindow}
	}

	// Estimate tx rate
	a := math.Pow(0.5, 1/float64(c.Halflife))
	weights := make([]float64, len(txs))
	r := float64(0)
	for i, tx := range txs {
		weights[i] = math.Pow(a, float64(t-tx.Time))
		r += weights[i]
	}
	txrate := r * math.Log(a) / (math.Pow(a, float64(window)) - 1)

	cutoff := len(txs) - c.MaxTxs
	if cutoff < 0 {
		cutoff = 0
	}
	txs = txs[cutoff:]
	weights = weights[cutoff:]

	feerates := make([]sim.FeeRate, len(txs))
	sizes := make([]sim.TxSize, len(txs))

	for i, tx := range txs {
		feerates[i], sizes[i] = tx.FeeRate, tx.Size
	}

	return sim.NewMultiTxSource(feerates, sizes, weights, txrate), nil
}

type TxWindowError struct {
	Window, MinWindow int64
}

func (err TxWindowError) Error() string {
	return fmt.Sprintf("Tx estimation window size was %vs, should be at least %vs",
		err.Window, err.MinWindow)
}

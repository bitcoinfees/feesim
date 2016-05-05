package estimate

import (
	"math"
	"math/rand"

	"github.com/bitcoinfees/feesim/sim"
)

type UniTxSourceConfig struct {
	MinWindow int64 `yaml:"minwindow" json:"minwindow"`
	MaxWindow int64 `yaml:"maxwindow" json:"maxwindow"`
	Halflife  int64 `yaml:"halflife" json:"halflife"`
}

type UniTxSource struct {
	txs      []Tx
	prevTime int64
	window   int64
	a        float64
	r        float64

	db  TxDB
	cfg UniTxSourceConfig
	rng *rand.Rand
}

func NewUniTxSource(db TxDB, cfg UniTxSourceConfig, rng *rand.Rand) *UniTxSource {
	a := math.Pow(0.5, 1/float64(cfg.Halflife))
	return &UniTxSource{
		a:   a,
		db:  db,
		cfg: cfg,
		rng: rng,
	}
}

func (s *UniTxSource) Estimate(currTime int64) (*sim.UniTxSource, error) {

	var (
		txs []Tx
		err error
	)
	if s.prevTime == 0 {
		// This is the first call to Estimate
		if txs, err = s.db.Get(currTime-s.cfg.MaxWindow, currTime); err != nil {
			return nil, err
		}
		if len(txs) == 0 {
			s.prevTime = currTime
		} else {
			s.prevTime = txs[0].Time
		}
	} else if txs, err = s.db.Get(s.prevTime+1, currTime); err != nil {
		return nil, err
	}

	for _, tx := range txs {
		s.addTx(tx)
	}

	if s.window < s.cfg.MinWindow {
		// Not enough data
		return nil, TxWindowError{Window: s.window, MinWindow: s.cfg.MinWindow}
	}

	feerates := make([]sim.FeeRate, len(s.txs))
	sizes := make([]sim.TxSize, len(s.txs))
	for i, tx := range s.txs {
		feerates[i], sizes[i] = tx.FeeRate, tx.Size
	}
	txrate := s.r * math.Log(s.a) / (math.Pow(s.a, float64(s.window)) - 1)
	return sim.NewUniTxSource(feerates, sizes, txrate), nil
}

func (s *UniTxSource) addTx(tx Tx) {
	defer func() { s.prevTime = tx.Time }()
	deltaTime := tx.Time - s.prevTime
	s.window += deltaTime
	p := math.Pow(s.a, float64(deltaTime))
	s.r = s.r*p + 1

	numDiscard := roundRandom((1-p)*float64(len(s.txs)), s.rng)
	for i := 0; i < numDiscard; i++ {
		s.txs = popRandom(s.txs, s.rng)
	}
	s.txs = append(s.txs, tx)
}

// popRandom pops and discards a tx chosen uniformly at random, and returns
// the shortened slice.
func popRandom(txs []Tx, rng *rand.Rand) []Tx {
	newlen := len(txs) - 1
	i := rng.Intn(len(txs))
	txs[i] = txs[newlen]
	return txs[:newlen]
}

// roundRandom rounds f randomly to either int(f) or int(f) + 1, such that
// the expected value is f.
func roundRandom(f float64, rng *rand.Rand) int {
	r := f - math.Floor(f)
	p := rng.Float64()
	if p > r {
		return int(f)
	}
	return int(f) + 1
}

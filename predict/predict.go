// Package predict contains routines for validating the sim models, by
// predicting the confirmation times of transactions and comparing with the
// observed confirmation times.
package predict

import (
	"log"
	"math"
	"os"
	"sort"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/sim"
)

type Tx struct {
	ConfirmIn int64
	ConfirmBy int64
}

type DB interface {
	// The returned map must only contain those txids which were previously Put.
	GetTxs(txids []string) (map[string]Tx, error)

	PutTxs(txs map[string]Tx) error

	GetScores() (attained, exceeded []float64, err error)
	PutScores(attained, exceeded []float64) error

	Reconcile(txids []string) error
	Close() error
}

type Config struct {
	MaxBlockConfirms int `yaml:"maxblockconfirms" json:"maxblockconfirms"`
	Halflife         int `yaml:"halflife" json:"halflife"` // In number of blocks

	Logger *log.Logger `yaml:"-" json:"-"`
}

// TODO: Consider tallying predicts once the block heights exceeds confirmBy
type Predictor struct {
	db    DB
	cfg   Config
	a     float64
	state *col.MempoolState
}

func NewPredictor(db DB, cfg Config) (*Predictor, error) {
	// Resize the Scores
	attained, exceeded, err := db.GetScores()
	if err != nil {
		return nil, err
	}
	if d := cfg.MaxBlockConfirms - len(attained); d > 0 {
		attained = append(attained, make([]float64, d)...)
	} else {
		attained = attained[:cfg.MaxBlockConfirms]
	}
	if d := cfg.MaxBlockConfirms - len(exceeded); d > 0 {
		exceeded = append(exceeded, make([]float64, d)...)
	} else {
		exceeded = exceeded[:cfg.MaxBlockConfirms]
	}
	if err := db.PutScores(attained, exceeded); err != nil {
		return nil, err
	}

	a := math.Pow(0.5, 1/float64(cfg.Halflife))
	p := &Predictor{
		db:  db,
		cfg: cfg,
		a:   a,
	}
	return p, nil
}

func (p *Predictor) ProcessBlock(b col.Block) error {
	logger := p.cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	attained := make([]float64, p.cfg.MaxBlockConfirms)
	exceeded := make([]float64, p.cfg.MaxBlockConfirms)
	height, txids := b.Height(), b.Txids()
	predictTxs, err := p.db.GetTxs(txids)
	if err != nil {
		return err
	}
	for _, tx := range predictTxs {
		if height <= tx.ConfirmBy {
			attained[tx.ConfirmIn-1]++
		} else {
			exceeded[tx.ConfirmIn-1]++
		}
	}
	logger.Printf("[DEBUG] Predictor: %d predicts tallied.", len(predictTxs))

	attainedTotal, exceededTotal, err := p.db.GetScores()
	if err != nil {
		return err
	}

	// Exponential decay
	for i := range attained {
		attainedTotal[i] = p.a*attainedTotal[i] + attained[i]
		exceededTotal[i] = p.a*exceededTotal[i] + exceeded[i]
	}
	return p.db.PutScores(attainedTotal, exceededTotal)
}

func (p *Predictor) AddPredicts(s *col.MempoolState, simResult []sim.FeeRate) error {
	defer func() { p.state = s }()
	if p.state == nil {
		return nil
	}

	logger := p.cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	d := s.Sub(p.state)
	predictTxs := make(map[string]Tx)
	for txid, entry := range d.Entries {
		if len(entry.Depends()) > 0 || entry.IsHighPriority() {
			// Don't predict for high priority txs or for txs with mempool
			// dependencies. Priority inclusion is getting deprecated in
			// Bitcoin Core, though..
			continue
		}
		confirmIn := searchResult(simResult, entry.FeeRate()) + 1
		if confirmIn > len(simResult) || confirmIn > p.cfg.MaxBlockConfirms {
			continue
		}
		confirmBy := s.Height + int64(confirmIn)
		predictTxs[txid] = Tx{ConfirmIn: int64(confirmIn), ConfirmBy: confirmBy}
	}
	logger.Printf("[DEBUG] Predictor: %d predicts added.", len(predictTxs))
	return p.db.PutTxs(predictTxs)
}

func (p *Predictor) Cleanup(s *col.MempoolState) error {
	txids := make([]string, 0, len(s.Entries))
	for txid, _ := range s.Entries {
		txids = append(txids, txid)
	}
	return p.db.Reconcile(txids)
}

func (p *Predictor) GetScores() (attained []float64, exceeded []float64, err error) {
	return p.db.GetScores()
}

// searchResult returns the smallest i such that x >= result[i]
func searchResult(result []sim.FeeRate, x sim.FeeRate) int {
	return sort.Search(len(result), func(i int) bool {
		if x >= result[i] && result[i] != -1 {
			return true
		}
		return false
	})
}

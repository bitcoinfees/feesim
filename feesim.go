package main

import (
	"errors"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"

	col "github.com/bitcoinfees/feesim/collect"
	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/predict"
	"github.com/bitcoinfees/feesim/sim"
)

var errPause = errors.New("sim is paused")
var errInProgress = errors.New("sim is in progress")
var errShutdown = errors.New("sim is shutting down")

type TxDB interface {
	est.TxDB
	col.TxDB
	Delete(start, end int64) error
	Close() error
}

type BlockStatDB interface {
	est.BlockStatDB
	col.BlockStatDB
	Delete(start, end int64) error
	Close() error
}

type FeeSim struct {
	result      []sim.FeeRate
	txsource    sim.TxSource
	blocksource sim.BlockSource

	err            error
	errTxSource    error
	errBlockSource error

	collect   *col.Collector
	predictor *predict.Predictor
	txdb      TxDB
	blkdb     BlockStatDB
	predictdb predict.DB
	cfg       FeeSimConfig

	pause chan bool
	done  chan struct{}
	wg    sync.WaitGroup
	mux   sync.RWMutex
}

type FeeSimConfig struct {
	Collect   col.Config          `yaml:"collect" json:"collect"`
	Transient sim.TransientConfig `yaml:"transient" json:"transient"`
	Predict   predict.Config      `yaml:"predict" json:"predict"`
	SimPeriod int                 `yaml:"simperiod" json:"simperiod"`
	TxMaxAge  int64               `yaml:"txmaxage" json:"txmaxage"`
	TxGapTol  int64               `yaml:"txgaptol" json:"txgaptol"`

	estTxSource    est.TxSourceEstimator    `yaml:"-" json:"-"`
	estBlockSource est.BlockSourceEstimator `yaml:"-" json:"-"`
	logger         *log.Logger              `yaml:"-" json:"-"`
}

func NewFeeSim(txdb TxDB, blkdb BlockStatDB, predictdb predict.DB, cfg FeeSimConfig) (*FeeSim, error) {
	cfg.Collect.Logger = cfg.logger
	collect := col.NewCollector(txdb, blkdb, cfg.Collect)

	cfg.Predict.Logger = cfg.logger
	predictor, err := predict.NewPredictor(predictdb, cfg.Predict)
	if err != nil {
		return nil, err
	}

	feesim := &FeeSim{
		collect:   collect,
		predictor: predictor,
		txdb:      txdb,
		blkdb:     blkdb,
		predictdb: predictdb,
		cfg:       cfg,
		pause:     make(chan bool),
		done:      make(chan struct{}),
	}
	return feesim, nil
}

func (s *FeeSim) Run() error {
	logger := s.cfg.logger
	s.wg.Add(1)
	defer logger.Println("Feesim all stopped.")
	defer s.wg.Wait()
	defer s.wg.Done()
	defer s.predictdb.Close()
	defer s.blkdb.Close()
	defer s.txdb.Close()

	logger.Printf("Feesim v%s starting up..", version)
	state, err := s.cfg.Collect.GetState()
	if err != nil {
		return err
	}
	timeNow := state.Time
	heightNow := state.Height

	if err := s.normalizeTxDB(timeNow); err != nil {
		return err
	}
	if err := s.predictor.Cleanup(state); err != nil {
		return err
	}

	if err := s.collect.Run(); err != nil {
		return err
	}
	defer s.collect.Stop()

	// Initial source estimation
	txsource, err := s.cfg.estTxSource(timeNow)
	s.SetTxSource(txsource, err)
	blocksource, err := s.cfg.estBlockSource(heightNow)
	s.SetBlockSource(blocksource, err)

	// Initial result
	s.SetResult(nil, errInProgress)

	s.wg.Add(1)
	go s.loopSim(s.cfg.SimPeriod)

	sc := make(chan *col.MempoolState, 10)
	bc := make(chan []col.Block, 10)
	s.wg.Add(1)
	go s.predictWorker(sc, bc)

	tc := make(chan int64)
	s.wg.Add(1)
	go s.estTxSourceWorker(tc)

	hc := make(chan int64, 10)
	s.wg.Add(1)
	go s.estBlockSourceWorker(hc)

	logger.Println("Feesim startup complete.")
	for {
		select {
		case state := <-s.collect.S:
			// Add predicts
			select {
			case sc <- state:
			default:
				logger.Println("[WARNING] Predictor (state) was busy.")
			}
			// Update the txsource
			select {
			case tc <- state.Time:
			default:
				logger.Println("[WARNING] TxSource estimator was busy.")
			}
		case blocks := <-s.collect.B:
			// Process predicts
			select {
			case bc <- blocks:
			default:
				logger.Println("[WARNING] Predictor (blocks) was busy.")
			}
			// Update the blocksource, if the worker is available.
			select {
			case hc <- blocks[len(blocks)-1].Height():
			default:
				logger.Println("[WARNING] BlockSource estimator was busy.")
			}
		case err := <-s.collect.E:
			// Error in collector
			logger.Println("[ERROR] Collector:", err)
		case <-s.done:
			// Terminate
			return nil
		}
	}
}

func (s *FeeSim) Status() map[string]string {
	status := make(map[string]string)

	if _, err := s.TxSource(); err != nil {
		status["txsource"] = err.Error()
	} else {
		status["txsource"] = "OK"
	}

	if _, err := s.BlockSource(); err != nil {
		status["blocksource"] = err.Error()
	} else {
		status["blocksource"] = "OK"
	}

	if _, err := s.Result(); err != nil {
		status["result"] = err.Error()
	} else {
		status["result"] = "OK"
	}

	if state := s.State(); state == nil {
		status["mempool"] = "Mempool state not available."
	} else {
		status["mempool"] = "OK"
	}

	return status
}

func (s *FeeSim) Pause(p bool) {
	s.pause <- p
	if p {
		s.cfg.logger.Println("Sim paused.")
	} else {
		s.cfg.logger.Println("Sim unpaused.")
	}
}

func (s *FeeSim) Stop() {
	s.closeDone()
	s.wg.Wait()
}

func (s *FeeSim) State() *col.MempoolState {
	return s.collect.State()
}

// closeDone closes s.done in a concurrent-safe way.
func (s *FeeSim) closeDone() {
	s.mux.Lock()
	defer s.mux.Unlock()
	select {
	case <-s.done: // Already closed
	default:
		close(s.done)
	}
}

func (s *FeeSim) predictWorker(sc <-chan *col.MempoolState, bc <-chan []col.Block) {
	logger := s.cfg.logger
	defer s.wg.Done()
	defer logger.Println("Predict worker stopped.")

	for {
		select {
		case state := <-sc:
			// state is never nil here.
			result, err := s.Result()
			if err != nil {
				continue
			}
			if err := s.predictor.AddPredicts(state, result); err != nil {
				logger.Println("[ERROR] AddPredicts:", err)
			}
		case blocks := <-bc:
			for _, b := range blocks {
				if err := s.predictor.ProcessBlock(b); err != nil {
					logger.Println("[ERROR] Predictor ProcessBlock:", err)
				}
			}
			if state := s.collect.State(); state != nil {
				if err := s.predictor.Cleanup(state); err != nil {
					logger.Println("[ERROR] Predictor Cleanup:", err)
				}
			}
		case <-s.done:
			return
		}
	}
}

func (s *FeeSim) estTxSourceWorker(tc <-chan int64) {
	logger := s.cfg.logger
	defer s.wg.Done()
	defer logger.Println("Tx source worker stopped.")

	var t int64
	for {
		select {
		case t = <-tc:
		case <-s.done:
			return
		}

		txsource, err := s.cfg.estTxSource(t)
		// Log error if it's not TxWindowError
		if _, isWindowErr := err.(est.TxWindowError); err != nil && !isWindowErr {
			logger.Println("[ERROR] estTxSource::", err)
		}

		logger.Println("[DEBUG] TxSource estimate updated.")
		s.SetTxSource(txsource, err)

		// Delete old txs
		if err := s.txdb.Delete(0, t-s.cfg.TxMaxAge); err != nil {
			logger.Println("[ERROR] TxDB delete:", err)
		}
	}
}

func (s *FeeSim) estBlockSourceWorker(hc <-chan int64) {
	logger := s.cfg.logger
	defer s.wg.Done()
	defer logger.Println("Block source worker stopped.")

	var height int64
	for {
		select {
		case height = <-hc:
		case <-s.done:
			return
		}

		blocksource, err := s.cfg.estBlockSource(height)
		// Log error if it's not BlockCoverageError
		if _, isCovErr := err.(est.BlockCoverageError); err != nil && !isCovErr {
			logger.Println("[ERROR] estBlockSourceWorker:", err)
		}

		logger.Printf("[DEBUG] Block %d: BlockSource estimate updated.", height)
		s.SetBlockSource(blocksource, err)
	}
}

func (s *FeeSim) loopSim(period int) {
	logger := s.cfg.logger
	defer s.wg.Done()
	defer logger.Println("Sim loop stopped.")
	ticker := time.NewTicker(time.Duration(period) * time.Second)
	defer func() { ticker.Stop() }() // Stop is idempotent, so no problems here

	// Metrics
	names := []string{"sim1", "sim60", "sim1440"}
	sizes := []int{1, 60, 1440}
	simTimers := make([]metrics.Timer, 3)
	for i, size := range sizes {
		h := metrics.NewHistogram(metrics.NewSimpleExpDecaySample(size))
		simTimers[i] = metrics.NewCustomTimer(h, metrics.NewMeter())
		metrics.Register(names[i], simTimers[i])
	}

	for {
		ts, err := s.setupSim()
		if err != nil {
			s.SetResult(nil, err)
		} else {
			if _, err := s.Result(); err != nil {
				s.SetResult(nil, errInProgress)
			}
			logger.Println("[DEBUG] Transient sim started.")
			startTime := time.Now()
			r := ts.Run()

		ResultLoop:
			select {
			case result := <-r:
				logger.Println("[DEBUG] Transient sim complete.")
				for _, m := range simTimers {
					m.UpdateSince(startTime)
				}
				s.SetResult(result, nil)
			case p := <-s.pause:
				if !p {
					goto ResultLoop // No change
				}
				ticker.Stop()
				ts.Stop()
				s.SetResult(nil, errPause)
			case <-s.done:
				ts.Stop()
				s.SetResult(nil, errShutdown)
				return
			}
		}

	WaitLoop:
		select {
		case <-ticker.C:
		case p := <-s.pause:
			if p {
				// Pause
				ticker.Stop()
				s.SetResult(nil, errPause)
				goto WaitLoop
			} else if !s.IsPaused() {
				// Not paused, so no change; wait for ticker
				goto WaitLoop
			}
			// Is paused, so restart the ticker and resume
			ticker = time.NewTicker(time.Duration(period) * time.Second)
			s.SetResult(nil, errInProgress)
		case <-s.done:
			s.SetResult(nil, errShutdown)
			return
		}
	}
}

func (s *FeeSim) setupSim() (*sim.TransientSim, error) {
	logger := s.cfg.logger

	state := s.collect.State()
	if state == nil {
		return nil, errors.New("mempool state not available")
	}
	txsource, err := s.TxSource()
	if err != nil {
		return nil, err
	}
	blocksource, err := s.BlockSource()
	if err != nil {
		return nil, err
	}

	// Trim the mempool to optimize sim time. The idea is that since we're only
	// simulating up to a certain MaxBlockConfirms, we can safely ignore many
	// low fee transactions.
	maxBlockConfirms := s.cfg.Transient.MaxBlockConfirms
	txratefn, capratefn, sizefn := txsource.RateFn(), blocksource.RateFn(), state.SizeFn()

	maxcap := capratefn.Eval(math.MaxFloat64) // Maximum capacity byte rate

	highfee := sizefn.Inverse(0)
	var n int
	if highfee > float64(math.MaxInt32) {
		n = math.MaxInt32
	} else {
		n = int(highfee)
	}

	cutoff := sim.FeeRate(sort.Search(n, func(i int) bool {
		d := sizefn.Eval(float64(i)) / (maxcap - txratefn.Eval(float64(i))) * blocksource.BlockRate()
		const buffer = 3
		return d < buffer*float64(maxBlockConfirms) && d >= 0
	}))

	initmempool, err := col.SimifyMempool(state.Entries)
	if err != nil {
		logger.Println("[ERROR] SimifyMempool:", err)
		return nil, err
	}

	// Remove all transactions with fee rate less than cutoff. We remove all the
	// parents as well, although it's not strictly necessary since it's done in
	// sim.NewSim as well (the model does not consider mempool deps anymore
	// CPFP, see sim.NewSim). However, we do it for neatness' sake, to avoid
	// dangling deps.
	var initmempoolTrimmed []*sim.Tx
	for _, tx := range initmempool {
		if tx.FeeRate >= cutoff {
			tx.Parents = tx.Parents[:0]
			initmempoolTrimmed = append(initmempoolTrimmed, tx)
		}
	}

	ns := sim.NewSim(txsource, blocksource, initmempoolTrimmed)
	transientCfg := s.cfg.Transient
	transientCfg.LowestFeeRate = cutoff
	logger.Println("[DEBUG] Transient sim stablefeerate:", ns.StableFee())
	logger.Println("[DEBUG] Transient sim lowfee:", transientCfg.LowestFeeRate)

	return sim.NewTransientSim(ns, transientCfg), nil
}

func (s *FeeSim) IsPaused() bool {
	_, err := s.Result()
	if err == errPause {
		return true
	}
	return false
}

func (s *FeeSim) Result() ([]sim.FeeRate, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.result, s.err
}

func (s *FeeSim) SetResult(result []sim.FeeRate, err error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.result, s.err = result, err
}

func (s *FeeSim) PredictScores() (attained, exceeded []float64, err error) {
	return s.predictor.GetScores()
}

func (s *FeeSim) BlockSource() (sim.BlockSource, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.blocksource, s.errBlockSource
}

func (s *FeeSim) SetBlockSource(b sim.BlockSource, err error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.blocksource, s.errBlockSource = b, err
}

func (s *FeeSim) TxSource() (sim.TxSource, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.txsource, s.errTxSource
}

func (s *FeeSim) SetTxSource(t sim.TxSource, err error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.txsource, s.errTxSource = t, err
}

func (s *FeeSim) normalizeTxDB(timeNow int64) error {
	txs, err := s.txdb.Get(timeNow-s.cfg.TxMaxAge, timeNow)
	if err != nil {
		return err
	}
	if err := s.txdb.Delete(0, math.MaxInt64); err != nil {
		return err
	}
	if len(txs) == 0 || txs[len(txs)-1].Time < timeNow-s.cfg.TxGapTol {
		s.cfg.logger.Println("TxDB outdated / empty; starting from scratch.")
		return nil
	}
	s.cfg.logger.Println("Normalizing TxDB.")
	d := timeNow - txs[len(txs)-1].Time
	for i := range txs {
		txs[i].Time += d
	}
	return s.txdb.Put(txs)
}

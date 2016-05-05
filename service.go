package main

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/rpc"
	jsonrpc "github.com/gorilla/rpc/json"
	"github.com/rcrowley/go-metrics"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/sim"
)

type Service struct {
	FeeSim *FeeSim
	DLog   *DebugLog
	Cfg    config
}

func (s *Service) ListenAndServe() error {
	var methods = map[string]string{
		"stop":          "Service.Stop",
		"status":        "Service.Status",
		"estimatefee":   "Service.EstimateFee",
		"predictscores": "Service.PredictScores",
		"txrate":        "Service.TxRate",
		"caprate":       "Service.CapRate",
		"mempoolsize":   "Service.MempoolSize",
		"pause":         "Service.Pause",
		"unpause":       "Service.Unpause",
		"setdebug":      "Service.SetDebug",
		"config":        "Service.Config",
		"metrics":       "Service.Metrics",
		"blocksource":   "Service.BlockSource",
		"txsource":      "Service.TxSource",
		"mempoolstate":  "Service.MempoolState",
	}
	srv := rpc.NewServer()
	srv.RegisterCodec(jsonrpc.NewCodec(), "application/json")
	srv.RegisterService(s, "")
	srv.RegisterCustomNames(methods)
	http.Handle("/", srv)
	addr := net.JoinHostPort(s.Cfg.AppRPC.Host, s.Cfg.AppRPC.Port)
	s.DLog.Logger.Println("RPC server listening on", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Service) Stop(r *http.Request, args *struct{}, reply *struct{}) error {
	go s.FeeSim.Stop()
	return nil
}

func (s *Service) Status(r *http.Request, args *struct{}, reply *map[string]string) error {
	*reply = s.FeeSim.Status()
	return nil
}

// NOTE: There's no fail-safe max value, take care.
func (s *Service) EstimateFee(r *http.Request, args *int, reply *interface{}) error {
	result, err := s.FeeSim.Result()
	if err != nil {
		return err
	}

	if *args < 0 {
		return fmt.Errorf("argument must be >= 0")
	}
	if *args > len(result) {
		return fmt.Errorf("MaxBlockConfirms=%d exceeded", len(result))
	}

	// Convert from satoshis to BTC, to conform to Bitcoin Core's estimatefee API
	resultBTC := make([]float64, len(result))
	for i, satoshis := range result {
		if satoshis == -1 {
			resultBTC[i] = -1
		} else {
			resultBTC[i] = float64(satoshis) / coin
		}
	}

	if *args == 0 {
		*reply = resultBTC
	} else {
		*reply = resultBTC[*args-1]
	}
	return nil
}

func (s *Service) PredictScores(r *http.Request, args *struct{}, reply *map[string][]float64) error {
	attained, exceeded, err := s.FeeSim.PredictScores()
	if err != nil {
		return err
	}
	scores := make(map[string][]float64)
	scores["attained"] = attained
	scores["exceeded"] = exceeded
	*reply = scores
	return nil
}

func (s *Service) TxRate(r *http.Request, args *int, reply *sim.MonotonicFn) error {
	n := *args
	if n <= 0 {
		n = 20
	}
	txsource, err := s.FeeSim.TxSource()
	if err != nil {
		return err
	}
	*reply = txsource.RateFn().Approx(n)
	return nil
}

func (s *Service) CapRate(r *http.Request, args *int, reply *sim.MonotonicFn) error {
	n := *args
	if n <= 0 {
		n = 20
	}
	blocksource, err := s.FeeSim.BlockSource()
	if err != nil {
		return err
	}
	*reply = blocksource.RateFn().Approx(n)
	return nil
}

func (s *Service) MempoolSize(r *http.Request, args *int, reply *sim.MonotonicFn) error {
	n := *args
	if n <= 0 {
		n = 20
	}
	state := s.FeeSim.State()
	if state == nil {
		return fmt.Errorf("mempool not available")
	}
	*reply = state.SizeFn().Approx(n)
	return nil
}

func (s *Service) Pause(r *http.Request, args *struct{}, reply *struct{}) error {
	s.FeeSim.Pause(true)
	return nil
}

func (s *Service) Unpause(r *http.Request, args *struct{}, reply *struct{}) error {
	s.FeeSim.Pause(false)
	return nil
}

func (s *Service) SetDebug(r *http.Request, args *bool, reply *bool) error {
	s.DLog.SetDebug(*args)
	*reply = *args
	return nil
}

func (s *Service) Config(r *http.Request, args *struct{}, reply *interface{}) error {
	c := s.Cfg
	// Hide the password just in case
	c.BitcoinRPC.Password = "********"
	*reply = c
	return nil
}

func (s *Service) Metrics(r *http.Request, args *struct{}, reply *metrics.Registry) error {
	*reply = metrics.DefaultRegistry
	return nil
}

func (s *Service) BlockSource(r *http.Request, args *struct{}, reply *sim.BlockSource) error {
	blocksource, err := s.FeeSim.BlockSource()
	if err != nil {
		return err
	}
	*reply = blocksource
	return nil
}

func (s *Service) TxSource(r *http.Request, args *struct{}, reply *sim.TxSource) error {
	txsource, err := s.FeeSim.TxSource()
	if err != nil {
		return err
	}
	*reply = txsource
	return nil
}

func (s *Service) MempoolState(r *http.Request, args *struct{}, reply **col.MempoolState) error {
	state := s.FeeSim.State()
	if state == nil {
		return fmt.Errorf("mempool not available")
	}
	*reply = state
	return nil
}

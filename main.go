package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/rcrowley/go-metrics"

	"github.com/bitcoinfees/feesim/api"
	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/collect/corerpc"
	"github.com/bitcoinfees/feesim/db/bolt"
	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/predict"
	"github.com/bitcoinfees/feesim/sim"
)

const usage = `
feesim [-c CONFIGFILE] [-d DATADIR] COMMAND [-h | -help] [args...]

Commands:
	start       (start the sim app)
	stop        (terminate the app)
	version     (show app version)
	status      (show application status)
	estimatefee (estimated feerate (BTC/kB) for confirmation in N blocks)
	scores      (show prediction scores)
	txrate      (show tx byterate)
	caprate     (show capacity byterate)
	mempoolsize (show mempool size)
	pause       (pause the sim)
	unpause     (resume the sim after pausing)
	setdebug    (turn on/off debug-level logging)
	metrics     (show app metrics)
	config      (show app config settings.)

`

const (
	coin    = 100000000
	version = "0.3.1"
)

func main() {
	var (
		configFile, dataDir string
	)
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		flag.CommandLine.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	flag.StringVar(&configFile, "c", "",
		fmt.Sprintf("Path to config file (alternatively, use %s env var).", configFileEnv))
	flag.StringVar(&dataDir, "d", "",
		fmt.Sprintf("Path to data directory (alternatively, use %s env var).", dataDirEnv))
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.CommandLine.Usage()
		os.Exit(1)
	}

	cfg, err := loadConfig(configFile, dataDir)
	if err != nil {
		log.Fatal(err)
	}

	apiclient := api.NewClient(api.Config{
		Host:    cfg.AppRPC.Host,
		Port:    cfg.AppRPC.Port,
		Timeout: 15,
	})

	switch args[0] {
	case "start":
		runFeeSim(args, cfg)
	case "version":
		fmt.Println(version)
	case "stop":
		stop(args, apiclient)
	case "status":
		status(args, apiclient)
	case "estimatefee":
		estimateFee(args, apiclient)
	case "scores":
		scores(args, apiclient)
	case "txrate":
		txRate(args, apiclient)
	case "caprate":
		capRate(args, apiclient)
	case "mempoolsize":
		mempoolSize(args, apiclient)
	case "pause":
		pause(args, apiclient)
	case "unpause":
		unpause(args, apiclient)
	case "setdebug":
		setDebug(args, apiclient)
	case "metrics":
		appMetrics(args, apiclient)
	case "config":
		appConfig(args, apiclient)
	default:
		log.Fatalf("Invalid command '%s'", args[0])
	}
}

func runFeeSim(args []string, cfg config) {
	const usage = `
feesim start

Start the program. The program will begin collecting mempool statistics through
getrawmempool polling, and will begin fee estimation once there is sufficient
data.

Use feesim status to check the data collection / sim status. Use feesim pause
to pause the sim (while still collecting data).
`
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, usage)
		f.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
	if err := f.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}
	txdb, err := loadTxDB(cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadTxDB: %v", err))
	}

	blkdb, err := loadBlockStatDB(cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadBlockStatDB: %v", err))
	}

	predictdb, err := loadPredictDB(cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadPredictDB: %v", err))
	}

	estTx, err := loadTxSourceEstimator(txdb, cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadTxSourceEstimator: %v", err))
	}

	estBlk, err := loadBlockSourceEstimator(blkdb, cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadBlockSourceEstimator: %v", err))
	}

	collectConfig, err := loadCollectorConfig(cfg)
	if err != nil {
		log.Fatal(fmt.Errorf("loadCollectorConfig: %v", err))
	}

	// Setup the logger
	var dLog *DebugLog
	logFileMode := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if f, err := os.OpenFile(cfg.LogFile, logFileMode, 0666); err != nil {
		log.Fatal(fmt.Errorf("opening logfile: %v", err))
	} else {
		dLog = NewDebugLog(f, "", log.LstdFlags)
	}

	feesimConfig := FeeSimConfig{
		estTxSource:    estTx,
		estBlockSource: estBlk,
		Collect:        collectConfig,
		Transient:      cfg.Transient,
		Predict:        cfg.Predict,
		SimPeriod:      cfg.SimPeriod,
		TxMaxAge:       cfg.TxMaxAge,
		TxGapTol:       cfg.TxGapTol,
		logger:         dLog.Logger,
	}
	feesim, err := NewFeeSim(txdb, blkdb, predictdb, feesimConfig)
	if err != nil {
		log.Fatal(fmt.Errorf("NewFeeSim: %v", err))
	}
	service := &Service{FeeSim: feesim, DLog: dLog, Cfg: cfg}

	os.Stdout.Close()
	os.Stderr.Close()
	os.Stdin.Close()

	errc := make(chan error)
	go func() { errc <- feesim.Run() }()
	go func() { errc <- service.ListenAndServe() }()

	// Signal handling
	sigc := make(chan os.Signal, 3)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigc
		feesim.Stop()
	}()

	err = <-errc
	// Blocks until it is safely shutdown. It is idempotent, so no harm if
	// feesim is already stopped.
	feesim.Stop()
	if err != nil {
		dLog.Logger.Fatal(err)
	}
}

func loadTxSourceEstimator(db est.TxDB, cfg config) (est.TxSourceEstimator, error) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	estimator := est.NewUniTxSource(db, cfg.UniTx, rng)
	estTx := func(t int64) (sim.TxSource, error) {
		return estimator.Estimate(t)
	}
	return estTx, nil
}

func loadBlockSourceEstimator(db est.BlockStatDB, cfg config) (est.BlockSourceEstimator, error) {
	estBlk := func(h int64) (sim.BlockSource, error) {
		return est.IndBlockSourceSMFR(h, cfg.IndBlock, db)
	}
	return estBlk, nil
}

func loadTxDB(cfg config) (TxDB, error) {
	const dbFileName = "tx.db"
	dbfile := filepath.Join(cfg.DataDir, dbFileName)
	return bolt.LoadTxDB(dbfile)
}

func loadBlockStatDB(cfg config) (BlockStatDB, error) {
	const dbFileName = "blockstat.db"
	dbfile := filepath.Join(cfg.DataDir, dbFileName)
	return bolt.LoadBlockStatDB(dbfile)
}

func loadPredictDB(cfg config) (predict.DB, error) {
	const dbFileName = "predict.db"
	dbfile := filepath.Join(cfg.DataDir, dbFileName)
	return bolt.LoadPredictDB(dbfile)
}

func loadCollectorConfig(cfg config) (col.Config, error) {
	timeNow := func() int64 {
		return time.Now().Unix()
	}
	getState, getBlock, err := corerpc.Getters(timeNow, cfg.BitcoinRPC)
	if err != nil {
		return col.Config{}, err
	}

	// Wrap getState with a timer
	reservoirSize := 60 / cfg.Collect.PollPeriod * 60 * 24 // About one day's worth
	getStateTimer := metrics.NewCustomTimer(metrics.NewHistogram(
		metrics.NewSimpleExpDecaySample(reservoirSize)), metrics.NewMeter())
	timedGetState := func() (*col.MempoolState, error) {
		start := time.Now()
		defer getStateTimer.UpdateSince(start)
		return getState()
	}
	name := "getstate" + strconv.Itoa(reservoirSize)
	metrics.Register(name, getStateTimer)

	c := col.Config{
		GetState:   timedGetState,
		GetBlock:   getBlock,
		PollPeriod: cfg.Collect.PollPeriod,
	}
	return c, nil
}

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/collect/corerpc"
	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/predict"
	"github.com/bitcoinfees/feesim/sim"
)

const (
	defaultConfigFileName = "config.yml"
	configFileEnv         = "FEESIM_CONFIG"
	dataDirEnv            = "FEESIM_DATADIR"
)

var (
	defaultFeeSimConfig = FeeSimConfig{
		Collect: col.Config{
			PollPeriod: 10,
		},
		Transient: sim.TransientConfig{
			MaxBlockConfirms: 12,
			MinSuccessPct:    0.9,
			NumIters:         10000,
		},
		Predict: predict.Config{
			MaxBlockConfirms: 6,
			Halflife:         1008, // 1 week
		},
		SimPeriod: 60,
		TxMaxAge:  10800, // 3 hours
		TxGapTol:  3600,  // 1 hour
	}
	defaultConfig = config{
		FeeSimConfig: defaultFeeSimConfig,
		UniTx: est.UniTxSourceConfig{
			MinWindow: 600,   // 10 minutes
			MaxWindow: 10800, // 3 hours
			Halflife:  3600,  // 1 hour
		},
		IndBlock: est.IndBlockSourceConfig{
			Window:        2016,
			MinCov:        0.5,
			GuardInterval: 300,
			TailPct:       0.1,
		},
		BitcoinRPC: corerpc.Config{
			Host:    "localhost",
			Port:    "8332",
			Timeout: 30,
		},
		AppRPC: AppRPCConfig{
			Host: "localhost",
			Port: "8350",
		},
		DataDir: AppDataDir("feesim", false),
	}
	defaultConfigFile  = filepath.Join(defaultConfig.DataDir, defaultConfigFileName)
	defaultLogFileName = "feesim.log"
)

type config struct {
	FeeSimConfig `yaml:",inline"`
	UniTx        est.UniTxSourceConfig    `yaml:"unitx" json:"unitx"`
	IndBlock     est.IndBlockSourceConfig `yaml:"indblock" json:"indblock"`
	BitcoinRPC   corerpc.Config           `yaml:"bitcoinrpc" json:"bitcoinrpc"`
	AppRPC       AppRPCConfig             `yaml:"apprpc" json:"apprpc"`
	DataDir      string                   `yaml:"datadir" json:"datadir"`
	LogFile      string                   `yaml:"logfile" json:"logfile"`
}

type AppRPCConfig struct {
	Host string `json:"host" yaml:"host"`
	Port string `json:"port" yaml:"port"`
}

// loadConfig loads the config. The input arguments specify the path to the
// config file / data directory.
// They can also be specified through env variables (configFileEnv / dataDirEnv),
// with lower precedence.
// If not specified, they are set to default values.
func loadConfig(configFile, dataDir string) (config, error) {
	cfg := defaultConfig

	if configFile == "" {
		configFile = os.Getenv(configFileEnv)
	}
	if dataDir == "" {
		dataDir = os.Getenv(dataDirEnv)
	}

	if configFile != "" {
		// Config file was specified explicitly, so return an error if it
		// couldn't be read.
		if c, err := ioutil.ReadFile(configFile); err != nil {
			return cfg, err
		} else if err := yaml.Unmarshal(c, &cfg); err != nil {
			return cfg, err
		}
	} else {
		// Check the default config file location. No error if it couldn't be
		// read, but error if the yaml could not be unmarshaled.
		if dataDir == "" {
			configFile = defaultConfigFile
		} else {
			configFile = filepath.Join(dataDir, defaultConfigFileName)
		}
		if c, err := ioutil.ReadFile(configFile); err == nil {
			if err := yaml.Unmarshal(c, &cfg); err != nil {
				return cfg, err
			}
		}
	}

	// dataDir specified by env or input argument takes precedence
	if dataDir != "" {
		cfg.DataDir = dataDir
	}

	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join(cfg.DataDir, defaultLogFileName)
	}

	// Create the datadir if not exists
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return cfg, err
	}

	return cfg, nil
}

/*
Package collect contains routines for collecting data from the Bitcoin network,
which is then used by package estimate.
*/
package collect

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	est "github.com/bitcoinfees/feesim/estimate"
)

type Config struct {
	PollPeriod int `yaml:"pollperiod" json:"pollperiod"`

	GetState MempoolStateGetter `yaml:"-" json:"-"`
	GetBlock BlockGetter        `yaml:"-" json:"-"`
	Logger   *log.Logger        `yaml:"-" json:"-"`
}

// NOTE: S,B,E channels must be serviced.
type Collector struct {
	S <-chan *MempoolState
	B <-chan []Block
	E <-chan error

	state *MempoolState
	txdb  TxDB
	blkdb BlockStatDB
	cfg   Config

	done chan struct{}
	mux  sync.RWMutex
}

func NewCollector(tdb TxDB, bdb BlockStatDB, cfg Config) *Collector {
	c := &Collector{
		txdb:  tdb,
		blkdb: bdb,
		cfg:   cfg,
		done:  make(chan struct{}),
	}
	return c
}

// NOTE: state can be nil, if getState returns errors.
func (c *Collector) State() *MempoolState {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.state
}

func (c *Collector) setState(state *MempoolState) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.state = state
}

func (c *Collector) Run() error {
	logger := c.cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	// Initial mempool state
	if s, err := c.cfg.GetState(); err != nil {
		return err
	} else {
		c.setState(s)
	}

	sc := make(chan *MempoolState)
	blkc := make(chan []Block)
	ec := make(chan error)
	c.S = sc
	c.B = blkc
	c.E = ec
	go c.run(sc, blkc, ec)
	return nil
}

func (c *Collector) Stop() {
	if err := c.closeDone(); err != nil {
		return
	}
	// Block until the err chan is closed when run terminates.
	for _ = range c.E {
	}
}

func (c *Collector) run(sc chan<- *MempoolState, blkc chan<- []Block, ec chan<- error) {
	defer close(ec)
	defer close(blkc)
	defer close(sc)
	defer c.setState(nil)

	logger := c.cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	ticker := time.NewTicker(time.Duration(c.cfg.PollPeriod) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-c.done:
			return
		}

		curr, err := c.cfg.GetState()
		if err != nil {
			select {
			case ec <- fmt.Errorf("GetState: %v", err):
				c.setState(nil)
				continue
			case <-c.done:
				return
			}
		}

		prev := c.State()
		c.setState(curr)
		if prev == nil {
			continue
		}
		if prev.Height > curr.Height {
			panic("Block height decreased!")
		}

		// Add new txs to the TxDB
		newTxs := getNewTxs(prev, curr)
		logger.Printf("[DEBUG] %d new txs, %s", len(newTxs), curr)
		if err := c.txdb.Put(newTxs); err != nil {
			select {
			case ec <- fmt.Errorf("TxDB.Put: %v", err):
				continue
			case <-c.done:
				return
			}
		}

		// Send out the current mempool state
		select {
		case sc <- curr:
		case <-c.done:
			return
		}

		if prev.Height == curr.Height {
			continue
		}
		// Block height has increased; process the new block
		b, blks, err := processBlock(prev, curr, c.cfg.GetBlock, logger)
		if err != nil {
			select {
			case ec <- fmt.Errorf("processBlock: %v", err):
				continue
			case <-c.done:
				return
			}
		}
		// Send out the new blocks
		select {
		case blkc <- blks:
		case <-c.done:
			return
		}
		// Add BlockStats to DB
		if err := c.blkdb.Put(b); err != nil {
			select {
			case ec <- fmt.Errorf("BlockStatDB.Put: %v", err):
				continue
			case <-c.done:
				return
			}
		}
	}
}

func (c *Collector) closeDone() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	select {
	case <-c.done: // Already closed
		return fmt.Errorf("Collector.done already closed")
	default:
		close(c.done)
		return nil
	}
}

func getNewTxs(prev, curr *MempoolState) []est.Tx {
	var txs []est.Tx
	d := curr.Sub(prev)
	for _, entry := range d.Entries {
		txs = append(txs, est.Tx{
			FeeRate: entry.FeeRate(),
			Size:    entry.Size(),
			Time:    entry.Time(),
		})
	}
	return txs
}

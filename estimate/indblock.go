package estimate

import (
	"errors"
	"fmt"
	"sort"

	"github.com/bitcoinfees/feesim/sim"
)

// This error should rarely happen, if block coverage is met.
var ErrInsufficientBlocks = errors.New("too few blocks to estimate blocksource")

type BlockCoverageError struct {
	cov    float64
	minCov float64
	window int64
}

func (err BlockCoverageError) Error() string {
	window := err.window
	covBlocks := int(err.cov * float64(window))
	minCovBlocks := int(err.minCov * float64(window))
	return fmt.Sprintf("Block coverage was only %d/%d, should be at least %d/%d.",
		covBlocks, window, minCovBlocks, window)
}

type IndBlockSourceConfig struct {
	Window        int64   `yaml:"window" json:"window"`
	MinCov        float64 `yaml:"mincov" json:"mincov"`
	GuardInterval int64   `yaml:"guardinterval" json:"guardinterval"`
	TailPct       float64 `yaml:"tailpct" json:"tailpct"`
}

// IndBlockSource returns an estimate of sim.IndBlockSource based on
// BlockStats from heights [height-window+1, height].
func IndBlockSource(height int64, c IndBlockSourceConfig, db BlockStatDB) (*sim.IndBlockSource, error) {
	// Check block coverage
	b, err := db.Get(height-c.Window+1, height)
	if err != nil {
		return nil, err
	}
	cov := float64(len(b)) / float64(c.Window)
	if cov < c.MinCov {
		return nil, BlockCoverageError{cov: cov, minCov: c.MinCov, window: c.Window}
	}

	totalhashes := float64(0)
	var prevBlock *BlockStat
	sizedata := BlockSizeData{}
	sfrdata := BlockSFRData{}
	for _, block := range b {
		totalhashes += block.NumHashes
		if prevBlock == nil {
			prevBlock = block
			continue
		}
		if block.Height == prevBlock.Height+1 {
			if block.Time-prevBlock.Time > c.GuardInterval {
				sizedata = append(sizedata, struct {
					mempoolDiff int64
					blockSize   int64
				}{
					block.MempoolSize - prevBlock.MempoolSizeRemain,
					block.Size,
				})
				sfrdata = append(sfrdata, struct {
					mempoolSize int64
					sfr         sim.FeeRate
				}{
					block.MempoolSize,
					block.SFRStat.SFR,
				})
			}
		} else {
			// Fill in the NumHashes of the missing blocks
			for mh := prevBlock.Height + 1; mh < block.Height; mh++ {
				if mh/diffAdjInterval == prevBlock.Height/diffAdjInterval {
					totalhashes += prevBlock.NumHashes
				} else {
					// Assumes that the height gap is <= 2016. If not, there are
					// serious problems anyway.
					// In any case, window should be <= 2016. Maybe we should
					// enforce that.
					totalhashes += block.NumHashes
				}
			}
		}
		prevBlock = block
	}

	if len(sfrdata) == 0 {
		return nil, ErrInsufficientBlocks
	}
	sort.Sort(sizedata)
	sort.Sort(sfrdata)
	tailidx := int(c.TailPct*float64(len(sfrdata))) + 1 // Min is 1
	sizestail := sizedata[len(sizedata)-tailidx:]
	sfrstail := sfrdata[:tailidx]
	maxblocksizes := make([]sim.TxSize, len(sizestail))
	minfeerates := make([]sim.FeeRate, len(sfrstail))
	for i, size := range sizestail {
		maxblocksizes[i] = sim.TxSize(size.blockSize)
	}
	for i, sfr := range sfrstail {
		minfeerates[i] = sfr.sfr
	}

	// Estimate the blockrate
	winstart := b[0].Time
	winend := b[len(b)-1].Time
	hashrate := totalhashes / float64(winend-winstart)
	blockrate := hashrate / b[len(b)-1].NumHashes

	return sim.NewIndBlockSource(minfeerates, maxblocksizes, blockrate), nil
}

type BlockSFRData []struct {
	mempoolSize int64
	sfr         sim.FeeRate
}

func (b BlockSFRData) Len() int {
	return len(b)
}

func (b BlockSFRData) Less(i, j int) bool {
	return b[i].mempoolSize < b[j].mempoolSize
}

func (b BlockSFRData) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type BlockSizeData []struct {
	mempoolDiff int64
	blockSize   int64
}

func (b BlockSizeData) Len() int {
	return len(b)
}

func (b BlockSizeData) Less(i, j int) bool {
	return b[i].mempoolDiff < b[j].mempoolDiff
}

func (b BlockSizeData) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

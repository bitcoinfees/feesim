package testutil

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

const (
	coin           = 100000000
	priorityThresh = 57600000
)

var (
	BlockData     map[string]*Block
	BlockHashData map[string]string
	MempoolData   map[string]map[string]*MempoolEntry
)

func LoadData(datadir string) {
	f, err := os.Open(filepath.Join(datadir, "mempool.json"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&MempoolData)
	if err != nil {
		panic(err)
	}

	// Load test block data
	f2, err := os.Open(filepath.Join(datadir, "blocks.json"))
	if err != nil {
		panic(err)
	}
	defer f2.Close()
	err = json.NewDecoder(f2).Decode(&BlockData)
	if err != nil {
		panic(err)
	}

	f3, err := os.Open(filepath.Join(datadir, "blockhashes.json"))
	if err != nil {
		panic(err)
	}
	defer f3.Close()
	err = json.NewDecoder(f3).Decode(&BlockHashData)
	if err != nil {
		panic(err)
	}
}

type MempoolEntry struct {
	Size            int64    `json:"size"`
	Time            int64    `json:"time"`
	Depends         []string `json:"depends"`
	Fee             float64  `json:"fee"`
	CurrentPriority float64  `json:"currentpriority"`
}

// Returns the tx fee rate in satoshis / kB
// Panics if called with a zero-value receiver
func (m *MempoolEntry) FeeRate() int64 {
	return int64(m.Fee*coin*1000) / int64(m.Size)
}

// Whether or not the tx is "high priority". We don't want to use these txs to
// estimate miner's min fee rate policies.
func (m *MempoolEntry) IsHighPriority() bool {
	return m.CurrentPriority > priorityThresh
}

type Block struct {
	Height_    int64    `json:"height"`
	Size_      int64    `json:"size"`
	Txids_     []string `json:"tx"`
	Difficulty float64  `json:"difficulty"`
}

// Height returns the block height.
func (b *Block) Height() int64 {
	return b.Height_
}

// Size returns the block size.
func (b *Block) Size() int64 {
	return b.Size_
}

// Txids returns a slice of the block txids. User is free to do whatever with it,
// as a copy is made each time.
func (b *Block) Txids() []string {
	txids := make([]string, len(b.Txids_))
	copy(txids, b.Txids_)
	return txids
}

// Calculate expected number of hashes needed to solve this block
func (b *Block) NumHashes() float64 {
	return b.Difficulty * 4295032833.000015
}

func GetBlock(height int64) (*Block, error) {
	heightstr := strconv.Itoa(int(height))
	hash := BlockHashData[heightstr]
	if hash == "" {
		return nil, errors.New("block data not available")
	}

	b := BlockData[hash]
	if b == nil {
		panic("block data missing")
	}
	return b, nil
}

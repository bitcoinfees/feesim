package testutil

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/btcsuite/btcutil"
)

const (
	coin           = 100000000
	priorityThresh = 57600000
)

var (
	BlockData   map[string]map[string]string
	MempoolData map[string]map[string]*MempoolEntry
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
}

func GetBlockBytes(height int64) []byte {
	heightstr := strconv.Itoa(int(height))
	blockhash := BlockData["blockhashes"][heightstr]
	blockhex := BlockData["blocks"][blockhash]
	blockbytes, err := hex.DecodeString(blockhex)
	if err != nil {
		panic(err)
	}
	return blockbytes
}

func GetBlock(height int64) (*Block, error) {
	blockbytes := GetBlockBytes(height)
	return NewBlockFromBytes(blockbytes, height)
}

type Block struct {
	*btcutil.Block
}

func NewBlockFromBytes(blockbytes []byte, height int64) (*Block, error) {
	b, err := btcutil.NewBlockFromBytes(blockbytes)
	if err != nil {
		return nil, err
	}
	b.SetHeight(int32(height))
	return &Block{b}, nil
}

func (b *Block) Height() int64 {
	return int64(b.Block.Height())
}

// Return the block size
func (b *Block) Size() int64 {
	return int64(b.MsgBlock().SerializeSize())
}

// txids returns a sorted slice of block txids
func (b *Block) Txids() []string {
	txs := b.Transactions()
	txids := make([]string, len(txs))
	for i, tx := range txs {
		txids[i] = tx.Sha().String()
	}
	return txids
}

// Calculate expected number of hashes needed to solve this block
func (b *Block) NumHashes() float64 {
	nbits := b.MsgBlock().Header.Bits
	significand := 0xffffff & nbits
	exponent := (nbits>>24 - 3) * 8
	logtarget := math.Log2(float64(significand)) + float64(exponent)

	return math.Pow(2, 256-logtarget)
}

// Get the scriptsig of the coinbase transaction of a block
func (b *Block) Tag() []byte {
	tx, err := b.Tx(0)
	if err != nil {
		panic("Shouldn't happen, because the only possible error is index out-of-range.")
	}
	return tx.MsgTx().TxIn[0].SignatureScript
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

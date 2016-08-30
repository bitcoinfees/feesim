package sim

import (
	"encoding/json"
	"math"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestTransient(t *testing.T) {
	runtime.GOMAXPROCS(4)

	txsrc := loadMultiTxSource()
	blksrc := loadIndBlockSource()
	initmempool := loadInitMempool("333931")

	// This one was with numiters == 10000
	//feeref := []FeeRate{44248, 27028, 22124, 18067, 14225, 12270, 11326, 11073, 10753, 10285, 10215, 10012, 10007, 10001, 9010, 8434, 7428, 6413}

	// This one was before BlockSource.Next time.Duration high-res tweak
	//feeref := []FeeRate{37736, 29667, 20430, 14948, 13587, 11423, 11326, 11326, 11199, 10969, 10788, 10054, 10018, 10018, 10018, 10012, 10012, 8434}

	// This one was before CPFP changes
	//feeref := []FeeRate{38760, 29586, 20577, 16598, 13387, 11423, 11326, 11326, 10765, 10194, 10194, 10125, 10012, 10012, 10012, 10007, 10007, 10006}

	feeref := []FeeRate{38760, 29627, 20662, 16864, 13720, 13587, 12516, 12516, 10765, 10018, 10018, 10018, 10012, 10012, 10012, 10010, 10010, 10006}
	c := TransientConfig{
		MaxBlockConfirms: 18,
		MinSuccessPct:    0.9,
		NumIters:         100,
		LowestFeeRate:    5000,
	}
	s := NewSim(txsrc, blksrc, initmempool)
	t.Log("sim stablefee is", s.StableFee())
	ts := NewTransientSim(s, c)
	result := ts.Run()
	r := <-result
	if err := testutil.CheckEqual(r, feeref); err != nil {
		t.Error(err)
	}
	// Test that result was closed
	r = <-result
	if r != nil {
		t.Error("r should be nil, was", r)
	}

	// TODO: Test TransientConfig boundary values.

	// Test with absurdly high txrate
	txsrc.txrate = 100
	s = NewSim(txsrc, blksrc, initmempool)
	_c := c
	_c.NumIters = 5
	ts = NewTransientSim(s, _c)
	r = <-ts.Run()
	for _, f := range r {
		if err := testutil.CheckEqual(int(f), 134409); err != nil {
			t.Fatal(err)
		}
	}

	// Test with some BlockPolicy.MinFeeRate == MaxFeeRate
	feeref = []FeeRate{
		-1, -1, -1, 50065, 44445, 44401, 44248, 44248, 44248, 44248, 39216,
		38388, 38388, 38388, 38388, 36102, 29851, 29851}
	blksrc = NewIndBlockSource([]FeeRate{MaxFeeRate, 1000}, []TxSize{1e6}, 1./600.)
	txsrc = loadMultiTxSource()
	s = NewSim(txsrc, blksrc, initmempool)
	ts = NewTransientSim(s, c)
	r = <-ts.Run()
	if err := testutil.CheckEqual(r, feeref); err != nil {
		t.Error(err)
	}

	// Test Stop
	txsrc.txrate = 2.5
	s = NewSim(txsrc, blksrc, initmempool)
	c.NumIters = 10000
	ts = NewTransientSim(s, c)
	rc := ts.Run()
	time.Sleep(time.Millisecond * 100) // Let some iterations complete
	ts.Stop()
	r = <-rc
	if r != nil {
		t.Error("r should be nil.")
	}
	ts.Stop() // Cancel should be idempotent; should not panic here.
}

func BenchmarkTransientGen(b *testing.B) {
	txsrc := loadMultiTxSource()
	blksrc := loadIndBlockSource()
	initmempool := loadInitMempool("333931")
	s := NewSim(txsrc, blksrc, initmempool)
	vc := make(chan transientVar)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	done := make(chan struct{})
	go transientGen(s, 5000, 18, math.MaxInt32, vc, done, wg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		<-vc
	}
	close(done)
	wg.Wait()
}

func BenchmarkTransientMulti(b *testing.B) {
	runtime.GOMAXPROCS(1)
	txsrc := loadMultiTxSource()
	blksrc := loadIndBlockSource()
	initmempool := loadInitMempool("333931")
	c := TransientConfig{
		MaxBlockConfirms: 18,
		MinSuccessPct:    0.9,
		NumIters:         1000,
		LowestFeeRate:    5000,
	}
	s := NewSim(txsrc, blksrc, initmempool)
	var r []FeeRate
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts := NewTransientSim(s, c)
		r = <-ts.Run()
	}
	b.Log(r)
}

func BenchmarkTransientUni(b *testing.B) {
	runtime.GOMAXPROCS(1)
	txsrc := loadUniTxSource()
	blksrc := loadIndBlockSource()
	initmempool := loadInitMempool("333931")
	c := TransientConfig{
		MaxBlockConfirms: 18,
		MinSuccessPct:    0.9,
		NumIters:         1000,
		LowestFeeRate:    5000,
	}
	s := NewSim(txsrc, blksrc, initmempool)
	var r []FeeRate
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts := NewTransientSim(s, c)
		r = <-ts.Run()
	}
	b.Log(r)
}

func loadIndBlockSource() *IndBlockSource {
	// Load the reference block source
	var v map[string]json.RawMessage
	f, err := os.Open("testdata/pools.json")
	defer f.Close()
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		panic(err)
	}
	var (
		minfeerates   []FeeRate
		maxblocksizes []TxSize
		blockrate     float64
	)
	if err := json.Unmarshal(v["minfeerates"], &minfeerates); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(v["maxblocksizes"], &maxblocksizes); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(v["blockrate"], &blockrate); err != nil {
		panic(err)
	}
	return NewIndBlockSource(minfeerates, maxblocksizes, blockrate)
}

func loadMultiTxSource() *MultiTxSource {
	txrate, txs := loadTxSample()
	fees := make([]FeeRate, len(txs))
	s := make([]TxSize, len(txs))
	for i, tx := range txs {
		fees[i], s[i] = tx.FeeRate, tx.Size
	}
	w := make([]float64, len(txs))
	for i := range w {
		w[i] = 1
	}
	return NewMultiTxSource(fees, s, w, txrate)
}

func loadUniTxSource() *UniTxSource {
	txrate, txs := loadTxSample()
	fees := make([]FeeRate, len(txs))
	s := make([]TxSize, len(txs))
	for i, tx := range txs {
		fees[i], s[i] = tx.FeeRate, tx.Size
	}
	return NewUniTxSource(fees, s, txrate)
}

func loadTxSample() (txrate float64, txs []Tx) {
	var v map[string]json.RawMessage
	f, err := os.Open("testdata/txsource.json")
	defer f.Close()
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(&v)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(v["txrate"], &txrate); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(v["txsample"], &txs); err != nil {
		panic(err)
	}
	return txrate, txs
}

package sim

import (
	"math/rand"
	"os"
	"sort"
	"testing"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestMain(m *testing.M) {
	testutil.LoadData("../testutil/testdata")

	// Replace main's getrand; seed value can be anything as long as
	// it's constant
	getrand = func(n int) []*rand.Rand {
		r := make([]*rand.Rand, n)
		for i := range r {
			r[i] = rand.New(rand.NewSource(int64(i + 1)))
		}
		return r
	}

	os.Exit(m.Run())
}

func BenchmarkSim(b *testing.B) {
	initmempool := []*Tx{}
	f := []FeeRate{20000, 10000, 5000}
	s := []TxSize{250, 500, 1000}
	w := []float64{1, 1, 1}

	blocksource := NewIndBlockSource([]FeeRate{5000}, []TxSize{1000000}, 1./600)
	txsource := NewMultiTxSource(f, s, w, 1.5)
	sim := NewSim(txsource, blocksource, initmempool)

	for i := 0; i < b.N; i++ {
		sfr, blksize := sim.NextBlock()
		if i < 20 {
			b.Log(sfr, blksize)
		}
	}
}

func TestSimSFR(t *testing.T) {
	const (
		mfr FeeRate = 10000 // min fee rate
		mbs TxSize  = 50000 // max block size
	)
	initmempool := loadInitMempool("333931")

	blocksource := NewIndBlockSource([]FeeRate{mfr}, []TxSize{mbs}, 1./600)
	txsource := NewMultiTxSource(nil, nil, nil, 0)
	sim := NewSim(txsource, blocksource, initmempool)

	// Updated, post-CPFP, ref data
	sfrsRef := []FeeRate{
		44248,
		38611,
		26738,
		22728,
		19194,
		14567,
		13563,
		12579,
		11631,
		11279,
		10225,
		10000,
	}
	for _, sfrRef := range sfrsRef {
		sfr, blocksize := sim.NextBlock()
		t.Logf("%d/%d", sfr, blocksize)
		if err := testutil.CheckEqual(sfr, sfrRef); err != nil {
			t.Error(err)
		}
	}

	// Test resetting mempool
	sim.Reset()
	for _, sfrRef := range sfrsRef {
		sfr, blocksize := sim.NextBlock()
		t.Logf("%d/%d", sfr, blocksize)
		if err := testutil.CheckEqual(sfr, sfrRef); err != nil {
			t.Error(err)
		}
	}

	// Test reuse initmempool
	sim = NewSim(txsource, blocksource, initmempool)
	for _, sfrRef := range sfrsRef {
		sfr, blocksize := sim.NextBlock()
		t.Logf("%d/%d", sfr, blocksize)
		if err := testutil.CheckEqual(sfr, sfrRef); err != nil {
			t.Error(err)
		}
	}

	// Test empty init mempool
	initmempool = []*Tx{}
	sim = NewSim(txsource, blocksource, initmempool)
	sfr, blocksize := sim.NextBlock()
	t.Logf("%d/%d", sfr, blocksize)
	if err := testutil.CheckEqual(sfr, mfr); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(blocksize, TxSize(0)); err != nil {
		t.Error(err)
	}

	// Various mempool edge cases
	// One tx, just fits into block
	initmempool = []*Tx{
		{
			FeeRate: mfr,
			Size:    mbs,
		},
	}
	sim = NewSim(txsource, blocksource, initmempool)
	sfr, blocksize = sim.NextBlock()
	t.Logf("%d/%d", sfr, blocksize)
	if err := testutil.CheckEqual(sfr, mfr); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(blocksize, mbs); err != nil {
		t.Error(err)
	}

	// One tx, just doesn't fit
	initmempool = []*Tx{
		{
			FeeRate: mfr,
			Size:    mbs + 1,
		},
	}
	sim = NewSim(txsource, blocksource, initmempool)
	sfr, blocksize = sim.NextBlock()
	t.Logf("%d/%d", sfr, blocksize)
	if err := testutil.CheckEqual(sfr, MaxFeeRate); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(blocksize, TxSize(0)); err != nil {
		t.Error(err)
	}

	initmempool = append(initmempool, &Tx{
		FeeRate: mfr,
		Size:    mbs - 1,
	})
	sim = NewSim(txsource, blocksource, initmempool)
	sfr, blocksize = sim.NextBlock()
	t.Logf("%d/%d", sfr, blocksize)
	if err := testutil.CheckEqual(sfr, mfr); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(blocksize, mbs-1); err != nil {
		t.Error(err)
	}

	// One tx with fee rate below min fee rate
	initmempool = []*Tx{
		{
			FeeRate: 9999,
			Size:    1000,
		},
	}
	sim = NewSim(txsource, blocksource, initmempool)
	sfr, blocksize = sim.NextBlock()
	t.Logf("%d/%d", sfr, blocksize)
	if err := testutil.CheckEqual(sfr, mfr); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(blocksize, TxSize(0)); err != nil {
		t.Error(err)
	}

	// With non-null tx source
	initmempool = []*Tx{}
	txsource = NewMultiTxSource([]FeeRate{10000}, []TxSize{1000}, []float64{1}, 0.05)
	sim = NewSim(txsource, blocksource, initmempool)
	k := 0
	N := 10000
	for i := 0; i < N; i++ {
		sfr, blocksize = sim.NextBlock()
		if i < 10 {
			t.Logf("%d/%d", sfr, blocksize)
		}
		if sfr > mfr {
			if err := testutil.CheckEqual(blocksize, mbs); err != nil {
				t.Fatal(err)
			}
			k++
		}
	}
	if err := testutil.CheckPctDiff(float64(k)/float64(N), 0.33, 0.01); err != nil {
		t.Error(err)
	}

	// With null block source
	blocksource = NewIndBlockSource([]FeeRate{MaxFeeRate}, []TxSize{1e6}, 1./600.)
	initmempool = loadInitMempool("333931")
	sim = NewSim(txsource, blocksource, initmempool)
	for i := 0; i < 10; i++ {
		sfr, blocksize := sim.NextBlock()
		if err := testutil.CheckEqual(blocksize, TxSize(0)); err != nil {
			t.Error(err)
		}
		if err := testutil.CheckEqual(sfr, MaxFeeRate); err != nil {
			t.Error(err)
		}
	}
	blocksource = NewIndBlockSource([]FeeRate{1000}, []TxSize{0}, 1./600.)
	sim = NewSim(txsource, blocksource, initmempool)
	for i := 0; i < 10; i++ {
		sfr, blocksize := sim.NextBlock()
		if err := testutil.CheckEqual(blocksize, TxSize(0)); err != nil {
			t.Error(err)
		}
		if err := testutil.CheckEqual(sfr, MaxFeeRate); err != nil {
			t.Error(err)
		}
	}
}

func loadInitMempool(height string) []*Tx {
	txids := []string{}
	m := make(map[string]*Tx)
	txs := testutil.MempoolData[height]
	for txid, tx := range txs {
		txids = append(txids, txid)
		mtx := m[txid]
		if mtx == nil {
			mtx = &Tx{}
		}
		mtx.FeeRate = FeeRate(tx.FeeRate())
		mtx.Size = TxSize(tx.Size)
		m[txid] = mtx
		for _, parent := range tx.Depends {
			if _, ok := txs[parent]; !ok {
				panic("Mempool not closed")
			}
			ptx := m[parent]
			if ptx == nil {
				ptx = &Tx{}
			}
			mtx.Parents = append(mtx.Parents, ptx)
			m[parent] = ptx
		}
	}
	// Establish an arbitrary canonical ordering for the sim mempool, to make
	// test results deterministic.
	sort.Strings(txids)
	s := make([]*Tx, len(txs))
	for i, txid := range txids {
		s[i] = m[txid]
	}
	return s
}

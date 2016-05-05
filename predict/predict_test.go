package predict

import (
	"errors"
	"testing"

	col "github.com/bitcoinfees/feesim/collect"
	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

type MockPredictDB struct {
	txs                map[string]Tx
	attained, exceeded []float64
}

func (d *MockPredictDB) GetTxs(txids []string) (map[string]Tx, error) {
	txs := make(map[string]Tx)
	for _, txid := range txids {
		if tx, ok := d.txs[txid]; ok {
			txs[txid] = tx
		}
	}
	return txs, nil
}

func (d *MockPredictDB) PutTxs(txs map[string]Tx) error {
	for txid, tx := range txs {
		switch txid {
		case "0":
			if err := testutil.CheckEqual(tx.ConfirmIn, int64(3)); err != nil {
				return err
			}
			if err := testutil.CheckEqual(tx.ConfirmBy, int64(4)); err != nil {
				return err
			}
		case "1":
			if err := testutil.CheckEqual(tx.ConfirmIn, int64(1)); err != nil {
				return err
			}
			if err := testutil.CheckEqual(tx.ConfirmBy, int64(2)); err != nil {
				return err
			}
		case "2":
			if err := testutil.CheckEqual(tx.ConfirmIn, int64(2)); err != nil {
				return err
			}
			if err := testutil.CheckEqual(tx.ConfirmBy, int64(3)); err != nil {
				return err
			}
		case "3":
			return errors.New("too low feerate")
		case "3.1":
			return errors.New("has depends")
		case "4":
			return errors.New("was present in prev state")
		default:
			return errors.New("invalid txid")
		}
		d.txs[txid] = tx
	}
	return nil
}

func (d *MockPredictDB) GetScores() ([]float64, []float64, error) {
	return d.attained, d.exceeded, nil
}

func (d *MockPredictDB) PutScores(attained []float64, exceeded []float64) error {
	d.attained = attained
	d.exceeded = exceeded
	return nil
}

func (d *MockPredictDB) Reconcile(txids []string) error {
	return testutil.CheckEqual(txids, []string{"4"})
}

func (d *MockPredictDB) Close() error {
	return nil
}

func NewMockPredictDB() *MockPredictDB {
	return &MockPredictDB{txs: make(map[string]Tx)}
}

func TestPredict(t *testing.T) {
	cfg := Config{MaxBlockConfirms: 4, Halflife: 8}
	p, err := NewPredictor(NewMockPredictDB(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Test AddPredicts
	state0 := &col.MempoolState{
		Entries: map[string]col.MempoolEntry{
			"4": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.00015,
				Size: 1000,
			}},
		},
		Height: 0,
	}
	state1 := &col.MempoolState{
		Entries: map[string]col.MempoolEntry{
			"0": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.00005,
				Size: 1000,
			}},
			"1": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.0001,
				Size: 1000,
			}},
			"2": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.00006,
				Size: 1000,
			}},
			"3": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.00004999,
				Size: 1000,
			}},
			"3.1": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:     0.00006,
				Size:    1000,
				Depends: []string{"0"},
			}},
			"4": &testMempoolEntry{&testutil.MempoolEntry{
				Fee:  0.00015,
				Size: 1000,
			}},
		},
		Height: 1,
	}

	result := []sim.FeeRate{10000, 5001, 5000}

	if err := p.AddPredicts(state0, result); err != nil {
		t.Fatal(err)
	}
	if err := p.AddPredicts(state1, result); err != nil {
		t.Fatal(err)
	}

	// Test ProcessBlock
	b := &testBlock{}
	if err := p.ProcessBlock(b); err != nil {
		t.Fatal(err)
	}
	attained, exceeded, err := p.GetScores()
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(attained, []float64{0, 0, 1, 0}); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(exceeded, []float64{1, 1, 0, 0}); err != nil {
		t.Error(err)
	}

	// Test the decay
	for i := 0; i < cfg.Halflife; i++ {
		if err := p.ProcessBlock(b); err != nil {
			t.Fatal(err)
		}
	}
	attained, exceeded, err = p.GetScores()
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range attained {
		if i != 2 {
			if err := testutil.CheckEqual(v, 0.0); err != nil {
				t.Error(err)
			}
		} else {
			if err := testutil.CheckPctDiff(v, 0.5, 0.0001); err != nil {
				t.Error(err)
			}
		}
	}
	for i, v := range exceeded {
		if i > 1 {
			if err := testutil.CheckEqual(v, 0.0); err != nil {
				t.Error(err)
			}
		} else {
			if err := testutil.CheckPctDiff(v, 0.5, 0.0001); err != nil {
				t.Error(err)
			}
		}
	}

	// Test cleanup
	if err := p.Cleanup(state0); err != nil {
		t.Fatal(err)
	}
}

type testBlock struct {
	i int
}

func (b *testBlock) Height() int64 {
	return 4
}

func (b *testBlock) Size() int64 {
	// Not relevant
	return 0
}

func (b *testBlock) Txids() []string {
	defer func() { b.i++ }()
	if b.i > 0 {
		return nil
	}
	return []string{"0", "1", "2", "3", "3.1", "100"}
}

func (b *testBlock) NumHashes() float64 {
	// Not relevant
	return 0
}

func (b *testBlock) Tag() []byte {
	// Not relevant
	return nil
}

type testMempoolEntry struct {
	*testutil.MempoolEntry
}

func (e *testMempoolEntry) Size() sim.TxSize {
	return sim.TxSize(e.MempoolEntry.Size)
}

func (e *testMempoolEntry) FeeRate() sim.FeeRate {
	return sim.FeeRate(e.MempoolEntry.FeeRate())
}

func (e *testMempoolEntry) Time() int64 {
	return e.MempoolEntry.Time
}

func (e *testMempoolEntry) Depends() []string {
	return e.MempoolEntry.Depends
}

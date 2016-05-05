package estimate

import (
	"math/rand"
	"testing"

	"github.com/bitcoinfees/feesim/testutil"
)

const (
	txrate = 1.5
	window = 7200
)

type TxMemDB struct {
	txs []Tx
}

func (d *TxMemDB) Get(start, end int64) ([]Tx, error) {
	var txs []Tx
	for _, tx := range d.txs {
		if start <= tx.Time && tx.Time <= end {
			txs = append(txs, tx)
		}
	}
	return txs, nil
}

func (d *TxMemDB) init() {
	rand.Seed(0)
	txpool := []Tx{
		{FeeRate: 20000, Size: 250},
		{FeeRate: 20000, Size: 250},
		{FeeRate: 20000, Size: 250},
		{FeeRate: 20000, Size: 250},
		{FeeRate: 20000, Size: 250},
		{FeeRate: 10000, Size: 500},
		{FeeRate: 10000, Size: 500},
		{FeeRate: 10000, Size: 500},
		{FeeRate: 5000, Size: 1000},
		{FeeRate: 5000, Size: 1000},
	}
	d.txs = make([]Tx, int(window*txrate))
	t := 0.0
	for i := range d.txs {
		tx := txpool[rand.Intn(10)]
		e := rand.ExpFloat64() / txrate
		t += e
		tx.Time = int64(t)
		d.txs[i] = tx
	}
}

func TestMultiTxSource(t *testing.T) {
	db := &TxMemDB{}
	db.init()
	tm := db.txs[len(db.txs)-1].Time

	c := &MultiTxSourceConfig{
		MinWindow: 600,
		MaxWindow: window,
		Halflife:  3600,
		MaxTxs:    10000,
	}

	txsrc, err := MultiTxSource(tm, c, db)
	if err != nil {
		t.Fatal(err)
	}

	ratefn := txsrc.RateFn()
	// c.f. sim/txsources_test.go
	xref := []float64{-1, 4999, 5000, 5001, 9999, 10000, 10001, 19999, 20000, 20001}
	yref := []float64{712.5, 712.5, 712.5, 412.5, 412.5, 412.5, 187.5, 187.5, 187.5, 0}
	for i, x := range xref {
		if err := testutil.CheckPctDiff(ratefn.Eval(x), yref[i], 0.02); err != nil {
			t.Error(err)
		}
	}

	// Test too small window
	c = &MultiTxSourceConfig{
		MinWindow: 7201,
		MaxWindow: window,
		Halflife:  3600,
		MaxTxs:    10000,
	}

	txsrc, err = MultiTxSource(tm, c, db)
	if werr, ok := err.(TxWindowError); !ok {
		t.Fatal("Should have WindowTooSmallError")
	} else {
		t.Log(werr)
	}
}

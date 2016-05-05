package estimate

import (
	"math"
	"math/rand"
	"testing"

	"github.com/bitcoinfees/feesim/testutil"
)

func TestUniTxSource(t *testing.T) {
	db := &TxMemDB{}
	db.init()

	// RateFn c.f. sim/txmulti_test.go
	xref := []float64{-1, 4999, 5000, 5001, 9999, 10000, 10001, 19999, 20000, 20001}
	yref := []float64{712.5, 712.5, 712.5, 412.5, 412.5, 412.5, 187.5, 187.5, 187.5, 0}

	c := UniTxSourceConfig{
		MinWindow: 600,
		MaxWindow: window,
		Halflife:  600,
	}
	e := NewUniTxSource(db, c, rand.New(rand.NewSource(0)))

	middle := db.txs[len(db.txs)/2].Time
	latest := db.txs[len(db.txs)-1].Time
	for tm := middle; tm <= latest; tm += 60 {
		txsrc, err := e.Estimate(tm)
		if err != nil {
			t.Fatal(err)
		}
		r := float64(len(e.txs)) * math.Log(e.a) / (math.Pow(e.a, float64(e.window)) - 1)
		if err := testutil.CheckPctDiff(r, txrate, 0.03); err != nil {
			t.Error(err)
		}
		ratefn := txsrc.RateFn()
		for i, x := range xref {
			if err := testutil.CheckPctDiff(ratefn.Eval(x), yref[i], 0.06); err != nil {
				t.Error(err)
			}
		}
	}

	// Test too small window
	c = UniTxSourceConfig{
		MinWindow: 5000,
		MaxWindow: window,
		Halflife:  600,
	}
	e = NewUniTxSource(db, c, rand.New(rand.NewSource(0)))
	_, err := e.Estimate(middle)
	if werr, ok := err.(TxWindowError); !ok {
		t.Fatal("Should have WindowTooSmallError")
	} else {
		t.Log(werr)
	}
}

func TestRoundRandom(t *testing.T) {
	const (
		f = 9.99
		N = 10000
	)
	rng := rand.New(rand.NewSource(0))
	var sum int
	for i := 0; i < N; i++ {
		sum += roundRandom(f, rng)
	}
	avg := float64(sum) / N
	if err := testutil.CheckPctDiff(avg, f, 0.001); err != nil {
		t.Error(err)
	}
}

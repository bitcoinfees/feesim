package bolt

import (
	"math"
	"os"
	"testing"

	"github.com/bitcoinfees/feesim/predict"
	"github.com/bitcoinfees/feesim/testutil"
)

func TestPredictDB(t *testing.T) {
	const dbfile = "testdata/.predict.db"
	os.Remove(dbfile)

	d, err := LoadPredictDB(dbfile)
	if err != nil {
		t.Fatal(err)
	}

	var _ predict.DB = d // Test that the interface is satisfied

	// Shouldn't be able to load again
	_, err = LoadPredictDB(dbfile)
	if err := testutil.CheckEqual(err.Error(), "timeout"); err != nil {
		t.Fatal(err)
	}

	// Close and reopen
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d, err = LoadPredictDB(dbfile); err != nil {
		t.Fatal(err)
	}

	// Put and Get Counts
	attained := []float64{1, 2, 3, 4}
	exceeded := []float64{4, 3, 2, 1}
	if err := d.PutScores(attained, exceeded); err != nil {
		t.Fatal(err)
	}
	attainedGet, exceededGet, err := d.GetScores()
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(attained, attainedGet); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(exceeded, exceededGet); err != nil {
		t.Error(err)
	}

	// Put and Get Txs
	txsRef := map[string]predict.Tx{
		"0": predict.Tx{ConfirmIn: 1, ConfirmBy: math.MaxInt64},
		"1": predict.Tx{ConfirmIn: 3, ConfirmBy: 4},
		"2": predict.Tx{ConfirmIn: 5, ConfirmBy: 1},
	}
	if err := d.PutTxs(txsRef); err != nil {
		t.Fatal(err)
	}
	txs, err := d.GetTxs([]string{"0", "2", "3"})
	if err != nil {
		t.Fatal(err)
	}
	for _, txid := range []string{"0", "2"} {
		if err := testutil.CheckEqual(txs[txid], txsRef[txid]); err != nil {
			t.Error(err)
		}
	}
	if err := testutil.CheckEqual(len(txs), 2); err != nil {
		t.Error(err)
	}

	// Reconcile Txs
	if err := d.Reconcile([]string{"1"}); err != nil {
		t.Fatal(err)
	}
	txs, err = d.GetTxs([]string{"0", "1", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(len(txs), 1); err != nil {
		t.Error(err)
	}
	if err := testutil.CheckEqual(txs["1"], txsRef["1"]); err != nil {
		t.Error(err)
	}

	// Remove dbfile, finally
	if err := os.Remove(dbfile); err != nil {
		t.Fatal(err)
	}
}

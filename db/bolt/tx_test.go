package bolt

import (
	"os"
	"testing"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/testutil"
)

func TestTxDB(t *testing.T) {
	const (
		dbfile = "testdata/.tx.db"
	)
	txsRef := []est.Tx{
		{FeeRate: 5000, Size: 1000, Time: 0},
		{FeeRate: 10000, Size: 500, Time: 1},
		{FeeRate: 20000, Size: 250, Time: 2},
	}

	os.Remove(dbfile)

	d, err := LoadTxDB(dbfile)
	if err != nil {
		t.Fatal(err)
	}

	// Shouldn't be able to load again
	_, err = LoadTxDB(dbfile)
	if err := testutil.CheckEqual(err.Error(), "timeout"); err != nil {
		t.Error(err)
	}

	// Close and reopen
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d, err = LoadTxDB(dbfile); err != nil {
		t.Fatal(err)
	}

	// Put and Get
	if err := d.Put(txsRef); err != nil {
		t.Fatal(err)
	}
	txs, err := d.Get(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(txs, txsRef); err != nil {
		t.Error(err)
	}

	// Get a subrange
	if txs, err = d.Get(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(txs, txsRef[1:]); err != nil {
		t.Error(err)
	}

	// Delete
	if err := d.Delete(0, 1); err != nil {
		t.Fatal(err)
	}
	if txs, err = d.Get(0, 3); err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(txs, txsRef[2:]); err != nil {
		t.Error(err)
	}

	// Remove dbfile, finally
	if err := os.Remove(dbfile); err != nil {
		t.Fatal(err)
	}
}

package bolt

import (
	"os"
	"testing"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/testutil"
)

func TestBlockStatDB(t *testing.T) {
	const (
		dbfile = "testdata/.blockstat.db"
	)

	// Just some random data
	statsRef := []*est.BlockStat{
		{
			Height:            0,
			Size:              250000,
			SFRStat:           est.SFRStat{SFR: 10000, AK: 20, AN: 20, BK: 10, BN: 10},
			MempoolSize:       100000,
			MempoolSizeRemain: 1,
			Time:              1,
			NumHashes:         100,
		},
		{
			Height:            1,
			Size:              251100,
			SFRStat:           est.SFRStat{SFR: 10100, AK: 21, AN: 22, BK: 9, BN: 14},
			MempoolSize:       100001,
			MempoolSizeRemain: 2,
			Time:              2,
			NumHashes:         200,
		},
		{
			Height:            2,
			Size:              251122,
			SFRStat:           est.SFRStat{SFR: 10120, AK: 30, AN: 22, BK: 8, BN: 17},
			MempoolSize:       100001,
			MempoolSizeRemain: 9,
			Time:              3,
			NumHashes:         300,
		},
	}

	os.Remove(dbfile)

	d, err := LoadBlockStatDB(dbfile)
	if err != nil {
		t.Fatal(err)
	}

	// Shouldn't be able to load again
	_, err = LoadBlockStatDB(dbfile)
	if err := testutil.CheckEqual(err.Error(), "timeout"); err != nil {
		t.Fatal(err)
	}

	// Close and reopen
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d, err = LoadBlockStatDB(dbfile); err != nil {
		t.Fatal(err)
	}

	// Put and Get
	if err := d.Put(statsRef); err != nil {
		t.Fatal(err)
	}
	stats, err := d.Get(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(stats, statsRef); err != nil {
		t.Error(err)
	}

	// Get a subrange
	stats, err = d.Get(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(stats, statsRef[1:]); err != nil {
		t.Error(err)
	}

	// Delete
	if err := d.Delete(0, 1); err != nil {
		t.Fatal(err)
	}
	stats, err = d.Get(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(stats, statsRef[2:]); err != nil {
		t.Error(err)
	}

	// Remove dbfile, finally
	if err := os.Remove(dbfile); err != nil {
		t.Fatal(err)
	}
}

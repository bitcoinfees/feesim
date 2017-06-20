package estimate

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/bitcoinfees/feesim/testutil"
)

// Test implementation of BlockStatDB
type BlockStatMemDB struct {
	b []*BlockStat
}

func (d *BlockStatMemDB) Get(start, end int64) (result []*BlockStat, err error) {
	for _, _b := range d.b {
		if start <= _b.Height && _b.Height <= end {
			result = append(result, _b)
		}
	}
	return
}

func (d *BlockStatMemDB) bestHeight() int64 {
	return d.b[len(d.b)-1].Height
}

func (d *BlockStatMemDB) init() {
	f, err := os.Open("testdata/blockstats.json")
	defer f.Close()
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(&d.b)
	if err != nil {
		panic(err)
	}
}

func TestIndBlockSource(t *testing.T) {
	db := &BlockStatMemDB{}
	db.init()
	height := db.bestHeight()
	c := IndBlockSourceConfig{
		Window:        2016,
		MinCov:        0.9,
		GuardInterval: 300,
		TailPct:       0.1,
	}
	blksrc, err := IndBlockSource(height, c, db)
	if err != nil {
		t.Fatal(err)
	}
	capfn := blksrc.RateFn()
	xref := []float64{
		4999, 5000, 6413, 10000, 10009, 10021, 10395, 44405, 222222, math.MaxFloat64}
	yref := []float64{
		0, 942.646, 1027.22, 1135.98, 1220.58, 1305.17, 1389.77, 1474.36, 1486.45, 1486.45}
	for i, x := range xref {
		if err := testutil.CheckEqual(int(capfn.Eval(x)), int(yref[i])); err != nil {
			t.Error(err)
		}
	}

	// Test coverage error
	c.MinCov = 0.999
	_, err = IndBlockSource(height, c, db)
	if _, ok := err.(BlockCoverageError); !ok {
		t.Fatal("Coverage error not returned.")
	}
	t.Log(err)
}

func TestIndBlockSourceSMFR(t *testing.T) {
	db := &BlockStatMemDB{}
	db.init()
	height := db.bestHeight()
	c := IndBlockSourceConfig{
		Window:        2016,
		MinCov:        0.9,
		GuardInterval: 300,
		TailPct:       0.1,
	}
	blksrc, err := IndBlockSourceSMFR(height, c, db)
	if err != nil {
		t.Fatal(err)
	}
	capfn := blksrc.RateFn()
	xref := []float64{
		4999, 5000, math.MaxFloat64}
	yref := []float64{
		0, 1498, 1498}
	for i, x := range xref {
		if err := testutil.CheckEqual(int(capfn.Eval(x)), int(yref[i])); err != nil {
			t.Error(err)
		}
	}

	// Test coverage error
	c.MinCov = 0.999
	_, err = IndBlockSourceSMFR(height, c, db)
	if _, ok := err.(BlockCoverageError); !ok {
		t.Fatal("Coverage error not returned.")
	}
	t.Log(err)
}

package collect

import (
	"testing"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

func TestProcessBlock(t *testing.T) {
	const height = 333931

	// Test basic
	prev, err := statedata(height)
	if err != nil {
		t.Fatal(err)
	}
	curr, err := statedata(height + 1)
	if err != nil {
		t.Fatal(err)
	}
	b, blocks, err := processBlock(prev, curr, getBlock, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 {
		t.Error("b should have len 1")
	}
	b_ref := []*est.BlockStat{
		{
			Height:            height,
			NumHashes:         1.718333983803829e+20,
			Size:              153669,
			Time:              prev.Time,
			MempoolSize:       681121,
			MempoolSizeRemain: 535628,
			SFRStat: est.SFRStat{
				SFR: 23310,
				AK:  305,
				AN:  308,
				BK:  204,
				BN:  204,
			},
		},
	}
	if err := testutil.CheckEqual(b, b_ref); err != nil {
		t.Error(err)
	}

	if len(blocks) != 1 {
		t.Error("blocks should have len1")
	}
	if err := testutil.CheckEqual(blocks[0].Height(), int64(height)); err != nil {
		t.Error(err)
	}
	// Subsequent tests also test whether prev is mutated by processBlock,
	// so there's no need to test that separately.

	// Test with conflicts
	curr.Entries = nil
	b, _, err = processBlock(prev, curr, getBlock, nil)
	b_ref[0].SFRStat = est.SFRStat{
		SFR: minrelaytxfee,
		AK:  305,
		AN:  305,
		BK:  0,
		BN:  0,
	}
	if err := testutil.CheckEqual(b, b_ref); err != nil {
		t.Error(err)
	}

	// Test with empty mempool
	prev.Entries = nil
	b, _, err = processBlock(prev, curr, getBlock, nil)
	if err != nil {
		t.Fatal(err)
	}
	b_ref[0].SFRStat = est.SFRStat{
		SFR: sim.MaxFeeRate,
		AK:  0,
		AN:  0,
		BK:  0,
		BN:  0,
	}
	b_ref[0].MempoolSize = 0
	b_ref[0].MempoolSizeRemain = 0
	if err := testutil.CheckEqual(b, b_ref); err != nil {
		t.Error(err)
	}

	// Test multiple blocks
	const n = 3
	if prev, err = statedata(height); err != nil {
		t.Fatal(err)
	}
	// Three blocks later
	curr = prev.Copy()
	curr.Height += n
	b, _, err = processBlock(prev, curr, getBlock, nil)
	if err != nil {
		t.Fatal(err)
	}
	b_ref = []*est.BlockStat{
		{
			Height:            height,
			NumHashes:         1.718333983803829e+20,
			Size:              153669,
			Time:              b[0].Time,
			MempoolSize:       681121,
			MempoolSizeRemain: 535628,
			SFRStat: est.SFRStat{
				SFR: 23310,
				AK:  305,
				AN:  308,
				BK:  204,
				BN:  204,
			},
		},
		{
			Height:            height + 1,
			NumHashes:         1.718333983803829e+20,
			Size:              499062,
			Time:              b[0].Time,
			MempoolSize:       535628,
			MempoolSizeRemain: 55677,
			SFRStat: est.SFRStat{
				SFR: 10224,
				AK:  207,
				AN:  212,
				BK:  11,
				BN:  11,
			},
		},
		{
			Height:            height + 2,
			NumHashes:         1.718333983803829e+20,
			Size:              642771,
			Time:              b[0].Time,
			MempoolSize:       55677,
			MempoolSizeRemain: 14512,
			SFRStat: est.SFRStat{
				SFR: 7427,
				AK:  8,
				AN:  8,
				BK:  8,
				BN:  8,
			},
		},
	}
	if err := testutil.CheckEqual(b, b_ref); err != nil {
		t.Error(err)
	}

	// Test multiple blocks with conflicts
	curr.Entries = nil
	b, _, err = processBlock(prev, curr, getBlock, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.CheckEqual(len(b), n); err != nil {
		t.Fatal(err)
	}

	if err := testutil.CheckEqual(b[n-1].SFRStat.SFR, sim.FeeRate(minrelaytxfee)); err != nil {
		t.Error(err)
	}
}

package estimate

import (
	"testing"

	"github.com/bitcoinfees/feesim/sim"
	"github.com/bitcoinfees/feesim/testutil"
)

const minrelaytxfee = 5000

func TestSFR(t *testing.T) {
	var (
		ref SFRStat
		txs SFRTxSlice
	)

	// Test #1
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: true},
		{FeeRate: 10000, InBlock: true},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 9999, InBlock: false},
		{FeeRate: 9999, InBlock: false},
	}, 10)
	ref = SFRStat{
		SFR: 11000,
		AK:  10,
		AN:  10,
		BK:  30,
		BN:  40,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #2
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 9999, InBlock: false},
		{FeeRate: 9999, InBlock: false},
	}, 10)
	ref = SFRStat{
		SFR: sim.MaxFeeRate,
		AK:  0,
		AN:  0,
		BK:  50,
		BN:  50,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #3
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: true},
		{FeeRate: 10000, InBlock: true},
		{FeeRate: 10000, InBlock: true},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9999, InBlock: true},
	}, 10)
	ref = SFRStat{
		SFR: minrelaytxfee,
		AK:  50,
		AN:  50,
		BK:  0,
		BN:  0,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #4
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9998, InBlock: true},
	}, 10)
	ref = SFRStat{
		SFR: sim.MaxFeeRate,
		AK:  0,
		AN:  0,
		BK:  30,
		BN:  60,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #5
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9998, InBlock: true},
		{FeeRate: 9998, InBlock: true},
	}, 10)
	ref = SFRStat{
		SFR: minrelaytxfee,
		AK:  40,
		AN:  70,
		BK:  0,
		BN:  0,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #6
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: false},
		{FeeRate: 10000, InBlock: true},
		{FeeRate: 10000, InBlock: false},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9999, InBlock: true},
		{FeeRate: 9998, InBlock: false},
		{FeeRate: 9998, InBlock: true},
	}, 10)
	ref = SFRStat{
		SFR: 9999,
		AK:  30,
		AN:  50,
		BK:  10,
		BN:  20,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #7 - Empty txs
	txs = replicateTxs(SFRTxSlice{}, 10)
	ref = SFRStat{
		SFR: sim.MaxFeeRate,
		AK:  0,
		AN:  0,
		BK:  0,
		BN:  0,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #8
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: true},
		{FeeRate: 11000, InBlock: false},
	}, 10)
	ref = SFRStat{
		SFR: sim.MaxFeeRate,
		AK:  0,
		AN:  0,
		BK:  10,
		BN:  20,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}

	// Test #9
	txs = replicateTxs(SFRTxSlice{
		{FeeRate: 11000, InBlock: true},
		{FeeRate: 11000, InBlock: false},
		{FeeRate: 10000, InBlock: true},
	}, 10)
	ref = SFRStat{
		SFR: minrelaytxfee,
		AK:  20,
		AN:  30,
		BK:  0,
		BN:  0,
	}
	if err := doSFRTest(txs, ref); err != nil {
		t.Error(err)
	}
}

func doSFRTest(txs SFRTxSlice, stat_ref SFRStat) error {
	stat := txs.StrandingFeeRate(minrelaytxfee)
	return testutil.CheckEqual(stat, stat_ref)
}

// Replicate input txs by a factor of r
func replicateTxs(txs SFRTxSlice, r int) (result SFRTxSlice) {
	result = make(SFRTxSlice, len(txs)*r)
	for i := range result {
		result[i] = txs[i%len(txs)]
	}
	return
}

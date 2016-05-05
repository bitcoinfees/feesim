// Stranding fee rate calculations

package estimate

import (
	"fmt"
	"sort"

	"github.com/bitcoinfees/feesim/sim"
)

type SFRStat struct {
	SFR sim.FeeRate `json:"sfr"`
	AK  int64       `json:"ak"`
	AN  int64       `json:"an"`
	BK  int64       `json:"bk"`
	BN  int64       `json:"bn"`
}

func (s SFRStat) String() string {
	return fmt.Sprintf("%d SFR, %d/%d AKN, %d/%d BKN",
		s.SFR, s.AK, s.AN, s.BK, s.BN)
}

type SFRTx struct {
	FeeRate sim.FeeRate
	InBlock bool
}

type SFRTxSlice []SFRTx

func (t SFRTxSlice) Len() int {
	return len(t)
}

func (t SFRTxSlice) Less(i, j int) bool {
	return t[i].FeeRate > t[j].FeeRate // Descending order
}

func (t SFRTxSlice) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t SFRTxSlice) Sort() {
	sort.Sort(t)
}

func (t SFRTxSlice) StrandingFeeRate(minrelaytxfee sim.FeeRate) (stat SFRStat) {
	if len(t) == 0 {
		return SFRStat{SFR: sim.MaxFeeRate}
	}

	// Sort by descending fee rate
	t.Sort()

	// Assert that all txs are at least minrelaytxfee
	if t[len(t)-1].FeeRate < minrelaytxfee {
		panic("SFR: tx has fee rate lower than MinRelayTxFee")
	}

	var (
		k, maxk int
		sfr     sim.FeeRate = sim.MaxFeeRate
	)
	for i, tx := range t {
		if tx.InBlock {
			k++
		} else {
			k--
		}

		// Continue if the next fee rate is the same as this one.
		// We only want to set maxk after considering all txs with the
		// same fee rate.
		if i < len(t)-1 && t[i+1].FeeRate == tx.FeeRate {
			continue
		}

		if k > maxk {
			maxk = k
			sfr = tx.FeeRate
		}
	}

	if sfr == t[len(t)-1].FeeRate {
		// SFR is equal to the lowest fee rate, so just set it equal to the
		// minrelaytxfee for a smoother result.
		sfr = minrelaytxfee
	}

	stat.SFR = sfr
	stat.AK, stat.AN, stat.BK, stat.BN = t.ABKN(sfr)

	return
}

// Calculate the Above/Below K/N values
func (t SFRTxSlice) ABKN(sfr sim.FeeRate) (ak, an, bk, bn int64) {
	for _, tx := range t {
		if tx.FeeRate >= sfr {
			an++
			if tx.InBlock {
				ak++
			}
		} else {
			bn++
			if !tx.InBlock {
				bk++
			}
		}
	}
	return
}

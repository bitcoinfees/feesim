package collect

import (
	"log"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	est "github.com/bitcoinfees/feesim/estimate"
)

func processBlock(prev, curr *MempoolState, getBlock BlockGetter, logger *log.Logger) (
	[]*est.BlockStat, []Block, error) {

	n := curr.Height - prev.Height
	if n <= 0 {
		panic("processBlock: must have new.Height > old.Height")
	}
	prev = prev.Copy() // Because prev will get mutated
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	b := make([]*est.BlockStat, 0, n)
	s := make([]map[string]est.SFRTx, 0, n)
	blocks := make([]Block, 0, n)
	minLeadTime := make([]int64, 0, n)
	for height := prev.Height + 1; height <= curr.Height; height++ {
		block, err := getBlock(height)
		if err != nil {
			return nil, nil, err
		}

		bi := &est.BlockStat{
			Height:    height,
			NumHashes: block.NumHashes(),
			Size:      block.Size(),
			Time:      prev.Time,
		}

		// Calculate the mempool size
		for _, entry := range prev.Entries {
			bi.MempoolSize += int64(entry.Size())
		}

		blockTxids := block.Txids()
		sort.Strings(blockTxids)
		// Initialize SFR txs
		si := make(map[string]est.SFRTx)
		var cutoff, inBlockSize int64
		for txid, entry := range prev.Entries {
			stx := est.SFRTx{FeeRate: entry.FeeRate()}
			if stringSliceContains(blockTxids, txid) {
				// tx is in block
				stx.InBlock = true
				if entry.Time() > cutoff {
					cutoff = entry.Time()
				}
				inBlockSize += int64(entry.Size())
				delete(prev.Entries, txid)
			}
			// Shortlist the tx if it satisfies the criteria:
			// 1. No mempool dependencies
			// 2. Not high priority
			if len(entry.Depends()) != 0 || entry.IsHighPriority() {
				continue
			}
			si[txid] = stx
		}
		// TODO: Consider subtracting conflicts from mempoolsizeremain
		bi.MempoolSizeRemain = bi.MempoolSize - inBlockSize

		// Further shortlist, if tx Time is below the cutoff.
		// The cutoff represents an estimate for the latest time for a tx to
		// be eligible for block inclusion. The basis for this is that miners
		// only update their block tx lists at set intervals, instead of
		// continuously (i.e. whenever a new tx enters the mempool). The default
		// in Bitcoin Core is every 60 seconds.
		for txid, stx := range si {
			// First operand is there because if stx.InBlock, it would have
			// earlier been deleted from old. It's not strictly necessary
			// because map will return the zero value if the key is not present,
			// but this way is slightly clearer.
			if !stx.InBlock && prev.Entries[txid].Time() > cutoff {
				delete(si, txid)
			}
		}

		b = append(b, bi)
		s = append(s, si)
		blocks = append(blocks, block)
		minLeadTime = append(minLeadTime, prev.Time-cutoff)
	}

	// Check for conflicts. Conflicts are txs which were removed from mempool
	// but yet were not included in any block, i.e. they were removed as a
	// result of a UTXO conflict. We don't want these txs in the SFR calcs.
	conflicts := prev.Sub(curr).Entries
	var (
		conflictsize int64
		conflictnum  int64
	)
	for txid, entry := range conflicts {
		for _, si := range s {
			delete(si, txid)
		}
		conflictsize += int64(entry.Size())
		conflictnum++
	}

	if conflictsize > 0 {
		logger.Printf("Block %d: %d conflicts (%d bytes) removed",
			prev.Height+1, conflictnum, conflictsize)
	}

	// Now we're ready to do SFR calcs.
	for i, si := range s {
		stxs := make(est.SFRTxSlice, 0, len(si))
		for _, stx := range si {
			stxs = append(stxs, stx)
		}
		b[i].SFRStat = stxs.StrandingFeeRate(prev.MinFeeRate)
		logger.Printf("Block %d: %d S, %d RS, %d MSR, %d MLT, %s, %s",
			b[i].Height, blocks[i].Size(), b[i].MempoolSize-b[i].MempoolSizeRemain,
			b[i].MempoolSizeRemain, minLeadTime[i], b[i].SFRStat,
			printable(string(blocks[i].Tag())))
	}

	return b, blocks, nil
}

// printable filters an input string, removing unprintable / undecodable UTF-8
func printable(in string) (out string) {
	out = strings.Map(func(in rune) (out rune) {
		if in == utf8.RuneError || !unicode.IsPrint(in) {
			return -1
		}
		return in
	}, in)
	return
}

// stringSliceContains tests if t is in (sorted) s.
func stringSliceContains(s []string, t string) bool {
	i := sort.SearchStrings(s, t)
	if i < len(s) && s[i] == t {
		return true
	}
	return false
}

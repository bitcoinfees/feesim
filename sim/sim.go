/*
Package sim performs transaction queue simulations based on abstract inputs.

The inputs to a sim are a transaction source (type TxSource) a block source
(type BlockSource), and the mempool state (type []*Tx).

A transaction source emits transactions into the mempool queue, given a time
interval (implying that the source is time-homogeneous). A block source models
the discovery of blocks by miners, while the mempool state is the graph of
mempool transactions.

Miners are assumed to include transactions greedily by fee rate, considering
each transaction in isolation (i.e. no child-pays-for-parent), and subject to a
minimum fee rate and a maximum block size, which is specified by the BlockPolicy
object.

The output of a sim is a sequence of blocks that are each represented by their
"stranding fee rate" (SFR), which is approximately the minimum fee rate required
for a transaction to be included in that block. From the SFR sequence, we can
obtain many of the queue metrics that are of interest.

Package sim also contains transaction / block source models (i.e.
implementations of the TxSource and BlockSource interfaces). Model estimation is
performed by package estimate.
*/
package sim

import (
	"math"
	"sort"
)

type Sim struct {
	txsource    TxSource
	blocksource BlockSource
	initmempool []*Tx
	initqueue   txqueue
	queue       txqueue
	stablefee   FeeRate
	minTxSize   TxSize
}

// NewSim ... initmempool must be closed; i.e. SimMempoolTx Children must be
// contained in initmempool.
func NewSim(txsource TxSource, blocksource BlockSource, initmempool []*Tx) *Sim {
	// Starting with v0.2.0, we pretend that all initial mempool transactions
	// have no mempool dependencies (i.e. all its txins are already in-chain). This
	// is due to Bitcoin Core's addition of child-pays-for-parent (CPFP) in v0.13.0;
	// ideally we would want to model CPFP here as well, but I think the added model
	// fidelity is not worth the extra computation cost - the mempool dependencies
	// of tx arrivals aren't modeled anyway.
	// Eventually we'll want to clean up the code to remove all the now-obsolete
	// pre-CPFP logic, but we're leaving it in for now because we might change our
	// minds about modeling CPFP, which would necessitate a large overhaul.
	for _, tx := range initmempool {
		tx.Parents = tx.Parents[:0]
	}

	// Calculate the stable fee rate. All tx arrivals with fee rate less than
	// this are discarded. This is a necessary but not sufficient condition
	// for sim stability; You should take measures elsewhere to bound the
	// sim time / memory usage.
	txratefn, capratefn := txsource.RateFn(), blocksource.RateFn()
	highfee := txratefn.Inverse(0)            // The highest possible stablefee
	maxcap := capratefn.Eval(math.MaxFloat64) // Maximum capacity byte rate
	// TODO: base lowfee on a factor of maxcap, say 0.25
	lowfee := capratefn.Inverse(1) // Lowest fee at which there's nonzero cap

	var n int
	if highfee > float64(math.MaxInt32) {
		// Because sort.Search takes int not int64.
		// If there are txs which pay a fee rate of > MaxInt32,
		// then it's not ideal but also not critical.
		n = math.MaxInt32
	} else {
		n = int(highfee)
	}

	stablefee := FeeRate(sort.Search(n, func(i int) bool {
		return maxcap > txratefn.Eval(float64(i)) && i >= int(lowfee)
	}))

	minTxSize := txsource.MinSize()

	// Zero all the children
	for _, tx := range initmempool {
		tx.children = tx.children[:0]
	}

	// Find minTxSize, add to initqueue, and add the children
	var initqueue txqueue
	for _, tx := range initmempool {
		if tx.Size < minTxSize {
			minTxSize = tx.Size
		}
		if len(tx.Parents) == 0 {
			initqueue = append(initqueue, tx)
		}
		for _, p := range tx.Parents {
			p.children = append(p.children, tx)
		}
	}
	initqueue.init()

	s := &Sim{
		txsource:    txsource,
		blocksource: blocksource,
		initmempool: initmempool,
		initqueue:   initqueue,
		stablefee:   stablefee,
		minTxSize:   minTxSize,
	}
	s.Reset()
	return s
}

func (s *Sim) NextBlock() (sfr FeeRate, blocksize TxSize) {
	t, b := s.blocksource.Next()
	// TODO: Consider passing stablefee to Generate instead of filtering here
	newtxs := s.txsource.Generate(t)
	for _, tx := range newtxs {
		if tx.FeeRate >= s.stablefee {
			s.queue = append(s.queue, tx)
		}
	}
	s.queue.init()

	var (
		sizeltd int
		spilled []*Tx
	)
	sfr = MaxFeeRate

	for len(s.queue) > 0 {
		if b.MaxBlockSize-blocksize < s.minTxSize {
			sizeltd = 1
			break
		}
		tx := s.queue.pop()
		if tx.FeeRate >= b.MinFeeRate {
			// Above min fee rate, now check max block size
			blocksize += tx.Size
			if blocksize <= b.MaxBlockSize {
				// tx accepted to this block
				if sizeltd > 0 {
					sizeltd--
				} else if tx.FeeRate < sfr {
					sfr = tx.FeeRate
				}
				s.processchildren(tx)
			} else {
				// Exceeded max block size; tx "spilled"
				sizeltd++
				blocksize -= tx.Size
				spilled = append(spilled, tx)
			}
		} else {
			// Below min fee rate, so we're done
			s.queue = append(s.queue, tx)
			break
		}
	}

	// Add spilled txs back to the queue
	s.queue = append(s.queue, spilled...)

	if sizeltd > 0 {
		if sfr < MaxFeeRate {
			sfr++
		}
	} else {
		sfr = b.MinFeeRate
	}
	if sfr < s.stablefee {
		sfr = s.stablefee
	}
	return sfr, blocksize
}

// Reset the mempool to initial state
func (s *Sim) Reset() {
	for _, tx := range s.initmempool {
		tx.removedparents = 0
	}
	s.queue = make(txqueue, len(s.initqueue))
	copy(s.queue, s.initqueue)
}

// Check if children of tx have satisfied all deps, i.e. all parents have
// been confirmed. If yes, push child onto queue heap.
func (s *Sim) processchildren(tx *Tx) {
	for _, c := range tx.children {
		c.removedparents++
		if c.removedparents == len(c.Parents) {
			s.queue.push(c)
		}
	}
}

func (s *Sim) StableFee() FeeRate {
	return s.stablefee
}

// Copy makes n copies of s, which have isolated random states and are
// concurrent-safe.
func (s *Sim) Copy(n int) []*Sim {
	ss := make([]*Sim, n)
	tt := s.txsource.Copy(n)
	bb := s.blocksource.Copy(n)
	for i := range ss {
		idx := make(map[*Tx]int)
		for i, tx := range s.initmempool {
			idx[tx] = i
		}
		m := make([]*Tx, len(s.initmempool))
		for i := range m {
			m[i] = &Tx{
				FeeRate:  s.initmempool[i].FeeRate,
				Size:     s.initmempool[i].Size,
				Parents:  make([]*Tx, len(s.initmempool[i].Parents)),
				children: make([]*Tx, len(s.initmempool[i].children)),
			}
		}
		for i := range m {
			for j := range m[i].Parents {
				pidx, ok := idx[s.initmempool[i].Parents[j]]
				if !ok {
					panic("mempool deps not closed")
				}
				m[i].Parents[j] = m[pidx]
			}
			for j := range m[i].children {
				cidx, ok := idx[s.initmempool[i].children[j]]
				if !ok {
					panic("mempool deps not closed")
				}
				m[i].children[j] = m[cidx]
			}
		}
		q := make(txqueue, len(s.initqueue))
		for i, tx := range s.initqueue {
			q[i] = m[idx[tx]]
		}
		ss[i] = &Sim{
			txsource:    tt[i],
			blocksource: bb[i],
			initmempool: m,
			stablefee:   s.stablefee,
			initqueue:   q,
			minTxSize:   s.minTxSize,
		}
		ss[i].Reset()
	}
	return ss
}

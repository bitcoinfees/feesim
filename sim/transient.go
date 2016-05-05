package sim

import (
	"runtime"
	"sort"
	"sync"
)

type TransientConfig struct {
	MaxBlockConfirms int     `yaml:"maxblockconfirms" json:"maxblockconfirms"`
	MinSuccessPct    float64 `yaml:"minsuccesspct" json:"minsuccesspct"` // should be in [0, 1)
	NumIters         int     `yaml:"numiters" json:"numiters"`

	LowestFeeRate FeeRate `yaml:"-" json:"-"`
}

type transientVar struct {
	feeRates  []FeeRate
	confTimes []int
}

type TransientSim struct {
	sim *Sim

	cfg TransientConfig

	// Lowest fee rate for which conf times will be estimated.
	// It's max(s.StableFee(), c.LowestFeeRate)
	lowestfee FeeRate

	// Used to stop the sim, ans also to signify that Run has been called, since
	// the channel is made in Run.
	done chan struct{}

	wg  sync.WaitGroup
	mux sync.RWMutex
}

func NewTransientSim(s *Sim, cfg TransientConfig) *TransientSim {
	lowestfee := cfg.LowestFeeRate
	if lowestfee < s.StableFee() {
		lowestfee = s.StableFee()
	}
	return &TransientSim{
		sim:       s,
		cfg:       cfg,
		lowestfee: lowestfee,
		done:      make(chan struct{}),
	}
}

// Stop (abort) the sim and block until all goroutines terminate.
func (ts *TransientSim) Stop() {
	ts.closeDone()
	ts.wg.Wait()
}

// closeDone closes ts.done in a concurrent-safe way.
func (ts *TransientSim) closeDone() {
	ts.mux.Lock()
	defer ts.mux.Unlock()
	select {
	case <-ts.done: // Already closed
	default:
		close(ts.done)
	}
}

func (ts *TransientSim) Run() <-chan []FeeRate {
	r := make(chan []FeeRate)
	ts.wg.Add(1)
	go ts.run(r)
	return r
}

func (ts *TransientSim) run(r chan<- []FeeRate) {
	var result []FeeRate

	defer close(r)
	defer ts.closeDone()
	defer func() {
		select {
		case r <- result:
		case <-ts.done:
		}
	}()
	defer ts.wg.Wait()
	defer ts.wg.Done()

	numprocs := runtime.GOMAXPROCS(0)

	ts.sim.Reset()
	ss := ts.sim.Copy(numprocs - 1)
	ss = append(ss, ts.sim)

	vc := make(chan transientVar, numprocs)
	ts.wg.Add(numprocs)
	for i, s := range ss {
		n := ts.cfg.NumIters / numprocs
		if i == 0 {
			n += ts.cfg.NumIters % numprocs
		}
		// We don't actually have to specify the work chunk (n) per goroutine;
		// we could simply read from vc until we have enough iterations, then
		// call ts.Stop to shut down the goroutines.
		// That would be slightly more efficient in the case where the
		// goroutines execute at different rates. However, we do it this way
		// (i.e. fixed work chunk per goroutine) so that the results will be
		// deterministic, which facilitates unit testing.
		go transientGen(s, ts.lowestfee, ts.cfg.MaxBlockConfirms, n, vc, ts.done, &ts.wg)
	}

	tvars := make([]transientVar, ts.cfg.NumIters)
	fset := make(map[FeeRate]struct{}) // A set of all tvar feerates
	for i := range tvars {
		select {
		case tvars[i] = <-vc:
			for _, feeRate := range tvars[i].feeRates {
				fset[feeRate] = struct{}{}
			}
		case <-ts.done:
			return
		}
	}

	// Form a reverse sorted array from fset
	f := make([]FeeRate, len(fset))
	i := 0
	for feeRate := range fset {
		f[i] = feeRate
		i++
	}
	feeRateSlice(f).ReverseSort()

	b := make([][]int, len(f))
	for i := range b {
		b[i] = make([]int, ts.cfg.MaxBlockConfirms+1)
	}
	// Get the blockconf variates for each fee rate
	for _, v := range tvars {
		k := 0
		for j, feeRate := range v.feeRates {
			for ; k < len(f) && f[k] >= feeRate; k++ {
				b[k][v.confTimes[j]-1]++
			}
		}
		// Sanity check
		if k != len(f) {
			panic("Some feerates were skipped.")
		}
	}

	// Now get the blockconf MinSuccessPct percentile for each fee rate
	p := make([]int, len(f))
	T := int(ts.cfg.MinSuccessPct * float64(len(tvars)))
	for i, _b := range b {
		sum := 0
		for j, count := range _b {
			sum += count
			if sum >= T {
				p[i] = j + 1
				break
			}
		}
	}

	// TODO: Sanity check. We can take this out later.
	if !sort.IntsAreSorted(p) {
		panic("p should be sorted.")
	}

	// result[i] is the lowest fee to confirm in i+1 blocks
	result = make([]FeeRate, ts.cfg.MaxBlockConfirms)
	// Get the lowest fee rate for each conf time
	for i := range result {
		idx := sort.SearchInts(p, i+2)
		if idx > 0 {
			result[i] = f[idx-1]
		} else {
			result[i] = -1
		}
	}
}

// transientGen ... maxblocks is MAX_BLOCK_CONFIRMS, n is numiters.
// To be run in a goroutine.
func transientGen(s *Sim, lowest FeeRate, maxblocks, n int, vc chan<- transientVar, done <-chan struct{}, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	for i := 0; i < n; i++ {
		low := MaxFeeRate
		v := transientVar{}
		for j := 1; j <= maxblocks; j++ {
			sfr, _ := s.NextBlock()
			if sfr < lowest {
				sfr = lowest
			}
			if sfr < low {
				v.feeRates = append(v.feeRates, sfr)
				v.confTimes = append(v.confTimes, j)
				low = sfr
			}
			if sfr == lowest {
				break
			}
		}
		if len(v.feeRates) == 0 || v.feeRates[len(v.feeRates)-1] != lowest {
			v.feeRates = append(v.feeRates, lowest)
			v.confTimes = append(v.confTimes, maxblocks+1)
		}
		select {
		case vc <- v:
		case <-done:
			return
		}
		s.Reset()
	}
}

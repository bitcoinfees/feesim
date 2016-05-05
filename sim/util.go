package sim

import (
	"math"
	"math/rand"
	"sort"
	"time"
)

// getrand returns n seeded *rand.Rand instances.
// Don't call this too often i.e. on the order of nanoseconds.
var getrand = func(n int) []*rand.Rand {
	initseed := time.Now().UnixNano()
	r := make([]*rand.Rand, n)
	for i := range r {
		r[i] = rand.New(rand.NewSource(initseed + int64(i)))
	}
	return r
}

// searchFloat64s optimizes sort.SearchFloat64s by inlining f.
func searchFloat64s(a []float64, x float64) int {
	//return Search(len(a), func(i int) bool { return a[i] >= x })

	n := len(a)
	// Define f(-1) == false and f(n) == true.
	// Invariant: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := i + (j-i)/2 // avoid overflow when computing h
		// i ≤ h < j
		if a[h] < x {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}

// Returns a poisson variate with expected value l.
func poissonvariate(l float64, r *rand.Rand) int64 {
	if l == 0 {
		return 0
	}
	if l > 30 {
		// Normal approximation
		x := r.NormFloat64()*math.Sqrt(l) + l
		// Round to nearest integer
		if i := int64(x); x-float64(i) > 0.5 {
			return i + 1
		} else {
			return i
		}
	}
	// http://en.wikipedia.org/wiki/Poisson_distribution
	// #Generating_Poisson-distributed_random_variables
	L := math.Exp(-l)
	var (
		k int64
		p float64 = 1
	)
	for p > L {
		k++
		p *= r.Float64()
	}
	return k - 1
}

// Implements sort.Interface
type feeRateSlice []FeeRate

func (p feeRateSlice) Len() int {
	return len(p)
}

func (p feeRateSlice) Less(i, j int) bool {
	return p[i] < p[j]
}

func (p feeRateSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p feeRateSlice) Sort() {
	sort.Sort(p)
}

func (p feeRateSlice) ReverseSort() {
	sort.Sort(sort.Reverse(p))
}

// Custom heap implementation, modified from container/heap.

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sim

func (q txqueue) init() {
	// heapify
	n := len(q)
	for i := n/2 - 1; i >= 0; i-- {
		q.down(i, n)
	}
}

func (q *txqueue) push(tx *Tx) {
	*q = append(*q, tx)
	q.up(len(*q) - 1)
}

func (q *txqueue) pop() *Tx {
	q_ := *q
	n := len(q_) - 1
	q_[0], q_[n] = q_[n], q_[0]
	q_.down(0, n)
	v := q_[n]
	*q = q_[:n]
	return v
}

func (q txqueue) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || q[j].FeeRate <= q[i].FeeRate {
			break
		}
		q[i], q[j] = q[j], q[i]
		j = i
	}
}

func (q txqueue) down(i, n int) {
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && q[j1].FeeRate <= q[j2].FeeRate {
			j = j2 // = 2*i + 2  // right child
		}
		if q[j].FeeRate <= q[i].FeeRate {
			break
		}
		q[i], q[j] = q[j], q[i]
		i = j
	}
}

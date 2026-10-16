// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
//
// Micro benchmarks plus the backlog's dense-graph Dijkstra driver:
// the same algorithm on index_pq (binary heap, O(log n) decrease-key)
// and on the Fibonacci heap (O(1) amortized decrease-key) — the
// workload where the Fibonacci heap is supposed to win.
package fib_heap

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/index_pq"
)

func BenchmarkInsert(b *testing.B) {
	q := NewIndexFibonacciPQ[int](b.N)
	b.ResetTimer()
	for i := range b.N {
		q.Insert(i, i)
	}
}

func BenchmarkPop(b *testing.B) {
	q := NewIndexFibonacciPQ[int](b.N)
	for i := range b.N {
		q.Insert(i, i)
	}
	b.ResetTimer()
	for range b.N {
		q.Pop()
	}
}

func BenchmarkDecreaseKey(b *testing.B) {
	const n = 1024
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(i, i)
	}
	b.ResetTimer()
	for i := range b.N {
		q.DecreaseKey(i%n, -i) // strictly decreasing on every revisit
	}
}

func BenchmarkChangeIncrease(b *testing.B) {
	const n = 1024
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(i, i)
	}
	// Consolidate into trees first so the increases hit the extract path
	// on deep nodes (pops remove indices 0..n/2-1).
	for range n / 2 {
		q.Pop()
	}
	b.ResetTimer()
	for i := range b.N {
		q.Change(n/2+i%(n/2), n+i) // increase-direction changes
	}
}

// pqDrive is the monomorphized driver interface: both queue kinds are
// wrapped in a thin struct so the measured loop has no interface
// dispatch (the sharded_hash benchmark pattern).
type pqDrive interface {
	contains(k int) bool
	insert(k int, v float64) bool
	decrease(k int, v float64) bool
	pop() (int, float64, bool)
}

type fibDrive struct{ q *IndexFibonacciPQ[float64] }

func (d fibDrive) contains(k int) bool          { return d.q.Contains(k) }
func (d fibDrive) insert(k int, v float64) bool { return d.q.Insert(k, v) }
func (d fibDrive) decrease(k int, v float64) bool {
	return d.q.DecreaseKey(k, v)
}
func (d fibDrive) pop() (int, float64, bool) { return d.q.Pop() }

type ipqDrive struct{ q *index_pq.IndexPQ[float64] }

func (d ipqDrive) contains(k int) bool          { return d.q.Contains(k) }
func (d ipqDrive) insert(k int, v float64) bool { return d.q.Insert(k, v) }
func (d ipqDrive) decrease(k int, v float64) bool {
	return d.q.Change(k, v) // index_pq's decrease-key
}
func (d ipqDrive) pop() (int, float64, bool) { return d.q.Pop() }

func dijkstraDense[P pqDrive](newQ func() P, w [][]float64, v int) float64 {
	const src = 0
	dist := make([]float64, v)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[src] = 0
	q := newQ()
	q.insert(src, 0)
	for {
		u, du, ok := q.pop()
		if !ok {
			break
		}
		if du > dist[u] {
			continue // stale entry
		}
		for x := range v {
			if x == u {
				continue
			}
			nd := du + w[u][x]
			if nd < dist[x] {
				dist[x] = nd
				if q.contains(x) {
					q.decrease(x, nd)
				} else {
					q.insert(x, nd)
				}
			}
		}
	}
	sum := 0.0
	for _, d := range dist {
		if !math.IsInf(d, 1) {
			sum += d
		}
	}
	return sum
}

// benchDenseGraph builds the shared dense weight matrix (every ordered
// pair connected, weight 1..1000) deterministically.
func benchDenseGraph(v int) [][]float64 {
	rng := rand.New(rand.NewPCG(42, 7))
	w := make([][]float64, v)
	for i := range w {
		w[i] = make([]float64, v)
		for j := range w[i] {
			w[i][j] = float64(1 + rng.IntN(1000))
		}
	}
	return w
}

// BenchmarkDijkstraFibHeap runs one full dense-graph Dijkstra per
// iteration on the Fibonacci heap.
func BenchmarkDijkstraFibHeap(b *testing.B) {
	const v = 400
	w := benchDenseGraph(v)
	b.ResetTimer()
	var sum float64
	for range b.N {
		sum = dijkstraDense(func() fibDrive { return fibDrive{NewIndexFibonacciPQ[float64](v)} }, w, v)
	}
	if sum <= 0 {
		b.Fatal("degenerate result")
	}
}

// BenchmarkDijkstraIndexPQ is the same driver on the binary-heap
// index_pq.
func BenchmarkDijkstraIndexPQ(b *testing.B) {
	const v = 400
	w := benchDenseGraph(v)
	b.ResetTimer()
	var sum float64
	for range b.N {
		sum = dijkstraDense(func() ipqDrive { return ipqDrive{index_pq.NewIndexPQ[float64](v)} }, w, v)
	}
	if sum <= 0 {
		b.Fatal("degenerate result")
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stable_pq

import (
	"testing"

	"github.com/pschlump/pluto/priority_queue"
)

// benchRegimes returns the input values for the insert benchmark: a
// small key range (heavy ties, where the stamp comparison actually
// runs) and a wide range (ties rare).
func benchInputs(rngSpread int) []int {
	const n = 4096
	vals := make([]int, n)
	for i := range vals {
		vals[i] = (i * 2654435761) % rngSpread // deterministic spread, no RNG in benchmarks
	}
	return vals
}

// BenchmarkInsert measures insert cost against pluto's plain
// priority_queue over the same underlying heap — the difference is the
// (key, seq) wrapper and its two-stage comparison.
func BenchmarkInsert(b *testing.B) {
	for _, spread := range []struct {
		name   string
		values []int
	}{
		{"ties-heavy", benchInputs(8)},
		{"wide-keys", benchInputs(1 << 30)},
	} {
		b.Run(spread.name, func(b *testing.B) {
			b.Run("stable", func(b *testing.B) {
				for range b.N {
					pq := NewStablePQ[int]()
					for _, v := range spread.values {
						pq.Insert(v)
					}
				}
			})
			b.Run("plain", func(b *testing.B) {
				for range b.N {
					pq := priority_queue.NewPriorityQueue[int]()
					for _, v := range spread.values {
						pq.Insert(v)
					}
				}
			})
		})
	}
}

// BenchmarkPop drains a pre-built queue; equal keys mean the stamp
// tie-break is exercised on every sift comparison.
func BenchmarkPop(b *testing.B) {
	values := benchInputs(8)
	for range b.N {
		pq := NewStablePQ[int]()
		for _, v := range values {
			pq.Insert(v)
		}
		for !pq.IsEmpty() {
			pq.Pop()
		}
	}
}

// BenchmarkPushPopCycle holds a steady-state queue and measures the
// amortized Insert+Pop pair.
func BenchmarkPushPopCycle(b *testing.B) {
	pq := NewStablePQ[int]()
	for _, v := range benchInputs(8) {
		pq.Insert(v)
	}
	i := 0
	for range b.N {
		pq.Insert(i * 7 % 8)
		pq.Pop()
		i++
	}
}

func BenchmarkPeek(b *testing.B) {
	pq := NewStablePQ[int]()
	for _, v := range benchInputs(8) {
		pq.Insert(v)
	}
	for range b.N {
		pq.Peek()
	}
}

// BenchmarkAll measures the non-destructive ordered snapshot (copy the
// heap, drain the copy).
func BenchmarkAll(b *testing.B) {
	pq := NewStablePQ[int]()
	for _, v := range benchInputs(8) {
		pq.Insert(v)
	}
	b.ReportAllocs()
	for range b.N {
		n := 0
		for range pq.All() {
			n++
		}
		if n != pq.Len() {
			b.Fatalf("All yielded %d elements, want %d", n, pq.Len())
		}
	}
}

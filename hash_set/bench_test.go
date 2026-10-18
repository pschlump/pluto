package hash_set

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"testing"
)

// Honest-constants benchmark, the fib_heap/multiway_pq tradition: a
// HashSet is a thin facade over hash_grow.HashTab, so the interesting
// question is what the facade + the auto-growing open-addressed engine
// cost against the Go runtime's built-in map[T]bool driven as a set —
// the tool every Go programmer already has.  The driver is the standard
// set workload: insert n distinct elements, then a mixed phase of hits
// and misses (lookup) and deletes interleaved with re-inserts, at
// n = 1,000 and n = 100,000.  map[T]bool] uses a bool value as the
// set marker (the `m[x] = true` idiom); HashSet stores nothing but the
// element.
//
// What to expect, and why it is not a scandal when map wins: the runtime
// map is one of the most tuned data structures in Go (SWISS tables,
// incremental growth, compiler-specialized hashing), while hash_grow is
// a textbook linear-probing table with a per-element function-call hash.
// The facade's batch and algebra methods are NOT measured here — they
// are plain loops over the core, so their constants follow these.
//
// The tree_set arm of this comparison (sorted-set semantics over the
// tree engine) is added by phase 7 of docs/set-capabilities-plan.md, in
// tree_set/bench_test.go.

const (
	benchSmallSize = 1_000
	benchLargeSize = 100_000
)

var benchSizes = []struct {
	name string
	n    int
}{
	{"n=1k", benchSmallSize},
	{"n=100k", benchLargeSize},
}

// benchKeys returns the distinct key universe: i*7919 spreads the keys
// over a wider integer range than 0..n so hashing does something.
func benchKeys(n int) []int {
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i * 7919
	}
	return keys
}

// runInsertAll measures building a set of n elements from scratch,
// b.N times.
func runInsertAll(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for range b.N {
		ss := NewHashSet[int]()
		for _, k := range keys {
			ss.Insert(k)
		}
	}
	b.StopTimer()
}

// runMixed measures the steady-state phase: for every key in order, one
// lookup (hit or miss depending on parity), a delete of the hits and a
// re-insert — the lookup/delete mix of a real workload.
func runMixed(b *testing.B, n int) {
	keys := benchKeys(n)
	ss := NewHashSet[int]()
	ss.InsertSlice(keys)
	misses := make([]int, n)
	for i := range misses {
		misses[i] = i*7919 + 1
	}
	b.ResetTimer()
	for range b.N {
		for i, k := range keys {
			if i%2 == 0 {
				ss.Contains(k)
			} else {
				ss.Delete(k)
				ss.Insert(misses[i])
			}
		}
		// restore the full population for the next iteration
		for _, k := range keys {
			ss.Insert(k)
		}
	}
	b.StopTimer()
}

func BenchmarkHashSetInsertAll(b *testing.B) {
	for _, sz := range benchSizes {
		b.Run(sz.name, func(b *testing.B) { runInsertAll(b, sz.n) })
	}
}

func BenchmarkHashSetMixed(b *testing.B) {
	for _, sz := range benchSizes {
		b.Run(sz.name, func(b *testing.B) { runMixed(b, sz.n) })
	}
}

// The map[T]bool driver — identical workloads over the runtime map.

func runMapInsertAll(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for range b.N {
		m := make(map[int]bool, n)
		for _, k := range keys {
			m[k] = true
		}
	}
	b.StopTimer()
}

func runMapMixed(b *testing.B, n int) {
	keys := benchKeys(n)
	m := make(map[int]bool, n)
	for _, k := range keys {
		m[k] = true
	}
	misses := make([]int, n)
	for i := range misses {
		misses[i] = i*7919 + 1
	}
	b.ResetTimer()
	for range b.N {
		for i, k := range keys {
			if i%2 == 0 {
				_ = m[k]
			} else {
				delete(m, k)
				m[misses[i]] = true
			}
		}
		for _, k := range keys {
			m[k] = true
		}
	}
	b.StopTimer()
}

func BenchmarkMapBoolInsertAll(b *testing.B) {
	for _, sz := range benchSizes {
		b.Run(sz.name, func(b *testing.B) { runMapInsertAll(b, sz.n) })
	}
}

func BenchmarkMapBoolMixed(b *testing.B) {
	for _, sz := range benchSizes {
		b.Run(sz.name, func(b *testing.B) { runMapMixed(b, sz.n) })
	}
}

// Sanity check that the benchmark drivers actually populate the set —
// a benchmark measuring empty structures lies.
func TestBenchDriversPopulate(b *testing.T) {
	for _, sz := range benchSizes {
		keys := benchKeys(sz.n)
		ss := NewHashSet[int]()
		for _, k := range keys {
			ss.Insert(k)
		}
		if ss.Len() != sz.n {
			b.Fatalf("size %d: set holds %d elements", sz.n, ss.Len())
		}
		m := make(map[int]bool, sz.n)
		for _, k := range keys {
			m[k] = true
		}
		if len(m) != sz.n {
			b.Fatalf("size %d: map holds %d elements", sz.n, len(m))
		}
	}
}

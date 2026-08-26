/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bench_compare measures the cost of the interface boxing and
// unboxing that charon/binary_tree removed relative to
// pluto/binary_tree.
//
// Three trees are benchmarked on identical workloads (same keys, same
// deterministic shuffled order, same operations):
//
//		pluto-interface  pluto/binary_tree.  The element type implements
//		                 comparable.Comparable, so every comparison boxes the
//		                 argument into an interface (a heap allocation — the
//		                 element is a struct containing a string), dispatches
//		                 through the interface, and unboxes with a type
//		                 assertion inside Compare.  Elements are stored as
//		                 *T and the API takes pointers.
//
//		charon-func      charon/binary_tree with the same struct shape as the
//		                 pluto element and a plain comparison function.  No
//		                 interface, no boxing, no type assertion; elements are
//		                 stored and passed by value.
//
//		charon-ordered   charon/binary_tree with plain string keys and the
//	                built-in < / > operators (NewBinaryTree).  No wrapper
//		                 struct exists at all — a shape pluto cannot express,
//		                 since a bare string cannot implement Comparable.
//
// The pluto-interface vs charon-func pair is the apples-to-apples
// comparison of the two designs; charon-ordered shows the additional win
// when the key type is naturally ordered.
//
// Run from this directory:
//
//	go test -run '^$' -bench . -benchmem
//
// For statistically robust numbers use multiple counts and benchstat:
//
//	go test -run '^$' -bench . -benchmem -count=10 > bench.txt
//	benchstat bench.txt
package bench_compare

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	cbtree "github.com/pschlump/charon/binary_tree"
	pbtree "github.com/pschlump/pluto/binary_tree"
	plutocomp "github.com/pschlump/pluto/comparable"
)

// -------------------------------------------------------------------------------------------------------
// Element types
// -------------------------------------------------------------------------------------------------------

// PlutoItem is the element type for the pluto tree.  It must implement
// comparable.Comparable, which is exactly the cost being measured: every
// comparison boxes the argument into an interface and Compare unboxes it
// with a type assertion.  The body mirrors pluto's own test element.
type PlutoItem struct {
	S string
}

// Compare implements comparable.Comparable.
func (a PlutoItem) Compare(x plutocomp.Comparable) int {
	b, ok := x.(PlutoItem)
	if !ok {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return strings.Compare(a.S, b.S)
}

// CharonItem has the same shape as PlutoItem but no methods at all;
// ordering is supplied to the tree as a plain function (cmpCharonItem).
type CharonItem struct {
	S string
}

func cmpCharonItem(a, b CharonItem) int {
	return strings.Compare(a.S, b.S)
}

// -------------------------------------------------------------------------------------------------------
// Workload setup — identical for every implementation
// -------------------------------------------------------------------------------------------------------

var benchSizes = []int{1_000, 10_000}

// shuffledKeys returns n zero-padded keys in a deterministic shuffled
// order, so both trees see exactly the same insertion order and the trees
// have the same shape (random-order input, not a degenerate chain).
func shuffledKeys(n int) []string {
	rng := rand.New(rand.NewSource(42))
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	rng.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	keys := make([]string, n)
	for i, v := range idx {
		keys[i] = fmt.Sprintf("%06d", v)
	}
	return keys
}

// Pre-built search/delete probes so the timed loops do not measure the
// construction of the probe value — only the tree operation itself.
// (pluto's API requires a *T probe, so the probes are addressable values.)
func plutoProbes(keys []string) []PlutoItem {
	probes := make([]PlutoItem, len(keys))
	for i, k := range keys {
		probes[i] = PlutoItem{S: k}
	}
	return probes
}

func buildPluto(keys []string) *pbtree.BinaryTree[PlutoItem] {
	tree := pbtree.NewBinaryTree[PlutoItem]()
	for _, k := range keys {
		tree.Insert(&PlutoItem{S: k})
	}
	return tree
}

func buildCharonFunc(keys []string) *cbtree.BinaryTree[CharonItem] {
	tree := cbtree.NewBinaryTreeFunc(cmpCharonItem)
	for _, k := range keys {
		tree.Insert(CharonItem{S: k})
	}
	return tree
}

func buildCharonOrdered(keys []string) *cbtree.BinaryTree[string] {
	tree := cbtree.NewBinaryTree[string]()
	for _, k := range keys {
		tree.Insert(k)
	}
	return tree
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

// BenchmarkInsert measures building a tree of n elements from the
// shuffled keys.  Each benchmark iteration builds the whole tree, so
// ns/op covers n inserts.
func BenchmarkInsert(b *testing.B) {
	for _, n := range benchSizes {
		keys := shuffledKeys(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.Run("pluto-interface", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					buildPluto(keys)
				}
			})
			b.Run("charon-func", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					buildCharonFunc(keys)
				}
			})
			b.Run("charon-ordered", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					buildCharonOrdered(keys)
				}
			})
		})
	}
}

// BenchmarkSearch measures n successful searches (every key, in the
// shuffled order) against a pre-built tree.
func BenchmarkSearch(b *testing.B) {
	for _, n := range benchSizes {
		keys := shuffledKeys(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.Run("pluto-interface", func(b *testing.B) {
				tree := buildPluto(keys)
				probes := plutoProbes(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for j := range probes {
						tree.Search(&probes[j])
					}
				}
			})
			b.Run("charon-func", func(b *testing.B) {
				tree := buildCharonFunc(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, k := range keys {
						tree.Search(CharonItem{S: k})
					}
				}
			})
			b.Run("charon-ordered", func(b *testing.B) {
				tree := buildCharonOrdered(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for _, k := range keys {
						tree.Search(k)
					}
				}
			})
		})
	}
}

// BenchmarkDelete measures deleting every element (in the shuffled
// order) from a tree that is rebuilt between timed iterations.  ns/op
// covers n deletes.
func BenchmarkDelete(b *testing.B) {
	for _, n := range benchSizes {
		keys := shuffledKeys(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.Run("pluto-interface", func(b *testing.B) {
				probes := plutoProbes(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					tree := buildPluto(keys)
					b.StartTimer()
					for j := range probes {
						tree.Delete(&probes[j])
					}
				}
			})
			b.Run("charon-func", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					tree := buildCharonFunc(keys)
					b.StartTimer()
					for _, k := range keys {
						tree.Delete(CharonItem{S: k})
					}
				}
			})
			b.Run("charon-ordered", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					tree := buildCharonOrdered(keys)
					b.StartTimer()
					for _, k := range keys {
						tree.Delete(k)
					}
				}
			})
		})
	}
}

// BenchmarkIterate measures a full in-order traversal with All() over a
// pre-built tree.  No comparisons are involved; this isolates the cost of
// pointer-chasing and yielding *T (pluto) versus values (charon).
func BenchmarkIterate(b *testing.B) {
	for _, n := range benchSizes {
		keys := shuffledKeys(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.Run("pluto-interface", func(b *testing.B) {
				tree := buildPluto(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for range tree.All() {
					}
				}
			})
			b.Run("charon-func", func(b *testing.B) {
				tree := buildCharonFunc(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for range tree.All() {
					}
				}
			})
			b.Run("charon-ordered", func(b *testing.B) {
				tree := buildCharonOrdered(keys)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for range tree.All() {
					}
				}
			})
		})
	}
}

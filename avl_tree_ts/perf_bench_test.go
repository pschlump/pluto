/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

// Performance benchmarks for the hot paths: genuine inserts, in-place
// replaces (the allocation count is the point), full and early-stop
// iteration, and the Search+Insert two-descent pattern that Upsert
// replaces (BenchmarkPerfUpsert lives in upsert_test.go).

import (
	"math/rand/v2"
	"testing"
)

const perfSize = 1024

// perfKeys returns perfSize distinct ints in a fixed shuffled order.
func perfKeys() []int {
	keys := make([]int, perfSize)
	for i := range keys {
		keys[i] = i * 2 // even keys: odd probes miss
	}
	r := rand.New(rand.NewPCG(1, 2))
	r.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	return keys
}

// perfTree returns an int tree holding perfKeys.
func perfTree() (*AvlTree[int], []int) {
	keys := perfKeys()
	tree := NewAvlTree[int]()
	for _, k := range keys {
		tree.Insert(k)
	}
	return tree, keys
}

// BenchmarkPerfInsertNew builds a perfSize tree from empty: one op is
// perfSize genuine inserts (perfSize node allocations is the floor).
func BenchmarkPerfInsertNew(b *testing.B) {
	keys := perfKeys()
	tree := NewAvlTree[int]()
	b.ReportAllocs()
	for b.Loop() {
		tree.Truncate()
		for _, k := range keys {
			tree.Insert(k)
		}
	}
}

// BenchmarkPerfReplace re-inserts keys already present: every Insert is a
// replace and should allocate nothing.
func BenchmarkPerfReplace(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		tree.Insert(keys[i%perfSize])
		i++
	}
}

// BenchmarkPerfSearchInsert is the two-descent "read old value, then
// write" pattern that Upsert replaces.
func BenchmarkPerfSearchInsert(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	sink := 0
	for b.Loop() {
		k := keys[i%perfSize]
		if old, ok := tree.Search(k); ok {
			sink += old
		}
		tree.Insert(k)
		i++
	}
	_ = sink
}

// BenchmarkPerfDeleteInsert deletes and re-inserts a present key.
func BenchmarkPerfDeleteInsert(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		k := keys[i%perfSize]
		tree.Delete(k)
		tree.Insert(k)
		i++
	}
}

// BenchmarkPerfAll ranges over every element.
func BenchmarkPerfAll(b *testing.B) {
	tree, _ := perfTree()
	b.ReportAllocs()
	sum := 0
	for b.Loop() {
		for v := range tree.All() {
			sum += v
		}
	}
	_ = sum
}

// BenchmarkPerfBackward ranges over every element in descending order.
func BenchmarkPerfBackward(b *testing.B) {
	tree, _ := perfTree()
	b.ReportAllocs()
	sum := 0
	for b.Loop() {
		for v := range tree.Backward() {
			sum += v
		}
	}
	_ = sum
}

// BenchmarkPerfAllEarlyStop takes only the first element.
func BenchmarkPerfAllEarlyStop(b *testing.B) {
	tree, _ := perfTree()
	b.ReportAllocs()
	sum := 0
	for b.Loop() {
		for v := range tree.All() {
			sum += v
			break
		}
	}
	_ = sum
}

// BenchmarkPerfWalkInOrder visits every element through the callback walk.
func BenchmarkPerfWalkInOrder(b *testing.B) {
	tree, _ := perfTree()
	b.ReportAllocs()
	sum := 0
	for b.Loop() {
		tree.WalkInOrder(func(_, _ int, v int) bool { sum += v; return true })
	}
	_ = sum
}

// BenchmarkPerfSlice materializes the tree.
func BenchmarkPerfSlice(b *testing.B) {
	tree, _ := perfTree()
	b.ReportAllocs()
	for b.Loop() {
		_ = tree.Slice()
	}
}

// BenchmarkPerfUnion rebuilds a destination from two operands.
func BenchmarkPerfUnion(b *testing.B) {
	t1, _ := perfTree()
	t2 := NewAvlTree[int]()
	for i := range perfSize {
		t2.Insert(i*2 + 1)
	}
	dst := NewAvlTree[int]()
	b.ReportAllocs()
	for b.Loop() {
		dst.Union(t1, t2)
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

// Benchmarks for the whole-tree set operations: each operation at a small
// (64) and a large (100k) operand size, with the two operands disjoint,
// overlapping by half, or identical.  One op rebuilds the destination.

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// setOpOperands returns two int trees of n elements each: yy holds
// 0..n-1 and zz holds the n keys starting at n-overlap, so exactly
// overlap keys are shared.  Both are built in a fixed shuffled order.
func setOpOperands(n, overlap int) (yy, zz *AvlTree[int]) {
	r := rand.New(rand.NewPCG(3, 4))
	build := func(lo int) *AvlTree[int] {
		keys := make([]int, n)
		for i := range keys {
			keys[i] = lo + i
		}
		r.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		t := NewAvlTree[int]()
		for _, k := range keys {
			t.Insert(k)
		}
		return t
	}
	return build(0), build(n - overlap)
}

// BenchmarkSetOp rebuilds a destination with each set operation.
func BenchmarkSetOp(b *testing.B) {
	type op struct {
		name string
		fn   func(dst, yy, zz *AvlTree[int])
	}
	ops := []op{
		{"Union", func(dst, yy, zz *AvlTree[int]) { dst.Union(yy, zz) }},
		{"Minus", func(dst, yy, zz *AvlTree[int]) { dst.Minus(yy, zz) }},
		{"Intersect", func(dst, yy, zz *AvlTree[int]) { dst.Intersect(yy, zz) }},
		{"SymDiff", func(dst, yy, zz *AvlTree[int]) { dst.SymmetricDifference(yy, zz) }},
	}
	for _, n := range []int{64, 100000} {
		b.Run(fmt.Sprintf("Copy/n=%d", n), func(b *testing.B) {
			yy, _ := setOpOperands(n, 0)
			dst := NewAvlTree[int]()
			b.ReportAllocs()
			for b.Loop() {
				dst.Copy(yy)
			}
		})
		for _, o := range ops {
			for _, pct := range []int{0, 50, 100} {
				b.Run(fmt.Sprintf("%s/n=%d/overlap=%d", o.name, n, pct), func(b *testing.B) {
					yy, zz := setOpOperands(n, n*pct/100)
					dst := NewAvlTree[int]()
					b.ReportAllocs()
					for b.Loop() {
						o.fn(dst, yy, zz)
					}
				})
			}
		}
	}
}

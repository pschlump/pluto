/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_tab_dll_ts

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (bucket-position, element) pairs, in bucket order and — within a
// bucket — from the most recently inserted element to the oldest.  Typical
// use:
//
//	for pos, item := range ht.All() { ... }
//
// As with dll/sll/hash_tab, a single-variable range yields the bucket
// position, not the element.  The iterator operates on a snapshot of the
// table copied under the read lock when All is called, so it is safe to
// call other table methods — including from inside the loop body.  Bucket
// order depends on the per-table hash seed, so it varies from process to
// process — never assert a fixed order.
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil table iterates as an empty one
	}
	type pair struct {
		pos  int
		item T
	}
	var snap []pair
	tt.lock.RLock()
	for i := range tt.buckets {
		for _, data := range tt.buckets[i].All() {
			snap = append(snap, pair{pos: i, item: data})
		}
	}
	tt.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for _, p := range snap {
			if !yield(p.pos, p.item) {
				return
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in the same order All yields them.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// The iterator operates on a snapshot of the table copied under the read
// lock when Values is called, so it is safe to call other table methods —
// including from inside the loop body.  The order depends on the per-table
// hash seed, so it varies from process to process — never assert a fixed
// order.
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil table iterates as an empty one
	}
	var snap []T
	tt.lock.RLock()
	for i := range tt.buckets {
		for _, data := range tt.buckets[i].All() {
			snap = append(snap, data)
		}
	}
	tt.lock.RUnlock()
	return func(yield func(T) bool) {
		for _, v := range snap {
			if !yield(v) {
				return
			}
		}
	}
}

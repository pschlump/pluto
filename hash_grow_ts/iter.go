package hash_grow_ts

/*
Copyright (C) Philip Schlump, 2023.

BSD 3 Clause Licensed. See ../LICENSE
*/

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (bucket-position, element) pairs, in bucket order.  Typical use:
//
//	for pos, item := range ht.All() { ... }
//
// The iterator operates on a snapshot of the table taken under the read lock
// when iteration starts, so it is safe to call other table methods from the
// loop body.
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		type pair struct {
			pos  int
			item *T
		}
		var snap []pair
		tt.lock.RLock()
		for i, v := range tt.buckets {
			if v != nil {
				snap = append(snap, pair{pos: i, item: v})
			}
		}
		tt.lock.RUnlock()
		for _, p := range snap {
			if !yield(p.pos, p.item) {
				return
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in bucket order.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// The iterator operates on a snapshot of the table taken under the read lock
// when iteration starts, so it is safe to call other table methods from the
// loop body.
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		var snap []*T
		tt.lock.RLock()
		for _, v := range tt.buckets {
			if v != nil {
				snap = append(snap, v)
			}
		}
		tt.lock.RUnlock()
		for _, v := range snap {
			if !yield(v) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */

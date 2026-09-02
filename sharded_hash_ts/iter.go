/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of
// the table as (position, element) pairs — position 0 for the first element
// visited, counting up in stripe order and bucket order within each stripe
// (the same order Walk uses; unlike hash_grow there is no per-table bucket
// position to report, a striped table's buckets are per-stripe and grow
// independently).  Typical use:
//
//	for pos, item := range h.All() { ... }
//
// As with dll/sll, a single-variable range yields the position, not the
// element.  The iterator operates on a snapshot copied stripe by stripe
// under each stripe's read lock when All is called, so it is safe to call
// other table methods from the loop body.  Each stripe's elements are a
// point-in-time view; stripes are not mutually consistent (inherent to
// sharding — there is no global lock to make them so).  Visit order depends
// on the hash seed and the per-stripe growth history, so it varies from
// process to process — never assert a fixed order.
// Complexity is O(n).
func (tt *ShardedHash[T]) All() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil table iterates as an empty one
	}
	type pair struct {
		pos  int
		item T
	}
	var snap []pair
	pos := 0
	for _, s := range tt.stripes {
		s.lock.RLock()
		for _, head := range s.tab.heads {
			for n := head; n != nil; n = n.next {
				snap = append(snap, pair{pos: pos, item: n.data})
				pos++
			}
		}
		s.lock.RUnlock()
	}
	return func(yield func(int, T) bool) {
		for _, p := range snap {
			if !yield(p.pos, p.item) {
				return
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in the same stripe-then-bucket order All uses.  Typical use:
//
//	for item := range h.Values() { ... }
//
// The iterator operates on a snapshot copied stripe by stripe under each
// stripe's read lock when Values is called, so it is safe to call other
// table methods from the loop body; see All for the per-stripe consistency
// caveat.  Visit order varies from process to process — never assert a
// fixed order.
// Complexity is O(n).
func (tt *ShardedHash[T]) Values() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil table iterates as an empty one
	}
	var snap []T
	for _, s := range tt.stripes {
		s.lock.RLock()
		for _, head := range s.tab.heads {
			for n := head; n != nil; n = n.next {
				snap = append(snap, n.data)
			}
		}
		s.lock.RUnlock()
	}
	return func(yield func(T) bool) {
		for _, v := range snap {
			if !yield(v) {
				return
			}
		}
	}
}

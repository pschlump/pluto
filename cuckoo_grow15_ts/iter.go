/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo_grow15_ts

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (slot-position, element) pairs, in slot order.  Typical use:
//
//	for pos, item := range ht.All() { ... }
//
// As with dll/sll, a single-variable range yields the slot position, not the
// element.  The iterator operates on a snapshot of the table copied under
// the read lock when All is called, so it is safe to call other table
// methods — and for the background resizer to rebuild the table — from the
// loop body.  Slot order depends on the hash seed and the displacement
// history, so it must not appear in fixed assertions.
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
	for i := range tt.slots {
		if tt.slots[i].used {
			snap = append(snap, pair{pos: i, item: tt.slots[i].data})
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
// the table, in slot order.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// The iterator operates on a snapshot of the table copied under the read
// lock when Values is called, so it is safe to call other table methods —
// and for the background resizer to rebuild the table — from the loop body.
// Slot order depends on the hash seed and the displacement history, so it
// must not appear in fixed assertions.
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil table iterates as an empty one
	}
	var snap []T
	tt.lock.RLock()
	for i := range tt.slots {
		if tt.slots[i].used {
			snap = append(snap, tt.slots[i].data)
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

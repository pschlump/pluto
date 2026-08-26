/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (slot-position, element) pairs, in slot order.  Typical use:
//
//	for pos, item := range ht.All() { ... }
//
// As with dll/sll, a single-variable range yields the slot position, not the
// element.  Slot order depends on the hash seed and the displacement history,
// so it must not appear in fixed assertions.  The iterator walks the live
// table — mutate the table only from inside the loop at your own risk (the
// cuckoo_ts twin iterates a snapshot).
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.slots {
			if tt.slots[i].used {
				if !yield(i, tt.slots[i].data) {
					return
				}
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in slot order.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// Slot order depends on the hash seed and the displacement history, so it
// must not appear in fixed assertions.  The iterator walks the live table
// (the cuckoo_ts twin iterates a snapshot).
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.slots {
			if tt.slots[i].used {
				if !yield(tt.slots[i].data) {
					return
				}
			}
		}
	}
}

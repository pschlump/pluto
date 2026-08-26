/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_tab_bt

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (bucket-position, element) pairs, in bucket order and — within a
// bucket — in the tree's in-order (ascending per the comparison function).
// Typical use:
//
//	for pos, item := range ht.All() { ... }
//
// As with dll/sll/hash_tab, a single-variable range yields the bucket
// position, not the element.  Bucket order depends on the hash function
// (a random seed for NewHashTab), so it varies from process to process —
// never assert a fixed order.
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.buckets {
			for data := range tt.buckets[i].All() {
				if !yield(i, data) {
					return
				}
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in the same order All yields them.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// The order depends on the hash function (a random seed for NewHashTab),
// so it varies from process to process — never assert a fixed order.
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.buckets {
			for data := range tt.buckets[i].All() {
				if !yield(data) {
					return
				}
			}
		}
	}
}

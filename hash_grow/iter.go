/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow

import "iter"

// All returns an iterator (Go 1.23 range-over-func) over the elements of the
// table as (bucket-position, element) pairs, in bucket order.  Typical use:
//
//	for pos, item := range ht.All() { ... }
//
// As with dll/sll, a single-variable range yields the bucket position, not
// the element.  Bucket order depends on the per-table hash seed, so it
// varies from process to process — never assert a fixed order.
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.buckets {
			if tt.originalHash[i] != 0 {
				if !yield(i, tt.buckets[i]) {
					return
				}
			}
		}
	}
}

// Values returns an iterator (Go 1.23 range-over-func) over the elements of
// the table, in bucket order.  Typical use:
//
//	for item := range ht.Values() { ... }
//
// Bucket order depends on the per-table hash seed, so it varies from process
// to process — never assert a fixed order.
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt == nil {
			return
		}
		for i := range tt.buckets {
			if tt.originalHash[i] != 0 {
				if !yield(tt.buckets[i]) {
					return
				}
			}
		}
	}
}

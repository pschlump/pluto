package hash_grow

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
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		for i, v := range tt.buckets {
			if v != nil {
				if !yield(i, v) {
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
// Complexity is O(n).
func (tt *HashTab[T]) Values() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for _, v := range tt.buckets {
			if v != nil {
				if !yield(v) {
					return
				}
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */

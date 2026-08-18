package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"iter"
)

// All returns a range-over-func iterator (Go 1.23+) over every element in
// the table.  The index is the position in iteration order (bucket order,
// head to tail within each bucket), which is not significant.  Iteration
// stops early if the loop body breaks.
//
//	for i, v := range ht.All() {
//		...
//	}
//
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		i := 0
		for _, b := range tt.buckets {
			for _, v := range b.IteratePtr() {
				if !yield(i, v) {
					return
				}
				i++
			}
		}
	}
}

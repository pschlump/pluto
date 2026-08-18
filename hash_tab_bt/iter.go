package hash_tab

import "iter"

// WalkFunc calls `fx` on every element of the table.  Iteration order is
// bucket order, not sorted order.
// Complexity is O(n).
func (tt *HashTab[T]) WalkFunc(fx func(a *T)) {
	for i := 0; i < tt.size; i++ {
		if tt.buckets[i] != nil {
			tt.buckets[i].WalkFunc(fx)
		}
	}
}

// All returns an iterator (a Go 1.23 range-over-func sequence) over every
// element of the table:
//
//	for item := range ht.All() { ... }
//
// Iteration order is bucket order, not sorted order.  The table must not be
// modified while iterating.
// Complexity is O(n) for a complete iteration.
func (tt *HashTab[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		done := false
		for _, b := range tt.buckets {
			if done {
				return
			}
			b.WalkFunc(func(a *T) {
				if !done && !yield(a) {
					done = true
				}
			})
		}
	}
}

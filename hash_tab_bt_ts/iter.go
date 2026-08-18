package hash_tab_bt_ts

import "iter"

// WalkFunc calls `fx` on every element of the table.  Iteration order is
// bucket order, not sorted order.  The table is read-locked for the
// duration of the walk; `fx` must not call back into the table or it will
// deadlock.
// Complexity is O(n).
func (tt *HashTab[T]) WalkFunc(fx func(a *T)) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
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
// A consistent snapshot of the table is taken under a read lock when All
// is called; the iteration itself runs without holding any lock, so it is
// safe to call other table methods from inside the loop.  Iteration order
// is bucket order, not sorted order.
// Complexity is O(n).
func (tt *HashTab[T]) All() iter.Seq[*T] {
	tt.lock.RLock()
	items := make([]*T, 0, tt.length)
	for _, b := range tt.buckets {
		b.WalkFunc(func(a *T) {
			items = append(items, a)
		})
	}
	tt.lock.RUnlock()
	return func(yield func(*T) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	}
}

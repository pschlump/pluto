/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bag_ts

import "iter"

// snapshot collects the chain into a slice under the read lock.  The
// All/Backward snapshot convention: the copy is taken at call time, so
// the returned iterator never observes later modifications.
func (b *Bag[T]) snapshot() []T {
	b.lock.RLock()
	defer b.lock.RUnlock()
	snapshot := make([]T, 0, b.length)
	for p := b.head; p != nil; p = p.next {
		snapshot = append(snapshot, p.data)
	}
	return snapshot
}

// All returns a range-over-func iterator that yields the index and value
// of each element in the bag, in insertion order (element 0 is the first
// element added).
//
//	for i, v := range b.All() {
//		...
//	}
//
// The iterator operates on a snapshot copied under the read lock when All
// is called, so it is safe to call other bag operations — including from
// inside the loop — and it never observes later modifications.
// Complexity is O(n).
func (b *Bag[T]) All() iter.Seq2[int, T] {
	if b == nil {
		return func(func(int, T) bool) {} // a nil bag iterates as an empty one
	}
	snapshot := b.snapshot()
	return func(yield func(int, T) bool) {
		for i, v := range snapshot {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and
// value of each element in the bag, from the most recently added to the
// first added.
//
//	for i, v := range b.Backward() {
//		...
//	}
//
// The index counts down from Length()-1 to 0, so it matches the index
// that All assigns to the same element.
//
// The iterator operates on a snapshot copied under the read lock when
// Backward is called (the All/Backward snapshot convention), so it is
// safe to call other bag operations — including from inside the loop —
// and it never observes later modifications.
// Complexity is O(n).
func (b *Bag[T]) Backward() iter.Seq2[int, T] {
	if b == nil {
		return func(func(int, T) bool) {} // a nil bag iterates as an empty one
	}
	snapshot := b.snapshot()
	return func(yield func(int, T) bool) {
		for i := len(snapshot) - 1; i >= 0; i-- {
			if !yield(i, snapshot[i]) {
				return
			}
		}
	}
}

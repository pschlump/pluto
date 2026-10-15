/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators over the live list: All follows the next
// pointers from the head (insertion order, element 0 = first added).
// The list is singly linked — there are no back pointers — so Backward
// collects the chain into a slice under the walk and yields it
// reversed, the skip_list Backward convention (O(n) extra space; All
// itself is O(1) extra space).  Neither iterator copies a snapshot
// ahead of time: the bag must not be modified while an iterator is
// running.

package bag

import "iter"

// All returns a range-over-func iterator that yields the index and value
// of each element in the bag, in insertion order (element 0 is the first
// element added).
//
//	for i, v := range b.All() {
//		...
//	}
//
// The bag must not be modified while the iterator is running.
// Complexity is O(n).
func (b *Bag[T]) All() iter.Seq2[int, T] {
	if b == nil {
		return func(func(int, T) bool) {} // a nil bag iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		i := 0
		for p := b.head; p != nil; p = p.next {
			if !yield(i, p.data) {
				return
			}
			i++
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
// that All assigns to the same element.  There are no back pointers, so
// the chain is collected and yielded reversed (O(n) extra space — the
// skip_list Backward convention).
//
// The bag must not be modified while the iterator is running.
// Complexity is O(n).
func (b *Bag[T]) Backward() iter.Seq2[int, T] {
	if b == nil {
		return func(func(int, T) bool) {} // a nil bag iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		chain := make([]T, 0, b.length)
		for p := b.head; p != nil; p = p.next {
			chain = append(chain, p.data)
		}
		for i := len(chain) - 1; i >= 0; i-- {
			if !yield(i, chain[i]) {
				return
			}
		}
	}
}

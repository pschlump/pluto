/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package patricia_trie

import "iter"

// All returns a range-over-func iterator (iter.Seq2) that visits every
// (key, value) pair of the trie in ascending key order:
//
//	for key, value := range pt.All() { ... }
//
// A single-variable range yields the key, not the value (use the
// two-variable form above).  A nil *PatriciaTrie iterates as an empty
// one.
//
// Ascending order falls out of the trie's shape: an in-order traversal
// visits the 0-side of every branch before the 1-side, which is
// ascending encoded-bit order — that is, ascending byte order with
// shorter prefixes first.
//
// The iterator walks the live trie: the trie must not be modified while
// the iterator is being consumed.
// Complexity is O(n) for a full traversal.
func (t *PatriciaTrie[T]) All() iter.Seq2[string, T] {
	if t == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	return func(yield func(string, T) bool) {
		var walk func(x *patriciaNode[T]) bool
		walk = func(x *patriciaNode[T]) bool {
			if x.bit < 0 {
				return yield(x.key, x.value)
			}
			return walk(x.child[0]) && walk(x.child[1])
		}
		if t.root != nil {
			walk(t.root)
		}
	}
}

// Backward returns a range-over-func iterator (iter.Seq2) that visits
// every (key, value) pair of the trie in descending key order — the
// mirror image of All.  A nil *PatriciaTrie iterates as an empty one.
//
// The iterator walks the live trie: the trie must not be modified while
// the iterator is being consumed.
// Complexity is O(n) for a full traversal.
func (t *PatriciaTrie[T]) Backward() iter.Seq2[string, T] {
	if t == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	return func(yield func(string, T) bool) {
		var walk func(x *patriciaNode[T]) bool
		walk = func(x *patriciaNode[T]) bool {
			if x.bit < 0 {
				return yield(x.key, x.value)
			}
			return walk(x.child[1]) && walk(x.child[0])
		}
		if t.root != nil {
			walk(t.root)
		}
	}
}

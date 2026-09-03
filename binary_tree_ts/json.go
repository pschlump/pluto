/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_tree_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a BinaryTree can be used
// directly with the encoding/json package.  The tree is encoded as a JSON
// array of its elements in in-order (ascending) order — the tree's natural
// iteration order, the same order All and Front produce.
//
// The elements are snapshotted under the read lock (the All snapshot
// convention) and the encoding itself runs without the lock, so this is
// safe to call concurrently with any tree operation — and an element type
// with its own MarshalJSON may safely call back into the tree.  Errors
// from the json package are returned unchanged (for example a T that
// cannot be encoded at all, such as a channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also encodes
// as [] (the "nil behaves as an empty tree" read contract); note that
// json.Marshal on a nil *BinaryTree never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *BinaryTree[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(tt.snapshotInOrder()) // snapshot takes and releases the read lock itself
}

// UnmarshalJSON implements json.Unmarshaler so a BinaryTree can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the tree —
// they are inserted in array order under one hold of the write lock.  The
// comparison function is kept, so the tree stays usable after
// unmarshaling.  Because the tree is not self-balancing, the shape of the
// rebuilt tree depends on the order of the elements in data (in-order
// output produces a right-leaning chain); this affects performance, not
// correctness.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the tree
// lock — and a decode error (malformed JSON, a non-array document, wrong
// element types) is returned with the tree untouched.  The nil/cmp guards
// are also checked before the lock is acquired (cmp is set only by the
// constructors, so reading it unlocked is safe).  Elements decoded as
// duplicates of one another collapse, exactly as repeated Insert calls
// would.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil tree or a zero-value tree (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the tree and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n log₂ n) on average, O(n²) in the worst case (the tree
// is NOT self-balancing), plus the cost of decoding the elements.
func (tt *BinaryTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Insert precedent: a nil tree cannot store an element).
	if len(items) > 0 {
		if tt == nil {
			panic("binary_tree_ts: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("binary_tree_ts: UnmarshalJSON called on a tree with no comparison function (create the tree with NewBinaryTree or NewBinaryTreeFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.root = nil
	tt.length = 0
	for _, d := range items {
		tt.nlInsert(d)
	}
	return nil
}

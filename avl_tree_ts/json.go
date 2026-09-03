/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so an AvlTree can be used directly
// with the encoding/json package.  The tree is encoded as a JSON array of
// its elements in in-order (sorted) sequence — the tree's natural
// iteration order — so the output is deterministic regardless of insertion
// order.
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any tree operation — and an element type with its own
// MarshalJSON may safely call back into the tree (the All/Backward
// snapshot convention).  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also
// encodes as [] (the "nil behaves as an empty tree" read contract);
// note that json.Marshal on a nil *AvlTree never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *AvlTree[T]) MarshalJSON() ([]byte, error) {
	items, _ := tt.snapshot() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil tree marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so an AvlTree can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// tree in in-order (sorted) sequence under one hold of the write lock.
// The comparison function is kept, so the tree stays usable after
// unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// tree lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the tree untouched.  The
// nil/comparison guards are also checked before the lock is acquired
// (cmp is set only by the constructors, so reading it unlocked is safe).
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil tree or a zero-value tree (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the tree and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log₂ n) plus the cost of decoding the elements.
func (tt *AvlTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Insert precedent).
	if len(items) > 0 {
		if tt == nil {
			panic("avl_tree_ts: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("avl_tree_ts: UnmarshalJSON called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.nlTruncate()
	for _, d := range items {
		tt.nlInsert(d)
	}
	return nil
}

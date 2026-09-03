/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so an AvlTree can be used directly
// with the encoding/json package.  The tree is encoded as a JSON array of
// its elements in in-order (ascending) sequence — the same order as All,
// Front and WalkInOrder — so the encoded form is canonical: two trees with
// the same contents encode identically regardless of insertion order.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the tree
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also encodes
// as [] (the "nil behaves as an empty tree" read contract); note that
// json.Marshal on a nil *AvlTree never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *AvlTree[T]) MarshalJSON() ([]byte, error) {
	items := tt.toSlice() // in-order snapshot; nil for a nil or empty tree
	if items == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so an AvlTree can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the tree.
// The elements are inserted in array order — after rebalancing the tree
// ends up sorted in in-order sequence regardless of that order — and a
// duplicate of an already decoded element replaces it, as Insert does.
// The comparison function is kept, so the tree stays usable after
// unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// happens before anything is mutated: a decode error (malformed JSON, a
// non-array document, wrong element types) is returned and leaves the
// tree untouched.
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
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("avl_tree: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("avl_tree: UnmarshalJSON called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate() // keeps the comparison function
	for _, d := range items {
		tt.insert(d) // the guards above made insert safe
	}
	return nil
}

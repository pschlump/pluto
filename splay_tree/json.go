/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package splay_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so a SplayTree can be used
// directly with the encoding/json package.  The tree is encoded as a JSON
// array of its elements in in-order (ascending) order — the same order
// the All iterator yields.  The elements are collected without splaying,
// so marshaling does not restructure the tree.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the tree
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also encodes
// as [] (the "nil behaves as an empty tree" read contract); note that
// json.Marshal on a nil *SplayTree never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *SplayTree[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	for _, v := range tt.All() {
		items = append(items, v)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a SplayTree can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the tree in
// order, so a document written by MarshalJSON rebuilds the same set of
// elements.  The comparison function is kept, so the tree stays usable
// after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The document is
// fully decoded before anything is stored, so a decode error (malformed
// JSON, a non-array document, wrong element types) is returned and leaves
// the tree untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil tree or a zero-value tree (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the tree and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log₂ n) amortized for the inserts, plus the cost of
// decoding the elements.
func (tt *SplayTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("splay_tree: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("splay_tree: UnmarshalJSON called on a tree with no comparison function (create the tree with NewSplayTree or NewSplayTreeFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate()
	for _, d := range items {
		tt.Insert(d)
	}
	return nil
}

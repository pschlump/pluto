/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package rb_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so an RbTree can be used directly
// with the encoding/json package.  The tree is encoded as a JSON array of
// its elements in ascending (in-order) sequence — the same order the All
// iterator yields — with the smallest element at index 0.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the tree
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also encodes
// as [] (the "nil behaves as an empty tree" read contract); note that
// json.Marshal on a nil *RbTree never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *RbTree[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	for cur := minNode(tt.root); cur != nil; cur = successor(cur) {
		items = append(items, cur.data)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so an RbTree can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the tree by
// inserting them in array order (ascending order for data produced by
// MarshalJSON — sorted insertion keeps a red-black tree balanced, unlike
// an unbalanced BST).  Elements that compare equal to one another follow
// the Insert rule: the later one replaces the earlier.  The comparison
// function is kept, so the tree stays usable after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode error
// (malformed JSON, a non-array document, wrong element types) is returned
// and leaves the tree untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil tree or a zero-value tree (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the tree and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log₂ n) plus the cost of decoding the elements.
func (tt *RbTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("rb_tree: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("rb_tree: UnmarshalJSON called on a tree with no comparison function (create the tree with NewRbTree or NewRbTreeFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate()
	for _, item := range items {
		tt.Insert(item)
	}
	return nil
}

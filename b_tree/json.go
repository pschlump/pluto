/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package b_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so a BTree can be used directly
// with the encoding/json package.  The tree is encoded as a JSON array
// of its elements in ascending (sorted) order — the order of the All
// iterator.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the tree
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also
// encodes as [] (the "nil behaves as an empty tree" read contract);
// note that json.Marshal on a nil *BTree never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *BTree[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	for _, item := range tt.All() {
		items = append(items, item)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a BTree can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// tree, inserted in array order — the tree is an ordered set, so
// iteration afterwards is ascending regardless of the input order.  The
// comparison function and the order are kept, so the tree stays usable
// after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  data is
// decoded before anything is mutated: a decode error (malformed JSON, a
// non-array document, wrong element types) is returned and leaves the
// tree untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil tree or a zero-value tree (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the tree and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log n) plus the cost of decoding the elements.
func (tt *BTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the dll precedent).
	if len(items) > 0 {
		if tt == nil {
			panic("b_tree: UnmarshalJSON called on a nil tree")
		}
		if tt.cmp == nil {
			panic("b_tree: UnmarshalJSON called on a tree with no comparison function (create the tree with NewBTree or NewBTreeFunc)")
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

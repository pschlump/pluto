/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package segment_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so a SegmentTree can be used
// directly with the encoding/json package.  The tree is encoded as a
// JSON array of the per-slot values: element 0 is Value(0), element 1
// is Value(1), and so on through Value(n-1) — the same numbers the
// constructors take, not the internal 2*size combine array.
//
// The values are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the tree
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty tree — a nil *SegmentTree or the zero value — encodes as [];
// note that json.Marshal on a nil *SegmentTree never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (st *SegmentTree[T]) MarshalJSON() ([]byte, error) {
	if st == nil || st.n == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(st.tree[st.size : st.size+st.n])
}

// UnmarshalJSON implements json.Unmarshaler so a SegmentTree can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded values replace the current contents —
// element i becomes the new Value(i) — and the slot count becomes the
// array length, exactly as if NewSegmentTreeFunc had been called on the
// decoded slice.  The combine function and identity are kept, so the
// tree stays fully usable after unmarshaling.
//
// The values are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-array document, wrong element types) is
// returned and leaves the tree untouched.
//
// Unmarshaling stores elements, so it follows the package's write
// contract: data with elements panics on a nil *SegmentTree (a nil
// structure cannot record an update) or on the zero value (no combine
// function to rebuild the tree with).  An empty array or null clears
// the tree to zero slots and is tolerated everywhere — it stores
// nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (st *SegmentTree[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	// The write contract only fires when an element would actually be
	// stored.
	if len(values) > 0 {
		if st == nil {
			panic("segment_tree: UnmarshalJSON called on a nil SegmentTree")
		}
		if st.combine == nil {
			panic("segment_tree: UnmarshalJSON called on a SegmentTree with no combine function (create the tree with NewSegmentTree or NewSegmentTreeFunc)")
		}
	}
	if st == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// Rebuild exactly as NewSegmentTreeFunc does: the decoded values are
	// the leaves, the internal nodes are re-derived bottom-up.
	n := len(values)
	size := 0
	var tree []T
	if n > 0 {
		size = 1
		for size < n {
			size *= 2
		}
		tree = make([]T, 2*size)
		for i := range tree {
			tree[i] = st.identity
		}
		copy(tree[size:], values)
		for k := size - 1; k >= 1; k-- {
			tree[k] = st.combine(tree[2*k], tree[2*k+1])
		}
	}
	st.tree = tree
	st.n = n
	st.size = size
	return nil
}

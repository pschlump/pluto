/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package segment_tree_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a SegmentTree can be used
// directly with the encoding/json package.  The tree is encoded as a
// JSON array of its slot values, slot 0 first — the underlying element
// array, not the internal 2*size layout, so a round trip rebuilds an
// equivalent tree.
//
// The slot values are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any tree operation — and an element type with its own
// MarshalJSON may safely call back into the tree.  Errors from the
// json package are returned unchanged (for example a T that cannot be
// encoded at all, such as a channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree also
// encodes as [] (the "nil behaves as an empty tree" read contract);
// note that json.Marshal on a nil *SegmentTree never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (st *SegmentTree[T]) MarshalJSON() ([]byte, error) {
	if st == nil {
		return []byte("[]"), nil
	}
	st.lock.RLock()
	items := make([]T, st.n)
	copy(items, st.tree[st.size:st.size+st.n])
	st.lock.RUnlock()
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a SegmentTree can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded values replace the current contents of
// the tree in order — element 0 becomes the value at slot 0 — under
// one hold of the write lock.  The tree is resized to the decoded
// length and the internal nodes are rebuilt, exactly as the
// constructors build them; the combine function and identity element
// are kept, so the tree stays usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// tree lock — and a decode error (malformed JSON, a non-array
// document, wrong element types) is returned with the tree untouched.
// The nil/combine guards are also checked before the lock is acquired
// (combine is set only by the constructors, so reading it unlocked is
// safe).
//
// Unmarshaling stores elements, so it follows the insert contract:
// data that would store an element into a nil tree or a zero-value
// tree (no combine function) panics with the standard insert-family
// message.  An empty array or null clears the tree to empty (keeping
// the combine function and identity) and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (st *SegmentTree[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if st == nil {
			panic("segment_tree_ts: UnmarshalJSON called on a nil SegmentTree")
		}
		if st.combine == nil {
			panic("segment_tree_ts: UnmarshalJSON called on a segment tree with no combine function (create the tree with NewSegmentTree or NewSegmentTreeFunc)")
		}
	}
	if st == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	st.lock.Lock()
	defer st.lock.Unlock()
	if len(items) == 0 {
		st.tree = nil
		st.n = 0
		st.size = 0
		return nil
	}
	n := len(items)
	size := 1
	for size < n {
		size *= 2
	}
	tree := make([]T, 2*size)
	for i := range tree {
		tree[i] = st.identity
	}
	copy(tree[size:], items)
	for k := size - 1; k >= 1; k-- {
		tree[k] = st.combine(tree[2*k], tree[2*k+1])
	}
	st.tree = tree
	st.n = n
	st.size = size
	return nil
}

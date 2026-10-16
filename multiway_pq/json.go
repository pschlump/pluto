/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq

import "encoding/json"

// MarshalJSON implements json.Marshaler so a DHeap can be used directly
// with the encoding/json package.  The heap is encoded as a JSON array
// of its elements in internal heap (breadth-first tree) order — the same
// order All iterates in; this is NOT sorted order (repeatedly calling
// Pop is the way to consume a heap in sorted order).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the heap
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty heap encodes as [].  A direct call on a nil heap also
// encodes as [] (the "nil behaves as an empty heap" read contract);
// note that json.Marshal on a nil *DHeap never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (dh *DHeap[T]) MarshalJSON() ([]byte, error) {
	if dh == nil || len(dh.data) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(dh.data)
}

// UnmarshalJSON implements json.Unmarshaler so a DHeap can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// heap, rebuilt as a valid heap in one bottom-up Heapify sweep — O(n),
// unlike pushing the elements one at a time.  The comparison function
// and branching factor are kept, so the heap stays usable after
// unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-array document, wrong element types) is
// returned and leaves the heap untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil heap or a zero-value heap (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the heap and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (dh *DHeap[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Push precedent: reads of a nil or zero-value heap are
	// tolerated, stores are not).
	if len(items) > 0 {
		if dh == nil {
			panic("multiway_pq: UnmarshalJSON called on a nil heap")
		}
		if dh.cmp == nil {
			panic("multiway_pq: UnmarshalJSON called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)")
		}
	}
	if dh == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	dh.Truncate()
	if len(items) > 0 {
		dh.AppendHeap(items)
		for i := (len(items) - 2) / dh.d; i >= 0; i-- {
			dh.Heapify(len(items), i)
		}
	}
	return nil
}

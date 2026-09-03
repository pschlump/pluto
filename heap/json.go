/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package heap

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Heap can be used directly
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
// note that json.Marshal on a nil *Heap never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (hp *Heap[T]) MarshalJSON() ([]byte, error) {
	if hp == nil || len(hp.data) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(hp.data)
}

// UnmarshalJSON implements json.Unmarshaler so a Heap can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// heap, pushed in array order so the result is a valid heap.  The
// comparison function is kept, so the heap stays usable after
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
// Complexity is O(n log n) plus the cost of decoding the elements.
func (hp *Heap[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Push precedent: reads of a nil or zero-value heap are
	// tolerated, stores are not).
	if len(items) > 0 {
		if hp == nil {
			panic("heap: UnmarshalJSON called on a nil heap")
		}
		if hp.cmp == nil {
			panic("heap: UnmarshalJSON called on a heap with no comparison function (create the heap with NewHeap or NewHeapFunc)")
		}
	}
	if hp == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	hp.Truncate()
	for _, x := range items {
		hp.Push(x)
	}
	return nil
}

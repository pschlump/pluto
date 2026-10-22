/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a DHeap can be used directly
// with the encoding/json package.  The heap is encoded as a JSON array
// of its elements in internal heap (breadth-first tree) order — the
// same order All iterates in; this is NOT sorted order (repeatedly
// calling Pop is the way to consume a heap in sorted order).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any heap operation — and an element type with its own
// MarshalJSON may safely call back into the heap (the All snapshot
// convention).  Errors from the json package are returned unchanged
// (for example a T that cannot be encoded at all, such as a channel or
// a function).
//
// An empty heap encodes as [].  A direct call on a nil heap also
// encodes as [] (the "nil behaves as an empty heap" read contract);
// note that json.Marshal on a nil *DHeap never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (dh *DHeap[T]) MarshalJSON() ([]byte, error) {
	items := dh.snapshot() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil heap marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a DHeap can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// heap, rebuilt as a valid heap in one bottom-up Heapify sweep — O(n),
// unlike pushing the elements one at a time.  The comparison function
// and branching factor are kept, so the heap stays usable after
// unmarshaling.
//
// The elements are decoded with NO lock held (a decode error must
// leave the heap untouched); the replace + rebuild then runs under ONE
// hold of the write lock — atomic to concurrent readers (the
// heap_sort_ts NlAppendHeap/NlHeapify precedent).
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
		return err // decode failed: the heap is untouched, no lock was ever held
	}

	// The insert contract only fires when an element would actually be
	// stored (the Push precedent: reads of a nil or zero-value heap are
	// tolerated, stores are not).
	if len(items) > 0 {
		if dh == nil {
			panic("multiway_pq_ts: UnmarshalJSON called on a nil heap")
		}
		if dh.cmp == nil {
			panic("multiway_pq_ts: UnmarshalJSON called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)")
		}
	}
	if dh == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	dh.lock.Lock()
	defer dh.lock.Unlock()
	dh.data = nil
	if len(items) > 0 {
		dh.NlAppendHeap(items)
		for i := (len(items) - 2) / dh.d; i >= 0; i-- {
			dh.NlHeapify(len(items), i) // one atomic O(n) rebuild sweep
		}
	}
	return nil
}

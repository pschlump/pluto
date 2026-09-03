/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package heap_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Heap can be used directly
// with the encoding/json package.  The heap is encoded as a JSON array
// of its elements in internal heap (breadth-first tree) order — the same
// order All produces — which is NOT sorted order; Pop is the way to
// consume a heap in sorted order.
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any heap operation — and an element type with its own
// MarshalJSON may safely call back into the heap (the All snapshot
// convention).  Errors from the json package are returned unchanged (for
// example a T that cannot be encoded at all, such as a channel or a
// function).
//
// An empty heap encodes as [].  A direct call on a nil heap also
// encodes as [] (the "nil behaves as an empty heap" read contract);
// note that json.Marshal on a nil *Heap never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (hp *Heap[T]) MarshalJSON() ([]byte, error) {
	items := hp.snapshot() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil heap marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Heap can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// heap, re-heapified into internal (breadth-first tree) order, under one
// hold of the write lock.  The comparison function is kept, so the heap
// stays usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the heap
// lock — and a decode error (malformed JSON, a non-array document, wrong
// element types) is returned with the heap untouched.  The nil/cmp
// guards are also checked before the lock is acquired (cmp is set only
// by the constructors, so reading it unlocked is safe).
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil heap or a zero-value heap (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the heap and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (hp *Heap[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the AppendHeap precedent).
	if len(items) > 0 {
		if hp == nil {
			panic("heap_ts: UnmarshalJSON called on a nil heap")
		}
		if hp.cmp == nil {
			panic("heap_ts: UnmarshalJSON called on a heap with no comparison function (create the heap with NewHeap or NewHeapFunc)")
		}
	}
	if hp == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	hp.lock.Lock()
	defer hp.lock.Unlock()
	hp.data = nil
	hp.data = append(hp.data, items...)
	// Rebuild the heap ordering; a slice that already satisfies the heap
	// property (a marshaled heap) passes through unchanged.
	for i := len(hp.data)/2 - 1; i >= 0; i-- {
		hp.heapify(len(hp.data), i)
	}
	return nil
}

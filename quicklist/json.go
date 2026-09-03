/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package quicklist

import "encoding/json"

// MarshalJSON implements json.Marshaler so a QuickList can be used
// directly with the encoding/json package.  The list is encoded as a
// JSON array of its elements, head to tail (the head is element 0) —
// the same order All iterates.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the list
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).  Segments stored compressed are read through
// transparently — materialized and re-packed — exactly as At reads
// them; the compression windows are unchanged by the call.
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *QuickList never reaches this method
// — the json package writes null for nil pointers itself.
// Complexity is O(S + n) plus the cost of encoding the elements.
func (q *QuickList[T]) MarshalJSON() ([]byte, error) {
	if q == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, q.length)
	for _, v := range q.All() {
		items = append(items, v)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a QuickList can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list in order — element 0 becomes the new head.  The configuration
// (fill target, byte cap, compression codec and depth) is kept, so the
// list — including a zero-value one, which needs no constructor — stays
// usable after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  data is
// decoded before anything is mutated: a decode error (malformed JSON, a
// non-array document, wrong element types) is returned and leaves the
// list untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list panics with the standard
// insert-family message.  An empty array or null clears the list and is
// tolerated everywhere — it stores nothing.
// Complexity is O(S + n) plus the cost of decoding the elements (the
// recompression window sweep is paid once, not per pushed element).
func (q *QuickList[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 && q == nil {
		panic("quicklist: UnmarshalJSON called on a nil QuickList")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.head, q.tail, q.length = nil, nil, 0
	// PushTail re-establishes the compression windows after every push at
	// O(S); suspend the sweep during the rebuild and run it once at the
	// end, keeping the whole unmarshal O(S + n).
	if q.codec != nil && q.depth > 0 {
		depth := q.depth
		q.depth = 0
		defer func() { q.depth = depth; q.recompress() }()
	}
	for _, v := range items {
		q.PushTail(v)
	}
	return nil
}

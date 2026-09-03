/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sll

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Sll can be used directly
// with the encoding/json package.  The list is encoded as a JSON array
// of its elements, head to tail (the head is element 0).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the list
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *Sll never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (ns *Sll[T]) MarshalJSON() ([]byte, error) {
	if ns == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Sll can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list in order — element 0 becomes the new head.  The equality
// function is kept, so the list stays usable after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-array document, wrong element types) is
// returned and leaves the list untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list or a zero-value list (no
// equality function) panics with the standard insert-family message.
// An empty array or null clears the list and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (ns *Sll[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if ns == nil {
			panic("sll: UnmarshalJSON called on a nil list")
		}
		if ns.eq == nil {
			panic("sll: UnmarshalJSON called on a list with no equality function (create the list with NewSll or NewSllFunc)")
		}
	}
	if ns == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	ns.Truncate()
	for _, d := range items {
		ns.InsertAfterTail(d)
	}
	return nil
}

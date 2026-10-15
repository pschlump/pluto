/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bag

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Bag can be used directly
// with the encoding/json package.  The bag is encoded as a JSON array
// of its elements in insertion order (element 0 is the first element
// added, matching the All iteration order).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the bag
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty bag encodes as [].  A direct call on a nil bag also encodes
// as [] (the "nil behaves as an empty bag" read contract); note that
// json.Marshal on a nil *Bag never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (b *Bag[T]) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, b.length)
	for p := b.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Bag can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// bag in order — element 0 is the first element the new All will yield.
// Unlike dll there is no equality function to keep: the zero value of
// Bag is usable as-is, so unmarshaling into a zero-value bag works.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is touched, so a decode error (malformed JSON, a
// non-array document, wrong element types) is returned with the bag
// untouched.
//
// Unmarshaling stores elements, so it follows the package's insert
// contract: data that would store an element into a nil bag panics with
// the standard insert-family message.  An empty array or null clears
// the bag and is tolerated everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (b *Bag[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the same precedent as Add being the only panicking call).
	if len(items) > 0 && b == nil {
		panic("bag: UnmarshalJSON called on a nil bag")
	}
	if b == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	b.Truncate()
	for _, d := range items {
		b.Add(d)
	}
	return nil
}

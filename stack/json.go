/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stack

import "encoding/json"
import "slices"

// MarshalJSON implements json.Marshaler so a Stack can be used directly
// with the encoding/json package.  The stack is encoded as a JSON array
// of its elements, top to bottom (the top — the most recently pushed
// element — is element 0, the same order All iterates in).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the stack
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty stack encodes as [].  A direct call on a nil stack also
// encodes as [] (the "nil behaves as an empty stack" read contract);
// note that json.Marshal on a nil *Stack never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (ns *Stack[T]) MarshalJSON() ([]byte, error) {
	if ns == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, len(ns.data))
	for _, v := range slices.Backward(ns.data) { // top to bottom
		items = append(items, v)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Stack can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// stack in order — element 0 becomes the new top.  The stack needs no
// constructor-set functions, so it stays fully usable after
// unmarshaling, zero value included.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-array document, wrong element types) is
// returned and leaves the stack untouched.
//
// Unmarshaling stores elements, so it follows the Push contract: data
// that would store an element into a nil stack panics with a message
// naming the method — the package's only panic.  An empty array or null
// clears the stack and is tolerated everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (ns *Stack[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Push precedent).
	if len(items) > 0 && ns == nil {
		panic("stack: UnmarshalJSON called on a nil stack")
	}
	if ns == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	if len(items) == 0 {
		ns.data = nil
		return nil
	}
	// items are top to bottom; the backing slice is bottom to top.
	ns.data = make([]T, len(items))
	for i, v := range items {
		ns.data[len(items)-1-i] = v
	}
	return nil
}

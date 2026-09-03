/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package skip_list

import "encoding/json"

// MarshalJSON implements json.Marshaler so a SkipList can be used
// directly with the encoding/json package.  The list is encoded as a
// JSON array of its elements in ascending order — the level-0 chain,
// which is exactly the order All() iterates.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the list
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *SkipList never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *SkipList[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	if !tt.IsEmpty() {
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			items = append(items, cur.data)
		}
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a SkipList can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list.  The elements are inserted in the order the array lists them —
// the list orders them itself, so the result is the sorted set of the
// decoded elements (array elements that compare equal replace each
// other, exactly as Insert treats duplicates).  The comparison function
// is kept, so the list stays usable after unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is mutated, so a decode error (malformed JSON, a
// non-array document, wrong element types) is returned and leaves the
// list untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list or a zero-value list (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the list and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log₂ n) expected plus the cost of decoding the
// elements (each decoded element is inserted).
func (tt *SkipList[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Insert precedent: nil and zero-value lists cannot store).
	if len(items) > 0 {
		if tt == nil {
			panic("skip_list: UnmarshalJSON called on a nil list")
		}
		if tt.cmp == nil {
			panic("skip_list: UnmarshalJSON called on a list with no comparison function (create the list with NewSkipList or NewSkipListFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate()
	for _, d := range items {
		tt.Insert(d)
	}
	return nil
}

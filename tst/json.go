/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tst

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Tst can be used directly
// with the encoding/json package.  The trie is encoded as a JSON object
// mapping each key to its value; the keys appear in ascending order —
// the natural iteration order of All and the order encoding/json writes
// string-keyed maps.
//
// The values are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the trie
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty trie encodes as {}.  A direct call on a nil trie also
// encodes as {} (the "nil behaves as an empty trie" read contract);
// note that json.Marshal on a nil *Tst never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n·L) plus the cost of encoding the values.
func (tt *Tst[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("{}"), nil
	}
	items := make(map[string]T, tt.length)
	for key, value := range tt.All() {
		items[key] = value
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Tst can be used
// directly with the encoding/json package.  data must be a JSON object
// (or null); the decoded key/value pairs replace the current contents
// of the trie.  There are no constructors to preserve: the zero value
// is fully usable, so the trie stays usable after unmarshaling.
//
// The values are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-object document, wrong value types) is
// returned and leaves the trie untouched.  An empty-string key in data
// is silently dropped: Insert rejects the empty key, and unmarshaling
// stores through it.
//
// Unmarshaling stores values, so it follows the insert contract: data
// that would store a value into a nil trie panics, as Insert does.  An
// empty object or null clears the trie and is tolerated everywhere —
// it stores nothing, so even a nil trie accepts it.
// Complexity is O(n·L) plus the cost of decoding the values.
func (tt *Tst[T]) UnmarshalJSON(data []byte) error {
	var items map[string]T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored ({} and null store nothing).
	if len(items) > 0 && tt == nil {
		panic("tst: UnmarshalJSON called on a nil Tst")
	}
	if tt == nil {
		return nil // null or {}: nothing to store, nothing to clear
	}

	tt.root = nil
	tt.length = 0
	for key, value := range items {
		tt.Insert(key, value)
	}
	return nil
}

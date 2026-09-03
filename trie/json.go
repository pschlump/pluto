/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package trie

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Trie can be used directly
// with the encoding/json package.  The trie is encoded as a JSON object
// mapping each key to its value; the json package emits object members
// in sorted key order, which is the trie's natural iteration order.
//
// The values are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the trie
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).  Keys are arbitrary byte strings; the json
// package replaces any invalid UTF-8 in them with the replacement
// character, so a trie with non-UTF-8 keys does not round-trip through
// JSON.
//
// An empty trie encodes as {}.  A direct call on a nil trie also
// encodes as {} (the "nil behaves as an empty trie" read contract);
// note that json.Marshal on a nil *Trie never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(nodes) plus the cost of encoding the values.
func (t *Trie[T]) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("{}"), nil
	}
	m := make(map[string]T, t.length)
	for k, v := range t.All() {
		m[k] = v
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler so a Trie can be used
// directly with the encoding/json package.  data must be a JSON object
// (or null); the decoded key/value pairs replace the current contents
// of the trie.  The trie needs no constructor-set functions, so it
// stays usable after unmarshaling no matter how it was created.
//
// The values are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is mutated: a decode error (malformed JSON, a
// non-object document, wrong value types) is returned and leaves the
// trie untouched.
//
// Unmarshaling stores values, so it follows the insert contract: data
// that would store a value into a nil trie panics, like Insert, with a
// message naming the method.  A zero-value trie is ready to use and
// accepts elements directly.  An empty object or null clears the trie
// and is tolerated everywhere — it stores nothing.
// Complexity is O(sum of key lengths) plus the cost of decoding the
// values.
func (t *Trie[T]) UnmarshalJSON(data []byte) error {
	var m map[string]T
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored (the Insert precedent).
	if len(m) > 0 && t == nil {
		panic("trie: UnmarshalJSON called on a nil Trie")
	}
	if t == nil {
		return nil // null or {}: nothing to store, nothing to clear
	}

	t.root = nil
	t.length = 0
	for k, v := range m {
		t.Insert(k, v)
	}
	return nil
}

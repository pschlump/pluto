/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package patricia_trie_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a PatriciaTrie can be used
// directly with the encoding/json package.  The trie is encoded as a
// JSON object whose keys are the trie's string keys and whose values
// are the associated elements; the json package writes object keys in
// ascending order, which is exactly the trie's natural iteration order
// (the order of All).
//
// The (key, value) pairs are snapshotted under the read lock (the same
// collector as All) and the encoding itself runs without the lock, so
// this is safe to call concurrently with any trie operation — and an
// element type with its own MarshalJSON may safely call back into the
// trie.  The elements are encoded by the json package itself, so a T
// with its own MarshalJSON — or struct field tags — is honored; errors
// from the json package are returned unchanged (for example a T that
// cannot be encoded at all, such as a channel or a function).
//
// An empty trie encodes as {}.  A direct call on a nil trie also
// encodes as {} (the "nil behaves as an empty trie" read contract);
// note that json.Marshal on a nil *PatriciaTrie never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (pt *PatriciaTrie[T]) MarshalJSON() ([]byte, error) {
	if pt == nil {
		return []byte("{}"), nil
	}
	pt.lock.RLock()
	snap := pt.snapshotPairs()
	pt.lock.RUnlock()
	m := make(map[string]T, len(snap))
	for _, p := range snap {
		m[p.key] = p.value
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler so a PatriciaTrie can be
// used directly with the encoding/json package.  data must be a JSON
// object (or null); the decoded (key, value) pairs replace the current
// contents of the trie under one hold of the write lock.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  data is
// decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// trie lock — and a decode error (malformed JSON, a non-object
// document, wrong element types) is returned with the trie untouched.
// The zero value needs no constructor, so the rebuilt trie is fully
// usable afterward.
//
// Unmarshaling stores elements, so it follows the insert contract:
// data that would store an element into a nil trie panics, like
// Insert — the package's only panic.  An empty object or null clears
// the trie and is tolerated everywhere — it stores nothing.
// Complexity is O(n·w) — one insertion per pair, w the key length in
// bits — plus the cost of decoding the elements.
func (pt *PatriciaTrie[T]) UnmarshalJSON(data []byte) error {
	var m map[string]T
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(m) > 0 && pt == nil {
		panic("patricia_trie_ts: UnmarshalJSON called on a nil PatriciaTrie")
	}
	if pt == nil {
		return nil // null or {}: nothing to store, nothing to clear
	}

	pt.lock.Lock()
	defer pt.lock.Unlock()
	pt.root = nil
	pt.length = 0
	for k, v := range m {
		pt.NlInsert(k, v)
	}
	return nil
}

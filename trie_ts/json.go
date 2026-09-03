/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package trie_ts

import (
	"bytes"
	"encoding/json"
)

// MarshalJSON implements json.Marshaler so a Trie can be used directly
// with the encoding/json package.  The trie is encoded as a JSON object
// whose members are the (key, value) pairs in ascending key order — the
// trie's natural iteration order (the empty-string key, if present, is
// the first member).
//
// The pairs are snapshotted under the read lock and the encoding itself
// runs without the lock, so this is safe to call concurrently with any
// trie operation — and a value type with its own MarshalJSON may safely
// call back into the trie (the All snapshot convention).  The values
// are encoded by the json package itself, so a T with its own
// MarshalJSON — or struct field tags — is honored; only the trie
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
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

	type pair struct {
		key   string
		value T
	}
	var snap []pair
	t.lock.RLock()
	var walk func(x *trieNode[T], prefix []byte)
	walk = func(x *trieNode[T], prefix []byte) {
		if x == nil {
			return
		}
		if x.hasValue {
			snap = append(snap, pair{key: string(prefix), value: x.value})
		}
		for b := 0; b < radix; b++ {
			if x.children[b] != nil {
				walk(x.children[b], append(prefix, byte(b)))
			}
		}
	}
	walk(t.root, nil)
	t.lock.RUnlock()

	if snap == nil {
		return []byte("{}"), nil // an empty trie marshals as an empty object
	}

	// Encode unlocked, members in ascending key order; a key marshal
	// cannot fail, a value marshal reports the json package's error.
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range snap {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(p.key)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(p.value)
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON implements json.Unmarshaler so a Trie can be used
// directly with the encoding/json package.  data must be a JSON object
// (or null); the decoded (key, value) pairs replace the current
// contents of the trie under one hold of the write lock.
//
// The values are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  data is
// decoded before the lock is taken — the json package runs value-level
// UnmarshalJSON methods, which must not run under the trie lock — and
// a decode error (malformed JSON, a non-object document, wrong value
// types) is returned with the trie untouched.  The nil guard is also
// checked before the lock is acquired.
//
// Unmarshaling stores values, so it follows the insert contract: data
// that would store a value into a nil trie panics with the standard
// insert-family message.  The zero value of Trie is ready to use, so
// unmarshaling into one works without any constructor.  An empty object
// or null clears the trie and is tolerated everywhere — it stores
// nothing.
// Complexity is O(total key bytes) plus the cost of decoding the values.
func (t *Trie[T]) UnmarshalJSON(data []byte) error {
	var items map[string]T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored (the Concat precedent).
	if len(items) > 0 && t == nil {
		panic("trie_ts: UnmarshalJSON called on a nil Trie")
	}
	if t == nil {
		return nil // null or {}: nothing to store, nothing to clear
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	t.root = nil
	t.length = 0
	for k, v := range items {
		t.NlInsert(k, v)
	}
	return nil
}

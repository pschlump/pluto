/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_index_disk_ts

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
)

// jsonKV is the wire form of one index entry: an object with the key
// and the value, used because K is generic — a JSON object would
// restrict the keys to strings.
type jsonKV[K any] struct {
	Key   K      `json:"key"`
	Value uint64 `json:"value"`
}

// MarshalJSON implements json.Marshaler so an Index can be used
// directly with the encoding/json package.  The index is encoded as a
// JSON array of {"key":…,"value":…} objects sorted by the encoded key
// bytes — deterministic regardless of insertion and split history (the
// hash_set String precedent: the physical bucket order of All is not a
// key order, so the wire form supplies one; the encodings are
// injective, so their byte order is a total order on keys).
//
// The pairs are snapshotted under the store lock (the All/collect
// snapshot convention) and the encoding itself runs without the lock,
// so this is safe to call concurrently with any index operation — and
// a key type with its own MarshalJSON may safely call back into the
// index.  Errors from the json package are returned unchanged (for
// example a K that cannot be encoded at all, such as a channel or a
// function).
//
// An empty index encodes as [].  A direct call on a nil index — or on
// a zero-value index with no store — also encodes as [] (the "nil
// behaves as an empty index" read contract); note that json.Marshal on
// a nil *Index never reaches this method — the json package writes
// null for nil pointers itself.  An index on a closed store encodes as
// [] as well (the All contract: a closed store yields nothing); other
// IO errors are returned.
// Complexity is O(n log₂ n) for the sort plus the cost of encoding the
// entries.
func (t *Index[K]) MarshalJSON() ([]byte, error) {
	if t == nil || t.store == nil {
		return []byte("[]"), nil
	}
	pairs, err := t.collect() // snapshots under the store lock itself
	if err != nil {
		if errors.Is(err, ErrClosed) {
			return []byte("[]"), nil
		}
		return nil, err
	}
	enc := make([][]byte, len(pairs))
	for i, p := range pairs {
		buf := make([]byte, t.keySize)
		t.encode(p.k, buf)
		enc[i] = buf
	}
	order := make([]int, len(pairs))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return bytes.Compare(enc[a], enc[b]) })
	out := make([]jsonKV[K], len(pairs))
	for i, j := range order {
		out[i] = jsonKV[K]{Key: pairs[j].k, Value: pairs[j].v}
	}
	return json.Marshal(out)
}

// UnmarshalJSON implements json.Unmarshaler so an Index can be used
// directly with the encoding/json package.  data must be a JSON array
// of {"key":…,"value":…} objects (or null); the decoded entries
// replace the current contents of the index under one hold of the
// store lock — every current entry is deleted and the decoded entries
// are inserted in array order (a later duplicate key replaces an
// earlier one, as Insert does).  The EncodeKey/DecodeKey functions are
// kept, so the index stays usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// store lock — and a decode error (malformed JSON, a non-array
// document, wrong key or value types) is returned with the index
// untouched.  The keys are also encoded (EncodeKey is caller code)
// before the lock is taken, and the nil/store guards are checked
// before the lock is acquired (store is set only by NewIndex, so
// reading it unlocked is safe).
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil index or a zero-value index (no
// store) panics with the standard insert-family message.  An empty
// array or null clears the index and is tolerated everywhere — it
// stores nothing.  On a closed store UnmarshalJSON returns ErrClosed,
// even for null or [], because clearing is a store mutation.  The new
// contents live in the cache when UnmarshalJSON returns; they are
// crash-durable after the next completed flush, exactly as with Insert
// (see the package durability contract).
// Complexity is O(m + n) expected block accesses for m current and n
// decoded entries, plus the cost of decoding the entries.
func (t *Index[K]) UnmarshalJSON(data []byte) error {
	var pairs []jsonKV[K]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the dll UnmarshalJSON precedent).
	if len(pairs) > 0 {
		if t == nil {
			panic("hash_index_disk_ts: UnmarshalJSON called on a nil index")
		}
		if t.store == nil {
			panic("hash_index_disk_ts: UnmarshalJSON called on an index with no store (create the index with NewIndex)")
		}
	}
	if t == nil || t.store == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// Encode the keys before the lock is taken: EncodeKey is caller
	// code, which must not run under the store lock.
	keys := make([][]byte, len(pairs))
	for i, p := range pairs {
		buf := make([]byte, t.keySize)
		t.encode(p.Key, buf)
		keys[i] = buf
	}

	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := t.truncateNoLock(); err != nil {
		return err
	}
	for i, p := range pairs {
		if _, err := t.insertNoLock(keys[i], p.Value); err != nil {
			return err
		}
	}
	return nil
}

// truncateNoLock removes every entry of the index through
// deleteNoLock, so emptied overflow blocks are freed onto the store's
// free list exactly as with Delete (primary buckets and the directory
// stay — the high-water-mark policy).
// Caller holds s.mu.
func (t *Index[K]) truncateNoLock() error {
	keys, err := t.encodedKeys()
	if err != nil {
		return err
	}
	for _, k := range keys {
		if _, err := t.deleteNoLock(k); err != nil {
			return err
		}
	}
	return nil
}

// encodedKeys snapshots every stored key encoding under the store
// lock (a copy, so later deletes cannot invalidate the slices).
// Caller holds s.mu.
func (t *Index[K]) encodedKeys() ([][]byte, error) {
	pairs, err := t.collectNoLock()
	if err != nil {
		return nil, err
	}
	keys := make([][]byte, len(pairs))
	for i, p := range pairs {
		buf := make([]byte, t.keySize)
		t.encode(p.k, buf)
		keys[i] = buf
	}
	return keys, nil
}

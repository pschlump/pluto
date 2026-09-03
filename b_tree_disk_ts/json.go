/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package b_tree_disk_ts

import (
	"encoding/json"
	"errors"
)

// jsonKV is the wire form of one tree entry: an object with the key and
// the value, used because K is generic — a JSON object would restrict
// the keys to strings.
type jsonKV[K any] struct {
	Key   K      `json:"key"`
	Value uint64 `json:"value"`
}

// MarshalJSON implements json.Marshaler so a Tree can be used directly
// with the encoding/json package.  The tree is encoded as a JSON array
// of {"key":…,"value":…} objects in ascending key order — the tree's
// natural iteration order (the order All yields) — so the output is
// deterministic regardless of insertion order.
//
// The pairs are snapshotted under the store lock (the All/collect
// snapshot convention) and the encoding itself runs without the lock, so
// this is safe to call concurrently with any tree operation — and a key
// type with its own MarshalJSON may safely call back into the tree.
// Errors from the json package are returned unchanged (for example a K
// that cannot be encoded at all, such as a channel or a function).
//
// An empty tree encodes as [].  A direct call on a nil tree — or on a
// zero-value tree with no store — also encodes as [] (the "nil behaves
// as an empty tree" read contract); note that json.Marshal on a nil
// *Tree never reaches this method — the json package writes null for nil
// pointers itself.  A tree on a closed store encodes as [] as well (the
// All contract: a closed store yields nothing); other IO errors are
// returned.
// Complexity is O(n) plus the cost of encoding the entries.
func (t *Tree[K]) MarshalJSON() ([]byte, error) {
	if t == nil || t.store == nil {
		return []byte("[]"), nil
	}
	pairs, err := t.collect(nil, false) // snapshots under the store lock itself
	if err != nil {
		if errors.Is(err, ErrClosed) {
			return []byte("[]"), nil
		}
		return nil, err
	}
	out := make([]jsonKV[K], 0, len(pairs))
	for _, p := range pairs {
		out = append(out, jsonKV[K]{Key: p.k, Value: p.v})
	}
	return json.Marshal(out)
}

// UnmarshalJSON implements json.Unmarshaler so a Tree can be used
// directly with the encoding/json package.  data must be a JSON array of
// {"key":…,"value":…} objects (or null); the decoded entries replace the
// current contents of the tree under one hold of the store lock — every
// current entry is deleted and the decoded entries are inserted in array
// order (a later duplicate key replaces an earlier one, as Insert does).
// The EncodeKey/DecodeKey/Compare functions are kept, so the tree stays
// usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// store lock — and a decode error (malformed JSON, a non-array document,
// wrong key or value types) is returned with the tree untouched.  The
// keys are also encoded (EncodeKey is caller code) before the lock is
// taken, and the nil/store guards are checked before the lock is
// acquired (store is set only by NewTree, so reading it unlocked is
// safe).
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil tree or a zero-value tree (no
// store) panics with the standard insert-family message.  An empty array
// or null clears the tree and is tolerated everywhere — it stores
// nothing.  On a closed store UnmarshalJSON returns ErrClosed, even for
// null or [], because clearing is a store mutation.  The new contents
// live in the cache when UnmarshalJSON returns; they are crash-durable
// after the next completed flush, exactly as with Insert (see the
// package durability contract).
// Complexity is O(m log₂ m + n log₂ n) for m current and n decoded
// entries, plus the cost of decoding the entries.
func (t *Tree[K]) UnmarshalJSON(data []byte) error {
	var pairs []jsonKV[K]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the dll UnmarshalJSON precedent).
	if len(pairs) > 0 {
		if t == nil {
			panic("b_tree_disk_ts: UnmarshalJSON called on a nil tree")
		}
		if t.store == nil {
			panic("b_tree_disk_ts: UnmarshalJSON called on a tree with no store (create the tree with NewTree)")
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

// truncateNoLock removes every entry of the tree through deleteNoLock,
// so blocks are freed onto the store's free list exactly as with Delete.
// Caller holds s.mu.
func (t *Tree[K]) truncateNoLock() error {
	s := t.store
	ks := t.keySize
	rootNo, _, _, err := s.readEntryLocked(t.slot)
	if err != nil {
		return err
	}
	cur, err := s.cacheGet(rootNo)
	if err != nil {
		return err
	}
	for !isLeaf(cur.data) {
		if cur, err = s.cacheGet(internalChild(cur.data, 0, ks, t.maxInt)); err != nil {
			return err
		}
	}

	// Copy the keys out before deleting: a leaf key is a slice of its
	// block, and deletion mutates that block.
	var keys [][]byte
	for {
		for i := 0; i < leafCount(cur.data); i++ {
			k := make([]byte, ks)
			copy(k, leafKey(cur.data, i, ks))
			keys = append(keys, k)
		}
		next := leafNext(cur.data)
		if next == 0 {
			break
		}
		if cur, err = s.cacheGet(next); err != nil {
			return err
		}
	}
	for _, k := range keys {
		if _, err := t.deleteNoLock(k); err != nil {
			return err
		}
	}
	return nil
}

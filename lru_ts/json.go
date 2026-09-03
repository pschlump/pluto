/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts

import "encoding/json"

// jsonKV is the wire form of one cache entry: an object with the key
// and the value, used because K is a generic comparable — a JSON object
// would restrict the keys to strings.
type jsonKV[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

// MarshalJSON implements json.Marshaler so an Lru can be used directly
// with the encoding/json package.  The cache is encoded as a JSON array
// of {"key":…,"value":…} objects in the natural iteration order — most
// recently used first (the All order) — so a round trip through
// UnmarshalJSON restores the recency order, not just the contents.
//
// The entries are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any cache operation — and a key or value type with its own
// MarshalJSON may safely call back into the cache (the All/Backward
// snapshot convention).  Errors from the json package are returned
// unchanged (for example a K or V that cannot be encoded at all, such
// as a channel or a function).
//
// An empty cache encodes as [].  A direct call on a nil cache also
// encodes as [] (the "nil behaves as an empty cache" read contract);
// note that json.Marshal on a nil *Lru never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the entries.
func (c *Lru[K, V]) MarshalJSON() ([]byte, error) {
	var pairs []jsonKV[K, V]
	for k, v := range c.All() { // snapshots under the read lock itself; nil-tolerant
		pairs = append(pairs, jsonKV[K, V]{Key: k, Value: v})
	}
	if pairs == nil {
		return []byte("[]"), nil // a nil or empty cache marshals as an empty array
	}
	return json.Marshal(pairs)
}

// UnmarshalJSON implements json.Unmarshaler so an Lru can be used
// directly with the encoding/json package.  data must be a JSON array
// of {"key":…,"value":…} objects (or null); the decoded entries replace
// the current contents of the cache under one hold of the write lock.
// The entries are stored least-recently-used first, so the array order
// (most recently used first, as MarshalJSON produces it) is restored as
// the recency order.  The capacity and the eviction-veto callback are
// kept, so the cache stays usable after unmarshaling — and if the array
// holds more entries than the capacity, the excess is evicted as usual,
// keeping the most recently used ones.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// cache lock — and a decode error (malformed JSON, a non-array
// document, wrong key or value types) is returned with the cache
// untouched.  The eviction-veto callback, if any, runs while the write
// lock is held, exactly as in Put: it must not call back into this
// cache.
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil cache or a zero-value cache (no
// capacity) panics with the standard insert-family message.  An empty
// array or null clears the cache and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n) plus the cost of decoding the entries.
func (c *Lru[K, V]) UnmarshalJSON(data []byte) error {
	var pairs []jsonKV[K, V]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the dll UnmarshalJSON precedent).  inner is set only by
	// the constructors, so reading it unlocked is safe.
	if len(pairs) > 0 {
		if c == nil {
			panic("lru_ts: UnmarshalJSON on a nil cache: a nil cache cannot store an entry; create it with NewLru or NewLruFunc")
		}
		if c.inner == nil {
			panic("lru_ts: UnmarshalJSON on a zero-value cache: no capacity; create the cache with NewLru or NewLruFunc")
		}
	}
	if c == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if c.inner == nil {
		return nil // zero value: null or [] stores nothing, and there is nothing to clear
	}
	c.inner.Clear()
	for i := len(pairs) - 1; i >= 0; i-- {
		// Put LRU-first so the array's MRU-first order is restored as
		// the recency order; each Put evicts to capacity as it goes.
		c.inner.Put(pairs[i].Key, pairs[i].Value)
	}
	return nil
}

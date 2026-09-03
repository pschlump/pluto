/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import "encoding/json"

// jsonPair is the wire form of one (key, value) entry of the cache.
type jsonPair[K comparable, V any] struct {
	K K `json:"k"`
	V V `json:"v"`
}

// MarshalJSON implements json.Marshaler so an Lru can be used directly
// with the encoding/json package.  The cache is encoded as a JSON array
// of {"k":key,"v":value} objects in recency order, most recently used
// first — the same order All yields.
//
// The keys and values are encoded by the json package itself, so a K or
// V with its own MarshalJSON — or struct field tags — is honored; only
// the cache structure is pluto's.  Errors from the json package are
// returned unchanged (for example a V that cannot be encoded at all,
// such as a channel or a function).
//
// An empty cache encodes as [].  A direct call on a nil cache also
// encodes as [] (the "nil behaves as an empty cache" read contract);
// note that json.Marshal on a nil *Lru never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the entries.
func (c *Lru[K, V]) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("[]"), nil
	}
	pairs := make([]jsonPair[K, V], 0, c.Len())
	for k, v := range c.All() { // nil/zero-value caches yield nothing
		pairs = append(pairs, jsonPair[K, V]{K: k, V: v})
	}
	return json.Marshal(pairs)
}

// UnmarshalJSON implements json.Unmarshaler so an Lru can be used
// directly with the encoding/json package.  data must be a JSON array
// of {"k":key,"v":value} objects (or null); the decoded entries replace
// the current contents of the cache, recreating the encoded recency
// order — pair 0 becomes the most recently used entry.  The capacity
// and the veto callback are kept, so the cache stays usable after
// unmarshaling.
//
// The entries are inserted through Put, so the eviction contract
// applies: an array longer than the capacity is trimmed as it loads,
// least recently used entry first, subject to the veto (with enough
// vetoes the rebuilt cache may exceed its capacity — the soft cap).
// Duplicate keys follow the recency convention: the first (most
// recently used) pair wins.
//
// The keys and values are decoded by the json package itself, so a K or
// V with its own UnmarshalJSON — or struct field tags — is honored.
// The whole document is decoded before anything is stored: a decode
// error (malformed JSON, a non-array document, wrong key or value
// types) is returned and leaves the cache untouched.
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil cache or a zero-value cache (no
// capacity) panics with the standard insert-family message.  An empty
// array or null clears the cache and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n) plus the cost of decoding the entries, except that
// eviction with a vetoing callback is O(scan) per insert in the worst
// case.
func (c *Lru[K, V]) UnmarshalJSON(data []byte) error {
	var pairs []jsonPair[K, V]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the Concat precedent in dll).
	if len(pairs) > 0 {
		if c == nil {
			panic("lru: UnmarshalJSON on a nil cache: a nil cache cannot store an entry; create it with NewLru or NewLruFunc")
		}
		if c.byKey == nil {
			panic("lru: UnmarshalJSON on a zero-value cache: no capacity; create the cache with NewLru or NewLruFunc")
		}
	}
	if c == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	c.Clear()
	// Insert least recently used first: each Put marks its entry most
	// recently used, so pair 0 — the encoded MRU entry — is Put last
	// and the encoded recency order is recreated exactly.
	for i := len(pairs) - 1; i >= 0; i-- {
		c.Put(pairs[i].K, pairs[i].V)
	}
	return nil
}

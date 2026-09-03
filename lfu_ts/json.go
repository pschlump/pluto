/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts

import "encoding/json"

// jsonEntry is the wire form of one key's frequency state — the plain
// package's lfuEntry with exported fields so the encoding/json package
// can see them (the same shape the plain twin emits, so a dump from one
// twin loads into the other).  Key is encoded by the json package
// itself (a K with its own MarshalJSON is honored); Counter and LastMin
// are the table's raw state — the Morris counter and the 16-bit
// last-access minute (LDT).
type jsonEntry[K comparable] struct {
	Key     K      `json:"key"`
	Counter uint8  `json:"counter"`
	LastMin uint16 `json:"lastMin"`
}

// jsonRawEntry is jsonEntry with the key left as its encoded bytes, so
// MarshalJSON can reorder the borrowed table's wire form by key without
// decoding and re-encoding the keys (a K whose MarshalJSON is not the
// identity still matches its own encoding).
type jsonRawEntry struct {
	Key     json.RawMessage `json:"key"`
	Counter uint8           `json:"counter"`
	LastMin uint16          `json:"lastMin"`
}

// MarshalJSON implements json.Marshaler so an Lfu can be used directly
// with the encoding/json package.  The table is encoded as a JSON array
// of key/state objects, one per key:
//
//	[{"key":"a","counter":6,"lastMin":1000}, ...]
//
// counter is the stored Morris counter (not decay-adjusted) and lastMin
// the stored 16-bit minute of the key's last Touch/Add, so a round trip
// preserves the exact table state — Counter and IdleMinutes agree with
// the original while the clock stands still.  Unlike the plain twin
// (whose entries come out in bucket order) the entries are enumerated
// in first-insert order — the key list the twin maintains for exactly
// this — so the output is deterministic from process to process.
//
// The keys are encoded by the json package itself, so a K with its own
// MarshalJSON — or struct field tags — is honored; the encoded key
// bytes pass through unchanged.  Errors from the json package are
// returned unchanged (for example a K that cannot be encoded at all,
// such as a channel).
//
// The first-insert order and the raw table state are snapshotted under
// the read lock; the reordering and the final encoding run without the
// lock, so this is safe to call concurrently with any table operation.
// (The snapshot encodes the borrowed plain table under the read lock —
// the borrowed structure has no locks of its own — so a K whose
// MarshalJSON calls back into this table can deadlock; do not do that.)
//
// An empty table encodes as [].  A direct call on a nil or zero-value
// table also encodes as [] (the "nil behaves as an empty table" read
// contract); note that json.Marshal on a nil *Lfu never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the keys.
func (l *Lfu[K]) MarshalJSON() ([]byte, error) {
	if l == nil || l.inner == nil {
		return []byte("[]"), nil
	}

	// Snapshot under the read lock: the first-insert key order and the
	// raw table state (the borrowed table's own wire form).
	l.lock.RLock()
	keys := append([]K(nil), l.keys...)
	raw, err := l.inner.MarshalJSON()
	l.lock.RUnlock()
	if err != nil {
		return nil, err
	}

	// Reorder the raw state into first-insert order and encode, both
	// without the lock.  The key bytes pass through as encoded (a K
	// whose MarshalJSON is not the identity still matches itself), and
	// keys the snapshot no longer holds (a Truncate leaves the order
	// list behind) are skipped.
	var entries []jsonRawEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	byKey := make(map[string]jsonRawEntry, len(entries))
	for _, e := range entries {
		byKey[string(e.Key)] = e
	}
	items := make([]jsonRawEntry, 0, len(keys))
	for _, k := range keys {
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		if e, ok := byKey[string(kb)]; ok {
			items = append(items, e)
		}
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so an Lfu can be used
// directly with the encoding/json package.  data must be a JSON array
// of key/state objects (or null); the decoded entries replace the
// current contents of the table — the exact stored counters and
// last-access minutes are restored, the first-insert order becomes the
// array order, and the configuration (logFactor, decay time, clock) is
// kept, so the table stays usable after unmarshaling.  Note that
// lastMin is interpreted against the table's own clock: restoring onto
// a table whose clock reads a different minute shifts every key's idle
// time and decay accordingly.
//
// data is decoded before the lock is taken and before anything is
// mutated — the json package runs key-level UnmarshalJSON methods,
// which must not run under the table lock — and a decode error
// (malformed JSON, a non-array document, wrong key or counter types) is
// returned with the table untouched.  The replacement itself runs under
// one hold of the write lock (the borrowed table restores its raw state
// from its own wire form, then the key order list is rebuilt), so a
// concurrent reader sees the old contents or the new, never a mix.
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil table or a zero-value table (no
// configuration) panics with the standard insert-family message naming
// the constructors.  An empty array or null clears the table and is
// tolerated everywhere — it stores nothing.
// Complexity is O(n) average plus the cost of decoding the keys.
func (l *Lfu[K]) UnmarshalJSON(data []byte) error {
	var items []jsonEntry[K]
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the Touch/Add precedent: the message names the fix).
	if len(items) > 0 {
		if l == nil {
			panic("lfu_ts: UnmarshalJSON on a nil *Lfu — a nil table cannot record an entry; create it with NewLfu or NewLfuWithClock")
		}
		if l.inner == nil {
			panic("lfu_ts: UnmarshalJSON on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
		}
	}
	if l == nil || l.inner == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// The borrowed table restores raw state only from its own wire
	// form; re-encoding the already-decoded entries cannot fail.
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}

	l.lock.Lock()
	defer l.lock.Unlock()
	if err := l.inner.UnmarshalJSON(raw); err != nil {
		return err // the borrowed table is untouched on a decode error
	}
	l.clearKeys()
	for _, e := range items {
		l.noteKey(e.Key)
	}
	return nil
}

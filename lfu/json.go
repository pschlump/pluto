/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import "encoding/json"

// jsonEntry is the wire form of one key's frequency state — the lfuEntry
// with exported fields so the encoding/json package can see them.  Key is
// encoded by the json package itself (a K with its own MarshalJSON is
// honored); Counter and LastMin are the table's raw state — the Morris
// counter and the 16-bit last-access minute (LDT).
type jsonEntry[K comparable] struct {
	Key     K      `json:"key"`
	Counter uint8  `json:"counter"`
	LastMin uint16 `json:"lastMin"`
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
// the original while the clock stands still.  The entries come out in
// the backing hash_grow table's bucket order, which depends on the
// per-table hash seed and varies from process to process — never depend
// on a fixed order.
//
// The keys are encoded by the json package itself, so a K with its own
// MarshalJSON — or struct field tags — is honored; only the table
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a K that cannot be encoded at all, such as a
// channel).
//
// An empty table encodes as [].  A direct call on a nil table also
// encodes as [] (the "nil behaves as an empty table" read contract);
// note that json.Marshal on a nil *Lfu never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the keys.
func (l *Lfu[K]) MarshalJSON() ([]byte, error) {
	if l == nil || l.tab == nil {
		return []byte("[]"), nil
	}
	items := make([]jsonEntry[K], 0, l.tab.Len())
	for e := range l.tab.Values() {
		items = append(items, jsonEntry[K]{Key: e.key, Counter: e.counter, LastMin: e.lastMin})
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so an Lfu can be used
// directly with the encoding/json package.  data must be a JSON array of
// key/state objects (or null); the decoded entries replace the current
// contents of the table — the exact stored counters and last-access
// minutes are restored, and the configuration (logFactor, decay time,
// clock) is kept, so the table stays usable after unmarshaling.  Note
// that lastMin is interpreted against the table's own clock: restoring
// onto a table whose clock reads a different minute shifts every key's
// idle time and decay accordingly.
//
// The keys are decoded by the json package itself, so a K with its own
// UnmarshalJSON — or struct field tags — is honored.  data is decoded
// before anything is mutated; a decode error (malformed JSON, a
// non-array document, wrong key types) is returned and leaves the table
// untouched.
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
			panic("lfu: UnmarshalJSON on a nil *Lfu — a nil table cannot record an entry; create it with NewLfu or NewLfuWithClock")
		}
		if l.tab == nil {
			panic("lfu: UnmarshalJSON on a zero-value Lfu — no configuration; create the table with NewLfu or NewLfuWithClock")
		}
	}
	if l == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	l.Truncate() // a nil tab (zero value) is tolerated by Truncate
	for _, e := range items {
		l.tab.Insert(lfuEntry[K]{key: e.Key, counter: e.Counter, lastMin: e.LastMin})
	}
	return nil
}

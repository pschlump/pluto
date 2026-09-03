/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/skip_list"
)

// MarshalJSON implements json.Marshaler so an ID can be used directly
// with the encoding/json package.  The ID is encoded as a JSON string in
// its canonical "ms-seq" form (the same form String prints and ParseID
// parses, so IDs round-trip).
// Complexity is O(1).
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

// UnmarshalJSON implements json.Unmarshaler so an ID can be used
// directly with the encoding/json package.  data must be a JSON string
// holding a numeric ID in any form ParseID accepts ("ms-seq", bare "ms"
// or "ms-*"); anything else — including a non-string document — is a
// decode error and leaves the ID untouched.
// Complexity is O(len(data)).
func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// MarshalJSON implements json.Marshaler so a Stream can be used directly
// with the encoding/json package.  The stream is encoded as a JSON array
// of its entries in ascending ID order (the Range order), each entry an
// object with its "id" in the canonical "ms-seq" string form and its
// "fields" as an array of name/value pairs.
//
// An empty stream encodes as [].  A direct call on a nil stream also
// encodes as [] (the "nil behaves as an empty stream" read contract);
// note that json.Marshal on a nil *Stream never reaches this method —
// the json package writes null for nil pointers itself.
//
// Only the entry log is encoded: the last assigned ID and the consumer
// groups are runtime state, not stream contents (LastID survives
// Delete/Trim the same way it is not part of this encoding).
// Complexity is O(n) plus the cost of encoding the entries.
func (s *Stream) MarshalJSON() ([]byte, error) {
	if s == nil || s.entries == nil {
		return []byte("[]"), nil
	}
	items := make([]Entry, 0, s.entries.Len())
	for e := range s.Range(MinID, MaxID, 0) {
		items = append(items, e)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Stream can be used
// directly with the encoding/json package.  data must be a JSON array of
// entries (or null); the decoded entries replace the current entry log
// in ascending ID order.  Each entry ID must be a valid stream entry ID
// — strictly increasing and above 0-0, the same rule Add enforces; a
// violation is returned as an error wrapping ErrIDTooSmall.
//
// The document is fully decoded and validated before anything is
// mutated: a decode error (malformed JSON, a non-array document, an
// invalid ID) or an ID-ordering error leaves the stream untouched.  The
// replacement is built offline and swapped in, and the last assigned ID
// is advanced to the newest decoded ID when that is above it (so
// subsequent Adds start above the restored tail, as after SetLastID).
// Consumer groups are left in place — replacing the log behaves like
// deleting every old entry, which keeps the PELs, per the Delete
// contract.
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil stream panics with the standard
// insert-family message (the Add precedent — the stream needs no
// constructor, so a zero-value Stream accepts data).  An empty array or
// null clears the entry log and is tolerated everywhere — it stores
// nothing.
// Complexity is O(n log₂ n) expected plus the cost of decoding.
func (s *Stream) UnmarshalJSON(data []byte) error {
	var items []Entry
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored; an empty array or null stores nothing.
	if len(items) > 0 && s == nil {
		panic("stream: UnmarshalJSON called on a nil Stream (use a stream value, not a nil pointer)")
	}
	if s == nil {
		return nil
	}

	// Validate and build the replacement log before touching the
	// stream, so any error leaves it untouched.
	entries := skip_list.NewSkipListFunc(compareEntry)
	prev := MinID
	for _, e := range items {
		if CompareID(e.ID, prev) <= 0 {
			return fmt.Errorf("stream: UnmarshalJSON entry ID %v is not above %v: %w", e.ID, prev, ErrIDTooSmall)
		}
		prev = e.ID
		entries.Insert(e)
	}

	s.entries = entries
	if len(items) > 0 && CompareID(prev, s.last) > 0 {
		s.last = prev
	}
	return nil
}

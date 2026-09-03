/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream_ts

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/stream"
)

// MarshalJSON implements json.Marshaler so a Stream can be used directly
// with the encoding/json package.  The stream is encoded as a JSON array
// of its entries in ascending ID order — the natural iteration order of
// Range(MinID, MaxID, 0).  Only the entries are encoded; consumer groups
// and the last-ID marker are per-run delivery state and do not appear in
// the document.
//
// The entries are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any stream operation (the Range snapshot convention).  The Entry
// and ID values are encoded by the json package itself, so a future
// MarshalJSON on either — or struct field tags — is honored; only the
// array structure is pluto's.  Errors from the json package are returned
// unchanged.
//
// An empty stream encodes as [].  A direct call on a nil stream also
// encodes as [] (the "nil behaves as an empty stream" read contract);
// note that json.Marshal on a nil *Stream never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the entries.
func (s *Stream) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	s.lock.RLock()
	snapshot := s.NlRangeSnapshot(MinID, MaxID, 0)
	s.lock.RUnlock()
	if snapshot == nil {
		return []byte("[]"), nil // an empty stream marshals as an empty array
	}
	return json.Marshal(snapshot)
}

// UnmarshalJSON implements json.Unmarshaler so a Stream can be used
// directly with the encoding/json package.  data must be a JSON array of
// entries (or null); the decoded entries replace the whole stream state
// — entries, consumer groups and the last-ID marker — in ascending ID
// order under one hold of the write lock.  The zero value is ready to
// use, so unmarshaling into a zero-value Stream works; afterwards the
// last assigned ID is the last entry's ID (MinID when the data cleared
// the stream).
//
// data is decoded and validated before the lock is taken and before
// anything mutates — the json package runs element-level UnmarshalJSON
// methods, which must not run under the stream lock.  A decode error
// (malformed JSON, a non-array document, wrong element types) is
// returned with the stream untouched, and so is a validation error: the
// entry IDs must strictly increase (as Add requires — 0-0 is never a
// valid entry ID) and no entry may carry the AutoSeq sentinel as its
// sequence part, which is an Add request form, never an assigned ID.
//
// Unmarshaling stores entries, so it follows the insert contract: data
// that would store an entry into a nil stream panics naming the method,
// like Add.  An empty array or null clears the stream and is tolerated
// everywhere — it stores nothing.
// Complexity is O(n log₂ n) expected plus the cost of decoding the
// entries.
func (s *Stream) UnmarshalJSON(data []byte) error {
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// The insert contract only fires when an entry would actually be
	// stored (the Concat precedent).
	if len(entries) > 0 && s == nil {
		panic("stream_ts: UnmarshalJSON called on a nil Stream (use a stream value, not a nil pointer)")
	}

	// Validate before mutating: re-adding through NlAdd must not be able
	// to fail once the stream state has been replaced.
	last := MinID
	for i := range entries {
		if entries[i].ID.Seq == AutoSeq {
			return fmt.Errorf("stream_ts: UnmarshalJSON: entry %d carries the AutoSeq sentinel as its sequence part: %w", i, ErrInvalidID)
		}
		if CompareID(last, entries[i].ID) >= 0 {
			return fmt.Errorf("stream_ts: UnmarshalJSON: entry IDs must strictly increase (%v is not before %v): %w", last, entries[i].ID, ErrIDTooSmall)
		}
		last = entries[i].ID
	}
	if s == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.inner = stream.Stream{}
	for _, e := range entries {
		if _, err := s.NlAdd(e.ID, e.Fields); err != nil {
			return err // unreachable: the IDs were validated above
		}
	}
	return nil
}

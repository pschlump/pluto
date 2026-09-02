/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stream implements an append-oriented log of entries identified
// by <ms>-<seq> IDs, with consumer groups and pending-entry tracking —
// the Redis stream of XADD/XRANGE/XREAD/XGROUP (note/redis/src/t_stream.c
// lineage) as a plain Go data structure.
//
// Storage is a pluto skip_list keyed by ID (an intra-pluto composition),
// which gives O(log₂ n) expected add/seek, O(log₂ n + m) range walks and —
// through the skip list's span counters — O(log₂ n + k) trimming by rank.
// Redis packs entries into listpack blocks under a radix tree for
// byte-level compactness; this port follows the request note's guidance
// to favor simplicity and concurrency over compactness, storing each
// Entry by value in the skip list.
//
// The zero value of Stream is an empty stream ready to use (including
// Add) — no constructor.  Ordering is inherent in the ID type
// (CompareID), like the key bytes of the trie packages, so there is no
// comparison function to store.  A nil *Stream behaves as an empty stream
// for every operation except Add and CreateGroup — there is nowhere to
// record the data, and those calls panic naming the method.  These are
// the package's only panics; Add reports ID rule violations (ErrIDTooSmall,
// ErrIDExhausted) and CreateGroup reports a duplicate name (ErrGroupExists)
// as errors.
//
// Core operations:
//
//	Add(id, fields) — Append an entry; AutoSeq assigns the sequence part.	O(log₂ n) expected
//	Len / IsEmpty — Number of entries / emptiness.							O(1)
//	FirstID — Smallest entry ID.												O(1)
//	LastID — Last assigned ID (survives trim/delete; SetLastID moves it).	O(1)
//	Range / RevRange — Iterate start ≤ id ≤ end ascending / descending.		O(log₂ n + m)
//	Delete — Remove one entry; group PELs are unaffected.					O(log₂ n) expected
//	TrimMaxLen / TrimMinID — Evict oldest / below-min entries.				O(log₂ n + k)
//	SetLastID — Move the last-assigned ID (XSETID).							O(1)
//
// Consumer groups (ReadGroup, Ack, Pending, PendingRange, Claim,
// AutoClaim and the Group* methods) track delivery per (group, consumer)
// in a pending-entries list ordered by ID — see group.go.
//
// Stream is not safe for concurrent use; the mutex-guarded twin
// stream_ts has the same interface.
package stream

import (
	"iter"
	"math"

	"github.com/pschlump/pluto/skip_list"
)

// Entry is one record of the log: its ID and its ordered field/value
// pairs.  Fields may repeat a name (Redis allows it) and are stored in
// order; Add copies the slice, so the caller may reuse it afterwards.
type Entry struct {
	ID     ID
	Fields [][2]string
}

// Stream is an append-oriented log of entries keyed by strictly
// increasing IDs, with consumer groups.
//
// The zero value is an empty stream ready to use.
type Stream struct {
	// entries holds the live entries ordered by ID.  Lazily created by
	// Add — nil behaves as empty for every read.
	entries *skip_list.SkipList[Entry]

	// last is the last assigned ID.  It survives Delete and the trims
	// (XDEL/XTRIM do not regress it) and is moved by SetLastID; Add
	// requires every new ID to be strictly greater than it.
	last ID

	// groups holds the consumer groups by name.  Lazily created by
	// CreateGroup.
	groups map[string]*group
}

// compareEntry orders entries (and therefore search probes carrying only
// an ID) by their ID — the comparison the entry skip list is built with.
func compareEntry(a, b Entry) int {
	return CompareID(a.ID, b.ID)
}

// copyFields returns a private copy of fields so the caller's slice can
// be reused without corrupting the stream.
func copyFields(fields [][2]string) [][2]string {
	if fields == nil {
		return nil
	}
	out := make([][2]string, len(fields))
	copy(out, fields)
	return out
}

// Add appends an entry and returns the ID assigned to it.
//
// When id.Seq is AutoSeq the sequence part is assigned: last.Seq+1 if
// id.Ms equals the last ID's ms part, 0 otherwise (so the resolved ID may
// still fail the monotonic check when id.Ms is below the last ID's ms
// part).  Otherwise id is used as given.
//
// Add returns ErrIDTooSmall when the resolved ID is equal to or smaller
// than the stream's last ID — IDs strictly increase, so 0-0 is never a
// valid entry ID, not even on an empty stream — and ErrIDExhausted when
// an auto sequence number would overflow.  On error no ID was assigned
// and the zero ID is returned.  fields is copied and may be empty
// (callers enforcing Redis's at-least-one-field rule do so at their
// layer).
//
// It panics on a nil *Stream — one of the package's two panics.
// Complexity is O(log₂ n) expected.
func (s *Stream) Add(id ID, fields [][2]string) (ID, error) {
	if s == nil {
		panic("stream: Add called on a nil Stream (use a stream value, not a nil pointer)")
	}
	if id.Seq == AutoSeq {
		switch {
		case id.Ms == s.last.Ms && s.last.Seq == math.MaxUint64:
			return ID{}, ErrIDExhausted
		case id.Ms == s.last.Ms:
			id = ID{Ms: id.Ms, Seq: s.last.Seq + 1}
		default:
			id = ID{Ms: id.Ms}
		}
	}
	if CompareID(id, s.last) <= 0 {
		return ID{}, ErrIDTooSmall
	}
	if s.entries == nil {
		s.entries = skip_list.NewSkipListFunc(compareEntry)
	}
	s.entries.Insert(Entry{ID: id, Fields: copyFields(fields)})
	s.last = id
	return id, nil
}

// IsEmpty reports whether the stream has no entries.
// Complexity is O(1).
func (s *Stream) IsEmpty() bool {
	return s.Len() == 0
}

// Len returns the number of entries in the stream.
// Complexity is O(1).
func (s *Stream) Len() int {
	if s == nil {
		return 0
	}
	return s.entries.Len()
}

// FirstID returns the smallest entry ID, or false when the stream is
// empty.
// Complexity is O(1).
func (s *Stream) FirstID() (ID, bool) {
	if s == nil {
		return ID{}, false
	}
	e, found := s.entries.FindMin()
	if !found {
		return ID{}, false
	}
	return e.ID, true
}

// LastID returns the last assigned ID — the entry ID of the most recent
// Add, or whatever SetLastID last set.  It does not regress when entries
// are deleted or trimmed, and MinID (0-0) on a fresh stream.
// Complexity is O(1).
func (s *Stream) LastID() ID {
	if s == nil {
		return ID{}
	}
	return s.last
}

// Range returns an iterator over the entries with start ≤ id ≤ end in
// ascending ID order (the XRANGE form; bounds are inclusive and a start
// greater than end iterates as empty).  A count ≤ 0 means no limit; the
// iteration stops after count entries otherwise.
//
// The iterator walks the live storage: the stream must not be modified
// while it is being consumed, and break exits early.
// Complexity is O(log₂ n + m) time, O(1) space.
func (s *Stream) Range(start, end ID, count int) iter.Seq[Entry] {
	if s == nil {
		return func(func(Entry) bool) {} // a nil stream iterates as an empty one
	}
	return func(yield func(Entry) bool) {
		n := 0
		for _, e := range s.entries.Range(Entry{ID: start}, Entry{ID: end}) {
			if !yield(e) {
				return
			}
			n++
			if count > 0 && n >= count {
				return
			}
		}
	}
}

// RevRange is Range in descending ID order (the XREVRANGE form) — note
// the parameter order is (end, start), mirroring the Redis command: it
// iterates the entries with start ≤ id ≤ end from end down to start.  A
// count ≤ 0 means no limit, a start greater than end iterates as empty,
// and the walk is live like Range's.
// Complexity is O(log₂ n + m) time, O(m) space (no back pointers).
func (s *Stream) RevRange(end, start ID, count int) iter.Seq[Entry] {
	if s == nil {
		return func(func(Entry) bool) {} // a nil stream iterates as an empty one
	}
	return func(yield func(Entry) bool) {
		n := 0
		for _, e := range s.entries.RangeBackward(Entry{ID: start}, Entry{ID: end}) {
			if !yield(e) {
				return
			}
			n++
			if count > 0 && n >= count {
				return
			}
		}
	}
}

// Delete removes the entry with the given ID and reports whether it
// existed (XDEL).  Consumer-group pending entries for the ID are left in
// place — Redis keeps them so XAUTOCLAIM can report them as deleted; Ack
// and the claim calls remove them.
// Complexity is O(log₂ n) expected.
func (s *Stream) Delete(id ID) bool {
	if s == nil {
		return false
	}
	return s.entries.Delete(Entry{ID: id})
}

// TrimMaxLen evicts the oldest entries so that at most maxLen remain,
// returning how many were evicted (XTRIM MAXLEN).  maxLen ≤ 0 empties the
// stream.  The last assigned ID and every group PEL are untouched.
// Complexity is O(log₂ n + k) where k is the number evicted.
func (s *Stream) TrimMaxLen(maxLen int) int {
	if s == nil {
		return 0
	}
	if maxLen < 0 {
		maxLen = 0
	}
	if evict := s.entries.Len() - maxLen; evict > 0 {
		return s.entries.DeleteByRank(0, evict-1)
	}
	return 0
}

// TrimMinID evicts every entry with an ID below min, returning how many
// were evicted (XTRIM MINID; the bound is exclusive).  The last assigned
// ID and every group PEL are untouched.
// Complexity is O(log₂ n + k) where k is the number evicted.
func (s *Stream) TrimMinID(min ID) int {
	if s == nil {
		return 0
	}
	below, ok := prevID(min)
	if !ok {
		return 0 // min is MinID — nothing sorts below it
	}
	return s.entries.DeleteRange(Entry{ID: MinID}, Entry{ID: below})
}

// SetLastID sets the stream's last assigned ID (XSETID) — used when
// pre-populating a stream so the next Add starts above a chosen ID.
//
// The set is unconditional: Redis's XSETID rejects an ID smaller than the
// stream's last ID, and callers wanting that check compare against
// LastID first.  Add keeps enforcing strictly-greater-than whatever the
// last ID is, and the entry storage stays ordered, so a backwards set
// cannot corrupt the stream — it only means later Adds may interleave
// below older entries.
// Complexity is O(1).
func (s *Stream) SetLastID(id ID) {
	if s == nil {
		return
	}
	s.last = id
}

// Lock is a no-op kept so code written against the stream_ts twin
// compiles unchanged; the plain stream has no lock.
func (s *Stream) Lock() {}

// Unlock is a no-op kept so code written against the stream_ts twin
// compiles unchanged.
func (s *Stream) Unlock() {}

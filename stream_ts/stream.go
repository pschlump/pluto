/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stream_ts implements the append-oriented ID-keyed entry log
// with consumer groups that is safe for concurrent use.  It is the
// thread-safe twin of github.com/pschlump/pluto/stream — the Redis
// stream of XADD/XRANGE/XREAD/XGROUP as a plain Go data structure —
// with the identical API guarded by a sync.RWMutex, plus the Lock and
// Unlock pair and the Nl-prefixed (no-lock) methods for compound
// operations.  The ID, Entry and PendingEntry types are aliases of the
// stream package's, so switching between the twins is an import change.
//
// Concurrency model:
//
// Reads (Len, IsEmpty, FirstID, LastID, GroupNames, GroupLastID,
// GroupConsumers, Pending, PendingRange) take the read lock and release
// it before returning, so they run in parallel with each other.
// Writes (Add, Delete, the trims, SetLastID, the whole group surface)
// take the write lock.  ReadGroup, Claim and AutoClaim also take the
// write lock — they move entries onto pending lists.
//
// Range and RevRange take an eager snapshot under the read lock: the
// matching entries are collected into a slice while the lock is held and
// yielded after it is released.  They are safe to use concurrently with
// any stream operation — including mutating the stream from inside the
// loop — and never observe later modifications.
//
// The zero value of Stream is an empty stream ready to use (including
// Add) — no constructor.  A nil *Stream behaves as an empty stream for
// every operation except Add and CreateGroup, which panic naming the
// method; the nil guards run before any lock acquisition.  These are
// the package's only panics.
//
// See the stream package documentation for the data-structure contracts
// (ID monotonicity, the AutoSeq sentinel, group delivery semantics, the
// PEL) — this twin changes only the concurrency.
//
// Run the tests with -race.
package stream_ts

import (
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/pschlump/pluto/stream"
)

// ID, Entry and PendingEntry are the stream package's types, aliased so
// that code written against either twin compiles against the other.
type (
	ID           = stream.ID
	Entry        = stream.Entry
	PendingEntry = stream.PendingEntry
)

// AutoSeq is the sequence-part sentinel for Add — see stream.AutoSeq.
const AutoSeq = stream.AutoSeq

// The stream package's errors and sentinels, re-exported for the same
// drop-in reason (compare with errors.Is across either twin).
var (
	ErrInvalidID   = stream.ErrInvalidID
	ErrIDTooSmall  = stream.ErrIDTooSmall
	ErrIDExhausted = stream.ErrIDExhausted
	ErrGroupExists = stream.ErrGroupExists

	MinID = stream.MinID
	MaxID = stream.MaxID
)

// ParseID parses a "ms-seq", "ms" or "ms-*" ID — see stream.ParseID.
func ParseID(s string) (ID, error) { return stream.ParseID(s) }

// CompareID orders two IDs — see stream.CompareID.
func CompareID(a, b ID) int { return stream.CompareID(a, b) }

// Stream is an append-oriented log of entries keyed by strictly
// increasing IDs, with consumer groups, safe for concurrent use.  It is
// the stream package's Stream behind one sync.RWMutex — the inner
// stream is a plain value guarded by this structure's lock (the
// hash_tab_bt_ts pattern: no locks inside the borrowed structure).
//
// The zero value is an empty stream ready to use.
type Stream struct {
	inner stream.Stream
	lock  sync.RWMutex
}

// Lock takes the write lock and is the entry point for compound
// operations: Lock, then the Nl-prefixed methods, then Unlock.  Calling
// a regular (locking) method while the lock is held deadlocks.  A no-op
// on a nil *Stream.
func (s *Stream) Lock() {
	if s == nil {
		return
	}
	s.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A no-op on a nil
// *Stream.
func (s *Stream) Unlock() {
	if s == nil {
		return
	}
	s.lock.Unlock()
}

// Add appends an entry and returns the ID assigned to it — see
// stream.Stream.Add for the AutoSeq and monotonicity contracts.
// It panics on a nil *Stream (before any lock acquisition).
// Complexity is O(log₂ n) expected.
func (s *Stream) Add(id ID, fields [][2]string) (ID, error) {
	if s == nil {
		panic("stream_ts: Add called on a nil Stream (use a stream value, not a nil pointer)")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlAdd(id, fields)
}

// NlAdd is Add without locking; call it only while holding Lock.
// Complexity is O(log₂ n) expected.
func (s *Stream) NlAdd(id ID, fields [][2]string) (ID, error) {
	return s.inner.Add(id, fields)
}

// IsEmpty reports whether the stream has no entries.
// Complexity is O(1).
func (s *Stream) IsEmpty() bool {
	return s.Len() == 0
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (s *Stream) NlIsEmpty() bool {
	return s.inner.IsEmpty()
}

// Len returns the number of entries in the stream.
// Complexity is O(1).
func (s *Stream) Len() int {
	if s == nil {
		return 0
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlLen()
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (s *Stream) NlLen() int {
	return s.inner.Len()
}

// FirstID returns the smallest entry ID, or false when the stream is
// empty.
// Complexity is O(1).
func (s *Stream) FirstID() (ID, bool) {
	if s == nil {
		return ID{}, false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlFirstID()
}

// NlFirstID is FirstID without locking; call it only while holding Lock.
// Complexity is O(1).
func (s *Stream) NlFirstID() (ID, bool) {
	return s.inner.FirstID()
}

// LastID returns the last assigned ID — see stream.Stream.LastID.
// Complexity is O(1).
func (s *Stream) LastID() ID {
	if s == nil {
		return ID{}
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlLastID()
}

// NlLastID is LastID without locking; call it only while holding Lock.
// Complexity is O(1).
func (s *Stream) NlLastID() ID {
	return s.inner.LastID()
}

// Range returns an iterator over the entries with start ≤ id ≤ end in
// ascending ID order, count-limited when count > 0 — see
// stream.Stream.Range for the bound contracts.
//
// Unlike the plain package's live walk, the result is an eager snapshot
// of the matching entries taken under the read lock at call time: it is
// safe to mutate the stream — even from inside the loop — and later
// modifications are not visible.
// Complexity is O(log₂ n + m).
func (s *Stream) Range(start, end ID, count int) iter.Seq[Entry] {
	if s == nil {
		return func(func(Entry) bool) {} // a nil stream iterates as an empty one
	}
	s.lock.RLock()
	snapshot := s.NlRangeSnapshot(start, end, count)
	s.lock.RUnlock()
	return slices.Values(snapshot)
}

// RevRange is Range in descending ID order — note the (end, start)
// parameter order, mirroring XREVRANGE.  Like Range it yields an eager
// snapshot taken under the read lock.
// Complexity is O(log₂ n + m).
func (s *Stream) RevRange(end, start ID, count int) iter.Seq[Entry] {
	if s == nil {
		return func(func(Entry) bool) {} // a nil stream iterates as an empty one
	}
	s.lock.RLock()
	snapshot := s.NlRevRangeSnapshot(end, start, count)
	s.lock.RUnlock()
	return slices.Values(snapshot)
}

// NlRangeSnapshot collects the ascending start ≤ id ≤ end window under
// a lock already held (Lock or the read lock); call it only while
// holding one.
// Complexity is O(log₂ n + m).
func (s *Stream) NlRangeSnapshot(start, end ID, count int) []Entry {
	var snapshot []Entry
	for e := range s.inner.Range(start, end, count) {
		snapshot = append(snapshot, e)
	}
	return snapshot
}

// NlRevRangeSnapshot collects the descending end..start window under a
// lock already held; call it only while holding one.
// Complexity is O(log₂ n + m).
func (s *Stream) NlRevRangeSnapshot(end, start ID, count int) []Entry {
	var snapshot []Entry
	for e := range s.inner.RevRange(end, start, count) {
		snapshot = append(snapshot, e)
	}
	return snapshot
}

// Delete removes the entry with the given ID and reports whether it
// existed — see stream.Stream.Delete.
// Complexity is O(log₂ n) expected.
func (s *Stream) Delete(id ID) bool {
	if s == nil {
		return false
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlDelete(id)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// Complexity is O(log₂ n) expected.
func (s *Stream) NlDelete(id ID) bool {
	return s.inner.Delete(id)
}

// TrimMaxLen evicts the oldest entries so that at most maxLen remain —
// see stream.Stream.TrimMaxLen.
// Complexity is O(log₂ n + k).
func (s *Stream) TrimMaxLen(maxLen int) int {
	if s == nil {
		return 0
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlTrimMaxLen(maxLen)
}

// NlTrimMaxLen is TrimMaxLen without locking; call it only while
// holding Lock.
// Complexity is O(log₂ n + k).
func (s *Stream) NlTrimMaxLen(maxLen int) int {
	return s.inner.TrimMaxLen(maxLen)
}

// TrimMinID evicts every entry with an ID below min — see
// stream.Stream.TrimMinID.
// Complexity is O(log₂ n + k).
func (s *Stream) TrimMinID(min ID) int {
	if s == nil {
		return 0
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlTrimMinID(min)
}

// NlTrimMinID is TrimMinID without locking; call it only while holding
// Lock.
// Complexity is O(log₂ n + k).
func (s *Stream) NlTrimMinID(min ID) int {
	return s.inner.TrimMinID(min)
}

// SetLastID sets the stream's last assigned ID — see
// stream.Stream.SetLastID.
// Complexity is O(1).
func (s *Stream) SetLastID(id ID) {
	if s == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.NlSetLastID(id)
}

// NlSetLastID is SetLastID without locking; call it only while holding
// Lock.
// Complexity is O(1).
func (s *Stream) NlSetLastID(id ID) {
	s.inner.SetLastID(id)
}

// CreateGroup creates a consumer group — see stream.Stream.CreateGroup.
// It panics on a nil *Stream (before any lock acquisition).
// Complexity is O(1).
func (s *Stream) CreateGroup(name string, startID ID) error {
	if s == nil {
		panic("stream_ts: CreateGroup called on a nil Stream (use a stream value, not a nil pointer)")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlCreateGroup(name, startID)
}

// NlCreateGroup is CreateGroup without locking; call it only while
// holding Lock.
// Complexity is O(1).
func (s *Stream) NlCreateGroup(name string, startID ID) error {
	return s.inner.CreateGroup(name, startID)
}

// DestroyGroup removes the group and returns how many pending entries
// it dropped — see stream.Stream.DestroyGroup.
// Complexity is O(1).
func (s *Stream) DestroyGroup(name string) int {
	if s == nil {
		return 0
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlDestroyGroup(name)
}

// NlDestroyGroup is DestroyGroup without locking; call it only while
// holding Lock.
// Complexity is O(1).
func (s *Stream) NlDestroyGroup(name string) int {
	return s.inner.DestroyGroup(name)
}

// GroupNames returns the group names in ascending order, or nil when
// there are none.
// Complexity is O(g log g).
func (s *Stream) GroupNames() []string {
	if s == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlGroupNames()
}

// NlGroupNames is GroupNames without locking; call it only while
// holding Lock.
// Complexity is O(g log g).
func (s *Stream) NlGroupNames() []string {
	return s.inner.GroupNames()
}

// GroupSetID sets the group's last-delivered ID — see
// stream.Stream.GroupSetID.
// Complexity is O(1).
func (s *Stream) GroupSetID(name string, id ID) {
	if s == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.NlGroupSetID(name, id)
}

// NlGroupSetID is GroupSetID without locking; call it only while
// holding Lock.
// Complexity is O(1).
func (s *Stream) NlGroupSetID(name string, id ID) {
	s.inner.GroupSetID(name, id)
}

// GroupLastID returns the group's last-delivered ID — see
// stream.Stream.GroupLastID.
// Complexity is O(1).
func (s *Stream) GroupLastID(name string) (ID, bool) {
	if s == nil {
		return ID{}, false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlGroupLastID(name)
}

// NlGroupLastID is GroupLastID without locking; call it only while
// holding Lock.
// Complexity is O(1).
func (s *Stream) NlGroupLastID(name string) (ID, bool) {
	return s.inner.GroupLastID(name)
}

// GroupCreateConsumer records a consumer name in the group — see
// stream.Stream.GroupCreateConsumer.
// Complexity is O(1).
func (s *Stream) GroupCreateConsumer(name, consumer string) bool {
	if s == nil {
		return false
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlGroupCreateConsumer(name, consumer)
}

// NlGroupCreateConsumer is GroupCreateConsumer without locking; call it
// only while holding Lock.
// Complexity is O(1).
func (s *Stream) NlGroupCreateConsumer(name, consumer string) bool {
	return s.inner.GroupCreateConsumer(name, consumer)
}

// GroupDeleteConsumer removes the consumer from the group, dropping its
// pending entries — see stream.Stream.GroupDeleteConsumer.
// Complexity is O(p).
func (s *Stream) GroupDeleteConsumer(name, consumer string) int {
	if s == nil {
		return 0
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlGroupDeleteConsumer(name, consumer)
}

// NlGroupDeleteConsumer is GroupDeleteConsumer without locking; call it
// only while holding Lock.
// Complexity is O(p).
func (s *Stream) NlGroupDeleteConsumer(name, consumer string) int {
	return s.inner.GroupDeleteConsumer(name, consumer)
}

// GroupConsumers returns the group's consumer names in ascending order —
// see stream.Stream.GroupConsumers.
// Complexity is O(c log c).
func (s *Stream) GroupConsumers(name string) []string {
	if s == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlGroupConsumers(name)
}

// NlGroupConsumers is GroupConsumers without locking; call it only
// while holding Lock.
// Complexity is O(c log c).
func (s *Stream) NlGroupConsumers(name string) []string {
	return s.inner.GroupConsumers(name)
}

// ReadGroup delivers up to count entries to consumer, moving each onto
// its pending list — see stream.Stream.ReadGroup for the ">" and replay
// forms.  The write lock is held across the read, so a concurrent Add
// cannot interleave into the delivered batch.
// Complexity is O(log₂ n + m·log₂ p) expected.
func (s *Stream) ReadGroup(name, consumer string, after ID, count int) []Entry {
	if s == nil {
		return nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlReadGroup(name, consumer, after, count)
}

// NlReadGroup is ReadGroup without locking; call it only while holding
// Lock.
// Complexity is O(log₂ n + m·log₂ p) expected.
func (s *Stream) NlReadGroup(name, consumer string, after ID, count int) []Entry {
	return s.inner.ReadGroup(name, consumer, after, count)
}

// Ack removes the IDs from the group's pending list — see
// stream.Stream.Ack.
// Complexity is O(k·log₂ p) expected.
func (s *Stream) Ack(name string, ids ...ID) int {
	if s == nil {
		return 0
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlAck(name, ids...)
}

// NlAck is Ack without locking; call it only while holding Lock.
// Complexity is O(k·log₂ p) expected.
func (s *Stream) NlAck(name string, ids ...ID) int {
	return s.inner.Ack(name, ids...)
}

// Pending summarizes the group's pending list — see
// stream.Stream.Pending.
// Complexity is O(p).
func (s *Stream) Pending(name string) (count int, min, max ID, perConsumer map[string]int) {
	if s == nil {
		return 0, ID{}, ID{}, make(map[string]int)
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlPending(name)
}

// NlPending is Pending without locking; call it only while holding
// Lock.
// Complexity is O(p).
func (s *Stream) NlPending(name string) (int, ID, ID, map[string]int) {
	return s.inner.Pending(name)
}

// PendingRange returns the pending entries with start ≤ id ≤ end — see
// stream.Stream.PendingRange.
// Complexity is O(log₂ p + m) expected.
func (s *Stream) PendingRange(name, consumer string, start, end ID, count int) []PendingEntry {
	if s == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.NlPendingRange(name, consumer, start, end, count)
}

// NlPendingRange is PendingRange without locking; call it only while
// holding Lock.
// Complexity is O(log₂ p + m) expected.
func (s *Stream) NlPendingRange(name, consumer string, start, end ID, count int) []PendingEntry {
	return s.inner.PendingRange(name, consumer, start, end, count)
}

// Claim transfers ownership of the pending IDs to consumer — see
// stream.Stream.Claim.
// Complexity is O(k·(log₂ p + log₂ n)) expected.
func (s *Stream) Claim(name, consumer string, minIdle time.Duration, ids ...ID) []Entry {
	if s == nil {
		return nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlClaim(name, consumer, minIdle, ids...)
}

// NlClaim is Claim without locking; call it only while holding Lock.
// Complexity is O(k·(log₂ p + log₂ n)) expected.
func (s *Stream) NlClaim(name, consumer string, minIdle time.Duration, ids ...ID) []Entry {
	return s.inner.Claim(name, consumer, minIdle, ids...)
}

// AutoClaim claims idle pending entries starting from a cursor — see
// stream.Stream.AutoClaim.
// Complexity is O(examined·(log₂ p + log₂ n)) expected.
func (s *Stream) AutoClaim(name, consumer string, minIdle time.Duration, start ID, count int) (entries []Entry, nextStart ID, deletedIDs []ID) {
	if s == nil {
		return nil, ID{}, nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.NlAutoClaim(name, consumer, minIdle, start, count)
}

// NlAutoClaim is AutoClaim without locking; call it only while holding
// Lock.
// Complexity is O(examined·(log₂ p + log₂ n)) expected.
func (s *Stream) NlAutoClaim(name, consumer string, minIdle time.Duration, start ID, count int) ([]Entry, ID, []ID) {
	return s.inner.AutoClaim(name, consumer, minIdle, start, count)
}

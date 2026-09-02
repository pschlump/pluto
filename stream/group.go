/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Consumer groups and their pending-entries lists (PELs) — the XGROUP /
// XREADGROUP / XACK / XPENDING / XCLAIM / XAUTOCLAIM half of a Redis
// stream (note/redis/src/t_stream.c lineage).
//
// Layout: one group per name, holding the group's last-delivered ID, its
// consumer name set, and a single group-wide PEL — a pluto skip_list
// ordered by ID whose nodes carry the owning consumer, the delivery time
// and the delivery count.  Redis keys the PEL by ID in a radix tree and
// separately indexes it per consumer; the ID order is kept here (a skip
// list instead of a rax) and the per-consumer view is derived by walking,
// because the PEL queries the Redis command family actually asks for are
// range-shaped: XPENDING walks an ID range, XAUTOCLAIM scans by ID from a
// cursor, and XCLAIM seeks explicit IDs.  XAUTOCLAIM's min-idle filter
// runs during that ID-ordered scan (capped at autoClaimAttempts × count
// examined entries, as in Redis) rather than through a delivery-time
// index — the scan is O(log p + examined) either way and one structure
// serves every query.

package stream

import (
	"slices"
	"time"

	"github.com/pschlump/pluto/skip_list"
)

// autoClaimAttempts caps how many PEL entries AutoClaim examines at
// autoClaimAttempts × count, the attempt limit of Redis XAUTOCLAIM, so a
// huge PEL of never-idle entries cannot turn one call into a full scan.
const autoClaimAttempts = 10

// PendingEntry is the delivery state of one entry in a group's
// pending-entries list: which consumer owns it, when it was last
// delivered and how many times.
type PendingEntry struct {
	ID            ID
	Consumer      string
	DeliveryTime  time.Time
	DeliveryCount int
}

// pelEntry is one node of a group's PEL, ordered by id.
type pelEntry struct {
	id       ID
	consumer string
	delivery time.Time
	count    int
}

// comparePELEntry orders PEL entries (and therefore probes carrying only
// an ID) by their ID — the comparison the PEL skip list is built with.
func comparePELEntry(a, b pelEntry) int {
	return CompareID(a.id, b.id)
}

// group is one consumer group: the last-delivered ID driving the ">"
// reads, the known consumer names, and the group-wide pending-entries
// list.  The PEL skip list is lazily created on first delivery.
type group struct {
	lastDelivered ID
	consumers     map[string]struct{}
	pel           *skip_list.SkipList[pelEntry]
}

// ensurePEL creates the PEL skip list on first use.
func (g *group) ensurePEL() {
	if g.pel == nil {
		g.pel = skip_list.NewSkipListFunc(comparePELEntry)
	}
}

// lookupGroup returns the named group, or nil for a missing group or a
// nil stream (every group operation tolerates both as "no such group").
func (s *Stream) lookupGroup(name string) *group {
	if s == nil {
		return nil
	}
	return s.groups[name]
}

// CreateGroup creates a consumer group that will deliver entries with an
// ID above startID (XGROUP CREATE): MinID (0-0) sees the whole stream,
// MaxID only entries added after creation (Redis's "$" — callers map it
// to s.LastID() to include the current tail).  It returns ErrGroupExists
// when the name is taken.
//
// It panics on a nil *Stream — one of the package's two panics.
// Complexity is O(1).
func (s *Stream) CreateGroup(name string, startID ID) error {
	if s == nil {
		panic("stream: CreateGroup called on a nil Stream (use a stream value, not a nil pointer)")
	}
	if s.groups == nil {
		s.groups = make(map[string]*group)
	}
	if _, ok := s.groups[name]; ok {
		return ErrGroupExists
	}
	s.groups[name] = &group{
		lastDelivered: startID,
		consumers:     make(map[string]struct{}),
	}
	return nil
}

// DestroyGroup removes the group and returns how many pending entries it
// dropped.  A missing group returns 0.
// Complexity is O(1) (the PEL is dropped with the group).
func (s *Stream) DestroyGroup(name string) int {
	if s == nil {
		return 0
	}
	g, ok := s.groups[name]
	if !ok {
		return 0
	}
	dropped := g.pel.Len()
	delete(s.groups, name)
	return dropped
}

// GroupNames returns the group names in ascending order, or nil when
// there are none.
// Complexity is O(g log g) where g is the number of groups.
func (s *Stream) GroupNames() []string {
	if s == nil || len(s.groups) == 0 {
		return nil
	}
	names := make([]string, 0, len(s.groups))
	for name := range s.groups {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// GroupSetID sets the group's last-delivered ID (XGROUP SETID),
// repositioning where the next ">" read resumes; entries already pending
// stay pending.  A missing group is a no-op (Redis's NOGROUP error is the
// caller's layer).
// Complexity is O(1).
func (s *Stream) GroupSetID(name string, id ID) {
	if g := s.lookupGroup(name); g != nil {
		g.lastDelivered = id
	}
}

// GroupLastID returns the group's last-delivered ID, or false when the
// group does not exist.  With GroupNames, GroupConsumers and Pending it
// is the XINFO GROUPS surface.
// Complexity is O(1).
func (s *Stream) GroupLastID(name string) (ID, bool) {
	if g := s.lookupGroup(name); g != nil {
		return g.lastDelivered, true
	}
	return ID{}, false
}

// GroupCreateConsumer records a consumer name in the group (XGROUP
// CREATECONSUMER) and returns true; false when the group is missing or
// the name already exists.  Consumers are also created implicitly by
// ReadGroup, Claim and AutoClaim.
// Complexity is O(1).
func (s *Stream) GroupCreateConsumer(name, consumer string) bool {
	g := s.lookupGroup(name)
	if g == nil {
		return false
	}
	if _, ok := g.consumers[consumer]; ok {
		return false
	}
	g.consumers[consumer] = struct{}{}
	return true
}

// GroupDeleteConsumer removes the consumer from the group, dropping its
// pending entries, and returns how many were dropped (XGROUP
// DELCONSUMER).  A missing group or consumer returns 0.
// Complexity is O(p) where p is the size of the group's PEL.
func (s *Stream) GroupDeleteConsumer(name, consumer string) int {
	g := s.lookupGroup(name)
	if g == nil {
		return 0
	}
	if _, ok := g.consumers[consumer]; !ok {
		return 0
	}
	dropped := 0
	if g.pel != nil {
		// Collect first, delete after the walk — the PEL iterator walks
		// live nodes and must not be mutated during consumption.
		var toDrop []ID
		for pe := range g.pel.All() {
			if pe.consumer == consumer {
				toDrop = append(toDrop, pe.id)
			}
		}
		for _, id := range toDrop {
			g.pel.Delete(pelEntry{id: id})
		}
		dropped = len(toDrop)
	}
	delete(g.consumers, consumer)
	return dropped
}

// GroupConsumers returns the group's consumer names in ascending order,
// or nil when the group does not exist (the XINFO CONSUMERS surface;
// consumers with no pending entries are included).
// Complexity is O(c log c) where c is the number of consumers.
func (s *Stream) GroupConsumers(name string) []string {
	g := s.lookupGroup(name)
	if g == nil {
		return nil
	}
	names := make([]string, 0, len(g.consumers))
	for name := range g.consumers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ReadGroup delivers up to count entries to consumer, moving each onto
// its pending list, and returns them (XREADGROUP).
//
// The after ID selects the delivery floor.  The zero value MinID (0-0)
// is the Redis ">" form: delivery starts above the group's
// last-delivered ID and, when anything is delivered, the last-delivered
// ID advances to the newest entry handed out — entries already pending
// in the group are never re-delivered.  Any other after value is a
// replay form: delivery starts above that ID, the group's
// last-delivered ID is left unchanged, and still only entries not
// currently pending are handed out.  A count ≤ 0 means no limit.
//
// The consumer is created if unknown (Redis behavior).  A missing group
// returns nil (Redis's NOGROUP error is the caller's layer).  Reading
// back a consumer's own pending history without re-delivering is what
// PendingRange is for — the explicit-ID form of XREADGROUP in Redis
// terms.
// Complexity is O(log₂ n + m·log₂ p) expected, where m is the number of
// entries examined and p the PEL size.
func (s *Stream) ReadGroup(name, consumer string, after ID, count int) []Entry {
	g := s.lookupGroup(name)
	if g == nil {
		return nil
	}
	auto := after == MinID // 0-0 selects the ">" form
	floor := after
	if auto {
		floor = g.lastDelivered
	}
	g.consumers[consumer] = struct{}{} // a read attempt registers the consumer
	if CompareID(floor, MaxID) >= 0 {
		return nil // nothing sorts above MaxID
	}
	g.ensurePEL()
	now := time.Now()
	var delivered []Entry
	for _, e := range s.entries.Range(Entry{ID: nextID(floor)}, Entry{ID: MaxID}) {
		if _, pending := g.pel.Search(pelEntry{id: e.ID}); pending {
			continue // never re-deliver a live pending entry
		}
		g.pel.Insert(pelEntry{id: e.ID, consumer: consumer, delivery: now, count: 1})
		delivered = append(delivered, e)
		if count > 0 && len(delivered) >= count {
			break
		}
	}
	if auto && len(delivered) > 0 {
		g.lastDelivered = delivered[len(delivered)-1].ID
	}
	return delivered
}

// Ack removes the IDs from the group's pending list (XACK) and returns
// how many were actually pending.  IDs whose entries were deleted from
// the stream are still PEL entries until acknowledged — this is what
// clears them.
// Complexity is O(k·log₂ p) expected for k IDs.
func (s *Stream) Ack(name string, ids ...ID) int {
	g := s.lookupGroup(name)
	if g == nil || len(ids) == 0 {
		return 0
	}
	acked := 0
	for _, id := range ids {
		if g.pel.Delete(pelEntry{id: id}) {
			acked++
		}
	}
	return acked
}

// Pending summarizes the group's pending list: the number of entries,
// the smallest and largest pending IDs (both MinID when none), and the
// per-consumer counts (the XPENDING summary form).
// Complexity is O(p) where p is the PEL size.
func (s *Stream) Pending(name string) (count int, min, max ID, perConsumer map[string]int) {
	g := s.lookupGroup(name)
	perConsumer = make(map[string]int)
	if g == nil {
		return 0, MinID, MinID, perConsumer
	}
	first := true
	for pe := range g.pel.All() {
		if first {
			min = pe.id
			first = false
		}
		max = pe.id
		perConsumer[pe.consumer]++
		count++
	}
	return count, min, max, perConsumer
}

// PendingRange returns the pending entries with start ≤ id ≤ end in
// ascending ID order (the XPENDING range form), optionally restricted to
// one consumer when consumer is non-empty.  A count ≤ 0 means no limit.
// A missing group returns nil.
// Complexity is O(log₂ p + m) expected.
func (s *Stream) PendingRange(name, consumer string, start, end ID, count int) []PendingEntry {
	g := s.lookupGroup(name)
	if g == nil {
		return nil
	}
	var out []PendingEntry
	n := 0
	for _, pe := range g.pel.Range(pelEntry{id: start}, pelEntry{id: end}) {
		if consumer != "" && pe.consumer != consumer {
			continue
		}
		out = append(out, PendingEntry{
			ID:            pe.id,
			Consumer:      pe.consumer,
			DeliveryTime:  pe.delivery,
			DeliveryCount: pe.count,
		})
		n++
		if count > 0 && n >= count {
			break
		}
	}
	return out
}

// Claim transfers ownership of the pending IDs to consumer, provided
// they have been pending at least minIdle (XCLAIM; minIdle 0 claims
// unconditionally) and returns the claimed entries.
//
// Each successful claim resets the delivery time and increments the
// delivery count.  A pending ID whose entry no longer exists in the
// stream is removed from the PEL and not returned — trimmed or deleted
// entries are collected by AutoClaim instead, which reports them.  IDs
// that are not pending (or not idle enough) are skipped silently.
// Redis's FORCE and JUSTID options are caller-level concepts.  The
// consumer is created if unknown; a missing group returns nil.
// Complexity is O(k·(log₂ p + log₂ n)) expected for k IDs.
func (s *Stream) Claim(name, consumer string, minIdle time.Duration, ids ...ID) []Entry {
	g := s.lookupGroup(name)
	if g == nil || len(ids) == 0 {
		return nil
	}
	g.consumers[consumer] = struct{}{}
	now := time.Now()
	var claimed []Entry
	for _, id := range ids {
		pe, ok := g.pel.Search(pelEntry{id: id})
		if !ok || now.Sub(pe.delivery) < minIdle {
			continue
		}
		if e, exists := s.entries.Search(Entry{ID: id}); exists {
			g.pel.Insert(pelEntry{id: id, consumer: consumer, delivery: now, count: pe.count + 1})
			claimed = append(claimed, e)
		} else {
			g.pel.Delete(pelEntry{id: id}) // entry vanished: drop the dangling PEL entry
		}
	}
	return claimed
}

// AutoClaim claims pending entries that have been idle at least minIdle
// starting from the start ID (inclusive, in ID order) and hands them to
// consumer (XAUTOCLAIM).  It scans the PEL in ID order, examining at
// most autoClaimAttempts × count entries when count > 0 (Redis's attempt
// limit); a count ≤ 0 means no limit and no cap.
//
// It returns the claimed entries; nextStart, the ID to continue scanning
// from on the next call (MinID when the PEL is exhausted); and the IDs
// whose pending entries were dropped because their stream entries no
// longer exist — the deleted array Redis reports so callers can react to
// trimmed or deleted data.  The consumer is created if unknown; a
// missing group returns (nil, MinID, nil).
// Complexity is O(examined·(log₂ p + log₂ n)) expected.
func (s *Stream) AutoClaim(name, consumer string, minIdle time.Duration, start ID, count int) (entries []Entry, nextStart ID, deletedIDs []ID) {
	g := s.lookupGroup(name)
	if g == nil {
		return nil, MinID, nil
	}
	limit := 0 // examined-entry cap; 0 = uncapped
	if count > 0 {
		limit = count * autoClaimAttempts
	}
	g.consumers[consumer] = struct{}{}
	now := time.Now()
	var (
		claimedEntries []Entry
		claimedPE      []pelEntry
		drop           []ID
		examined       int
		lastExamined   ID
	)
	for _, pe := range g.pel.Range(pelEntry{id: start}, pelEntry{id: MaxID}) {
		if limit > 0 && examined >= limit {
			break
		}
		examined++
		lastExamined = pe.id
		if now.Sub(pe.delivery) < minIdle {
			continue
		}
		if e, exists := s.entries.Search(Entry{ID: pe.id}); exists {
			claimedPE = append(claimedPE, pelEntry{id: pe.id, consumer: consumer, delivery: now, count: pe.count + 1})
			claimedEntries = append(claimedEntries, e)
			if count > 0 && len(claimedEntries) >= count {
				break
			}
		} else {
			drop = append(drop, pe.id)
		}
	}
	// Apply the mutations after the walk — the PEL iterator walks live
	// nodes (Insert-on-existing only replaces data in place, but deferring
	// keeps that an implementation detail of the skip list, not a
	// dependency of this loop).
	for _, pe := range claimedPE {
		g.pel.Insert(pe)
	}
	for _, id := range drop {
		g.pel.Delete(pelEntry{id: id})
	}
	if examined > 0 {
		if succ, ok := g.pel.Ceil(pelEntry{id: nextID(lastExamined)}); ok {
			nextStart = succ.id
		}
	}
	return claimedEntries, nextStart, drop
}

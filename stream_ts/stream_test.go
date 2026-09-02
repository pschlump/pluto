/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream_ts

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pschlump/pluto/stream"
)

// The twin must be a drop-in for the plain package: the aliased types
// are identical, so plain-stream helpers type-check against it.
var (
	_ stream.ID             = ID{}
	_ []stream.Entry        = []Entry{}
	_ []stream.PendingEntry = []PendingEntry{}
)

// expectPanic runs fn and requires a panic message starting with want.
func expectPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic %q, got none", want)
			return
		}
		got, ok := r.(string)
		if !ok || !strings.HasPrefix(got, want) {
			t.Errorf("panic = %v, want prefix %q", r, want)
		}
	}()
	fn()
}

func TestBasicBehavior(t *testing.T) {
	var s Stream // zero value ready to use
	for i := range 5 {
		if _, err := s.Add(ID{Ms: 1, Seq: uint64(i)}, [][2]string{{"f", "v"}}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if _, err := s.Add(ID{Ms: 1, Seq: 2}, nil); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("monotonicity not enforced: %v", err)
	}
	if s.Len() != 5 || s.LastID() != (ID{Ms: 1, Seq: 4}) {
		t.Errorf("Len %d LastID %v", s.Len(), s.LastID())
	}

	// The auto-seq form.
	if got, err := s.Add(ID{Ms: 2, Seq: AutoSeq}, nil); err != nil || got != (ID{Ms: 2}) {
		t.Errorf("auto seq: (%v, %v)", got, err)
	}

	// Range and RevRange.
	n := 0
	for e := range s.Range(MinID, MaxID, 0) {
		n++
		_ = e
	}
	if n != 6 {
		t.Errorf("Range visited %d", n)
	}
	n = 0
	for e := range s.RevRange(MaxID, MinID, 2) {
		n++
		_ = e
	}
	if n != 2 {
		t.Errorf("RevRange visited %d", n)
	}

	// Trims and delete.
	if n := s.TrimMaxLen(3); n != 3 {
		t.Errorf("TrimMaxLen evicted %d", n)
	}
	if !s.Delete(ID{Ms: 2}) {
		t.Error("Delete should report true")
	}

	// The group surface.
	if err := s.CreateGroup("g", MinID); err != nil {
		t.Fatal(err)
	}
	if got := s.ReadGroup("g", "c1", MinID, 10); len(got) != 2 {
		t.Errorf("ReadGroup delivered %v", got)
	}
	if n := s.Ack("g", ID{Ms: 1, Seq: 4}); n != 1 {
		t.Errorf("Ack = %d", n)
	}
	if count, _, _, per := s.Pending("g"); count != 1 || per["c1"] != 1 {
		t.Errorf("Pending = %d %v", count, per)
	}
	if pes := s.PendingRange("g", "", MinID, MaxID, 0); len(pes) != 1 || pes[0].Consumer != "c1" {
		t.Errorf("PendingRange = %v", pes)
	}
	entries, next, deleted := s.AutoClaim("g", "c2", 0, MinID, 10)
	if len(entries) != 1 || entries[0].ID != (ID{Ms: 1, Seq: 3}) || next != MinID || len(deleted) != 0 {
		t.Errorf("AutoClaim = (%v, %v, %v)", entries, next, deleted)
	}
	if got := s.Claim("g", "c3", 0, ID{Ms: 1, Seq: 3}); len(got) != 1 {
		t.Errorf("Claim = %v", got)
	}
	if names := s.GroupNames(); len(names) != 1 || names[0] != "g" {
		t.Errorf("GroupNames = %v", names)
	}
	if last, ok := s.GroupLastID("g"); !ok || last != (ID{Ms: 1, Seq: 4}) {
		t.Errorf("GroupLastID = (%v, %v)", last, ok)
	}
	if !s.GroupCreateConsumer("g", "c4") {
		t.Error("GroupCreateConsumer")
	}
	if cons := s.GroupConsumers("g"); len(cons) != 4 {
		t.Errorf("GroupConsumers = %v", cons)
	}
	if n := s.GroupDeleteConsumer("g", "c3"); n != 1 {
		t.Errorf("GroupDeleteConsumer dropped %d", n)
	}
	s.GroupSetID("g", MinID)
	// c3's pending entry was dropped above, so the group PEL is empty.
	if n := s.DestroyGroup("g"); n != 0 {
		t.Errorf("DestroyGroup dropped %d", n)
	}

	// The re-exported ID helpers work through the twin.
	id, err := ParseID("42-7")
	if err != nil || id != (ID{Ms: 42, Seq: 7}) || id.String() != "42-7" {
		t.Errorf("ParseID through twin = (%v, %v)", id, err)
	}
	if CompareID(MinID, MaxID) >= 0 {
		t.Error("CompareID through twin")
	}
}

func TestSnapshotIterators(t *testing.T) {
	var s Stream
	for i := range 5 {
		_, _ = s.Add(ID{Ms: 1, Seq: uint64(i)}, nil)
	}

	// The iterator is an eager snapshot: adds and deletes after the call
	// (including from inside the loop) are not visible.
	n := 0
	for e := range s.Range(MinID, MaxID, 0) {
		_ = e
		if n == 0 {
			_, _ = s.Add(ID{Ms: 2}, nil)
			s.Delete(ID{Ms: 1})
		}
		n++
	}
	if n != 5 {
		t.Errorf("snapshot Range visited %d, want the 5 present at call time", n)
	}
	if s.Len() != 5 { // 5 - 1 deleted + 1 added
		t.Errorf("Len after loop = %d, want 5", s.Len())
	}

	rev := 0
	for e := range s.RevRange(MaxID, MinID, 0) {
		_ = e
		rev++
	}
	if rev != 5 {
		t.Errorf("snapshot RevRange visited %d", rev)
	}
}

func TestNilStreamTolerated(t *testing.T) {
	var s *Stream
	if s.Len() != 0 || !s.IsEmpty() {
		t.Error("nil stream should be empty")
	}
	if _, ok := s.FirstID(); ok {
		t.Error("FirstID on nil")
	}
	if s.LastID() != MinID {
		t.Error("LastID on nil")
	}
	seen := 0
	for range s.Range(MinID, MaxID, 0) {
		seen++
	}
	for range s.RevRange(MaxID, MinID, 0) {
		seen++
	}
	if seen != 0 {
		t.Error("iterators on nil should visit nothing")
	}
	if s.Delete(ID{Ms: 1}) || s.TrimMaxLen(0) != 0 || s.TrimMinID(MaxID) != 0 {
		t.Error("mutations on nil should report nothing")
	}
	s.SetLastID(ID{Ms: 9})
	s.GroupSetID("g", MinID)
	s.Lock() // no-ops on nil
	_ = s.Len()
	s.Unlock()
	if s.GroupNames() != nil || s.DestroyGroup("g") != 0 || s.ReadGroup("g", "c", MinID, 1) != nil ||
		s.Ack("g", ID{Ms: 1}) != 0 || s.PendingRange("g", "", MinID, MaxID, 0) != nil ||
		s.Claim("g", "c", 0, ID{Ms: 1}) != nil {
		t.Error("group surface on nil should report empties")
	}
	if e, n, d := s.AutoClaim("g", "c", 0, MinID, 1); e != nil || n != MinID || d != nil {
		t.Error("AutoClaim on nil")
	}
	if _, ok := s.GroupLastID("g"); ok {
		t.Error("GroupLastID on nil")
	}
	if s.GroupCreateConsumer("g", "c") || s.GroupDeleteConsumer("g", "c") != 0 || s.GroupConsumers("g") != nil {
		t.Error("consumer surface on nil")
	}
	if count, _, _, per := s.Pending("g"); count != 0 || per == nil {
		t.Error("Pending on nil")
	}

	expectPanic(t, "stream_ts: Add called on a nil Stream", func() { _, _ = s.Add(ID{Ms: 1}, nil) })
	expectPanic(t, "stream_ts: CreateGroup called on a nil Stream", func() { _ = s.CreateGroup("g", MinID) })
}

func TestMinIdleGate(t *testing.T) {
	// A claim under a large minIdle changes nothing; minIdle 0 does —
	// timing-independent, as in the plain package's tests.
	var s Stream
	for i := range 3 {
		_, _ = s.Add(ID{Ms: 1, Seq: uint64(i)}, nil)
	}
	_ = s.CreateGroup("g", MinID)
	_ = s.ReadGroup("g", "c1", MinID, 3)
	if got := s.Claim("g", "c2", time.Hour, ID{Ms: 1}); len(got) != 0 {
		t.Errorf("Claim under minIdle = %v", got)
	}
	if got := s.Claim("g", "c2", 0, ID{Ms: 1}); len(got) != 1 {
		t.Errorf("Claim with minIdle 0 = %v", got)
	}
}

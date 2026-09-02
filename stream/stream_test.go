/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stream

import (
	"errors"
	"math"
	"testing"
)

// mustAdd adds id (already resolved — no AutoSeq) and fails the test on
// the ID rules being rejected.
func mustAdd(t *testing.T, s *Stream, id ID, fields ...[2]string) ID {
	t.Helper()
	got, err := s.Add(id, fields)
	if err != nil {
		t.Fatalf("Add(%s): unexpected error: %v", id, err)
	}
	return got
}

func TestParseID(t *testing.T) {
	valid := []struct {
		in   string
		want ID
	}{
		{"1234-56", ID{Ms: 1234, Seq: 56}},
		{"0-0", ID{}},
		{"1234", ID{Ms: 1234}},
		{"0", ID{}},
		{"1234-*", ID{Ms: 1234, Seq: AutoSeq}},
		{"18446744073709551615-18446744073709551615", MaxID},
	}
	for _, tc := range valid {
		got, err := ParseID(tc.in)
		if err != nil {
			t.Errorf("ParseID(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	invalid := []string{
		"",               // empty
		"-",              // no ms
		"-56",            // no ms
		"1234-",          // no seq
		"a-56", "1234-b", // non-digits
		" 1234", "1234 ", "12 34", // whitespace
		"+1234", "1234-+56", // signs strconv would accept
		"1234-56-7",              // extra dash
		"99999999999999999999",   // ms overflows uint64
		"1-99999999999999999999", // seq overflows uint64
	}
	for _, in := range invalid {
		if _, err := ParseID(in); !errors.Is(err, ErrInvalidID) {
			t.Errorf("ParseID(%q): got err %v, want ErrInvalidID", in, err)
		}
	}
}

func TestIDString(t *testing.T) {
	cases := []struct {
		id   ID
		want string
	}{
		{ID{}, "0-0"},
		{ID{Ms: 1234, Seq: 56}, "1234-56"},
		{ID{Ms: 5, Seq: AutoSeq}, "5-*"},
		// MaxID's Seq is the AutoSeq sentinel value, so it prints in the
		// request form (numerically it round-trips, like every other ID).
		{MaxID, "18446744073709551615-*"},
	}
	for _, tc := range cases {
		if got := tc.id.String(); got != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.id, got, tc.want)
		}
	}
	// Every non-sentinel ID round-trips through String and ParseID.
	for _, id := range []ID{{}, {Ms: 1}, {Ms: 1, Seq: 2}, {Ms: 0, Seq: 7}, MaxID} {
		back, err := ParseID(id.String())
		if err != nil || back != id {
			t.Errorf("round trip of %v: got (%v, %v)", id, back, err)
		}
	}
}

func TestCompareID(t *testing.T) {
	less := [][2]ID{
		{{Ms: 1, Seq: 5}, {Ms: 2, Seq: 0}}, // ms dominates
		{{Ms: 1, Seq: 5}, {Ms: 1, Seq: 6}}, // seq breaks ties
		{{}, {Ms: 0, Seq: 1}},              // the start sentinel is smallest
		{{Ms: 0, Seq: math.MaxUint64}, {Ms: 1, Seq: 0}},
	}
	for _, pair := range less {
		if CompareID(pair[0], pair[1]) >= 0 {
			t.Errorf("CompareID(%v, %v) should be negative", pair[0], pair[1])
		}
		if CompareID(pair[1], pair[0]) <= 0 {
			t.Errorf("CompareID(%v, %v) should be positive", pair[1], pair[0])
		}
	}
	if CompareID(ID{Ms: 7, Seq: 8}, ID{Ms: 7, Seq: 8}) != 0 {
		t.Error("CompareID of equal IDs should be 0")
	}
}

func TestNextPrevID(t *testing.T) {
	if got := nextID(ID{Ms: 5, Seq: 7}); got != (ID{Ms: 5, Seq: 8}) {
		t.Errorf("nextID(5-7) = %v", got)
	}
	if got := nextID(ID{Ms: 5, Seq: math.MaxUint64}); got != (ID{Ms: 6}) {
		t.Errorf("nextID(5-max) = %v", got)
	}
	if got := nextID(ID{Ms: 5}); got != (ID{Ms: 5, Seq: 1}) {
		t.Errorf("nextID(5-0) = %v", got)
	}
	if got, ok := prevID(ID{Ms: 5}); got != (ID{Ms: 4, Seq: math.MaxUint64}) || !ok {
		t.Errorf("prevID(5-0) = %v, %v", got, ok)
	}
	if got, ok := prevID(ID{Ms: 5, Seq: 9}); got != (ID{Ms: 5, Seq: 8}) || !ok {
		t.Errorf("prevID(5-9) = %v, %v", got, ok)
	}
	if _, ok := prevID(MinID); ok {
		t.Error("prevID(0-0) should report nothing below")
	}
}

func TestAddMonotonicity(t *testing.T) {
	s := &Stream{}

	// 0-0 is never a valid entry ID, not even on an empty stream.
	if _, err := s.Add(MinID, nil); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("Add(0-0) on empty: got %v, want ErrIDTooSmall", err)
	}
	// An explicit 0-1 on an empty stream is allowed.
	if got := mustAdd(t, s, ID{Ms: 0, Seq: 1}); got != (ID{Ms: 0, Seq: 1}) {
		t.Fatalf("Add(0-1) returned %v", got)
	}
	// Equal and smaller IDs are rejected.
	for _, id := range []ID{{Ms: 0, Seq: 1}, {Ms: 0, Seq: 0}, MinID} {
		if _, err := s.Add(id, nil); !errors.Is(err, ErrIDTooSmall) {
			t.Errorf("Add(%v): got %v, want ErrIDTooSmall", id, err)
		}
	}
	// An explicit Seq of math.MaxUint64 IS the AutoSeq sentinel: it
	// resolves to last.Seq+1 rather than being stored literally.
	if got, err := s.Add(ID{Ms: 0, Seq: math.MaxUint64}, nil); err != nil || got != (ID{Ms: 0, Seq: 2}) {
		t.Errorf("max-uint64 seq as auto form: (%v, %v), want 0-2", got, err)
	}
	if _, err := s.Add(ID{Ms: 1}, nil); err != nil {
		t.Errorf("Add(1-0): %v", err)
	}
	// A lower ms part with an explicit seq is rejected.
	if _, err := s.Add(ID{Ms: 0, Seq: 42}, nil); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("Add(0-42) after 1-0: got %v, want ErrIDTooSmall", err)
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3 (rejected Adds must not store)", s.Len())
	}
}

func TestAddAutoSeq(t *testing.T) {
	s := &Stream{}
	// Empty stream: last is 0-0, so 5-* resolves to 5-0.
	for i, want := range []ID{{Ms: 5}, {Ms: 5, Seq: 1}, {Ms: 5, Seq: 2}, {Ms: 6}} {
		got, err := s.Add(ID{Ms: want.Ms, Seq: AutoSeq}, nil)
		if err != nil {
			t.Fatalf("auto seq %d: %v", i, err)
		}
		if got != want {
			t.Errorf("auto seq %d: got %v, want %v", i, got, want)
		}
	}
	// Auto below the last ms resolves to ms-0, which fails the check.
	if _, err := s.Add(ID{Ms: 5, Seq: AutoSeq}, nil); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("auto seq below last ms: got %v, want ErrIDTooSmall", err)
	}
	// Exhaustion: bump the last ID to ms-max and ask for the next seq.
	s.SetLastID(ID{Ms: 6, Seq: math.MaxUint64})
	if _, err := s.Add(ID{Ms: 6, Seq: AutoSeq}, nil); !errors.Is(err, ErrIDExhausted) {
		t.Errorf("auto seq overflow: got %v, want ErrIDExhausted", err)
	}
	// A later ms still works.
	if got, err := s.Add(ID{Ms: 7, Seq: AutoSeq}, nil); err != nil || got != (ID{Ms: 7}) {
		t.Errorf("auto seq after overflow ms: (%v, %v)", got, err)
	}
	// Auto on the empty stream's 0 ms bumps the 0-0 last ID to 0-1.
	if got, err := (&Stream{}).Add(ID{Seq: AutoSeq}, nil); err != nil || got != (ID{Seq: 1}) {
		t.Errorf("0-* on empty: (%v, %v), want 0-1", got, err)
	}
}

func TestAddCopiesFields(t *testing.T) {
	s := &Stream{}
	fields := [][2]string{{"f1", "v1"}, {"f1", "v2"}} // duplicate names allowed
	mustAdd(t, s, ID{Ms: 1}, fields...)
	// Mutating the caller's slice afterwards must not leak into the copy.
	fields[0] = [2]string{"mutated", "mutated"}

	for e := range s.Range(MinID, MaxID, 0) {
		want := [][2]string{{"f1", "v1"}, {"f1", "v2"}}
		if len(e.Fields) != 2 || e.Fields[0] != want[0] || e.Fields[1] != want[1] {
			t.Errorf("stored fields = %v, want %v", e.Fields, want)
		}
	}
	// nil fields are stored as nil.
	mustAdd(t, s, ID{Ms: 2})
	for e := range s.Range(ID{Ms: 2}, MaxID, 0) {
		if e.Fields != nil {
			t.Errorf("nil fields stored as %v", e.Fields)
		}
	}
}

func idsOf(es []Entry) []ID {
	out := make([]ID, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

func TestLenFirstLastID(t *testing.T) {
	var s Stream // zero value ready to use
	if s.Len() != 0 || !s.IsEmpty() {
		t.Error("fresh stream should be empty")
	}
	if id, ok := s.FirstID(); ok {
		t.Errorf("FirstID on empty = (%v, %v)", id, ok)
	}
	if s.LastID() != MinID {
		t.Errorf("LastID on empty = %v, want 0-0", s.LastID())
	}

	mustAdd(t, &s, ID{Ms: 1})
	mustAdd(t, &s, ID{Ms: 2})
	mustAdd(t, &s, ID{Ms: 3})
	if id, _ := s.FirstID(); id != (ID{Ms: 1}) {
		t.Errorf("FirstID = %v", id)
	}
	if s.LastID() != (ID{Ms: 3}) {
		t.Errorf("LastID = %v", s.LastID())
	}

	// The last ID does not regress on delete or trim.
	s.Delete(ID{Ms: 3})
	if s.LastID() != (ID{Ms: 3}) {
		t.Errorf("LastID after delete = %v", s.LastID())
	}
	s.TrimMaxLen(1)
	if s.LastID() != (ID{Ms: 3}) {
		t.Errorf("LastID after trim = %v", s.LastID())
	}
	if s.Len() != 1 {
		t.Errorf("Len after trim = %d", s.Len())
	}
}

func TestRange(t *testing.T) {
	s := &Stream{}
	for i := range 10 {
		mustAdd(t, s, ID{Ms: 1, Seq: uint64(i)})
	}

	got := idsOf(collect(s.Range(MinID, MaxID, 0)))
	want := []ID{}
	for i := range 10 {
		want = append(want, ID{Ms: 1, Seq: uint64(i)})
	}
	if len(got) != 10 || got[0] != want[0] || got[9] != want[9] {
		t.Errorf("full range = %v", got)
	}

	// Inclusive bounds.
	got = idsOf(collect(s.Range(ID{Ms: 1, Seq: 2}, ID{Ms: 1, Seq: 4}, 0)))
	if len(got) != 3 || got[0].Seq != 2 || got[2].Seq != 4 {
		t.Errorf("inclusive range = %v", got)
	}
	// count limits.
	got = idsOf(collect(s.Range(MinID, MaxID, 3)))
	if len(got) != 3 || got[0].Seq != 0 {
		t.Errorf("counted range = %v", got)
	}
	// start > end is empty.
	if got := collect(s.Range(ID{Ms: 1, Seq: 5}, ID{Ms: 1, Seq: 2}, 0)); len(got) != 0 {
		t.Errorf("reversed range = %v", got)
	}
	// Ranges on an empty stream iterate nothing.
	if got := collect((&Stream{}).Range(MinID, MaxID, 0)); len(got) != 0 {
		t.Errorf("empty stream range = %v", got)
	}
}

func TestRevRange(t *testing.T) {
	s := &Stream{}
	for i := range 10 {
		mustAdd(t, s, ID{Ms: 1 + uint64(i)/3, Seq: uint64(i) % 3})
	}

	// Note the (end, start) parameter order, mirroring XREVRANGE.
	got := idsOf(collect(s.RevRange(MaxID, MinID, 0)))
	if len(got) != 10 {
		t.Fatalf("full revrange len = %d", len(got))
	}
	if got[0] != (ID{Ms: 4}) || got[9] != (ID{Ms: 1}) {
		t.Errorf("full revrange = %v", got)
	}
	// Descending within a window, count limited.
	got = idsOf(collect(s.RevRange(ID{Ms: 2, Seq: 2}, ID{Ms: 2}, 2)))
	if len(got) != 2 || got[0] != (ID{Ms: 2, Seq: 2}) || got[1] != (ID{Ms: 2, Seq: 1}) {
		t.Errorf("windowed revrange = %v", got)
	}
	// end < start is empty.
	if got := collect(s.RevRange(ID{Ms: 1}, ID{Ms: 1, Seq: 2}, 0)); len(got) != 0 {
		t.Errorf("reversed revrange = %v", got)
	}
}

func TestDelete(t *testing.T) {
	s := &Stream{}
	for i := range 5 {
		mustAdd(t, s, ID{Ms: 1, Seq: uint64(i)})
	}
	if !s.Delete(ID{Ms: 1, Seq: 2}) {
		t.Error("Delete of a present ID should report true")
	}
	if s.Delete(ID{Ms: 1, Seq: 2}) {
		t.Error("Delete of a missing ID should report false")
	}
	if s.Len() != 4 {
		t.Errorf("Len = %d, want 4", s.Len())
	}
	if got := collect(s.Range(ID{Ms: 1, Seq: 1}, ID{Ms: 1, Seq: 3}, 0)); len(got) != 2 {
		t.Errorf("range after delete = %v", got)
	}
}

func TestTrimMaxLen(t *testing.T) {
	s := &Stream{}
	for i := range 10 {
		mustAdd(t, s, ID{Ms: 1, Seq: uint64(i)})
	}
	if n := s.TrimMaxLen(3); n != 7 {
		t.Errorf("TrimMaxLen(3) evicted %d, want 7", n)
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}
	if id, _ := s.FirstID(); id != (ID{Ms: 1, Seq: 7}) {
		t.Errorf("FirstID after trim = %v, want 1-7 (oldest evicted)", id)
	}
	if n := s.TrimMaxLen(10); n != 0 {
		t.Errorf("TrimMaxLen(10) evicted %d, want 0", n)
	}
	// TrimMaxLen(0) empties the stream (negative is treated as 0).
	if n := s.TrimMaxLen(0); n != 3 {
		t.Errorf("TrimMaxLen(0) evicted %d, want 3", n)
	}
	if !s.IsEmpty() || s.LastID() != (ID{Ms: 1, Seq: 9}) {
		t.Errorf("after TrimMaxLen(0): Len %d LastID %v", s.Len(), s.LastID())
	}
	if n := s.TrimMaxLen(-1); n != 0 {
		t.Errorf("TrimMaxLen(-1) on empty evicted %d, want 0", n)
	}
}

func TestTrimMinID(t *testing.T) {
	s := &Stream{}
	for i := range 10 {
		mustAdd(t, s, ID{Ms: 1, Seq: uint64(i)})
	}
	// The bound is exclusive: everything below 1-5 goes.
	if n := s.TrimMinID(ID{Ms: 1, Seq: 5}); n != 5 {
		t.Errorf("TrimMinID(1-5) evicted %d, want 5", n)
	}
	if id, _ := s.FirstID(); id != (ID{Ms: 1, Seq: 5}) {
		t.Errorf("FirstID after trim = %v", id)
	}
	// MinID has nothing below it.
	if n := s.TrimMinID(MinID); n != 0 {
		t.Errorf("TrimMinID(0-0) evicted %d, want 0", n)
	}
	// Evicting past a ms boundary uses the previous ms's max seq.
	if n := s.TrimMinID(ID{Ms: 2}); n != 5 {
		t.Errorf("TrimMinID(2-0) evicted %d, want 5", n)
	}
	if !s.IsEmpty() {
		t.Errorf("Len = %d, want 0", s.Len())
	}
}

func TestSetLastID(t *testing.T) {
	s := &Stream{}
	mustAdd(t, s, ID{Ms: 5, Seq: 5})
	s.SetLastID(ID{Ms: 100})
	// Add now enforces strictly greater than the set ID.
	if _, err := s.Add(ID{Ms: 50}, nil); !errors.Is(err, ErrIDTooSmall) {
		t.Errorf("Add below SetLastID: got %v, want ErrIDTooSmall", err)
	}
	if got, err := s.Add(ID{Ms: 101}, nil); err != nil || got != (ID{Ms: 101}) {
		t.Errorf("Add above SetLastID: (%v, %v)", got, err)
	}
	// The entry storage stays ordered even when the set moved the bar
	// below existing entries (documented XSETID behavior).
	s2 := &Stream{}
	mustAdd(t, s2, ID{Ms: 9, Seq: 9})
	s2.SetLastID(ID{Ms: 3})
	if got, err := s2.Add(ID{Ms: 4}, nil); err != nil || got != (ID{Ms: 4}) {
		t.Errorf("Add after backwards SetLastID: (%v, %v)", got, err)
	}
	got := idsOf(collect(s2.Range(MinID, MaxID, 0)))
	if len(got) != 2 || got[0] != (ID{Ms: 4}) || got[1] != (ID{Ms: 9, Seq: 9}) {
		t.Errorf("entries after interleave = %v", got)
	}
}

func TestNilStreamTolerated(t *testing.T) {
	var s *Stream
	if s.Len() != 0 || !s.IsEmpty() {
		t.Error("nil stream should be empty")
	}
	if id, ok := s.FirstID(); ok || id != MinID {
		t.Errorf("FirstID on nil = (%v, %v)", id, ok)
	}
	if s.LastID() != MinID {
		t.Error("LastID on nil should be 0-0")
	}
	if got := collect(s.Range(MinID, MaxID, 0)); len(got) != 0 {
		t.Errorf("Range on nil = %v", got)
	}
	if got := collect(s.RevRange(MaxID, MinID, 0)); len(got) != 0 {
		t.Errorf("RevRange on nil = %v", got)
	}
	if s.Delete(ID{Ms: 1}) {
		t.Error("Delete on nil should report false")
	}
	if s.TrimMaxLen(0) != 0 || s.TrimMinID(MaxID) != 0 {
		t.Error("trims on nil should evict nothing")
	}
	s.SetLastID(ID{Ms: 9}) // no-op, must not panic
	s.Lock()               // no-ops on nil
	_ = s.Len()
	s.Unlock()
	// Group surface on nil.
	if s.GroupNames() != nil {
		t.Error("GroupNames on nil should be nil")
	}
	if s.DestroyGroup("g") != 0 {
		t.Error("DestroyGroup on nil should return 0")
	}
	if s.ReadGroup("g", "c", MinID, 10) != nil {
		t.Error("ReadGroup on nil should return nil")
	}
	if s.Ack("g", ID{Ms: 1}) != 0 {
		t.Error("Ack on nil should return 0")
	}
	if count, min, max, per := s.Pending("g"); count != 0 || min != MinID || max != MinID || len(per) != 0 {
		t.Errorf("Pending on nil = (%d, %v, %v, %v)", count, min, max, per)
	}
	if s.PendingRange("g", "", MinID, MaxID, 0) != nil {
		t.Error("PendingRange on nil should be nil")
	}
	if s.Claim("g", "c", 0, ID{Ms: 1}) != nil {
		t.Error("Claim on nil should return nil")
	}
	if entries, next, deleted := s.AutoClaim("g", "c", 0, MinID, 10); entries != nil || next != MinID || deleted != nil {
		t.Errorf("AutoClaim on nil = (%v, %v, %v)", entries, next, deleted)
	}
	s.GroupSetID("g", ID{Ms: 1}) // no-op
	if _, ok := s.GroupLastID("g"); ok {
		t.Error("GroupLastID on nil should report false")
	}
	if s.GroupCreateConsumer("g", "c") {
		t.Error("GroupCreateConsumer on nil should report false")
	}
	if s.GroupDeleteConsumer("g", "c") != 0 {
		t.Error("GroupDeleteConsumer on nil should return 0")
	}
	if s.GroupConsumers("g") != nil {
		t.Error("GroupConsumers on nil should be nil")
	}
}

func TestNilPanics(t *testing.T) {
	var s *Stream
	expectPanic(t, "stream: Add called on a nil Stream", func() { _, _ = s.Add(ID{Ms: 1}, nil) })
	expectPanic(t, "stream: CreateGroup called on a nil Stream", func() { _ = s.CreateGroup("g", MinID) })
}

func collect(seq func(func(Entry) bool)) []Entry {
	var out []Entry
	for e := range seq {
		out = append(out, e)
	}
	return out
}

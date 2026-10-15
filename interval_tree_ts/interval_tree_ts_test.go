package interval_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"iter"
	"slices"
	"strings"
	"sync"
	"testing"
)

// iv is shorthand for building an interval of ints.
func iv(lo, hi int) Interval[int] {
	return Interval[int]{Lo: lo, Hi: hi}
}

// entryInt is one (interval, value) pair for the test model.
type entryInt struct {
	iv  Interval[int]
	val int
}

// collect drains a Seq2 of (interval, value) pairs into a slice.
func collect(seq iter.Seq2[Interval[int], int]) []entryInt {
	var out []entryInt
	for k, v := range seq {
		out = append(out, entryInt{k, v})
	}
	return out
}

// expectPanic runs fn and fails the test unless it panics with a
// message containing want.
func expectPanic(t *testing.T, name, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
			t.Errorf("%s panic message %q does not contain %q.", name, r, want)
		}
	}()
	fn()
}

func TestInsertSearchDelete(t *testing.T) {
	tt := NewIntervalTree[int, int]()

	data := []entryInt{
		{iv(17, 19), 1}, {iv(5, 8), 2}, {iv(21, 24), 3},
		{iv(4, 8), 4}, {iv(15, 18), 5}, {iv(7, 10), 6},
		{iv(16, 22), 7},
	}
	for _, e := range data {
		if !tt.Insert(e.iv, e.val) {
			t.Errorf("Insert of %v reported replaced, expected added", e.iv)
		}
	}
	if tt.Len() != len(data) {
		t.Errorf("Len = %d, expected %d", tt.Len(), len(data))
	}
	for _, e := range data {
		if v, ok := tt.Search(e.iv); !ok || v != e.val {
			t.Errorf("Search(%v) = (%d, %v), expected (%d, true)", e.iv, v, ok, e.val)
		}
	}

	// Duplicate insert replaces the value, returns false, keeps length.
	if tt.Insert(iv(5, 8), 99) {
		t.Errorf("Duplicate Insert reported added")
	}
	if v, _ := tt.Search(iv(5, 8)); v != 99 {
		t.Errorf("Duplicate Insert did not replace the value: %d", v)
	}

	for _, e := range data {
		if !tt.Delete(e.iv) {
			t.Errorf("Delete(%v) returned false", e.iv)
		}
	}
	if !tt.IsEmpty() {
		t.Errorf("Tree not empty after deleting every interval, len=%d", tt.Len())
	}
}

// TestNilAndZeroValue verifies the tolerance contract: a nil tree and a
// zero-value tree behave as empty trees for every read — with the nil
// guards BEFORE lock acquisition.
func TestNilAndZeroValue(t *testing.T) {
	var nilTree *IntervalTree[int, int]

	if nilTree.Len() != 0 || !nilTree.IsEmpty() || nilTree.Length() != 0 {
		t.Errorf("nil tree is not empty")
	}
	if _, ok := nilTree.Search(iv(1, 2)); ok {
		t.Errorf("nil tree Search reported found")
	}
	if nilTree.Delete(iv(1, 2)) {
		t.Errorf("nil tree Delete returned true")
	}
	if _, _, ok := nilTree.AnyOverlap(iv(1, 2)); ok {
		t.Errorf("nil tree AnyOverlap reported found")
	}
	if n := collect(nilTree.All()); n != nil {
		t.Errorf("nil tree All yielded %v", n)
	}
	if n := collect(nilTree.Backward()); n != nil {
		t.Errorf("nil tree Backward yielded %v", n)
	}
	if n := collect(nilTree.Overlaps(iv(1, 2))); n != nil {
		t.Errorf("nil tree Overlaps yielded %v", n)
	}
	if n := collect(nilTree.Stab(1)); n != nil {
		t.Errorf("nil tree Stab yielded %v", n)
	}
	if _, _, ok := nilTree.Min(); ok {
		t.Errorf("nil tree Min reported found")
	}
	if _, _, ok := nilTree.Max(); ok {
		t.Errorf("nil tree Max reported found")
	}
	if d := nilTree.Depth(); d != 0 {
		t.Errorf("nil tree Depth = %d", d)
	}
	nilTree.Truncate()
	nilTree.Lock() // nil no-ops
	nilTree.Unlock()

	var zero IntervalTree[int, int]
	if zero.Len() != 0 || !zero.IsEmpty() {
		t.Errorf("zero-value tree is not empty")
	}
	if _, ok := zero.Search(iv(1, 2)); ok {
		t.Errorf("zero-value tree Search reported found")
	}
	if zero.Delete(iv(1, 2)) {
		t.Errorf("zero-value tree Delete returned true")
	}
	if n := collect(zero.All()); n != nil {
		t.Errorf("zero-value tree All yielded %v", n)
	}
	if n := collect(zero.Overlaps(iv(1, 2))); n != nil {
		t.Errorf("zero-value tree Overlaps yielded %v", n)
	}
	zero.Truncate()
}

// TestPanics verifies the twin's four panics, each with the
// interval_tree_ts: prefix.
func TestPanics(t *testing.T) {
	expectPanic(t, "NewIntervalTreeFunc(nil)", "interval_tree_ts: NewIntervalTreeFunc", func() {
		NewIntervalTreeFunc[int, int](nil)
	})
	expectPanic(t, "Insert on nil", "interval_tree_ts: Insert called on a nil tree", func() {
		var nilTree *IntervalTree[int, int]
		nilTree.Insert(iv(1, 2), 0)
	})
	expectPanic(t, "Insert on zero-value", "interval_tree_ts: Insert called on a tree with no comparison function", func() {
		var zero IntervalTree[int, int]
		zero.Insert(iv(1, 2), 0)
	})
	expectPanic(t, "Insert inverted", "interval_tree_ts: Insert called with an inverted interval", func() {
		tt := NewIntervalTree[int, int]()
		tt.Insert(iv(5, 1), 0)
	})
	expectPanic(t, "NlInsert inverted", "interval_tree_ts: NlInsert called with an inverted interval", func() {
		tt := NewIntervalTree[int, int]()
		tt.Lock()
		defer tt.Unlock()
		tt.NlInsert(iv(5, 1), 0)
	})
}

// TestIteratorSnapshotSemantics pins the INVERTED contract: the ts
// iterators are eager snapshots at call time — mutating the tree from
// inside the loop is safe, and a mutation after the call is NOT seen.
func TestIteratorSnapshotSemantics(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	tt.Insert(iv(1, 2), 1)
	tt.Insert(iv(3, 4), 2)
	tt.Insert(iv(5, 6), 3)

	// Snapshot then mutate: the running iterator keeps the old view.
	seen := 0
	for range tt.All() {
		seen++
		tt.Insert(iv(100, 200), 4) // must not invalidate the loop
		tt.Delete(iv(1, 2))
	}
	if seen != 3 {
		t.Errorf("snapshot iterator saw %d elements, expected 3", seen)
	}
	if tt.Len() != 3 { // 3 - delete(1,2) + insert(100,200)
		t.Errorf("Len after in-loop mutations = %d, expected 3", tt.Len())
	}

	// A snapshot taken before a mutation keeps the old view; the next
	// call reflects the mutation.
	all := collect(tt.All())
	tt.Delete(iv(3, 4))
	if n := len(all); n != 3 {
		t.Errorf("stale snapshot changed length to %d", n)
	}
	if !slices.ContainsFunc(all, func(e entryInt) bool { return e.iv == iv(3, 4) }) {
		t.Errorf("stale snapshot lost the deleted interval")
	}
	if slices.ContainsFunc(collect(tt.All()), func(e entryInt) bool { return e.iv == iv(3, 4) }) {
		t.Errorf("live tree still yields the deleted interval")
	}

	// Overlaps and Stab are snapshots too.
	over := tt.Overlaps(iv(90, 300))
	for range over {
		tt.Delete(iv(100, 200))
	}
	if got := collect(over); len(got) != 1 || got[0].iv != iv(100, 200) {
		t.Errorf("Overlaps snapshot drifted: %v", got)
	}
}

// TestLockNlCompound exercises the canonical compound — atomic
// check-overlap-then-book — against a concurrent writer that would
// split the check from the insert without the lock.
func TestLockNlCompound(t *testing.T) {
	tt := NewIntervalTree[int, int]()

	book := func(lo, hi int) bool {
		tt.Lock()
		defer tt.Unlock()
		if _, _, clash := tt.NlAnyOverlap(Interval[int]{Lo: lo, Hi: hi}); clash {
			return false
		}
		tt.NlInsert(Interval[int]{Lo: lo, Hi: hi}, lo)
		return true
	}

	// A concurrent writer keeps sliding a fixed-width interval across
	// the same space; whatever it books, our bookings must never
	// overlap another interval when book returns true.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			lo := 10 * (i % 50)
			tt.Lock()
			if _, _, clash := tt.NlAnyOverlap(Interval[int]{Lo: lo, Hi: lo + 4}); !clash {
				tt.NlInsert(Interval[int]{Lo: lo, Hi: lo + 4}, -1)
			}
			tt.Unlock()
		}
	}()

	for i := range 500 {
		if book(7*i%997, 7*i%997+3) {
			// Verify under the lock that nothing overlaps the fresh
			// booking (every insert was conditional on no overlap, and
			// all of them ran under the lock — so at most the booking
			// itself matches its own probe).
			tt.Lock()
			n := len(tt.nlOverlaps(Interval[int]{Lo: 7 * i % 997, Hi: 7*i%997 + 3}))
			tt.Unlock()
			if n > 1 {
				t.Fatalf("booking %d overlaps %d intervals — the compound is not atomic", i, n)
			}
		}
	}
	close(stop)
	wg.Wait()

	// Final global sanity: no interval overlaps another booked one more
	// than itself.  (Intervals may legitimately overlap the writer's
	// when book returned false for it, so only the count is checked.)
	tt.Lock()
	n := tt.NlLen()
	tt.Unlock()
	if n == 0 {
		t.Errorf("tree empty after compound test")
	}
}

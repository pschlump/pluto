package interval_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"iter"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

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

func TestInsertSearchDelete(t *testing.T) {
	tt := NewIntervalTree[int, int]()

	if _, ok := tt.Search(iv(1, 5)); ok {
		t.Errorf("Search on empty tree reported found")
	}
	if tt.Delete(iv(1, 5)) {
		t.Errorf("Delete on empty tree returned true")
	}

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
		v, ok := tt.Search(e.iv)
		if !ok || v != e.val {
			t.Errorf("Search(%v) = (%d, %v), expected (%d, true)", e.iv, v, ok, e.val)
		}
	}

	// A different interval overlapping a stored one is not found by
	// exact-match Search.
	if _, ok := tt.Search(iv(6, 7)); ok {
		t.Errorf("Search of a mere overlap reported found")
	}

	// Duplicate insert replaces the value, returns false, keeps length.
	if tt.Insert(iv(5, 8), 99) {
		t.Errorf("Duplicate Insert reported added")
	}
	if v, _ := tt.Search(iv(5, 8)); v != 99 {
		t.Errorf("Duplicate Insert did not replace the value: %d", v)
	}
	if tt.Len() != len(data) {
		t.Errorf("Duplicate Insert changed the length to %d", tt.Len())
	}

	// Point intervals are legal keys.
	if !tt.Insert(iv(9, 9), 100) {
		t.Errorf("Insert of point interval reported replaced")
	}

	// Delete removes exactly the named interval.
	for _, e := range data {
		if !tt.Delete(e.iv) {
			t.Errorf("Delete(%v) returned false", e.iv)
		}
	}
	if !tt.Delete(iv(9, 9)) {
		t.Errorf("Delete of the point interval returned false")
	}
	if !tt.IsEmpty() {
		t.Errorf("Tree not empty after deleting every interval, len=%d", tt.Len())
	}
	if tt.Delete(iv(5, 8)) {
		t.Errorf("Re-Delete returned true")
	}
}

// TestClosedIntervalSemantics pins the closed-endpoint convention:
// touching intervals overlap at the shared point.
func TestClosedIntervalSemantics(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	tt.Insert(iv(1, 2), 1)
	tt.Insert(iv(2, 3), 2)

	if got, _, ok := tt.AnyOverlap(iv(2, 2)); !ok {
		t.Errorf("AnyOverlap([2,2]) found nothing among touching intervals")
	} else if got != iv(1, 2) && got != iv(2, 3) {
		t.Errorf("AnyOverlap([2,2]) = %v, expected a touching interval", got)
	}

	n := 0
	for range tt.Stab(2) {
		n++
	}
	if n != 2 {
		t.Errorf("Stab(2) hit %d intervals, expected 2 (both touching)", n)
	}
}

func TestAnyOverlap(t *testing.T) {
	tt := NewIntervalTree[int, string]()
	for _, e := range []struct {
		iv  Interval[int]
		val string
	}{
		{iv(17, 19), "a"}, {iv(5, 8), "b"}, {iv(21, 24), "c"},
		{iv(4, 8), "d"}, {iv(15, 18), "e"}, {iv(7, 10), "f"}, {iv(16, 22), "g"},
	} {
		tt.Insert(e.iv, e.val)
	}

	// [11,14] is a genuine gap: the tree jumps from [7,10] to [15,18].
	if _, _, ok := tt.AnyOverlap(iv(11, 14)); ok {
		t.Errorf("AnyOverlap of the gap [11,14] reported a hit")
	}
	if _, _, ok := tt.AnyOverlap(iv(25, 30)); ok {
		t.Errorf("AnyOverlap past the end reported a hit")
	}
	if _, _, ok := tt.AnyOverlap(iv(0, 3)); ok {
		t.Errorf("AnyOverlap before the start reported a hit")
	}

	got, val, ok := tt.AnyOverlap(iv(9, 16)) // touches e=[15,18]
	if !ok {
		t.Errorf("AnyOverlap([9,16]) reported no hit")
	} else if !overlapsInt(got, iv(9, 16)) {
		t.Errorf("AnyOverlap returned non-overlapping %v", got)
	} else if val == "" {
		t.Errorf("AnyOverlap returned an empty value")
	}

	// Containment and containment-by both count.
	if _, _, ok := tt.AnyOverlap(iv(6, 7)); !ok {
		t.Errorf("AnyOverlap of a contained probe reported no hit")
	}
	if _, _, ok := tt.AnyOverlap(iv(0, 100)); !ok {
		t.Errorf("AnyOverlap of a containing probe reported no hit")
	}
}

// overlapsInt is the plain-int reference overlap predicate for tests.
func overlapsInt(a, b Interval[int]) bool {
	return a.Lo <= b.Hi && b.Lo <= a.Hi
}

func TestOverlapsAndStab(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	for i, e := range []entryInt{
		{iv(1, 3), 10}, {iv(2, 5), 11}, {iv(4, 7), 12},
		{iv(10, 12), 13}, {iv(12, 12), 14},
	} {
		tt.Insert(e.iv, i) // value == subscript
	}

	got := collect(tt.Overlaps(iv(2, 4)))
	want := []entryInt{{iv(1, 3), 0}, {iv(2, 5), 1}, {iv(4, 7), 2}}
	if !slices.Equal(got, want) {
		t.Errorf("Overlaps([2,4]) = %v, expected %v (ascending, exact)", got, want)
	}

	// Stab is Overlaps over [p, p]; the point interval [12,12] and the
	// touching [10,12] both contain 12.
	got = collect(tt.Stab(12))
	want = []entryInt{{iv(10, 12), 3}, {iv(12, 12), 4}}
	if !slices.Equal(got, want) {
		t.Errorf("Stab(12) = %v, expected %v", got, want)
	}

	if got := collect(tt.Stab(8)); got != nil {
		t.Errorf("Stab(8) = %v, expected nothing", got)
	}
	if got := collect(tt.Overlaps(iv(100, 200))); got != nil {
		t.Errorf("Overlaps far past the end = %v, expected nothing", got)
	}

	// Early termination: the first yielded element is the smallest match.
	var first *entryInt
	for k, v := range tt.Overlaps(iv(0, 100)) {
		first = &entryInt{k, v}
		break
	}
	if first == nil || first.iv != iv(1, 3) {
		t.Errorf("Overlaps broke before yielding the smallest match: %v", first)
	}
}

func TestMinAndMax(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	if _, _, ok := tt.Min(); ok {
		t.Errorf("Min on empty tree reported found")
	}
	if _, _, ok := tt.Max(); ok {
		t.Errorf("Max on empty tree reported found")
	}

	tt.Insert(iv(5, 9), 1)
	tt.Insert(iv(5, 7), 2) // same Lo, smaller Hi — the (Lo, Hi) tiebreak
	tt.Insert(iv(2, 3), 3)
	tt.Insert(iv(8, 9), 4)

	if got, _, _ := tt.Min(); got != iv(2, 3) {
		t.Errorf("Min = %v, expected [2,3]", got)
	}
	if got, _, _ := tt.Max(); got != iv(8, 9) {
		t.Errorf("Max = %v, expected [8,9]", got)
	}
}

func TestAllBackwardOrder(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	data := []entryInt{{iv(5, 6), 1}, {iv(1, 2), 2}, {iv(3, 4), 3}, {iv(5, 8), 4}}
	for _, e := range data {
		tt.Insert(e.iv, e.val)
	}
	slices.SortFunc(data, func(a, b entryInt) int {
		if a.iv.Lo != b.iv.Lo {
			return a.iv.Lo - b.iv.Lo
		}
		return a.iv.Hi - b.iv.Hi
	})

	if got := collect(tt.All()); !slices.Equal(got, data) {
		t.Errorf("All = %v, expected %v", got, data)
	}
	slices.Reverse(data)
	if got := collect(tt.Backward()); !slices.Equal(got, data) {
		t.Errorf("Backward = %v, expected %v", got, data)
	}
}

// TestAnyOverlapPointAndDegenerate probes degenerate and empty-query
// shapes that must not panic and must behave sanely.
func TestAnyOverlapPointAndDegenerate(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	tt.Insert(iv(0, 0), 1) // a point interval at 0
	if _, _, ok := tt.AnyOverlap(iv(0, 0)); !ok {
		t.Errorf("Point interval does not overlap itself")
	}
	if _, _, ok := tt.AnyOverlap(iv(1, 1)); ok {
		t.Errorf("Point [1,1] overlaps nothing stored")
	}
	// An inverted probe is not validated on queries: whatever it returns
	// must be non-panicking; assert only that here.
	_, _, _ = tt.AnyOverlap(iv(3, 1))
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("AnyOverlap with inverted probe panicked: %v", r)
			}
		}()
		for range tt.Overlaps(iv(3, 1)) {
		}
	}()
}

// TestNilAndZeroValue verifies the tolerance contract: a nil tree and a
// zero-value tree behave as empty trees for every read.
func TestNilAndZeroValue(t *testing.T) {
	var nilTree *IntervalTree[int, int]

	if nilTree.Len() != 0 || !nilTree.IsEmpty() {
		t.Errorf("nil tree is not empty")
	}
	if nilTree.Length() != 0 {
		t.Errorf("nil tree Length = %d", nilTree.Length())
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
	nilTree.Truncate() // must not panic

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
	if _, _, ok := zero.AnyOverlap(iv(1, 2)); ok {
		t.Errorf("zero-value tree AnyOverlap reported found")
	}
	if n := collect(zero.All()); n != nil {
		t.Errorf("zero-value tree All yielded %v", n)
	}
	if n := collect(zero.Overlaps(iv(1, 2))); n != nil {
		t.Errorf("zero-value tree Overlaps yielded %v", n)
	}
	zero.Truncate()
}

// TestPanics verifies the package's four panic situations with their
// messages naming the method and the fix.
func TestPanics(t *testing.T) {
	expectPanic(t, "NewIntervalTreeFunc(nil)", "NewIntervalTreeFunc", func() {
		NewIntervalTreeFunc[int, int](nil)
	})
	expectPanic(t, "Insert on nil", "Insert called on a nil tree", func() {
		var nilTree *IntervalTree[int, int]
		nilTree.Insert(iv(1, 2), 0)
	})
	expectPanic(t, "Insert on zero-value", "no comparison function", func() {
		var zero IntervalTree[int, int]
		zero.Insert(iv(1, 2), 0)
	})
	expectPanic(t, "Insert inverted", "inverted interval", func() {
		tt := NewIntervalTree[int, int]()
		tt.Insert(iv(5, 1), 0)
	})
}

// TestInsertFuncCustomKey runs the tree over a non-Ordered key type
// through NewIntervalTreeFunc — a struct ordered by one field.
func TestInsertFuncCustomKey(t *testing.T) {
	type bound struct {
		Major, Minor int
	}
	cmpBound := func(a, b bound) int {
		if a.Major != b.Major {
			return a.Major - b.Major
		}
		return a.Minor - b.Minor
	}
	tt := NewIntervalTreeFunc[bound, string](cmpBound)
	tt.Insert(Interval[bound]{Lo: bound{1, 0}, Hi: bound{1, 5}}, "one")
	tt.Insert(Interval[bound]{Lo: bound{2, 2}, Hi: bound{3, 0}}, "two")

	if v, ok := tt.Search(Interval[bound]{Lo: bound{1, 0}, Hi: bound{1, 5}}); !ok || v != "one" {
		t.Errorf("Search = (%q,%v)", v, ok)
	}
	if _, _, ok := tt.AnyOverlap(Interval[bound]{Lo: bound{0, 0}, Hi: bound{1, 2}}); !ok {
		t.Errorf("AnyOverlap over the custom key missed [1,0]..[1,5]")
	}
	if _, _, ok := tt.AnyOverlap(Interval[bound]{Lo: bound{1, 6}, Hi: bound{2, 1}}); ok {
		t.Errorf("AnyOverlap in the gap reported a hit")
	}
}

// TestTruncateThenReuse verifies Truncate keeps the comparison
// function: the tree remains usable and refills.
func TestTruncateThenReuse(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	tt.Insert(iv(1, 2), 1)
	tt.Insert(iv(3, 4), 2)
	tt.Truncate()
	if !tt.IsEmpty() || tt.Len() != 0 {
		t.Errorf("Truncate left len=%d", tt.Len())
	}
	if got := collect(tt.All()); got != nil {
		t.Errorf("All after Truncate yielded %v", got)
	}
	if !tt.Insert(iv(9, 10), 3) {
		t.Errorf("Insert after Truncate reported replaced")
	}
	if v, ok := tt.Search(iv(9, 10)); !ok || v != 3 {
		t.Errorf("Search after refill = (%d,%v)", v, ok)
	}
}

// TestIteratorsReflectLiveTree pins the plain package's live-walk
// contract: an iterator started after a mutation sees the mutation (the
// ts twin inverts this).
func TestIteratorsReflectLiveTree(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	tt.Insert(iv(1, 2), 1)
	if n := len(collect(tt.All())); n != 1 {
		t.Fatalf("expected 1 element, got %d", n)
	}
	tt.Insert(iv(3, 4), 2)
	if n := len(collect(tt.All())); n != 2 {
		t.Errorf("iterator does not reflect the live tree: saw %d of 2", n)
	}
	tt.Delete(iv(1, 2))
	if n := len(collect(tt.All())); n != 1 {
		t.Errorf("iterator does not reflect the live tree: saw %d of 1", n)
	}
}

func TestDump(t *testing.T) {
	tt := NewIntervalTree[int, int]()
	var sb strings.Builder
	tt.Dump(&sb)
	if s := sb.String(); !strings.Contains(s, "empty") {
		t.Errorf("Dump of empty tree: %q", s)
	}

	sb.Reset()
	tt.Insert(iv(1, 5), 7)
	tt.Insert(iv(2, 3), 8)
	tt.Dump(&sb)
	s := sb.String()
	if !strings.Contains(s, "1-5") || !strings.Contains(s, "2-3") || !strings.Contains(s, "max=5") {
		t.Errorf("Dump missing pieces: %q", s)
	}
}

// TestDeleteAllShapes drives deletions that hit each deleteFixup case:
// red leaf, black node with red child, interior node with two children
// (successor is the child, and successor deeper), root deletion.
func TestDeleteAllShapes(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for range 200 {
		tt := NewIntervalTree[int, int]()
		n := 1 + rng.IntN(64)
		for range n {
			lo := rng.IntN(200)
			tt.Insert(iv(lo, lo+rng.IntN(4)), 0)
		}
		for !tt.IsEmpty() {
			lo := rng.IntN(200)
			tt.Delete(iv(lo, lo+rng.IntN(4)))
			checkInvariants(t, tt)
		}
	}
}

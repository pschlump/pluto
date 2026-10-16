/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"math"
	"strings"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// tracePoints build, hand-traced:
//
//	Insert A(0.7,0.2) → root (splits x)
//	Insert B(0.5,0.4) → 0.5 < 0.7 → left of A (splits y)
//	Insert C(0.2,0.3) → left of A, 0.3 < 0.4 → left of B
//	Insert D(0.4,0.7) → left of A, 0.7 ≥ 0.4 → right of B
//	Insert E(0.9,0.6) → 0.9 ≥ 0.7 → right of A
//
//	        A
//	       / \
//	      B   E
//	     / \
//	    C   D
var (
	ptA = g_lib.Point2D{X: 0.7, Y: 0.2}
	ptB = g_lib.Point2D{X: 0.5, Y: 0.4}
	ptC = g_lib.Point2D{X: 0.2, Y: 0.3}
	ptD = g_lib.Point2D{X: 0.4, Y: 0.7}
	ptE = g_lib.Point2D{X: 0.9, Y: 0.6}
)

func newTraceTree(t *testing.T) *KdTree {
	t.Helper()
	tr := NewKdTree()
	for _, p := range []g_lib.Point2D{ptA, ptB, ptC, ptD, ptE} {
		if !tr.Insert(p) {
			t.Fatalf("Insert(%v) reported duplicate in a fresh tree", p)
		}
	}
	return tr
}

func expectPanic(t *testing.T, name, substring string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected a panic, got none", name)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("%s: panic value is %T (%v), want a string", name, r, r)
		}
		if !strings.Contains(msg, substring) {
			t.Fatalf("%s: panic message %q does not contain %q", name, msg, substring)
		}
	}()
	fn()
}

func TestInsertStructureAndLen(t *testing.T) {
	tr := newTraceTree(t)
	if got := tr.Len(); got != 5 {
		t.Fatalf("Len = %d, want 5", got)
	}
	if tr.root.p != ptA || tr.root.left.p != ptB || tr.root.right.p != ptE ||
		tr.root.left.left.p != ptC || tr.root.left.right.p != ptD {
		t.Fatalf("tree shape does not match the hand trace")
	}
	if tr.root.left.left.left != nil || tr.root.left.left.right != nil {
		t.Fatalf("C should be a leaf")
	}
	// Duplicates are rejected no-ops.
	for _, p := range []g_lib.Point2D{ptA, ptC, ptE} {
		if tr.Insert(p) {
			t.Fatalf("duplicate Insert(%v) reported added", p)
		}
	}
	if got := tr.Len(); got != 5 {
		t.Fatalf("Len after duplicates = %d, want 5", got)
	}
}

func TestZeroValueUsable(t *testing.T) {
	var tr KdTree
	if !tr.IsEmpty() || tr.Len() != 0 {
		t.Fatalf("zero value is not empty")
	}
	if !tr.Insert(ptA) {
		t.Fatalf("Insert on the zero value reported duplicate")
	}
	if !tr.Contains(ptA) || tr.Len() != 1 {
		t.Fatalf("zero-value insert lost the point")
	}
	var ps PointSet
	if !ps.Insert(ptB) || !ps.Contains(ptB) {
		t.Fatalf("zero-value PointSet insert lost the point")
	}
}

func TestNilTreeTolerated(t *testing.T) {
	var tr *KdTree
	if tr.Len() != 0 || !tr.IsEmpty() {
		t.Fatalf("nil tree is not empty")
	}
	if tr.Contains(ptA) {
		t.Fatalf("nil tree Contains = true")
	}
	if _, ok := tr.Nearest(ptA); ok {
		t.Fatalf("nil tree Nearest reported found")
	}
	if tr.NearestK(ptA, 3) != nil {
		t.Fatalf("nil tree NearestK is not nil")
	}
	if tr.Range(g_lib.RectHV{Xmin: math.Inf(-1), Xmax: math.Inf(1), Ymin: math.Inf(-1), Ymax: math.Inf(1)}) != nil {
		t.Fatalf("nil tree Range is not nil")
	}
	n := 0
	for range tr.All() {
		n++
	}
	if n != 0 {
		t.Fatalf("nil tree All yielded %d points", n)
	}

	var ps *PointSet
	if ps.Len() != 0 || ps.Contains(ptA) {
		t.Fatalf("nil PointSet is not empty")
	}
	if _, ok := ps.Nearest(ptA); ok {
		t.Fatalf("nil PointSet Nearest reported found")
	}
	if ps.NearestK(ptA, 2) != nil || ps.Range(wholePlane) != nil {
		t.Fatalf("nil PointSet queries are not nil")
	}
	for range ps.All() {
		t.Fatalf("nil PointSet All yielded a point")
	}
}

func TestContains(t *testing.T) {
	tr := newTraceTree(t)
	for _, p := range []g_lib.Point2D{ptA, ptB, ptC, ptD, ptE} {
		if !tr.Contains(p) {
			t.Fatalf("Contains(%v) = false", p)
		}
	}
	for _, p := range []g_lib.Point2D{
		{X: 0.7, Y: 0.3}, // same x as A, different y
		{X: 0.5, Y: 0.4000001},
		{X: -1, Y: -1},
	} {
		if tr.Contains(p) {
			t.Fatalf("Contains(%v) = true", p)
		}
	}
}

func TestNearestBasic(t *testing.T) {
	tr := newTraceTree(t)
	// Unambiguous minima, one of which (q3) requires the search to
	// improve its champion across subtrees.
	cases := []struct {
		q    g_lib.Point2D
		want g_lib.Point2D
	}{
		{q: g_lib.Point2D{X: 0.65, Y: 0.45}, want: ptB},
		{q: g_lib.Point2D{X: 0.3, Y: 0.36}, want: ptC},
		{q: g_lib.Point2D{X: 0.88, Y: 0.55}, want: ptE},
		{q: ptD, want: ptD}, // a query that is in the tree returns itself
	}
	for _, tc := range cases {
		got, ok := tr.Nearest(tc.q)
		if !ok {
			t.Fatalf("Nearest(%v) reported not-found", tc.q)
		}
		if got != tc.want {
			t.Fatalf("Nearest(%v) = %v, want %v", tc.q, got, tc.want)
		}
	}
	if _, ok := (*KdTree)(nil).Nearest(ptA); ok {
		t.Fatalf("Nearest on an empty tree reported found")
	}
}

func TestNearestKBasic(t *testing.T) {
	tr := newTraceTree(t)
	// k=2 from B: B itself (d²=0) then A (0.08); the next two (C and D)
	// tie at 0.1 so k=2 stays clear of the tie.
	got := tr.NearestK(ptB, 2)
	want := []g_lib.Point2D{ptB, ptA}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("NearestK(B, 2) = %v, want %v", got, want)
	}
	// k larger than the tree returns everything.  C and D are
	// mathematically tied at d² = 0.1 from B, but the float arithmetic
	// differs in the last ulp (C: 0.1, D: 0.09999999999999995) — both
	// structures compute the identical values, so the full order is
	// deterministic: B, A, D, C, E.
	got = tr.NearestK(ptB, 10)
	want = []g_lib.Point2D{ptB, ptA, ptD, ptC, ptE}
	if len(got) != len(want) {
		t.Fatalf("NearestK(B, 10) has %d points, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NearestK(B, 10)[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	if tr.NearestK(ptB, 0) != nil || tr.NearestK(ptB, -1) != nil {
		t.Fatalf("NearestK with k ≤ 0 is not nil")
	}
}

func TestRangeBasic(t *testing.T) {
	tr := newTraceTree(t)
	// A sits exactly on the rectangle's corner (x = Xmax, y = Ymin) —
	// the boundary-inclusive contract.
	r := g_lib.RectHV{Xmin: 0.4, Ymin: 0.2, Xmax: 0.7, Ymax: 0.5}
	got := sortPoints(tr.Range(r))
	want := []g_lib.Point2D{ptB, ptA}
	if len(got) != len(want) {
		t.Fatalf("Range has %d points, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Range[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	if tr.Range(g_lib.RectHV{Xmin: 0.0, Ymin: 0.0, Xmax: 0.1, Ymax: 0.1}) != nil {
		t.Fatalf("Range of a matching-nothing rectangle is not nil")
	}
}

func TestAllPreorder(t *testing.T) {
	tr := newTraceTree(t)
	var got []g_lib.Point2D
	for p := range tr.All() {
		got = append(got, p)
	}
	want := []g_lib.Point2D{ptA, ptB, ptC, ptD, ptE} // node, left, right
	if len(got) != len(want) {
		t.Fatalf("All yielded %d points, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("All[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPanics(t *testing.T) {
	expectPanic(t, "nil tree Insert", "kd_tree: Insert called on a nil *KdTree", func() {
		var tr *KdTree
		tr.Insert(ptA)
	})
	expectPanic(t, "NaN x Insert", "kd_tree: Insert called with a NaN coordinate", func() {
		tr := NewKdTree()
		tr.Insert(g_lib.Point2D{X: math.NaN(), Y: 0})
	})
	expectPanic(t, "NaN y Insert", "kd_tree: Insert called with a NaN coordinate", func() {
		tr := NewKdTree()
		tr.Insert(g_lib.Point2D{X: 0, Y: math.NaN()})
	})
	expectPanic(t, "nil PointSet Insert", "kd_tree: PointSet Insert called on a nil *PointSet", func() {
		var ps *PointSet
		ps.Insert(ptA)
	})
	expectPanic(t, "PointSet NaN Insert", "kd_tree: PointSet Insert called with a NaN coordinate", func() {
		ps := NewPointSet()
		ps.Insert(g_lib.Point2D{X: math.NaN(), Y: 1})
	})
	// NaN queries never panic — unspecified, not fatal.
	tr := newTraceTree(t)
	tr.Nearest(g_lib.Point2D{X: math.NaN(), Y: 0})
	tr.NearestK(g_lib.Point2D{X: 0, Y: math.NaN()}, 2)
	tr.Range(g_lib.RectHV{Xmin: math.NaN(), Ymin: 0, Xmax: 1, Ymax: 1})
	tr.Contains(g_lib.Point2D{X: math.NaN(), Y: math.NaN()})
}

func TestInfinityCoordinates(t *testing.T) {
	// ±Inf coordinates are legal input (only NaN is rejected): points
	// far outside any unit square still insert, search and query.  The
	// finite point stays at the 1e150 scale so its squared distances
	// do not overflow — at 1e300 every d² saturates to +Inf and ties
	// with the Inf point's distance.
	tr := NewKdTree()
	inf := g_lib.Point2D{X: math.Inf(1), Y: 0}
	big := g_lib.Point2D{X: 1e150, Y: -1e150}
	if !tr.Insert(inf) || !tr.Insert(big) {
		t.Fatalf("Inf/big inserts reported duplicate")
	}
	if !tr.Contains(inf) || !tr.Contains(big) {
		t.Fatalf("Contains missed the Inf/big points")
	}
	got, ok := tr.Nearest(g_lib.Point2D{X: 9e149, Y: -9e149})
	if !ok || got != big {
		t.Fatalf("Nearest near big = %v %v, want %v", got, ok, big)
	}
}

func TestPointSetBasics(t *testing.T) {
	ps := NewPointSet()
	for _, p := range []g_lib.Point2D{ptA, ptB, ptC, ptD, ptE} {
		if !ps.Insert(p) {
			t.Fatalf("PointSet Insert(%v) reported duplicate", p)
		}
	}
	if ps.Insert(ptA) {
		t.Fatalf("duplicate PointSet Insert reported added")
	}
	if ps.Len() != 5 || ps.IsEmpty() {
		t.Fatalf("PointSet Len/IsEmpty wrong")
	}
	got, ok := ps.Nearest(g_lib.Point2D{X: 0.65, Y: 0.45})
	if !ok || got != ptB {
		t.Fatalf("PointSet Nearest = %v %v, want %v", got, ok, ptB)
	}
	kn := ps.NearestK(ptB, 3)
	if len(kn) != 3 || kn[0] != ptB {
		t.Fatalf("PointSet NearestK = %v", kn)
	}
	// C and D are a mathematical tie at d²=0.1 from B that float
	// arithmetic splits (D one ulp below C) — see TestNearestKBasic.
	kn4 := ps.NearestK(ptB, 4)
	if len(kn4) != 4 || kn4[0] != ptB || kn4[1] != ptA || kn4[2] != ptD || kn4[3] != ptC {
		t.Fatalf("PointSet NearestK(B, 4) = %v, want [B A D C]", kn4)
	}
	in := sortPoints(ps.Range(g_lib.RectHV{Xmin: 0.4, Ymin: 0.2, Xmax: 0.7, Ymax: 0.5}))
	if len(in) != 2 || in[0] != ptB || in[1] != ptA {
		t.Fatalf("PointSet Range = %v, want [B A]", in)
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree_ts

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pschlump/pluto/kd_tree"
)

var tracePoints = []Point2D{
	{X: 0.7, Y: 0.2}, {X: 0.5, Y: 0.4}, {X: 0.2, Y: 0.3}, {X: 0.4, Y: 0.7}, {X: 0.9, Y: 0.6},
}

func newTraceTree(t *testing.T) *KdTree {
	t.Helper()
	tr := NewKdTree()
	for _, p := range tracePoints {
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

func sortPoints(pts []Point2D) []Point2D {
	out := slices.Clone(pts)
	slices.SortFunc(out, func(a, b Point2D) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
	return out
}

func TestKdTreeBasic(t *testing.T) {
	tr := newTraceTree(t)
	if tr.Len() != 5 || tr.IsEmpty() {
		t.Fatalf("Len/IsEmpty wrong after 5 inserts")
	}
	for _, p := range tracePoints {
		if !tr.Contains(p) {
			t.Fatalf("Contains(%v) = false", p)
		}
	}
	if tr.Insert(tracePoints[0]) {
		t.Fatalf("duplicate Insert reported added")
	}
	got, ok := tr.Nearest(Point2D{X: 0.85, Y: 0.45})
	if !ok || got != (Point2D{X: 0.9, Y: 0.6}) {
		t.Fatalf("Nearest = %v %v", got, ok)
	}
	kk := tr.NearestK(Point2D{X: 0.6, Y: 0.5}, 2)
	if len(kk) != 2 || kk[0] != (Point2D{X: 0.5, Y: 0.4}) || kk[1] != (Point2D{X: 0.4, Y: 0.7}) {
		t.Fatalf("NearestK = %v", kk)
	}
	in := tr.Range(RectHV{Xmin: 0.2, Ymin: 0.2, Xmax: 0.5, Ymax: 0.4})
	if len(in) != 2 || !slices.Contains(in, Point2D{X: 0.2, Y: 0.3}) || !slices.Contains(in, Point2D{X: 0.5, Y: 0.4}) {
		t.Fatalf("Range = %v", in)
	}
	var n int
	for range tr.All() {
		n++
	}
	if n != 5 {
		t.Fatalf("All yielded %d points", n)
	}
}

func TestZeroValueUsable(t *testing.T) {
	var tr KdTree
	if !tr.IsEmpty() || tr.Len() != 0 {
		t.Fatalf("zero value is not empty")
	}
	if !tr.Insert(tracePoints[0]) || !tr.Contains(tracePoints[0]) {
		t.Fatalf("zero-value insert lost the point")
	}
}

func TestNilTreeTolerated(t *testing.T) {
	var tr *KdTree
	if tr.Len() != 0 || tr.IsEmpty() == false || tr.Contains(tracePoints[0]) {
		t.Fatalf("nil tree is not an empty read-only tree")
	}
	if _, ok := tr.Nearest(tracePoints[0]); ok {
		t.Fatalf("nil tree Nearest reported found")
	}
	if tr.NearestK(tracePoints[0], 3) != nil || tr.Range(RectHV{}) != nil {
		t.Fatalf("nil tree query results are not nil")
	}
	for range tr.All() {
		t.Fatalf("nil tree All yielded a point")
	}
	tr.Lock() // nil no-ops, must not panic
	tr.Unlock()
}

func TestPanics(t *testing.T) {
	expectPanic(t, "nil Insert", "kd_tree_ts: Insert called on a nil *KdTree", func() {
		var tr *KdTree
		tr.Insert(tracePoints[0])
	})
	expectPanic(t, "NaN Insert", "kd_tree_ts: Insert called with a NaN coordinate", func() {
		tr := NewKdTree()
		tr.Insert(Point2D{X: math.NaN(), Y: 0})
	})
	// A NaN insert through NlInsert hits the plain core's message —
	// only the locking path pre-checks with the twin's prefix.
	expectPanic(t, "NlInsert NaN", "kd_tree: Insert called with a NaN coordinate", func() {
		tr := NewKdTree()
		tr.Lock()
		defer tr.Unlock()
		tr.NlInsert(Point2D{X: 0, Y: math.NaN()})
	})
	// NaN queries never panic.
	tr := newTraceTree(t)
	tr.Nearest(Point2D{X: math.NaN(), Y: 0})
	tr.NearestK(Point2D{X: 0, Y: math.NaN()}, 2)
	tr.Range(RectHV{Xmin: math.NaN(), Ymin: 0, Xmax: 1, Ymax: 1})
}

// TestIteratorSnapshotSemantics checks the inverted contract: the twin's
// All is an eager snapshot taken under the read lock — mutating the tree
// from inside the loop is safe and never observed.
func TestIteratorSnapshotSemantics(t *testing.T) {
	tr := newTraceTree(t)
	seen := 0
	for range tr.All() {
		seen++
		tr.Insert(Point2D{X: float64(seen) + 10, Y: float64(seen) + 10}) // mutation mid-iteration
		if seen > 5 {
			t.Fatalf("snapshot iterated more points than it started with")
		}
	}
	if seen != 5 {
		t.Fatalf("snapshot visited %d points, want the original 5", seen)
	}
	if tr.Len() != 10 {
		t.Fatalf("Len after mid-loop inserts = %d, want 10", tr.Len())
	}
}

// TestLockNlCompound exercises the canonical compound: a
// nearest-neighbor test then a conditional insert, atomic against
// concurrent writers.
func TestLockNlCompound(t *testing.T) {
	tr := NewKdTree()
	for _, p := range tracePoints {
		tr.Insert(p)
	}
	// insertIfFar adds p only when nothing existing lies within minDist
	// of it — one atomic check-then-book under Lock.
	insertIfFar := func(p Point2D, minDist float64) bool {
		tr.Lock()
		defer tr.Unlock()
		if near, ok := tr.NlNearest(p); ok {
			if dx, dy := near.X-p.X, near.Y-p.Y; dx*dx+dy*dy < minDist*minDist {
				return false
			}
		}
		return tr.NlInsert(p)
	}
	if !insertIfFar(Point2D{X: 0.95, Y: 0.95}, 0.2) {
		t.Fatalf("far point rejected")
	}
	if insertIfFar(Point2D{X: 0.92, Y: 0.62}, 0.2) {
		t.Fatalf("point 0.03 from an existing site accepted")
	}
	if tr.Len() != 6 {
		t.Fatalf("Len = %d, want 6", tr.Len())
	}
	// Nl reads agree with the locked reads.
	tr.Lock()
	nlLen := tr.NlLen()
	nlContains := tr.NlContains(Point2D{X: 0.95, Y: 0.95})
	nlNearest, _ := tr.NlNearest(Point2D{X: 0.85, Y: 0.45})
	tr.Unlock()
	if nlLen != 6 || !nlContains || nlNearest != (Point2D{X: 0.9, Y: 0.6}) {
		t.Fatalf("Nl reads disagree: len=%d contains=%v nearest=%v", nlLen, nlContains, nlNearest)
	}
}

// TestKdTreeConcurrent is the writer/reader storm: 8 writers insert
// disjoint point clouds while readers hammer every read path.
func TestKdTreeConcurrent(t *testing.T) {
	tr := NewKdTree()
	const writers, each = 8, 500
	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				tr.Insert(Point2D{X: float64(w)*100 + float64(i)/1000, Y: float64(i) / 1000})
			}
		}()
	}
	for r := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				q := Point2D{X: float64(r)*100 + 0.5, Y: float64(i) / 1000}
				tr.Contains(q)
				tr.Nearest(q)
				tr.NearestK(q, 5)
				tr.Range(RectHV{Xmin: q.X - 0.1, Ymin: q.Y - 0.1, Xmax: q.X + 0.1, Ymax: q.Y + 0.1})
				tr.Len()
				for range tr.All() {
					break // snapshot iteration must be safe mid-storm
				}
			}
		}()
	}
	wg.Wait()
	if got := tr.Len(); got != writers*each {
		t.Fatalf("Len = %d, want %d (lost or duplicated inserts)", got, writers*each)
	}
}

// TestKdTreeConstrainedInsertCompound races the Lock+Nl spacing compound
// against plain writers: every accepted point must be spaced ≥ 0.1 from
// every point visible to its own atomic section — which, because both
// kinds of writer serialize on the write lock, means from every point
// accepted before it.
func TestKdTreeConstrainedInsertCompound(t *testing.T) {
	tr := NewKdTree()
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := make(map[Point2D]bool)

	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(w+1), 7))
			for range 500 {
				p := Point2D{X: math.Trunc(rng.Float64()*200) / 10, Y: math.Trunc(rng.Float64()*200) / 10}
				tr.Lock()
				near, ok := tr.NlNearest(p)
				far := !ok || (near.X-p.X)*(near.X-p.X)+(near.Y-p.Y)*(near.Y-p.Y) >= 0.01
				if far {
					tr.NlInsert(p)
				}
				tr.Unlock()
				if far {
					mu.Lock()
					if accepted[p] {
						mu.Unlock()
						t.Errorf("point %v accepted twice", p)
						return
					}
					accepted[p] = true
					mu.Unlock()
				}
			}
		}()
	}
	// Plain writers compete for the same lock — far away in the plane,
	// so they can never violate the spacing of the compound points.
	for w := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(w+11), 42))
			for range 500 {
				tr.Insert(Point2D{X: 500 + rng.Float64(), Y: rng.Float64()})
			}
		}()
	}
	wg.Wait()

	// Post-storm, single-threaded: every accepted point is in the tree
	// and spaced from every other accepted point.
	for p := range accepted {
		if !tr.Contains(p) {
			t.Fatalf("accepted point %v missing from the tree", p)
		}
		for q := range accepted {
			if p == q {
				continue
			}
			if dx, dy := p.X-q.X, p.Y-q.Y; dx*dx+dy*dy < 0.01 {
				t.Fatalf("accepted points %v and %v closer than the constraint", p, q)
			}
		}
	}
}

// TestKdTreeVsPlainModel cross-checks the twin against the plain package
// under a fixed seed — same inserts, same query answers.
func TestKdTreeVsPlainModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tr := NewKdTree()
	var plain kd_tree.KdTree
	for range 500 {
		p := Point2D{X: math.Trunc(rng.Float64()*1000) / 10, Y: math.Trunc(rng.Float64()*1000) / 10}
		if got, want := tr.Insert(p), plain.Insert(p); got != want {
			t.Fatalf("Insert(%v) = %v, plain %v", p, got, want)
		}
	}
	for range 200 {
		q := Point2D{X: rng.Float64() * 100, Y: rng.Float64() * 100}
		kp, kok := tr.Nearest(q)
		pp, pok := plain.Nearest(q)
		if kok != pok || q.DistanceSquaredTo(kp) != q.DistanceSquaredTo(pp) {
			t.Fatalf("Nearest(%v): twin %v (d²=%v) ok=%v, plain %v (d²=%v) ok=%v",
				q, kp, q.DistanceSquaredTo(kp), kok, pp, q.DistanceSquaredTo(pp), pok)
		}
		kk, ok := tr.NearestK(q, 5), plain.NearestK(q, 5)
		if !slices.Equal(kk, ok) {
			t.Fatalf("NearestK(%v, 5) disagrees with the plain package", q)
		}
		r := RectHV{
			Xmin: rng.Float64() * 40, Ymin: rng.Float64() * 40,
			Xmax: rng.Float64()*60 + 40, Ymax: rng.Float64()*60 + 40,
		}
		if got, want := sortPoints(tr.Range(r)), sortPoints(plain.Range(r)); !slices.Equal(got, want) {
			t.Fatalf("Range(%v) disagrees with the plain package", r)
		}
	}
}

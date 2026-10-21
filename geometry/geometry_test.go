/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geometry

import (
	"math"
	"slices"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// expectPanic runs fn and fails the test unless it panics with the
// given message.
func expectPanic(t *testing.T, name, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if got, ok := r.(string); !ok || got != want {
			t.Errorf("%s panicked with %v, want %q", name, r, want)
		}
	}()
	fn()
}

var pt = func(x, y float64) g_lib.Point2D { return g_lib.Point2D{X: x, Y: y} }

func TestGrahamScanSquare(t *testing.T) {
	// A square with an interior point and a mid-edge point: the hull
	// is the four corners only (corner-only convention).
	g := NewGrahamScan([]g_lib.Point2D{
		pt(1, 1), pt(2, 0), pt(2, 2), pt(0, 2), pt(0, 0), pt(1, 0),
	})
	hull := g.Hull()
	want := []g_lib.Point2D{pt(0, 0), pt(2, 0), pt(2, 2), pt(0, 2)}
	if !slices.Equal(hull, want) {
		t.Fatalf("square hull: got %v, want %v (ccw from the lowest point)", hull, want)
	}
	if g.Len() != 4 {
		t.Fatalf("Len = %d, want 4", g.Len())
	}
}

func TestGrahamScanDegenerate(t *testing.T) {
	// Collinear input: the hull is the two endpoints.
	g := NewGrahamScan([]g_lib.Point2D{pt(2, 2), pt(0, 0), pt(3, 3), pt(1, 1)})
	if hull, want := g.Hull(), []g_lib.Point2D{pt(0, 0), pt(3, 3)}; !slices.Equal(hull, want) {
		t.Fatalf("collinear hull: got %v, want %v", hull, want)
	}

	// A long collinear run in shuffled order — the input shape that
	// breaks an unstable polar sort (the farthest point of the run
	// must be found last among the angle ties).
	pts := make([]g_lib.Point2D, 0, 64)
	for i := range 32 {
		pts = append(pts, pt(float64(i), float64(2*i))) // y = 2x
	}
	slices.Reverse(pts)
	g = NewGrahamScan(pts)
	if hull, want := g.Hull(), []g_lib.Point2D{pt(0, 0), pt(31, 62)}; !slices.Equal(hull, want) {
		t.Fatalf("shuffled collinear hull: got %v, want %v", hull, want)
	}

	// Horizontal and vertical collinear runs.
	g = NewGrahamScan([]g_lib.Point2D{pt(5, 0), pt(1, 0), pt(3, 0)})
	if hull, want := g.Hull(), []g_lib.Point2D{pt(1, 0), pt(5, 0)}; !slices.Equal(hull, want) {
		t.Fatalf("horizontal hull: got %v, want %v", hull, want)
	}
	g = NewGrahamScan([]g_lib.Point2D{pt(0, 5), pt(0, 1), pt(0, 3)})
	if hull, want := g.Hull(), []g_lib.Point2D{pt(0, 1), pt(0, 5)}; !slices.Equal(hull, want) {
		t.Fatalf("vertical hull: got %v, want %v", hull, want)
	}

	// All points equal: the hull is that single point.
	g = NewGrahamScan([]g_lib.Point2D{pt(1, 1), pt(1, 1), pt(1, 1)})
	if hull, want := g.Hull(), []g_lib.Point2D{pt(1, 1)}; !slices.Equal(hull, want) {
		t.Fatalf("all-equal hull: got %v, want %v", hull, want)
	}

	// Two points, a single point, no points.
	if hull, want := NewGrahamScan([]g_lib.Point2D{pt(1, 5), pt(2, 3)}).Hull(),
		[]g_lib.Point2D{pt(2, 3), pt(1, 5)}; !slices.Equal(hull, want) {
		t.Fatalf("two-point hull: got %v, want %v", hull, want)
	}
	if hull, want := NewGrahamScan([]g_lib.Point2D{pt(4, 9)}).Hull(),
		[]g_lib.Point2D{pt(4, 9)}; !slices.Equal(hull, want) {
		t.Fatalf("one-point hull: got %v, want %v", hull, want)
	}
	if hull := NewGrahamScan(nil).Hull(); hull == nil || len(hull) != 0 {
		t.Fatalf("empty hull: got %v, want empty non-nil", hull)
	}
}

// TestGrahamScanInputNotModified pins the purity contract.
func TestGrahamScanInputNotModified(t *testing.T) {
	pts := []g_lib.Point2D{pt(3, 1), pt(1, 3), pt(2, 2), pt(0, 0)}
	before := slices.Clone(pts)
	_ = NewGrahamScan(pts)
	if !slices.Equal(pts, before) {
		t.Fatalf("input modified: got %v, want %v", pts, before)
	}
}

func TestClosestPair(t *testing.T) {
	c := NewClosestPair([]g_lib.Point2D{
		pt(0, 0), pt(10, 10), pt(1, 1), pt(10, 0), pt(3, 4),
	})
	p, q, found := c.Pair()
	if !found {
		t.Fatalf("closest pair not found")
	}
	if got := p.DistanceSquaredTo(q); got != 2 {
		t.Fatalf("closest pair distanceSquared = %v, want 2 ((0,0) and (1,1))", got)
	}
	if c.Distance() != math.Sqrt2 {
		t.Fatalf("Distance = %v, want sqrt(2)", c.Distance())
	}

	// Coincident points are a legal pair at distance 0.
	c = NewClosestPair([]g_lib.Point2D{pt(5, 5), pt(1, 2), pt(5, 5), pt(9, 9)})
	if _, _, found = c.Pair(); !found || c.Distance() != 0 {
		t.Fatalf("coincident pair: found=%v distance=%v, want true/0", found, c.Distance())
	}

	// Fewer than two points: no pair, +Inf.
	for _, pts := range [][]g_lib.Point2D{nil, {pt(1, 1)}} {
		c = NewClosestPair(pts)
		if _, _, found = c.Pair(); found {
			t.Fatalf("pair found for %v", pts)
		}
		if c.Distance() != math.Inf(1) {
			t.Fatalf("Distance for %v = %v, want +Inf", pts, c.Distance())
		}
	}
}

func TestFarthestPair(t *testing.T) {
	// A 6×8 rectangle: the farthest pair is the diagonal sqrt(100) = 10.
	f := NewFarthestPair([]g_lib.Point2D{
		pt(0, 0), pt(6, 0), pt(6, 8), pt(0, 8), pt(3, 4), pt(0, 5),
	})
	p, q, found := f.Pair()
	if !found {
		t.Fatalf("farthest pair not found")
	}
	if d := p.DistanceTo(q); d != 10 {
		t.Fatalf("farthest pair distance = %v, want 10 (the diagonal)", d)
	}
	if f.Distance() != 10 {
		t.Fatalf("Distance = %v, want 10", f.Distance())
	}

	// Collinear input: the endpoints.
	f = NewFarthestPair([]g_lib.Point2D{pt(1, 1), pt(5, 5), pt(3, 3), pt(9, 9)})
	if p, q, found = f.Pair(); !found || p.DistanceTo(q) != 8*math.Sqrt2 {
		t.Fatalf("collinear farthest: found=%v p=%v q=%v, want (1,1)/(9,9)", found, p, q)
	}

	// No pair: empty, single, and all-equal input.
	for _, pts := range [][]g_lib.Point2D{
		nil, {pt(1, 1)}, {pt(2, 2), pt(2, 2), pt(2, 2)},
	} {
		f = NewFarthestPair(pts)
		if _, _, found = f.Pair(); found {
			t.Fatalf("pair found for %v", pts)
		}
		if f.Distance() != 0 {
			t.Fatalf("Distance for %v = %v, want 0", pts, f.Distance())
		}
	}
}

// TestNaNPanics is the package's whole panic contract: one class, a
// NaN coordinate at construction, each message naming its constructor.
func TestNaNPanics(t *testing.T) {
	bad := []g_lib.Point2D{pt(0, 0), {X: math.NaN(), Y: 1}}
	expectPanic(t, "NewGrahamScan with NaN",
		"geometry: NewGrahamScan called with input containing a NaN coordinate",
		func() { NewGrahamScan(bad) })
	expectPanic(t, "NewClosestPair with NaN",
		"geometry: NewClosestPair called with input containing a NaN coordinate",
		func() { NewClosestPair(bad) })
	expectPanic(t, "NewFarthestPair with NaN",
		"geometry: NewFarthestPair called with input containing a NaN coordinate",
		func() { NewFarthestPair(bad) })
}

// TestNilAndZeroValue pins the tolerant semantics: a nil or zero-value
// query object behaves as the empty case everywhere, never panics.
func TestNilAndZeroValue(t *testing.T) {
	var g *GrahamScan
	if hull := g.Hull(); hull == nil || len(hull) != 0 {
		t.Errorf("nil GrahamScan.Hull = %v, want empty non-nil", hull)
	}
	if g.Len() != 0 {
		t.Errorf("nil GrahamScan.Len = %d, want 0", g.Len())
	}
	var g0 GrahamScan // zero value
	if g0.Len() != 0 || len(g0.Hull()) != 0 {
		t.Errorf("zero-value GrahamScan not empty")
	}

	var c *ClosestPair
	if _, _, found := c.Pair(); found {
		t.Errorf("nil ClosestPair.Pair found a pair")
	}
	if c.Distance() != math.Inf(1) {
		t.Errorf("nil ClosestPair.Distance = %v, want +Inf", c.Distance())
	}
	var c0 ClosestPair
	if _, _, found := c0.Pair(); found || c0.Distance() != math.Inf(1) {
		t.Errorf("zero-value ClosestPair not the empty case")
	}

	var f *FarthestPair
	if _, _, found := f.Pair(); found {
		t.Errorf("nil FarthestPair.Pair found a pair")
	}
	if f.Distance() != 0 {
		t.Errorf("nil FarthestPair.Distance = %v, want 0", f.Distance())
	}
	var f0 FarthestPair
	if _, _, found := f0.Pair(); found || f0.Distance() != 0 {
		t.Errorf("zero-value FarthestPair not the empty case")
	}
}

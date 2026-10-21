/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the independent-oracle suite — GrahamScan against
// a Jarvis-march (gift wrapping) hull implementation, ClosestPair and
// FarthestPair against O(n²) brute force — over uniform random,
// integer-grid (exact CCW), cocircular (max tie density), and
// collinear-heavy inputs, at fixed PCG seeds.

package geometry

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// jarvisHull is the independent hull oracle: gift wrapping — from the
// lowest point, repeatedly take the endpoint such that no point is
// strictly right of the edge, breaking collinear ties toward the
// FARthest candidate so mid-edge points are skipped (the corner-only
// convention GrahamScan shares).
func jarvisHull(pts []g_lib.Point2D) []g_lib.Point2D {
	n := len(pts)
	if n == 0 {
		return nil
	}
	start := 0
	for i := 1; i < n; i++ {
		if g_lib.CompareYX(pts[i], pts[start]) < 0 {
			start = i
		}
	}
	hull := []g_lib.Point2D{pts[start]}
	cur := start
	for range n + 1 {
		next := -1
		for cand := range n {
			if cand == cur || pts[cand] == pts[cur] {
				continue // never step to the current point or its duplicates
			}
			if next < 0 {
				next = cand
				continue
			}
			t := g_lib.CCW(pts[cur], pts[next], pts[cand])
			if t < 0 || (t == 0 && pts[cur].DistanceSquaredTo(pts[cand]) > pts[cur].DistanceSquaredTo(pts[next])) {
				next = cand
			}
		}
		if next < 0 || pts[next] == pts[start] {
			break // wrapped (or only duplicates of the start remain)
		}
		hull = append(hull, pts[next])
		cur = next
	}
	return hull
}

// checkHullProperties verifies the geometric invariants directly:
// strict left turns all the way around (corner-only), every input
// point on or left of every hull edge, the canonical start vertex,
// and every hull vertex present in the input.
func checkHullProperties(t *testing.T, hull, pts []g_lib.Point2D) {
	t.Helper()
	m := len(hull)
	if m == 0 {
		return
	}
	// Canonical start: the lowest point, ties by lowest x.
	for _, p := range pts {
		if g_lib.CompareYX(p, hull[0]) < 0 {
			t.Fatalf("hull starts at %v but %v is lower", hull[0], p)
		}
	}
	// Strictly convex ccw (corner-only) once the hull is 2-dimensional.
	if m >= 3 {
		for i := range m {
			if g_lib.CCW(hull[(i+m-1)%m], hull[i], hull[(i+1)%m]) <= 0 {
				t.Fatalf("hull not strictly ccw at %v (triple %v %v %v)",
					i, hull[(i+m-1)%m], hull[i], hull[(i+1)%m])
			}
		}
	}
	// Every input point is on or left of every directed hull edge.
	for i := range m {
		a, b := hull[i], hull[(i+1)%m]
		for _, p := range pts {
			if g_lib.CCW(a, b, p) < 0 {
				t.Fatalf("point %v is right of hull edge %v→%v", p, a, b)
			}
		}
	}
	// Hull vertices come from the input (Point2D is comparable).
	for _, v := range hull {
		if !slices.Contains(pts, v) {
			t.Fatalf("hull vertex %v not in input", v)
		}
	}
}

// randomPoints draws n uniform points from [0,1)² at the given rng.
func randomPoints(rng *rand.Rand, n int) []g_lib.Point2D {
	pts := make([]g_lib.Point2D, n)
	for i := range pts {
		pts[i] = g_lib.Point2D{X: rng.Float64(), Y: rng.Float64()}
	}
	return pts
}

// TestGrahamScanVsJarvisOracle is the hull's defining verification:
// exact sequence equality (same start, same ccw corner order) with
// the independent gift-wrapping implementation, over uniform random
// inputs of every small size including the degenerate counts.
func TestGrahamScanVsJarvisOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for range 500 {
		n := rng.IntN(120)
		pts := randomPoints(rng, n)
		got := NewGrahamScan(pts).Hull()
		want := jarvisHull(pts)
		if !slices.Equal(got, want) {
			t.Fatalf("hull disagreement n=%d: graham %v, jarvis %v", n, got, want)
		}
		checkHullProperties(t, got, pts)
	}
}

// TestGrahamScanIntegerGrid runs complete integer grids — 5×5, 6×6,
// 7×7 — the densest collinearity an integer lattice offers (rows,
// columns, both diagonals, knight lines...): CCW is exact there, so
// any disagreement is an algorithm bug, not float noise.
func TestGrahamScanIntegerGrid(t *testing.T) {
	for side := 5; side <= 7; side++ {
		var pts []g_lib.Point2D
		for x := range side {
			for y := range side {
				pts = append(pts, pt(float64(x), float64(y)))
			}
		}
		got := NewGrahamScan(pts).Hull()
		want := jarvisHull(pts)
		if !slices.Equal(got, want) {
			t.Fatalf("%dx%d grid: graham %v, jarvis %v", side, side, got, want)
		}
		if len(got) != 4 {
			t.Fatalf("%dx%d grid hull has %d vertices, want the 4 corners", side, side, len(got))
		}
		checkHullProperties(t, got, pts)
	}
}

// TestGrahamScanCollinearHeavy puts most points on a few random
// lines with INTEGER coordinates — exact arithmetic, so the
// degenerate-turn storm (collinear ties everywhere) is exercised
// with CCW exact and the oracle agreement is meaningful.
func TestGrahamScanCollinearHeavy(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 23))
	for range 300 {
		k := 1 + rng.IntN(3)
		var pts []g_lib.Point2D
		for range k {
			x0, y0 := rng.IntN(20)-10, rng.IntN(20)-10
			dx, dy := rng.IntN(5)-2, rng.IntN(5)-2
			if dx == 0 && dy == 0 {
				dx = 1
			}
			for range 1 + rng.IntN(20) {
				s := rng.IntN(8)
				pts = append(pts, pt(float64(x0+dx*s), float64(y0+dy*s)))
			}
		}
		got := NewGrahamScan(pts).Hull()
		want := jarvisHull(pts)
		if !slices.Equal(got, want) {
			t.Fatalf("collinear-heavy disagreement: graham %v, jarvis %v", got, want)
		}
		checkHullProperties(t, got, pts)
	}
}

// TestGrahamScanNearlyCollinearNoPanic pins the tolerant contract on
// the input class exact-arithmetic reasoning does not cover: points
// on float lines, where CCW's sign is rounding noise.  The scan must
// terminate without panic (the base-vertex guard), return hull
// vertices drawn from the input, and produce SOME closed ccw circuit
// — but no exact oracle is asserted, because with the orientation
// predicate at noise level there is no "the" hull to agree with (the
// algs4 original crashes on this input class; pluto degrades).
func TestGrahamScanNearlyCollinearNoPanic(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 23))
	for range 500 {
		k := 1 + rng.IntN(3)
		var pts []g_lib.Point2D
		for range k {
			x0, y0 := rng.Float64()*4, rng.Float64()*4
			dx, dy := rng.Float64()-0.5, rng.Float64()-0.5
			if dx == 0 && dy == 0 {
				dx = 1
			}
			for range 1 + rng.IntN(20) {
				s := rng.Float64() * 3
				pts = append(pts, pt(x0+dx*s, y0+dy*s))
			}
		}
		got := NewGrahamScan(pts).Hull()
		if len(pts) == 0 {
			continue
		}
		if len(got) < 2 && len(pts) >= 2 {
			// At least a segment must come back... unless every point
			// collapses to one value, which float lines do not do.
			seen := map[g_lib.Point2D]bool{}
			for _, p := range pts {
				seen[p] = true
			}
			if len(seen) > 1 {
				t.Fatalf("hull of a spread input has %d vertices: %v over %v", len(got), got, pts)
			}
			continue
		}
		for _, v := range got {
			if !slices.Contains(pts, v) {
				t.Fatalf("hull vertex %v not in input", v)
			}
		}
	}
}

// TestGrahamScanCocircular puts every point on a circle — the
// maximum-hull-size case (all points are hull vertices), with
// symmetric collinearity ties across the base point.
func TestGrahamScanCocircular(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 31))
	for _, n := range []int{3, 4, 5, 8, 64, 257} {
		pts := make([]g_lib.Point2D, n)
		for i := range n {
			theta := 2 * math.Pi * float64(i) / float64(n)
			pts[i] = pt(1+math.Cos(theta), 1+math.Sin(theta))
		}
		rng.Shuffle(n, func(i, j int) { pts[i], pts[j] = pts[j], pts[i] })
		got := NewGrahamScan(pts).Hull()
		want := jarvisHull(pts)
		if !slices.Equal(got, want) {
			t.Fatalf("cocircular n=%d: graham %v, jarvis %v", n, got, want)
		}
		if len(got) != n {
			t.Fatalf("cocircular n=%d hull has %d vertices, want %d", n, len(got), n)
		}
		checkHullProperties(t, got, pts)
	}
}

// bruteClosest is the O(n²) closest-pair oracle over squared
// distances.  ok mirrors the package contract: a pair exists iff
// there are at least two points.
func bruteClosest(pts []g_lib.Point2D) (float64, bool) {
	best := math.Inf(1)
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if d := pts[i].DistanceSquaredTo(pts[j]); d < best {
				best = d
			}
		}
	}
	return best, len(pts) >= 2
}

// TestClosestPairVsBruteForce checks the reported pair's squared
// distance against the brute-force minimum EXACTLY — both sides
// compute dx*dx+dy*dy over the same float64 values, so equality is
// bitwise.  Inputs: uniform random, duplicate-heavy tiny grids
// (many zero and near-zero pairs), integer grids (exact), and
// collinear runs.
func TestClosestPairVsBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	one := func(pts []g_lib.Point2D) {
		t.Helper()
		c := NewClosestPair(pts)
		wantD, wantOK := bruteClosest(pts)
		p, q, found := c.Pair()
		if found != wantOK {
			t.Fatalf("found=%v, want %v (n=%d)", found, wantOK, len(pts))
		}
		if !found {
			if c.Distance() != math.Inf(1) {
				t.Fatalf("no pair, Distance = %v, want +Inf", c.Distance())
			}
			return
		}
		if got := p.DistanceSquaredTo(q); got != wantD {
			t.Fatalf("closest squared distance %v, brute force %v", got, wantD)
		}
		if c.Distance() != math.Sqrt(wantD) {
			t.Fatalf("Distance %v, want sqrt(%v)", c.Distance(), wantD)
		}
	}

	for range 500 {
		one(randomPoints(rng, rng.IntN(150)))
	}
	// Duplicate-heavy: coordinates on a coarse grid, n four times the
	// grid area — coincident and near-coincident pairs everywhere.
	for range 300 {
		n := 20 + rng.IntN(80)
		pts := make([]g_lib.Point2D, n)
		for i := range pts {
			pts[i] = pt(float64(rng.IntN(6)), float64(rng.IntN(6)))
		}
		one(pts)
	}
	// Integer grid: exact arithmetic.
	for side := 5; side <= 7; side++ {
		var pts []g_lib.Point2D
		for x := range side {
			for y := range side {
				pts = append(pts, pt(float64(x), float64(y)))
			}
		}
		one(pts)
	}
	// Collinear run: the closest pair are neighbors.
	for range 100 {
		n := 2 + rng.IntN(50)
		pts := make([]g_lib.Point2D, n)
		for i := range pts {
			pts[i] = pt(float64(i), 0)
		}
		rng.Shuffle(n, func(i, j int) { pts[i], pts[j] = pts[j], pts[i] })
		one(pts)
	}
}

// bruteFarthest is the O(n²) farthest-pair oracle over squared
// distances.  ok mirrors the package contract: a pair exists iff at
// least two distinct points are present.
func bruteFarthest(pts []g_lib.Point2D) (float64, bool) {
	best := 0.0
	distinct := false
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if pts[i] != pts[j] {
				distinct = true
			}
			if d := pts[i].DistanceSquaredTo(pts[j]); d > best {
				best = d
			}
		}
	}
	return best, distinct
}

// TestFarthestPairVsBruteForce checks the reported pair against the
// brute-force maximum — including the calipers killers: cocircular
// sets (every point a hull vertex, antipodal ties everywhere),
// squares and rectangles (parallel opposite edges), and collinear
// runs (degenerate two-vertex hulls).
func TestFarthestPairVsBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	one := func(pts []g_lib.Point2D) {
		t.Helper()
		f := NewFarthestPair(pts)
		wantD, wantOK := bruteFarthest(pts)
		p, q, found := f.Pair()
		if found != wantOK {
			t.Fatalf("found=%v, want %v (n=%d)", found, wantOK, len(pts))
		}
		if !found {
			if f.Distance() != 0 {
				t.Fatalf("no pair, Distance = %v, want 0", f.Distance())
			}
			return
		}
		if got := p.DistanceSquaredTo(q); got != wantD {
			t.Fatalf("farthest squared distance %v, brute force %v (pair %v %v)", got, wantD, p, q)
		}
		if f.Distance() != math.Sqrt(wantD) {
			t.Fatalf("Distance %v, want sqrt(%v)", f.Distance(), wantD)
		}
	}

	for range 500 {
		one(randomPoints(rng, rng.IntN(120)))
	}
	// Cocircular: all points on the hull; for n even the diameter is
	// exact antipodal, for n odd near-antipodal — the tie storm.
	for _, n := range []int{3, 4, 5, 6, 7, 8, 16, 101, 256} {
		pts := make([]g_lib.Point2D, n)
		for i := range n {
			theta := 2 * math.Pi * float64(i) / float64(n)
			pts[i] = pt(2+3*math.Cos(theta), -1+3*math.Sin(theta))
		}
		rng.Shuffle(n, func(i, j int) { pts[i], pts[j] = pts[j], pts[i] })
		one(pts)
	}
	// Duplicate-heavy tiny grids and collinear runs.
	for range 300 {
		n := 20 + rng.IntN(60)
		pts := make([]g_lib.Point2D, n)
		for i := range pts {
			pts[i] = pt(float64(rng.IntN(5)), float64(rng.IntN(5)))
		}
		one(pts)
	}
	for range 100 {
		n := 2 + rng.IntN(40)
		pts := make([]g_lib.Point2D, n)
		for i := range pts {
			pts[i] = pt(float64(i%17), float64(i))
		}
		rng.Shuffle(n, func(i, j int) { pts[i], pts[j] = pts[j], pts[i] })
		one(pts)
	}
	// The parallel-edge cases, hand-shaped: square and rectangle with
	// interior clutter.
	one([]g_lib.Point2D{pt(0, 0), pt(1, 0), pt(1, 1), pt(0, 1), pt(0.5, 0.5), pt(0.25, 0.75)})
	one([]g_lib.Point2D{pt(-3, -4), pt(3, -4), pt(3, 4), pt(-3, 4), pt(0, 0), pt(-3, 0), pt(3, 0)})
}

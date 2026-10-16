/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// sortPoints orders points by (X, Y) — the canonical comparison used
// everywhere the tests need a deterministic order.
func sortPoints(pts []g_lib.Point2D) []g_lib.Point2D {
	out := slices.Clone(pts)
	slices.SortFunc(out, func(a, b g_lib.Point2D) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
	return out
}

// treeDepth returns the height of the subtree (a nil node is depth 0).
func treeDepth(n *kdNode) int {
	if n == nil {
		return 0
	}
	return 1 + max(treeDepth(n.left), treeDepth(n.right))
}

// allCoordLess reports whether every point of the subtree has the
// given coordinate strictly below x; allCoordAtLeast is the ≥ mirror.
func allCoordLess(n *kdNode, x float64) bool {
	if n == nil {
		return true
	}
	return n.p.X < x && allCoordLess(n.left, x) && allCoordLess(n.right, x)
}

func allCoordAtLeast(n *kdNode, x float64) bool {
	if n == nil {
		return true
	}
	return n.p.X >= x && allCoordAtLeast(n.left, x) && allCoordAtLeast(n.right, x)
}

// checkInvariants verifies the structural contracts of the k-d tree —
// call it after any structural change:
//
//   - every node's point lies in the rectangle its ancestors' splits
//     assign to it (subtree point containment),
//   - the partition property: at a vertical node every left-subtree
//     point has smaller x and every right-subtree point x ≥ the node's
//     x (mirrored for horizontal nodes) — partition by depth,
//   - no duplicate points, and the node count equals the Len field,
//   - Contains finds every point the walk visits.
func checkInvariants(t *testing.T, tr *KdTree) {
	t.Helper()
	seen := make(map[g_lib.Point2D]bool)
	checkNode(t, tr.root, wholePlane, true, seen)
	if len(seen) != tr.n {
		t.Fatalf("walk found %d distinct points, Len reports %d", len(seen), tr.n)
	}
	for p := range seen {
		if !tr.Contains(p) {
			t.Fatalf("Contains misses in-tree point %v", p)
		}
	}
}

func checkNode(t *testing.T, n *kdNode, rect g_lib.RectHV, vertical bool, seen map[g_lib.Point2D]bool) {
	t.Helper()
	if n == nil {
		return
	}
	if !rect.Contains(n.p) {
		t.Fatalf("point %v lies outside its subtree rectangle %v", n.p, rect)
	}
	if seen[n.p] {
		t.Fatalf("duplicate point %v in the tree", n.p)
	}
	seen[n.p] = true
	if vertical {
		if !allCoordLess(n.left, n.p.X) || !allCoordAtLeast(n.right, n.p.X) {
			t.Fatalf("partition violated at vertical node %v", n.p)
		}
	} else {
		if !allCoordLessY(n.left, n.p.Y) || !allCoordAtLeastY(n.right, n.p.Y) {
			t.Fatalf("partition violated at horizontal node %v", n.p)
		}
	}
	lo, hi := splitRects(n, rect, vertical)
	checkNode(t, n.left, lo, !vertical, seen)
	checkNode(t, n.right, hi, !vertical, seen)
}

func allCoordLessY(n *kdNode, y float64) bool {
	if n == nil {
		return true
	}
	return n.p.Y < y && allCoordLessY(n.left, y) && allCoordLessY(n.right, y)
}

func allCoordAtLeastY(n *kdNode, y float64) bool {
	if n == nil {
		return true
	}
	return n.p.Y >= y && allCoordAtLeastY(n.left, y) && allCoordAtLeastY(n.right, y)
}

// modelPoint draws a query/insert point; the coarse one-decimal grid
// creates exact distance ties often enough to exercise the tie
// contracts (both structures compute d² through the same float
// expression, so equality comparisons stay exact).
func modelPoint(rng *rand.Rand) g_lib.Point2D {
	return g_lib.Point2D{
		X: math.Trunc(rng.Float64()*1000) / 10,
		Y: math.Trunc(rng.Float64()*1000) / 10,
	}
}

// TestKdTreeRandomizedModel cross-checks every operation against the
// brute-force PointSet oracle — the algs4 pairing of KdTree and
// PointSET run as a differential test (PCG 42,7, fixed seed).
func TestKdTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tr := NewKdTree()
	ps := NewPointSet()
	var inserted []g_lib.Point2D

	for step := 0; step < 4000; step++ {
		switch op := rng.IntN(100); {
		case op < 40: // insert (a quarter of them duplicates)
			var p g_lib.Point2D
			if len(inserted) > 0 && rng.IntN(4) == 0 {
				p = inserted[rng.IntN(len(inserted))]
			} else {
				p = modelPoint(rng)
			}
			added := tr.Insert(p)
			if want := ps.Insert(p); added != want {
				t.Fatalf("step %d: Insert(%v) = %v, oracle %v", step, p, added, want)
			}
			if added {
				inserted = append(inserted, p)
			}

		case op < 60: // contains
			var p g_lib.Point2D
			if len(inserted) > 0 && rng.IntN(2) == 0 {
				p = inserted[rng.IntN(len(inserted))]
			} else {
				p = modelPoint(rng)
			}
			if got, want := tr.Contains(p), ps.Contains(p); got != want {
				t.Fatalf("step %d: Contains(%v) = %v, oracle %v", step, p, got, want)
			}

		case op < 80: // nearest
			q := modelPoint(rng)
			kp, kok := tr.Nearest(q)
			op_, ook := ps.Nearest(q)
			if kok != ook {
				t.Fatalf("step %d: Nearest(%v) found = %v, oracle %v", step, q, kok, ook)
			}
			if kok {
				kd2, od2 := q.DistanceSquaredTo(kp), q.DistanceSquaredTo(op_)
				if kd2 != od2 {
					t.Fatalf("step %d: Nearest(%v) = %v (d²=%v), oracle %v (d²=%v)",
						step, q, kp, kd2, op_, od2)
				}
				// Different points are legal only as an exact tie: the
				// answer must itself be an oracle point at the minimum.
				if kp != op_ && (!ps.Contains(kp) || q.DistanceSquaredTo(kp) != od2) {
					t.Fatalf("step %d: Nearest(%v) = %v, not a minimum-distance point", step, q, kp)
				}
			}

		case op < 90: // nearest-k
			q := modelPoint(rng)
			k := 1 + rng.IntN(8)
			kk := tr.NearestK(q, k)
			ok := ps.NearestK(q, k)
			if len(kk) != len(ok) {
				t.Fatalf("step %d: NearestK(%v, %d) has %d points, oracle %d", step, q, k, len(kk), len(ok))
			}
			for i := range kk {
				if q.DistanceSquaredTo(kk[i]) != q.DistanceSquaredTo(ok[i]) {
					t.Fatalf("step %d: NearestK(%v, %d)[%d] = %v, oracle %v (distance mismatch)",
						step, q, k, i, kk[i], ok[i])
				}
			}

		default: // range
			x0, y0 := modelPoint(rng), modelPoint(rng)
			dy := rng.Float64() * 50
			r := g_lib.RectHV{
				Xmin: math.Min(x0.X, y0.X), Xmax: math.Max(x0.X, y0.X),
				Ymin: math.Min(x0.Y, y0.Y+dy), Ymax: math.Max(x0.Y, y0.Y+dy),
			}
			got, want := sortPoints(tr.Range(r)), sortPoints(ps.Range(r))
			if !slices.Equal(got, want) {
				t.Fatalf("step %d: Range(%v) = %v, oracle %v", step, r, got, want)
			}
		}

		if step%37 == 0 {
			checkInvariants(t, tr)
			var all []g_lib.Point2D
			for p := range tr.All() {
				all = append(all, p)
			}
			var oracle []g_lib.Point2D
			for p := range ps.All() {
				oracle = append(oracle, p)
			}
			if got, want := sortPoints(all), sortPoints(oracle); !slices.Equal(got, want) {
				t.Fatalf("step %d: full-state mismatch: %v vs oracle %v", step, got, want)
			}
		}
	}
}

// TestGridCircleClient is the algs4 nearest-neighbor assignment's
// circle client: uniformly random points, queries taken around a
// circle so that the answer set is geometrically clustered and the
// rectangle prunes have real work to do.
func TestGridCircleClient(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tr := NewKdTree()
	ps := NewPointSet()
	for range 1000 {
		p := g_lib.Point2D{X: rng.Float64(), Y: rng.Float64()}
		tr.Insert(p)
		ps.Insert(p)
	}
	const cx, cy, radius = 0.5, 0.5, 0.25
	for i := range 200 {
		theta := 2 * math.Pi * float64(i) / 200
		q := g_lib.Point2D{X: cx + radius*math.Cos(theta), Y: cy + radius*math.Sin(theta)}

		kp, kok := tr.Nearest(q)
		op, ook := ps.Nearest(q)
		if kok != ook || q.DistanceSquaredTo(kp) != q.DistanceSquaredTo(op) {
			t.Fatalf("circle query %d: Nearest(%v) disagrees with the oracle", i, q)
		}

		kk, ok := tr.NearestK(q, 10), ps.NearestK(q, 10)
		for j := range kk {
			if q.DistanceSquaredTo(kk[j]) != q.DistanceSquaredTo(ok[j]) {
				t.Fatalf("circle query %d: NearestK rank %d disagrees with the oracle", i, j)
			}
		}

		box := g_lib.RectHV{Xmin: cx - radius, Ymin: cy - radius, Xmax: cx + radius, Ymax: cy + radius}
		got, want := sortPoints(tr.Range(box)), sortPoints(ps.Range(box))
		if !slices.Equal(got, want) {
			t.Fatalf("circle query %d: bounding-box Range disagrees with the oracle", i)
		}
	}
}

// TestNearestTies pins the tie contracts: equidistant answers may be
// any of the tied points (Nearest) but the k-distance sequence is exact
// (NearestK), and the final (X, Y) order is deterministic when the tie
// fits inside k.
func TestNearestTies(t *testing.T) {
	tr := NewKdTree()
	ps := NewPointSet()
	// Four points at unit distance from the origin, on the axes.
	for _, p := range []g_lib.Point2D{
		{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1},
	} {
		tr.Insert(p)
		ps.Insert(p)
	}
	q := g_lib.Point2D{}
	got, ok := tr.Nearest(q)
	if !ok || q.DistanceSquaredTo(got) != 1 {
		t.Fatalf("Nearest(origin) = %v %v, want one of the unit points", got, ok)
	}
	if !slices.Contains([]g_lib.Point2D{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}, got) {
		t.Fatalf("Nearest(origin) = %v is not a tied point", got)
	}
	// k=2 of a 4-way tie: any two, both at distance 1.
	kk := tr.NearestK(q, 2)
	if len(kk) != 2 || q.DistanceSquaredTo(kk[0]) != 1 || q.DistanceSquaredTo(kk[1]) != 1 {
		t.Fatalf("NearestK(origin, 2) = %v, want two unit-distance points", kk)
	}
	// k=5 takes all four; the (X, Y) tie order is deterministic.
	all := tr.NearestK(q, 5)
	want := []g_lib.Point2D{{X: -1, Y: 0}, {X: 0, Y: -1}, {X: 0, Y: 1}, {X: 1, Y: 0}}
	if !slices.Equal(all, want) {
		t.Fatalf("NearestK(origin, 5) = %v, want %v", all, want)
	}
	if got := ps.NearestK(q, 5); !slices.Equal(got, want) {
		t.Fatalf("PointSet NearestK(origin, 5) = %v, want %v", got, want)
	}
}

// TestDegenerateChain pins the documented worst case: points monotone
// along the diagonal build a depth-n chain (no rebalancing), and every
// operation still answers correctly — just in O(n).
func TestDegenerateChain(t *testing.T) {
	const n = 1000
	tr := NewKdTree()
	for i := range n {
		if !tr.Insert(g_lib.Point2D{X: float64(i), Y: float64(i)}) {
			t.Fatalf("Insert(%d, %d) reported duplicate", i, i)
		}
	}
	if d := treeDepth(tr.root); d != n {
		t.Fatalf("monotone diagonal builds depth %d, want the full chain %d", d, n)
	}
	for i := range n {
		if !tr.Contains(g_lib.Point2D{X: float64(i), Y: float64(i)}) {
			t.Fatalf("Contains missed (%d, %d)", i, i)
		}
	}
	got, ok := tr.Nearest(g_lib.Point2D{X: 100.4, Y: 100.6})
	if !ok || got != (g_lib.Point2D{X: 100, Y: 100}) {
		t.Fatalf("Nearest near (100,100) = %v %v", got, ok)
	}
	line := g_lib.RectHV{Xmin: 10.5, Ymin: 10.5, Xmax: 12.5, Ymax: 12.5}
	if got := sortPoints(tr.Range(line)); len(got) != 2 || got[0] != (g_lib.Point2D{X: 11, Y: 11}) || got[1] != (g_lib.Point2D{X: 12, Y: 12}) {
		t.Fatalf("Range on the chain = %v, want (11,11) (12,12)", got)
	}
}

// TestShuffledUniformDepth pins the expected-case shape: uniformly
// random insertion order keeps the tree near logarithmic depth.
func TestShuffledUniformDepth(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tr := NewKdTree()
	const n = 10_000
	for range n {
		tr.Insert(g_lib.Point2D{X: rng.Float64(), Y: rng.Float64()})
	}
	d := treeDepth(tr.root)
	// Expected depth for random insertion is ~2·ln n ≈ 19; 4·log₂ n is
	// a generous ceiling that still catches a structurally broken tree.
	if limit := 4 * math.Log2(n); float64(d) > limit {
		t.Fatalf("shuffled uniform depth %d exceeds %.0f", d, limit)
	}
	t.Logf("shuffled uniform depth %d (n=%d, 4·log₂n ≈ %.0f)", d, n, 4*math.Log2(n))
}

// TestRangeEdgeRects covers the degenerate query rectangles: zero-width
// (vertical/horizontal lines), empty, inverted, and whole-plane.
func TestRangeEdgeRects(t *testing.T) {
	tr := newTraceTree(t)

	vertLine := g_lib.RectHV{Xmin: 0.5, Ymin: 0, Xmax: 0.5, Ymax: 1}
	if got := sortPoints(tr.Range(vertLine)); len(got) != 1 || got[0] != ptB {
		t.Fatalf("vertical line through x=0.5 = %v, want [B]", got)
	}
	horizLine := g_lib.RectHV{Xmin: 0, Ymin: 0.6, Xmax: 1, Ymax: 0.6}
	if got := tr.Range(horizLine); len(got) != 1 || got[0] != ptE {
		t.Fatalf("horizontal line through y=0.6 = %v, want [E] (E sits on the line)", got)
	}
	horizNone := g_lib.RectHV{Xmin: 0, Ymin: 0.5, Xmax: 1, Ymax: 0.5}
	if got := tr.Range(horizNone); got != nil {
		t.Fatalf("horizontal line through y=0.5 = %v, want none", got)
	}
	empty := g_lib.RectHV{Xmin: 0.3, Ymin: 0.3, Xmax: 0.3, Ymax: 0.3}
	if got := tr.Range(empty); got != nil {
		t.Fatalf("point rectangle matching nothing = %v, want nil", got)
	}
	onPoint := g_lib.RectHV{Xmin: 0.2, Ymin: 0.3, Xmax: 0.2, Ymax: 0.3}
	if got := tr.Range(onPoint); len(got) != 1 || got[0] != ptC {
		t.Fatalf("point rectangle on C = %v, want [C]", got)
	}
	inverted := g_lib.RectHV{Xmin: 0.9, Ymin: 0.9, Xmax: 0.1, Ymax: 0.1}
	if got := tr.Range(inverted); got != nil {
		t.Fatalf("inverted rectangle = %v, want nil (tolerated, never a panic)", got)
	}
	if got := sortPoints(tr.Range(wholePlane)); len(got) != 5 {
		t.Fatalf("whole-plane Range = %v, want all 5 points", got)
	}
}

// TestAllYieldsEverything cross-checks All against the oracle on a
// randomized tree (map order on one side, preorder on the other).
func TestAllYieldsEverything(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	tr := NewKdTree()
	ps := NewPointSet()
	for range 500 {
		p := g_lib.Point2D{X: rng.Float64() * 100, Y: rng.Float64() * 100}
		tr.Insert(p)
		ps.Insert(p)
	}
	var all, oracle []g_lib.Point2D
	for p := range tr.All() {
		all = append(all, p)
	}
	for p := range ps.All() {
		oracle = append(oracle, p)
	}
	if got, want := sortPoints(all), sortPoints(oracle); !slices.Equal(got, want) {
		t.Fatalf("All disagrees with the oracle")
	}
	// Early termination stops the walk.
	count := 0
	for range tr.All() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("break inside All stopped at %d, want 3", count)
	}
}

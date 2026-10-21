/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geometry

import (
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// itoaa is strconv.Itoa hoisted for the b.Run names.
func itoa(n int) string { return strconv.Itoa(n) }

// benchPoints draws n uniform points from [0,1)² at a fixed PCG seed
// — reproducible benchmark input.
func benchPoints(n int) []g_lib.Point2D {
	rng := rand.New(rand.NewPCG(42, 23))
	pts := make([]g_lib.Point2D, n)
	for i := range pts {
		pts[i] = g_lib.Point2D{X: rng.Float64(), Y: rng.Float64()}
	}
	return pts
}

// benchCircle puts all n points on a circle — every point is a hull
// vertex, the maximum-hull worst case for the hull algorithms.
func benchCircle(n int) []g_lib.Point2D {
	pts := make([]g_lib.Point2D, n)
	for i := range n {
		theta := 2 * math.Pi * float64(i) / float64(n)
		pts[i] = g_lib.Point2D{X: 1 + math.Cos(theta), Y: 1 + math.Sin(theta)}
	}
	return pts
}

// BenchmarkGrahamScan measures the hull on uniform points (expected
// hull size O(log n)) and on cocircular points (hull size n, the
// worst case: every point survives the sort and lands on the stack).
func BenchmarkGrahamScan(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("uniform/n="+itoa(n), func(b *testing.B) {
			pts := benchPoints(n)
			for b.Loop() {
				_ = NewGrahamScan(pts)
			}
		})
	}
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("cocircular/n="+itoa(n), func(b *testing.B) {
			pts := benchCircle(n)
			for b.Loop() {
				_ = NewGrahamScan(pts)
			}
		})
	}
}

// BenchmarkClosestPair is the divide-and-conquer at three sizes; the
// brute-force baseline is kept to n=2,000 (2M pairs) so it finishes
// in benchmark time — the gap is the whole point of the algorithm.
func BenchmarkClosestPair(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("n="+itoa(n), func(b *testing.B) {
			pts := benchPoints(n)
			for b.Loop() {
				_ = NewClosestPair(pts)
			}
		})
	}
	pts := benchPoints(2_000)
	b.Run("brute/n=2000", func(b *testing.B) {
		for b.Loop() {
			_, _ = bruteClosest(pts)
		}
	})
}

// BenchmarkFarthestPair measures hull + calipers on uniform points
// (tiny hull) and cocircular points (hull = n, the calipers worst
// case with an antipodal tie at nearly every step for even n).
func BenchmarkFarthestPair(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("uniform/n="+itoa(n), func(b *testing.B) {
			pts := benchPoints(n)
			for b.Loop() {
				_ = NewFarthestPair(pts)
			}
		})
	}
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("cocircular/n="+itoa(n), func(b *testing.B) {
			pts := benchCircle(n)
			for b.Loop() {
				_ = NewFarthestPair(pts)
			}
		})
	}
}

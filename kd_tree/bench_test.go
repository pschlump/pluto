/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// benchPoints builds n uniformly random points in the unit square with
// a fixed seed, so benchmark numbers are comparable across runs.
func benchPoints(n int) []g_lib.Point2D {
	rng := rand.New(rand.NewPCG(42, 7))
	pts := make([]g_lib.Point2D, n)
	for i := range pts {
		pts[i] = g_lib.Point2D{X: rng.Float64(), Y: rng.Float64()}
	}
	return pts
}

// benchQueries builds a fixed pool of query points disjoint from the
// data (offset out of the square so answers are not trivially the
// query itself).
func benchQueries(n int) []g_lib.Point2D {
	rng := rand.New(rand.NewPCG(7, 42))
	qs := make([]g_lib.Point2D, n)
	for i := range qs {
		qs[i] = g_lib.Point2D{X: rng.Float64()*2 - 0.5, Y: rng.Float64()*2 - 0.5}
	}
	return qs
}

func BenchmarkKdBuild(b *testing.B) {
	pts := benchPoints(100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		tr := NewKdTree()
		for _, p := range pts {
			tr.Insert(p)
		}
	}
}

func benchTree(b *testing.B, n int) *KdTree {
	b.Helper()
	tr := NewKdTree()
	for _, p := range benchPoints(n) {
		tr.Insert(p)
	}
	return tr
}

func BenchmarkKdContains(b *testing.B) {
	tr := benchTree(b, 100_000)
	qs := benchQueries(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		tr.Contains(qs[i%len(qs)])
	}
}

func BenchmarkKdNearest(b *testing.B) {
	tr := benchTree(b, 100_000)
	qs := benchQueries(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		tr.Nearest(qs[i%len(qs)])
	}
}

func BenchmarkKdNearestK(b *testing.B) {
	tr := benchTree(b, 100_000)
	qs := benchQueries(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		tr.NearestK(qs[i%len(qs)], 10)
	}
}

func BenchmarkKdRange(b *testing.B) {
	tr := benchTree(b, 100_000)
	qs := benchQueries(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		q := qs[i%len(qs)]
		tr.Range(g_lib.RectHV{Xmin: q.X - 0.01, Ymin: q.Y - 0.01, Xmax: q.X + 0.01, Ymax: q.Y + 0.01})
	}
}

func benchSet(b *testing.B, n int) *PointSet {
	b.Helper()
	ps := NewPointSet()
	for _, p := range benchPoints(n) {
		ps.Insert(p)
	}
	return ps
}

// The brute-force comparators at the same n — the whole reason the
// tree exists; the README table reports the tree-vs-set ratio from
// these.
func BenchmarkPointSetNearest(b *testing.B) {
	ps := benchSet(b, 100_000)
	qs := benchQueries(1000)
	b.ResetTimer()
	for i := range b.N {
		ps.Nearest(qs[i%len(qs)])
	}
}

func BenchmarkPointSetNearestK(b *testing.B) {
	ps := benchSet(b, 100_000)
	qs := benchQueries(1000)
	b.ResetTimer()
	for i := range b.N {
		ps.NearestK(qs[i%len(qs)], 10)
	}
}

func BenchmarkPointSetRange(b *testing.B) {
	ps := benchSet(b, 100_000)
	qs := benchQueries(1000)
	b.ResetTimer()
	for i := range b.N {
		q := qs[i%len(qs)]
		ps.Range(g_lib.RectHV{Xmin: q.X - 0.01, Ymin: q.Y - 0.01, Xmax: q.X + 0.01, Ymax: q.Y + 0.01})
	}
}

package graph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"strings"
	"testing"
)

func TestBFSPathsKnown(t *testing.T) {
	g := newTinyG()
	p := NewBFSPaths(g, 0)

	// Shortest distances from 0 in the tinyG graph.
	expectDist := []int{0, 1, 1, 2, 2, 1}
	for v, expect := range expectDist {
		if !p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be true", v)
		}
		if d, ok := p.DistTo(v); !ok || d != expect {
			t.Errorf("DistTo(%d) error, expected %d got %d (ok=%v)", v, expect, d, ok)
		}
	}
	// BFS finds shortest paths, source first.  With insertion-order
	// adjacency: 0-2-4 (not the DFS path 0-5-3-2-4) and 0-5-3.
	if got, ok := p.PathTo(4); !ok || !reflect.DeepEqual(got, []int{0, 2, 4}) {
		t.Errorf("PathTo(4) error, expected [0 2 4] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(3); !ok || !reflect.DeepEqual(got, []int{0, 5, 3}) {
		t.Errorf("PathTo(3) error, expected [0 5 3] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(0); !ok || !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("PathTo(0) error, expected [0] got %v (ok=%v)", got, ok)
	}
}

// TestBFSPathsGrid checks BFS distances on an unweighted 3x3 grid from a
// corner: dist((0,0), (r,c)) = r+c.
func TestBFSPathsGrid(t *testing.T) {
	const side = 3
	at := func(r, c int) int { return r*side + c }

	g := NewGraph(side * side)
	for r := 0; r < side; r++ {
		for c := 0; c < side; c++ {
			if r+1 < side {
				g.AddEdge(at(r, c), at(r+1, c))
			}
			if c+1 < side {
				g.AddEdge(at(r, c), at(r, c+1))
			}
		}
	}

	p := NewBFSPaths(g, at(0, 0))
	for r := 0; r < side; r++ {
		for c := 0; c < side; c++ {
			v := at(r, c)
			d, ok := p.DistTo(v)
			if !ok || d != r+c {
				t.Errorf("DistTo(%d) error, expected %d got %d (ok=%v)", v, r+c, d, ok)
			}
			path, ok := p.PathTo(v)
			if !ok {
				t.Fatalf("Expected a path to %d", v)
			}
			if len(path) != d+1 || path[0] != 0 || path[len(path)-1] != v {
				t.Errorf("PathTo(%d) error, expected length %d from 0 to %d, got %v", v, d+1, v, path)
			}
			for i := 0; i+1 < len(path); i++ {
				if !g.HasEdge(path[i], path[i+1]) {
					t.Errorf("PathTo(%d) = %v is not a walk: no edge %d-%d", v, path, path[i], path[i+1])
				}
			}
		}
	}
}

func TestBFSPathsDisconnected(t *testing.T) {
	g := NewGraph(4)
	g.AddEdge(0, 1)
	g.AddEdge(2, 3)

	p := NewBFSPaths(g, 0)
	for _, v := range []int{2, 3} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (disconnected)", v)
		}
		if path, ok := p.PathTo(v); ok || path != nil {
			t.Errorf("Expected no path to %d, got %v (ok=%v)", v, path, ok)
		}
		if d, ok := p.DistTo(v); ok || d != 0 {
			t.Errorf("Expected DistTo(%d) to report no path, got %d (ok=%v)", v, d, ok)
		}
	}
}

func TestBFSPathsPanics(t *testing.T) {
	g := NewGraph(3)
	var nilGraph *Graph
	var zeroGraph Graph

	expectPanic(t, "NewBFSPaths(nil)", func() { NewBFSPaths(nilGraph, 0) })
	expectPanic(t, "NewBFSPaths(zero)", func() { NewBFSPaths(&zeroGraph, 0) })
	expectPanic(t, "NewBFSPaths(s=-1)", func() { NewBFSPaths(g, -1) })
	expectPanic(t, "NewBFSPaths(s=3)", func() { NewBFSPaths(g, 3) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewBFSPaths to panic on out-of-range source.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBFSPaths") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewBFSPaths(g, 3)
	}()
}

func TestBFSPathsNilTolerated(t *testing.T) {
	var p *BFSPaths
	if p.HasPathTo(0) {
		t.Errorf("Expected false from HasPathTo on nil receiver.")
	}
	if path, ok := p.PathTo(0); ok || path != nil {
		t.Errorf("Expected no path from PathTo on nil receiver, got %v (ok=%v)", path, ok)
	}
	if _, ok := p.DistTo(0); ok {
		t.Errorf("Expected not-found from DistTo on nil receiver.")
	}
}

func TestBFSPathsOutOfRangeQuery(t *testing.T) {
	g := newTinyG()
	p := NewBFSPaths(g, 0)
	for _, v := range []int{-1, 6, 100} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (out of range)", v)
		}
		if _, ok := p.PathTo(v); ok {
			t.Errorf("Expected PathTo(%d) to report not-found (out of range)", v)
		}
		if _, ok := p.DistTo(v); ok {
			t.Errorf("Expected DistTo(%d) to report not-found (out of range)", v)
		}
	}
}

package digraph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"strings"
	"testing"
)

// TestBFSDirectedPathsKnown checks BFS on the algs4 tinyDG digraph from
// source 0: exactly {0,1,2,3,4,5} is reachable, with known distances.
func TestBFSDirectedPathsKnown(t *testing.T) {
	g := newTinyDG(t)
	p := NewBFSDirectedPaths(g, 0)

	// Shortest distances from 0 in tinyDG (verified against the in-test
	// reference BFS in thorough_test.go).
	expectDist := []int{0, 1, 3, 3, 2, 1}
	for v, expect := range expectDist {
		if !p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be true", v)
		}
		if d, ok := p.DistTo(v); !ok || d != expect {
			t.Errorf("DistTo(%d) error, expected %d got %d (ok=%v)", v, expect, d, ok)
		}
	}
	for v := 6; v < g.V(); v++ {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (unreachable from 0)", v)
		}
		if d, ok := p.DistTo(v); ok || d != 0 {
			t.Errorf("Expected DistTo(%d) to report no path, got %d (ok=%v)", v, d, ok)
		}
		if path, ok := p.PathTo(v); ok || path != nil {
			t.Errorf("Expected no path to %d, got %v (ok=%v)", v, path, ok)
		}
	}
	// BFS finds shortest directed paths, source first.  With
	// insertion-order adjacency: 0->5->4->2 and 0->5->4->3.
	if got, ok := p.PathTo(2); !ok || !reflect.DeepEqual(got, []int{0, 5, 4, 2}) {
		t.Errorf("PathTo(2) error, expected [0 5 4 2] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(3); !ok || !reflect.DeepEqual(got, []int{0, 5, 4, 3}) {
		t.Errorf("PathTo(3) error, expected [0 5 4 3] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(4); !ok || !reflect.DeepEqual(got, []int{0, 5, 4}) {
		t.Errorf("PathTo(4) error, expected [0 5 4] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(0); !ok || !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("PathTo(0) error, expected [0] got %v (ok=%v)", got, ok)
	}
}

// TestBFSDirectedPathsChain checks BFS distances on a directed chain
// 0->1->...->4 from the head.
func TestBFSDirectedPathsChain(t *testing.T) {
	g := NewDigraph(5)
	for v := 0; v+1 < 5; v++ {
		g.AddEdge(v, v+1)
	}

	p := NewBFSDirectedPaths(g, 0)
	for v := 0; v < 5; v++ {
		d, ok := p.DistTo(v)
		if !ok || d != v {
			t.Errorf("DistTo(%d) error, expected %d got %d (ok=%v)", v, v, d, ok)
		}
		path, ok := p.PathTo(v)
		if !ok {
			t.Fatalf("Expected a path to %d", v)
		}
		if len(path) != v+1 || path[0] != 0 || path[len(path)-1] != v {
			t.Errorf("PathTo(%d) error, got %v", v, path)
		}
		for i := 0; i+1 < len(path); i++ {
			if !g.HasEdge(path[i], path[i+1]) {
				t.Errorf("PathTo(%d) = %v is not a walk: no edge %d->%d", v, path, path[i], path[i+1])
			}
		}
	}
	// Backwards along the chain there is no path.
	q := NewBFSDirectedPaths(g, 4)
	for _, v := range []int{0, 1, 2, 3} {
		if q.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) from 4 to be false", v)
		}
	}
}

func TestBFSDirectedPathsSnapshot(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	p := NewBFSDirectedPaths(g, 0)
	// The result object is a snapshot: later edges are not reflected.
	g.AddEdge(1, 2)
	if p.HasPathTo(2) {
		t.Errorf("Expected snapshot semantics: HasPathTo(2) must not see the later AddEdge")
	}
}

func TestBFSDirectedPathsPanics(t *testing.T) {
	g := NewDigraph(3)
	var nilGraph *Digraph
	var zeroGraph Digraph

	expectPanic(t, "NewBFSDirectedPaths(nil)", func() { NewBFSDirectedPaths(nilGraph, 0) })
	expectPanic(t, "NewBFSDirectedPaths(zero)", func() { NewBFSDirectedPaths(&zeroGraph, 0) })
	expectPanic(t, "NewBFSDirectedPaths(s=-1)", func() { NewBFSDirectedPaths(g, -1) })
	expectPanic(t, "NewBFSDirectedPaths(s=3)", func() { NewBFSDirectedPaths(g, 3) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewBFSDirectedPaths to panic on out-of-range source.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBFSDirectedPaths") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewBFSDirectedPaths(g, 3)
	}()
}

func TestBFSDirectedPathsNilTolerated(t *testing.T) {
	var p *BFSDirectedPaths
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

func TestBFSDirectedPathsOutOfRangeQuery(t *testing.T) {
	g := newTinyDG(t)
	p := NewBFSDirectedPaths(g, 0)
	for _, v := range []int{-1, 13, 100} {
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

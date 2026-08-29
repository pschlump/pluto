package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// DirectedDFS (multi-source reachability)
// -------------------------------------------------------------------------------------------------------

// TestDirectedDFSKnown is the algs4 trace: on tinyDG the sources 1, 2, 6
// together reach every vertex except 7.
func TestDirectedDFSKnown(t *testing.T) {
	g := newTinyDG(t)
	d := NewDirectedDFS(g, 1, 2, 6)

	for v := 0; v < g.V(); v++ {
		if v == 7 {
			if d.Marked(7) {
				t.Errorf("Expected Marked(7) to be false (7 is not reachable from 1, 2, 6)")
			}
			continue
		}
		if !d.Marked(v) {
			t.Errorf("Expected Marked(%d) to be true", v)
		}
	}
	if d.Count() != 12 {
		t.Errorf("Expected Count 12, got %d", d.Count())
	}
}

func TestDirectedDFSSingleSource(t *testing.T) {
	g := newTinyDG(t)
	d := NewDirectedDFS(g, 0)

	// From 0, exactly {0,1,2,3,4,5} is reachable.
	for v := 0; v < g.V(); v++ {
		want := v <= 5
		if d.Marked(v) != want {
			t.Errorf("Marked(%d)=%v, expected %v", v, d.Marked(v), want)
		}
	}
	if d.Count() != 6 {
		t.Errorf("Expected Count 6, got %d", d.Count())
	}
}

func TestDirectedDFSDuplicateSources(t *testing.T) {
	g := newTinyDG(t)
	// Duplicate and singleton sources must not break the count.
	d := NewDirectedDFS(g, 0, 0, 5)
	if d.Count() != 6 {
		t.Errorf("Expected Count 6 for sources 0 0 5, got %d", d.Count())
	}
}

func TestDirectedDFSPanics(t *testing.T) {
	g := NewDigraph(3)
	var nilGraph *Digraph
	var zeroGraph Digraph

	expectPanic(t, "NewDirectedDFS(nil)", func() { NewDirectedDFS(nilGraph, 0) })
	expectPanic(t, "NewDirectedDFS(zero)", func() { NewDirectedDFS(&zeroGraph, 0) })
	expectPanic(t, "NewDirectedDFS(no sources)", func() { NewDirectedDFS(g) })
	expectPanic(t, "NewDirectedDFS(s=-1)", func() { NewDirectedDFS(g, 0, -1) })
	expectPanic(t, "NewDirectedDFS(s=3)", func() { NewDirectedDFS(g, 3) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewDirectedDFS to panic on out-of-range source.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDirectedDFS") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewDirectedDFS(g, 3)
	}()
}

func TestDirectedDFSNilTolerated(t *testing.T) {
	var d *DirectedDFS
	if d.Marked(0) {
		t.Errorf("Expected false from Marked on nil receiver.")
	}
	if d.Count() != 0 {
		t.Errorf("Expected Count 0 on nil receiver.")
	}
}

func TestDirectedDFSOutOfRangeQuery(t *testing.T) {
	g := newTinyDG(t)
	d := NewDirectedDFS(g, 0)
	for _, v := range []int{-1, 13, 100} {
		if d.Marked(v) {
			t.Errorf("Expected Marked(%d) to be false (out of range)", v)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// DFSDirectedPaths
// -------------------------------------------------------------------------------------------------------

// TestDFSDirectedPathsKnown traces DFS on a small digraph with
// insertion-order adjacency: 0->1, 0->2, 1->3, 2->3, 3->4.
func TestDFSDirectedPathsKnown(t *testing.T) {
	g := NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	p := NewDFSDirectedPaths(g, 0)
	for v := 0; v < g.V(); v++ {
		if !p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be true", v)
		}
	}
	// DFS from 0 dives 0->1->3->4 first.
	if got, ok := p.PathTo(4); !ok || !reflect.DeepEqual(got, []int{0, 1, 3, 4}) {
		t.Errorf("PathTo(4) error, expected [0 1 3 4] got %v (ok=%v)", got, ok)
	}
	if got, ok := p.PathTo(2); !ok || !reflect.DeepEqual(got, []int{0, 2}) {
		t.Errorf("PathTo(2) error, expected [0 2] got %v (ok=%v)", got, ok)
	}
	// The source's own path is just itself.
	if got, ok := p.PathTo(0); !ok || !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("PathTo(0) error, expected [0] got %v (ok=%v)", got, ok)
	}
}

// TestDFSDirectedPathsDirection verifies that paths respect edge
// direction: on 0->1->2, vertex 0 is not reachable from 2.
func TestDFSDirectedPathsDirection(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)

	p := NewDFSDirectedPaths(g, 2)
	if !p.HasPathTo(2) {
		t.Errorf("Expected HasPathTo(2) to be true (source)")
	}
	for _, v := range []int{0, 1} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (edges point away)", v)
		}
		if path, ok := p.PathTo(v); ok || path != nil {
			t.Errorf("Expected no path to %d, got %v (ok=%v)", v, path, ok)
		}
	}
}

func TestDFSDirectedPathsFreshSlice(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	p := NewDFSDirectedPaths(g, 0)

	first, _ := p.PathTo(2)
	first[0] = 99 // mutate the returned slice
	second, _ := p.PathTo(2)
	if !reflect.DeepEqual(second, []int{0, 1, 2}) {
		t.Errorf("Expected PathTo to return a fresh slice each call, got %v", second)
	}
}

func TestDFSDirectedPathsSnapshot(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	p := NewDFSDirectedPaths(g, 0)
	// The result object is a snapshot: later edges are not reflected.
	g.AddEdge(1, 2)
	if p.HasPathTo(2) {
		t.Errorf("Expected snapshot semantics: HasPathTo(2) must not see the later AddEdge")
	}
}

func TestDFSDirectedPathsPanics(t *testing.T) {
	g := NewDigraph(3)
	var nilGraph *Digraph
	var zeroGraph Digraph

	expectPanic(t, "NewDFSDirectedPaths(nil)", func() { NewDFSDirectedPaths(nilGraph, 0) })
	expectPanic(t, "NewDFSDirectedPaths(zero)", func() { NewDFSDirectedPaths(&zeroGraph, 0) })
	expectPanic(t, "NewDFSDirectedPaths(s=-1)", func() { NewDFSDirectedPaths(g, -1) })
	expectPanic(t, "NewDFSDirectedPaths(s=3)", func() { NewDFSDirectedPaths(g, 3) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewDFSDirectedPaths to panic on out-of-range source.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDFSDirectedPaths") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewDFSDirectedPaths(g, 3)
	}()
}

func TestDFSDirectedPathsNilTolerated(t *testing.T) {
	var p *DFSDirectedPaths
	if p.HasPathTo(0) {
		t.Errorf("Expected false from HasPathTo on nil receiver.")
	}
	if path, ok := p.PathTo(0); ok || path != nil {
		t.Errorf("Expected no path from PathTo on nil receiver, got %v (ok=%v)", path, ok)
	}
}

func TestDFSDirectedPathsOutOfRangeQuery(t *testing.T) {
	g := newTinyDG(t)
	p := NewDFSDirectedPaths(g, 0)
	for _, v := range []int{-1, 13, 100} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (out of range)", v)
		}
		if _, ok := p.PathTo(v); ok {
			t.Errorf("Expected PathTo(%d) to report not-found (out of range)", v)
		}
	}
}

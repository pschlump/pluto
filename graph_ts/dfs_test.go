package graph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"strings"
	"testing"
)

func TestDFSPathsKnown(t *testing.T) {
	g := newTinyG()
	p := NewDFSPaths(g, 0)

	// The tinyG graph is connected: every vertex is reachable from 0.
	for v := 0; v < g.V(); v++ {
		if !p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be true", v)
		}
	}
	// DFS from 0 with insertion-order adjacency: 0-5-3-2-4.
	if got, ok := p.PathTo(4); !ok || !reflect.DeepEqual(got, []int{0, 5, 3, 2, 4}) {
		t.Errorf("PathTo(4) error, expected [0 5 3 2 4] got %v (ok=%v)", got, ok)
	}
	// PathTo is source-first, and the source's own path is just itself.
	if got, ok := p.PathTo(0); !ok || !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("PathTo(0) error, expected [0] got %v (ok=%v)", got, ok)
	}
}

func TestDFSPathsDisconnected(t *testing.T) {
	g := NewGraph(4)
	g.AddEdge(0, 1)
	g.AddEdge(2, 3)

	p := NewDFSPaths(g, 0)
	if !p.HasPathTo(1) {
		t.Errorf("Expected HasPathTo(1) to be true")
	}
	for _, v := range []int{2, 3} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (disconnected)", v)
		}
		if path, ok := p.PathTo(v); ok || path != nil {
			t.Errorf("Expected no path to %d, got %v (ok=%v)", v, path, ok)
		}
	}
	if got, ok := p.PathTo(1); !ok || !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("PathTo(1) error, expected [0 1] got %v (ok=%v)", got, ok)
	}
}

func TestDFSPathsFreshSlice(t *testing.T) {
	g := newTinyG()
	p := NewDFSPaths(g, 0)

	first, _ := p.PathTo(4)
	first[0] = 99 // mutate the returned slice
	second, _ := p.PathTo(4)
	if !reflect.DeepEqual(second, []int{0, 5, 3, 2, 4}) {
		t.Errorf("Expected PathTo to return a fresh slice each call, got %v", second)
	}
}

func TestDFSPathsSnapshot(t *testing.T) {
	g := NewGraph(3)
	g.AddEdge(0, 1)
	p := NewDFSPaths(g, 0)
	// The result object is a snapshot: later edges are not reflected.
	g.AddEdge(1, 2)
	if p.HasPathTo(2) {
		t.Errorf("Expected snapshot semantics: HasPathTo(2) must not see the later AddEdge")
	}
}

func TestDFSPathsPanics(t *testing.T) {
	g := NewGraph(3)
	var nilGraph *Graph
	var zeroGraph Graph

	expectPanic(t, "NewDFSPaths(nil)", func() { NewDFSPaths(nilGraph, 0) })
	expectPanic(t, "NewDFSPaths(zero)", func() { NewDFSPaths(&zeroGraph, 0) })
	expectPanic(t, "NewDFSPaths(s=-1)", func() { NewDFSPaths(g, -1) })
	expectPanic(t, "NewDFSPaths(s=3)", func() { NewDFSPaths(g, 3) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewDFSPaths to panic on out-of-range source.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDFSPaths") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewDFSPaths(g, 3)
	}()
}

func TestDFSPathsNilTolerated(t *testing.T) {
	var p *DFSPaths
	if p.HasPathTo(0) {
		t.Errorf("Expected false from HasPathTo on nil receiver.")
	}
	if path, ok := p.PathTo(0); ok || path != nil {
		t.Errorf("Expected no path from PathTo on nil receiver, got %v (ok=%v)", path, ok)
	}
}

func TestDFSPathsOutOfRangeQuery(t *testing.T) {
	g := newTinyG()
	p := NewDFSPaths(g, 0)
	for _, v := range []int{-1, 6, 100} {
		if p.HasPathTo(v) {
			t.Errorf("Expected HasPathTo(%d) to be false (out of range)", v)
		}
		if _, ok := p.PathTo(v); ok {
			t.Errorf("Expected PathTo(%d) to report not-found (out of range)", v)
		}
	}
}

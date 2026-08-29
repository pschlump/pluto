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
// Constructor
// -------------------------------------------------------------------------------------------------------

func TestDigraphNewDigraph(t *testing.T) {
	g := NewDigraph(5)
	if g == nil {
		t.Fatalf("NewDigraph returned nil.")
	}
	if g.V() != 5 || g.Len() != 5 {
		t.Errorf("Expected V/Len 5, got %d/%d", g.V(), g.Len())
	}
	if g.E() != 0 {
		t.Errorf("Expected E 0, got %d", g.E())
	}
	for v := 0; v < 5; v++ {
		if d, ok := g.OutDegree(v); !ok || d != 0 {
			t.Errorf("Expected out-degree 0 for %d, got %d (ok=%v)", v, d, ok)
		}
		if d, ok := g.InDegree(v); !ok || d != 0 {
			t.Errorf("Expected in-degree 0 for %d, got %d (ok=%v)", v, d, ok)
		}
		if got := adjOf(g, v); len(got) != 0 {
			t.Errorf("Expected no out-neighbors of %d, got %v", v, got)
		}
	}
	// The constructed digraph must be usable.
	if !g.AddEdge(0, 4) {
		t.Errorf("Expected AddEdge on constructed digraph to return true.")
	}
	if g.E() != 1 {
		t.Errorf("Expected E 1 after AddEdge, got %d", g.E())
	}
}

func TestDigraphNewDigraphPanics(t *testing.T) {
	expectPanic(t, "NewDigraph(0)", func() { NewDigraph(0) })
	expectPanic(t, "NewDigraph(-3)", func() { NewDigraph(-3) })

	// Verify the panic message names the constructor.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewDigraph(0) to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewDigraph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewDigraph(0)
	}()
}

// -------------------------------------------------------------------------------------------------------
// AddEdge / Adj / OutDegree / InDegree / HasEdge
// -------------------------------------------------------------------------------------------------------

// TestDigraphTinyDG checks counts, adjacency (insertion order), and both
// degrees against the algs4 tinyDG reference digraph (13 vertices, 22
// edges).
func TestDigraphTinyDG(t *testing.T) {
	g := newTinyDG(t)

	if g.V() != 13 || g.E() != 22 {
		t.Fatalf("Expected V=13 E=22, got V=%d E=%d", g.V(), g.E())
	}
	// Adjacency in insertion order, from the tinyDG.txt edge listing.
	expectAdj := [][]int{
		{1, 5},
		nil,
		{3, 0},
		{2, 5},
		{2, 3},
		{4},
		{0, 8, 4, 9},
		{9, 6},
		{6},
		{10, 11},
		{12},
		{12, 4},
		{9},
	}
	expectIndeg := []int{2, 1, 2, 2, 3, 2, 2, 0, 1, 3, 1, 1, 2}
	for v, expect := range expectAdj {
		if got := adjOf(g, v); !reflect.DeepEqual(got, expect) {
			t.Errorf("Adj(%d) error, expected %v got %v", v, expect, got)
		}
		if d, ok := g.OutDegree(v); !ok || d != len(expect) {
			t.Errorf("OutDegree(%d) error, expected %d got %d (ok=%v)", v, len(expect), d, ok)
		}
		if d, ok := g.InDegree(v); !ok || d != expectIndeg[v] {
			t.Errorf("InDegree(%d) error, expected %d got %d (ok=%v)", v, expectIndeg[v], d, ok)
		}
	}
	// Every listed neighbor is an edge, in the one direction.
	for v, ws := range expectAdj {
		for _, w := range ws {
			if !g.HasEdge(v, w) {
				t.Errorf("Expected HasEdge(%d, %d)", v, w)
			}
		}
	}
	// Absent edges report false (note 2->1 is absent even though the
	// reverse edge would not imply it either).
	for _, e := range [][2]int{{0, 3}, {1, 0}, {7, 5}, {12, 11}} {
		if g.HasEdge(e[0], e[1]) {
			t.Errorf("Expected HasEdge(%d, %d) to be false", e[0], e[1])
		}
	}
	// Direction matters: 4->2 exists, 2->4 does not.
	if !g.HasEdge(4, 2) || g.HasEdge(2, 4) {
		t.Errorf("Expected HasEdge(4, 2) and not HasEdge(2, 4)")
	}
}

func TestDigraphAddEdgeOutOfRange(t *testing.T) {
	g := NewDigraph(3)
	for _, e := range [][2]int{{-1, 0}, {0, -1}, {3, 0}, {0, 3}, {-5, 99}} {
		if g.AddEdge(e[0], e[1]) {
			t.Errorf("Expected AddEdge(%d, %d) to return false", e[0], e[1])
		}
	}
	if g.E() != 0 {
		t.Errorf("Expected E 0 after rejected edges, got %d", g.E())
	}
	if _, ok := g.OutDegree(3); ok {
		t.Errorf("Expected OutDegree(3) to report out of range")
	}
	if _, ok := g.InDegree(-1); ok {
		t.Errorf("Expected InDegree(-1) to report out of range")
	}
	if g.HasEdge(0, 3) || g.HasEdge(3, 0) || g.HasEdge(-1, 0) {
		t.Errorf("Expected HasEdge with out-of-range vertex to be false")
	}
}

func TestDigraphSelfLoop(t *testing.T) {
	g := NewDigraph(3)
	if !g.AddEdge(2, 2) {
		t.Fatalf("Expected AddEdge(2, 2) self-loop to return true")
	}
	// A self-loop counts once in E, once in the out-degree, and once in
	// the in-degree.
	if g.E() != 1 {
		t.Errorf("Expected E 1 for one self-loop, got %d", g.E())
	}
	if d, _ := g.OutDegree(2); d != 1 {
		t.Errorf("Expected out-degree 1 for a self-loop, got %d", d)
	}
	if d, _ := g.InDegree(2); d != 1 {
		t.Errorf("Expected in-degree 1 for a self-loop, got %d", d)
	}
	if !g.HasEdge(2, 2) {
		t.Errorf("Expected HasEdge(2, 2) to be true")
	}
	if got := adjOf(g, 2); !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("Expected Adj(2) = [2], got %v", got)
	}
}

func TestDigraphParallelEdges(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(1, 2)
	g.AddEdge(1, 2)
	if g.E() != 2 {
		t.Errorf("Expected E 2 for two parallel edges, got %d", g.E())
	}
	if d, _ := g.OutDegree(1); d != 2 {
		t.Errorf("Expected out-degree 2, got %d", d)
	}
	if d, _ := g.InDegree(2); d != 2 {
		t.Errorf("Expected in-degree 2, got %d", d)
	}
	if got := adjOf(g, 1); !reflect.DeepEqual(got, []int{2, 2}) {
		t.Errorf("Expected Adj(1) = [2 2], got %v", got)
	}
}

func TestDigraphAdjEarlyStop(t *testing.T) {
	g := newTinyDG(t)
	count := 0
	for range g.Adj(6) {
		count++
		break // stop after the first neighbor
	}
	if count != 1 {
		t.Errorf("Expected early stop after 1 neighbor, got %d", count)
	}
	// The iterator must be reusable after an early stop.
	if got := adjOf(g, 6); len(got) != 4 {
		t.Errorf("Expected 4 neighbors on re-iteration, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Reverse
// -------------------------------------------------------------------------------------------------------

func TestDigraphReverse(t *testing.T) {
	g := newTinyDG(t)
	r := g.Reverse()

	if r.V() != 13 || r.E() != 22 {
		t.Fatalf("Expected reversed V=13 E=22, got V=%d E=%d", r.V(), r.E())
	}
	// Every edge v->w of g is the edge w->v of r, and degrees swap.
	expectAdj := [][]int{
		{1, 5},
		nil,
		{3, 0},
		{2, 5},
		{2, 3},
		{4},
		{0, 8, 4, 9},
		{9, 6},
		{6},
		{10, 11},
		{12},
		{12, 4},
		{9},
	}
	for v, ws := range expectAdj {
		for _, w := range ws {
			if !r.HasEdge(w, v) {
				t.Errorf("Expected reversed edge HasEdge(%d, %d)", w, v)
			}
		}
		if din, _ := r.InDegree(v); din != len(ws) {
			t.Errorf("Reverse InDegree(%d)=%d, expected original out-degree %d", v, din, len(ws))
		}
		if dout, _ := g.OutDegree(v); dout != len(ws) {
			t.Fatalf("OutDegree(%d)=%d, expected %d", v, dout, len(ws))
		}
	}
	// In-degree of the original is the out-degree of the reverse.
	expectIndeg := []int{2, 1, 2, 2, 3, 2, 2, 0, 1, 3, 1, 1, 2}
	for v, expect := range expectIndeg {
		if d, _ := r.OutDegree(v); d != expect {
			t.Errorf("Reverse OutDegree(%d)=%d, expected original in-degree %d", v, d, expect)
		}
	}
	// The original digraph is untouched.
	if g.E() != 22 || !g.HasEdge(4, 2) {
		t.Errorf("Reverse must not modify the original digraph")
	}
}

func TestDigraphReverseNil(t *testing.T) {
	var nilGraph *Digraph
	if nilGraph.Reverse() != nil {
		t.Errorf("Expected Reverse on a nil digraph to return nil.")
	}
	var zero Digraph
	r := zero.Reverse()
	if r == nil || r.V() != 0 || r.E() != 0 {
		t.Errorf("Expected Reverse on a zero-value digraph to return an empty digraph, got %+v", r)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value digraphs
// -------------------------------------------------------------------------------------------------------

// TestDigraphNilPanics verifies the documented panic when AddEdge is
// called on a nil digraph — the one structural operation with no sane
// answer.
func TestDigraphNilPanics(t *testing.T) {
	var nilGraph *Digraph
	expectPanic(t, "AddEdge", func() { nilGraph.AddEdge(0, 1) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected AddEdge to panic on nil digraph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "AddEdge") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilGraph.AddEdge(0, 1)
	}()
}

// TestDigraphNilTolerated verifies that every operation other than
// AddEdge treats a nil digraph as an empty digraph instead of panicking.
func TestDigraphNilTolerated(t *testing.T) {
	var nilGraph *Digraph

	if nilGraph.V() != 0 || nilGraph.Len() != 0 || nilGraph.E() != 0 {
		t.Errorf("Expected nil digraph to have V/Len/E of 0.")
	}
	if _, ok := nilGraph.OutDegree(0); ok {
		t.Errorf("Expected out-of-range from OutDegree on nil digraph.")
	}
	if _, ok := nilGraph.InDegree(0); ok {
		t.Errorf("Expected out-of-range from InDegree on nil digraph.")
	}
	if nilGraph.HasEdge(0, 0) {
		t.Errorf("Expected false from HasEdge on nil digraph.")
	}
	for range nilGraph.Adj(0) {
		t.Errorf("Expected no neighbors from Adj on nil digraph.")
	}
}

// TestDigraphZeroValue verifies that the zero value behaves as an empty
// digraph with no vertices: every vertex is out of range, so AddEdge
// reports false instead of panicking.
func TestDigraphZeroValue(t *testing.T) {
	var g Digraph
	if g.V() != 0 || g.Len() != 0 || g.E() != 0 {
		t.Errorf("Expected zero-value digraph to have V/Len/E of 0.")
	}
	if g.AddEdge(0, 0) {
		t.Errorf("Expected AddEdge on zero-value digraph to return false (all vertices out of range).")
	}
	if _, ok := g.OutDegree(0); ok {
		t.Errorf("Expected out-of-range from OutDegree on zero-value digraph.")
	}
	if _, ok := g.InDegree(0); ok {
		t.Errorf("Expected out-of-range from InDegree on zero-value digraph.")
	}
	if g.HasEdge(0, 0) {
		t.Errorf("Expected false from HasEdge on zero-value digraph.")
	}
	for range g.Adj(0) {
		t.Errorf("Expected no neighbors from Adj on zero-value digraph.")
	}
}

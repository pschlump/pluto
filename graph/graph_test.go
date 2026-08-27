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

// -------------------------------------------------------------------------------------------------------
// Constructor
// -------------------------------------------------------------------------------------------------------

func TestGraphNewGraph(t *testing.T) {
	g := NewGraph(5)
	if g == nil {
		t.Fatalf("NewGraph returned nil.")
	}
	if g.V() != 5 || g.Len() != 5 {
		t.Errorf("Expected V/Len 5, got %d/%d", g.V(), g.Len())
	}
	if g.E() != 0 {
		t.Errorf("Expected E 0, got %d", g.E())
	}
	for v := 0; v < 5; v++ {
		if d, ok := g.Degree(v); !ok || d != 0 {
			t.Errorf("Expected degree 0 for %d, got %d (ok=%v)", v, d, ok)
		}
		if got := adjOf(g, v); len(got) != 0 {
			t.Errorf("Expected no neighbors of %d, got %v", v, got)
		}
	}
	// The constructed graph must be usable.
	if !g.AddEdge(0, 4) {
		t.Errorf("Expected AddEdge on constructed graph to return true.")
	}
	if g.E() != 1 {
		t.Errorf("Expected E 1 after AddEdge, got %d", g.E())
	}
}

func TestGraphNewGraphPanics(t *testing.T) {
	expectPanic(t, "NewGraph(0)", func() { NewGraph(0) })
	expectPanic(t, "NewGraph(-3)", func() { NewGraph(-3) })

	// Verify the panic message names the constructor.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewGraph(0) to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewGraph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewGraph(0)
	}()
}

// -------------------------------------------------------------------------------------------------------
// AddEdge / Adj / Degree / HasEdge
// -------------------------------------------------------------------------------------------------------

func TestGraphAddEdge(t *testing.T) {
	g := newTinyG() // the 6-vertex, 8-edge graph of Sedgwick's §4.1 trace

	if g.V() != 6 || g.E() != 8 {
		t.Fatalf("Expected V=6 E=8, got V=%d E=%d", g.V(), g.E())
	}
	// Adj yields in insertion order.
	expectAdj := [][]int{
		{5, 1, 2},
		{2, 0},
		{4, 3, 1, 0},
		{2, 4, 5},
		{2, 3},
		{0, 3},
	}
	for v, expect := range expectAdj {
		if got := adjOf(g, v); !reflect.DeepEqual(got, expect) {
			t.Errorf("Adj(%d) error, expected %v got %v", v, expect, got)
		}
		if d, ok := g.Degree(v); !ok || d != len(expect) {
			t.Errorf("Degree(%d) error, expected %d got %d (ok=%v)", v, len(expect), d, ok)
		}
	}
	// Every listed neighbor is an edge, in both directions.
	for v, ws := range expectAdj {
		for _, w := range ws {
			if !g.HasEdge(v, w) || !g.HasEdge(w, v) {
				t.Errorf("Expected HasEdge(%d, %d) both ways", v, w)
			}
		}
	}
	// Absent edges report false.
	for _, e := range [][2]int{{0, 3}, {0, 4}, {1, 5}, {4, 5}} {
		if g.HasEdge(e[0], e[1]) {
			t.Errorf("Expected HasEdge(%d, %d) to be false", e[0], e[1])
		}
	}
}

func TestGraphAddEdgeOutOfRange(t *testing.T) {
	g := NewGraph(3)
	for _, e := range [][2]int{{-1, 0}, {0, -1}, {3, 0}, {0, 3}, {-5, 99}} {
		if g.AddEdge(e[0], e[1]) {
			t.Errorf("Expected AddEdge(%d, %d) to return false", e[0], e[1])
		}
	}
	if g.E() != 0 {
		t.Errorf("Expected E 0 after rejected edges, got %d", g.E())
	}
	if _, ok := g.Degree(3); ok {
		t.Errorf("Expected Degree(3) to report out of range")
	}
	if _, ok := g.Degree(-1); ok {
		t.Errorf("Expected Degree(-1) to report out of range")
	}
	if g.HasEdge(0, 3) || g.HasEdge(3, 0) || g.HasEdge(-1, 0) {
		t.Errorf("Expected HasEdge with out-of-range vertex to be false")
	}
}

func TestGraphSelfLoop(t *testing.T) {
	g := NewGraph(3)
	if !g.AddEdge(2, 2) {
		t.Fatalf("Expected AddEdge(2, 2) self-loop to return true")
	}
	// A self-loop counts once in E and twice in the degree (like Sedgwick).
	if g.E() != 1 {
		t.Errorf("Expected E 1 for one self-loop, got %d", g.E())
	}
	if d, _ := g.Degree(2); d != 2 {
		t.Errorf("Expected degree 2 for a self-loop, got %d", d)
	}
	if !g.HasEdge(2, 2) {
		t.Errorf("Expected HasEdge(2, 2) to be true")
	}
	if got := adjOf(g, 2); !reflect.DeepEqual(got, []int{2, 2}) {
		t.Errorf("Expected Adj(2) = [2 2], got %v", got)
	}
}

func TestGraphParallelEdges(t *testing.T) {
	g := NewGraph(3)
	g.AddEdge(1, 2)
	g.AddEdge(1, 2)
	if g.E() != 2 {
		t.Errorf("Expected E 2 for two parallel edges, got %d", g.E())
	}
	if d, _ := g.Degree(1); d != 2 {
		t.Errorf("Expected degree 2, got %d", d)
	}
	if got := adjOf(g, 2); !reflect.DeepEqual(got, []int{1, 1}) {
		t.Errorf("Expected Adj(2) = [1 1], got %v", got)
	}
}

func TestGraphAdjEarlyStop(t *testing.T) {
	g := newTinyG()
	count := 0
	for range g.Adj(0) {
		count++
		break // stop after the first neighbor
	}
	if count != 1 {
		t.Errorf("Expected early stop after 1 neighbor, got %d", count)
	}
	// The iterator must be reusable after an early stop.
	if got := adjOf(g, 0); len(got) != 3 {
		t.Errorf("Expected 3 neighbors on re-iteration, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value graphs
// -------------------------------------------------------------------------------------------------------

// TestGraphNilPanics verifies the documented panic when AddEdge is called
// on a nil graph — the one structural operation with no sane answer.
func TestGraphNilPanics(t *testing.T) {
	var nilGraph *Graph
	expectPanic(t, "AddEdge", func() { nilGraph.AddEdge(0, 1) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected AddEdge to panic on nil graph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "AddEdge") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilGraph.AddEdge(0, 1)
	}()
}

// TestGraphNilTolerated verifies that every operation other than AddEdge
// treats a nil graph as an empty graph instead of panicking.
func TestGraphNilTolerated(t *testing.T) {
	var nilGraph *Graph

	if nilGraph.V() != 0 || nilGraph.Len() != 0 || nilGraph.E() != 0 {
		t.Errorf("Expected nil graph to have V/Len/E of 0.")
	}
	if _, ok := nilGraph.Degree(0); ok {
		t.Errorf("Expected out-of-range from Degree on nil graph.")
	}
	if nilGraph.HasEdge(0, 0) {
		t.Errorf("Expected false from HasEdge on nil graph.")
	}
	for range nilGraph.Adj(0) {
		t.Errorf("Expected no neighbors from Adj on nil graph.")
	}
}

// TestGraphZeroValue verifies that the zero value behaves as an empty
// graph with no vertices: every vertex is out of range, so AddEdge reports
// false instead of panicking.
func TestGraphZeroValue(t *testing.T) {
	var g Graph
	if g.V() != 0 || g.Len() != 0 || g.E() != 0 {
		t.Errorf("Expected zero-value graph to have V/Len/E of 0.")
	}
	if g.AddEdge(0, 0) {
		t.Errorf("Expected AddEdge on zero-value graph to return false (all vertices out of range).")
	}
	if _, ok := g.Degree(0); ok {
		t.Errorf("Expected out-of-range from Degree on zero-value graph.")
	}
	if g.HasEdge(0, 0) {
		t.Errorf("Expected false from HasEdge on zero-value graph.")
	}
	for range g.Adj(0) {
		t.Errorf("Expected no neighbors from Adj on zero-value graph.")
	}
}

package graph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import "testing"

func TestCCKnown(t *testing.T) {
	// Four components: {0,1,2}, {3,4}, {5}, {6,7}.
	g := NewGraph(8)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {3, 4}, {6, 7}} {
		g.AddEdge(e[0], e[1])
	}

	c := NewCC(g)
	if c.Count() != 4 {
		t.Fatalf("Expected 4 components, got %d", c.Count())
	}
	expectID := []int{0, 0, 0, 1, 1, 2, 3, 3}
	for v, expect := range expectID {
		if id, ok := c.ID(v); !ok || id != expect {
			t.Errorf("ID(%d) error, expected %d got %d (ok=%v)", v, expect, id, ok)
		}
	}
	for _, pair := range [][2]int{{0, 1}, {1, 2}, {3, 4}, {6, 7}} {
		if !c.Connected(pair[0], pair[1]) {
			t.Errorf("Expected Connected(%d, %d) to be true", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]int{{0, 3}, {0, 5}, {1, 6}, {3, 7}, {5, 6}} {
		if c.Connected(pair[0], pair[1]) {
			t.Errorf("Expected Connected(%d, %d) to be false", pair[0], pair[1])
		}
	}
	// A vertex is always connected to itself.
	for v := 0; v < g.V(); v++ {
		if !c.Connected(v, v) {
			t.Errorf("Expected Connected(%d, %d) to be true", v, v)
		}
	}
}

func TestCCSingleComponent(t *testing.T) {
	g := newTinyG()
	c := NewCC(g)
	if c.Count() != 1 {
		t.Errorf("Expected 1 component for the connected tinyG graph, got %d", c.Count())
	}
	for v := 0; v < g.V(); v++ {
		for w := 0; w < g.V(); w++ {
			if !c.Connected(v, w) {
				t.Errorf("Expected Connected(%d, %d) to be true", v, w)
			}
		}
	}
}

func TestCCNoEdges(t *testing.T) {
	g := NewGraph(5) // 5 isolated vertices = 5 components
	c := NewCC(g)
	if c.Count() != 5 {
		t.Errorf("Expected 5 components for an edgeless graph, got %d", c.Count())
	}
	for v := 0; v < 5; v++ {
		if id, ok := c.ID(v); !ok || id != v {
			t.Errorf("ID(%d) error, expected %d got %d (ok=%v)", v, v, id, ok)
		}
		if !c.Connected(v, v) {
			t.Errorf("Expected Connected(%d, %d) to be true", v, v)
		}
	}
	if c.Connected(0, 1) {
		t.Errorf("Expected Connected(0, 1) to be false")
	}
}

func TestCCSelfLoop(t *testing.T) {
	g := NewGraph(2)
	g.AddEdge(0, 0) // a self-loop does not merge components
	c := NewCC(g)
	if c.Count() != 2 {
		t.Errorf("Expected 2 components, got %d", c.Count())
	}
	if c.Connected(0, 1) {
		t.Errorf("Expected Connected(0, 1) to be false")
	}
}

func TestCCSnapshot(t *testing.T) {
	g := NewGraph(2)
	c := NewCC(g)
	g.AddEdge(0, 1)
	if c.Connected(0, 1) || c.Count() != 2 {
		t.Errorf("Expected snapshot semantics: later AddEdge must not be reflected")
	}
}

func TestCCNilAndEmpty(t *testing.T) {
	// A nil or empty graph has a sane answer — no components — so NewCC
	// does not panic.
	var nilGraph *Graph
	var zeroGraph Graph
	for _, g := range []*Graph{nilGraph, &zeroGraph} {
		c := NewCC(g)
		if c.Count() != 0 {
			t.Errorf("Expected 0 components for nil/empty graph, got %d", c.Count())
		}
		if c.Connected(0, 0) {
			t.Errorf("Expected Connected to be false for nil/empty graph")
		}
		if _, ok := c.ID(0); ok {
			t.Errorf("Expected ID to report out of range for nil/empty graph")
		}
	}
}

func TestCCNilTolerated(t *testing.T) {
	var c *CC
	if c.Count() != 0 {
		t.Errorf("Expected Count 0 on nil receiver.")
	}
	if c.Connected(0, 0) {
		t.Errorf("Expected false from Connected on nil receiver.")
	}
	if _, ok := c.ID(0); ok {
		t.Errorf("Expected not-found from ID on nil receiver.")
	}
}

func TestCCOutOfRangeQuery(t *testing.T) {
	g := newTinyG()
	c := NewCC(g)
	for _, v := range []int{-1, 6, 100} {
		if _, ok := c.ID(v); ok {
			t.Errorf("Expected ID(%d) to report out of range", v)
		}
		if c.Connected(v, 0) || c.Connected(0, v) {
			t.Errorf("Expected Connected with out-of-range %d to be false", v)
		}
	}
}

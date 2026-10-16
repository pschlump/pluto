package graph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

// TestCycleNone verifies that acyclic graphs (edgeless, paths, trees)
// report no cycle.
func TestCycleNone(t *testing.T) {
	edgeless := NewGraph(6)
	if c := NewCycle(edgeless); c.HasCycle() || c.Cycle() != nil {
		t.Errorf("Expected no cycle in an edgeless graph, got %v", c.Cycle())
	}

	path := NewGraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}} {
		path.AddEdge(e[0], e[1])
	}
	if c := NewCycle(path); c.HasCycle() || c.Cycle() != nil {
		t.Errorf("Expected no cycle in a path, got %v", c.Cycle())
	}

	// A random tree: every v > 0 gets one edge to a lower vertex.
	tree := NewGraph(10)
	for v := 1; v < 10; v++ {
		tree.AddEdge(v, v/2)
	}
	if c := NewCycle(tree); c.HasCycle() || c.Cycle() != nil {
		t.Errorf("Expected no cycle in a tree, got %v", c.Cycle())
	}
}

// TestCycleSelfLoop verifies the 1-edge cycle [v v].
func TestCycleSelfLoop(t *testing.T) {
	g := NewGraph(4)
	g.AddEdge(1, 2)
	g.AddEdge(3, 3) // self-loop
	c := NewCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected the self-loop to be a cycle.")
	}
	if got := c.Cycle(); !reflect.DeepEqual(got, []int{3, 3}) {
		t.Errorf("Expected cycle [3 3], got %v", got)
	}
	checkCycleValid(t, g, c.Cycle())
}

// TestCycleParallel verifies the 2-edge cycle [v w v].
func TestCycleParallel(t *testing.T) {
	g := NewGraph(4)
	g.AddEdge(0, 1)
	g.AddEdge(2, 3)
	g.AddEdge(1, 2)
	g.AddEdge(2, 3) // parallel edge
	c := NewCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected the parallel edges to be a cycle.")
	}
	if got := c.Cycle(); !reflect.DeepEqual(got, []int{2, 3, 2}) {
		t.Errorf("Expected cycle [2 3 2], got %v", got)
	}
	checkCycleValid(t, g, c.Cycle())
}

// TestCycleTinyG verifies a DFS-found cycle on the tinyG trace graph:
// it is a valid closed walk over real edges, first vertex repeated at
// the end, and the returned slice is a fresh copy.
func TestCycleTinyG(t *testing.T) {
	g := newTinyG()
	c := NewCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected tinyG (E=8 > V-1=5, connected) to have a cycle.")
	}
	cyc := c.Cycle()
	checkCycleValid(t, g, cyc)
	// A DFS-found cycle on a simple subgraph is simple: distinct
	// interior vertices.
	seen := map[int]bool{}
	for _, v := range cyc[:len(cyc)-1] {
		if seen[v] {
			t.Fatalf("Cycle %v repeats interior vertex %d", cyc, v)
		}
		seen[v] = true
	}
	// The returned slice is fresh: mutating it does not affect the query.
	cyc[0] = -99
	if got := c.Cycle(); got[0] == -99 {
		t.Errorf("Cycle() returned the same backing slice twice.")
	}
}

// TestCycleTriangle verifies the plain odd cycle 0-1-2-0.
func TestCycleTriangle(t *testing.T) {
	g := NewGraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	g.AddEdge(2, 0)
	c := NewCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected the triangle to have a cycle.")
	}
	checkCycleValid(t, g, c.Cycle())
	if len(c.Cycle()) != 4 { // 3 vertices + repeated first
		t.Errorf("Expected a 3-edge cycle, got %v", c.Cycle())
	}
}

// TestCycleNilEmpty verifies the tolerant contract: a nil graph, a
// zero-value graph, and a nil query object all report no cycle.
func TestCycleNilEmpty(t *testing.T) {
	var nilGraph *Graph
	if c := NewCycle(nilGraph); c.HasCycle() || c.Cycle() != nil {
		t.Errorf("Expected no cycle on a nil graph.")
	}
	var zero Graph
	if c := NewCycle(&zero); c.HasCycle() || c.Cycle() != nil {
		t.Errorf("Expected no cycle on a zero-value graph.")
	}
	var nilCycle *Cycle
	if nilCycle.HasCycle() || nilCycle.Cycle() != nil {
		t.Errorf("Expected a nil *Cycle to report no cycle.")
	}
}

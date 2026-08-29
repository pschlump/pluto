package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

// TestDirectedCycleTinyDG: the algs4 tinyDG digraph has directed cycles;
// the found cycle must be a genuine one.
func TestDirectedCycleTinyDG(t *testing.T) {
	g := newTinyDG(t)
	c := NewDirectedCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected tinyDG to have a directed cycle")
	}
	checkCycleIsReal(t, g, c.Cycle())
}

// TestDirectedCycleTinyDAG: the algs4 tinyDAG digraph is acyclic.
func TestDirectedCycleTinyDAG(t *testing.T) {
	g := newTinyDAG(t)
	c := NewDirectedCycle(g)
	if c.HasCycle() {
		t.Errorf("Expected tinyDAG to have no directed cycle, got %v", c.Cycle())
	}
	if cyc := c.Cycle(); cyc != nil {
		t.Errorf("Expected nil Cycle on an acyclic digraph, got %v", cyc)
	}
}

// TestDirectedCycleKnown traces the search on a small digraph:
// 0->1, 1->2, 2->0, 2->3, 3->4.  DFS from 0 finds the cycle 0->1->2->0.
func TestDirectedCycleKnown(t *testing.T) {
	g := NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	c := NewDirectedCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected a directed cycle")
	}
	if got := c.Cycle(); !reflect.DeepEqual(got, []int{0, 1, 2, 0}) {
		t.Errorf("Cycle error, expected [0 1 2 0] got %v", got)
	}
}

func TestDirectedCycleSelfLoop(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 1) // a self-loop is a directed cycle

	c := NewDirectedCycle(g)
	if !c.HasCycle() {
		t.Fatalf("Expected a self-loop to be a directed cycle")
	}
	if got := c.Cycle(); !reflect.DeepEqual(got, []int{1, 1}) {
		t.Errorf("Cycle error, expected [1 1] got %v", got)
	}
}

func TestDirectedCycleFreshSlice(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 0)
	c := NewDirectedCycle(g)

	first := c.Cycle()
	first[0] = 99 // mutate the returned slice
	if got := c.Cycle(); !reflect.DeepEqual(got, []int{0, 1, 0}) {
		t.Errorf("Expected Cycle to return a fresh slice each call, got %v", got)
	}
}

// TestDirectedCycleNilAndEmpty: a nil or empty digraph has a sane answer
// — no cycle — so NewDirectedCycle does not panic.
func TestDirectedCycleNilAndEmpty(t *testing.T) {
	var nilGraph *Digraph
	var zeroGraph Digraph
	for _, g := range []*Digraph{nilGraph, &zeroGraph} {
		c := NewDirectedCycle(g)
		if c.HasCycle() {
			t.Errorf("Expected no cycle for nil/empty digraph")
		}
		if cyc := c.Cycle(); cyc != nil {
			t.Errorf("Expected nil Cycle for nil/empty digraph, got %v", cyc)
		}
	}
}

func TestDirectedCycleNilTolerated(t *testing.T) {
	var c *DirectedCycle
	if c.HasCycle() {
		t.Errorf("Expected false from HasCycle on nil receiver.")
	}
	if cyc := c.Cycle(); cyc != nil {
		t.Errorf("Expected nil from Cycle on nil receiver, got %v", cyc)
	}
}

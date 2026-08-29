package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

// TestTopologicalTinyDAG: the algs4 tinyDAG digraph is acyclic, so it has
// a topological order; validate the order property (for every edge v->w,
// v precedes w) rather than a hardcoded order.
func TestTopologicalTinyDAG(t *testing.T) {
	g := newTinyDAG(t)
	top := NewTopological(g)
	if !top.HasOrder() {
		t.Fatalf("Expected tinyDAG to have a topological order")
	}
	n, edges := parseAlgs4Digraph(t, tinyDAGData)
	model := make([][]int, n)
	for _, e := range edges {
		model[e[0]] = append(model[e[0]], e[1])
	}
	checkTopologicalOrder(t, top.Order(), model)
}

// TestTopologicalTinyDG: the algs4 tinyDG digraph has directed cycles, so
// it has no topological order.
func TestTopologicalTinyDG(t *testing.T) {
	g := newTinyDG(t)
	top := NewTopological(g)
	if top.HasOrder() {
		t.Errorf("Expected tinyDG to have no topological order, got %v", top.Order())
	}
	if order := top.Order(); order != nil {
		t.Errorf("Expected nil Order on a cyclic digraph, got %v", order)
	}
}

// TestTopologicalKnown: on a forced chain 0->1->2 the topological order
// is exactly [0 1 2].
func TestTopologicalKnown(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)

	top := NewTopological(g)
	if !top.HasOrder() {
		t.Fatalf("Expected a chain to have a topological order")
	}
	if got := top.Order(); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Errorf("Order error, expected [0 1 2] got %v", got)
	}
}

func TestTopologicalFreshSlice(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	top := NewTopological(g)

	first := top.Order()
	first[0] = 99 // mutate the returned slice
	if got := top.Order(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("Expected Order to return a fresh slice each call, got %v", got)
	}
}

func TestTopologicalSelfLoop(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 0) // a self-loop is a cycle: no topological order

	top := NewTopological(g)
	if top.HasOrder() {
		t.Errorf("Expected no topological order with a self-loop, got %v", top.Order())
	}
}

// TestTopologicalNilAndEmpty: a nil or empty digraph has no vertices to
// order — a sane answer — so NewTopological does not panic.
func TestTopologicalNilAndEmpty(t *testing.T) {
	var nilGraph *Digraph
	var zeroGraph Digraph
	for _, g := range []*Digraph{nilGraph, &zeroGraph} {
		top := NewTopological(g)
		if top.HasOrder() {
			t.Errorf("Expected no order for nil/empty digraph")
		}
		if order := top.Order(); order != nil {
			t.Errorf("Expected nil Order for nil/empty digraph, got %v", order)
		}
	}
}

func TestTopologicalNilTolerated(t *testing.T) {
	var top *Topological
	if top.HasOrder() {
		t.Errorf("Expected false from HasOrder on nil receiver.")
	}
	if order := top.Order(); order != nil {
		t.Errorf("Expected nil from Order on nil receiver, got %v", order)
	}
}

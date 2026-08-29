package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

// TestDepthFirstOrderKnown traces the orders on a small DAG:
// 0->1, 0->2, 1->3, 2->3, 3->4.  DFS from 0 dives 0->1->3->4 first.
func TestDepthFirstOrderKnown(t *testing.T) {
	g := NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	o := NewDepthFirstOrder(g)
	if got := o.Pre(); !reflect.DeepEqual(got, []int{0, 1, 3, 4, 2}) {
		t.Errorf("Pre error, expected [0 1 3 4 2] got %v", got)
	}
	if got := o.Post(); !reflect.DeepEqual(got, []int{4, 3, 1, 2, 0}) {
		t.Errorf("Post error, expected [4 3 1 2 0] got %v", got)
	}
	if got := o.ReversePost(); !reflect.DeepEqual(got, []int{0, 2, 1, 3, 4}) {
		t.Errorf("ReversePost error, expected [0 2 1 3 4] got %v", got)
	}
}

func TestDepthFirstOrderFreshSlices(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	o := NewDepthFirstOrder(g)

	first := o.ReversePost()
	first[0] = 99 // mutate the returned slice
	if got := o.ReversePost(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("Expected ReversePost to return a fresh slice each call, got %v", got)
	}
}

// TestDepthFirstOrderNilAndEmpty: a nil or empty digraph has empty
// orders — a sane answer, so NewDepthFirstOrder does not panic.
func TestDepthFirstOrderNilAndEmpty(t *testing.T) {
	var nilGraph *Digraph
	var zeroGraph Digraph
	for _, g := range []*Digraph{nilGraph, &zeroGraph} {
		o := NewDepthFirstOrder(g)
		if o.Pre() != nil || o.Post() != nil || o.ReversePost() != nil {
			t.Errorf("Expected empty orders for nil/empty digraph, got %v %v %v", o.Pre(), o.Post(), o.ReversePost())
		}
	}
}

func TestDepthFirstOrderNilTolerated(t *testing.T) {
	var o *DepthFirstOrder
	if o.Pre() != nil || o.Post() != nil || o.ReversePost() != nil {
		t.Errorf("Expected nil orders on nil receiver.")
	}
}

package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import "testing"

// TestTransitiveClosureTinyDG verifies the closure against the Warshall
// oracle on the algs4 tinyDG digraph (13 vertices, 22 edges, 5 SCCs).
func TestTransitiveClosureTinyDG(t *testing.T) {
	g := newTinyDG(t)
	tc := NewTransitiveClosure(g)
	want := refWarshall(adjOfAll(t, g))
	for v := 0; v < g.V(); v++ {
		for w := 0; w < g.V(); w++ {
			if got := tc.Reachable(v, w); got != want[v][w] {
				t.Errorf("Reachable(%d, %d)=%v, Warshall says %v", v, w, got, want[v][w])
			}
		}
	}
}

// TestTransitiveClosureBasics verifies the tolerance contract: empty
// digraphs, self-reachability, out-of-range queries, and a nil query
// object.
func TestTransitiveClosureBasics(t *testing.T) {
	var nilDigraph *Digraph
	tc := NewTransitiveClosure(nilDigraph)
	if tc.Reachable(0, 0) {
		t.Errorf("Expected no reachable pairs on a nil digraph.")
	}
	var zero Digraph
	if tc := NewTransitiveClosure(&zero); tc.Reachable(0, 0) {
		t.Errorf("Expected no reachable pairs on a zero-value digraph.")
	}

	// Every vertex reaches itself, even isolated ones.
	g := NewDigraph(4)
	g.AddEdge(0, 1)
	g.AddEdge(2, 3)
	g.AddEdge(3, 2)
	tc = NewTransitiveClosure(g)
	for v := 0; v < 4; v++ {
		if !tc.Reachable(v, v) {
			t.Errorf("Expected Reachable(%d, %d) to be true (self-reach).", v, v)
		}
	}
	if !tc.Reachable(0, 1) || tc.Reachable(0, 2) || tc.Reachable(1, 0) {
		t.Errorf("Reachability across the two components is wrong.")
	}
	if !tc.Reachable(2, 3) || !tc.Reachable(3, 2) {
		t.Errorf("Expected 2 and 3 to reach each other.")
	}

	// Out-of-range vertices and a nil receiver report false.
	if tc.Reachable(-1, 0) || tc.Reachable(0, 4) || tc.Reachable(4, 4) {
		t.Errorf("Expected out-of-range Reachable queries to report false.")
	}
	var nilTC *TransitiveClosure
	if nilTC.Reachable(0, 0) {
		t.Errorf("Expected a nil *TransitiveClosure to report false.")
	}
}

// TestTransitiveClosureDAG verifies reachability along a chain: the
// closure of 0->1->2->3 knows every forward pair and no backward pair.
func TestTransitiveClosureDAG(t *testing.T) {
	g := NewDigraph(4)
	for v := 0; v+1 < 4; v++ {
		g.AddEdge(v, v+1)
	}
	tc := NewTransitiveClosure(g)
	for v := 0; v < 4; v++ {
		for w := 0; w < 4; w++ {
			want := w >= v
			if tc.Reachable(v, w) != want {
				t.Errorf("Reachable(%d, %d)=%v, expected %v on the chain", v, w, tc.Reachable(v, w), want)
			}
		}
	}
}

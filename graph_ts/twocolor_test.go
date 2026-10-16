package graph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import "testing"

// checkProperColoring verifies that the TwoColor colors split every
// edge of g between the two sides — a certificate of bipartiteness.
func checkProperColoring(t *testing.T, g *Graph, tc *TwoColor) {
	t.Helper()
	for v := 0; v < g.V(); v++ {
		cv, ok := tc.Color(v)
		if !ok {
			t.Fatalf("Color(%d) out of range on a %d-vertex graph", v, g.V())
		}
		for w := range g.Adj(v) {
			cw, _ := tc.Color(w)
			if cw == cv {
				t.Fatalf("Edge %d-%d joins two %v-colored vertices", v, w, cv)
			}
		}
	}
}

// TestTwoColorBipartite verifies bipartite graphs: an edgeless graph,
// a path, an even cycle, a tree, and a graph with parallel edges (a
// 2-edge cycle is even).
func TestTwoColorBipartite(t *testing.T) {
	edgeless := NewGraph(5)
	if tc := NewTwoColor(edgeless); !tc.IsBipartite() {
		t.Errorf("Expected an edgeless graph to be bipartite.")
	}

	path := NewGraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}} {
		path.AddEdge(e[0], e[1])
	}
	if tc := NewTwoColor(path); !tc.IsBipartite() {
		t.Errorf("Expected a path to be bipartite.")
	} else {
		checkProperColoring(t, path, tc)
	}

	even := NewGraph(4)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}} {
		even.AddEdge(e[0], e[1])
	}
	if tc := NewTwoColor(even); !tc.IsBipartite() {
		t.Errorf("Expected an even cycle to be bipartite.")
	} else {
		checkProperColoring(t, even, tc)
	}

	tree := NewGraph(10)
	for v := 1; v < 10; v++ {
		tree.AddEdge(v, v/2)
	}
	if tc := NewTwoColor(tree); !tc.IsBipartite() {
		t.Errorf("Expected a tree to be bipartite.")
	} else {
		checkProperColoring(t, tree, tc)
	}

	parallel := NewGraph(3)
	parallel.AddEdge(0, 1)
	parallel.AddEdge(0, 1) // parallel edges form a 2-edge (even) cycle
	parallel.AddEdge(1, 2)
	if tc := NewTwoColor(parallel); !tc.IsBipartite() {
		t.Errorf("Expected parallel edges to keep the graph bipartite.")
	} else {
		checkProperColoring(t, parallel, tc)
	}
}

// TestTwoColorNotBipartite verifies non-bipartite graphs: an odd cycle
// and a self-loop (a 1-edge odd cycle).
func TestTwoColorNotBipartite(t *testing.T) {
	odd := NewGraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 0}} {
		odd.AddEdge(e[0], e[1])
	}
	if tc := NewTwoColor(odd); tc.IsBipartite() {
		t.Errorf("Expected a 5-cycle to be non-bipartite.")
	}

	selfLoop := NewGraph(2)
	selfLoop.AddEdge(0, 1)
	selfLoop.AddEdge(1, 1)
	if tc := NewTwoColor(selfLoop); tc.IsBipartite() {
		t.Errorf("Expected a self-loop to be non-bipartite.")
	}
}

// TestTwoColorNilEmpty verifies the tolerant contract.
func TestTwoColorNilEmpty(t *testing.T) {
	var nilGraph *Graph
	if tc := NewTwoColor(nilGraph); !tc.IsBipartite() {
		t.Errorf("Expected a nil graph to be vacuously bipartite.")
	}
	var zero Graph
	if tc := NewTwoColor(&zero); !tc.IsBipartite() {
		t.Errorf("Expected a zero-value graph to be vacuously bipartite.")
	}
	// Out-of-range Color queries report ok=false.
	g := NewGraph(3)
	g.AddEdge(0, 1)
	tc := NewTwoColor(g)
	if _, ok := tc.Color(-1); ok {
		t.Errorf("Expected Color(-1) to report ok=false.")
	}
	if _, ok := tc.Color(3); ok {
		t.Errorf("Expected Color(3) on a 3-vertex graph to report ok=false.")
	}
	var nilTC *TwoColor
	if nilTC.IsBipartite() {
		t.Errorf("Expected a nil *TwoColor to report not-bipartite.")
	}
	if _, ok := nilTC.Color(0); ok {
		t.Errorf("Expected a nil *TwoColor Color query to report ok=false.")
	}
}

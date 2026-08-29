package digraph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import "testing"

// TestKosarajuSCCTinyDG is the algs4 known answer: tinyDG has exactly 5
// strong components — {0 2 3 4 5}, {6 8}, {9 10 11 12}, {1}, {7}
// (https://algs4.cs.princeton.edu/42directed/, KosarajuSharirSCC trace).
func TestKosarajuSCCTinyDG(t *testing.T) {
	g := newTinyDG(t)
	scc := NewKosarajuSCC(g)

	if scc.Count() != 5 {
		t.Fatalf("Expected 5 strong components, got %d", scc.Count())
	}
	components := [][]int{
		{0, 2, 3, 4, 5},
		{6, 8},
		{9, 10, 11, 12},
		{1},
		{7},
	}
	for _, comp := range components {
		for _, v := range comp {
			for _, w := range comp {
				if !scc.StronglyConnected(v, w) {
					t.Errorf("Expected StronglyConnected(%d, %d) to be true", v, w)
				}
				// Same component means same id.
				idv, okv := scc.ID(v)
				idw, okw := scc.ID(w)
				if !okv || !okw || idv != idw {
					t.Errorf("Expected ID(%d)==ID(%d), got %d/%d (ok=%v/%v)", v, w, idv, idw, okv, okw)
				}
			}
		}
	}
	for i, a := range components {
		for j, b := range components {
			if i == j {
				continue
			}
			if scc.StronglyConnected(a[0], b[0]) {
				t.Errorf("Expected StronglyConnected(%d, %d) to be false (different components)", a[0], b[0])
			}
		}
	}
	// Component ids are compact: 0..Count()-1, all used.
	used := make([]bool, scc.Count())
	for v := 0; v < g.V(); v++ {
		id, ok := scc.ID(v)
		if !ok || id < 0 || id >= scc.Count() {
			t.Errorf("ID(%d)=%d (ok=%v) out of compact range", v, id, ok)
			continue
		}
		used[id] = true
	}
	for id, u := range used {
		if !u {
			t.Errorf("Component id %d is unused — ids must be compact", id)
		}
	}
}

// TestKosarajuSCCTinyDAG: a DAG has only singleton strong components.
func TestKosarajuSCCTinyDAG(t *testing.T) {
	g := newTinyDAG(t)
	scc := NewKosarajuSCC(g)
	if scc.Count() != 13 {
		t.Errorf("Expected 13 strong components for a DAG, got %d", scc.Count())
	}
	for v := 0; v < g.V(); v++ {
		if !scc.StronglyConnected(v, v) {
			t.Errorf("Expected StronglyConnected(%d, %d) to be true", v, v)
		}
		for w := v + 1; w < g.V(); w++ {
			if scc.StronglyConnected(v, w) {
				t.Errorf("Expected StronglyConnected(%d, %d) to be false in a DAG", v, w)
			}
		}
	}
}

func TestKosarajuSCCNoEdges(t *testing.T) {
	g := NewDigraph(5) // 5 isolated vertices = 5 strong components
	scc := NewKosarajuSCC(g)
	if scc.Count() != 5 {
		t.Errorf("Expected 5 strong components for an edgeless digraph, got %d", scc.Count())
	}
	for v := 0; v < 5; v++ {
		if !scc.StronglyConnected(v, v) {
			t.Errorf("Expected StronglyConnected(%d, %d) to be true", v, v)
		}
	}
	if scc.StronglyConnected(0, 1) {
		t.Errorf("Expected StronglyConnected(0, 1) to be false")
	}
}

func TestKosarajuSCCSelfLoop(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 0) // a self-loop does not merge components
	scc := NewKosarajuSCC(g)
	if scc.Count() != 2 {
		t.Errorf("Expected 2 strong components, got %d", scc.Count())
	}
	if scc.StronglyConnected(0, 1) {
		t.Errorf("Expected StronglyConnected(0, 1) to be false")
	}
}

func TestKosarajuSCCSnapshot(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	scc := NewKosarajuSCC(g)
	// The result object is a snapshot: later edges are not reflected.
	g.AddEdge(1, 0)
	if scc.StronglyConnected(0, 1) || scc.Count() != 2 {
		t.Errorf("Expected snapshot semantics: later AddEdge must not be reflected")
	}
}

// TestKosarajuSCCNilAndEmpty: a nil or empty digraph has a sane answer —
// no components — so NewKosarajuSCC does not panic.
func TestKosarajuSCCNilAndEmpty(t *testing.T) {
	var nilGraph *Digraph
	var zeroGraph Digraph
	for _, g := range []*Digraph{nilGraph, &zeroGraph} {
		scc := NewKosarajuSCC(g)
		if scc.Count() != 0 {
			t.Errorf("Expected 0 components for nil/empty digraph, got %d", scc.Count())
		}
		if scc.StronglyConnected(0, 0) {
			t.Errorf("Expected StronglyConnected to be false for nil/empty digraph")
		}
		if _, ok := scc.ID(0); ok {
			t.Errorf("Expected ID to report out of range for nil/empty digraph")
		}
	}
}

func TestKosarajuSCCNilTolerated(t *testing.T) {
	var scc *KosarajuSCC
	if scc.Count() != 0 {
		t.Errorf("Expected Count 0 on nil receiver.")
	}
	if scc.StronglyConnected(0, 0) {
		t.Errorf("Expected false from StronglyConnected on nil receiver.")
	}
	if _, ok := scc.ID(0); ok {
		t.Errorf("Expected not-found from ID on nil receiver.")
	}
}

func TestKosarajuSCCOutOfRangeQuery(t *testing.T) {
	g := newTinyDG(t)
	scc := NewKosarajuSCC(g)
	for _, v := range []int{-1, 13, 100} {
		if _, ok := scc.ID(v); ok {
			t.Errorf("Expected ID(%d) to report out of range", v)
		}
		if scc.StronglyConnected(v, 0) || scc.StronglyConnected(0, v) {
			t.Errorf("Expected StronglyConnected with out-of-range %d to be false", v)
		}
	}
}

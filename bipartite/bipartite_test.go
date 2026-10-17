/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite

import (
	"slices"
	"testing"

	"github.com/pschlump/pluto/graph"
)

func expectPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", what)
		}
	}()
	fn()
}

// buildGraph builds a graph from edge pairs (deduplicated callers
// beware: parallel edges are legal input).
func buildGraph(n int, edges [][2]int) *graph.Graph {
	g := graph.NewGraph(n)
	for _, e := range edges {
		g.AddEdge(e[0], e[1])
	}
	return g
}

func TestSingleEdge(t *testing.T) {
	g := buildGraph(2, [][2]int{{0, 1}})
	for _, m := range []struct {
		name string
		api  matchingAPI
	}{
		{"BipartiteMatching", NewBipartiteMatching(g)},
		{"HopcroftKarp", NewHopcroftKarp(g)},
	} {
		if m.api.Cardinality() != 1 {
			t.Fatalf("%s: cardinality = %d, want 1", m.name, m.api.Cardinality())
		}
		if got, want := m.api.Edges(), [][2]int{{0, 1}}; !slices.Equal(got, want) {
			t.Fatalf("%s: edges = %v, want %v", m.name, got, want)
		}
		checkCertificate(t, m.name, g, m.api)
	}
}

// TestAugmentingFlipWitness is the graph where a greedy maximal
// matching gets stuck at 1 and only a re-augmenting flip reaches the
// maximum 2: whites {0,2}, blacks {1,3}, edges 0-1, 0-3, 2-1.  Vertex
// order makes both algorithms first match 0-1, then 2 must steal 1 and
// re-house 0 with 3.
func TestAugmentingFlipWitness(t *testing.T) {
	g := buildGraph(4, [][2]int{{0, 1}, {0, 3}, {2, 1}})
	for name, api := range map[string]matchingAPI{
		"BipartiteMatching": NewBipartiteMatching(g),
		"HopcroftKarp":      NewHopcroftKarp(g),
	} {
		if api.Cardinality() != 2 {
			t.Fatalf("%s: cardinality = %d, want 2 (the flip witness)", name, api.Cardinality())
		}
		checkCertificate(t, name, g, api)
	}
}

func TestKnownGraphs(t *testing.T) {
	cases := []struct {
		name        string
		n           int
		edges       [][2]int
		cardinality int
	}{
		{"even path", 4, [][2]int{{0, 1}, {1, 2}, {2, 3}}, 2},
		{"odd path", 5, [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}}, 2},
		{"star", 4, [][2]int{{0, 1}, {0, 2}, {0, 3}}, 1},
		{"K33", 6, [][2]int{{0, 1}, {0, 3}, {0, 5}, {2, 1}, {2, 3}, {2, 5}, {4, 1}, {4, 3}, {4, 5}}, 3},
		{"no edges", 5, nil, 0},
		{"parallel edges", 3, [][2]int{{0, 1}, {0, 1}, {0, 2}}, 1},
	}
	for _, tc := range cases {
		g := buildGraph(tc.n, tc.edges)
		for name, api := range map[string]matchingAPI{
			"BipartiteMatching": NewBipartiteMatching(g),
			"HopcroftKarp":      NewHopcroftKarp(g),
		} {
			if api.Cardinality() != tc.cardinality {
				t.Fatalf("%s/%s: cardinality = %d, want %d", tc.name, name, api.Cardinality(), tc.cardinality)
			}
			checkCertificate(t, tc.name+"/"+name, g, api)
		}
	}
}

func TestNonBipartitePanics(t *testing.T) {
	triangle := buildGraph(3, [][2]int{{0, 1}, {1, 2}, {2, 0}})
	expectPanic(t, "NewBipartiteMatching on a triangle", func() {
		NewBipartiteMatching(triangle)
	})
	expectPanic(t, "NewHopcroftKarp on a triangle", func() {
		NewHopcroftKarp(triangle)
	})

	selfLoop := graph.NewGraph(2)
	selfLoop.AddEdge(0, 0) // a self-loop is an odd cycle
	selfLoop.AddEdge(0, 1)
	expectPanic(t, "NewBipartiteMatching on a self-loop", func() {
		NewBipartiteMatching(selfLoop)
	})
	expectPanic(t, "NewHopcroftKarp on a self-loop", func() {
		NewHopcroftKarp(selfLoop)
	})
}

func TestNilEmptyTolerance(t *testing.T) {
	var nilBM *BipartiteMatching
	var nilHK *HopcroftKarp
	if nilBM.Cardinality() != 0 || nilHK.Cardinality() != 0 {
		t.Fatalf("nil receivers: cardinality not 0")
	}
	for _, v := range []int{-1, 0, 3} {
		if nilBM.Mate(v) != -1 || nilHK.Mate(v) != -1 {
			t.Fatalf("nil receivers: Mate(%d) != -1", v)
		}
		if nilBM.InMatching(v) || nilHK.InMatching(v) {
			t.Fatalf("nil receivers: InMatching(%d) true", v)
		}
		if nilBM.InMinVertexCover(v) || nilHK.InMinVertexCover(v) {
			t.Fatalf("nil receivers: InMinVertexCover(%d) true", v)
		}
	}
	if nilBM.Edges() != nil || nilHK.Edges() != nil {
		t.Fatalf("nil receivers: Edges() not nil")
	}
	if nilBM.MinVertexCover() != nil || nilHK.MinVertexCover() != nil {
		t.Fatalf("nil receivers: MinVertexCover() not nil")
	}

	var zeroBM BipartiteMatching
	var zeroHK HopcroftKarp
	if zeroBM.Cardinality() != 0 || zeroHK.Cardinality() != 0 {
		t.Fatalf("zero values: cardinality not 0")
	}
	if zeroBM.Mate(0) != -1 || zeroHK.Mate(0) != -1 {
		t.Fatalf("zero values: Mate(0) != -1")
	}

	// A nil or empty graph is vacuously bipartite: the empty matching.
	for name, api := range map[string]matchingAPI{
		"BipartiteMatching": NewBipartiteMatching(nil),
		"HopcroftKarp":      NewHopcroftKarp(nil),
	} {
		if api.Cardinality() != 0 {
			t.Fatalf("%s(nil graph): cardinality = %d, want 0", name, api.Cardinality())
		}
		if api.Edges() != nil {
			t.Fatalf("%s(nil graph): Edges() = %v, want nil", name, api.Edges())
		}
	}
	empty := graph.NewGraph(3) // vertices, no edges
	if m := NewBipartiteMatching(empty); m.Cardinality() != 0 || m.MinVertexCover() != nil {
		t.Fatalf("empty graph: cardinality = %d, cover = %v", m.Cardinality(), m.MinVertexCover())
	}
	if m := NewHopcroftKarp(empty); m.Cardinality() != 0 {
		t.Fatalf("empty graph (HK): cardinality = %d", m.Cardinality())
	}
}

// TestOutOfRangeAndIsolated checks isolated vertices and out-of-range
// queries against a graph with both.
func TestOutOfRangeAndIsolated(t *testing.T) {
	g := buildGraph(6, [][2]int{{0, 1}, {2, 3}}) // 4 and 5 isolated
	bm := NewBipartiteMatching(g)
	hk := NewHopcroftKarp(g)
	for name, api := range map[string]matchingAPI{"BM": bm, "HK": hk} {
		if api.Cardinality() != 2 {
			t.Fatalf("%s: cardinality = %d, want 2", name, api.Cardinality())
		}
		if api.Mate(4) != -1 || api.Mate(5) != -1 {
			t.Fatalf("%s: isolated vertices matched", name)
		}
		if api.Mate(-1) != -1 || api.Mate(6) != -1 || api.Mate(1000) != -1 {
			t.Fatalf("%s: out-of-range Mate != -1", name)
		}
		if api.InMinVertexCover(-1) || api.InMinVertexCover(6) {
			t.Fatalf("%s: out-of-range InMinVertexCover true", name)
		}
		checkCertificate(t, name, g, api)
	}
}

// TestDeterministicOutput: the same graph produces the same matching
// on repeated construction (adjacency iteration is deterministic).
func TestDeterministicOutput(t *testing.T) {
	g := buildGraph(8, [][2]int{{0, 5}, {0, 7}, {2, 5}, {2, 7}, {4, 5}, {4, 7}, {6, 5}})
	for name, build := range map[string]func() matchingAPI{
		"BipartiteMatching": func() matchingAPI { return NewBipartiteMatching(g) },
		"HopcroftKarp":      func() matchingAPI { return NewHopcroftKarp(g) },
	} {
		a, b := build(), build()
		if !slices.Equal(a.Edges(), b.Edges()) {
			t.Fatalf("%s: nondeterministic Edges: %v vs %v", name, a.Edges(), b.Edges())
		}
		if !slices.Equal(a.MinVertexCover(), b.MinVertexCover()) {
			t.Fatalf("%s: nondeterministic MinVertexCover: %v vs %v", name, a.MinVertexCover(), b.MinVertexCover())
		}
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/internal/testgraphs"
)

// sccAPI is the surface the three SCC algorithms share.
type sccAPI interface {
	StronglyConnected(v, w int) bool
	ID(v int) (id int, ok bool)
	Count() int
}

// TestSCCStrongLabels runs all three SCC algorithms on Strong-generated
// digraphs, which come with a KNOWN strong-component partition (two
// vertices are strongly connected exactly when their labels are equal —
// cross-label edges all point from smaller to larger, so no path
// returns).  This is an external oracle: TestSCCThreeWayAgreement only
// proves the three algorithms agree with each other, which a shared
// bug would hide.
func TestSCCStrongLabels(t *testing.T) {
	rng := rand.New(rand.NewSource(1403)) // fixed seed: deterministic run
	rounds := 0
	for step := 0; rounds < 250; step++ {
		v := 2 + rng.Intn(20)
		c := 1 + rng.Intn(v)
		lo := 2 * (v - c)     // Strong requires e > lo (the in/out trees)
		hi := v * (v - 1) / 2 // and e <= hi (simple digraph).
		if lo+1 > hi {
			continue // e.g. v=2 needs c=2 — one 2-vertex SCC wants 2 edges but hi is 1
		}
		rounds++
		e := lo + 1 + rng.Intn(hi-lo)
		sg := testgraphs.Strong(rng, v, e, c)
		if len(sg.Edges) != e {
			t.Fatalf("step %d: Strong returned %d edges, want exactly %d", step, len(sg.Edges), e)
		}
		g := buildDigraph(t, sg.V, sg.Edges)
		want := canonicalComponents(func(w int) (int, bool) { return sg.Label[w], true }, v)
		for name, scc := range map[string]sccAPI{
			"Kosaraju": NewKosarajuSCC(g),
			"Tarjan":   NewTarjanSCC(g),
			"Gabow":    NewGabowSCC(g),
		} {
			if scc.Count() != c {
				t.Fatalf("step %d %s: SCC count %d, want exactly %d", step, name, scc.Count(), c)
			}
			if got := canonicalComponents(scc.ID, v); got != want {
				t.Fatalf("step %d %s: partition %s, want the label partition %s", step, name, got, want)
			}
			for a := 0; a < v; a++ {
				for b := a + 1; b < v; b++ {
					got, wantSC := scc.StronglyConnected(a, b), sg.Label[a] == sg.Label[b]
					if got != wantSC {
						t.Fatalf("step %d %s: StronglyConnected(%d, %d) = %v, want %v (labels %d, %d)",
							step, name, a, b, got, wantSC, sg.Label[a], sg.Label[b])
					}
				}
			}
		}
	}
}

// TestTopologicalGeneratedDAGs feeds generated DAGs through the
// topological query: an order must exist and must place every edge's
// tail before its head.  Generated directed cycles (which have no
// topological order) must report HasOrder false.
func TestTopologicalGeneratedDAGs(t *testing.T) {
	rng := rand.New(rand.NewSource(1404)) // fixed seed: deterministic run
	for step := 0; step < 200; step++ {
		v := 2 + rng.Intn(16)
		hi := v * (v - 1) / 2
		e := (v - 1) + rng.Intn(hi-(v-1)+1) // the rooted DAGs need at least v−1
		// A slice, not a map: the three generators draw from rng, and map
		// iteration order is randomized — a map would break determinism.
		cases := []struct {
			name string
			gen  testgraphs.G
		}{
			{"DAG", testgraphs.DAG(rng, v, e)},
			{"RootedInDAG", testgraphs.RootedInDAG(rng, v, e)},
			{"RootedOutDAG", testgraphs.RootedOutDAG(rng, v, e)},
		}
		for _, tc := range cases {
			g := buildDigraph(t, tc.gen.V, tc.gen.Edges)
			topo := NewTopological(g)
			if !topo.HasOrder() {
				t.Fatalf("step %d %s: generated DAG reported no topological order", step, tc.name)
			}
			pos := make([]int, tc.gen.V)
			for i, x := range topo.Order() {
				pos[x] = i
			}
			for _, ed := range tc.gen.Edges {
				if pos[ed[0]] >= pos[ed[1]] {
					t.Fatalf("step %d %s: order puts edge tail %d not before head %d", step, tc.name, ed[0], ed[1])
				}
			}
		}
	}
	bt := buildDigraph(t, 15, testgraphs.BinaryTreeDigraph(15).Edges)
	if topo := NewTopological(bt); !topo.HasOrder() {
		t.Fatalf("binary tree digraph (edges to heap parents) reported no topological order")
	}
	for step := 0; step < 50; step++ {
		v := 2 + rng.Intn(12)
		cases := []struct {
			name string
			gen  testgraphs.G
		}{
			{"cycle", testgraphs.CycleDigraph(rng, v)},
			{"eulerianCycle", testgraphs.EulerianCycleDigraph(rng, v, 1+rng.Intn(3*v))},
		}
		for _, tc := range cases {
			if topo := NewTopological(buildDigraph(t, tc.gen.V, tc.gen.Edges)); topo.HasOrder() {
				t.Fatalf("step %d %s: a directed cycle has no topological order but one was reported", step, tc.name)
			}
		}
	}
}

// TestDirectedEulerianGenerated feeds EulerianCycleDigraph- and
// EulerianPathDigraph-generated digraphs — guaranteed to HAVE the walk,
// unlike TestDirectedEulerianRandomized — through the queries and
// verifies the returned walks use every edge exactly once.  A directed
// Eulerian cycle also implies an Eulerian path.
func TestDirectedEulerianGenerated(t *testing.T) {
	rng := rand.New(rand.NewSource(1405)) // fixed seed: deterministic run
	for step := 0; step < 300; step++ {
		v := 1 + rng.Intn(18)
		e := 1 + rng.Intn(3*v)
		g := buildDigraph(t, v, testgraphs.EulerianCycleDigraph(rng, v, e).Edges)
		ec := NewDirectedEulerianCycle(g)
		if !ec.HasEulerianCycle() {
			t.Fatalf("step %d: generated digraph HAS a directed Eulerian cycle but HasEulerianCycle is false", step)
		}
		verifyDirectedWalk(t, directedEdgeMultiset(g), ec.Cycle(), true)
		if p := NewDirectedEulerianPath(g); !p.HasEulerianPath() {
			t.Fatalf("step %d: an Eulerian cycle implies an Eulerian path, but HasEulerianPath is false", step)
		}
	}
	for step := 0; step < 300; step++ {
		v := 1 + rng.Intn(18)
		e := 1 + rng.Intn(3*v)
		g := buildDigraph(t, v, testgraphs.EulerianPathDigraph(rng, v, e).Edges)
		p := NewDirectedEulerianPath(g)
		if !p.HasEulerianPath() {
			t.Fatalf("step %d: generated digraph HAS a directed Eulerian path but HasEulerianPath is false", step)
		}
		verifyDirectedWalk(t, directedEdgeMultiset(g), p.Path(), false)
	}
}

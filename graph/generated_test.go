/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/internal/testgraphs"
)

// buildFromG builds a graph from a testgraphs edge list (parallel edges
// and self loops preserved — both are legal input; vertices are in
// range by construction, so AddEdge's bool is ignored).
func buildFromG(gen testgraphs.G) *Graph {
	g := NewGraph(gen.V)
	for _, e := range gen.Edges {
		_ = g.AddEdge(e[0], e[1])
	}
	return g
}

// TestEulerianCycleGenerated feeds EulerianCycle-generated graphs —
// guaranteed to HAVE an Eulerian cycle, unlike the random graphs of
// TestEulerianRandomized where a cycle usually does not exist and the
// walk-building code is never exercised — through the query and
// verifies the returned walk uses every edge exactly once.
func TestEulerianCycleGenerated(t *testing.T) {
	rng := rand.New(rand.NewSource(1401)) // fixed seed: deterministic run
	for step := 0; step < 300; step++ {
		v := 1 + rng.Intn(18)
		e := 1 + rng.Intn(3*v)
		g := buildFromG(testgraphs.EulerianCycle(rng, v, e))
		ec := NewEulerianCycle(g)
		if !ec.HasEulerianCycle() {
			t.Fatalf("step %d: generated graph HAS an Eulerian cycle (even degrees, touched part connected) but HasEulerianCycle is false", step)
		}
		verifyUndirectedWalk(t, undirectedEdgeMultiset(g), ec.Cycle(), true)
	}
}

// TestEulerianPathGenerated does the same for EulerianPath-generated
// graphs (guaranteed to have a path; the walk may also be closed when
// start and end coincide).  A graph with an Eulerian cycle also has an
// Eulerian path, so the cycle generator feeds the path query too.
func TestEulerianPathGenerated(t *testing.T) {
	rng := rand.New(rand.NewSource(1402)) // fixed seed: deterministic run
	for step := 0; step < 300; step++ {
		v := 1 + rng.Intn(18)
		e := 1 + rng.Intn(3*v)
		g := buildFromG(testgraphs.EulerianPath(rng, v, e))
		p := NewEulerianPath(g)
		if !p.HasEulerianPath() {
			t.Fatalf("step %d: generated graph HAS an Eulerian path but HasEulerianPath is false", step)
		}
		verifyUndirectedWalk(t, undirectedEdgeMultiset(g), p.Path(), false)
	}
	for step := 0; step < 200; step++ {
		v := 1 + rng.Intn(14)
		e := 1 + rng.Intn(2*v)
		g := buildFromG(testgraphs.EulerianCycle(rng, v, e))
		if p := NewEulerianPath(g); !p.HasEulerianPath() {
			t.Fatalf("cycle step %d: an Eulerian cycle implies an Eulerian path, but HasEulerianPath is false", step)
		}
	}
}

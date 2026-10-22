/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/internal/testgraphs"
)

// TestMatchingOnGeneratedSides runs both matchings on generated
// bipartite graphs whose two sides are KNOWN — side A is [0, V1) and
// side B is [V1, V), the contiguous partition testgraphs documents.
// Every reported matching edge must cross the sides, the cardinality
// can never exceed the smaller side, and the König certificate
// (checkCertificate) proves the reported matching maximum.
func TestMatchingOnGeneratedSides(t *testing.T) {
	rng := rand.New(rand.NewSource(1406)) // fixed seed: deterministic run
	for step := 0; step < 200; step++ {
		v1 := 1 + rng.Intn(10)
		v2 := 1 + rng.Intn(10)
		e := rng.Intn(v1*v2 + 1)
		bg := testgraphs.Bipartite(rng, v1, v2, e)
		g := buildGraph(bg.V, bg.Edges)
		for name, m := range map[string]matchingAPI{
			"BipartiteMatching": NewBipartiteMatching(g),
			"HopcroftKarp":      NewHopcroftKarp(g),
		} {
			if card := m.Cardinality(); card > min(v1, v2) {
				t.Fatalf("step %d %s: cardinality %d exceeds the smaller side min(%d, %d)", step, name, card, v1, v2)
			}
			for _, ed := range m.Edges() {
				if (ed[0] < bg.V1) == (ed[1] < bg.V1) {
					t.Fatalf("step %d %s: matching edge %v lies inside one side — it is not an edge of the generated graph", step, name, ed)
				}
			}
			checkCertificate(t, name, g, m)
		}
	}
}

// TestMatchingOnCompleteBipartite pins the exact maximum: in K(v1, v2)
// every vertex of the smaller side can be matched and nothing larger
// can exist, so the cardinality must be exactly min(v1, v2).  The
// generated sides are contiguous, which also makes this a known-answer
// test of the side handling (a match reported between two vertices of
// the same side would fail here even if it sized the matching right).
func TestMatchingOnCompleteBipartite(t *testing.T) {
	for _, tc := range []struct{ v1, v2 int }{
		{1, 1}, {3, 4}, {4, 3}, {5, 5}, {2, 9}, {9, 2},
	} {
		bg := testgraphs.CompleteBipartite(tc.v1, tc.v2)
		g := buildGraph(bg.V, bg.Edges)
		for name, m := range map[string]matchingAPI{
			"BipartiteMatching": NewBipartiteMatching(g),
			"HopcroftKarp":      NewHopcroftKarp(g),
		} {
			if m.Cardinality() != min(tc.v1, tc.v2) {
				t.Fatalf("K(%d, %d) %s: cardinality %d, want exactly %d",
					tc.v1, tc.v2, name, m.Cardinality(), min(tc.v1, tc.v2))
			}
			checkCertificate(t, name, g, m)
		}
	}
}

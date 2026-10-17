/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bipartite implements maximum bipartite matching (Sedgwick,
// Algorithms 4th ed. §6.4): pairing the two sides of a bipartite graph
// so that as many vertices as possible are matched, no two pairs
// sharing a vertex.
//
// Two constructor-style immutable query objects share one surface (the
// mst precedent of same-shape alternatives):
//
//	NewBipartiteMatching — the classic augmenting-path algorithm
//	                      (Kuhn's method): one alternating path at a
//	                      time, O(V·E) worst case.
//	NewHopcroftKarp     — the phased algorithm of Hopcroft and Karp:
//	                      BFS layering plus a maximal set of
//	                      vertex-disjoint shortest augmenting paths per
//	                      phase, O(E·√V).
//
// The bipartition itself is computed by graph.NewTwoColor (an
// intra-pluto composition — the two-coloring IS the partition).  A
// graph that is not bipartite (an odd cycle — a self-loop is one) has
// no bipartite matching, so the constructors panic naming themselves;
// a nil or empty graph is vacuously bipartite and yields the empty
// matching (the NewCC precedent).  Results are immutable snapshots:
// later AddEdge calls on the graph are not reflected, and there is no
// _ts twin (the maxflow precedent).
//
// Beyond the matching, both objects expose the minimum vertex cover of
// König's theorem (constructively derived from the maximum matching):
// the smallest set of vertices touching every edge, whose size equals
// the matching's cardinality.  The pair is self-certifying — a valid
// cover no larger than the matching proves the matching maximum (every
// matching edge needs its own cover vertex) — and the test suite
// checks exactly that certificate.
//
// Panic contract: the two constructors panic on a non-bipartite graph
// (no sane answer); every query on a nil result object reports
// zero / -1 / false / nil, and out-of-range vertices report the same
// as unmatched.  There are no other panics.
package bipartite

import (
	"github.com/pschlump/pluto/graph"
)

const unmatched = -1

// matching is the shared state of both algorithms: the bipartition,
// the mate array, the cardinality, and (filled by computeCover) the
// König minimum vertex cover.  Whites are one side of the partition
// (TwoColor's color false), blacks the other; which side is "white" is
// an implementation detail — swapping the two colors yields the mirror
// matching.
type matching struct {
	white       []bool // white[v]: v is on side A of the bipartition
	mate        []int  // mate[v]: the matched partner, or -1
	cardinality int
	inCover     []bool // König's minimum vertex cover
}

// BipartiteMatching is a maximum matching computed one augmenting path
// at a time (Kuhn's method, the direct augmenting-path form of the
// §6.4 algorithm; algs4's class of this name composes FordFulkerson
// instead — see the package README for the benchmark that picked
// direct integer augmentation).  Use NewBipartiteMatching to create
// one; the zero value reports the empty matching.
type BipartiteMatching struct {
	m matching
}

// HopcroftKarp is a maximum matching computed by the Hopcroft-Karp
// algorithm: each phase BFS-builds the layered graph of shortest
// augmenting paths and augments along a maximal vertex-disjoint set of
// them, which bounds the total at O(√V) phases.  Use NewHopcroftKarp
// to create one; the zero value reports the empty matching.
type HopcroftKarp struct {
	m matching
}

// newMatching validates the graph and snapshots the two-coloring, or
// panics naming the given constructor when the graph is not bipartite.
// It returns the prepared state with mate initialized to unmatched.
func newMatching(g *graph.Graph, ctor string) matching {
	if g == nil || g.V() == 0 {
		return matching{} // vacuously bipartite, empty matching
	}
	tc := graph.NewTwoColor(g)
	if !tc.IsBipartite() {
		panic("bipartite: " + ctor + ": the graph is not bipartite (it contains an odd cycle); a bipartite matching needs a bipartite graph")
	}
	n := g.V()
	m := matching{white: make([]bool, n), mate: make([]int, n)}
	for i := range m.mate {
		m.mate[i] = unmatched
	}
	for v := range n {
		c, _ := tc.Color(v)
		m.white[v] = !c // side A is TwoColor's color false: the DFS root's side
	}
	return m
}

// adjacency copies the graph's adjacency lists once so the algorithms
// work on a stable snapshot (the mst_ts per-vertex snapshot pattern,
// here for a plain graph — g.Adj returns a live iterator).
// Complexity is O(V+E).
func adjacency(g *graph.Graph) [][]int {
	adj := make([][]int, g.V())
	for v := range g.V() {
		for w := range g.Adj(v) {
			adj[v] = append(adj[v], w)
		}
	}
	return adj
}

// NewBipartiteMatching computes a maximum matching of g with the
// classic augmenting-path algorithm: every free vertex on one side is
// the start of a depth-first search for an alternating path to a free
// vertex on the other side; finding one flips the path's edges,
// growing the matching by one.  It panics if g is not bipartite; a nil
// or empty g yields the empty matching.
//
// algs4's BipartiteMatching routes the same problem through
// FordFulkerson on a unit-capacity flow network; this implementation
// augments directly on the integers, which the package benchmark
// measures at several times faster than composing maxflow.
// Complexity is O(V·E) time, O(V) space.
func NewBipartiteMatching(g *graph.Graph) *BipartiteMatching {
	m := newMatching(g, "NewBipartiteMatching")
	if len(m.mate) == 0 {
		return &BipartiteMatching{m: m}
	}
	adj := adjacency(g)

	// One alternating-path search per free white vertex.  visited
	// covers the BLACK vertices and is cleared for each attempt —
	// within one attempt a black vertex is considered once, which is
	// what makes the flip along the found path well-defined.
	visited := make([]bool, len(m.mate))
	for v := range m.mate {
		if !m.white[v] || m.mate[v] != unmatched {
			continue
		}
		clear(visited)
		if m.augment(v, adj, visited) {
			m.cardinality++
		}
	}
	m.computeCover(adj)
	return &BipartiteMatching{m: m}
}

// augment searches for an alternating path from the free white vertex
// v to a free black vertex, flipping it in place on success (Kuhn's
// method, recursive: the search depth is bounded by the number of
// white vertices).  Every black neighbor w is tried once: either it is
// free — the path completes and w pairs with v — or its current white
// partner can itself be re-augmented off w.
func (m *matching) augment(v int, adj [][]int, visited []bool) bool {
	for _, w := range adj[v] { // w is black: g is bipartite
		if visited[w] {
			continue
		}
		visited[w] = true
		if m.mate[w] == unmatched || m.augment(m.mate[w], adj, visited) {
			m.mate[v] = w
			m.mate[w] = v
			return true
		}
	}
	return false
}

// Cardinality returns the number of edges in the matching (which is
// also the size of the minimum vertex cover).
// Complexity is O(1).
func (bm *BipartiteMatching) Cardinality() int {
	if bm == nil {
		return 0
	}
	return bm.m.cardinality
}

// Mate returns the vertex matched to v, or -1 if v is unmatched (also
// for an out-of-range v or a nil receiver).
// Complexity is O(1).
func (bm *BipartiteMatching) Mate(v int) int {
	if bm == nil || v < 0 || v >= len(bm.m.mate) {
		return unmatched
	}
	return bm.m.mate[v]
}

// InMatching reports whether v is an endpoint of a matched edge.
// Complexity is O(1).
func (bm *BipartiteMatching) InMatching(v int) bool {
	return bm.Mate(v) != unmatched
}

// Edges returns the matched edges as [white, black] vertex pairs —
// the first element of each pair is on side A of the bipartition
// (TwoColor's color false; which side that is, is an implementation
// detail).  The slice is fresh; the caller may keep or mutate it.  A
// nil receiver reports nil.
// Complexity is O(V).
func (bm *BipartiteMatching) Edges() [][2]int {
	if bm == nil {
		return nil
	}
	return bm.m.edges()
}

// InMinVertexCover reports whether v is in the minimum vertex cover
// derived from the matching (König's theorem: the minimum number of
// vertices touching every edge equals the maximum matching's
// cardinality, and both objects expose the same number).
// Complexity is O(1).
func (bm *BipartiteMatching) InMinVertexCover(v int) bool {
	if bm == nil || v < 0 || v >= len(bm.m.inCover) {
		return false
	}
	return bm.m.inCover[v]
}

// MinVertexCover returns the minimum vertex cover as a slice of
// vertices (ascending order).  The slice is fresh.  A nil receiver
// reports nil.
// Complexity is O(V).
func (bm *BipartiteMatching) MinVertexCover() []int {
	if bm == nil {
		return nil
	}
	return bm.m.minVertexCover()
}

// edges collects the matched edges as [white, black] pairs.
func (m *matching) edges() [][2]int {
	if len(m.mate) == 0 {
		return nil
	}
	out := make([][2]int, 0, m.cardinality)
	for v := range m.mate {
		if m.white[v] && m.mate[v] != unmatched {
			out = append(out, [2]int{v, m.mate[v]})
		}
	}
	return out
}

// minVertexCover collects the cover vertices in ascending order.
func (m *matching) minVertexCover() []int {
	if len(m.inCover) == 0 {
		return nil
	}
	var out []int
	for v, in := range m.inCover {
		if in {
			out = append(out, v)
		}
	}
	return out
}

// computeCover derives the König minimum vertex cover from the
// (maximum) matching: Z is the set of vertices reachable from free
// white vertices along alternating paths — a non-matching edge from a
// white to a black, then the matching edge back to that black's white.
// The cover is (whites NOT in Z) ∪ (blacks in Z).
// Complexity is O(V+E).
func (m *matching) computeCover(adj [][]int) {
	n := len(m.mate)
	if n == 0 {
		return
	}
	inZ := make([]bool, n)
	stack := make([]int, 0, n)
	for v := range n {
		if m.white[v] && m.mate[v] == unmatched {
			inZ[v] = true
			stack = append(stack, v)
		}
	}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, w := range adj[v] { // w is black
			if w == m.mate[v] || inZ[w] {
				continue // the matching edge never alternates; visit each black once
			}
			inZ[w] = true
			if u := m.mate[w]; u != unmatched && !inZ[u] {
				inZ[u] = true
				stack = append(stack, u)
			}
		}
	}
	m.inCover = make([]bool, n)
	for v := range n {
		m.inCover[v] = m.white[v] != inZ[v] // white out of Z, black in Z
	}
}

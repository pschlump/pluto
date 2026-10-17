/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/graph"
)

// matchingAPI is the shared surface of the two implementations — the
// mst precedent of same-shape alternatives checked by one certificate.
type matchingAPI interface {
	Cardinality() int
	Mate(v int) int
	InMatching(v int) bool
	Edges() [][2]int
	InMinVertexCover(v int) bool
	MinVertexCover() []int
}

// checkCertificate verifies the complete König certificate of a
// reported matching, which PROVES it maximum: every matched pair is a
// real edge with symmetric mates, exactly 2·cardinality vertices are
// matched, the reported vertex cover touches every edge, and its size
// equals the cardinality (a cover of matching size proves no larger
// matching can exist — every matching edge needs its own cover
// vertex).  Also checks the derived views (InMatching, MinVertexCover
// slice agreement, ascending order).
func checkCertificate(t *testing.T, name string, g *graph.Graph, m matchingAPI) {
	t.Helper()
	card := m.Cardinality()

	// Matching validity and symmetry.
	edges := m.Edges()
	if len(edges) != card {
		t.Fatalf("%s: len(Edges) = %d, cardinality = %d", name, len(edges), card)
	}
	matched := 0
	for v := range g.V() {
		w := m.Mate(v)
		if w == -1 {
			if m.InMatching(v) {
				t.Fatalf("%s: InMatching(%d) true but Mate is -1", name, v)
			}
			continue
		}
		matched++
		if m.Mate(w) != v {
			t.Fatalf("%s: mate not symmetric: Mate(%d)=%d but Mate(%d)=%d", name, v, w, w, m.Mate(w))
		}
		if !g.HasEdge(v, w) {
			t.Fatalf("%s: matched pair %d-%d is not an edge of the graph", name, v, w)
		}
		if v == w {
			t.Fatalf("%s: self matched", name)
		}
	}
	if matched != 2*card {
		t.Fatalf("%s: %d matched vertices, want 2·%d", name, matched, card)
	}
	seen := map[[2]int]bool{}
	for _, e := range edges {
		seen[e] = true
	}
	if len(seen) != card {
		t.Fatalf("%s: duplicate edges in Edges(): %v", name, edges)
	}

	// Cover: every edge covered, size == cardinality (König).
	cover := m.MinVertexCover()
	if !slices.IsSorted(cover) {
		t.Fatalf("%s: MinVertexCover not ascending: %v", name, cover)
	}
	if len(cover) != card {
		t.Fatalf("%s: |MinVertexCover| = %d, cardinality = %d (König: equal)", name, len(cover), card)
	}
	inCoverSet := map[int]bool{}
	for _, v := range cover {
		inCoverSet[v] = true
		if !m.InMinVertexCover(v) {
			t.Fatalf("%s: MinVertexCover has %d but InMinVertexCover(%d) false", name, v, v)
		}
	}
	for v := range g.V() {
		if m.InMinVertexCover(v) != inCoverSet[v] {
			t.Fatalf("%s: InMinVertexCover(%d) disagrees with the slice", name, v)
		}
	}
	for v := range g.V() {
		for w := range g.Adj(v) {
			if !m.InMinVertexCover(v) && !m.InMinVertexCover(w) {
				t.Fatalf("%s: edge %d-%d uncovered by the vertex cover", name, v, w)
			}
		}
	}
}

// bruteForceMaxMatching enumerates every matching over the edge list
// (exponential — small graphs only) and returns the maximum size.
func bruteForceMaxMatching(n int, edges [][2]int) int {
	var best int
	used := make([]bool, n)
	var rec func(i, count int)
	rec = func(i, count int) {
		if count > best {
			best = count
		}
		if i == len(edges) {
			return
		}
		rec(i+1, count) // skip edge i
		v, w := edges[i][0], edges[i][1]
		if !used[v] && !used[w] {
			used[v], used[w] = true, true
			rec(i+1, count+1)
			used[v], used[w] = false, false
		}
	}
	rec(0, 0)
	return best
}

// refMatching is an independent reference: BFS-based augmenting paths
// with explicit parent-link flipping (structurally different from both
// the recursive DFS of BipartiteMatching and the layered phases of
// HopcroftKarp).  white[v] marks side A.
func refMatching(adj [][]int, white []bool) int {
	n := len(adj)
	mate := make([]int, n)
	for i := range mate {
		mate[i] = -1
	}
	card := 0
	for s := range n {
		if !white[s] || mate[s] != -1 {
			continue
		}
		// BFS over alternating paths from free white s.
		parent := make([]int, n) // parent[w] = the white that reached black w
		for i := range parent {
			parent[i] = -1
		}
		found := -1
		queue := []int{s}
		visitedW := make([]bool, n)
		visitedW[s] = true
		for i := 0; i < len(queue) && found == -1; i++ {
			v := queue[i]
			for _, w := range adj[v] {
				if parent[w] != -1 {
					continue
				}
				parent[w] = v
				if mate[w] == -1 {
					found = w
					break
				}
				u := mate[w]
				if !visitedW[u] {
					visitedW[u] = true
					queue = append(queue, u)
				}
			}
		}
		if found == -1 {
			continue
		}
		// Flip along the parent chain: black found <- its white <- ...
		w := found
		for w != -1 {
			v := parent[w]
			prev := mate[v]
			mate[v] = w
			mate[w] = v
			w = prev // v's old partner is the next black up the chain
		}
		card++
	}
	return card
}

// randomBipartite builds a random bipartite graph with sideA + sideB
// vertices: side A is 0..sideA-1, side B is sideA..sideA+sideB-1, and
// each cross edge appears with probability p.  Direct construction
// guarantees bipartiteness.
func randomBipartite(rng *rand.Rand, sideA, sideB int, p float64) *graph.Graph {
	g := graph.NewGraph(sideA + sideB)
	for v := range sideA {
		for w := sideA; w < sideA+sideB; w++ {
			if rng.Float64() < p {
				g.AddEdge(v, w)
			}
		}
	}
	return g
}

// randomAnyGraph builds an arbitrary random graph (any parity of
// cycle), keeping only the instances that happen to be bipartite —
// exercising the real graph.NewTwoColor composition, including
// multi-component colorings.
func randomAnyGraph(rng *rand.Rand, n int, p float64) *graph.Graph {
	for attempt := 0; attempt < 100; attempt++ {
		g := graph.NewGraph(n)
		for v := range n {
			for w := v + 1; w < n; w++ {
				if rng.Float64() < p {
					g.AddEdge(v, w)
				}
			}
		}
		if graph.NewTwoColor(g).IsBipartite() {
			return g
		}
	}
	return graph.NewGraph(1) // degenerate fallback, effectively unreachable
}

// crossCheck runs both implementations plus the reference on g and
// verifies agreement and the full certificate of each.
func crossCheck(t *testing.T, name string, g *graph.Graph) {
	t.Helper()
	tc := graph.NewTwoColor(g)
	white := make([]bool, g.V())
	for v := range g.V() {
		white[v], _ = tc.Color(v)
	}
	adj := make([][]int, g.V())
	for v := range g.V() {
		for w := range g.Adj(v) {
			adj[v] = append(adj[v], w)
		}
	}
	want := refMatching(adj, white)

	bm := NewBipartiteMatching(g)
	hk := NewHopcroftKarp(g)
	if bm.Cardinality() != want {
		t.Fatalf("%s: BipartiteMatching cardinality = %d, reference = %d", name, bm.Cardinality(), want)
	}
	if hk.Cardinality() != want {
		t.Fatalf("%s: HopcroftKarp cardinality = %d, reference = %d", name, hk.Cardinality(), want)
	}
	checkCertificate(t, name+"/BM", g, bm)
	checkCertificate(t, name+"/HK", g, hk)
}

func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for range 300 {
		crossCheck(t, "random-bipartite", randomBipartite(rng, 1+rng.IntN(30), 1+rng.IntN(30), rng.Float64()))
	}
	for range 200 {
		crossCheck(t, "random-any", randomAnyGraph(rng, 1+rng.IntN(14), rng.Float64()*0.4))
	}
}

func TestBruteForceOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 42))
	for iter := range 400 {
		// Small side counts so the edge list stays brute-forceable.
		a, b := 1+rng.IntN(5), 1+rng.IntN(5)
		g := randomBipartite(rng, a, b, rng.Float64())
		var edges [][2]int
		for v := range a {
			for w := range g.Adj(v) {
				if w >= a { // each cross edge once (from the side-A end)
					edges = append(edges, [2]int{v, w})
				}
			}
		}
		want := bruteForceMaxMatching(a+b, edges)
		if got := NewBipartiteMatching(g).Cardinality(); got != want {
			t.Fatalf("iter %d: BipartiteMatching = %d, brute force = %d (edges %v)", iter, got, want, edges)
		}
		if got := NewHopcroftKarp(g).Cardinality(); got != want {
			t.Fatalf("iter %d: HopcroftKarp = %d, brute force = %d (edges %v)", iter, got, want, edges)
		}
		crossCheck(t, "brute-force", g)
	}
}

// TestGreedyIsNeverLarger sanity-checks the bound from below: a greedy
// maximal matching is never larger than the reported maximum.
func TestGreedyIsNeverLarger(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 5))
	for range 200 {
		g := randomBipartite(rng, 1+rng.IntN(40), 1+rng.IntN(40), rng.Float64())
		used := make([]bool, g.V())
		greedy := 0
		for v := range g.V() {
			if used[v] {
				continue
			}
			for w := range g.Adj(v) {
				if !used[w] {
					used[v], used[w] = true, true
					greedy++
					break
				}
			}
		}
		bm := NewBipartiteMatching(g)
		if bm.Cardinality() < greedy {
			t.Fatalf("maximum %d below greedy %d", bm.Cardinality(), greedy)
		}
	}
}

// TestLargerGraphs exercises sizes where the phased algorithm's
// asymptotic advantage starts to matter, with sparse/square/dense
// shapes (certificate-checked, cross-checked against the reference).
func TestLargerGraphs(t *testing.T) {
	rng := rand.New(rand.NewPCG(1234, 5678))
	shapes := []struct {
		name         string
		sideA, sideB int
		p            float64
	}{
		{"sparse-square", 200, 200, 3.0 / 200},
		{"square", 200, 200, 0.1},
		{"dense-square", 200, 200, 0.5},
		{"tall", 300, 20, 0.5},
		{"wide", 20, 300, 0.5},
	}
	for _, s := range shapes {
		crossCheck(t, s.name, randomBipartite(rng, s.sideA, s.sideB, s.p))
	}
}

// TestChainGraphs: long paths (all one component, alternating colors,
// every other vertex free at the start) drive multi-edge augmenting
// paths in both algorithms.
func TestChainGraphs(t *testing.T) {
	for _, n := range []int{2, 3, 10, 101, 1000} {
		g := graph.NewGraph(n)
		for v := 1; v < n; v++ {
			g.AddEdge(v-1, v)
		}
		want := n / 2 // a path on n vertices matches floor(n/2) pairs
		if got := NewBipartiteMatching(g).Cardinality(); got != want {
			t.Fatalf("chain n=%d: BipartiteMatching = %d, want %d", n, got, want)
		}
		if got := NewHopcroftKarp(g).Cardinality(); got != want {
			t.Fatalf("chain n=%d: HopcroftKarp = %d, want %d", n, got, want)
		}
		crossCheck(t, "chain", g)
	}
}

// TestEmptySidesNoEdges: graphs whose entire one side has no edges at
// all (K_{n,0}).
func TestEmptySidesNoEdges(t *testing.T) {
	g := graph.NewGraph(10)
	for _, m := range []matchingAPI{NewBipartiteMatching(g), NewHopcroftKarp(g)} {
		if m.Cardinality() != 0 {
			t.Fatalf("edgeless graph: cardinality = %d, want 0", m.Cardinality())
		}
	}
}

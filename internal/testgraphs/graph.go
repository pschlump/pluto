/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Undirected generators — algs4 §4.1 GraphGenerator.java.  Every G they
// return is simple (no self-loops, no parallel edges) except the
// walk-based Eulerian pair, which is simple only in the multigraph
// sense.

package testgraphs

import (
	"fmt"
	"math/rand"
)

// Simple returns a random simple graph on v vertices with exactly e
// edges, by rejection sampling: pick two distinct vertices, retry while
// the pair is already present.  Panics when e exceeds the v·(v−1)/2
// possible edges or is negative, or v < 1.
// Complexity is O(e) expected draws (slower as e approaches the maximum).
func Simple(r *rand.Rand, v, e int) G {
	requireRand("Simple", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Simple called with v=%d, need v >= 1", v))
	}
	if e > v*(v-1)/2 {
		panic(fmt.Sprintf("testgraphs: Simple called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)/2))
	}
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: Simple called with e=%d, need e >= 0", e))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x != y {
			addEdge(&edges, seen, uKey(x, y), x, y)
		}
	}
	return G{V: v, Edges: edges}
}

// ErdosRenyi returns the G(n, p) random graph — every one of the
// v·(v−1)/2 pairs included independently with probability p (algs4's
// simple(V, p)).  Panics when p is outside [0,1] or v < 1.
// Complexity is Θ(v²).
func ErdosRenyi(r *rand.Rand, v int, p float64) G {
	requireRand("ErdosRenyi", r)
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("testgraphs: ErdosRenyi called with p=%v, need 0 <= p <= 1", p))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: ErdosRenyi called with v=%d, need v >= 1", v))
	}
	var edges [][2]int
	for x := 0; x < v; x++ {
		for y := x + 1; y < v; y++ {
			if r.Float64() < p {
				edges = append(edges, [2]int{x, y})
			}
		}
	}
	return G{V: v, Edges: edges}
}

// Complete returns the complete graph on v vertices — every pair an
// edge.  Built directly (algs4 routes through simple(V, 1.0); building
// the pairs directly is deterministic and consumes no randomness).
// Panics when v < 1.
// Complexity is Θ(v²).
func Complete(v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Complete called with v=%d, need v >= 1", v))
	}
	var edges [][2]int
	for x := 0; x < v; x++ {
		for y := x + 1; y < v; y++ {
			edges = append(edges, [2]int{x, y})
		}
	}
	return G{V: v, Edges: edges}
}

// Path returns the path graph 0-1-2-…-(v−1), on shuffled labels when r
// is non-nil (nil keeps the identity order — the deterministic form).
// Panics when v < 1.
// Complexity is O(v).
func Path(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Path called with v=%d, need v >= 1", v))
	}
	order := shuffled(r, v)
	return G{V: v, Edges: pathEdges(order)}
}

// Cycle returns the cycle graph — the path plus the closing edge from
// the last vertex to the first.  A cycle on 1 vertex is a self loop and
// on 2 vertices a parallel pair; both are legal input where the
// generator is documented multigraph-capable.  Panics when v < 1.
// Complexity is O(v).
func Cycle(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Cycle called with v=%d, need v >= 1", v))
	}
	order := shuffled(r, v)
	edges := pathEdges(order)
	return G{V: v, Edges: append(edges, [2]int{order[v-1], order[0]})}
}

// Star returns the star graph: the hub (label 0 in identity order)
// connected to every other vertex.  Panics when v < 1.
// Complexity is O(v).
func Star(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Star called with v=%d, need v >= 1", v))
	}
	order := shuffled(r, v)
	edges := make([][2]int, 0, v-1)
	for _, w := range order[1:] {
		edges = append(edges, [2]int{order[0], w})
	}
	return G{V: v, Edges: edges}
}

// Wheel returns the wheel graph: a hub connected to every vertex of a
// cycle on the remaining v−1 vertices (algs4 requires v ≥ 2; the wheel
// on 2 is the hub plus a self-looped rim vertex).  Panics when v < 2.
// Complexity is O(v).
func Wheel(r *rand.Rand, v int) G {
	if v < 2 {
		panic(fmt.Sprintf("testgraphs: Wheel called with v=%d, need v >= 2", v))
	}
	order := shuffled(r, v)
	edges := make([][2]int, 0, 2*v-2)
	for i := 1; i < v-1; i++ {
		edges = append(edges, [2]int{order[i], order[i+1]})
	}
	edges = append(edges, [2]int{order[v-1], order[1]}) // close the rim
	for i := 1; i < v; i++ {
		edges = append(edges, [2]int{order[0], order[i]}) // the spokes
	}
	return G{V: v, Edges: edges}
}

// BinaryTree returns the complete-binary-tree shape: vertex i ≥ 1
// connected to its array-heap parent (i−1)/2 (on shuffled labels when r
// is non-nil).  Panics when v < 1.
// Complexity is O(v).
func BinaryTree(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: BinaryTree called with v=%d, need v >= 1", v))
	}
	order := shuffled(r, v)
	edges := make([][2]int, 0, v-1)
	for i := 1; i < v; i++ {
		edges = append(edges, [2]int{order[i], order[(i-1)/2]})
	}
	return G{V: v, Edges: edges}
}

// Tree returns a uniformly random labeled tree on v vertices, built
// from a random Prüfer sequence of length v−2 (the algs4 algorithm:
// degree[i] = 1 + Prüfer count of i, repeatedly delete the minimum
// degree-1 vertex and attach it to the next Prüfer entry).  A tree on 1
// vertex has no edges.  Requires r (the Prüfer sequence is the
// randomness).  Panics when v < 1.
// Complexity is O(v log v).
func Tree(r *rand.Rand, v int) G {
	requireRand("Tree", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Tree called with v=%d, need v >= 1", v))
	}
	if v == 1 {
		return G{V: 1}
	}
	prufer := make([]int, v-2)
	for i := range prufer {
		prufer[i] = r.Intn(v)
	}
	degree := make([]int, v)
	for i := range degree {
		degree[i] = 1
	}
	for _, p := range prufer {
		degree[p]++
	}
	h := &intHeap{}
	for x, d := range degree {
		if d == 1 {
			h.push(x)
		}
	}
	edges := make([][2]int, 0, v-1)
	for _, p := range prufer {
		x := h.pop()
		edges = append(edges, [2]int{x, p})
		degree[x]--
		if degree[p]--; degree[p] == 1 {
			h.push(p)
		}
	}
	a, b := h.pop(), h.pop()
	return G{V: v, Edges: append(edges, [2]int{a, b})}
}

// Bipartite returns a random bipartite graph with exactly e edges
// across the two sides — side A is [0, v1), side B is [v1, v1+v2).  The
// contiguous sides are the deviation from algs4 (which shuffles all
// labels): tests get the 2-coloring for free.  Panics when e exceeds
// v1·v2 or is negative, or either side is empty.
// Complexity is O(e) expected draws.
func Bipartite(r *rand.Rand, v1, v2, e int) B {
	requireRand("Bipartite", r)
	if v1 < 1 || v2 < 1 {
		panic(fmt.Sprintf("testgraphs: Bipartite called with v1=%d, v2=%d, need both >= 1", v1, v2))
	}
	if e > v1*v2 {
		panic(fmt.Sprintf("testgraphs: Bipartite called with e=%d, too many for %d×%d sides", e, v1, v2))
	}
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: Bipartite called with e=%d, need e >= 0", e))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for len(edges) < e {
		x := r.Intn(v1)
		y := v1 + r.Intn(v2)
		addEdge(&edges, seen, uKey(x, y), x, y)
	}
	return B{G: G{V: v1 + v2, Edges: edges}, V1: v1}
}

// BipartiteProb is Bipartite in the G(n, p) form: each of the v1·v2
// cross pairs is included independently with probability p.  Panics
// when p is outside [0,1] or either side is empty.
// Complexity is Θ(v1·v2).
func BipartiteProb(r *rand.Rand, v1, v2 int, p float64) B {
	requireRand("BipartiteProb", r)
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("testgraphs: BipartiteProb called with p=%v, need 0 <= p <= 1", p))
	}
	if v1 < 1 || v2 < 1 {
		panic(fmt.Sprintf("testgraphs: BipartiteProb called with v1=%d, v2=%d, need both >= 1", v1, v2))
	}
	var edges [][2]int
	for x := 0; x < v1; x++ {
		for y := v1; y < v1+v2; y++ {
			if r.Float64() < p {
				edges = append(edges, [2]int{x, y})
			}
		}
	}
	return B{G: G{V: v1 + v2, Edges: edges}, V1: v1}
}

// CompleteBipartite returns the complete bipartite graph K(v1, v2) —
// every cross pair.  Built directly (algs4 routes through
// bipartite(V1, V2, V1*V2)), so it consumes no randomness.  Panics
// when either side is empty.
// Complexity is Θ(v1·v2).
func CompleteBipartite(v1, v2 int) B {
	if v1 < 1 || v2 < 1 {
		panic(fmt.Sprintf("testgraphs: CompleteBipartite called with v1=%d, v2=%d, need both >= 1", v1, v2))
	}
	var edges [][2]int
	for x := 0; x < v1; x++ {
		for y := v1; y < v1+v2; y++ {
			edges = append(edges, [2]int{x, y})
		}
	}
	return B{G: G{V: v1 + v2, Edges: edges}, V1: v1}
}

// EulerianCycle returns a graph that HAS an Eulerian cycle: a random
// walk of e edges on v vertices, closed from the last vertex back to
// the first, so every touched vertex has even degree and the touched
// part is connected.  Isolated vertices (never picked by the walk) are
// legal.  Parallel edges and self loops are legal (the walk can revisit
// a pair or stay put).  Panics when e < 1 or v < 1.
// Complexity is O(e).
func EulerianCycle(r *rand.Rand, v, e int) G {
	requireRand("EulerianCycle", r)
	if e < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianCycle called with e=%d, an Eulerian cycle must have at least one edge", e))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianCycle called with v=%d, need v >= 1", v))
	}
	return closedWalk(r, v, e)
}

// EulerianPath returns a graph that HAS an Eulerian path: a random open
// walk of e edges on e+1 vertex picks.  The two walk endpoints have odd
// degree (or all even, if the walk happens to start and end on the same
// vertex — then it has an Eulerian cycle too).  Panics when e < 0 or
// v < 1.
// Complexity is O(e).
func EulerianPath(r *rand.Rand, v, e int) G {
	requireRand("EulerianPath", r)
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: EulerianPath called with e=%d, need e >= 0", e))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianPath called with v=%d, need v >= 1", v))
	}
	vertices := make([]int, e+1)
	for i := range vertices {
		vertices[i] = r.Intn(v)
	}
	return walkEdges(vertices)
}

// Grid returns the rows×cols grid graph (algs4 has no grid generator;
// this is the backlog's addition): vertex (i, j) is i·cols + j,
// orthogonally adjacent cells are connected.  A grid is always
// bipartite — (i+j)%2 is the 2-coloring.  Deterministic; no rand.
// Panics when rows or cols is < 1.
// Complexity is O(rows·cols).
func Grid(rows, cols int) G {
	if rows < 1 || cols < 1 {
		panic(fmt.Sprintf("testgraphs: Grid called with rows=%d, cols=%d, need both >= 1", rows, cols))
	}
	var edges [][2]int
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			v := i*cols + j
			if j+1 < cols {
				edges = append(edges, [2]int{v, v + 1})
			}
			if i+1 < rows {
				edges = append(edges, [2]int{v, v + cols})
			}
		}
	}
	return G{V: rows * cols, Edges: edges}
}

// pathEdges chains consecutive entries of order.
func pathEdges(order []int) [][2]int {
	edges := make([][2]int, 0, len(order)-1)
	for i := 0; i+1 < len(order); i++ {
		edges = append(edges, [2]int{order[i], order[i+1]})
	}
	return edges
}

// walkEdges chains consecutive entries of vertices (an open walk).
func walkEdges(vertices []int) G {
	v := 0
	for _, x := range vertices {
		if x > v {
			v = x
		}
	}
	edges := make([][2]int, 0, len(vertices)-1)
	for i := 0; i+1 < len(vertices); i++ {
		edges = append(edges, [2]int{vertices[i], vertices[i+1]})
	}
	return G{V: v + 1, Edges: edges}
}

// closedWalk is EulerianCycle's construction: e random vertex picks
// chained (e−1 edges), plus the closing edge back to the first — e
// edges total, even degree everywhere the walk touches.
func closedWalk(r *rand.Rand, v, e int) G {
	vertices := make([]int, e)
	for i := range vertices {
		vertices[i] = r.Intn(v)
	}
	g := walkEdges(vertices)
	return G{V: g.V, Edges: append(g.Edges, [2]int{vertices[e-1], vertices[0]})}
}

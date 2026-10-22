/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Directed generators — algs4 §4.2 DigraphGenerator.java.  The names
// carry the Digraph suffix (or are inherently directed: DAG, Strong,
// Tournament) because a directed edge list is NOT interchangeable with
// the undirected ones: pairs here are ordered (v → w), and simple-form
// generators dedup on the ordered pair.

package testgraphs

import (
	"fmt"
	"math/rand"
)

// SimpleDigraph returns a random simple digraph on v vertices with
// exactly e edges — ordered pairs, no self loops, no parallel edges —
// by rejection sampling.  Panics when e exceeds the v·(v−1) possible
// ordered edges or is negative, or v < 1.
// Complexity is O(e) expected draws.
func SimpleDigraph(r *rand.Rand, v, e int) G {
	requireRand("SimpleDigraph", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: SimpleDigraph called with v=%d, need v >= 1", v))
	}
	if e > v*(v-1) {
		panic(fmt.Sprintf("testgraphs: SimpleDigraph called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)))
	}
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: SimpleDigraph called with e=%d, need e >= 0", e))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x != y {
			addEdge(&edges, seen, [2]int{x, y}, x, y)
		}
	}
	return G{V: v, Edges: edges}
}

// ErdosRenyiDigraph is the G(n, p) model on ordered pairs: every pair
// x ≠ y included independently with probability p (algs4's
// simple(V, p)).  Panics when p is outside [0,1] or v < 1.
// Complexity is Θ(v²).
func ErdosRenyiDigraph(r *rand.Rand, v int, p float64) G {
	requireRand("ErdosRenyiDigraph", r)
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("testgraphs: ErdosRenyiDigraph called with p=%v, need 0 <= p <= 1", p))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: ErdosRenyiDigraph called with v=%d, need v >= 1", v))
	}
	var edges [][2]int
	for x := 0; x < v; x++ {
		for y := 0; y < v; y++ {
			if x != y && r.Float64() < p {
				edges = append(edges, [2]int{x, y})
			}
		}
	}
	return G{V: v, Edges: edges}
}

// CompleteDigraph returns the complete digraph — every ordered pair of
// distinct vertices.  Panics when v < 1.
// Complexity is Θ(v²).
func CompleteDigraph(v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: CompleteDigraph called with v=%d, need v >= 1", v))
	}
	var edges [][2]int
	for x := 0; x < v; x++ {
		for y := 0; y < v; y++ {
			if x != y {
				edges = append(edges, [2]int{x, y})
			}
		}
	}
	return G{V: v, Edges: edges}
}

// PathDigraph returns the directed path along the vertex order —
// 0→1→2→…→(v−1) in identity order, shuffled labels when r is non-nil
// (algs4 shuffles unconditionally; nil is this package's deterministic
// form, and then v−1 is the unique sink).  Panics when v < 1.
// Complexity is O(v).
func PathDigraph(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: PathDigraph called with v=%d, need v >= 1", v))
	}
	return G{V: v, Edges: pathEdges(shuffled(r, v))}
}

// CycleDigraph returns the directed cycle along the vertex order — the
// path plus the closing edge back to the first vertex.  The identity
// form (r nil) makes every vertex reachable from every vertex (one
// strong component).  Panics when v < 1.
// Complexity is O(v).
func CycleDigraph(r *rand.Rand, v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: CycleDigraph called with v=%d, need v >= 1", v))
	}
	order := shuffled(r, v)
	edges := pathEdges(order)
	return G{V: v, Edges: append(edges, [2]int{order[v-1], order[0]})}
}

// BinaryTreeDigraph returns the complete-binary-tree shape with every
// edge pointing FROM a node TO its array-heap parent ((i−1)/2) — the
// algs4 binaryTree digraph, a DAG rooted at vertex 0 in identity order.
// Panics when v < 1.
// Complexity is O(v).
func BinaryTreeDigraph(v int) G {
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: BinaryTreeDigraph called with v=%d, need v >= 1", v))
	}
	edges := make([][2]int, 0, v-1)
	for i := 1; i < v; i++ {
		edges = append(edges, [2]int{i, (i - 1) / 2})
	}
	return G{V: v, Edges: edges}
}

// DAG returns a random directed acyclic graph with exactly e edges.
// DEVIATION from algs4 (which shuffles a vertex order and adds edges
// along it): this generator uses the identity order, so every edge goes
// x → y with x < y — which makes 0, 1, …, v−1 itself a topological
// order that tests can verify against (any topological order of the
// generated digraph must respect every edge).  Panics when e exceeds
// v·(v−1)/2 or is negative, or v < 1.
// Complexity is O(e) expected draws.
func DAG(r *rand.Rand, v, e int) G {
	requireRand("DAG", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: DAG called with v=%d, need v >= 1", v))
	}
	if e > v*(v-1)/2 {
		panic(fmt.Sprintf("testgraphs: DAG called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)/2))
	}
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: DAG called with e=%d, need e >= 0", e))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x < y {
			addEdge(&edges, seen, [2]int{x, y}, x, y)
		}
	}
	return G{V: v, Edges: edges}
}

// Tournament returns the random tournament: exactly one directed edge
// between every pair of distinct vertices, its direction a fair coin
// flip.  Panics when v < 1.
// Complexity is Θ(v²).
func Tournament(r *rand.Rand, v int) G {
	requireRand("Tournament", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Tournament called with v=%d, need v >= 1", v))
	}
	var edges [][2]int
	for x := 0; x < v; x++ {
		for y := x + 1; y < v; y++ {
			if r.Intn(2) == 0 {
				edges = append(edges, [2]int{x, y})
			} else {
				edges = append(edges, [2]int{y, x})
			}
		}
	}
	return G{V: v, Edges: edges}
}

// RootedInDAG returns a random rooted-in DAG: every vertex can REACH
// the root, which in the identity order (the documented deviation from
// algs4's shuffle) is vertex v−1.  Built as algs4 builds it — a
// spanning in-tree (each vertex x < v−1 points at a uniformly random
// later vertex) plus rejection-sampled forward edges x → y, x < y.
// Panics when e < v−1 (the tree needs that many), e exceeds v·(v−1)/2,
// or v < 1.
// Complexity is O(e) expected draws.
func RootedInDAG(r *rand.Rand, v, e int) G {
	requireRand("RootedInDAG", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: RootedInDAG called with v=%d, need v >= 1", v))
	}
	if e < v-1 {
		panic(fmt.Sprintf("testgraphs: RootedInDAG called with e=%d, too few for v=%d (the in-tree alone needs %d)", e, v, v-1))
	}
	if e > v*(v-1)/2 {
		panic(fmt.Sprintf("testgraphs: RootedInDAG called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)/2))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for x := 0; x < v-1; x++ { // the in-tree: x points at a random later vertex
		y := x + 1 + r.Intn(v-1-x)
		addEdge(&edges, seen, [2]int{x, y}, x, y)
	}
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x < y {
			addEdge(&edges, seen, [2]int{x, y}, x, y)
		}
	}
	return G{V: v, Edges: edges}
}

// RootedOutDAG returns a random rooted-out DAG: every vertex is
// reachable FROM the root, which in the identity order is vertex 0.
// The mirror of RootedInDAG (each vertex x > 0 gets an edge from a
// random earlier vertex, plus forward edges).  Panics on the same
// bounds as RootedInDAG.
// Complexity is O(e) expected draws.
func RootedOutDAG(r *rand.Rand, v, e int) G {
	requireRand("RootedOutDAG", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: RootedOutDAG called with v=%d, need v >= 1", v))
	}
	if e < v-1 {
		panic(fmt.Sprintf("testgraphs: RootedOutDAG called with e=%d, too few for v=%d (the out-tree alone needs %d)", e, v, v-1))
	}
	if e > v*(v-1)/2 {
		panic(fmt.Sprintf("testgraphs: RootedOutDAG called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)/2))
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for x := 1; x < v; x++ { // the out-tree: a random earlier vertex points at x
		y := r.Intn(x)
		addEdge(&edges, seen, [2]int{y, x}, y, x)
	}
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x < y {
			addEdge(&edges, seen, [2]int{x, y}, x, y)
		}
	}
	return G{V: v, Edges: edges}
}

// RootedInTree is RootedInDAG with the minimum e = v−1: a tree in
// which every vertex reaches the root v−1.
// Complexity is O(v).
func RootedInTree(r *rand.Rand, v int) G {
	return RootedInDAG(r, v, v-1)
}

// RootedOutTree is RootedOutDAG with the minimum e = v−1: a tree
// reachable from the root 0.
// Complexity is O(v).
func RootedOutTree(r *rand.Rand, v int) G {
	return RootedOutDAG(r, v, v-1)
}

// EulerianCycleDigraph returns a digraph that HAS a directed Eulerian
// cycle: a closed random walk, so in-degree = out-degree at every
// touched vertex and the touched part is strongly connected.  Parallel
// edges and self loops are legal.  Panics when e < 1 or v < 1.
// Complexity is O(e).
func EulerianCycleDigraph(r *rand.Rand, v, e int) G {
	requireRand("EulerianCycleDigraph", r)
	if e < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianCycleDigraph called with e=%d, an Eulerian cycle must have at least one edge", e))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianCycleDigraph called with v=%d, need v >= 1", v))
	}
	return closedWalk(r, v, e)
}

// EulerianPathDigraph returns a digraph that HAS a directed Eulerian
// path: an open random walk — at every touched vertex in-degree equals
// out-degree except the walk's two endpoints (out one extra at the
// start, in one extra at the end), unless start and end coincide, in
// which case it has a cycle.  Panics when e < 0 or v < 1.
// Complexity is O(e).
func EulerianPathDigraph(r *rand.Rand, v, e int) G {
	requireRand("EulerianPathDigraph", r)
	if e < 0 {
		panic(fmt.Sprintf("testgraphs: EulerianPathDigraph called with e=%d, need e >= 0", e))
	}
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: EulerianPathDigraph called with v=%d, need v >= 1", v))
	}
	vertices := make([]int, e+1)
	for i := range vertices {
		vertices[i] = r.Intn(v)
	}
	return walkEdges(vertices)
}

// Strong returns a random digraph with a KNOWN strong-component
// structure ("tinyDG-style"): c labeled components, each built by
// superimposing a rooted-in and a rooted-out tree over the label's
// (shuffled) vertices — 2(k−1) edges for a k-vertex component — plus
// rejection-sampled extra edges v → w allowed only when
// label[v] <= label[w] and new.  Cross-component edges all point from
// smaller labels to larger, so no path ever returns: two vertices are
// strongly connected EXACTLY when their labels are equal, which is the
// oracle SCC tests verify against.
//
// DEVIATION from algs4 (which draws every label uniformly, so a label
// can go unused — merging two groups and overshooting the exact edge
// count): the first c vertices of a shuffled order pin the c labels,
// so all c are always used, the edge count is exactly e, and the SCC
// count is exactly c.
// Panics when c < 1 or c > v, e ≤ 2(v−c) (the per-component trees need
// that many) or e > v·(v−1)/2, or v < 1.
// Complexity is O(e) expected draws.
func Strong(r *rand.Rand, v, e, c int) Labeled {
	requireRand("Strong", r)
	if v < 1 {
		panic(fmt.Sprintf("testgraphs: Strong called with v=%d, need v >= 1", v))
	}
	if c < 1 || c > v {
		panic(fmt.Sprintf("testgraphs: Strong called with c=%d, need 1 <= c <= v=%d", c, v))
	}
	if e <= 2*(v-c) {
		panic(fmt.Sprintf("testgraphs: Strong called with e=%d, need e > %d (the per-component in/out trees alone need that many)", e, 2*(v-c)))
	}
	if e > v*(v-1)/2 {
		panic(fmt.Sprintf("testgraphs: Strong called with e=%d, too many for v=%d (max %d)", e, v, v*(v-1)/2))
	}

	label := make([]int, v)
	order := shuffled(r, v)
	for l := 0; l < c; l++ {
		label[order[l]] = l // pin every label — all c are used
	}
	for _, x := range order[c:] {
		label[x] = r.Intn(c)
	}
	edges := make([][2]int, 0, e)
	seen := make(map[[2]int]bool, e)
	for l := 0; l < c; l++ {
		var group []int
		for x, gl := range label {
			if gl == l {
				group = append(group, x)
			}
		}
		if len(group) < 2 {
			continue
		}
		r.Shuffle(len(group), func(i, j int) { group[i], group[j] = group[j], group[i] })
		// Superimpose a rooted-in tree and a rooted-out tree over the
		// group (root = the last vertex): each non-root vertex x gets an
		// edge to and from a uniformly random LATER vertex in the group,
		// so the whole group is strongly connected — 2(k−1) edges.
		for i := 0; i+1 < len(group); i++ {
			w := group[i+1+r.Intn(len(group)-1-i)]
			addEdge(&edges, seen, [2]int{group[i], w}, group[i], w)
			addEdge(&edges, seen, [2]int{w, group[i]}, w, group[i])
		}
	}
	for len(edges) < e {
		x, y := r.Intn(v), r.Intn(v)
		if x != y && label[x] <= label[y] {
			addEdge(&edges, seen, [2]int{x, y}, x, y)
		}
	}
	return Labeled{G: G{V: v, Edges: edges}, Label: label}
}

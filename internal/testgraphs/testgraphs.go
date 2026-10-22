/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package testgraphs ports the algs4 graph and digraph generators
// (Sedgewick & Wayne, Algorithms 4th ed., §4.1 GraphGenerator.java and
// §4.2 DigraphGenerator.java) as shared test helpers — not a public
// data-structure package (the internal/ path keeps it out of the
// module's public API).
//
// Three design rules, so that any package's tests can use these:
//
//   - Generators return EDGE LISTS, never pluto structures.  The
//     package imports nothing from pluto, so even in-package tests
//     (package graph, package digraph, …) can import it without an
//     import cycle.  A test builds its graph in three lines:
//     g := NewGraph(gen.V); for _, e := range gen.Edges { g.AddEdge(e[0], e[1]) }.
//
//   - Randomness comes from a caller-supplied *rand.Rand — tests pass a
//     fixed-seed source and stay deterministic.  Where algs4's only
//     randomness is a vertex-label shuffle (Path, Cycle, Star, Wheel,
//     BinaryTree, …), a nil r means "identity order", which makes those
//     generators deterministic without a seed.
//
//   - Documented deviations from algs4 make the output a better TEST
//     ORACLE, and each one is noted on the generator: Bipartite keeps
//     the two sides contiguous ([0,V1) vs [V1,V)) instead of shuffling
//     labels, so tests know the 2-coloring; DAG and the rooted DAGs use
//     the identity order, so 0,1,…,V-1 is a topological order and the
//     roots are the endpoints; Complete and CompleteBipartite are built
//     directly rather than via simple(V,1.0), consuming no randomness.
//
// Validation follows algs4: impossible arguments panic with a message
// naming the generator and the problem (a wrong generator call is a
// test bug, and failing loudly beats a test that quietly tests nothing).
package testgraphs

import (
	"fmt"
	"math/rand"
)

// G is a generated graph: V vertices numbered 0..V-1 and an edge list.
// For the undirected generators each pair is an unordered pair (mostly
// stored sorted, v < w); for the directed generators the pair is an
// ordered edge (v → w).  Duplicate pairs are possible exactly where the
// generator documents them (parallel edges are legal graph input).
type G struct {
	V     int
	Edges [][2]int
}

// B is a generated bipartite graph.  Side A is the vertices
// [0, V1) and side B is [V1, V) with V = V1 + V2 — the contiguous
// partition is this package's documented deviation from algs4's
// shuffled labels, and it is the known 2-coloring oracle.
type B struct {
	G
	V1 int
}

// Labeled is a generated digraph whose vertices carry generated
// strong-component labels: two vertices are strongly connected exactly
// when their labels are equal (Strong builds it that way — cross-label
// edges all point from smaller labels to larger, so no path returns).
// The labels 0..c−1 are all used, so the distinct labels form the SCC
// partition oracle and c is the SCC count.
type Labeled struct {
	G
	Label []int
}

// requireRand rejects a nil source for the generators whose
// construction needs real randomness, not just a label shuffle.
func requireRand(caller string, r *rand.Rand) {
	if r == nil {
		panic(fmt.Sprintf("testgraphs: %s called with a nil rand — pass a fixed-seed *rand.Rand (rejection sampling needs it)", caller))
	}
}

// shuffled returns the identity 0..n-1 when r is nil (the deterministic
// form) and a Fisher-Yates shuffle of it otherwise.
func shuffled(r *rand.Rand, n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if r != nil {
		r.Shuffle(n, func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	return order
}

// uKey canonicalizes an unordered pair to its dedup key.
func uKey(v, w int) [2]int {
	if w < v {
		v, w = w, v
	}
	return [2]int{v, w}
}

// addEdge appends (v, w) to edges when key is new in seen, reporting
// whether anything was added — the rejection-sampling step shared by
// every simple-graph generator.
func addEdge(edges *[][2]int, seen map[[2]int]bool, key [2]int, v, w int) bool {
	if seen[key] {
		return false
	}
	seen[key] = true
	*edges = append(*edges, [2]int{v, w})
	return true
}

// intHeap is the minimum-first int queue Tree's Prüfer construction
// deletes its degree-1 leaves through (container/heap — stdlib only).
type intHeap struct {
	data []int
}

func (h *intHeap) push(x int) {
	h.data = append(h.data, x)
	i := len(h.data) - 1
	for i > 0 {
		p := (i - 1) / 2
		if h.data[p] <= h.data[i] {
			break
		}
		h.data[p], h.data[i] = h.data[i], h.data[p]
		i = p
	}
}

func (h *intHeap) pop() int {
	top := h.data[0]
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	i := 0
	for {
		l, r := 2*i+1, 2*i+2
		m := i
		if l < len(h.data) && h.data[l] < h.data[m] {
			m = l
		}
		if r < len(h.data) && h.data[r] < h.data[m] {
			m = r
		}
		if m == i {
			return top
		}
		h.data[i], h.data[m] = h.data[m], h.data[i]
		i = m
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph

import (
	"encoding/json"
	"fmt"
	"slices"
)

// graphJSON is the wire form of a Graph: the vertex list — always the
// contiguous range 0..n-1, since the vertices ARE the indices — and the
// edge list, each edge a [v, w] pair in the package's usual notation.
type graphJSON struct {
	Vertices []int    `json:"vertices"`
	Edges    [][2]int `json:"edges"`
}

// MarshalJSON implements json.Marshaler so a Graph can be used directly
// with the encoding/json package.  The graph is encoded as a JSON object
// with the vertex list ("vertices") and the edge list ("edges"), e.g.
// {"vertices":[0,1,2],"edges":[[0,1],[1,2]]}.
//
// Each edge is emitted exactly once (the two adjacency entries of a
// self-loop count as one edge, matching E), as [v, w] with v <= w, and
// the edge list is sorted lexicographically — the encoding depends only
// on the edge multiset, not on insertion history, and a round-tripped
// graph re-marshals byte-identically.  Errors from the json package are
// returned unchanged.
//
// An edgeless graph still lists every vertex; a zero-value graph encodes
// as {"vertices":[],"edges":[]}.  A direct call on a nil graph encodes
// as {} (the "nil behaves as an empty graph" read contract); note that
// json.Marshal on a nil *Graph never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(V+E log E).
func (g *Graph) MarshalJSON() ([]byte, error) {
	if g == nil {
		return []byte("{}"), nil
	}
	doc := graphJSON{
		Vertices: make([]int, g.n),
		Edges:    make([][2]int, 0, g.e),
	}
	for v := range doc.Vertices {
		doc.Vertices[v] = v
	}
	for v := 0; v < g.n; v++ {
		selfLoop := 0
		for _, w := range g.adj[v] {
			switch {
			case w > v:
				doc.Edges = append(doc.Edges, [2]int{v, w})
			case w == v:
				selfLoop++
				if selfLoop%2 == 1 { // one edge per self-loop pair of entries
					doc.Edges = append(doc.Edges, [2]int{v, v})
				}
			}
		}
	}
	slices.SortFunc(doc.Edges, func(a, b [2]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return a[1] - b[1]
	})
	return json.Marshal(doc)
}

// UnmarshalJSON implements json.Unmarshaler so a Graph can be used
// directly with the encoding/json package.  data must be the JSON object
// MarshalJSON produces (or null); the decoded vertices and edges replace
// the current contents of the graph — the vertex count becomes the
// length of the vertex list and the edges are re-added in document
// order, so decoding MarshalJSON output rebuilds the adjacency lists in
// the canonical sorted order and preserves the edge multiset exactly
// (self-loop and parallel-edge counts included).  A graph
// has no constructor-set state to preserve, so a zero-value graph is
// simply rebuilt in place.
//
// The document is decoded and validated before the graph is touched: a
// decode error (malformed JSON, a non-object document, wrong field
// types), a vertex list that is not the contiguous range 0..n-1, or an
// edge endpoint outside the vertex list is returned as an error and
// leaves the graph untouched.
//
// Unmarshaling stores vertices and edges, so it follows the insert
// contract: data that would store a vertex or an edge into a nil graph
// panics with the standard insert-family message.  null, {}, and a
// document with no vertices and no edges clear the graph (to the zero
// value, the empty graph with no vertices) and are tolerated everywhere —
// they store nothing.
// Complexity is O(V+E) plus the cost of decoding the document.
func (g *Graph) UnmarshalJSON(data []byte) error {
	var doc graphJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	// Validate before touching the graph: the vertices must be the
	// contiguous range 0..n-1 (the vertices ARE the indices) and every
	// edge endpoint must be an in-range vertex.
	for i, v := range doc.Vertices {
		if v != i {
			return fmt.Errorf("graph: UnmarshalJSON: vertices must be 0..n-1 in order, got %v", doc.Vertices)
		}
	}
	n := len(doc.Vertices)
	for _, e := range doc.Edges {
		if e[0] < 0 || e[0] >= n || e[1] < 0 || e[1] >= n {
			return fmt.Errorf("graph: UnmarshalJSON: edge %v is out of range for %d vertices", e, n)
		}
	}

	// The insert contract only fires when an element would actually be
	// stored (the Concat precedent).
	if len(doc.Vertices) > 0 || len(doc.Edges) > 0 {
		if g == nil {
			panic("graph: UnmarshalJSON called on a nil graph")
		}
	}
	if g == nil {
		return nil // null or empty: nothing to store, nothing to clear
	}

	adj := make([][]int, n)
	e := 0
	for _, edge := range doc.Edges {
		v, w := edge[0], edge[1]
		adj[v] = append(adj[v], w)
		adj[w] = append(adj[w], v)
		e++
	}
	g.n = n
	g.e = e
	g.adj = adj
	return nil
}

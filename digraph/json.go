/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

import (
	"encoding/json"
	"fmt"
)

// digraphJSON is the wire form of a Digraph: the vertex count (the
// vertices ARE the indices 0..vertices-1, so the count is the whole
// vertex list) and the edges as [v, w] pairs.
type digraphJSON struct {
	Vertices int      `json:"vertices"`
	Edges    [][2]int `json:"edges"`
}

// MarshalJSON implements json.Marshaler so a Digraph can be used directly
// with the encoding/json package.  The digraph is encoded as a JSON
// object with the vertex count and the edge list:
//
//	{"vertices":5,"edges":[[0,1],[0,2],[1,3],[2,3],[3,4]]}
//
// The edges are listed in the digraph's natural iteration order — source
// vertex ascending, out-neighbors of each vertex in adjacency (insertion)
// order — so a round trip rebuilds an identical digraph, self-loops and
// parallel edges included.
//
// An edgeless digraph encodes with "edges":[].  A direct call on a nil
// digraph encodes as {} (the "nil behaves as an empty digraph" read
// contract); note that json.Marshal on a nil *Digraph never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(V+E); the encoding itself cannot fail (ints only).
func (g *Digraph) MarshalJSON() ([]byte, error) {
	if g == nil {
		return []byte("{}"), nil
	}
	edges := make([][2]int, 0, g.e)
	for v := 0; v < g.n; v++ {
		for _, w := range g.adj[v] {
			edges = append(edges, [2]int{v, w})
		}
	}
	return json.Marshal(digraphJSON{Vertices: g.n, Edges: edges})
}

// UnmarshalJSON implements json.Unmarshaler so a Digraph can be used
// directly with the encoding/json package.  data must be a JSON object
// with a vertex count and an edge list (or null); the decoded digraph
// replaces the current contents — the vertex set becomes
// 0..vertices-1 and the edges are added in document order, so adjacency
// insertion order (and hence Adj iteration order) is preserved.
//
// data is decoded and fully validated before anything is mutated: a
// malformed document, a negative vertex count, or an edge endpoint
// outside 0..vertices-1 is returned as an error with the digraph
// untouched.
//
// Unmarshaling stores vertices and edges, so it follows the insert
// contract: data that would store into a nil digraph panics with the
// standard insert-family message.  A zero-value digraph is fine — the
// replacement allocates fresh adjacency lists, and there is no
// constructor-set function to lose.  null, {}, and {"vertices":0} store
// nothing: they clear the digraph and are tolerated everywhere, even on
// a nil digraph.
// Complexity is O(V+E) plus the cost of decoding.
func (g *Digraph) UnmarshalJSON(data []byte) error {
	var doc digraphJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Vertices < 0 {
		return fmt.Errorf("digraph: UnmarshalJSON: vertices %d is negative", doc.Vertices)
	}
	for _, e := range doc.Edges {
		if e[0] < 0 || e[0] >= doc.Vertices || e[1] < 0 || e[1] >= doc.Vertices {
			return fmt.Errorf("digraph: UnmarshalJSON: edge [%d, %d] out of range for %d vertices", e[0], e[1], doc.Vertices)
		}
	}

	// The insert contract only fires when something would actually be
	// stored (the Concat precedent).
	if doc.Vertices > 0 || len(doc.Edges) > 0 {
		if g == nil {
			panic("digraph: UnmarshalJSON called on a nil digraph")
		}
	}
	if g == nil {
		return nil // null, {}, or {"vertices":0}: nothing to store, nothing to clear
	}

	// Build the replacement off to the side, then swap it in whole, so
	// the digraph is never left half-updated.  Every edge validated
	// in range above, so AddEdge cannot fail here.
	fresh := &Digraph{n: doc.Vertices, adj: make([][]int, doc.Vertices), indeg: make([]int, doc.Vertices)}
	for _, e := range doc.Edges {
		fresh.AddEdge(e[0], e[1])
	}
	*g = *fresh
	return nil
}

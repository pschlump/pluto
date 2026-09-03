/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

import (
	"encoding/json"
	"fmt"
)

// digraphJSON is the wire form of a Digraph: the vertex list — always
// the contiguous range 0..n-1, since the vertices ARE the indices — and
// the edge list, each edge a [v, w] pair in the package's usual
// notation.
type digraphJSON struct {
	Vertices []int    `json:"vertices"`
	Edges    [][2]int `json:"edges"`
}

// MarshalJSON implements json.Marshaler so a Digraph can be used
// directly with the encoding/json package.  The digraph is encoded as a
// JSON object with the vertex list ("vertices") and the edge list
// ("edges"), e.g. {"vertices":[0,1,2],"edges":[[0,1],[1,2]]}.
//
// Each directed edge is emitted exactly once (a self-loop counts once,
// matching E); edges are listed by ascending source vertex and then in
// adjacency insertion order, so the encoding is deterministic and a
// marshal/unmarshal round-trip reproduces the identical digraph,
// adjacency order included.  Errors from the json package are returned
// unchanged.
//
// The adjacency lists are snapshotted under the read lock and the
// encoding itself runs without the lock, so this is safe to call
// concurrently with any digraph operation (the Adj snapshot
// convention).
//
// An edgeless digraph still lists every vertex; a zero-value digraph
// encodes as {"vertices":[],"edges":[]}.  A direct call on a nil
// digraph encodes as {} (the "nil behaves as an empty digraph" read
// contract); note that json.Marshal on a nil *Digraph never reaches
// this method — the json package writes null for nil pointers itself.
// Complexity is O(V+E).
func (g *Digraph) MarshalJSON() ([]byte, error) {
	if g == nil {
		return []byte("{}"), nil
	}
	g.lock.RLock()
	n := g.n
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	doc := digraphJSON{
		Vertices: make([]int, n),
		Edges:    make([][2]int, 0),
	}
	for v := range doc.Vertices {
		doc.Vertices[v] = v
	}
	for v, ws := range adj {
		for _, w := range ws {
			doc.Edges = append(doc.Edges, [2]int{v, w})
		}
	}
	return json.Marshal(doc)
}

// UnmarshalJSON implements json.Unmarshaler so a Digraph can be used
// directly with the encoding/json package.  data must be the JSON
// object MarshalJSON produces (or null); the decoded vertices and edges
// replace the current contents of the digraph under one hold of the
// write lock — the vertex count becomes the length of the vertex list
// and the edges are re-added in document order, reproducing the
// original adjacency insertion order.  A digraph has no constructor-set
// state to preserve, so a zero-value digraph is simply rebuilt in
// place.
//
// The document is decoded and validated before the lock is taken and
// before the digraph is touched: a decode error (malformed JSON, a
// non-object document, wrong field types), a vertex list that is not
// the contiguous range 0..n-1, or an edge endpoint outside the vertex
// list is returned as an error and leaves the digraph untouched.
//
// Unmarshaling stores vertices and edges, so it follows the insert
// contract: data that would store a vertex or an edge into a nil
// digraph panics with the standard insert-family message.  null, {},
// and a document with no vertices and no edges clear the digraph (to
// the zero value, the empty digraph with no vertices) and are tolerated
// everywhere — they store nothing.
// Complexity is O(V+E) plus the cost of decoding the document.
func (g *Digraph) UnmarshalJSON(data []byte) error {
	var doc digraphJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	// Validate before touching the digraph: the vertices must be the
	// contiguous range 0..n-1 (the vertices ARE the indices) and every
	// edge endpoint must be an in-range vertex.
	for i, v := range doc.Vertices {
		if v != i {
			return fmt.Errorf("digraph_ts: UnmarshalJSON: vertices must be 0..n-1 in order, got %v", doc.Vertices)
		}
	}
	n := len(doc.Vertices)
	for _, e := range doc.Edges {
		if e[0] < 0 || e[0] >= n || e[1] < 0 || e[1] >= n {
			return fmt.Errorf("digraph_ts: UnmarshalJSON: edge %v is out of range for %d vertices", e, n)
		}
	}

	// The insert contract only fires when an element would actually be
	// stored (the Concat precedent).
	if len(doc.Vertices) > 0 || len(doc.Edges) > 0 {
		if g == nil {
			panic("digraph_ts: UnmarshalJSON called on a nil digraph")
		}
	}
	if g == nil {
		return nil // null or empty: nothing to store, nothing to clear
	}

	adj := make([][]int, n)
	indeg := make([]int, n)
	e := 0
	for _, edge := range doc.Edges {
		v, w := edge[0], edge[1]
		adj[v] = append(adj[v], w)
		indeg[w]++
		e++
	}

	g.lock.Lock()
	defer g.lock.Unlock()
	g.n = n
	g.e = e
	g.adj = adj
	g.indeg = indeg
	return nil
}

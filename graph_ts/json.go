/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph_ts

import (
	"encoding/json"
	"fmt"
)

// graphJSON is the wire form of a Graph: the vertex list (always
// [0 1 ... n-1], so its length carries the vertex count) and the edge
// list, one [v w] pair per edge.
type graphJSON struct {
	Vertices []int    `json:"vertices"`
	Edges    [][2]int `json:"edges"`
}

// MarshalJSON implements json.Marshaler so a Graph can be used directly
// with the encoding/json package.  The graph is encoded as a JSON object
// with the vertex list — always [0 1 ... n-1], the only vertex set a
// Graph can hold — and the edge list:
//
//	{"vertices":[0,1,2,3],"edges":[[0,1],[1,2],[2,3]]}
//
// Edges are emitted in canonical order — ascending lower endpoint, then
// in adjacency insertion order within the lower endpoint — with each
// edge once, so parallel edges appear repeatedly and a self-loop appears
// as a single [v v] pair (matching how each counts once in E).
//
// The adjacency structure is snapshotted under the read lock (the
// NewDFSPaths/NewBFSPaths/NewCC snapshot convention) and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any graph operation.
//
// An edgeless graph encodes with an empty edge list.  A direct call on a
// nil or zero-value graph encodes as {"vertices":[],"edges":[]} (the
// "nil behaves as an empty graph" read contract); note that json.Marshal
// on a nil *Graph never reaches this method — the json package writes
// null for nil pointers itself.
// Complexity is O(V+E).
func (g *Graph) MarshalJSON() ([]byte, error) {
	if g == nil {
		return []byte(`{"vertices":[],"edges":[]}`), nil
	}
	g.lock.RLock()
	n := g.n
	e := g.e
	adj := g.snapshotAdj()
	g.lock.RUnlock()

	doc := graphJSON{
		Vertices: make([]int, n),
		Edges:    make([][2]int, 0, e),
	}
	for v := range doc.Vertices {
		doc.Vertices[v] = v
	}
	for v, list := range adj {
		self := 0 // a self-loop is listed twice in adj[v]; emit it once
		for _, w := range list {
			switch {
			case w > v:
				doc.Edges = append(doc.Edges, [2]int{v, w})
			case w == v:
				if self%2 == 0 {
					doc.Edges = append(doc.Edges, [2]int{v, v})
				}
				self++
			}
		}
	}
	return json.Marshal(doc)
}

// UnmarshalJSON implements json.Unmarshaler so a Graph can be used
// directly with the encoding/json package.  data must be a JSON object
// of the form {"vertices":[...],"edges":[[v,w],...]} (or null); the
// decoded graph replaces the current contents — the vertex set is
// resized to the vertex list and the edges are rebuilt in edge-list
// order — under one hold of the write lock.
//
// The vertex list must be exactly [0 1 ... n-1] (the only vertex set a
// Graph can hold) and every edge endpoint must be in range; anything
// else is a decode error.  data is decoded and validated before the lock
// is taken, and a decode error (malformed JSON, a non-object document,
// a bad vertex list, an out-of-range endpoint) is returned with the
// graph untouched.
//
// Unmarshaling stores the graph, so it follows the insert contract: data
// that would store vertices or edges into a nil graph or a zero-value
// graph (one not created by NewGraph) panics with the standard
// insert-family message.  null, {}, and an empty document
// ({"vertices":[],"edges":[]}) store nothing: they are tolerated
// everywhere, and on a constructed graph they clear the edges and keep
// the vertex set.
//
// The rebuilt adjacency lists follow the edge list, so a round trip
// preserves V, E, Degree, and HasEdge exactly; per-vertex neighbor order
// is preserved when the edge list is in canonical order (the MarshalJSON
// output always is, so a second round trip is a fixed point).
// Complexity is O(V+E) plus the cost of decoding.
func (g *Graph) UnmarshalJSON(data []byte) error {
	var doc graphJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	n := len(doc.Vertices)
	for i, v := range doc.Vertices {
		if v != i {
			return fmt.Errorf("graph_ts: UnmarshalJSON: vertices must be 0..n-1 in order, got %v", doc.Vertices)
		}
	}
	for _, e := range doc.Edges {
		if e[0] < 0 || e[0] >= n || e[1] < 0 || e[1] >= n {
			return fmt.Errorf("graph_ts: UnmarshalJSON: edge %v out of range for %d vertices", e, n)
		}
	}

	// The insert contract only fires when something would actually be
	// stored.  n is read under the read lock: UnmarshalJSON itself may
	// change it under the write lock in another goroutine.
	if len(doc.Vertices) > 0 || len(doc.Edges) > 0 {
		if g == nil {
			panic("graph_ts: UnmarshalJSON called on a nil graph")
		}
		g.lock.RLock()
		zero := g.n == 0 // NewGraph requires n >= 1, so n == 0 means zero-value
		g.lock.RUnlock()
		if zero {
			panic("graph_ts: UnmarshalJSON called on a zero-value graph (create the graph with NewGraph)")
		}
	}
	if g == nil {
		return nil // null or an empty document: nothing to store, nothing to clear
	}

	g.lock.Lock()
	defer g.lock.Unlock()
	if len(doc.Vertices) == 0 && len(doc.Edges) == 0 {
		// null, {}, or an empty document: clear the edges, keep the
		// vertex set.
		g.adj = make([][]int, g.n)
		g.e = 0
		return nil
	}
	g.n = n
	g.adj = make([][]int, n)
	g.e = 0
	for _, e := range doc.Edges {
		v, w := e[0], e[1]
		g.adj[v] = append(g.adj[v], w)
		g.adj[w] = append(g.adj[w], v)
		g.e++
	}
	return nil
}

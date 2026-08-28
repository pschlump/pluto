/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package dijkstra_ts implements Dijkstra's shortest-path algorithm over
// edge-weighted digraphs and graphs on the integer vertices 0..n-1, safe
// for concurrent use (Sedgwick, Algorithms, 4th ed., §4.4).  It is the
// thread-safe twin of github.com/pschlump/pluto/dijkstra — the same API,
// guarded by a sync.RWMutex — built on pluto/index_pq for the indexed
// priority queue.
//
// Concurrency model:
//
//	AddEdge takes the write lock.
//	V and E take the read lock and release it before returning, so
//	they run in parallel with each other.
//	Adj returns a snapshot copy of the one adjacency list taken under
//	the read lock when Adj is called, so it is safe to use
//	concurrently with any graph operation and never observes later
//	modifications.
//	The Dijkstra constructors (NewDijkstraSP, NewDijkstraUndirectedSP,
//	NewDijkstraAllPairsSP) snapshot the whole adjacency structure
//	under the read lock, validate the snapshot's weights, then run
//	the search lock-free on the snapshot — no lock is held during the
//	search.
//
// The query objects (DijkstraSP, DijkstraUndirectedSP,
// DijkstraAllPairsSP) are immutable after construction — snapshots of
// the graph at construction time — so they are safe for concurrent
// reads; their Lock/Unlock are no-ops kept for API parity with the
// dijkstra package.
//
// The graphs expose the REAL write lock as Lock/Unlock (nil no-ops).
// There are no Nl-prefixed methods in this package, so while a graph's
// Lock is held you must not call any of its public methods — V, E, Adj,
// AddEdge and the Dijkstra constructors would all deadlock.
//
// Self-loops and parallel edges are allowed, exactly like pluto/graph.
// There is deliberately no RemoveEdge — matching Sedgwick.
//
// Panic contract (each panic message names the method and the fix):
// NewEdgeWeightedDigraph/NewEdgeWeightedGraph with v < 1; AddEdge on a
// nil graph; the Dijkstra constructors on a nil or empty graph, with an
// out-of-range source, or when any edge has a negative weight.  Every
// other operation tolerates nil and zero values as an empty result.
//
// Run the tests with -race.
package dijkstra_ts

import (
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/pschlump/pluto/index_pq"
)

// DirectedEdge is an edge From -> To with a non-negative weight.
type DirectedEdge struct {
	From, To int
	Weight   float64
}

// Edge is an undirected edge between V and W with a non-negative weight.
type Edge struct {
	V, W   int
	Weight float64
}

// Other returns the endpoint of e that is not v.
// Complexity is O(1).
func (e Edge) Other(v int) int {
	if v == e.V {
		return e.W
	}
	return e.V
}

// -------------------------------------------------------------------------------------------------------
// EdgeWeightedDigraph
// -------------------------------------------------------------------------------------------------------

// EdgeWeightedDigraph is an edge-weighted digraph on the vertices
// 0..n-1, stored as adjacency lists, safe for concurrent use.
type EdgeWeightedDigraph struct {
	n    int
	e    int
	adj  [][]DirectedEdge
	lock sync.RWMutex
}

// NewEdgeWeightedDigraph creates a digraph with v vertices (0..v-1) and
// no edges.
// It panics if v < 1.
// Complexity is O(v).
func NewEdgeWeightedDigraph(v int) *EdgeWeightedDigraph {
	if v < 1 {
		panic(fmt.Sprintf("dijkstra_ts: NewEdgeWeightedDigraph called with v=%d, need v >= 1", v))
	}
	return &EdgeWeightedDigraph{n: v, adj: make([][]DirectedEdge, v)}
}

// AddEdge adds the directed edge e.From -> e.To to the digraph and
// returns true.  It returns false if either endpoint is out of range.
// Self-loops and parallel edges are allowed; each call counts once in E.
// It panics on a nil graph — a nil graph cannot store an edge.
// Complexity is O(1).
func (g *EdgeWeightedDigraph) AddEdge(e DirectedEdge) bool {
	if g == nil {
		panic("dijkstra_ts: AddEdge called on a nil graph")
	}
	g.lock.Lock()
	defer g.lock.Unlock()
	if e.From < 0 || e.From >= g.n || e.To < 0 || e.To >= g.n {
		return false
	}
	g.adj[e.From] = append(g.adj[e.From], e)
	g.e++
	return true
}

// V returns the number of vertices in the digraph.
// Complexity is O(1).
func (g *EdgeWeightedDigraph) V() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.n
}

// E returns the number of edges in the digraph.
// Complexity is O(1).
func (g *EdgeWeightedDigraph) E() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.e
}

// Adj returns a snapshot copy of the edges leaving v in insertion order,
// taken under the read lock when Adj is called, or nil if v is out of
// range or the graph is nil.  The snapshot never observes later AddEdge
// calls and the caller may mutate it freely.
// Complexity is O(degree(v)).
func (g *EdgeWeightedDigraph) Adj(v int) []DirectedEdge {
	if g == nil {
		return nil
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	if v < 0 || v >= g.n {
		return nil
	}
	return append([]DirectedEdge(nil), g.adj[v]...)
}

// Lock takes the graph's write lock.  There are no Nl-prefixed methods
// in this package, so while the lock is held do not call any public
// method of the same graph (V, E, Adj, AddEdge, the Dijkstra
// constructors) — they would deadlock.  Pair every Lock with a
// corresponding Unlock.  Locking a nil graph is a no-op.
func (g *EdgeWeightedDigraph) Lock() {
	if g == nil {
		return
	}
	g.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil graph
// is a no-op.
func (g *EdgeWeightedDigraph) Unlock() {
	if g == nil {
		return
	}
	g.lock.Unlock()
}

// snapshotAdj returns a deep copy of the adjacency lists; the caller
// must hold the read lock.
func (g *EdgeWeightedDigraph) snapshotAdj() [][]DirectedEdge {
	adj := make([][]DirectedEdge, g.n)
	for i := range g.adj {
		adj[i] = append([]DirectedEdge(nil), g.adj[i]...)
	}
	return adj
}

// -------------------------------------------------------------------------------------------------------
// EdgeWeightedGraph
// -------------------------------------------------------------------------------------------------------

// EdgeWeightedGraph is an edge-weighted undirected graph on the vertices
// 0..n-1, stored as adjacency lists, safe for concurrent use.
type EdgeWeightedGraph struct {
	n    int
	e    int
	adj  [][]Edge
	lock sync.RWMutex
}

// NewEdgeWeightedGraph creates a graph with v vertices (0..v-1) and no
// edges.
// It panics if v < 1.
// Complexity is O(v).
func NewEdgeWeightedGraph(v int) *EdgeWeightedGraph {
	if v < 1 {
		panic(fmt.Sprintf("dijkstra_ts: NewEdgeWeightedGraph called with v=%d, need v >= 1", v))
	}
	return &EdgeWeightedGraph{n: v, adj: make([][]Edge, v)}
}

// AddEdge adds the undirected edge e.V -- e.W to the graph and returns
// true: the edge is recorded in both endpoints' adjacency lists, so each
// edge is relaxed in both directions.  It returns false if either
// endpoint is out of range.  Self-loops and parallel edges are allowed;
// each call counts once in E (a self-loop appears twice in Adj(e.V),
// exactly like pluto/graph's Degree convention).
// It panics on a nil graph — a nil graph cannot store an edge.
// Complexity is O(1).
func (g *EdgeWeightedGraph) AddEdge(e Edge) bool {
	if g == nil {
		panic("dijkstra_ts: AddEdge called on a nil graph")
	}
	g.lock.Lock()
	defer g.lock.Unlock()
	if e.V < 0 || e.V >= g.n || e.W < 0 || e.W >= g.n {
		return false
	}
	g.adj[e.V] = append(g.adj[e.V], e)
	g.adj[e.W] = append(g.adj[e.W], e)
	g.e++
	return true
}

// V returns the number of vertices in the graph.
// Complexity is O(1).
func (g *EdgeWeightedGraph) V() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.n
}

// E returns the number of edges in the graph (a self-loop counts once).
// Complexity is O(1).
func (g *EdgeWeightedGraph) E() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.e
}

// Adj returns a snapshot copy of the edges incident to v in insertion
// order, taken under the read lock when Adj is called, or nil if v is
// out of range or the graph is nil.  The snapshot never observes later
// AddEdge calls and the caller may mutate it freely.
// Complexity is O(degree(v)).
func (g *EdgeWeightedGraph) Adj(v int) []Edge {
	if g == nil {
		return nil
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	if v < 0 || v >= g.n {
		return nil
	}
	return append([]Edge(nil), g.adj[v]...)
}

// Lock takes the graph's write lock.  There are no Nl-prefixed methods
// in this package, so while the lock is held do not call any public
// method of the same graph (V, E, Adj, AddEdge, the Dijkstra
// constructors) — they would deadlock.  Pair every Lock with a
// corresponding Unlock.  Locking a nil graph is a no-op.
func (g *EdgeWeightedGraph) Lock() {
	if g == nil {
		return
	}
	g.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil graph
// is a no-op.
func (g *EdgeWeightedGraph) Unlock() {
	if g == nil {
		return
	}
	g.lock.Unlock()
}

// snapshotAdj returns a deep copy of the adjacency lists; the caller
// must hold the read lock.
func (g *EdgeWeightedGraph) snapshotAdj() [][]Edge {
	adj := make([][]Edge, g.n)
	for i := range g.adj {
		adj[i] = append([]Edge(nil), g.adj[i]...)
	}
	return adj
}

// -------------------------------------------------------------------------------------------------------
// DijkstraSP — single-source shortest paths in a digraph
// -------------------------------------------------------------------------------------------------------

// DijkstraSP answers shortest-path queries from a source vertex s in an
// edge-weighted digraph with non-negative weights (Sedgwick's
// DijkstraSP).  It is an immutable snapshot of the graph at construction
// time — later AddEdge calls are not reflected — and is safe for
// concurrent reads.
type DijkstraSP struct {
	distTo  []float64      // distTo[v] is the length of a shortest s -> v path (+Inf if none)
	edgeTo  []DirectedEdge // edgeTo[v] is the last edge on a shortest s -> v path
	hasEdge []bool         // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
}

// validateDigraphWeights panics if any edge in adj has a negative weight
// — Dijkstra's algorithm is only correct for non-negative weights.  who
// names the constructor for the panic message.
func validateDigraphWeights(who string, adj [][]DirectedEdge) {
	for v := range adj {
		for _, e := range adj[v] {
			if e.Weight < 0 {
				panic(fmt.Sprintf("dijkstra_ts: %s: edge %d->%d has negative weight %g", who, e.From, e.To, e.Weight))
			}
		}
	}
}

// validateGraphWeights panics if any edge in adj has a negative weight.
func validateGraphWeights(who string, adj [][]Edge) {
	for v := range adj {
		for _, e := range adj[v] {
			if e.Weight < 0 {
				panic(fmt.Sprintf("dijkstra_ts: %s: edge %d--%d has negative weight %g", who, e.V, e.W, e.Weight))
			}
		}
	}
}

// shortestPathsFrom runs Dijkstra's algorithm from s over the snapshot
// adjacency adj (already validated); the lock-free core shared by
// NewDijkstraSP and NewDijkstraAllPairsSP.  No lock is held while it
// runs.
// Complexity is O(E log V) time, O(V) space.
func shortestPathsFrom(adj [][]DirectedEdge, s int) *DijkstraSP {
	d := &DijkstraSP{
		distTo:  make([]float64, len(adj)),
		edgeTo:  make([]DirectedEdge, len(adj)),
		hasEdge: make([]bool, len(adj)),
		s:       s,
	}
	for v := range d.distTo {
		d.distTo[v] = math.Inf(1)
	}
	d.distTo[s] = 0
	pq := index_pq.NewIndexPQ[float64](len(adj))
	pq.Insert(s, 0)
	for !pq.IsEmpty() {
		v, _, _ := pq.Pop()
		for _, e := range adj[v] {
			w := e.To
			if nd := d.distTo[v] + e.Weight; d.distTo[w] > nd {
				d.distTo[w] = nd
				d.edgeTo[w] = e
				d.hasEdge[w] = true
				if pq.Contains(w) {
					pq.Change(w, nd) // decrease-key
				} else {
					pq.Insert(w, nd)
				}
			}
		}
	}
	return d
}

// NewDijkstraSP snapshots the graph's adjacency lists under the read
// lock, then computes shortest paths from s lock-free on the snapshot.
// It panics on a nil or empty graph, on an out-of-range source, or when
// any edge of the snapshot has a negative weight — programmer errors
// with no sane answer.
// Complexity is O(E log V) time plus O(V+E) for the snapshot, O(V)
// space.
func NewDijkstraSP(g *EdgeWeightedDigraph, s int) *DijkstraSP {
	if g == nil {
		panic("dijkstra_ts: NewDijkstraSP called on a nil graph")
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		panic("dijkstra_ts: NewDijkstraSP called on an empty graph")
	}
	if s < 0 || s >= g.n {
		g.lock.RUnlock()
		panic(fmt.Sprintf("dijkstra_ts: NewDijkstraSP called with out-of-range source %d (graph has %d vertices)", s, g.n))
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	validateDigraphWeights("NewDijkstraSP", adj)
	return shortestPathsFrom(adj, s)
}

// DistTo returns the length of a shortest path from the source to v, or
// +Inf if v is unreachable or out of range (or the receiver is nil).
// Complexity is O(1).
func (d *DijkstraSP) DistTo(v int) float64 {
	if d == nil || v < 0 || v >= len(d.distTo) {
		return math.Inf(1)
	}
	return d.distTo[v]
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (d *DijkstraSP) HasPathTo(v int) bool {
	if d == nil || v < 0 || v >= len(d.distTo) {
		return false
	}
	return !math.IsInf(d.distTo[v], 1)
}

// PathTo returns a shortest path from the source to v as a slice of
// edges ordered source-first, as a fresh slice the caller may mutate.
// It returns nil if there is no path (or v is out of range); the path
// from the source to itself is an empty non-nil slice.  The edge weights
// along the returned path sum to DistTo(v).
// Complexity is O(path length).
func (d *DijkstraSP) PathTo(v int) []DirectedEdge {
	if !d.HasPathTo(v) {
		return nil
	}
	path := make([]DirectedEdge, 0)
	for x := v; x != d.s; x = d.edgeTo[x].From {
		path = append(path, d.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op: a DijkstraSP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the dijkstra
// package.
func (d *DijkstraSP) Lock() {}

// Unlock is a no-op: a DijkstraSP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the dijkstra
// package.
func (d *DijkstraSP) Unlock() {}

// -------------------------------------------------------------------------------------------------------
// DijkstraUndirectedSP — single-source shortest paths in an undirected graph
// -------------------------------------------------------------------------------------------------------

// DijkstraUndirectedSP answers shortest-path queries from a source
// vertex s in an edge-weighted undirected graph with non-negative
// weights (Sedgwick's DijkstraUndirectedSP).  It is an immutable
// snapshot of the graph at construction time — later AddEdge calls are
// not reflected — and is safe for concurrent reads.
type DijkstraUndirectedSP struct {
	distTo  []float64
	edgeTo  []Edge
	hasEdge []bool
	s       int
}

// NewDijkstraUndirectedSP snapshots the graph's adjacency lists under
// the read lock, then computes shortest paths from s lock-free on the
// snapshot.  It panics on a nil or empty graph, on an out-of-range
// source, or when any edge of the snapshot has a negative weight —
// programmer errors with no sane answer.
// Complexity is O(E log V) time plus O(V+E) for the snapshot, O(V)
// space.
func NewDijkstraUndirectedSP(g *EdgeWeightedGraph, s int) *DijkstraUndirectedSP {
	if g == nil {
		panic("dijkstra_ts: NewDijkstraUndirectedSP called on a nil graph")
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		panic("dijkstra_ts: NewDijkstraUndirectedSP called on an empty graph")
	}
	if s < 0 || s >= g.n {
		g.lock.RUnlock()
		panic(fmt.Sprintf("dijkstra_ts: NewDijkstraUndirectedSP called with out-of-range source %d (graph has %d vertices)", s, g.n))
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	validateGraphWeights("NewDijkstraUndirectedSP", adj)

	d := &DijkstraUndirectedSP{
		distTo:  make([]float64, len(adj)),
		edgeTo:  make([]Edge, len(adj)),
		hasEdge: make([]bool, len(adj)),
		s:       s,
	}
	for v := range d.distTo {
		d.distTo[v] = math.Inf(1)
	}
	d.distTo[s] = 0
	pq := index_pq.NewIndexPQ[float64](len(adj))
	pq.Insert(s, 0)
	for !pq.IsEmpty() {
		v, _, _ := pq.Pop()
		for _, e := range adj[v] {
			w := e.Other(v)
			if nd := d.distTo[v] + e.Weight; d.distTo[w] > nd {
				d.distTo[w] = nd
				d.edgeTo[w] = e
				d.hasEdge[w] = true
				if pq.Contains(w) {
					pq.Change(w, nd) // decrease-key
				} else {
					pq.Insert(w, nd)
				}
			}
		}
	}
	return d
}

// DistTo returns the length of a shortest path from the source to v, or
// +Inf if v is unreachable or out of range (or the receiver is nil).
// Complexity is O(1).
func (d *DijkstraUndirectedSP) DistTo(v int) float64 {
	if d == nil || v < 0 || v >= len(d.distTo) {
		return math.Inf(1)
	}
	return d.distTo[v]
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (d *DijkstraUndirectedSP) HasPathTo(v int) bool {
	if d == nil || v < 0 || v >= len(d.distTo) {
		return false
	}
	return !math.IsInf(d.distTo[v], 1)
}

// PathTo returns a shortest path from the source to v as a slice of
// edges ordered source-first, as a fresh slice the caller may mutate.
// It returns nil if there is no path (or v is out of range); the path
// from the source to itself is an empty non-nil slice.  The edge weights
// along the returned path sum to DistTo(v).
// Complexity is O(path length).
func (d *DijkstraUndirectedSP) PathTo(v int) []Edge {
	if !d.HasPathTo(v) {
		return nil
	}
	path := make([]Edge, 0)
	for x := v; x != d.s; x = d.edgeTo[x].Other(x) {
		path = append(path, d.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op: a DijkstraUndirectedSP is immutable after
// construction, so there is nothing to lock.  Kept for API parity with
// the dijkstra package.
func (d *DijkstraUndirectedSP) Lock() {}

// Unlock is a no-op: a DijkstraUndirectedSP is immutable after
// construction, so there is nothing to lock.  Kept for API parity with
// the dijkstra package.
func (d *DijkstraUndirectedSP) Unlock() {}

// -------------------------------------------------------------------------------------------------------
// DijkstraAllPairsSP — shortest paths from every vertex
// -------------------------------------------------------------------------------------------------------

// DijkstraAllPairsSP answers shortest-path queries between every pair of
// vertices in an edge-weighted digraph with non-negative weights: one
// DijkstraSP per source vertex (Sedgwick's DijkstraAllPairsSP).  It is an
// immutable snapshot of the graph at construction time and is safe for
// concurrent reads.
type DijkstraAllPairsSP struct {
	all []*DijkstraSP
}

// NewDijkstraAllPairsSP snapshots the graph's adjacency lists under the
// read lock, then computes shortest paths from every vertex lock-free on
// the snapshot.  It panics on a nil or empty graph, or when any edge of
// the snapshot has a negative weight — programmer errors with no sane
// answer.
// Complexity is O(V·E log V) time plus O(V+E) for the snapshot, O(V²)
// space.
func NewDijkstraAllPairsSP(g *EdgeWeightedDigraph) *DijkstraAllPairsSP {
	if g == nil {
		panic("dijkstra_ts: NewDijkstraAllPairsSP called on a nil graph")
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		panic("dijkstra_ts: NewDijkstraAllPairsSP called on an empty graph")
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	validateDigraphWeights("NewDijkstraAllPairsSP", adj)
	ap := &DijkstraAllPairsSP{all: make([]*DijkstraSP, len(adj))}
	for s := range adj {
		ap.all[s] = shortestPathsFrom(adj, s)
	}
	return ap
}

// Dist returns the length of a shortest s -> t path, or +Inf if there is
// none or either vertex is out of range (or the receiver is nil).
// Complexity is O(1).
func (ap *DijkstraAllPairsSP) Dist(s, t int) float64 {
	if ap == nil || s < 0 || s >= len(ap.all) {
		return math.Inf(1)
	}
	return ap.all[s].DistTo(t)
}

// HasPath reports whether t is reachable from s.  Out-of-range vertices
// (and a nil receiver) report false.
// Complexity is O(1).
func (ap *DijkstraAllPairsSP) HasPath(s, t int) bool {
	if ap == nil || s < 0 || s >= len(ap.all) {
		return false
	}
	return ap.all[s].HasPathTo(t)
}

// Path returns a shortest path from s to t as a slice of edges ordered
// source-first, as a fresh slice the caller may mutate.  It returns nil
// if there is no path (or a vertex is out of range); the path from a
// vertex to itself is an empty non-nil slice.
// Complexity is O(path length).
func (ap *DijkstraAllPairsSP) Path(s, t int) []DirectedEdge {
	if ap == nil || s < 0 || s >= len(ap.all) {
		return nil
	}
	return ap.all[s].PathTo(t)
}

// Lock is a no-op: a DijkstraAllPairsSP is immutable after construction,
// so there is nothing to lock.  Kept for API parity with the dijkstra
// package.
func (ap *DijkstraAllPairsSP) Lock() {}

// Unlock is a no-op: a DijkstraAllPairsSP is immutable after
// construction, so there is nothing to lock.  Kept for API parity with
// the dijkstra package.
func (ap *DijkstraAllPairsSP) Unlock() {}

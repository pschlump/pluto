package graph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"reflect"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------------------------

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// newTinyG returns the 6-vertex, 8-edge undirected graph of Sedgwick's
// §4.1 DFS/BFS trace figures (tinyG-style).
func newTinyG() *Graph {
	g := NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}
	return g
}

// adjOf collects the neighbors of v into a slice.
func adjOf(g *Graph, v int) []int {
	var adj []int
	for w := range g.Adj(v) {
		adj = append(adj, w)
	}
	return adj
}

// checkInvariants verifies the structural invariants of g against the
// reference adjacency lists model (built with the same AddEdge calls) and
// the reference edge count e: V/E counts, per-vertex adjacency in
// insertion order, Degree, and HasEdge in both directions.
func checkInvariants(t *testing.T, g *Graph, model [][]int, e int) {
	t.Helper()
	n := len(model)
	if g.V() != n || g.Len() != n {
		t.Errorf("V/Len mismatch: V()=%d Len()=%d, model has %d vertices", g.V(), g.Len(), n)
	}
	if g.E() != e {
		t.Errorf("E()=%d, model has %d edges", g.E(), e)
	}
	degreeSum := 0
	for v := 0; v < n; v++ {
		got := adjOf(g, v)
		if !reflect.DeepEqual(got, model[v]) {
			t.Errorf("Adj(%d) mismatch, expected %v got %v", v, model[v], got)
		}
		if d, ok := g.Degree(v); !ok || d != len(model[v]) {
			t.Errorf("Degree(%d)=%d (ok=%v), model degree is %d", v, d, ok, len(model[v]))
		}
		degreeSum += len(model[v])
		for _, w := range model[v] {
			if !g.HasEdge(v, w) || !g.HasEdge(w, v) {
				t.Errorf("HasEdge(%d, %d) missing in one direction", v, w)
			}
		}
	}
	// Handshaking: self-loops and parallel edges are appended to both
	// lists, so the degree sum is always twice the edge count.
	if degreeSum != 2*e {
		t.Errorf("Degree sum %d != 2*E %d", degreeSum, 2*e)
	}
}

// refBFS is the reference breadth-first search on plain Go slices (no
// pluto imports — the reference stays independent of the code under test).
// It returns the shortest distances and reachability from s.
func refBFS(adj [][]int, s int) (dist []int, marked []bool) {
	n := len(adj)
	dist = make([]int, n)
	marked = make([]bool, n)
	marked[s] = true
	q := []int{s}
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		for _, w := range adj[v] {
			if !marked[w] {
				marked[w] = true
				dist[w] = dist[v] + 1
				q = append(q, w)
			}
		}
	}
	return dist, marked
}

// refComponents is the reference connected-components computation by naive
// relabeling (union-find's slow ancestor) on plain Go slices.  It returns
// compact component ids (assigned in vertex order) and the count.
func refComponents(n int, edges [][2]int) ([]int, int) {
	id := make([]int, n)
	for i := range id {
		id[i] = i
	}
	for _, e := range edges {
		ra, rb := id[e[0]], id[e[1]]
		if ra != rb {
			for i := range id {
				if id[i] == rb {
					id[i] = ra
				}
			}
		}
	}
	remap := make(map[int]int)
	next := 0
	for v := 0; v < n; v++ {
		if _, ok := remap[id[v]]; !ok {
			remap[id[v]] = next
			next++
		}
		id[v] = remap[id[v]]
	}
	return id, next
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against reference models (fixed seed)
// -------------------------------------------------------------------------------------------------------

func TestGraphRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 40
	g := NewGraph(n)
	model := make([][]int, n) // reference adjacency lists
	var edgeList [][2]int     // reference edge list (for refComponents)

	verify := func(step int) {
		checkInvariants(t, g, model, len(edgeList))

		// BFS distances and reachability against the reference BFS.
		bp := NewBFSPaths(g, 0)
		dist, marked := refBFS(model, 0)
		dp := NewDFSPaths(g, 0)
		for v := 0; v < n; v++ {
			if bp.HasPathTo(v) != marked[v] {
				t.Fatalf("step %d: BFS HasPathTo(%d)=%v, reference says %v", step, v, bp.HasPathTo(v), marked[v])
			}
			if dp.HasPathTo(v) != marked[v] {
				t.Fatalf("step %d: DFS HasPathTo(%d)=%v, reference says %v", step, v, dp.HasPathTo(v), marked[v])
			}
			d, ok := bp.DistTo(v)
			if ok != marked[v] || (ok && d != dist[v]) {
				t.Fatalf("step %d: BFS DistTo(%d)=%d (ok=%v), reference says %d", step, v, d, ok, dist[v])
			}
			// Every BFS path is a valid shortest walk, source first.
			if path, ok := bp.PathTo(v); ok {
				if path[0] != 0 || path[len(path)-1] != v || len(path) != dist[v]+1 {
					t.Fatalf("step %d: BFS PathTo(%d)=%v invalid (dist %d)", step, v, path, dist[v])
				}
				for i := 0; i+1 < len(path); i++ {
					if !g.HasEdge(path[i], path[i+1]) {
						t.Fatalf("step %d: BFS PathTo(%d)=%v: no edge %d-%d", step, v, path, path[i], path[i+1])
					}
				}
			}
			// Every DFS path is a valid walk, source first.
			if path, ok := dp.PathTo(v); ok {
				if path[0] != 0 || path[len(path)-1] != v {
					t.Fatalf("step %d: DFS PathTo(%d)=%v invalid endpoints", step, v, path)
				}
				for i := 0; i+1 < len(path); i++ {
					if !g.HasEdge(path[i], path[i+1]) {
						t.Fatalf("step %d: DFS PathTo(%d)=%v: no edge %d-%d", step, v, path, path[i], path[i+1])
					}
				}
			}
		}

		// Connected components against naive relabeling.
		c := NewCC(g)
		refID, refCount := refComponents(n, edgeList)
		if c.Count() != refCount {
			t.Fatalf("step %d: CC Count()=%d, reference says %d", step, c.Count(), refCount)
		}
		for v := 0; v < n; v++ {
			id, ok := c.ID(v)
			if !ok || id != refID[v] {
				t.Fatalf("step %d: CC ID(%d)=%d (ok=%v), reference says %d", step, v, id, ok, refID[v])
			}
			for w := 0; w < n; w++ {
				if c.Connected(v, w) != (refID[v] == refID[w]) {
					t.Fatalf("step %d: CC Connected(%d, %d)=%v, reference says %v",
						step, v, w, c.Connected(v, w), refID[v] == refID[w])
				}
			}
		}
	}

	for step := range 800 {
		v, w := rng.Intn(n), rng.Intn(n)
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4, 5, 6: // AddEdge (incl. possible self-loops/parallel edges)
			if !g.AddEdge(v, w) {
				t.Fatalf("step %d: AddEdge(%d, %d) returned false on in-range vertices", step, v, w)
			}
			model[v] = append(model[v], w)
			model[w] = append(model[w], v)
			edgeList = append(edgeList, [2]int{v, w})
		case 7: // HasEdge consistency, both hit and miss
			got := g.HasEdge(v, w)
			want := false
			for _, x := range model[v] {
				if x == w {
					want = true
					break
				}
			}
			if got != want {
				t.Fatalf("step %d: HasEdge(%d, %d)=%v, model says %v", step, v, w, got, want)
			}
		case 8: // Degree/Adj spot check on one vertex
			if d, ok := g.Degree(v); !ok || d != len(model[v]) {
				t.Fatalf("step %d: Degree(%d)=%d (ok=%v), model degree is %d", step, v, d, ok, len(model[v]))
			}
			if got := adjOf(g, v); !reflect.DeepEqual(got, model[v]) {
				t.Fatalf("step %d: Adj(%d) mismatch, expected %v got %v", step, v, model[v], got)
			}
		case 9: // Out-of-range AddEdge reports false and changes nothing
			if g.AddEdge(-1, v) || g.AddEdge(v, n) || g.AddEdge(n, -1) {
				t.Fatalf("step %d: out-of-range AddEdge returned true", step)
			}
		}
		if step%50 == 0 {
			verify(step)
		}
	}
	verify(800)
}

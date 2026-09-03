package digraph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"sync"
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

// tinyDGData is the algs4 reference digraph tinyDG.txt, embedded verbatim
// (13 vertices, 22 edges — the classic §4.4 trace digraph).
// Source: https://algs4.cs.princeton.edu/42digraph/tinyDG.txt
const tinyDGData = `13
22
 4  2
 2  3
 3  2
 6  0
 0  1
 2  0
11 12
12  9
 9 10
 9 11
 7  9
10 12
11  4
 4  3
 3  5
 6  8
 8  6
 5  4
 0  5
 6  4
 6  9
 7  6
`

// tinyDAGData is the algs4 reference digraph tinyDAG.txt, embedded
// verbatim (13 vertices, 15 edges, acyclic).
// Source: https://algs4.cs.princeton.edu/42digraph/tinyDAG.txt
const tinyDAGData = `13
15
2 3
0 6
0 1
2 0
11 12
9 12
9 10
9 11
3 5
8 7
5 4
0 5
6 4
6 9
7 6
`

// parseAlgs4Digraph parses the algs4 digraph file format: the vertex
// count, the edge count, then that many "v w" pairs.
func parseAlgs4Digraph(t *testing.T, data string) (n int, edges [][2]int) {
	t.Helper()
	fields := strings.Fields(data)
	if len(fields) < 2 {
		t.Fatalf("malformed digraph data: %d fields", len(fields))
	}
	atoi := func(s string) int {
		v, err := strconv.Atoi(s)
		if err != nil {
			t.Fatalf("malformed digraph data: %q: %v", s, err)
		}
		return v
	}
	n = atoi(fields[0])
	m := atoi(fields[1])
	if len(fields) != 2+2*m {
		t.Fatalf("malformed digraph data: %d edges declared, %d pairs present", m, (len(fields)-2)/2)
	}
	for i := 0; i < m; i++ {
		v, w := atoi(fields[2+2*i]), atoi(fields[2+2*i+1])
		if v < 0 || v >= n || w < 0 || w >= n {
			t.Fatalf("malformed digraph data: edge %d %d out of range %d", v, w, n)
		}
		edges = append(edges, [2]int{v, w})
	}
	return n, edges
}

// buildDigraph constructs a digraph from a vertex count and an edge list.
func buildDigraph(t *testing.T, n int, edges [][2]int) *Digraph {
	t.Helper()
	g := NewDigraph(n)
	for _, e := range edges {
		if !g.AddEdge(e[0], e[1]) {
			t.Fatalf("AddEdge(%d, %d) returned false on in-range vertices", e[0], e[1])
		}
	}
	return g
}

// newTinyDG returns the 13-vertex, 22-edge algs4 tinyDG digraph.
func newTinyDG(t *testing.T) *Digraph {
	t.Helper()
	n, edges := parseAlgs4Digraph(t, tinyDGData)
	return buildDigraph(t, n, edges)
}

// newTinyDAG returns the 13-vertex, 15-edge algs4 tinyDAG digraph.
func newTinyDAG(t *testing.T) *Digraph {
	t.Helper()
	n, edges := parseAlgs4Digraph(t, tinyDAGData)
	return buildDigraph(t, n, edges)
}

// adjOf collects the out-neighbors of v into a slice.
func adjOf(g *Digraph, v int) []int {
	var adj []int
	for w := range g.Adj(v) {
		adj = append(adj, w)
	}
	return adj
}

// checkInvariants verifies the structural invariants of g against the
// reference adjacency lists model (built with the same AddEdge calls) and
// the reference edge count e: V/E counts, per-vertex adjacency in
// insertion order, OutDegree/InDegree, HasEdge, and the directed
// handshaking lemma (sum of out-degrees == sum of in-degrees == E).
func checkInvariants(t *testing.T, g *Digraph, model [][]int, e int) {
	t.Helper()
	n := len(model)
	if g.V() != n || g.Len() != n {
		t.Errorf("V/Len mismatch: V()=%d Len()=%d, model has %d vertices", g.V(), g.Len(), n)
	}
	if g.E() != e {
		t.Errorf("E()=%d, model has %d edges", g.E(), e)
	}
	indeg := make([]int, n)
	for _, ws := range model {
		for _, w := range ws {
			indeg[w]++
		}
	}
	outSum, inSum := 0, 0
	for v := 0; v < n; v++ {
		got := adjOf(g, v)
		if !reflect.DeepEqual(got, model[v]) {
			t.Errorf("Adj(%d) mismatch, expected %v got %v", v, model[v], got)
		}
		if d, ok := g.OutDegree(v); !ok || d != len(model[v]) {
			t.Errorf("OutDegree(%d)=%d (ok=%v), model out-degree is %d", v, d, ok, len(model[v]))
		}
		if d, ok := g.InDegree(v); !ok || d != indeg[v] {
			t.Errorf("InDegree(%d)=%d (ok=%v), model in-degree is %d", v, d, ok, indeg[v])
		}
		outSum += len(model[v])
		inSum += indeg[v]
		for _, w := range model[v] {
			if !g.HasEdge(v, w) {
				t.Errorf("HasEdge(%d, %d) missing", v, w)
			}
		}
	}
	// Directed handshaking: every edge leaves one vertex and enters one.
	if outSum != e || inSum != e {
		t.Errorf("Degree sums: out %d, in %d, E %d — all must be equal", outSum, inSum, e)
	}
	// HasEdge agrees with the model for every ordered pair.
	for v := 0; v < n; v++ {
		for w := 0; w < n; w++ {
			want := false
			for _, x := range model[v] {
				if x == w {
					want = true
					break
				}
			}
			if g.HasEdge(v, w) != want {
				t.Errorf("HasEdge(%d, %d)=%v, model says %v", v, w, g.HasEdge(v, w), want)
			}
		}
	}
}

// refBFS is the reference breadth-first search on plain Go slices (no
// pluto imports — the reference stays independent of the code under
// test).  It returns the shortest distances and reachability from s.
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

// refSCC is the reference strong-component computation by mutual
// reachability: BFS from every vertex (O(V·(V+E))), grouping vertices
// that reach each other.  It returns compact component ids (assigned in
// vertex order — not the same numbering as Kosaraju, so compare via the
// equivalence relation, not raw ids) and the count.
func refSCC(adj [][]int) ([]int, int) {
	n := len(adj)
	reach := make([][]bool, n)
	for v := 0; v < n; v++ {
		_, reach[v] = refBFS(adj, v)
	}
	id := make([]int, n)
	for i := range id {
		id[i] = -1
	}
	count := 0
	for v := 0; v < n; v++ {
		if id[v] >= 0 {
			continue
		}
		for w := v; w < n; w++ {
			if id[w] < 0 && reach[v][w] && reach[w][v] {
				id[w] = count
			}
		}
		count++
	}
	return id, count
}

// refHasCycle is the reference cycle check: three-color (white/gray/
// black) iterative DFS on plain Go slices, independent of the code under
// test.
func refHasCycle(adj [][]int) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	n := len(adj)
	color := make([]int, n)
	type frame struct {
		v, next int
	}
	for s := 0; s < n; s++ {
		if color[s] != white {
			continue
		}
		stack := []frame{{v: s}}
		for len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.next == 0 {
				color[f.v] = gray
			}
			if f.next < len(adj[f.v]) {
				w := adj[f.v][f.next]
				f.next++
				switch color[w] {
				case gray:
					return true
				case white:
					stack = append(stack, frame{v: w})
				}
			} else {
				color[f.v] = black
				stack = stack[:len(stack)-1]
			}
		}
	}
	return false
}

// checkTopologicalOrder verifies that order is a permutation of 0..n-1
// with every edge pointing forward (for every edge v->w, v precedes w).
func checkTopologicalOrder(t *testing.T, order []int, model [][]int) {
	t.Helper()
	n := len(model)
	if len(order) != n {
		t.Fatalf("Order has %d vertices, expected %d: %v", len(order), n, order)
	}
	pos := make([]int, n)
	seen := make([]bool, n)
	for i, v := range order {
		if v < 0 || v >= n || seen[v] {
			t.Fatalf("Order %v is not a permutation of 0..%d", order, n-1)
		}
		seen[v] = true
		pos[v] = i
	}
	for v, ws := range model {
		for _, w := range ws {
			if v != w && pos[v] >= pos[w] {
				t.Fatalf("Order %v violates edge %d->%d (pos %d >= %d)", order, v, w, pos[v], pos[w])
			}
		}
	}
}

// checkCycleIsReal verifies that cyc is a genuine directed cycle of g:
// consecutive pairs (and the wraparound) are edges, and all vertices
// except the repeated last one are distinct.
func checkCycleIsReal(t *testing.T, g *Digraph, cyc []int) {
	t.Helper()
	if len(cyc) < 2 {
		t.Fatalf("Cycle %v too short", cyc)
	}
	if cyc[0] != cyc[len(cyc)-1] {
		t.Fatalf("Cycle %v does not return to its start", cyc)
	}
	seen := make(map[int]bool)
	for i, v := range cyc[:len(cyc)-1] {
		if seen[v] {
			t.Fatalf("Cycle %v repeats vertex %d", cyc, v)
		}
		seen[v] = true
		if !g.HasEdge(v, cyc[i+1]) {
			t.Fatalf("Cycle %v: no edge %d->%d", cyc, v, cyc[i+1])
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against reference models (fixed seed)
// -------------------------------------------------------------------------------------------------------

func TestDigraphRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 40
	g := NewDigraph(n)
	model := make([][]int, n) // reference adjacency lists
	edgeCount := 0            // reference edge count

	verify := func(step int) {
		checkInvariants(t, g, model, edgeCount)

		// BFS distances and reachability against the reference BFS.
		bp := NewBFSDirectedPaths(g, 0)
		dist, marked := refBFS(model, 0)
		dp := NewDFSDirectedPaths(g, 0)
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
			// Every BFS path is a valid shortest directed walk, source first.
			if path, ok := bp.PathTo(v); ok {
				if path[0] != 0 || path[len(path)-1] != v || len(path) != dist[v]+1 {
					t.Fatalf("step %d: BFS PathTo(%d)=%v invalid (dist %d)", step, v, path, dist[v])
				}
				for i := 0; i+1 < len(path); i++ {
					if !g.HasEdge(path[i], path[i+1]) {
						t.Fatalf("step %d: BFS PathTo(%d)=%v: no edge %d->%d", step, v, path, path[i], path[i+1])
					}
				}
			}
			// Every DFS path is a valid directed walk, source first.
			if path, ok := dp.PathTo(v); ok {
				if path[0] != 0 || path[len(path)-1] != v {
					t.Fatalf("step %d: DFS PathTo(%d)=%v invalid endpoints", step, v, path)
				}
				for i := 0; i+1 < len(path); i++ {
					if !g.HasEdge(path[i], path[i+1]) {
						t.Fatalf("step %d: DFS PathTo(%d)=%v: no edge %d->%d", step, v, path, path[i], path[i+1])
					}
				}
			}
		}

		// Multi-source reachability: the union of the per-source
		// reference reachability sets.
		sources := []int{0, n / 2}
		dd := NewDirectedDFS(g, sources...)
		wantCount := 0
		for v := 0; v < n; v++ {
			want := false
			for _, s := range sources {
				_, m := refBFS(model, s)
				want = want || m[v]
			}
			if dd.Marked(v) != want {
				t.Fatalf("step %d: DirectedDFS Marked(%d)=%v, reference says %v", step, v, dd.Marked(v), want)
			}
			if want {
				wantCount++
			}
		}
		if dd.Count() != wantCount {
			t.Fatalf("step %d: DirectedDFS Count()=%d, reference says %d", step, dd.Count(), wantCount)
		}

		// Reverse: the reversed model adjacency matches.
		r := g.Reverse()
		for v := 0; v < n; v++ {
			var want []int
			for u := 0; u < n; u++ {
				for _, w := range model[u] {
					if w == v {
						want = append(want, u)
					}
				}
			}
			// Reverse builds its lists in vertex order of the source
			// endpoints; build the expectation the same way.
			if got := adjOf(r, v); !reflect.DeepEqual(got, want) {
				t.Fatalf("step %d: Reverse Adj(%d)=%v, expected %v", step, v, got, want)
			}
		}

		// Strong components against mutual reachability.
		scc := NewKosarajuSCC(g)
		refID, refCount := refSCC(model)
		if scc.Count() != refCount {
			t.Fatalf("step %d: KosarajuSCC Count()=%d, reference says %d", step, scc.Count(), refCount)
		}
		for v := 0; v < n; v++ {
			if _, ok := scc.ID(v); !ok {
				t.Fatalf("step %d: KosarajuSCC ID(%d) reported out of range", step, v)
			}
			for w := 0; w < n; w++ {
				if scc.StronglyConnected(v, w) != (refID[v] == refID[w]) {
					t.Fatalf("step %d: KosarajuSCC StronglyConnected(%d, %d)=%v, reference says %v",
						step, v, w, scc.StronglyConnected(v, w), refID[v] == refID[w])
				}
			}
		}

		// Cycle detection and topological order against the reference.
		c := NewDirectedCycle(g)
		hasCycle := refHasCycle(model)
		if c.HasCycle() != hasCycle {
			t.Fatalf("step %d: DirectedCycle HasCycle()=%v, reference says %v", step, c.HasCycle(), hasCycle)
		}
		if c.HasCycle() {
			checkCycleIsReal(t, g, c.Cycle())
		} else if cyc := c.Cycle(); cyc != nil {
			t.Fatalf("step %d: Cycle()=%v on an acyclic digraph, expected nil", step, cyc)
		}
		top := NewTopological(g)
		if top.HasOrder() != !hasCycle {
			t.Fatalf("step %d: Topological HasOrder()=%v, reference cycle check says %v", step, top.HasOrder(), hasCycle)
		}
		if top.HasOrder() {
			checkTopologicalOrder(t, top.Order(), model)
		} else if order := top.Order(); order != nil {
			t.Fatalf("step %d: Order()=%v on a cyclic digraph, expected nil", step, order)
		}

		// Depth-first order is always a permutation of 0..n-1.
		o := NewDepthFirstOrder(g)
		for name, order := range map[string][]int{"Pre": o.Pre(), "Post": o.Post(), "ReversePost": o.ReversePost()} {
			if len(order) != n {
				t.Fatalf("step %d: DepthFirstOrder %s has %d vertices, expected %d", step, name, len(order), n)
			}
			seen := make([]bool, n)
			for _, v := range order {
				if v < 0 || v >= n || seen[v] {
					t.Fatalf("step %d: DepthFirstOrder %s=%v is not a permutation", step, name, order)
				}
				seen[v] = true
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
			edgeCount++
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
		case 8: // OutDegree/InDegree/Adj spot check on one vertex
			if d, ok := g.OutDegree(v); !ok || d != len(model[v]) {
				t.Fatalf("step %d: OutDegree(%d)=%d (ok=%v), model out-degree is %d", step, v, d, ok, len(model[v]))
			}
			indeg := 0
			for u := 0; u < n; u++ {
				for _, x := range model[u] {
					if x == v {
						indeg++
					}
				}
			}
			if d, ok := g.InDegree(v); !ok || d != indeg {
				t.Fatalf("step %d: InDegree(%d)=%d (ok=%v), model in-degree is %d", step, v, d, ok, indeg)
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

// -------------------------------------------------------------------------------------------------------
// Concurrency (run under -race)
// -------------------------------------------------------------------------------------------------------

// TestDigraphConcurrent hammers one digraph with concurrent AddEdge
// writers while readers run full traversals (DFS/BFS/cycle/order/
// topological/SCC), iterate Adj snapshots, and call the read operations.
// It is primarily a test for the race detector (`make race`): the
// traversals run on snapshots, so they must never observe a torn digraph,
// and the accounting must balance at the end.
func TestDigraphConcurrent(t *testing.T) {
	const n = 64
	const writers = 4
	const perWriter = 500

	g := NewDigraph(n)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Concurrent readers: traversals, Adj snapshots, and reads.
	for r := range 3 {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				bp := NewBFSDirectedPaths(g, 0)
				_ = bp.HasPathTo(n - 1)
				_, _ = bp.PathTo(n - 1)
				_, _ = bp.DistTo(n - 1)
				dp := NewDFSDirectedPaths(g, 0)
				_ = dp.HasPathTo(n - 1)
				_, _ = dp.PathTo(n - 1)
				dd := NewDirectedDFS(g, 0, n/2)
				_ = dd.Marked(n - 1)
				_ = dd.Count()
				c := NewDirectedCycle(g)
				_ = c.HasCycle()
				_ = c.Cycle()
				o := NewDepthFirstOrder(g)
				_ = o.Pre()
				_ = o.Post()
				_ = o.ReversePost()
				top := NewTopological(g)
				_ = top.HasOrder()
				_ = top.Order()
				scc := NewKosarajuSCC(g)
				_ = scc.Count()
				_ = scc.StronglyConnected(0, n-1)
				_, _ = scc.ID(r)
				rg := g.Reverse()
				_ = rg.E()
				for range g.Adj(r) {
				}
				_, _ = g.OutDegree(r)
				_, _ = g.InDegree(r)
				_ = g.HasEdge(0, n-1)
				_ = g.V()
				_ = g.Len()
				_ = g.E()
			}
		}(r)
	}

	// Concurrent writers.
	var writersWG sync.WaitGroup
	for w := range writers {
		writersWG.Add(1)
		go func(w int) {
			defer writersWG.Done()
			for i := range perWriter {
				if !g.AddEdge((w*7+i)%n, (i*13+w)%n) {
					t.Errorf("AddEdge returned false on in-range vertices")
					return
				}
			}
		}(w)
	}
	writersWG.Wait()
	close(stop)
	wg.Wait()

	// Every accepted edge is in the digraph: parallel edges are allowed,
	// so the edge count must match the number of AddEdge calls exactly.
	if got, want := g.E(), writers*perWriter; got != want {
		t.Errorf("Expected E %d after concurrent writes, got %d", want, got)
	}
	checkInvariantsAfterConcurrent(t, g, n)
}

// checkInvariantsAfterConcurrent verifies the structural invariants of a
// concurrently built digraph: the directed handshaking lemma (out-degree
// sum == in-degree sum == E), HasEdge for every listed out-neighbor, and
// OutDegree == len(Adj).
func checkInvariantsAfterConcurrent(t *testing.T, g *Digraph, n int) {
	t.Helper()
	outSum, inSum := 0, 0
	for v := 0; v < n; v++ {
		adj := adjOf(g, v)
		outSum += len(adj)
		if d, ok := g.OutDegree(v); !ok || d != len(adj) {
			t.Errorf("OutDegree(%d)=%d (ok=%v), Adj has %d neighbors", v, d, ok, len(adj))
		}
		if d, ok := g.InDegree(v); !ok {
			t.Errorf("InDegree(%d) reported out of range", v)
		} else {
			inSum += d
		}
		for _, w := range adj {
			if !g.HasEdge(v, w) {
				t.Errorf("HasEdge(%d, %d) missing", v, w)
			}
		}
	}
	if outSum != g.E() || inSum != g.E() {
		t.Errorf("Degree sums: out %d, in %d, E %d — all must be equal", outSum, inSum, g.E())
	}
	// BFS from 0 and Kosaraju's strong components must agree on
	// reachability within component 0's vertices.
	scc := NewKosarajuSCC(g)
	bp := NewBFSDirectedPaths(g, 0)
	for v := 0; v < n; v++ {
		if scc.StronglyConnected(0, v) && !bp.HasPathTo(v) {
			t.Errorf("StronglyConnected(0, %d) but BFS cannot reach %d", v, v)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

func TestMarshalJSON(t *testing.T) {
	// Exact object output: edges by ascending source vertex, then in
	// adjacency insertion order; a self-loop counts once.
	g := NewDigraph(4)
	for _, e := range [][2]int{{0, 2}, {0, 1}, {2, 2}, {1, 2}, {3, 0}} {
		g.AddEdge(e[0], e[1])
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"vertices":[0,1,2,3],"edges":[[0,2],[0,1],[1,2],[2,2],[3,0]]}`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// An edgeless digraph still lists every vertex.
	if b, err := json.Marshal(NewDigraph(3)); err != nil || string(b) != `{"vertices":[0,1,2],"edges":[]}` {
		t.Errorf("Expected an edgeless digraph to list its vertices, got (%s, %v)", b, err)
	}

	// A zero-value digraph is a tolerated read: empty vertices and edges.
	var zero Digraph
	if b, err := zero.MarshalJSON(); err != nil || string(b) != `{"vertices":[],"edges":[]}` {
		t.Errorf("Expected empty document for a zero-value digraph, got (%s, %v)", b, err)
	}

	// A direct call on a nil digraph encodes as {}; json.Marshal on a
	// nil *Digraph never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilGraph *Digraph
	if b, err := nilGraph.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} from a direct nil-digraph call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilGraph); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil digraph, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: the round-trip reproduces the
	// identical digraph, adjacency insertion order included.
	src := newTinyDG(t)
	n, edges := parseAlgs4Digraph(t, tinyDGData)
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	dst := NewDigraph(1)
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, dst, mustModel(t, n, edges), len(edges))

	// Unmarshaling replaces the current contents.
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	if err := g.UnmarshalJSON([]byte(`{"vertices":[0,1,2],"edges":[[0,2],[1,2]]}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	checkInvariants(t, g, [][]int{{2}, {2}, nil}, 2)

	// null, {}, and an empty document clear the digraph to the zero
	// value and are tolerated everywhere.
	for _, data := range []string{"null", "{}", `{"vertices":[],"edges":[]}`} {
		if err := g.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s to be tolerated, got %v", data, err)
		}
		if g.V() != 0 || g.E() != 0 {
			t.Errorf("Expected %s to clear the digraph, got V=%d E=%d", data, g.V(), g.E())
		}
	}

	// A zero-value digraph has no constructor-set state, so it is
	// rebuilt in place.
	var zero Digraph
	if err := zero.UnmarshalJSON([]byte(`{"vertices":[0,1],"edges":[[0,1],[1,1]]}`)); err != nil {
		t.Fatalf("UnmarshalJSON on a zero-value digraph: %v", err)
	}
	checkInvariants(t, &zero, [][]int{{1}, {1}}, 2)
}

// mustModel builds the reference adjacency lists for checkInvariants.
func mustModel(t *testing.T, n int, edges [][2]int) [][]int {
	t.Helper()
	model := make([][]int, n)
	for _, e := range edges {
		model[e[0]] = append(model[e[0]], e[1])
	}
	return model
}

// TestUnmarshalJSONDecodeErrors verifies that malformed JSON, a
// non-object document, wrong field types, a non-contiguous vertex list,
// and an out-of-range edge are all returned as errors and leave the
// digraph untouched.
func TestUnmarshalJSONDecodeErrors(t *testing.T) {
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(0, 2)
	keep := mustModel(t, 3, [][2]int{{0, 1}, {0, 2}})

	for _, data := range []string{
		`{"vertices":[0,1,2],"edges":[[0,1]`,  // truncated JSON
		`[0,1,2]`,                             // a non-object document
		`{"vertices":"x","edges":[]}`,         // wrong field type
		`{"vertices":[0,2],"edges":[]}`,       // vertices not 0..n-1 in order
		`{"vertices":[0,1],"edges":[[0,2]]}`,  // edge endpoint out of range
		`{"vertices":[0,1],"edges":[[-1,0]]}`, // negative edge endpoint
	} {
		if err := g.UnmarshalJSON([]byte(data)); err == nil {
			t.Errorf("Expected an error unmarshaling %s", data)
		}
	}
	checkInvariants(t, g, keep, 2)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing vertices or edges into a nil digraph panics with a
// message naming the method, while null, {}, and an empty document —
// which store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilGraph *Digraph
	for _, data := range []string{"null", "{}", `{"vertices":[],"edges":[]}`} {
		if err := nilGraph.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil digraph to be tolerated, got %v", data, err)
		}
	}
	for _, data := range []string{`{"vertices":[0],"edges":[]}`, `{"vertices":[0],"edges":[[0,0]]}`} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected UnmarshalJSON(%s) to panic on a nil digraph.", data)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil digraph") {
					t.Errorf("Unexpected panic message: %v", r)
				}
			}()
			_ = nilGraph.UnmarshalJSON([]byte(data))
		}()
	}
}

// TestJSONStructField marshals and unmarshals a Digraph nested in a
// struct through the encoding/json package.  For a nil *Digraph field
// the json package allocates a zero-value digraph itself — harmless
// here, since a digraph carries no constructor-set state.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Name string   `json:"name"`
		Net  *Digraph `json:"net"`
	}

	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	b, err := json.Marshal(Doc{Name: "x", Net: g})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"name":"x","net":{"vertices":[0,1,2],"edges":[[0,1],[1,2]]}}`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	var doc Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if doc.Name != "x" || doc.Net == nil {
		t.Fatalf("Round-trip lost the struct fields: %+v", doc)
	}
	checkInvariants(t, doc.Net, [][]int{{1}, {2}, nil}, 2)
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against the reference adjacency-list model at fixed seed: every
// round-trip must reproduce the identical digraph.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260903)) // fixed seed: deterministic run

	const n = 40
	g := NewDigraph(n)
	model := make([][]int, n)
	edgeCount := 0

	for step := range 500 {
		v, w := rng.Intn(n), rng.Intn(n)
		if !g.AddEdge(v, w) {
			t.Fatalf("step %d: AddEdge(%d, %d) returned false on in-range vertices", step, v, w)
		}
		model[v] = append(model[v], w)
		edgeCount++

		if step%50 == 0 {
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatalf("step %d: json.Marshal: %v", step, err)
			}
			var back Digraph // zero-value: rebuilt in place
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("step %d: json.Unmarshal: %v", step, err)
			}
			checkInvariants(t, &back, model, edgeCount)
			// The re-encoded document is byte-identical: the encoding
			// is deterministic.
			if b2, err := json.Marshal(&back); err != nil || string(b2) != string(b) {
				t.Fatalf("step %d: re-marshal mismatch: %s vs %s (%v)", step, b2, b, err)
			}
		}
	}
}

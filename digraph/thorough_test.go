package digraph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
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
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

func TestMarshalJSON(t *testing.T) {
	// Exact object output: vertex count, then edges in natural iteration
	// order (source ascending, out-neighbors in insertion order).
	g := buildDigraph(t, 5, [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}})
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"vertices":5,"edges":[[0,1],[0,2],[1,3],[2,3],[3,4]]}`; string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// Self-loops and parallel edges survive the encoding.
	g2 := buildDigraph(t, 2, [][2]int{{0, 0}, {1, 0}, {1, 0}})
	if b, err := json.Marshal(g2); err != nil || string(b) != `{"vertices":2,"edges":[[0,0],[1,0],[1,0]]}` {
		t.Errorf("Unexpected encoding of self-loop/parallel edges: (%s, %v)", b, err)
	}

	// An edgeless digraph encodes with an empty edge list.
	if b, err := json.Marshal(NewDigraph(3)); err != nil || string(b) != `{"vertices":3,"edges":[]}` {
		t.Errorf("Expected edgeless digraph to encode with [], got (%s, %v)", b, err)
	}

	// A zero-value digraph is a tolerated read: no vertices, no edges.
	var zero Digraph
	if b, err := zero.MarshalJSON(); err != nil || string(b) != `{"vertices":0,"edges":[]}` {
		t.Errorf("Expected zero-value digraph to encode as empty, got (%s, %v)", b, err)
	}

	// A direct call on a nil digraph encodes as {}; json.Marshal on a
	// nil *Digraph never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilG *Digraph
	if b, err := nilG.MarshalJSON(); err != nil || string(b) != `{}` {
		t.Errorf("Expected {} from a direct nil-digraph call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilG); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil digraph, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// A round trip through JSON rebuilds an identical digraph: adjacency
	// insertion order and the in-degree bookkeeping included.
	g := newTinyDG(t)
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewDigraph(1)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	model := make([][]int, g.V())
	for v := 0; v < g.V(); v++ {
		model[v] = adjOf(g, v)
	}
	checkInvariants(t, again, model, g.E())

	// A second round trip of the rebuilt digraph is byte-identical.
	b2, err := json.Marshal(again)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Errorf("Round trip not stable: %s vs %s", b, b2)
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`{"vertices":2,"edges":[[0,1]]}`), again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again, [][]int{{1}, nil}, 1)

	// null, {}, and an empty document clear the digraph.
	for _, data := range []string{"null", "{}", `{"vertices":0,"edges":[]}`} {
		full := buildDigraph(t, 2, [][2]int{{0, 1}})
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if full.V() != 0 || full.E() != 0 {
			t.Errorf("Expected %s to clear the digraph, got V=%d E=%d", data, full.V(), full.E())
		}
	}

	// Decode and validation errors are returned and leave the digraph
	// untouched.
	keep := buildDigraph(t, 3, [][2]int{{0, 1}, {1, 2}})
	keepModel := [][]int{{1}, {2}, nil}
	for _, badData := range []string{
		`{"vertices":3,`,                  // truncated
		`[0,1]`,                           // not an object
		`{"vertices":"3"}`,                // wrong field type
		`{"vertices":-1}`,                 // negative vertex count
		`{"vertices":2,"edges":[[0,2]]}`,  // endpoint out of range
		`{"vertices":2,"edges":[[-1,0]]}`, // negative endpoint
		`{"vertices":0,"edges":[[0,0]]}`,  // edge with no vertices
	} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		checkInvariants(t, keep, keepModel, 2)
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing into a nil digraph panics with a message naming the
// method, while null/{}/{"vertices":0} — which store nothing — are
// tolerated even on a nil digraph.  A zero-value digraph is replaced
// wholesale (there is no constructor-set function to lose), so storing
// into one works.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilG *Digraph
	for _, data := range []string{"null", "{}", `{"vertices":0,"edges":[]}`} {
		if err := nilG.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil digraph to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with vertices to panic on a nil digraph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "digraph") || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil digraph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilG.UnmarshalJSON([]byte(`{"vertices":2,"edges":[[0,1]]}`))
	}()
	// Vertices alone (no edges) are still a store.
	expectPanic(t, "UnmarshalJSON on a nil digraph with vertices only", func() {
		_ = nilG.UnmarshalJSON([]byte(`{"vertices":3,"edges":[]}`))
	})

	// A zero-value digraph accepts a full replacement.
	var zero Digraph
	if err := json.Unmarshal([]byte(`{"vertices":2,"edges":[[0,1]]}`), &zero); err != nil {
		t.Fatalf("json.Unmarshal into a zero-value digraph: %v", err)
	}
	checkInvariants(t, &zero, [][]int{{1}, nil}, 1)
}

// TestJSONStructField marshals and unmarshals a Digraph nested in a
// struct through the encoding/json package.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Name string   `json:"name"`
		Net  *Digraph `json:"net"`
	}

	doc := Doc{Name: "web", Net: buildDigraph(t, 3, [][2]int{{0, 1}, {1, 2}})}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"name":"web","net":{"vertices":3,"edges":[[0,1],[1,2]]}}`; string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	var back Doc
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, back.Net, [][]int{{1}, {2}, nil}, 2)

	// For a nil *Digraph field the json package allocates a zero-value
	// digraph itself; that is fine here because the replacement needs no
	// constructor-set function.
	var fresh Doc
	if err := json.Unmarshal([]byte(`{"name":"x","net":{"vertices":2,"edges":[[1,0]]}}`), &fresh); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, fresh.Net, [][]int{nil, {0}}, 1)
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against the adjacency-list reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260903)) // fixed seed: deterministic run

	const n = 25
	g := NewDigraph(n)
	model := make([][]int, n)
	edgeCount := 0

	for step := range 400 {
		switch rng.Intn(4) {
		case 0, 1, 2: // AddEdge (incl. possible self-loops/parallel edges)
			v, w := rng.Intn(n), rng.Intn(n)
			if !g.AddEdge(v, w) {
				t.Fatalf("step %d: AddEdge(%d, %d) returned false on in-range vertices", step, v, w)
			}
			model[v] = append(model[v], w)
			edgeCount++
		case 3: // JSON round trip must reproduce the model exactly
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatalf("step %d: json.Marshal: %v", step, err)
			}
			again := NewDigraph(1)
			if err := json.Unmarshal(b, again); err != nil {
				t.Fatalf("step %d: json.Unmarshal: %v", step, err)
			}
			checkInvariants(t, again, model, edgeCount)
			// The rebuilt digraph re-encodes byte-identically.
			if b2, err := json.Marshal(again); err != nil || string(b2) != string(b) {
				t.Fatalf("step %d: re-encode mismatch: (%s, %v) vs %s", step, b2, err, b)
			}
		}
	}
	checkInvariants(t, g, model, edgeCount)
}

// TestJSONFormatIsDocumented pins the wire format so a change to it is a
// deliberate act: fmt prints the exact document for a known digraph.
func TestJSONFormatIsDocumented(t *testing.T) {
	g := buildDigraph(t, 4, [][2]int{{0, 1}, {0, 2}, {1, 3}})
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if got, want := fmt.Sprintf("%s", b), `{"vertices":4,"edges":[[0,1],[0,2],[1,3]]}`; got != want {
		t.Errorf("Wire format changed: expected %s, got %s", want, got)
	}
}

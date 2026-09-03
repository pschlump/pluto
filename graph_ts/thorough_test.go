package graph_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"slices"
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

// -------------------------------------------------------------------------------------------------------
// Concurrency (run under -race)
// -------------------------------------------------------------------------------------------------------

// TestGraphConcurrent hammers one graph with concurrent AddEdge writers
// while readers run full traversals (DFS/BFS/CC), iterate Adj snapshots,
// and call the read operations.  It is primarily a test for the race
// detector (`make race`): the traversals run on snapshots, so they must
// never observe a torn graph, and the accounting must balance at the end.
func TestGraphConcurrent(t *testing.T) {
	const n = 64
	const writers = 4
	const perWriter = 500

	g := NewGraph(n)

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
				bp := NewBFSPaths(g, 0)
				_ = bp.HasPathTo(n - 1)
				_, _ = bp.PathTo(n - 1)
				_, _ = bp.DistTo(n - 1)
				dp := NewDFSPaths(g, 0)
				_ = dp.HasPathTo(n - 1)
				_, _ = dp.PathTo(n - 1)
				c := NewCC(g)
				_ = c.Count()
				_ = c.Connected(0, n-1)
				_, _ = c.ID(r)
				for range g.Adj(r) {
				}
				_, _ = g.Degree(r)
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

	// Every accepted edge is in the graph: parallel edges are allowed, so
	// the edge count must match the number of AddEdge calls exactly.
	if got, want := g.E(), writers*perWriter; got != want {
		t.Errorf("Expected E %d after concurrent writes, got %d", want, got)
	}
	checkInvariantsAfterConcurrent(t, g, n, writers, perWriter)
}

// checkInvariantsAfterConcurrent verifies the structural invariants of a
// concurrently built graph: handshaking (degree sum == 2E), HasEdge in
// both directions for every listed neighbor, and Degree == len(Adj).
func checkInvariantsAfterConcurrent(t *testing.T, g *Graph, n, writers, perWriter int) {
	t.Helper()
	degreeSum := 0
	for v := 0; v < n; v++ {
		adj := adjOf(g, v)
		degreeSum += len(adj)
		if d, ok := g.Degree(v); !ok || d != len(adj) {
			t.Errorf("Degree(%d)=%d (ok=%v), Adj has %d neighbors", v, d, ok, len(adj))
		}
		for _, w := range adj {
			if !g.HasEdge(v, w) || !g.HasEdge(w, v) {
				t.Errorf("HasEdge(%d, %d) missing in one direction", v, w)
			}
		}
	}
	if degreeSum != 2*g.E() {
		t.Errorf("Degree sum %d != 2*E %d", degreeSum, 2*g.E())
	}
	// The full graph is connected enough that BFS/CC agree: vertex 0
	// reaches at least the vertices touched by writer 0.
	c := NewCC(g)
	bp := NewBFSPaths(g, 0)
	for v := 0; v < n; v++ {
		if bp.HasPathTo(v) != c.Connected(0, v) {
			t.Errorf("BFS HasPathTo(%d)=%v but CC Connected(0, %d)=%v", v, bp.HasPathTo(v), v, c.Connected(0, v))
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// canonicalTinyG is the adjacency model of newTinyG after a JSON round
// trip: the edge list is emitted in canonical order (ascending lower
// endpoint), so the rebuilt adjacency lists hold lower-vertex neighbors
// before higher-vertex ones.
var canonicalTinyG = [][]int{{5, 1, 2}, {0, 2}, {0, 1, 4, 3}, {2, 4, 5}, {2, 3}, {0, 3}}

func TestMarshalJSON(t *testing.T) {
	// Exact object output: vertex list, then the edge list in canonical
	// order (ascending lower endpoint, insertion order within it).
	g := newTinyG()
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"vertices":[0,1,2,3,4,5],"edges":[[0,5],[0,1],[0,2],[1,2],[2,4],[2,3],[3,4],[3,5]]}`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// Self-loops are emitted once (as they count in E) and parallel
	// edges are kept.
	g2 := NewGraph(2)
	g2.AddEdge(0, 0)
	g2.AddEdge(0, 1)
	g2.AddEdge(0, 1)
	if b, err := json.Marshal(g2); err != nil || string(b) != `{"vertices":[0,1],"edges":[[0,0],[0,1],[0,1]]}` {
		t.Errorf("Unexpected encoding of self-loop/parallel edges: (%s, %v)", b, err)
	}

	// An edgeless constructed graph keeps its vertices.
	if b, err := json.Marshal(NewGraph(3)); err != nil || string(b) != `{"vertices":[0,1,2],"edges":[]}` {
		t.Errorf("Expected an empty edge list for an edgeless graph, got (%s, %v)", b, err)
	}

	// A zero-value graph is a tolerated read: an empty document.
	var zero Graph
	if b, err := zero.MarshalJSON(); err != nil || string(b) != `{"vertices":[],"edges":[]}` {
		t.Errorf("Expected an empty document for a zero-value graph, got (%s, %v)", b, err)
	}

	// A direct call on a nil graph encodes as an empty document;
	// json.Marshal on a nil *Graph never reaches the method — the json
	// package writes null for nil pointers itself.
	var nilG *Graph
	if b, err := nilG.MarshalJSON(); err != nil || string(b) != `{"vertices":[],"edges":[]}` {
		t.Errorf("Expected an empty document from a direct nil-graph call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilG); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil graph, got (%s, %v)", b, err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Round trip: V/E/Degree/HasEdge are preserved exactly, with the
	// adjacency lists in canonical order.
	g := newTinyG()
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	rt := NewGraph(1) // resized by the unmarshal
	if err := json.Unmarshal(b, rt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, rt, canonicalTinyG, 8)

	// The canonical form is a fixed point: a second round trip encodes
	// identically.
	b2, err := json.Marshal(rt)
	if err != nil || string(b2) != string(b) {
		t.Errorf("Expected a fixed point on the second round trip, got (%s, %v)", b2, err)
	}

	// Unmarshaling replaces the contents, resizing the vertex set up.
	g2 := NewGraph(2)
	g2.AddEdge(0, 1)
	if err := g2.UnmarshalJSON([]byte(`{"vertices":[0,1,2,3],"edges":[[0,1],[1,2],[2,3]]}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	checkInvariants(t, g2, [][]int{{1}, {0, 2}, {1, 3}, {2}}, 3)

	// ... and down.
	if err := g2.UnmarshalJSON([]byte(`{"vertices":[0,1],"edges":[[1,1]]}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	checkInvariants(t, g2, [][]int{nil, {1, 1}}, 1)

	// null, {}, and an empty document clear the edges and keep the
	// vertex set.
	for _, data := range []string{"null", "{}", `{"vertices":[],"edges":[]}`} {
		g3 := newTinyG()
		if err := g3.UnmarshalJSON([]byte(data)); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", data, err)
		}
		checkInvariants(t, g3, make([][]int, 6), 0)
	}

	// Decode errors leave the graph untouched.
	keep := newTinyG()
	tinyGModel := [][]int{{5, 1, 2}, {2, 0}, {4, 3, 1, 0}, {2, 4, 5}, {2, 3}, {0, 3}}
	for _, data := range []string{
		"not json",
		"[1,2]",                                  // a non-object document
		`{"vertices":"x"}`,                       // wrong field type
		`{"vertices":[1,2,3],"edges":[]}`,        // vertices must start at 0
		`{"vertices":[0,2],"edges":[]}`,          // vertices must be consecutive
		`{"vertices":[0,1],"edges":[[0,2]]}`,     // endpoint out of range
		`{"vertices":[0,1],"edges":[[0,-1]]}`,    // negative endpoint
		`{"vertices":null,"edges":[[0,1]]}`,      // no vertices, so no edge is in range
		`{"vertices":[0,1],"edges":[[0,1]],xxx}`, // trailing garbage
	} {
		if err := keep.UnmarshalJSON([]byte(data)); err == nil {
			t.Errorf("Expected a decode error for %s", data)
		}
	}
	checkInvariants(t, keep, tinyGModel, 8)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing vertices or edges into a nil or zero-value graph panics
// with a message naming the method and the fix, while null, {}, and an
// empty document — which store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Graph
	for _, data := range []string{"null", "{}", `{"vertices":[],"edges":[]}`} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value graph to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with vertices to panic on a zero-value graph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewGraph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`{"vertices":[0],"edges":[]}`))
	}()

	var nilG *Graph
	for _, data := range []string{"null", "{}", `{"vertices":[],"edges":[]}`} {
		if err := nilG.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil graph to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with edges to panic on a nil graph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil graph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilG.UnmarshalJSON([]byte(`{"vertices":[0,1],"edges":[[0,1]]}`))
	}()
}

// TestJSONStructField marshals and unmarshals a Graph nested in a struct
// through the encoding/json package.  The graph must be created with
// NewGraph before unmarshaling: for a nil *Graph field the json package
// allocates a zero-value graph itself, so non-empty data panics with the
// insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Name  string `json:"name"`
		Graph *Graph `json:"graph"`
	}

	d := Doc{Name: "tiny", Graph: newTinyG()}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	d2 := Doc{Graph: NewGraph(1)} // constructed, then resized by the unmarshal
	if err := json.Unmarshal(b, &d2); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if d2.Name != "tiny" {
		t.Errorf("Expected name %q, got %q", "tiny", d2.Name)
	}
	checkInvariants(t, d2.Graph, canonicalTinyG, 8)

	// A null graph field unmarshals to a nil *Graph.
	var d3 Doc
	if err := json.Unmarshal([]byte(`{"name":"x","graph":null}`), &d3); err != nil || d3.Graph != nil {
		t.Errorf("Expected a nil graph from null, got (%v, %v)", d3.Graph, err)
	}

	// A nil *Graph field is allocated as a zero-value graph by the json
	// package, so non-empty data panics with the insert-family message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into a nil graph field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewGraph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal(b, &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a reference adjacency model at fixed seed: a round trip must
// preserve V, E, and the per-vertex neighbor multisets, and the
// re-marshaled canonical form must be byte-identical.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(43)) // fixed seed: deterministic run

	const n = 30
	g := NewGraph(n)
	model := make([][]int, n)
	e := 0

	verify := func(step int) {
		b, err := json.Marshal(g)
		if err != nil {
			t.Fatalf("step %d: json.Marshal: %v", step, err)
		}
		rt := NewGraph(1)
		if err := json.Unmarshal(b, rt); err != nil {
			t.Fatalf("step %d: json.Unmarshal(%s): %v", step, b, err)
		}
		if rt.V() != n || rt.E() != e {
			t.Fatalf("step %d: round trip changed V/E to %d/%d, expected %d/%d", step, rt.V(), rt.E(), n, e)
		}
		for v := 0; v < n; v++ {
			got, want := adjOf(rt, v), append([]int(nil), model[v]...)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("step %d: round trip changed the neighbors of %d to %v, expected %v", step, v, got, want)
			}
		}
		// The canonical form is a fixed point.
		if b2, err := json.Marshal(rt); err != nil || string(b2) != string(b) {
			t.Fatalf("step %d: second marshal (%s, %v) differs from the first %s", step, b2, err, b)
		}
	}

	for step := range 400 {
		v, w := rng.Intn(n), rng.Intn(n) // includes self-loops and parallel edges
		if !g.AddEdge(v, w) {
			t.Fatalf("step %d: AddEdge(%d, %d) returned false on in-range vertices", step, v, w)
		}
		model[v] = append(model[v], w)
		model[w] = append(model[w], v)
		e++
		if step%40 == 0 {
			verify(step)
		}
	}
	verify(400)
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with AddEdge writers and a marshaling reader; every output must be a
// valid JSON document with the fixed vertex count.  Run under -race.
func TestJSONConcurrent(t *testing.T) {
	const n = 6
	g := NewGraph(n)

	stop := make(chan struct{})
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock, so
	// it is safe while the writers add edges and replace the contents.
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := g.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe graphJSON
			if err := json.Unmarshal(b, &probe); err != nil {
				t.Errorf("MarshalJSON produced invalid JSON %s: %v", b, err)
				return
			}
			if len(probe.Vertices) != n {
				t.Errorf("MarshalJSON produced %d vertices, expected %d", len(probe.Vertices), n)
				return
			}
		}
	}()

	// Concurrent writers: AddEdge, plus marshal/unmarshal round trips
	// (every document keeps the vertex count at n, so AddEdge never goes
	// out of range).
	var writers sync.WaitGroup
	for w := range 4 {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range 200 {
				if !g.AddEdge((w+i)%n, (i*5+w)%n) {
					t.Errorf("worker %d: AddEdge returned false on in-range vertices", w)
					return
				}
				if i%10 == 0 {
					b, err := g.MarshalJSON()
					if err != nil {
						t.Errorf("worker %d: %v", w, err)
						return
					}
					if err := g.UnmarshalJSON(b); err != nil {
						t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
						return
					}
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	// A final unmarshal of the tinyG document must rebuild it exactly.
	if err := g.UnmarshalJSON([]byte(`{"vertices":[0,1,2,3,4,5],"edges":[[0,5],[0,1],[0,2],[1,2],[2,4],[2,3],[3,4],[3,5]]}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	checkInvariants(t, g, canonicalTinyG, 8)
}

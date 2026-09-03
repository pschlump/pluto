package graph

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
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

func TestGraphMarshalJSON(t *testing.T) {
	// Exact output on the tinyG trace graph: the vertex list is 0..n-1
	// and each edge appears once as [v, w] with v <= w, sorted.
	g := newTinyG()
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal(g): %v", err)
	}
	want := `{"vertices":[0,1,2,3,4,5],"edges":[[0,1],[0,2],[0,5],[1,2],[2,3],[2,4],[3,4],[3,5]]}`
	if string(b) != want {
		t.Errorf("Expected %s, got %s", want, b)
	}

	// A self-loop counts once; parallel edges each appear.
	g2 := NewGraph(3)
	g2.AddEdge(2, 2)
	g2.AddEdge(1, 2)
	g2.AddEdge(1, 2)
	if b, err := json.Marshal(g2); err != nil || string(b) != `{"vertices":[0,1,2],"edges":[[1,2],[1,2],[2,2]]}` {
		t.Errorf(`Expected {"vertices":[0,1,2],"edges":[[1,2],[1,2],[2,2]]}, got (%s, %v)`, b, err)
	}

	// An edgeless graph still lists every vertex.
	if b, err := json.Marshal(NewGraph(2)); err != nil || string(b) != `{"vertices":[0,1],"edges":[]}` {
		t.Errorf(`Expected {"vertices":[0,1],"edges":[]} for an edgeless graph, got (%s, %v)`, b, err)
	}

	// A zero-value graph is a tolerated read: empty lists.
	var zero Graph
	if b, err := zero.MarshalJSON(); err != nil || string(b) != `{"vertices":[],"edges":[]}` {
		t.Errorf(`Expected {"vertices":[],"edges":[]} for a zero-value graph, got (%s, %v)`, b, err)
	}

	// A direct call on a nil graph encodes as {}; json.Marshal on a nil
	// *Graph never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilGraph *Graph
	if b, err := nilGraph.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} from a direct nil-graph call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilGraph); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil graph, got (%s, %v)", b, err)
	}
}

func TestGraphUnmarshalJSON(t *testing.T) {
	// Round-trip: the decoded graph has the same vertices and the same
	// edge multiset, whatever the previous size; the adjacency lists come
	// back in the canonical sorted order of the edge list.
	g := newTinyG()
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("json.Marshal(g): %v", err)
	}
	back := NewGraph(1)
	if err := json.Unmarshal(b, back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back.V() != 6 || back.E() != 8 {
		t.Fatalf("Expected V=6 E=8 after round-trip, got V=%d E=%d", back.V(), back.E())
	}
	wantAdj := [][]int{{1, 2, 5}, {0, 2}, {0, 1, 3, 4}, {2, 4, 5}, {2, 3}, {0, 3}}
	for v, want := range wantAdj {
		if got := adjOf(back, v); !reflect.DeepEqual(got, want) {
			t.Errorf("Adj(%d) mismatch after round-trip, expected %v got %v", v, want, got)
		}
	}
	// The encoding is a fixed point: re-marshaling is byte-identical.
	if b2, err := json.Marshal(back); err != nil || string(b2) != string(b) {
		t.Errorf("Expected re-marshal to be byte-identical, got (%s, %v) want %s", b2, err, b)
	}

	// Self-loops and parallel edges survive the round-trip.
	g2 := NewGraph(3)
	g2.AddEdge(2, 2)
	g2.AddEdge(1, 2)
	g2.AddEdge(1, 2)
	b2, err := json.Marshal(g2)
	if err != nil {
		t.Fatalf("json.Marshal(g2): %v", err)
	}
	back2 := NewGraph(3)
	if err := json.Unmarshal(b2, back2); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back2.E() != 3 {
		t.Errorf("Expected E 3 after round-trip, got %d", back2.E())
	}
	if d, _ := back2.Degree(2); d != 4 { // self-loop twice + two parallel edges
		t.Errorf("Expected degree 4 for vertex 2, got %d", d)
	}
	if got := adjOf(back2, 2); !reflect.DeepEqual(got, []int{1, 1, 2, 2}) {
		t.Errorf("Expected Adj(2) = [1 1 2 2] after round-trip, got %v", got)
	}

	// The unmarshaled graph stays fully usable.
	back.AddEdge(0, 5)
	if !back.HasEdge(5, 0) {
		t.Errorf("Expected the unmarshaled graph to accept AddEdge.")
	}

	// A zero-value graph is rebuilt in place — there is no
	// constructor-set state to preserve.
	var zero Graph
	if err := json.Unmarshal([]byte(`{"vertices":[0,1],"edges":[[0,1]]}`), &zero); err != nil {
		t.Fatalf("json.Unmarshal on a zero-value graph: %v", err)
	}
	if zero.V() != 2 || zero.E() != 1 || !zero.HasEdge(0, 1) {
		t.Errorf("Expected the zero-value graph rebuilt with V=2 E=1, got V=%d E=%d", zero.V(), zero.E())
	}

	// An empty object, empty lists, and null clear the graph to the zero
	// value and are tolerated everywhere.
	for _, data := range []string{"{}", `{"vertices":[],"edges":[]}`, "null"} {
		clearMe := newTinyG()
		if err := json.Unmarshal([]byte(data), clearMe); err != nil {
			t.Errorf("Expected %s to clear the graph, got error %v", data, err)
		}
		if clearMe.V() != 0 || clearMe.E() != 0 {
			t.Errorf("Expected %s to clear the graph, got V=%d E=%d", data, clearMe.V(), clearMe.E())
		}
		var zeroG Graph
		if err := zeroG.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value graph to be tolerated, got %v", data, err)
		}
		var nilG *Graph
		if err := nilG.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil graph to be tolerated, got %v", data, err)
		}
	}
}

func TestGraphUnmarshalJSONErrors(t *testing.T) {
	for _, badData := range []string{
		"[",                                    // malformed JSON
		"[1,2]",                                // not an object
		`{"vertices":"x"}`,                     // wrong field type
		`{"vertices":[0,2]}`,                   // vertices not the range 0..n-1
		`{"vertices":[0,1],"edges":[[0,2]]}`,   // edge endpoint out of range
		`{"vertices":[0,1],"edges":[[-1,0]]}`,  // negative edge endpoint
		`{"edges":[[0,0]]}`,                    // edges with no vertices
		`{"vertices":[0,1],"edges":[[0,1],1]}`, // wrong edge type
	} {
		keep := newTinyG()
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.V() != 6 || keep.E() != 8 {
			t.Errorf("Graph changed after the error on %s: V=%d E=%d", badData, keep.V(), keep.E())
		}
	}
}

// TestGraphUnmarshalJSONPanics verifies that UnmarshalJSON joins the
// insert family: storing vertices or edges into a nil graph panics with a
// message naming the method and the problem, while the empty documents —
// which store nothing — are tolerated everywhere.
func TestGraphUnmarshalJSONPanics(t *testing.T) {
	var nilGraph *Graph
	if err := nilGraph.UnmarshalJSON([]byte("{}")); err != nil {
		t.Errorf("Expected {} on a nil graph to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with vertices to panic on a nil graph.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil graph") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilGraph.UnmarshalJSON([]byte(`{"vertices":[0],"edges":[]}`))
	}()
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
		_ = nilGraph.UnmarshalJSON([]byte(`{"vertices":[0],"edges":[[0,0]]}`))
	}()
}

// TestGraphJSONStructField marshals and unmarshals a Graph nested in a
// struct through the encoding/json package.  A graph has no
// constructor-set state, so even a nil *Graph field round-trips: the
// json package allocates a zero-value graph and UnmarshalJSON rebuilds
// it in place.
func TestGraphJSONStructField(t *testing.T) {
	type Doc struct {
		Name string `json:"name"`
		G    *Graph `json:"graph"`
	}

	d := Doc{Name: "tinyG", G: newTinyG()}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal(Doc): %v", err)
	}

	var back Doc
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal(Doc): %v", err)
	}
	if back.Name != "tinyG" || back.G == nil || back.G.V() != 6 || back.G.E() != 8 {
		t.Fatalf("Round-trip through a struct field failed: name=%q V=%d E=%d",
			back.Name, back.G.V(), back.G.E())
	}
	for v := 0; v < 6; v++ {
		got, want := adjOf(back.G, v), adjOf(d.G, v)
		slices.Sort(got)
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Adj(%d) mismatch after struct round-trip, expected %v got %v", v, want, got)
		}
	}
	// The nested graph re-marshals byte-identically.
	if b2, err := json.Marshal(back); err != nil || string(b2) != string(b) {
		t.Errorf("Expected re-marshal to be byte-identical, got (%s, %v) want %s", b2, err, b)
	}
}

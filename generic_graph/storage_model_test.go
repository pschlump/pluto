package generic_graph

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

// orderedRef is a deliberately naive reference for the exact iteration-order
// contract of InsertionOrder graphs: nodes in (re-)insertion order, edges in
// insertion order with their insertion orientation, and adjacency lists in
// edge-insertion order with removals compacted.
type orderedRef struct {
	directed bool
	nodes    []int
	succ     map[int][]int
	pred     map[int][]int
	edges    []EndpointPair[int]
	values   map[EndpointPair[int]]int
}

func newOrderedRef(directed bool) *orderedRef {
	return &orderedRef{directed: directed, succ: map[int][]int{}, pred: map[int][]int{}, values: map[EndpointPair[int]]int{}}
}

func (r *orderedRef) hasNode(n int) bool { return slices.Contains(r.nodes, n) }

func (r *orderedRef) find(u, v int) (EndpointPair[int], bool) {
	for _, e := range r.edges {
		if (e.Source == u && e.Target == v) || (!r.directed && e.Source == v && e.Target == u) {
			return e, true
		}
	}
	return EndpointPair[int]{}, false
}

func deleteFirst(list []int, x int) []int {
	if i := slices.Index(list, x); i >= 0 {
		return slices.Delete(list, i, i+1)
	}
	return list
}

func (r *orderedRef) connect(u, v int) {
	r.succ[u] = append(r.succ[u], v)
	r.pred[v] = append(r.pred[v], u)
	if !r.directed && u != v {
		r.succ[v] = append(r.succ[v], u)
		r.pred[u] = append(r.pred[u], v)
	}
}

func (r *orderedRef) disconnect(e EndpointPair[int]) {
	u, v := e.Source, e.Target
	r.succ[u] = deleteFirst(r.succ[u], v)
	r.pred[v] = deleteFirst(r.pred[v], u)
	if !r.directed && u != v {
		r.succ[v] = deleteFirst(r.succ[v], u)
		r.pred[u] = deleteFirst(r.pred[u], v)
	}
	r.edges = slices.DeleteFunc(r.edges, func(x EndpointPair[int]) bool { return x == e })
	delete(r.values, e)
}

func (r *orderedRef) degree(n int) int {
	if r.directed {
		return len(r.succ[n]) + len(r.pred[n])
	}
	d := len(r.succ[n])
	if _, loop := r.find(n, n); loop {
		d++
	}
	return d
}

// storageCase selects the node index (native map or Func/hash_grow) and the
// degree shape (uniform, or hub-heavy so out-lists cross indexThreshold and
// the per-node position index is built, grown, rebuilt and dropped).
type storageCase struct {
	name     string
	funcCfg  bool
	universe int
	hubs     bool
}

func TestGraphStorageDifferential(t *testing.T) {
	cases := []storageCase{
		{name: "native", universe: 24},
		{name: "func", funcCfg: true, universe: 24},
		{name: "hubs", universe: 72, hubs: true},
		{name: "func-hubs", funcCfg: true, universe: 72, hubs: true},
	}
	for _, c := range cases {
		for _, directed := range []bool{true, false} {
			for _, seed := range []int64{1, 7, 42, 1729} {
				name := fmt.Sprintf("%s/%s/seed%d", c.name, map[bool]string{true: "directed", false: "undirected"}[directed], seed)
				t.Run(name, func(t *testing.T) {
					runStorageDifferential(t, c, directed, seed)
				})
			}
		}
	}
}

func runStorageDifferential(t *testing.T, c storageCase, directed bool, seed int64) {
	random := rand.New(rand.NewSource(seed))
	graph := NewGraph[int](directed, true)
	values := NewValueGraph[int, int](directed, true)
	if c.funcCfg {
		config := Config[int]{Directed: directed, AllowSelfLoops: true, Order: InsertionOrder,
			Equal: func(a, b int) bool { return a == b }, Hash: func(v int) uint64 { return uint64(v) * 0x9E3779B97F4A7C15 }}
		graph = NewGraphFunc(config)
		values = NewValueGraphFunc[int, int](config)
		if graph.nodes.table == nil || values.graph.nodes.table == nil {
			t.Fatal("Func configuration must use the Equal/Hash index")
		}
	} else if graph.nodes.table != nil {
		t.Fatal("natural configuration must use the native index")
	}
	ref := newOrderedRef(directed)
	var snapshot *Graph[int]
	var snapshotRef [][]int
	universe, checkEvery := c.universe, 7
	if c.hubs {
		checkEvery = 53 // checks are O(universe²·E) against the naive reference
	}
	for step := range 4000 {
		u, v := random.Intn(universe), random.Intn(universe)
		if c.hubs && random.Intn(2) == 0 {
			u = random.Intn(3) // concentrate edges on hubs 0..2
		}
		switch op := random.Intn(20); {
		case op < 3:
			errG, errV := graph.AddNode(u), values.AddNode(u)
			if ref.hasNode(u) != errors.Is(errG, ErrDuplicateNode) || (errG == nil) != (errV == nil) {
				t.Fatalf("seed %d step %d AddNode(%d): %v/%v", seed, step, u, errG, errV)
			}
			if errG == nil {
				ref.nodes = append(ref.nodes, u)
			}
		case op < 12:
			value := random.Intn(1000)
			errG, errV := graph.AddEdge(u, v), values.AddEdge(u, v, value)
			_, dup := ref.find(u, v)
			wantOK := ref.hasNode(u) && ref.hasNode(v) && !dup
			if (errG == nil) != wantOK || (errV == nil) != wantOK {
				t.Fatalf("seed %d step %d AddEdge(%d,%d): %v/%v want ok=%v", seed, step, u, v, errG, errV, wantOK)
			}
			if wantOK {
				e := EndpointPair[int]{u, v}
				ref.edges = append(ref.edges, e)
				ref.values[e] = value
				ref.connect(u, v)
			}
		case op < 15:
			errG, errV := graph.RemoveEdge(u, v), values.RemoveEdge(u, v)
			e, found := ref.find(u, v)
			wantOK := ref.hasNode(u) && ref.hasNode(v) && found
			if (errG == nil) != wantOK || (errV == nil) != wantOK {
				t.Fatalf("seed %d step %d RemoveEdge(%d,%d): %v/%v", seed, step, u, v, errG, errV)
			}
			if wantOK {
				ref.disconnect(e)
			}
		case op < 16:
			errG, errV := graph.RemoveNode(u), values.RemoveNode(u)
			if (errG == nil) != ref.hasNode(u) || (errV == nil) != ref.hasNode(u) {
				t.Fatalf("seed %d step %d RemoveNode(%d): %v/%v", seed, step, u, errG, errV)
			}
			if errG == nil {
				for _, e := range slices.Clone(ref.edges) {
					if e.Source == u || e.Target == u {
						ref.disconnect(e)
					}
				}
				ref.nodes = deleteFirst(ref.nodes, u)
				delete(ref.succ, u)
				delete(ref.pred, u)
			}
		case op < 18:
			value := random.Intn(1000)
			err := values.SetEdgeValue(u, v, value)
			e, found := ref.find(u, v)
			if (err == nil) != (found && ref.hasNode(u) && ref.hasNode(v)) {
				t.Fatalf("seed %d step %d SetEdgeValue: %v", seed, step, err)
			}
			if err == nil {
				ref.values[e] = value
			}
		case op < 19:
			// A copy must be independent of later mutation of the source.
			snapshot = graph.Copy()
			snapshotRef = graphSnapshot(graph)
			if random.Intn(2) == 0 {
				// Continue on copies: their capped list windows and cloned
				// indexes must take every later mutation correctly.
				graph, values = graph.Copy(), values.Copy()
			}
		default:
			if snapshot != nil {
				if !slices.EqualFunc(graphSnapshot(snapshot), snapshotRef, slices.Equal) {
					t.Fatalf("seed %d step %d copy changed after source mutation", seed, step)
				}
				// Mutating the copy must not leak into the source either.
				before := graphSnapshot(graph)
				for _, n := range snapshot.NodesSlice() {
					_ = snapshot.AddEdge(n, n)
					_ = snapshot.RemoveNode(n)
					break
				}
				if !slices.EqualFunc(graphSnapshot(graph), before, slices.Equal) {
					t.Fatalf("seed %d step %d copy mutation leaked into source", seed, step)
				}
				snapshot = nil
			}
		}
		if step%checkEvery == 0 || step == 3999 {
			checkOrderedRef(t, seed, step, graph, ref)
			checkOrderedRef(t, seed, step, values.graph, ref)
			checkRefValues(t, seed, step, values, ref, universe)
			checkOrderedRef(t, seed, step, values.Copy().graph, ref)
		}
	}
}

// graphSnapshot flattens nodes, edges and adjacency into comparable slices.
func graphSnapshot(g *Graph[int]) [][]int {
	var out [][]int
	out = append(out, g.NodesSlice())
	var edges []int
	for e := range g.Edges() {
		edges = append(edges, e.Source, e.Target)
	}
	out = append(out, edges)
	for _, n := range g.NodesSlice() {
		out = append(out, g.SuccessorsSlice(n), g.PredecessorsSlice(n))
	}
	return out
}

func checkOrderedRef(t *testing.T, seed int64, step int, g *Graph[int], ref *orderedRef) {
	t.Helper()
	if got := g.NodesSlice(); !slices.Equal(got, ref.nodes) {
		t.Fatalf("seed %d step %d nodes=%v want %v", seed, step, got, ref.nodes)
	}
	if got := slices.Collect(g.Edges()); !slices.Equal(got, ref.edges) {
		t.Fatalf("seed %d step %d edges=%v want %v", seed, step, got, ref.edges)
	}
	if g.EdgeCount() != len(ref.edges) || g.NodeCount() != len(ref.nodes) {
		t.Fatalf("seed %d step %d counts %d/%d", seed, step, g.NodeCount(), g.EdgeCount())
	}
	for _, n := range ref.nodes {
		if got := g.SuccessorsSlice(n); !slices.Equal(got, ref.succ[n]) {
			t.Fatalf("seed %d step %d succ(%d)=%v want %v", seed, step, n, got, ref.succ[n])
		}
		if got := g.PredecessorsSlice(n); !slices.Equal(got, ref.pred[n]) {
			t.Fatalf("seed %d step %d pred(%d)=%v want %v", seed, step, n, got, ref.pred[n])
		}
		if got := slices.Collect(g.Successors(n)); !slices.Equal(got, ref.succ[n]) {
			t.Fatalf("seed %d step %d Successors(%d)=%v want %v", seed, step, n, got, ref.succ[n])
		}
		d, _ := g.Degree(n)
		in, _ := g.InDegree(n)
		out, _ := g.OutDegree(n)
		if d != ref.degree(n) || in != len(ref.pred[n]) || out != len(ref.succ[n]) {
			t.Fatalf("seed %d step %d degree(%d)=%d/%d/%d want %d/%d/%d", seed, step, n, d, in, out, ref.degree(n), len(ref.pred[n]), len(ref.succ[n]))
		}
		incident, err := g.IncidentEdges(n)
		want := slices.DeleteFunc(slices.Clone(ref.edges), func(e EndpointPair[int]) bool { return e.Source != n && e.Target != n })
		if err != nil || !slices.Equal(incident, want) {
			t.Fatalf("seed %d step %d incident(%d)=%v want %v", seed, step, n, incident, want)
		}
	}
}

func checkRefValues(t *testing.T, seed int64, step int, g *ValueGraph[int, int], ref *orderedRef, universe int) {
	t.Helper()
	for u := range universe {
		for v := range universe {
			got, found := g.EdgeValue(u, v)
			e, want := ref.find(u, v)
			if found != want || g.HasEdge(u, v) != want || (want && got != ref.values[e]) {
				t.Fatalf("seed %d step %d EdgeValue(%d,%d)=%d/%v want %d/%v", seed, step, u, v, got, found, ref.values[e], want)
			}
		}
	}
}

// TestGraphOrderModes checks that Unordered graphs hold the same contents as
// insertion-ordered ones and that SortedOrder sorts every snapshot.
func TestGraphOrderModes(t *testing.T) {
	for _, directed := range []bool{true, false} {
		random := rand.New(rand.NewSource(99))
		base := naturalConfig[int](directed, true)
		unordered, sorted := base, base
		unordered.Order = Unordered
		sorted.Order = SortedOrder
		sorted.Less = func(a, b int) bool { return a < b }
		graphs := []*Graph[int]{NewGraphFunc(base), NewGraphFunc(unordered), NewGraphFunc(sorted)}
		for range 2000 {
			u, v := random.Intn(30), random.Intn(30)
			for _, g := range graphs {
				switch random := (u*31 + v) % 9; {
				case random < 2:
					_ = g.AddNode(u)
				case random < 7:
					_ = g.AddEdge(u, v)
				case random < 8:
					_ = g.RemoveEdge(u, v)
				default:
					if u%5 == 0 {
						_ = g.RemoveNode(u)
					}
				}
			}
		}
		sortedCopy := func(values []int) []int { return slices.Sorted(slices.Values(values)) }
		for n := range 30 {
			want := sortedCopy(graphs[0].SuccessorsSlice(n))
			if got := sortedCopy(graphs[1].SuccessorsSlice(n)); !slices.Equal(got, want) {
				t.Fatalf("unordered succ(%d)=%v want %v", n, got, want)
			}
			if got := graphs[2].SuccessorsSlice(n); !slices.Equal(got, want) {
				t.Fatalf("sorted succ(%d)=%v want %v", n, got, want)
			}
			wantPred := sortedCopy(graphs[0].PredecessorsSlice(n))
			if got := graphs[2].PredecessorsSlice(n); !slices.Equal(got, wantPred) {
				t.Fatalf("sorted pred(%d)=%v want %v", n, got, wantPred)
			}
		}
		if got, want := graphs[2].NodesSlice(), sortedCopy(graphs[0].NodesSlice()); !slices.Equal(got, want) {
			t.Fatalf("sorted nodes=%v want %v", got, want)
		}
		edges := slices.Collect(graphs[2].Edges())
		if len(edges) != graphs[0].EdgeCount() || !slices.IsSortedFunc(edges, func(a, b EndpointPair[int]) int {
			if a.Source != b.Source {
				return a.Source - b.Source
			}
			return a.Target - b.Target
		}) {
			t.Fatalf("sorted edges not sorted or wrong size: %v", edges)
		}
	}
}

// TestGraphConstructionAllocations pins the arena: building a sparse graph
// must not cost per-list growth allocations.
func TestGraphConstructionAllocations(t *testing.T) {
	const nodes, edges = 1024, 4096
	random := rand.New(rand.NewSource(5))
	pairs := make([][2]int, edges)
	for i := range pairs {
		pairs[i] = [2]int{random.Intn(nodes), random.Intn(nodes)}
	}
	allocs := testing.AllocsPerRun(5, func() {
		config := naturalConfig[int](true, false)
		config.ExpectedNodes = nodes
		g := NewGraphFunc(config)
		for n := range nodes {
			_ = g.AddNode(n)
		}
		for _, p := range pairs {
			_ = g.AddEdge(p[0], p[1])
		}
	})
	// Previously ~10 allocations per node; the arena and flat per-node
	// adjacency bring it to well under one.
	if allocs > nodes/2 {
		t.Fatalf("construction allocations = %.0f, want <= %d", allocs, nodes/2)
	}
}

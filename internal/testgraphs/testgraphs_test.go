/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package testgraphs

import (
	"slices"
	"testing"
)

// checkSimple asserts the simple-graph invariants: every endpoint in
// range, no self loops, no duplicate (unordered) pairs, exact count.
func checkSimple(t *testing.T, name string, g G) {
	t.Helper()
	seen := make(map[[2]int]bool)
	for _, e := range g.Edges {
		if e[0] < 0 || e[0] >= g.V || e[1] < 0 || e[1] >= g.V {
			t.Fatalf("%s: edge %v outside 0..%d", name, e, g.V-1)
		}
		if e[0] == e[1] {
			t.Fatalf("%s: self loop %v", name, e)
		}
		if key := uKey(e[0], e[1]); seen[key] {
			t.Fatalf("%s: duplicate edge %v", name, e)
		} else {
			seen[key] = true
		}
	}
}

// degrees returns the degree of every vertex (undirected reading of
// the edge list).
func degrees(g G) []int {
	deg := make([]int, g.V)
	for _, e := range g.Edges {
		deg[e[0]]++
		deg[e[1]]++
	}
	return deg
}

// connectedOn reports whether the non-isolated vertices form one
// connected piece (undirected).
func connectedOn(g G) bool {
	adj := make([][]int, g.V)
	for _, e := range g.Edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		adj[e[1]] = append(adj[e[1]], e[0])
	}
	seed := -1
	for v, d := range degrees(g) {
		if d > 0 {
			seed = v
			break
		}
	}
	if seed < 0 {
		return true // nothing to connect
	}
	seen := make([]bool, g.V)
	seen[seed] = true
	stack := []int{seed}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, w := range adj[v] {
			if !seen[w] {
				seen[w] = true
				stack = append(stack, w)
			}
		}
	}
	for v, d := range degrees(g) {
		if d > 0 && !seen[v] {
			return false
		}
	}
	return true
}

func TestSimpleFamily(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(20)
		maxE := v * (v - 1) / 2
		e := rng.Intn(maxE + 1)

		sg := Simple(rng, v, e)
		checkSimple(t, "Simple", sg)
		if len(sg.Edges) != e {
			t.Fatalf("Simple: got %d edges, want %d", len(sg.Edges), e)
		}

		er := ErdosRenyi(rng, v, float64(rng.Intn(101))/100)
		checkSimple(t, "ErdosRenyi", er)
	}
	if got := Complete(6); len(got.Edges) != 15 {
		t.Errorf("Complete(6): %d edges, want 15", len(got.Edges))
	}
	checkSimple(t, "Complete", Complete(7))

	// Determinism: the same seed reproduces the same graph bit for bit.
	a := Simple(newRand(), 30, 60)
	b := Simple(newRand(), 30, 60)
	if !slices.Equal(a.Edges, b.Edges) {
		t.Error("Simple is not deterministic across equal seeds")
	}
}

func TestShapes(t *testing.T) {
	rng := newRand()

	path := Path(rng, 9)
	checkSimple(t, "Path", path)
	if len(path.Edges) != 8 || !connectedOn(path) {
		t.Error("Path is not a path")
	}
	// nil rand = the identity order: 0-1-2-…
	if got := Path(nil, 4); !slices.Equal(got.Edges, [][2]int{{0, 1}, {1, 2}, {2, 3}}) {
		t.Errorf("Path(nil, 4) = %v, want the identity path", got.Edges)
	}

	cyc := Cycle(rng, 9)
	checkSimple(t, "Cycle", cyc)
	if deg := degrees(cyc); len(cyc.Edges) != 9 || slices.ContainsFunc(deg, func(d int) bool { return d != 2 }) {
		t.Error("Cycle is not 2-regular")
	}

	star := Star(nil, 8) // identity order: the hub is label 0
	checkSimple(t, "Star", star)
	for i, d := range degrees(star) {
		want := 1
		if i == 0 {
			want = 7
		}
		if d != want {
			t.Errorf("Star degree of %d = %d, want %d", i, d, want)
		}
	}

	wheel := Wheel(rng, 9)
	checkSimple(t, "Wheel", wheel)
	if len(wheel.Edges) != 2*9-2 {
		t.Errorf("Wheel(9): %d edges, want %d", len(wheel.Edges), 2*9-2)
	}

	bt := BinaryTree(rng, 15)
	checkSimple(t, "BinaryTree", bt)
	if len(bt.Edges) != 14 || !connectedOn(bt) {
		t.Error("BinaryTree is not a tree")
	}

	grid := Grid(3, 5)
	checkSimple(t, "Grid", grid)
	if len(grid.Edges) != 2*5+3*4 {
		t.Errorf("Grid(3,5): %d edges, want %d", len(grid.Edges), 2*5+3*4)
	}
	// The grid is bipartite by (i+j)%2.
	for _, e := range grid.Edges {
		if (e[0]/5+e[0]%5)%2 == (e[1]/5+e[1]%5)%2 {
			t.Fatalf("Grid: edge %v joins same-color cells", e)
		}
	}
}

func TestTree(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(24)
		tree := Tree(rng, v)
		checkSimple(t, "Tree", tree)
		if len(tree.Edges) != v-1 {
			t.Fatalf("Tree(%d): %d edges, want %d", v, len(tree.Edges), v-1)
		}
		if !connectedOn(tree) {
			t.Fatalf("Tree(%d) is not connected", v)
		}
	}
	if one := Tree(newRand(), 1); len(one.Edges) != 0 {
		t.Error("Tree(1) has edges")
	}
}

func TestBipartiteFamily(t *testing.T) {
	rng := newRand()
	cross := func(b B) {
		t.Helper()
		for _, e := range b.Edges {
			if (e[0] < b.V1) == (e[1] < b.V1) {
				t.Fatalf("edge %v does not cross the [0,%d)/[%d,%d) sides", e, b.V1, b.V1, b.V)
			}
		}
	}
	for range 100 {
		v1 := 1 + rng.Intn(10)
		v2 := 1 + rng.Intn(10)
		b := Bipartite(rng, v1, v2, rng.Intn(v1*v2+1))
		cross(b)
		checkSimple(t, "Bipartite", b.G)

		cross(BipartiteProb(rng, v1, v2, float64(rng.Intn(101))/100))
	}
	full := CompleteBipartite(3, 4)
	cross(full)
	if len(full.Edges) != 12 || full.V != 7 || full.V1 != 3 {
		t.Errorf("CompleteBipartite(3,4): %d edges on V=%d V1=%d, want 12 on 7/3", len(full.Edges), full.V, full.V1)
	}
}

func TestEulerianWalks(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(12)
		e := 1 + rng.Intn(40)

		cyc := EulerianCycle(rng, v, e)
		if len(cyc.Edges) != e {
			t.Fatalf("EulerianCycle: %d edges, want %d", len(cyc.Edges), e)
		}
		for i, d := range degrees(cyc) {
			if d%2 != 0 {
				t.Fatalf("EulerianCycle: vertex %d has odd degree %d", i, d)
			}
		}
		if !connectedOn(cyc) {
			t.Fatal("EulerianCycle: the walked part is disconnected")
		}

		path := EulerianPath(rng, v, rng.Intn(40))
		odd := 0
		for _, d := range degrees(path) {
			if d%2 != 0 {
				odd++
			}
		}
		if odd != 0 && odd != 2 {
			t.Fatalf("EulerianPath: %d odd-degree vertices, want 0 or 2", odd)
		}
		if !connectedOn(path) {
			t.Fatal("EulerianPath: the walked part is disconnected")
		}
	}
}

// reach returns the boolean transitive closure of a digraph edge list.
func reach(g G) [][]bool {
	cl := make([][]bool, g.V)
	for i := range cl {
		cl[i] = make([]bool, g.V)
		cl[i][i] = true
	}
	for _, e := range g.Edges {
		cl[e[0]][e[1]] = true
	}
	for k := range cl {
		for i := range cl {
			if !cl[i][k] {
				continue
			}
			for j := range cl {
				if cl[k][j] {
					cl[i][j] = true
				}
			}
		}
	}
	return cl
}

func checkSimpleDigraph(t *testing.T, name string, g G) {
	t.Helper()
	seen := make(map[[2]int]bool)
	for _, e := range g.Edges {
		if e[0] < 0 || e[0] >= g.V || e[1] < 0 || e[1] >= g.V {
			t.Fatalf("%s: edge %v outside 0..%d", name, e, g.V-1)
		}
		if e[0] == e[1] {
			t.Fatalf("%s: self loop %v", name, e)
		}
		if seen[e] {
			t.Fatalf("%s: duplicate ordered edge %v", name, e)
		}
		seen[e] = true
	}
}

func TestDigraphGenerators(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(16)

		sd := SimpleDigraph(rng, v, rng.Intn(v*(v-1)+1))
		checkSimpleDigraph(t, "SimpleDigraph", sd)

		checkSimpleDigraph(t, "ErdosRenyiDigraph", ErdosRenyiDigraph(rng, v, float64(rng.Intn(101))/100))

		// DAG: every edge x -> y with x < y, so 0..V-1 is a topological
		// order — in particular acyclic.
		dag := DAG(rng, v, rng.Intn(v*(v-1)/2+1))
		checkSimpleDigraph(t, "DAG", dag)
		for _, e := range dag.Edges {
			if e[0] >= e[1] {
				t.Fatalf("DAG: edge %v violates the identity topological order", e)
			}
		}

		// Tournament: exactly one directed edge per unordered pair — the
		// normalized-pair count and the total must both be v·(v−1)/2.
		tt := Tournament(rng, v)
		both := make(map[[2]int]bool)
		for _, e := range tt.Edges {
			if e[0] < e[1] {
				both[e] = true
			} else {
				both[[2]int{e[1], e[0]}] = true
			}
		}
		if len(tt.Edges) != v*(v-1)/2 || len(both) != v*(v-1)/2 {
			t.Fatalf("Tournament(%d) is not a tournament (%d edges, %d pairs)", v, len(tt.Edges), len(both))
		}

		// Rooted DAGs: the root reaches / is reached, and the graph is a DAG.
		e := v - 1 + rng.Intn(max(1, v*(v-1)/2-v+2))
		in := RootedInDAG(rng, v, e)
		checkSimpleDigraph(t, "RootedInDAG", in)
		if len(in.Edges) != e {
			t.Fatalf("RootedInDAG: %d edges, want %d", len(in.Edges), e)
		}
		cl := reach(in)
		for x := range v {
			if !cl[x][v-1] {
				t.Fatalf("RootedInDAG: vertex %d cannot reach the root %d", x, v-1)
			}
		}
		out := RootedOutDAG(rng, v, e)
		cl = reach(out)
		for x := range v {
			if !cl[0][x] {
				t.Fatalf("RootedOutDAG: the root 0 cannot reach vertex %d", x)
			}
		}

		// Trees are the minimal rooted DAGs.
		if got := len(RootedInTree(rng, v).Edges); v > 1 && got != v-1 {
			t.Fatalf("RootedInTree(%d): %d edges, want %d", v, got, v-1)
		}
		if got := len(RootedOutTree(rng, v).Edges); v > 1 && got != v-1 {
			t.Fatalf("RootedOutTree(%d): %d edges, want %d", v, got, v-1)
		}
	}

	cd := CycleDigraph(nil, 5)
	if len(cd.Edges) != 5 || !reach(cd)[4][0] {
		t.Error("CycleDigraph(nil, 5) is not the identity directed cycle")
	}
	if got := PathDigraph(nil, 3); !slices.Equal(got.Edges, [][2]int{{0, 1}, {1, 2}}) {
		t.Errorf("PathDigraph(nil, 3) = %v, want 0->1->2", got.Edges)
	}
	if got := len(CompleteDigraph(4).Edges); got != 12 {
		t.Errorf("CompleteDigraph(4): %d edges, want 12", got)
	}
	bt := BinaryTreeDigraph(7)
	if len(bt.Edges) != 6 {
		t.Errorf("BinaryTreeDigraph(7): %d edges, want 6", len(bt.Edges))
	}
	for _, e := range bt.Edges {
		if e[1] != (e[0]-1)/2 {
			t.Fatalf("BinaryTreeDigraph: %v is not a node-to-parent edge", e)
		}
	}
}

func TestDirectedEulerianWalks(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(10)
		e := 1 + rng.Intn(30)

		cyc := EulerianCycleDigraph(rng, v, e)
		if len(cyc.Edges) != e {
			t.Fatalf("EulerianCycleDigraph: %d edges, want %d", len(cyc.Edges), e)
		}
		indeg, outdeg := make([]int, v), make([]int, v)
		for _, ed := range cyc.Edges {
			outdeg[ed[0]]++
			indeg[ed[1]]++
		}
		if !slices.Equal(indeg, outdeg) {
			t.Fatalf("EulerianCycleDigraph: in-degrees %v != out-degrees %v", indeg, outdeg)
		}

		path := EulerianPathDigraph(rng, v, rng.Intn(30))
		indeg, outdeg = make([]int, v), make([]int, v)
		for _, ed := range path.Edges {
			outdeg[ed[0]]++
			indeg[ed[1]]++
		}
		imbalanced := 0
		for x := range v {
			switch {
			case outdeg[x] == indeg[x]+1, indeg[x] == outdeg[x]+1:
				imbalanced++
			case indeg[x] != outdeg[x]:
				t.Fatalf("EulerianPathDigraph: vertex %d has in %d out %d", x, indeg[x], outdeg[x])
			}
		}
		if imbalanced > 2 {
			t.Fatalf("EulerianPathDigraph: %d imbalanced vertices, want <= 2", imbalanced)
		}
	}
}

// TestStrongOracle is the reason the package exists: the generated
// labels must BE the strong-component partition — strongly connected
// exactly when labels are equal, checked against a boolean transitive
// closure.
func TestStrongOracle(t *testing.T) {
	rng := newRand()
	for range 100 {
		v := 1 + rng.Intn(20)
		c := 1 + rng.Intn(v)
		lo := 2 * (v - c)
		hi := v * (v - 1) / 2
		if lo >= hi {
			continue
		}
		e := lo + 1 + rng.Intn(hi-lo)

		sg := Strong(rng, v, e, c)
		if len(sg.Edges) != e {
			t.Fatalf("Strong: %d edges, want %d", len(sg.Edges), e)
		}
		cl := reach(sg.G)
		distinct := make(map[int]bool)
		for x := range v {
			distinct[sg.Label[x]] = true
			for y := range v {
				want := sg.Label[x] == sg.Label[y]
				if (cl[x][y] && cl[y][x]) != want {
					t.Fatalf("Strong: strongly-connected(%d,%d)=%v, labels say %v (label %d vs %d)",
						x, y, cl[x][y] && cl[y][x], want, sg.Label[x], sg.Label[y])
				}
			}
		}
		if got := len(distinct); got != c {
			t.Fatalf("Strong: %d distinct labels, want exactly c=%d", got, c)
		}
	}
}

func TestValidationPanics(t *testing.T) {
	rng := newRand()
	expectPanic(t, "testgraphs: Simple called with e=8, too many", func() { Simple(rng, 4, 8) })
	expectPanic(t, "testgraphs: Simple called with e=-1", func() { Simple(rng, 4, -1) })
	expectPanic(t, "testgraphs: Simple called with v=0", func() { Simple(rng, 0, 0) })
	expectPanic(t, "testgraphs: ErdosRenyi called with p=1.5", func() { ErdosRenyi(rng, 4, 1.5) })
	expectPanic(t, "testgraphs: Path called with v=0", func() { Path(nil, 0) })
	expectPanic(t, "testgraphs: Wheel called with v=1", func() { Wheel(nil, 1) })
	expectPanic(t, "testgraphs: Star called with v=0", func() { Star(nil, 0) })
	expectPanic(t, "testgraphs: BinaryTree called with v=0", func() { BinaryTree(nil, 0) })
	expectPanic(t, "testgraphs: Tree called with v=0", func() { Tree(rng, 0) })
	expectPanic(t, "testgraphs: Bipartite called with e=7", func() { Bipartite(rng, 2, 3, 7) })
	expectPanic(t, "testgraphs: CompleteBipartite called with v1=0", func() { CompleteBipartite(0, 3) })
	expectPanic(t, "testgraphs: EulerianCycle called with e=0", func() { EulerianCycle(rng, 3, 0) })
	expectPanic(t, "testgraphs: EulerianPath called with e=-1", func() { EulerianPath(rng, 3, -1) })
	expectPanic(t, "testgraphs: Grid called with rows=0", func() { Grid(0, 3) })
	expectPanic(t, "testgraphs: SimpleDigraph called with e=13", func() { SimpleDigraph(rng, 4, 13) })
	expectPanic(t, "testgraphs: DAG called with e=7", func() { DAG(rng, 4, 7) })
	expectPanic(t, "testgraphs: Tournament called with v=0", func() { Tournament(rng, 0) })
	expectPanic(t, "testgraphs: RootedInDAG called with e=1, too few", func() { RootedInDAG(rng, 4, 1) })
	expectPanic(t, "testgraphs: RootedOutDAG called with e=0, too few", func() { RootedOutDAG(rng, 4, 0) })
	expectPanic(t, "testgraphs: EulerianCycleDigraph called with e=0", func() { EulerianCycleDigraph(rng, 3, 0) })
	expectPanic(t, "testgraphs: EulerianPathDigraph called with v=0", func() { EulerianPathDigraph(rng, 0, 0) })
	expectPanic(t, "testgraphs: Strong called with c=5", func() { Strong(rng, 4, 6, 5) })
	expectPanic(t, "testgraphs: Strong called with e=2", func() { Strong(rng, 4, 2, 2) })

	// The randomness-requiring generators reject nil.
	expectPanic(t, "testgraphs: Simple called with a nil rand", func() { Simple(nil, 4, 2) })
	expectPanic(t, "testgraphs: Tree called with a nil rand", func() { Tree(nil, 4) })
	expectPanic(t, "testgraphs: DAG called with a nil rand", func() { DAG(nil, 4, 2) })
	expectPanic(t, "testgraphs: Strong called with a nil rand", func() { Strong(nil, 6, 8, 2) })
}

package generic_graph

import (
	"errors"
	"math/rand"
	"slices"
	"testing"
)

type modelPair struct{ u, v int }

func canonicalModelPair(directed bool, u, v int) modelPair {
	if !directed && u > v {
		u, v = v, u
	}
	return modelPair{u, v}
}

func TestGraphRandomizedModel(t *testing.T) {
	for _, directed := range []bool{false, true} {
		t.Run(map[bool]string{false: "undirected", true: "directed"}[directed], func(t *testing.T) {
			random := rand.New(rand.NewSource(42))
			graph := NewGraph[int](directed, true)
			nodes := make(map[int]bool)
			edges := make(map[modelPair]bool)
			for step := range 3000 {
				u, v := random.Intn(16), random.Intn(16)
				switch random.Intn(5) {
				case 0:
					err := graph.AddNode(u)
					if nodes[u] && !errors.Is(err, ErrDuplicateNode) {
						t.Fatalf("step %d duplicate node: %v", step, err)
					}
					if !nodes[u] && err != nil {
						t.Fatalf("step %d add node: %v", step, err)
					}
					nodes[u] = true
				case 1:
					err := graph.AddEdge(u, v)
					key := canonicalModelPair(directed, u, v)
					if !nodes[u] || !nodes[v] {
						if !errors.Is(err, ErrUnknownNode) {
							t.Fatalf("step %d unknown nodes: %v", step, err)
						}
					} else if edges[key] {
						if !errors.Is(err, ErrDuplicateEdge) {
							t.Fatalf("step %d duplicate edge: %v", step, err)
						}
					} else if err != nil {
						t.Fatalf("step %d add edge: %v", step, err)
					} else {
						edges[key] = true
					}
				case 2:
					err := graph.RemoveEdge(u, v)
					key := canonicalModelPair(directed, u, v)
					if !nodes[u] || !nodes[v] {
						if !errors.Is(err, ErrUnknownNode) {
							t.Fatalf("step %d remove unknown nodes: %v", step, err)
						}
					} else if !edges[key] {
						if !errors.Is(err, ErrUnknownEdge) {
							t.Fatalf("step %d unknown edge: %v", step, err)
						}
					} else if err != nil {
						t.Fatalf("step %d remove edge: %v", step, err)
					} else {
						delete(edges, key)
					}
				case 3:
					err := graph.RemoveNode(u)
					if !nodes[u] {
						if !errors.Is(err, ErrUnknownNode) {
							t.Fatalf("step %d remove unknown node: %v", step, err)
						}
					} else if err != nil {
						t.Fatalf("step %d remove node: %v", step, err)
					} else {
						delete(nodes, u)
						for key := range edges {
							if key.u == u || key.v == u {
								delete(edges, key)
							}
						}
					}
				case 4:
					if got := graph.HasEdge(u, v); got != edges[canonicalModelPair(directed, u, v)] {
						t.Fatalf("step %d HasEdge(%d,%d)=%v", step, u, v, got)
					}
				}
				if graph.NodeCount() != len(nodes) || graph.EdgeCount() != len(edges) {
					t.Fatalf("step %d counts=%d/%d want %d/%d", step, graph.NodeCount(), graph.EdgeCount(), len(nodes), len(edges))
				}
				if step%25 == 0 {
					checkGraphModel(t, graph, nodes, edges, directed)
				}
			}
			checkGraphModel(t, graph, nodes, edges, directed)
		})
	}
}

func checkGraphModel(t *testing.T, graph *Graph[int], nodes map[int]bool, edges map[modelPair]bool, directed bool) {
	t.Helper()
	if got := graph.NodesSlice(); len(got) != len(nodes) {
		t.Fatalf("nodes length=%d want %d", len(got), len(nodes))
	}
	for node := range nodes {
		if !graph.ContainsNode(node) {
			t.Fatalf("missing node %d", node)
		}
		out, in := 0, 0
		successors, predecessors := make(map[int]bool), make(map[int]bool)
		for pair := range edges {
			if pair.u == node {
				out++
				successors[pair.v] = true
			}
			if pair.v == node {
				in++
				predecessors[pair.u] = true
			}
		}
		if !directed {
			out, in = 0, 0
			successors, predecessors = make(map[int]bool), make(map[int]bool)
			for pair := range edges {
				if pair.u == pair.v && pair.u == node {
					out += 2
					in += 2
					successors[node] = true
					predecessors[node] = true
				} else if pair.u == node {
					out++
					in++
					successors[pair.v] = true
					predecessors[pair.v] = true
				} else if pair.v == node {
					out++
					in++
					successors[pair.u] = true
					predecessors[pair.u] = true
				}
			}
		}
		if got := graph.SuccessorsSlice(node); !sameIntSet(got, successors) {
			t.Fatalf("node %d successors=%v want %v", node, got, successors)
		}
		if got := graph.PredecessorsSlice(node); !sameIntSet(got, predecessors) {
			t.Fatalf("node %d predecessors=%v want %v", node, got, predecessors)
		}
		degree, err := graph.Degree(node)
		if err != nil {
			t.Fatal(err)
		}
		wantDegree := out + in
		if !directed {
			wantDegree = out
		}
		if degree != wantDegree {
			t.Fatalf("node %d degree=%d want %d", node, degree, wantDegree)
		}
	}
}

func sameIntSet(values []int, set map[int]bool) bool {
	if len(values) != len(set) {
		return false
	}
	for _, value := range values {
		if !set[value] {
			return false
		}
	}
	return true
}

func TestIterationOrdersAndReinsertion(t *testing.T) {
	graph := NewGraph[int](true, false)
	for _, node := range []int{3, 1, 2} {
		_ = graph.AddNode(node)
	}
	if got := graph.NodesSlice(); !slices.Equal(got, []int{3, 1, 2}) {
		t.Fatal(got)
	}
	for _, pair := range [][2]int{{3, 2}, {3, 1}, {2, 1}} {
		_ = graph.AddEdge(pair[0], pair[1])
	}
	if got := graph.SuccessorsSlice(3); !slices.Equal(got, []int{2, 1}) {
		t.Fatal(got)
	}
	_ = graph.RemoveEdge(3, 2)
	_ = graph.AddEdge(3, 2)
	if got := graph.SuccessorsSlice(3); !slices.Equal(got, []int{1, 2}) {
		t.Fatal(got)
	}
	if got := len(slices.Collect(graph.Edges())); got != 3 {
		t.Fatalf("edges after reinsertion=%d", got)
	}
	ordered := NewOrderedGraph[int](true, false)
	for _, node := range []int{3, 1, 2} {
		_ = ordered.AddNode(node)
	}
	for _, pair := range [][2]int{{3, 2}, {3, 1}, {2, 1}} {
		_ = ordered.AddEdge(pair[0], pair[1])
	}
	if got := ordered.NodesSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatal(got)
	}
	if got := ordered.SuccessorsSlice(3); !slices.Equal(got, []int{1, 2}) {
		t.Fatal(got)
	}
}

func TestSelfLoopRules(t *testing.T) {
	for _, directed := range []bool{false, true} {
		graph := NewGraph[int](directed, true)
		_ = graph.AddNode(1)
		_ = graph.AddEdge(1, 1)
		degree, _ := graph.Degree(1)
		in, _ := graph.InDegree(1)
		out, _ := graph.OutDegree(1)
		if degree != 2 || in != 1 || out != 1 {
			t.Fatalf("directed=%v degree/in/out=%d/%d/%d", directed, degree, in, out)
		}
	}
	graph := NewGraph[int](false, false)
	_ = graph.AddNode(1)
	if err := graph.AddEdge(1, 1); !errors.Is(err, ErrSelfLoop) {
		t.Fatal(err)
	}
}

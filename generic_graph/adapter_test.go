package generic_graph

import (
	"math"
	"slices"
	"testing"

	"github.com/pschlump/pluto/digraph"
	"github.com/pschlump/pluto/graph"
)

func TestDenseAdapters(t *testing.T) {
	directed := NewGraph[string](true, false)
	for _, node := range []string{"a", "b", "c", "isolated"} {
		_ = directed.AddNode(node)
	}
	_ = directed.AddEdge("a", "b")
	_ = directed.AddEdge("b", "c")
	adapter := directed.Dense()
	a, _ := adapter.NodeID("a")
	c, _ := adapter.NodeID("c")
	if adapter.IntGraph() != nil || adapter.IntDigraph().V() != 4 {
		t.Fatal("directed adapter mismatch")
	}
	reachable := digraph.NewDirectedDFS(adapter.IntDigraph(), a)
	if !reachable.Marked(c) || reachable.Count() != 3 {
		t.Fatal("adapter reachability mismatch")
	}
	if node, found := adapter.Node(c); !found || node != "c" {
		t.Fatal(node)
	}

	undirected := NewGraph[string](false, false)
	for _, node := range []string{"a", "b", "isolated"} {
		_ = undirected.AddNode(node)
	}
	_ = undirected.AddEdge("a", "b")
	undirectedAdapter := undirected.Dense()
	ua, _ := undirectedAdapter.NodeID("a")
	ub, _ := undirectedAdapter.NodeID("b")
	components := graph.NewCC(undirectedAdapter.IntGraph())
	if !components.Connected(ua, ub) || components.Count() != 2 {
		t.Fatal("undirected adapter mismatch")
	}
	if empty := NewGraph[int](true, false).Dense(); empty.IntGraph() != nil || empty.IntDigraph() != nil || empty.Len() != 0 {
		t.Fatal("empty adapter mismatch")
	}
}

func TestDijkstraAdapters(t *testing.T) {
	for _, directed := range []bool{true, false} {
		values := NewValueGraph[string, float64](directed, false)
		for _, node := range []string{"a", "b", "c", "d", "isolated"} {
			_ = values.AddNode(node)
		}
		_ = values.AddEdge("a", "b", 4)
		_ = values.AddEdge("a", "c", 1)
		_ = values.AddEdge("c", "b", 2)
		_ = values.AddEdge("b", "d", 1)
		paths, err := Dijkstra(values, "a")
		if err != nil {
			t.Fatal(err)
		}
		if distance, found := paths.Distance("d"); !found || distance != 4 {
			t.Fatalf("directed=%v distance=%g/%v", directed, distance, found)
		}
		if path, found := paths.Path("d"); !found || !slices.Equal(path, []string{"a", "c", "b", "d"}) {
			t.Fatalf("path=%v/%v", path, found)
		}
		if path, found := paths.Path("a"); !found || !slices.Equal(path, []string{"a"}) {
			t.Fatalf("source path=%v/%v", path, found)
		}
		if distance, found := paths.Distance("isolated"); !found || !math.IsInf(distance, 1) {
			t.Fatal("unreachable distance mismatch")
		}
		if _, found := paths.Path("isolated"); found {
			t.Fatal("unreachable path found")
		}
	}
	values := NewValueGraph[string, float64](true, false)
	_ = values.AddNode("a")
	_ = values.AddNode("b")
	_ = values.AddEdge("a", "b", -1)
	if _, err := Dijkstra(values, "a"); err == nil {
		t.Fatal("negative weight accepted")
	}
}

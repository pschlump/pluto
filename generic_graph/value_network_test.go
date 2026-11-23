package generic_graph

import (
	"errors"
	"slices"
	"testing"
)

func TestValueGraphSemantics(t *testing.T) {
	graph := NewValueGraph[string, int](false, true)
	for _, node := range []string{"a", "b", "isolated"} {
		_ = graph.AddNode(node)
	}
	if err := graph.AddEdge("a", "b", 0); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddEdge("b", "a", 9); !errors.Is(err, ErrDuplicateEdge) {
		t.Fatal(err)
	}
	if value, found := graph.EdgeValue("b", "a"); !found || value != 0 {
		t.Fatalf("reverse value=%d/%v", value, found)
	}
	if err := graph.SetEdgeValue("b", "a", 7); err != nil {
		t.Fatal(err)
	}
	if value, _ := graph.EdgeValue("a", "b"); value != 7 {
		t.Fatal(value)
	}
	if err := graph.AddEdge("a", "a", 3); err != nil {
		t.Fatal(err)
	}
	if degree, _ := graph.Degree("a"); degree != 3 {
		t.Fatalf("degree=%d", degree)
	}
	copyGraph := graph.Copy()
	if err := graph.RemoveNode("a"); err != nil {
		t.Fatal(err)
	}
	if graph.EdgeCount() != 0 || graph.NodeCount() != 2 || !graph.ContainsNode("isolated") {
		t.Fatal("node removal did not clean values/topology")
	}
	if copyGraph.EdgeCount() != 2 || !copyGraph.HasEdge("a", "b") {
		t.Fatalf("copy aliases source: edges=%d has=%v nodes=%v", copyGraph.EdgeCount(), copyGraph.HasEdge("a", "b"), copyGraph.NodesSlice())
	}
	if _, found := graph.EdgeValue("a", "b"); found {
		t.Fatal("stale edge value")
	}
}

type flexibleNode struct {
	ID    int
	Label []byte
}

func TestCustomFunctions(t *testing.T) {
	graph := NewGraphFunc(Config[flexibleNode]{Directed: true, Equal: func(a, b flexibleNode) bool { return a.ID == b.ID },
		Hash: func(value flexibleNode) uint64 { return uint64(value.ID) * 0x9e3779b97f4a7c15 }, Order: InsertionOrder})
	a := flexibleNode{ID: 1, Label: []byte("a")}
	b := flexibleNode{ID: 2, Label: []byte("b")}
	_ = graph.AddNode(a)
	_ = graph.AddNode(b)
	if err := graph.AddEdge(flexibleNode{ID: 1}, flexibleNode{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if !graph.HasEdge(b, a) && graph.HasEdge(a, b) {
		return
	}
	t.Fatal("custom equality lookup failed")
}

func TestNetworkUniqueParallelAndRemoval(t *testing.T) {
	network := NewNetwork[string, string](true, true, true)
	for _, node := range []string{"a", "b", "c"} {
		_ = network.AddNode(node)
	}
	for _, edge := range []struct{ u, v, id string }{{"a", "b", "ab1"}, {"a", "b", "ab2"}, {"b", "a", "ba"}, {"a", "a", "aa"}} {
		if err := network.AddEdge(edge.u, edge.v, edge.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := network.AddEdge("a", "c", "ab1"); !errors.Is(err, ErrDuplicateEdge) {
		t.Fatal(err)
	}
	if got, _ := network.EdgesConnecting("a", "b"); !slices.Equal(got, []string{"ab1", "ab2"}) {
		t.Fatal(got)
	}
	if !network.HasEdge("b", "a") || network.HasEdge("a", "c") {
		t.Fatal("antiparallel lookup mismatch")
	}
	if degree, _ := network.Degree("a"); degree != 5 {
		t.Fatalf("directed degree=%d", degree)
	}
	if pair, found := network.Endpoints("ba"); !found || pair.Source != "b" || pair.Target != "a" {
		t.Fatal(pair)
	}
	if err := network.CheckEndpoints("ba", "a", "b"); !errors.Is(err, ErrInconsistentEndpoints) {
		t.Fatal(err)
	}
	_ = network.RemoveEdge("ab1")
	if network.EdgeCount() != 3 || network.ContainsEdge("ab1") {
		t.Fatal("edge removal failed")
	}
	_ = network.RemoveNode("a")
	if network.EdgeCount() != 0 || network.NodeCount() != 2 || network.ContainsEdge("ba") {
		t.Fatal("node removal cleanup failed")
	}
}

func TestNetworkRejectsParallelWhenConfigured(t *testing.T) {
	network := NewNetwork[int, int](false, false, false)
	_ = network.AddNode(1)
	_ = network.AddNode(2)
	_ = network.AddEdge(1, 2, 10)
	if err := network.AddEdge(2, 1, 11); !errors.Is(err, ErrParallelEdge) {
		t.Fatal(err)
	}
	_ = network.AddNode(3)
	_ = network.AddEdge(2, 3, 12)
	if degree, _ := network.Degree(2); degree != 2 {
		t.Fatal(degree)
	}
}

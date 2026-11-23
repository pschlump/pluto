package generic_graph

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

type graphGolden struct {
	Graph struct {
		Nodes, Edges, SuccessorsA, PredecessorsB []string
		BreadthFirst, PreOrder, PostOrder        []string
		ReachableC, ClosureAlways, ClosureCycles []string
		Transpose, Induced                       []string
	} `json:"graph"`
	ValueGraph struct {
		Nodes, Edges []string
		AB           int `json:"ab"`
		TransposeBA  int `json:"transposeBA"`
	} `json:"valueGraph"`
	Network struct {
		Nodes, Edges, ConnectingAB []string
		DegreeA                    int `json:"degreeA"`
		IncidentA, BA              []string
	} `json:"network"`
}

func readGraphGolden(t *testing.T) graphGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/guava_graph.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden graphGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

func pairStrings(values []EndpointPair[string]) []string {
	result := make([]string, 0, len(values))
	for _, pair := range values {
		result = append(result, pair.Source+"->"+pair.Target)
	}
	return result
}

func requireStrings(t *testing.T, got, want []string, sort bool) {
	t.Helper()
	if sort {
		got = slices.Clone(got)
		want = slices.Clone(want)
		slices.Sort(got)
		slices.Sort(want)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGuavaGraphDifferential(t *testing.T) {
	golden := readGraphGolden(t)
	graph := traversalFixture(t)
	requireStrings(t, graph.NodesSlice(), golden.Graph.Nodes, false)
	requireStrings(t, pairStrings(slices.Collect(graph.Edges())), golden.Graph.Edges, true)
	requireStrings(t, graph.SuccessorsSlice("a"), golden.Graph.SuccessorsA, false)
	requireStrings(t, graph.PredecessorsSlice("b"), golden.Graph.PredecessorsB, false)
	traverser := ForGraph(graph)
	requireStrings(t, slices.Collect(traverser.BreadthFirst("a")), golden.Graph.BreadthFirst, false)
	requireStrings(t, slices.Collect(traverser.DepthFirstPreOrder("a")), golden.Graph.PreOrder, false)
	requireStrings(t, slices.Collect(traverser.DepthFirstPostOrder("a")), golden.Graph.PostOrder, false)
	reachable, _ := ReachableNodes(graph, "c")
	requireStrings(t, reachable, golden.Graph.ReachableC, false)
	requireStrings(t, pairStrings(slices.Collect(graph.TransitiveClosure(AddSelfLoopsAlways).Edges())), golden.Graph.ClosureAlways, true)
	requireStrings(t, pairStrings(slices.Collect(graph.TransitiveClosure(AddSelfLoopsForCycles).Edges())), golden.Graph.ClosureCycles, true)
	requireStrings(t, pairStrings(slices.Collect(graph.Transpose().Edges())), golden.Graph.Transpose, true)
	induced, _ := graph.InducedSubgraph("a", "b", "d", "isolated")
	requireStrings(t, pairStrings(slices.Collect(induced.Edges())), golden.Graph.Induced, true)

	values := NewValueGraph[string, int](true, false)
	_ = values.AddNode("isolated")
	_ = values.AddNode("a")
	_ = values.AddNode("b")
	_ = values.AddNode("c")
	_ = values.AddEdge("a", "b", 4)
	_ = values.AddEdge("b", "c", 5)
	requireStrings(t, values.NodesSlice(), golden.ValueGraph.Nodes, false)
	requireStrings(t, pairStrings(slices.Collect(values.Edges())), golden.ValueGraph.Edges, true)
	if value, _ := values.EdgeValue("a", "b"); value != golden.ValueGraph.AB {
		t.Fatal(value)
	}
	if value, _ := values.Transpose().EdgeValue("b", "a"); value != golden.ValueGraph.TransposeBA {
		t.Fatal(value)
	}
}

func TestGuavaNetworkDifferential(t *testing.T) {
	golden := readGraphGolden(t)
	network := NewNetwork[string, string](true, true, true)
	for _, node := range []string{"a", "b", "isolated"} {
		_ = network.AddNode(node)
	}
	_ = network.AddEdge("a", "b", "ab1")
	_ = network.AddEdge("a", "b", "ab2")
	_ = network.AddEdge("b", "a", "ba")
	_ = network.AddEdge("a", "a", "aa")
	requireStrings(t, network.NodesSlice(), golden.Network.Nodes, false)
	requireStrings(t, network.EdgesSlice(), golden.Network.Edges, true)
	connecting, _ := network.EdgesConnecting("a", "b")
	requireStrings(t, connecting, golden.Network.ConnectingAB, true)
	if degree, _ := network.Degree("a"); degree != golden.Network.DegreeA {
		t.Fatal(degree)
	}
	incident, _ := network.IncidentEdges("a")
	requireStrings(t, incident, golden.Network.IncidentA, true)
	pair, _ := network.Endpoints("ba")
	requireStrings(t, pairStrings([]EndpointPair[string]{pair}), golden.Network.BA, false)
}

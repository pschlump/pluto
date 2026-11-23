package generic_graph

import (
	"errors"
	"testing"
)

func TestZeroValueReads(t *testing.T) {
	var graph Graph[int]
	if graph.NodeCount() != 0 || graph.EdgeCount() != 0 || graph.ContainsNode(1) || graph.HasEdge(1, 2) || len(graph.NodesSlice()) != 0 || len(graph.SuccessorsSlice(1)) != 0 {
		t.Fatal("zero Graph is not empty")
	}
	if _, err := graph.Degree(1); !errors.Is(err, ErrUnknownNode) {
		t.Fatal(err)
	}
	if err := graph.AddNode(1); err == nil {
		t.Fatal("uninitialized graph write succeeded")
	}
	var values ValueGraph[int, int]
	if values.NodeCount() != 0 || values.ContainsNode(1) || values.HasEdge(1, 2) {
		t.Fatal("zero ValueGraph is not empty")
	}
	if _, found := values.EdgeValue(1, 2); found {
		t.Fatal("zero ValueGraph has an edge value")
	}
	var network Network[int, int]
	if network.NodeCount() != 0 || network.EdgeCount() != 0 || network.ContainsNode(1) || network.ContainsEdge(1) || network.HasEdge(1, 2) {
		t.Fatal("zero Network is not empty")
	}
}

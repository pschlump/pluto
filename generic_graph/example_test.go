package generic_graph_test

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/generic_graph"
)

func ExampleGraph() {
	graph := generic_graph.NewOrderedGraph[string](true, false)
	for _, node := range []string{"a", "b", "c", "isolated"} {
		_ = graph.AddNode(node)
	}
	_ = graph.AddEdge("a", "b")
	_ = graph.AddEdge("b", "c")
	fmt.Println(graph.NodeCount(), graph.EdgeCount())
	fmt.Println(slices.Collect(generic_graph.ForGraph(graph).BreadthFirst("a")))
	// Output:
	// 4 2
	// [a b c]
}

func ExampleNetwork() {
	network := generic_graph.NewNetwork[string, string](true, false, true)
	_ = network.AddNode("LAX")
	_ = network.AddNode("ATL")
	_ = network.AddEdge("LAX", "ATL", "flight-3025")
	_ = network.AddEdge("LAX", "ATL", "flight-1598")
	edges, _ := network.EdgesConnecting("LAX", "ATL")
	fmt.Println(edges)
	// Output: [flight-3025 flight-1598]
}

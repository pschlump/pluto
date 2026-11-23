package generic_graph

import (
	"iter"
	"slices"
	"sync"
	"testing"
)

func traversalFixture(t *testing.T) *Graph[string] {
	t.Helper()
	graph := NewOrderedGraph[string](true, true)
	for _, node := range []string{"a", "b", "c", "d", "e", "isolated"} {
		_ = graph.AddNode(node)
	}
	for _, pair := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"d", "e"}, {"e", "b"}} {
		_ = graph.AddEdge(pair[0], pair[1])
	}
	return graph
}

func TestTraversalOrdersAndEarlyStop(t *testing.T) {
	graph := traversalFixture(t)
	traverser := ForGraph(graph)
	if got := slices.Collect(traverser.BreadthFirst("a")); !slices.Equal(got, []string{"a", "b", "c", "d", "e"}) {
		t.Fatal(got)
	}
	if got := slices.Collect(traverser.DepthFirstPreOrder("a")); !slices.Equal(got, []string{"a", "b", "d", "e", "c"}) {
		t.Fatal(got)
	}
	if got := slices.Collect(traverser.DepthFirstPostOrder("a")); !slices.Equal(got, []string{"e", "d", "b", "c", "a"}) {
		t.Fatal(got)
	}
	calls := 0
	successors := func(value int) iter.Seq[int] {
		calls++
		return sequence([]int{value + 1, value + 2})
	}
	count := 0
	for range ForFunc(successors).BreadthFirst(0) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 || calls != 1 {
		t.Fatalf("early stop count/calls=%d/%d", count, calls)
	}
	if got := slices.Collect(ForGraph(graph).BreadthFirst("isolated")); !slices.Equal(got, []string{"isolated"}) {
		t.Fatal(got)
	}
}

func TestReachableClosureTransposeAndSubgraph(t *testing.T) {
	graph := traversalFixture(t)
	reachable, err := ReachableNodes(graph, "c")
	if err != nil || !slices.Equal(reachable, []string{"c", "d", "e", "b"}) {
		t.Fatalf("reachable=%v err=%v", reachable, err)
	}
	always := graph.TransitiveClosure(AddSelfLoopsAlways)
	if always.EdgeCount() != 19 { // 13 reachable non-self pairs + 6 self-loops
		t.Fatalf("closure edges=%d", always.EdgeCount())
	}
	if !always.HasEdge("isolated", "isolated") || !always.HasEdge("a", "e") {
		t.Fatal("closure missing reachability")
	}
	cycles := graph.TransitiveClosure(AddSelfLoopsForCycles)
	if cycles.HasEdge("isolated", "isolated") || cycles.HasEdge("a", "a") || !cycles.HasEdge("b", "b") {
		t.Fatal("cycle self-loop strategy mismatch")
	}
	transpose := graph.Transpose()
	if !transpose.HasEdge("b", "a") || transpose.HasEdge("a", "b") || !transpose.ContainsNode("isolated") {
		t.Fatal("transpose mismatch")
	}
	subgraph, err := graph.InducedSubgraph("a", "b", "d", "isolated")
	if err != nil || subgraph.EdgeCount() != 2 || subgraph.NodeCount() != 4 || !subgraph.ContainsNode("isolated") {
		t.Fatalf("subgraph edges/nodes=%d/%d err=%v", subgraph.EdgeCount(), subgraph.NodeCount(), err)
	}
}

func TestImmutableSnapshotsAndConcurrentReads(t *testing.T) {
	graph := traversalFixture(t)
	immutable := ImmutableGraphOf(graph)
	_ = graph.RemoveNode("a")
	if !immutable.HasEdge("a", "b") || immutable.NodeCount() != 6 {
		t.Fatal("immutable snapshot changed")
	}
	var wait sync.WaitGroup
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 100 {
				if !immutable.HasEdge("a", "b") || len(slices.Collect(immutable.Traverser().BreadthFirst("a"))) != 5 {
					t.Error("concurrent immutable read failed")
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestValueAndNetworkTransforms(t *testing.T) {
	values := NewValueGraph[string, int](true, false)
	_ = values.AddNode("a")
	_ = values.AddNode("b")
	_ = values.AddNode("c")
	_ = values.AddEdge("a", "b", 4)
	_ = values.AddEdge("b", "c", 5)
	transpose := values.Transpose()
	if value, found := transpose.EdgeValue("b", "a"); !found || value != 4 {
		t.Fatal(value, found)
	}
	immutable := ImmutableValueGraphOf(values)
	_ = values.RemoveEdge("a", "b")
	if value, found := immutable.EdgeValue("a", "b"); !found || value != 4 {
		t.Fatal("immutable value graph changed")
	}
	network := NewNetwork[string, string](true, false, true)
	_ = network.AddNode("a")
	_ = network.AddNode("b")
	_ = network.AddEdge("a", "b", "edge")
	networkImmutable := ImmutableNetworkOf(network)
	_ = network.RemoveNode("a")
	if pair, found := networkImmutable.Endpoints("edge"); !found || pair.Source != "a" {
		t.Fatal("immutable network changed")
	}
}

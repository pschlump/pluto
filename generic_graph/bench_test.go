package generic_graph

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/internal/guava_compare"
)

var benchmarkGraphBool bool
var benchmarkGraphInt int

func graphBenchCorpus(b *testing.B) guava_compare.GraphCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadGraphCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "graph.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

func buildGraphBench(corpus guava_compare.GraphCorpus) *Graph[int] {
	graph := NewGraph[int](true, false)
	for node := range corpus.Nodes {
		_ = graph.AddNode(node)
	}
	for _, edge := range corpus.Edges {
		_ = graph.AddEdge(edge.Source, edge.Target)
	}
	return graph
}

func buildValueGraphBench(corpus guava_compare.GraphCorpus) *ValueGraph[int, float64] {
	graph := NewValueGraph[int, float64](true, false)
	for node := range corpus.Nodes {
		_ = graph.AddNode(node)
	}
	for i, edge := range corpus.Edges {
		_ = graph.AddEdge(edge.Source, edge.Target, float64(i%97+1))
	}
	return graph
}

func BenchmarkGenericGraphInsert(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		_ = buildGraphBench(corpus)
	}
}

func BenchmarkGenericGraphInsertDense(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		graph := NewGraph[int](true, false)
		for node := range 512 {
			_ = graph.AddNode(node)
		}
		for source := range 512 {
			for target := range 512 {
				if source != target {
					_ = graph.AddEdge(source, target)
				}
			}
		}
	}
}

func BenchmarkGenericGraphLookup(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildGraphBench(corpus)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		edge := corpus.Lookups[i%len(corpus.Lookups)]
		benchmarkGraphBool = graph.HasEdge(edge.Source, edge.Target)
	}
}

func BenchmarkGenericGraphRemoval(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildGraphBench(corpus)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		copyGraph := graph.Copy()
		edge := corpus.Edges[i%len(corpus.Edges)]
		_ = copyGraph.RemoveEdge(edge.Source, edge.Target)
	}
}

func BenchmarkGenericGraphAdjacency(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildGraphBench(corpus)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		benchmarkGraphInt = len(graph.SuccessorsSlice(i % corpus.Nodes))
	}
}

func BenchmarkGenericGraphTraversal(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildGraphBench(corpus)
	start := corpus.Edges[0].Source
	b.ReportAllocs()
	for b.Loop() {
		// Count lazily, as the JMH benchmark does, rather than collecting.
		count := 0
		for range ForGraph(graph).BreadthFirst(start) {
			count++
		}
		benchmarkGraphInt = count
	}
}

func BenchmarkGenericValueGraphInsert(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		_ = buildValueGraphBench(corpus)
	}
}

func BenchmarkGenericValueGraphLookup(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildValueGraphBench(corpus)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		edge := corpus.Lookups[i%len(corpus.Lookups)]
		_, benchmarkGraphBool = graph.EdgeValue(edge.Source, edge.Target)
	}
}

func BenchmarkGenericNetworkInsert(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		network := NewNetwork[int, int](true, false, false)
		for node := range corpus.Nodes {
			_ = network.AddNode(node)
		}
		for id, edge := range corpus.Edges {
			_ = network.AddEdge(edge.Source, edge.Target, id)
		}
	}
}

// The Expected variants pass the same capacity hints the JMH builders use
// (expectedNodeCount, and expectedEdgeCount for the network). The natural
// constructors have no capacity parameter, so these reach the natural
// configuration internally; see docs/packages/generic_graph.md.
func BenchmarkGenericGraphInsertExpected(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		config := naturalConfig[int](true, false)
		config.ExpectedNodes = corpus.Nodes
		graph := NewGraphFunc(config)
		for node := range corpus.Nodes {
			_ = graph.AddNode(node)
		}
		for _, edge := range corpus.Edges {
			_ = graph.AddEdge(edge.Source, edge.Target)
		}
	}
}

func BenchmarkGenericGraphInsertDenseExpected(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		config := naturalConfig[int](true, false)
		config.ExpectedNodes = 512
		graph := NewGraphFunc(config)
		for node := range 512 {
			_ = graph.AddNode(node)
		}
		for source := range 512 {
			for target := range 512 {
				if source != target {
					_ = graph.AddEdge(source, target)
				}
			}
		}
	}
}

func BenchmarkGenericValueGraphInsertExpected(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		config := naturalConfig[int](true, false)
		config.ExpectedNodes = corpus.Nodes
		graph := NewValueGraphFunc[int, float64](config)
		for node := range corpus.Nodes {
			_ = graph.AddNode(node)
		}
		for i, edge := range corpus.Edges {
			_ = graph.AddEdge(edge.Source, edge.Target, float64(i%97+1))
		}
	}
}

func BenchmarkGenericNetworkInsertExpected(b *testing.B) {
	corpus := graphBenchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		config := naturalNetworkConfig[int, int](true, false, false)
		config.ExpectedNodes = corpus.Nodes
		config.ExpectedEdges = len(corpus.Edges)
		network := NewNetworkFunc(config)
		for node := range corpus.Nodes {
			_ = network.AddNode(node)
		}
		for id, edge := range corpus.Edges {
			_ = network.AddEdge(edge.Source, edge.Target, id)
		}
	}
}

func BenchmarkGenericGraphDenseAdapter(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildGraphBench(corpus)
	b.ReportAllocs()
	for b.Loop() {
		_ = graph.Dense()
	}
}

func BenchmarkGenericGraphDijkstraAdapter(b *testing.B) {
	corpus := graphBenchCorpus(b)
	graph := buildValueGraphBench(corpus)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Dijkstra(graph, corpus.Edges[0].Source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenericGraphDijkstraDirect(b *testing.B) {
	corpus := graphBenchCorpus(b)
	weighted := dijkstra.NewEdgeWeightedDigraph(corpus.Nodes)
	for i, edge := range corpus.Edges {
		weighted.AddEdge(dijkstra.DirectedEdge{From: edge.Source, To: edge.Target, Weight: float64(i%97 + 1)})
	}
	start := corpus.Edges[0].Source
	b.ReportAllocs()
	for b.Loop() {
		_ = dijkstra.NewDijkstraSP(weighted, start)
	}
}

func BenchmarkGenericGraphDenseMemory(b *testing.B) {
	for b.Loop() {
		runtime.GC()
		var base runtime.MemStats
		runtime.ReadMemStats(&base)
		graph := NewGraph[int](true, false)
		for node := range 512 {
			_ = graph.AddNode(node)
		}
		for source := range 512 {
			for target := range 512 {
				if source != target {
					_ = graph.AddEdge(source, target)
				}
			}
		}
		runtime.GC()
		var populated runtime.MemStats
		runtime.ReadMemStats(&populated)
		overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
		b.ReportMetric(float64(overhead), "retained-bytes")
		b.ReportMetric(float64(overhead)/float64(512+512*511), "retained-bytes/input")
		runtime.KeepAlive(graph)
	}
}

func BenchmarkGenericGraphMemory(b *testing.B) {
	corpus := graphBenchCorpus(b)
	for b.Loop() {
		runtime.GC()
		var base runtime.MemStats
		runtime.ReadMemStats(&base)
		graph := buildGraphBench(corpus)
		runtime.GC()
		var populated runtime.MemStats
		runtime.ReadMemStats(&populated)
		overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
		b.ReportMetric(float64(overhead), "retained-bytes")
		b.ReportMetric(float64(overhead)/float64(corpus.Nodes+len(corpus.Edges)), "retained-bytes/input")
		runtime.KeepAlive(graph)
	}
}

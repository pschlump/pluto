package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.graph.GraphBuilder;
import com.google.common.graph.Graphs;
import com.google.common.graph.MutableGraph;
import com.google.common.graph.MutableNetwork;
import com.google.common.graph.MutableValueGraph;
import com.google.common.graph.NetworkBuilder;
import com.google.common.graph.Traverser;
import com.google.common.graph.ValueGraphBuilder;
import java.io.File;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.BenchmarkMode;
import org.openjdk.jmh.annotations.Fork;
import org.openjdk.jmh.annotations.Level;
import org.openjdk.jmh.annotations.Measurement;
import org.openjdk.jmh.annotations.Mode;
import org.openjdk.jmh.annotations.OutputTimeUnit;
import org.openjdk.jmh.annotations.Scope;
import org.openjdk.jmh.annotations.Setup;
import org.openjdk.jmh.annotations.State;
import org.openjdk.jmh.annotations.Warmup;

@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@State(Scope.Benchmark)
public class GraphBenchmark {
  public static class Edge { public int source; public int target; }
  public static class Corpus {
    public int version;
    public long seed;
    public String inputDistribution;
    public int nodes;
    public List<Edge> edges;
    public List<Edge> lookups;
  }
  private Corpus corpus;
  private MutableGraph<Integer> graph;
  private MutableValueGraph<Integer, Double> values;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty("graphCorpus", "internal/guava_compare/testdata/graph.corpus.json");
    corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    graph = buildGraph();
    values = buildValues();
  }

  @Setup(Level.Invocation)
  public void invocation() { index++; }

  private Edge lookup() { return corpus.lookups.get(index % corpus.lookups.size()); }

  private MutableGraph<Integer> buildGraph() {
    MutableGraph<Integer> result = GraphBuilder.directed().expectedNodeCount(corpus.nodes).build();
    for (int i = 0; i < corpus.nodes; i++) result.addNode(i);
    for (Edge edge : corpus.edges) result.putEdge(edge.source, edge.target);
    return result;
  }

  private MutableValueGraph<Integer, Double> buildValues() {
    MutableValueGraph<Integer, Double> result = ValueGraphBuilder.directed().expectedNodeCount(corpus.nodes).build();
    for (int i = 0; i < corpus.nodes; i++) result.addNode(i);
    int i = 0;
    for (Edge edge : corpus.edges) result.putEdgeValue(edge.source, edge.target, (double) (i++ % 97 + 1));
    return result;
  }

  @Benchmark public MutableGraph<Integer> graphInsert() { return buildGraph(); }
  @Benchmark public boolean graphLookup() { Edge edge = lookup(); return graph.hasEdgeConnecting(edge.source, edge.target); }
  @Benchmark public boolean graphRemoval() {
    MutableGraph<Integer> copy = Graphs.copyOf(graph);
    Edge edge = corpus.edges.get(index % corpus.edges.size());
    return copy.removeEdge(edge.source, edge.target);
  }
  @Benchmark public int graphAdjacency() {
    int count = 0;
    for (Integer ignored : graph.successors(index % corpus.nodes)) count++;
    return count;
  }
  @Benchmark public int graphTraversal() {
    int count = 0;
    for (Integer ignored : Traverser.forGraph(graph).breadthFirst(corpus.edges.get(0).source)) count++;
    return count;
  }
  @Benchmark public MutableValueGraph<Integer, Double> valueGraphInsert() { return buildValues(); }
  @Benchmark public double valueGraphLookup() {
    Edge edge = lookup();
    return values.edgeValueOrDefault(edge.source, edge.target, -1.0);
  }
  @Benchmark public MutableNetwork<Integer, Integer> networkInsert() {
    MutableNetwork<Integer, Integer> result = NetworkBuilder.directed().expectedNodeCount(corpus.nodes).expectedEdgeCount(corpus.edges.size()).build();
    for (int i = 0; i < corpus.nodes; i++) result.addNode(i);
    int id = 0;
    for (Edge edge : corpus.edges) result.addEdge(edge.source, edge.target, id++);
    return result;
  }
  @Benchmark public MutableGraph<Integer> graphInsertDense() {
    MutableGraph<Integer> result = GraphBuilder.directed().expectedNodeCount(512).build();
    for (int i = 0; i < 512; i++) result.addNode(i);
    for (int i = 0; i < 512; i++) for (int j = 0; j < 512; j++) if (i != j) result.putEdge(i, j);
    return result;
  }
}

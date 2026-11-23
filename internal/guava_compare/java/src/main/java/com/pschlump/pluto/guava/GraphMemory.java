package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.graph.GraphBuilder;
import com.google.common.graph.MutableGraph;
import java.io.File;
import java.util.LinkedHashMap;
import java.util.Map;
import org.openjdk.jol.info.GraphLayout;

public final class GraphMemory {
  public static void main(String[] args) throws Exception {
    if (args.length != 2) throw new IllegalArgumentException("usage: GraphMemory corpus.json output.json");
    GraphBenchmark.Corpus corpus = new ObjectMapper().readValue(new File(args[0]), GraphBenchmark.Corpus.class);
    MutableGraph<Integer> sparse = GraphBuilder.directed().expectedNodeCount(corpus.nodes).build();
    for (int i = 0; i < corpus.nodes; i++) sparse.addNode(i);
    for (GraphBenchmark.Edge edge : corpus.edges) sparse.putEdge(edge.source, edge.target);
    MutableGraph<Integer> dense = GraphBuilder.directed().expectedNodeCount(512).build();
    for (int i = 0; i < 512; i++) dense.addNode(i);
    for (int i = 0; i < 512; i++) for (int j = 0; j < 512; j++) if (i != j) dense.putEdge(i, j);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("sparseBytes", GraphLayout.parseInstance(sparse).totalSize());
    result.put("sparseNodes", corpus.nodes);
    result.put("sparseEdges", corpus.edges.size());
    result.put("denseBytes", GraphLayout.parseInstance(dense).totalSize());
    result.put("denseNodes", 512);
    result.put("denseEdges", 512 * 511);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT).writeValue(new File(args[1]), result);
  }
}

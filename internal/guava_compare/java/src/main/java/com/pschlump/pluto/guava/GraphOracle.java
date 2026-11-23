package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.collect.ImmutableList;
import com.google.common.graph.EndpointPair;
import com.google.common.graph.Graph;
import com.google.common.graph.GraphBuilder;
import com.google.common.graph.Graphs;
import com.google.common.graph.MutableGraph;
import com.google.common.graph.MutableNetwork;
import com.google.common.graph.MutableValueGraph;
import com.google.common.graph.NetworkBuilder;
import com.google.common.graph.Traverser;
import com.google.common.graph.ValueGraphBuilder;
import java.io.File;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class GraphOracle {
  private static List<String> pairs(Iterable<EndpointPair<String>> pairs) {
    List<String> result = new ArrayList<>();
    for (EndpointPair<String> pair : pairs) result.add(pair.source() + "->" + pair.target());
    return result;
  }

  private static List<String> nodes(Iterable<String> values) {
    List<String> result = new ArrayList<>();
    for (String value : values) result.add(value);
    return result;
  }

  public static void main(String[] args) throws Exception {
    MutableGraph<String> graph = GraphBuilder.directed().allowsSelfLoops(true).build();
    for (String node : new String[] {"a", "b", "c", "d", "e", "isolated"}) graph.addNode(node);
    String[][] edges = {{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"d", "e"}, {"e", "b"}};
    for (String[] edge : edges) graph.putEdge(edge[0], edge[1]);
    Traverser<String> traverser = Traverser.forGraph(graph);
    Map<String, Object> graphResult = new LinkedHashMap<>();
    graphResult.put("nodes", nodes(graph.nodes()));
    graphResult.put("edges", pairs(graph.edges()));
    graphResult.put("successorsA", nodes(graph.successors("a")));
    graphResult.put("predecessorsB", nodes(graph.predecessors("b")));
    graphResult.put("breadthFirst", nodes(traverser.breadthFirst("a")));
    graphResult.put("preOrder", nodes(traverser.depthFirstPreOrder("a")));
    graphResult.put("postOrder", nodes(traverser.depthFirstPostOrder("a")));
    graphResult.put("reachableC", nodes(Graphs.reachableNodes(graph, "c")));
    graphResult.put("closureAlways", pairs(Graphs.transitiveClosure(graph, Graphs.TransitiveClosureSelfLoopStrategy.ADD_SELF_LOOPS_ALWAYS).edges()));
    graphResult.put("closureCycles", pairs(Graphs.transitiveClosure(graph, Graphs.TransitiveClosureSelfLoopStrategy.ADD_SELF_LOOPS_FOR_CYCLES).edges()));
    graphResult.put("transpose", pairs(Graphs.transpose(graph).edges()));
    graphResult.put("induced", pairs(Graphs.inducedSubgraph(graph, java.util.Arrays.asList("a", "b", "d", "isolated")).edges()));

    MutableValueGraph<String, Integer> values = ValueGraphBuilder.directed().build();
    values.addNode("isolated");
    values.putEdgeValue("a", "b", 4);
    values.putEdgeValue("b", "c", 5);
    Map<String, Object> valueResult = new LinkedHashMap<>();
    valueResult.put("nodes", nodes(values.nodes()));
    valueResult.put("edges", pairs(values.edges()));
    valueResult.put("ab", values.edgeValue("a", "b").get());
    valueResult.put("transposeBA", Graphs.transpose(values).edgeValue("b", "a").get());

    MutableNetwork<String, String> network = NetworkBuilder.directed().allowsSelfLoops(true).allowsParallelEdges(true).build();
    network.addEdge("a", "b", "ab1");
    network.addEdge("a", "b", "ab2");
    network.addEdge("b", "a", "ba");
    network.addEdge("a", "a", "aa");
    network.addNode("isolated");
    Map<String, Object> networkResult = new LinkedHashMap<>();
    networkResult.put("nodes", nodes(network.nodes()));
    networkResult.put("edges", nodes(network.edges()));
    networkResult.put("connectingAB", nodes(network.edgesConnecting("a", "b")));
    networkResult.put("degreeA", network.degree("a"));
    networkResult.put("incidentA", nodes(network.incidentEdges("a")));
    networkResult.put("ba", pairs(ImmutableList.of(network.incidentNodes("ba"))));

    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    root.put("graph", graphResult);
    root.put("valueGraph", valueResult);
    root.put("network", networkResult);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT).writeValue(new File(args[0]), root);
  }
}

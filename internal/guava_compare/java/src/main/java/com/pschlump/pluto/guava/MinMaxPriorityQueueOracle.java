/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.MinMaxPriorityQueue;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Executes the Phase 1e double-ended priority-queue trace against Guava. */
public final class MinMaxPriorityQueueOracle {
  private MinMaxPriorityQueueOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: MinMaxPriorityQueueOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    List<Object> variants = new ArrayList<>();
    for (int maximumSize : trace.maximumSizes) {
      variants.add(execute(trace, maximumSize));
    }
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    result.put("variants", variants);
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(result) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> execute(Trace trace, int maximumSize) {
    MinMaxPriorityQueue<Integer> queue = maximumSize < 0
        ? MinMaxPriorityQueue.create()
        : MinMaxPriorityQueue.maximumSize(maximumSize).create();
    List<Object> results = new ArrayList<>();
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "offer": queue.offer(operation.value); results.add(queue.size()); break;
        case "poll": results.add(queue.poll()); break;
        case "pollLast": results.add(queue.pollLast()); break;
        case "peek": results.add(queue.peek()); break;
        case "peekLast": results.add(queue.peekLast()); break;
        case "size": results.add(queue.size()); break;
        default: throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }
    List<Integer> elements = new ArrayList<>(queue);
    Collections.sort(elements);
    Map<String, Object> variant = new LinkedHashMap<>();
    variant.put("maximumSize", maximumSize);
    variant.put("results", results);
    variant.put("final", elements);
    return variant;
  }

  public static final class Trace {
    public int version;
    public List<Integer> maximumSizes;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public int value;
  }
}

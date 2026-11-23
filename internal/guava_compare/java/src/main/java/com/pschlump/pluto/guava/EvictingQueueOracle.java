/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.EvictingQueue;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Executes the Phase 1e evicting-queue trace against Guava. */
public final class EvictingQueueOracle {
  private EvictingQueueOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: EvictingQueueOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    List<Object> variants = new ArrayList<>();
    for (int capacity : trace.capacities) {
      variants.add(execute(trace, capacity));
    }
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    result.put("variants", variants);
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(result) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> execute(Trace trace, int capacity) {
    EvictingQueue<String> queue = EvictingQueue.create(capacity);
    List<Object> results = new ArrayList<>();
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "add": results.add(queue.offer(operation.value)); break;
        case "addAll": results.add(queue.addAll(operation.values)); break;
        case "poll": results.add(queue.poll()); break;
        case "peek": results.add(queue.peek()); break;
        case "remainingCapacity": results.add(queue.remainingCapacity()); break;
        case "size": results.add(queue.size()); break;
        default: throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }
    Map<String, Object> variant = new LinkedHashMap<>();
    variant.put("capacity", capacity);
    variant.put("results", results);
    variant.put("final", new ArrayList<>(queue));
    return variant;
  }

  public static final class Trace {
    public int version;
    public List<Integer> capacities;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String value;
    public List<String> values;
  }
}

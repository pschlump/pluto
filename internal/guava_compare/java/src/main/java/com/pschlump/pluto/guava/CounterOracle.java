/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.HashMultiset;
import com.google.common.collect.Multiset;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Executes the counter trace and writes the canonical Phase 0 result. */
public final class CounterOracle {
  private static final String COMMIT = "36e3995499d6c3b74b1207d362fa9e19915d2589";
  private static final ObjectMapper JSON = new ObjectMapper();

  private CounterOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: CounterOracle TRACE OUTPUT");
    }
    Trace trace = JSON.readValue(new File(arguments[0]), Trace.class);
    if (trace.version != 1 || !"counter".equals(trace.fixture)) {
      throw new IllegalArgumentException("unsupported trace");
    }

    Multiset<String> counts = HashMultiset.create();
    List<Integer> results = new ArrayList<>(trace.operations.size());
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "add":
          requirePositive(operation.count, "add");
          counts.add(operation.value, operation.count);
          results.add(counts.count(operation.value));
          break;
        case "remove":
          requirePositive(operation.count, "remove");
          counts.remove(operation.value, operation.count);
          results.add(counts.count(operation.value));
          break;
        case "setCount":
          if (operation.count < 0) {
            throw new IllegalArgumentException("setCount count must not be negative");
          }
          counts.setCount(operation.value, operation.count);
          results.add(counts.count(operation.value));
          break;
        case "count":
          results.add(counts.count(operation.value));
          break;
        case "size":
          results.add(counts.size());
          break;
        case "distinctSize":
          results.add(counts.elementSet().size());
          break;
        default:
          throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }

    List<Multiset.Entry<String>> sourceEntries = new ArrayList<>(counts.entrySet());
    sourceEntries.sort(Comparator.comparing(Multiset.Entry::getElement));
    List<Map<String, Object>> entries = new ArrayList<>(sourceEntries.size());
    for (Multiset.Entry<String> source : sourceEntries) {
      Map<String, Object> entry = new LinkedHashMap<>();
      entry.put("value", source.getElement());
      entry.put("count", source.getCount());
      entries.add(entry);
    }
    Map<String, Object> finalState = new LinkedHashMap<>();
    finalState.put("size", counts.size());
    finalState.put("distinctSize", counts.elementSet().size());
    finalState.put("entries", entries);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("fixture", "counter");
    result.put("guavaCommit", COMMIT);
    result.put("results", results);
    result.put("final", finalState);
    String encoded = JSON.writeValueAsString(result) + "\n";
    Files.write(new File(arguments[1]).toPath(), encoded.getBytes(StandardCharsets.UTF_8));
  }

  private static void requirePositive(int count, String operation) {
    if (count <= 0) {
      throw new IllegalArgumentException(operation + " count must be positive");
    }
  }

  public static final class Trace {
    public int version;
    public String fixture;
    public long seed;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String value;
    public int count;
  }
}

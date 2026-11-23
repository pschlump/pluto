/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.BoundType;
import com.google.common.collect.ContiguousSet;
import com.google.common.collect.DiscreteDomain;
import com.google.common.collect.Range;
import com.google.common.collect.TreeRangeMap;
import com.google.common.collect.TreeRangeSet;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Executes Phase 3 canonical ranges and range collections against Guava. */
public final class RangeCollectionOracle {
  private RangeCollectionOracle() {}

  public static void main(String[] arguments) throws Exception {
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    Map<String, Object> output = new LinkedHashMap<>();
    output.put("version", 1);
    output.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    List<Object> canonical = new ArrayList<>();
    for (Operation operation : trace.canonical) {
      canonical.add(format(range(operation).canonical(DiscreteDomain.integers())));
    }
    output.put("canonical", canonical);
    List<Object> contiguous = new ArrayList<>();
    for (Operation operation : trace.contiguous) {
      contiguous.add(new ArrayList<>(ContiguousSet.create(range(operation), DiscreteDomain.integers())));
    }
    output.put("contiguous", contiguous);
    output.put("set", set(trace.setOperations));
    output.put("map", map(trace.mapOperations));
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(output) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> set(List<Operation> operations) {
    TreeRangeSet<Integer> set = TreeRangeSet.create();
    List<Object> results = new ArrayList<>();
    for (Operation operation : operations) {
      switch (operation.op) {
        case "add": set.add(range(operation)); results.add(set.asRanges().size()); break;
        case "remove": set.remove(range(operation)); results.add(set.asRanges().size()); break;
        case "contains": results.add(set.contains(operation.point)); break;
        case "size": results.add(set.asRanges().size()); break;
        default: throw new IllegalArgumentException("unknown set operation " + operation.op);
      }
    }
    List<Object> ranges = new ArrayList<>();
    for (Range<Integer> range : set.asRanges()) ranges.add(format(range));
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("results", results);
    result.put("final", ranges);
    return result;
  }

  private static Map<String, Object> map(List<Operation> operations) {
    TreeRangeMap<Integer, String> map = TreeRangeMap.create();
    List<Object> results = new ArrayList<>();
    for (Operation operation : operations) {
      switch (operation.op) {
        case "put": map.put(range(operation), operation.value); results.add(map.asMapOfRanges().size()); break;
        case "remove": map.remove(range(operation)); results.add(map.asMapOfRanges().size()); break;
        case "get": results.add(map.get(operation.point)); break;
        case "size": results.add(map.asMapOfRanges().size()); break;
        default: throw new IllegalArgumentException("unknown map operation " + operation.op);
      }
    }
    List<Object> entries = new ArrayList<>();
    for (Map.Entry<Range<Integer>, String> entry : map.asMapOfRanges().entrySet()) {
      entries.add(format(entry.getKey()) + "=" + entry.getValue());
    }
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("results", results);
    result.put("final", entries);
    return result;
  }

  private static Range<Integer> range(Operation operation) {
    if (operation.lowerOpen && operation.upperOpen) return Range.open(operation.low, operation.high);
    if (operation.lowerOpen) return Range.openClosed(operation.low, operation.high);
    if (operation.upperOpen) return Range.closedOpen(operation.low, operation.high);
    return Range.closed(operation.low, operation.high);
  }

  private static String format(Range<Integer> range) {
    StringBuilder result = new StringBuilder();
    if (range.hasLowerBound()) {
      result.append(range.lowerBoundType() == BoundType.OPEN ? '(' : '[').append(range.lowerEndpoint());
    } else result.append("(-inf");
    result.append("..");
    if (range.hasUpperBound()) {
      result.append(range.upperEndpoint()).append(range.upperBoundType() == BoundType.OPEN ? ')' : ']');
    } else result.append("+inf)");
    return result.toString();
  }

  public static final class Trace {
    public int version;
    public List<Operation> canonical;
    public List<Operation> contiguous;
    public List<Operation> setOperations;
    public List<Operation> mapOperations;
  }

  public static final class Operation {
    public String op;
    public int low;
    public int high;
    public boolean lowerOpen;
    public boolean upperOpen;
    public int point;
    public String value;
  }
}

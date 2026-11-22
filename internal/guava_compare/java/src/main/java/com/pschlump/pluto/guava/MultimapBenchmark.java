/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ArrayListMultimap;
import com.google.common.collect.HashMultimap;
import com.google.common.collect.Multimap;
import com.google.common.collect.TreeMultimap;
import java.io.File;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
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
import org.openjdk.jmh.annotations.Threads;
import org.openjdk.jmh.annotations.Warmup;

/** Matched Phase 1b workload for multimap/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class MultimapBenchmark {
  private Multimap<String, String> arrayList;
  private Multimap<String, String> hash;
  private Multimap<String, String> tree;
  private Map<String, List<String>> listMap;
  private Map<String, Set<String>> setMap;
  private Operation[] operations;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "multimapCorpus", "internal/guava_compare/testdata/multimap.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    arrayList = ArrayListMultimap.create();
    hash = HashMultimap.create();
    tree = TreeMultimap.create();
    listMap = new HashMap<>(corpus.capacity);
    setMap = new HashMap<>(corpus.capacity);
    operations = corpus.operations.toArray(new Operation[0]);
  }

  @Benchmark public int arrayListMultimap() { return apply(arrayList, next()); }
  @Benchmark public int hashMultimap() { return apply(hash, next()); }
  @Benchmark public int treeMultimap() { return apply(tree, next()); }
  @Benchmark public int listMap() { return applyListMap(listMap, next()); }
  @Benchmark public int setMap() { return applySetMap(setMap, next()); }

  private static int apply(Multimap<String, String> multimap, Operation operation) {
    switch (operation.op) {
      case "put":
        return multimap.put(operation.key, operation.value) ? 1 : 0;
      case "putAll":
        int before = multimap.size();
        multimap.putAll(operation.key, operation.values);
        return multimap.size() - before;
      case "get":
        return multimap.get(operation.key).size();
      case "remove":
        return multimap.remove(operation.key, operation.value) ? 1 : 0;
      case "containsEntry":
        return multimap.containsEntry(operation.key, operation.value) ? 1 : 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private static int applyListMap(Map<String, List<String>> map, Operation operation) {
    List<String> values = map.get(operation.key);
    switch (operation.op) {
      case "put":
        map.computeIfAbsent(operation.key, unused -> new ArrayList<>()).add(operation.value);
        return 1;
      case "putAll":
        map.computeIfAbsent(operation.key, unused -> new ArrayList<>()).addAll(operation.values);
        return operation.values.size();
      case "get":
        return values == null ? 0 : values.size();
      case "remove":
        if (values != null && values.remove(operation.value)) {
          if (values.isEmpty()) {
            map.remove(operation.key);
          }
          return 1;
        }
        return 0;
      case "containsEntry":
        return values != null && values.contains(operation.value) ? 1 : 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private static int applySetMap(Map<String, Set<String>> map, Operation operation) {
    Set<String> values = map.get(operation.key);
    switch (operation.op) {
      case "put":
        return map.computeIfAbsent(operation.key, unused -> new HashSet<>()).add(operation.value) ? 1 : 0;
      case "putAll":
        int added = 0;
        Set<String> target = map.computeIfAbsent(operation.key, unused -> new HashSet<>());
        for (String value : operation.values) {
          if (target.add(value)) {
            added++;
          }
        }
        return added;
      case "get":
        return values == null ? 0 : values.size();
      case "remove":
        if (values != null && values.remove(operation.value)) {
          if (values.isEmpty()) {
            map.remove(operation.key);
          }
          return 1;
        }
        return 0;
      case "containsEntry":
        return values != null && values.contains(operation.value) ? 1 : 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private Operation next() {
    return operations[index++ % operations.length];
  }

  public static final class Corpus {
    public int version;
    public long seed;
    public int capacity;
    public int valuesPerKey;
    public String inputDistribution;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String key;
    public String value;
    public List<String> values;
  }
}

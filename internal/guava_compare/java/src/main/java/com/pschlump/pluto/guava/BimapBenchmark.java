/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.HashBiMap;
import java.io.File;
import java.util.HashMap;
import java.util.Map;
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

/** Matched Phase 1c workload for bimap/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class BimapBenchmark {
  private HashBiMap<String, String> bimap;
  private HashBiMap<String, String> iteration;
  private Map<String, String> forward;
  private Map<String, String> reverse;
  private MultimapBenchmark.Operation[] operations;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "bimapCorpus", "internal/guava_compare/testdata/bimap.corpus.json");
    MultimapBenchmark.Corpus corpus = new ObjectMapper().readValue(new File(path), MultimapBenchmark.Corpus.class);
    bimap = HashBiMap.create(corpus.capacity);
    forward = new HashMap<>(corpus.capacity);
    reverse = new HashMap<>(corpus.capacity);
    iteration = HashBiMap.create(corpus.capacity);
    for (int i = 0; i < corpus.capacity; i++) {
      iteration.forcePut(String.format("key-%04d", i), String.format("value-%04d", i));
    }
    operations = corpus.operations.toArray(new MultimapBenchmark.Operation[0]);
  }

  @Benchmark public int hashBiMap() { return apply(bimap, next()); }
  @Benchmark public int mapBiMap() { return applyMaps(forward, reverse, next()); }

  @Benchmark
  public int biMapInverseIteration() {
    int result = 0;
    for (Map.Entry<String, String> entry : iteration.inverse().entrySet()) {
      result += entry.getKey().length() + entry.getValue().length();
    }
    return result;
  }

  private static int apply(HashBiMap<String, String> map, MultimapBenchmark.Operation operation) {
    switch (operation.op) {
      case "put":
        try {
          map.put(operation.key, operation.value);
          return 1;
        } catch (IllegalArgumentException conflict) {
          return 0;
        }
      case "forcePut":
        map.forcePut(operation.key, operation.value);
        return 1;
      case "getValue":
        return map.containsKey(operation.key) ? 1 : 0;
      case "getKey":
        return map.inverse().containsKey(operation.value) ? 1 : 0;
      case "deleteKey":
        return map.remove(operation.key) != null ? 1 : 0;
      case "deleteValue":
        return map.inverse().remove(operation.value) != null ? 1 : 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private static int applyMaps(
      Map<String, String> forward, Map<String, String> reverse,
      MultimapBenchmark.Operation operation) {
    switch (operation.op) {
      case "put":
        String oldKey = reverse.get(operation.value);
        if (oldKey != null && !oldKey.equals(operation.key)) {
          return 0;
        }
        String oldValue = forward.put(operation.key, operation.value);
        if (oldValue != null && !oldValue.equals(operation.value)) {
          reverse.remove(oldValue);
        }
        reverse.put(operation.value, operation.key);
        return 1;
      case "forcePut":
        oldKey = reverse.get(operation.value);
        if (oldKey != null && !oldKey.equals(operation.key)) {
          forward.remove(oldKey);
        }
        oldValue = forward.get(operation.key);
        if (oldValue != null && !oldValue.equals(operation.value)) {
          reverse.remove(oldValue);
        }
        forward.put(operation.key, operation.value);
        reverse.put(operation.value, operation.key);
        return 1;
      case "getValue":
        return forward.containsKey(operation.key) ? 1 : 0;
      case "getKey":
        return reverse.containsKey(operation.value) ? 1 : 0;
      case "deleteKey":
        oldValue = forward.remove(operation.key);
        if (oldValue != null) {
          reverse.remove(oldValue);
          return 1;
        }
        return 0;
      case "deleteValue":
        oldKey = reverse.remove(operation.value);
        if (oldKey != null) {
          forward.remove(oldKey);
          return 1;
        }
        return 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private MultimapBenchmark.Operation next() {
    return operations[Math.floorMod(index++, operations.length)];
  }
}

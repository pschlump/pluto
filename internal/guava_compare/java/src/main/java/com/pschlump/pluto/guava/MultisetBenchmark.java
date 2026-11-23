/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.google.common.collect.HashMultiset;
import com.google.common.collect.Multiset;
import com.google.common.collect.TreeMultiset;
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
import com.fasterxml.jackson.databind.ObjectMapper;

/** Matched Phase 1a workload for multiset/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class MultisetBenchmark {
  private Multiset<String> hash;
  private Multiset<String> tree;
  private Map<String, Integer> map;
  private CounterOracle.Operation[] operations;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "corpus", "internal/guava_compare/testdata/counter.corpus.json");
    CounterBenchmark.Corpus corpus = new ObjectMapper().readValue(new File(path), CounterBenchmark.Corpus.class);
    hash = HashMultiset.create(corpus.capacity);
    tree = TreeMultiset.create();
    map = new HashMap<>(corpus.capacity);
    operations = corpus.operations.toArray(new CounterOracle.Operation[0]);
  }

  @Benchmark
  public int hashMultiset() {
    CounterOracle.Operation operation = next();
    switch (operation.op) {
      case "add":
        hash.add(operation.value, operation.count);
        break;
      case "remove":
        hash.remove(operation.value, operation.count);
        break;
      case "setCount":
        hash.setCount(operation.value, operation.count);
        break;
      default:
        break;
    }
    return hash.count(operation.value);
  }

  @Benchmark
  public int treeMultiset() {
    CounterOracle.Operation operation = next();
    switch (operation.op) {
      case "add":
        tree.add(operation.value, operation.count);
        break;
      case "remove":
        tree.remove(operation.value, operation.count);
        break;
      case "setCount":
        tree.setCount(operation.value, operation.count);
        break;
      default:
        break;
    }
    return tree.count(operation.value);
  }

  @Benchmark
  public int mapMultiset() {
    CounterOracle.Operation operation = next();
    int old = map.getOrDefault(operation.value, 0);
    switch (operation.op) {
      case "add":
        map.put(operation.value, old + operation.count);
        break;
      case "remove":
        if (old <= operation.count) {
          map.remove(operation.value);
        } else {
          map.put(operation.value, old - operation.count);
        }
        break;
      case "setCount":
        if (operation.count == 0) {
          map.remove(operation.value);
        } else {
          map.put(operation.value, operation.count);
        }
        break;
      default:
        break;
    }
    return map.getOrDefault(operation.value, 0);
  }

  private CounterOracle.Operation next() {
    return operations[Math.floorMod(index++, operations.length)];
  }
}

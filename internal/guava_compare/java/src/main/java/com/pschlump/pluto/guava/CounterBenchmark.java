/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.HashMultiset;
import com.google.common.collect.Multiset;
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
import org.openjdk.jmh.annotations.Threads;
import org.openjdk.jmh.annotations.Warmup;

/** Matched workload for internal/guava_compare/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class CounterBenchmark {
  private Multiset<String> counts;
  private CounterOracle.Operation[] operations;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "corpus", "internal/guava_compare/testdata/counter.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    if (corpus.version != 1 || !"counter".equals(corpus.fixture)) {
      throw new IllegalArgumentException("unsupported corpus");
    }
    counts = HashMultiset.create(corpus.capacity);
    operations = corpus.operations.toArray(new CounterOracle.Operation[0]);
  }

  @Benchmark
  public int counter() {
    CounterOracle.Operation operation = operations[Math.floorMod(index++, operations.length)];
    switch (operation.op) {
      case "add":
        counts.add(operation.value, operation.count);
        return counts.count(operation.value);
      case "remove":
        counts.remove(operation.value, operation.count);
        return counts.count(operation.value);
      case "setCount":
        counts.setCount(operation.value, operation.count);
        return counts.count(operation.value);
      case "count":
        return counts.count(operation.value);
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  public static final class Corpus {
    public int version;
    public String fixture;
    public long seed;
    public int capacity;
    public String inputDistribution;
    public List<CounterOracle.Operation> operations;
  }
}

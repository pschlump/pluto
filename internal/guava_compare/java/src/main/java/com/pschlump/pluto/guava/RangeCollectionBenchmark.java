/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.Range;
import com.google.common.collect.TreeRangeMap;
import com.google.common.collect.TreeRangeSet;
import java.io.File;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;
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

/** Matched Phase 3 range and positioned sorted-set workloads. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class RangeCollectionBenchmark {
  private TreeRangeSet<Integer> rangeSet;
  private TreeRangeMap<Integer, String> rangeMap;
  private TreeRangeSet<Integer> rangeSetIteration;
  private TreeRangeMap<Integer, String> rangeMapIteration;
  private TreeSet<Integer> treeSet;
  private Operation[] operations;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty("rangeCorpus", "internal/guava_compare/testdata/range.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    rangeSet = TreeRangeSet.create();
    rangeMap = TreeRangeMap.create();
    treeSet = new TreeSet<>();
    for (int i = 0; i < 4096; i++) treeSet.add(i);
    rangeSetIteration = TreeRangeSet.create();
    rangeMapIteration = TreeRangeMap.create();
    for (int i = 0; i < 1024; i += 2) {
      Range<Integer> range = Range.closedOpen(i, i + 1);
      rangeSetIteration.add(range);
      rangeMapIteration.put(range, "value");
    }
    operations = corpus.operations.toArray(new Operation[0]);
  }

  @Benchmark public int rangeSet() {
    Operation operation = next();
    switch (operation.op) {
      case "add": rangeSet.add(range(operation)); return 1;
      case "remove": rangeSet.remove(range(operation)); return 1;
      default: return rangeSet.contains(operation.point) ? 1 : 0;
    }
  }

  @Benchmark public int rangeMap() {
    Operation operation = next();
    switch (operation.op) {
      case "add": rangeMap.put(range(operation), operation.value); return 1;
      case "remove": rangeMap.remove(range(operation)); return 1;
      default: return rangeMap.get(operation.point) != null ? 1 : 0;
    }
  }

  @Benchmark public int rangeSetIteration() {
    int result = 0;
    for (Range<Integer> range : rangeSetIteration.asRanges()) result++;
    return result;
  }

  @Benchmark public int rangeMapIteration() {
    int result = 0;
    for (Map.Entry<Range<Integer>, String> entry : rangeMapIteration.asMapOfRanges().entrySet()) result++;
    return result;
  }

  @Benchmark public int treeSetRangePositioned() {
    int result = 0;
    for (int value : treeSet.subSet(1024, true, 1035, true)) result += value;
    return result;
  }

  private Range<Integer> range(Operation operation) { return Range.closedOpen(operation.low, operation.high); }
  private Operation next() { return operations[Math.floorMod(index++, operations.length)]; }

  public static final class Corpus {
    public int version;
    public long seed;
    public int capacity;
    public String inputDistribution;
    public List<Operation> operations;
  }
  public static final class Operation {
    public String op;
    public int low;
    public int high;
    public int point;
    public String value;
  }
}

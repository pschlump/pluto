/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.Interner;
import com.google.common.collect.Interners;
import com.google.common.collect.LinkedHashMultimap;
import com.google.common.collect.LinkedHashMultiset;
import com.google.common.collect.LinkedListMultimap;
import com.google.common.collect.Multimap;
import com.google.common.collect.Multiset;
import java.io.File;
import java.util.LinkedHashMap;
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

/** Matched Phase 9 linked-collection and strong-interning workloads. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class Phase9Benchmark {
  private Multiset<String> linkedMultiset;
  private Multimap<String, String> linkedList;
  private Multimap<String, String> linkedHash;
  private Map<String, Integer> linkedMap;
  private Interner<String> interner;
  private CounterOracle.Operation[] multisetOperations;
  private MultimapBenchmark.Operation[] multimapOperations;
  private String[] values;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    ObjectMapper mapper = new ObjectMapper();
    CounterBenchmark.Corpus multisetCorpus = mapper.readValue(
        new File(System.getProperty("corpus", "internal/guava_compare/testdata/counter.corpus.json")),
        CounterBenchmark.Corpus.class);
    MultimapBenchmark.Corpus multimapCorpus = mapper.readValue(
        new File(System.getProperty("multimapCorpus", "internal/guava_compare/testdata/multimap.corpus.json")),
        MultimapBenchmark.Corpus.class);
    linkedMultiset = LinkedHashMultiset.create(multisetCorpus.capacity);
    linkedList = LinkedListMultimap.create();
    linkedHash = LinkedHashMultimap.create();
    linkedMap = new LinkedHashMap<>(10000);
    multisetOperations = multisetCorpus.operations.toArray(new CounterOracle.Operation[0]);
    multimapOperations = multimapCorpus.operations.toArray(new MultimapBenchmark.Operation[0]);
    values = new String[10000];
    interner = Interners.newStrongInterner();
    for (int i = 0; i < values.length; i++) {
      values[i] = String.format("value-%05d", i);
      if (i < values.length / 2) interner.intern(values[i]);
    }
  }

  @Benchmark
  public int linkedHashMultiset() {
    CounterOracle.Operation operation = multisetOperations[Math.floorMod(index++, multisetOperations.length)];
    switch (operation.op) {
      case "add": linkedMultiset.add(operation.value, operation.count); break;
      case "remove": linkedMultiset.remove(operation.value, operation.count); break;
      case "setCount": linkedMultiset.setCount(operation.value, operation.count); break;
      default: break;
    }
    return linkedMultiset.count(operation.value);
  }

  @Benchmark public int linkedListMultimap() { return apply(linkedList, nextMultimap()); }
  @Benchmark public int linkedHashMultimap() { return apply(linkedHash, nextMultimap()); }

  private MultimapBenchmark.Operation nextMultimap() {
    return multimapOperations[Math.floorMod(index++, multimapOperations.length)];
  }

  private static int apply(Multimap<String, String> multimap, MultimapBenchmark.Operation operation) {
    switch (operation.op) {
      case "put": return multimap.put(operation.key, operation.value) ? 1 : 0;
      case "putAll":
        int before = multimap.size();
        multimap.putAll(operation.key, operation.values);
        return multimap.size() - before;
      case "get": return multimap.get(operation.key).size();
      case "remove": return multimap.remove(operation.key, operation.value) ? 1 : 0;
      case "containsEntry": return multimap.containsEntry(operation.key, operation.value) ? 1 : 0;
      default: throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  @Benchmark
  public int linkedHashMap() {
    int slot = Math.floorMod(index++, values.length);
    String value = values[slot];
    linkedMap.put(value, slot);
    return linkedMap.get(value);
  }

  @Benchmark
  public String strongInterner() {
    return interner.intern(new String(values[Math.floorMod(index++, values.length)]));
  }
}

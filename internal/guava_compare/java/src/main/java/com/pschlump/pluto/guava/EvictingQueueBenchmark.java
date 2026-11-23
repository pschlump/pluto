/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.EvictingQueue;
import java.io.File;
import java.util.ArrayDeque;
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

/** Matched Phase 1e workload for evicting_queue/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class EvictingQueueBenchmark {
  private EvictingQueue<Integer> evicting;
  private ArrayDeque<Integer> deque;
  private EvictingQueue<Integer> evictingIteration;
  private ArrayDeque<Integer> dequeIteration;
  private Operation[] operations;
  private int capacity;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "evictingQueueCorpus", "internal/guava_compare/testdata/evicting_queue.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    capacity = corpus.capacity;
    evicting = EvictingQueue.create(capacity);
    deque = new ArrayDeque<>(capacity);
    evictingIteration = EvictingQueue.create(capacity);
    dequeIteration = new ArrayDeque<>(capacity);
    for (int i = 0; i < capacity; i++) {
      evictingIteration.offer(i);
      dequeIteration.addLast(i);
    }
    operations = corpus.operations.toArray(new Operation[0]);
  }

  @Benchmark public int evictingQueue() { return applyEvicting(next()); }
  @Benchmark public int dQueueEviction() { return applyDeque(next()); }

  @Benchmark public int evictingQueueIteration() {
    int result = 0;
    for (int value : evictingIteration) result += value;
    return result;
  }

  @Benchmark public int dQueueIteration() {
    int result = 0;
    for (int value : dequeIteration) result += value;
    return result;
  }

  private int applyEvicting(Operation operation) {
    switch (operation.op) {
      case "push": evicting.offer(operation.value); return 1;
      case "pushAll": return evicting.addAll(operation.values) ? 1 : 0;
      case "pop": return evicting.poll() != null ? 1 : 0;
      case "peek": return evicting.peek() != null ? 1 : 0;
      default: throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private int applyDeque(Operation operation) {
    switch (operation.op) {
      case "push": offerDeque(operation.value); return 1;
      case "pushAll":
        for (int value : operation.values) offerDeque(value);
        return 1;
      case "pop": return deque.pollFirst() != null ? 1 : 0;
      case "peek": return deque.peekFirst() != null ? 1 : 0;
      default: throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private void offerDeque(int value) {
    if (deque.size() == capacity) deque.removeFirst();
    deque.addLast(value);
  }

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
    public int value;
    public List<Integer> values;
  }
}

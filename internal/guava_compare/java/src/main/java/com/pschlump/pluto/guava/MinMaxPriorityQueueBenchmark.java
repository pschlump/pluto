/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.MinMaxPriorityQueue;
import java.io.File;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.PriorityQueue;
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

/** Matched Phase 1e workloads for min_max_pq/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class MinMaxPriorityQueueBenchmark {
  private MinMaxPriorityQueue<Integer> minMax;
  private MinMaxPriorityQueue<Integer> bounded;
  private MinMaxPriorityQueue<Integer> oneEndMinMax;
  private PriorityQueue<Integer> priority;
  private TwoHeap twoHeap;
  private Operation[] operations;
  private Operation[] oneEnded;
  private int index;
  private int oneEndIndex;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "minMaxCorpus", "internal/guava_compare/testdata/min_max_pq.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    minMax = MinMaxPriorityQueue.create();
    bounded = MinMaxPriorityQueue.maximumSize(corpus.maximumSize).create();
    oneEndMinMax = MinMaxPriorityQueue.create();
    priority = new PriorityQueue<>();
    twoHeap = new TwoHeap();
    operations = corpus.operations.toArray(new Operation[0]);
    oneEnded = corpus.oneEnded.toArray(new Operation[0]);
  }

  @Benchmark public int minMaxPriorityQueue() { return applyMinMax(minMax, next()); }
  @Benchmark public int boundedMinMaxPriorityQueue() { return applyMinMax(bounded, next()); }
  @Benchmark public int twoHeap() { return applyTwoHeap(twoHeap, next()); }
  @Benchmark public int minMaxOneEnd() { return applyMinMax(oneEndMinMax, nextOneEnd()); }
  @Benchmark public int priorityQueue() { return applyPriority(priority, nextOneEnd()); }

  private static int applyMinMax(MinMaxPriorityQueue<Integer> queue, Operation operation) {
    switch (operation.op) {
      case "push": queue.offer(operation.value); return 1;
      case "popMin": return queue.poll() != null ? 1 : 0;
      case "popMax": return queue.pollLast() != null ? 1 : 0;
      case "peekMin": return queue.peek() != null ? 1 : 0;
      case "peekMax": return queue.peekLast() != null ? 1 : 0;
      default: throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private static int applyPriority(PriorityQueue<Integer> queue, Operation operation) {
    switch (operation.op) {
      case "push": queue.offer(operation.value); return 1;
      case "popMin": return queue.poll() != null ? 1 : 0;
      case "peekMin": return queue.peek() != null ? 1 : 0;
      default: throw new IllegalArgumentException("unknown one-ended operation " + operation.op);
    }
  }

  private static int applyTwoHeap(TwoHeap queue, Operation operation) {
    switch (operation.op) {
      case "push": queue.offer(operation.value); return 1;
      case "popMin": return queue.poll(false) != null ? 1 : 0;
      case "popMax": return queue.poll(true) != null ? 1 : 0;
      case "peekMin": return queue.peek(false) != null ? 1 : 0;
      case "peekMax": return queue.peek(true) != null ? 1 : 0;
      default: throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private Operation next() { return operations[index++ % operations.length]; }
  private Operation nextOneEnd() { return oneEnded[oneEndIndex++ % oneEnded.length]; }

  private static final class Entry {
    final int value;
    final int id;
    Entry(int value, int id) { this.value = value; this.id = id; }
  }

  private static final class TwoHeap {
    private static final Comparator<Entry> MIN =
        Comparator.comparingInt((Entry entry) -> entry.value).thenComparingInt(entry -> entry.id);
    private final PriorityQueue<Entry> minimum = new PriorityQueue<>(MIN);
    private final PriorityQueue<Entry> maximum = new PriorityQueue<>(MIN.reversed());
    private final Set<Integer> discarded = new HashSet<>();
    private int nextID;

    void offer(int value) {
      Entry entry = new Entry(value, nextID++);
      minimum.offer(entry);
      maximum.offer(entry);
    }

    Integer poll(boolean max) {
      clean(max);
      PriorityQueue<Entry> heap = max ? maximum : minimum;
      Entry entry = heap.poll();
      if (entry == null) return null;
      discarded.add(entry.id);
      return entry.value;
    }

    Integer peek(boolean max) {
      clean(max);
      PriorityQueue<Entry> heap = max ? maximum : minimum;
      Entry entry = heap.peek();
      return entry == null ? null : entry.value;
    }

    private void clean(boolean max) {
      PriorityQueue<Entry> heap = max ? maximum : minimum;
      while (!heap.isEmpty() && discarded.contains(heap.peek().id)) {
        discarded.remove(heap.poll().id);
      }
    }
  }

  public static final class Corpus {
    public int version;
    public long seed;
    public int maximumSize;
    public String inputDistribution;
    public List<Operation> operations;
    public List<Operation> oneEnded;
  }

  public static final class Operation {
    public String op;
    public int value;
  }
}

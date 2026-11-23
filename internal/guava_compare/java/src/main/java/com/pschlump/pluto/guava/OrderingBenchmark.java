package com.pschlump.pluto.guava;

import com.google.common.collect.Ordering;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.BenchmarkMode;
import org.openjdk.jmh.annotations.Fork;
import org.openjdk.jmh.annotations.Measurement;
import org.openjdk.jmh.annotations.Mode;
import org.openjdk.jmh.annotations.OutputTimeUnit;
import org.openjdk.jmh.annotations.Scope;
import org.openjdk.jmh.annotations.Setup;
import org.openjdk.jmh.annotations.State;
import org.openjdk.jmh.annotations.Warmup;

@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@State(Scope.Benchmark)
public class OrderingBenchmark {
  private Integer[] values;
  private List<Integer> input;

  @Setup
  public void setup() {
    values = new Integer[10000];
    for (int i = 0; i < values.length; i++) {
      values[i] = (int) ((i * 1103515245L + 12345) & 0x7fffffffL) % 100000;
    }
    input = Arrays.asList(values);
  }

  @Benchmark public List<Integer> orderingSortedCopy() {
    return Ordering.natural().sortedCopy(input);
  }

  @Benchmark public List<Integer> orderingLeastOf() {
    return Ordering.natural().leastOf(input, 32);
  }

  @Benchmark public List<Integer> orderingGreatestOf() {
    return Ordering.natural().greatestOf(input, 32);
  }

  @Benchmark public int[] orderingFullSort() {
    int[] copy = new int[values.length];
    for (int i = 0; i < values.length; i++) copy[i] = values[i];
    Arrays.sort(copy);
    return copy;
  }
}

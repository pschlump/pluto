package com.pschlump.pluto.guava;

import com.google.common.math.PairedStatsAccumulator;
import com.google.common.math.Quantiles;
import com.google.common.math.StatsAccumulator;
import java.util.Arrays;
import java.util.Map;
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
public class StatsBenchmark {
  private double[] values;
  private double[] paired;
  private int[] indexes;

  @Setup
  public void setup() {
    values = new double[10000];
    paired = new double[values.length];
    for (int i = 0; i < values.length; i++) {
      values[i] = ((i * 7919) % 1000) / 7.0;
      paired[i] = 3 * values[i] + i % 17;
    }
    indexes = new int[] {0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100};
  }

  @Benchmark public double statsAccumulator() {
    StatsAccumulator accumulator = new StatsAccumulator();
    accumulator.addAll(values);
    return accumulator.populationVariance();
  }

  @Benchmark public double statsPaired() {
    PairedStatsAccumulator accumulator = new PairedStatsAccumulator();
    for (int i = 0; i < values.length; i++) accumulator.add(values[i], paired[i]);
    return accumulator.pearsonsCorrelationCoefficient();
  }

  @Benchmark public Map<Integer, Double> statsQuantiles() {
    return Quantiles.scale(100).indexes(indexes).compute(values);
  }

  @Benchmark public double[] statsQuantilesSort() {
    double[] ordered = values.clone();
    Arrays.sort(ordered);
    double[] result = new double[indexes.length];
    for (int i = 0; i < indexes.length; i++) {
      long numerator = (long) indexes[i] * (ordered.length - 1);
      int quotient = (int) (numerator / 100);
      int remainder = (int) (numerator % 100);
      if (remainder == 0) result[i] = ordered[quotient];
      else result[i] = ordered[quotient] + (ordered[quotient + 1] - ordered[quotient]) * remainder / 100.0;
    }
    return result;
  }
}

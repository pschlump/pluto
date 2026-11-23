package com.pschlump.pluto.guava;

import com.google.common.hash.BloomFilter;
import com.google.common.hash.Funnels;
import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
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
import org.openjdk.jmh.annotations.Warmup;

@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@State(Scope.Benchmark)
public class BloomFilterBenchmark {
  private BloomFilter<String> filter;
  private BloomFilter<String> left;
  private BloomFilter<String> right;
  private String[] values;
  private String[] absent;
  private int index;

  @Setup(Level.Trial)
  public void setup() {
    values = new String[1000];
    absent = new String[1000];
    for (int i = 0; i < 1000; i++) {
      values[i] = String.format("value-%04d", i);
      absent[i] = String.format("absent-%04d", i);
    }
    filter = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 1000, 0.01);
    left = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 1000, 0.01);
    right = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 1000, 0.01);
    for (int i = 0; i < 1000; i++) {
      filter.put(values[i]);
      left.put(String.format("left-%d", i));
      right.put(String.format("right-%d", i));
    }
  }

  @Setup(Level.Invocation)
  public void invocation() {
    index = (index + 1) & 1023;
  }

  @Benchmark
  public boolean bloomFilterPut() {
    BloomFilter<String> target = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 100000, 0.01);
    boolean result = false;
    for (int i = 0; i < 1000; i++) result |= target.put(values[i]);
    return result;
  }

  @Benchmark public boolean bloomFilterHit() { return filter.mightContain(values[index % 1000]); }
  @Benchmark public boolean bloomFilterMiss() { return filter.mightContain(absent[index % 1000]); }

  @Benchmark
  public boolean bloomFilterMerge() {
    BloomFilter<String> copy = left.copy();
    copy.putAll(right);
    return copy.mightContain(values[0]);
  }

  @Benchmark
  public byte[] bloomFilterSerialization() throws Exception {
    ByteArrayOutputStream output = new ByteArrayOutputStream();
    filter.writeTo(output);
    return output.toByteArray();
  }
}

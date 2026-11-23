package com.pschlump.pluto.guava;

import com.google.common.math.LongMath;
import java.math.RoundingMode;
import java.util.concurrent.TimeUnit;
import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.BenchmarkMode;
import org.openjdk.jmh.annotations.Fork;
import org.openjdk.jmh.annotations.Measurement;
import org.openjdk.jmh.annotations.Mode;
import org.openjdk.jmh.annotations.OutputTimeUnit;
import org.openjdk.jmh.annotations.Scope;
import org.openjdk.jmh.annotations.State;
import org.openjdk.jmh.annotations.Warmup;

@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@State(Scope.Benchmark)
public class MathBenchmark {
  @Benchmark public long mathDivide() {
    return LongMath.divide(9223372036854775806L, 37, RoundingMode.HALF_EVEN);
  }
  @Benchmark public long mathLog10() {
    return LongMath.log10(Long.MAX_VALUE - 24, RoundingMode.HALF_EVEN);
  }
  @Benchmark public long mathSqrt() {
    return LongMath.sqrt(Long.MAX_VALUE - 24, RoundingMode.HALF_EVEN);
  }
  @Benchmark public boolean mathIsPrime() {
    return LongMath.isPrime(Long.MAX_VALUE - 24);
  }
  @Benchmark public long mathBinomial() {
    return LongMath.binomial(30, 15);
  }
}

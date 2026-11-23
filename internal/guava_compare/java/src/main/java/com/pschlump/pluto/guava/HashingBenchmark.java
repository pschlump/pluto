package com.pschlump.pluto.guava;

import com.google.common.hash.HashCode;
import com.google.common.hash.HashFunction;
import com.google.common.hash.Hasher;
import com.google.common.hash.Hashing;
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
public class HashingBenchmark {
  private byte[] data16;
  private byte[] data64;
  private byte[] data1024;
  private HashFunction murmur32;
  private HashFunction murmur128;
  private HashFunction sip;
  private HashFunction farm;
  private HashFunction fingerprint;
  private HashCode consistent;

  @Setup
  public void setup() {
    data16 = input(16);
    data64 = input(64);
    data1024 = input(1024);
    murmur32 = Hashing.murmur3_32();
    murmur128 = Hashing.murmur3_128();
    sip = Hashing.sipHash24();
    farm = Hashing.farmHashFingerprint64();
    fingerprint = Hashing.fingerprint2011();
    consistent = murmur128.hashString("bucket-key", java.nio.charset.StandardCharsets.UTF_8);
  }

  private static byte[] input(int size) {
    byte[] data = new byte[size];
    for (int i = 0; i < size; i++) data[i] = (byte) (i * 131 + size);
    return data;
  }

  @Benchmark public HashCode murmur3_128_16() { return murmur128.hashBytes(data16); }
  @Benchmark public HashCode murmur3_128_64() { return murmur128.hashBytes(data64); }
  @Benchmark public HashCode murmur3_128_1024() { return murmur128.hashBytes(data1024); }
  @Benchmark public HashCode murmur3_32_1024() { return murmur32.hashBytes(data1024); }
  @Benchmark public HashCode sipHash24_64() { return sip.hashBytes(data64); }
  @Benchmark public HashCode farmHashFingerprint64_1024() { return farm.hashBytes(data1024); }
  @Benchmark public HashCode fingerprint2011_1024() { return fingerprint.hashBytes(data1024); }
  @Benchmark public int consistentHash1024() { return Hashing.consistentHash(consistent, 1024); }

  @Benchmark
  public HashCode murmur3_128_stream1() {
    Hasher hasher = murmur128.newHasher();
    for (int i = 0; i < data1024.length; i++) hasher.putByte(data1024[i]);
    return hasher.hash();
  }

  @Benchmark
  public HashCode murmur3_128_stream7() {
    Hasher hasher = murmur128.newHasher();
    for (int i = 0; i < data1024.length; i += 7) hasher.putBytes(data1024, i, Math.min(7, data1024.length - i));
    return hasher.hash();
  }

  @Benchmark
  public HashCode murmur3_128_stream64() {
    Hasher hasher = murmur128.newHasher();
    for (int i = 0; i < data1024.length; i += 64) hasher.putBytes(data1024, i, 64);
    return hasher.hash();
  }
}

/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.base.Ticker;
import com.google.common.cache.CacheBuilder;
import com.google.common.cache.CacheLoader;
import com.google.common.cache.LoadingCache;
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

/** Matched Phase 4 loading, expiration, refresh, and eviction workloads. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class CacheBenchmark {
  private LoadingCache<String, String> hits;
  private LoadingCache<String, String> evicting;
  private LoadingCache<String, String> misses;
  private LoadingCache<String, String> expiration;
  private LoadingCache<String, String> refresh;
  private FakeTicker expirationTicker;
  private FakeTicker refreshTicker;
  private Operation[] operations;
  private String[] zipfKeys;
  private String[] scanKeys;
  private String[] phaseKeys;
  private int index;
  private int distributionIndex;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty("cacheCorpus", "internal/guava_compare/testdata/cache.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    operations = corpus.operations.toArray(new Operation[0]);
    zipfKeys = corpus.zipfKeys.toArray(new String[0]);
    scanKeys = corpus.scanKeys.toArray(new String[0]);
    phaseKeys = corpus.phaseKeys.toArray(new String[0]);
    CacheLoader<String, String> loader = new CacheLoader<String, String>() {
      @Override public String load(String key) { return key + "!"; }
    };
    hits = CacheBuilder.newBuilder().build(loader);
    evicting = CacheBuilder.newBuilder().maximumSize(256).build(loader);
    misses = CacheBuilder.newBuilder().build(loader);
    expirationTicker = new FakeTicker();
    refreshTicker = new FakeTicker();
    expiration = CacheBuilder.newBuilder().expireAfterWrite(1, TimeUnit.NANOSECONDS)
        .ticker(expirationTicker).build(loader);
    refresh = CacheBuilder.newBuilder().refreshAfterWrite(1, TimeUnit.NANOSECONDS)
        .ticker(refreshTicker).build(loader);
    for (Operation operation : operations) hits.put(operation.key, operation.value);
  }

  @Benchmark public int cacheHit() {
    Operation operation = next();
    if (operation.op.equals("put")) { hits.put(operation.key, operation.value); return 1; }
    return hits.getIfPresent(operation.key) != null ? 1 : 0;
  }

  @Benchmark public int cacheEvictingWrite() {
    Operation operation = next();
    evicting.put(operation.key, operation.value);
    return 1;
  }

  @Benchmark public String cacheMissLoading() throws Exception {
    String key = next().key;
    misses.invalidate(key);
    return misses.get(key);
  }

  @Benchmark public String cacheExpiration() throws Exception {
    expirationTicker.nanos += 2;
    return expiration.get(next().key);
  }

  @Benchmark public String cacheRefresh() throws Exception {
    refreshTicker.nanos += 2;
    return refresh.get(next().key);
  }

  @Benchmark public int cacheHitZipf() { return hit(zipfKeys); }
  @Benchmark public int cacheHitScan() { return hit(scanKeys); }
  @Benchmark public int cacheHitPhase() { return hit(phaseKeys); }

  @Threads(8)
  @Benchmark public int cacheConcurrentHit8() { return hit(operations); }

  @Threads(16)
  @Benchmark public int cacheConcurrentHit16() { return hit(operations); }

  @Threads(8)
  @Benchmark public String cacheCoalescedMiss8() throws Exception {
    misses.invalidate("shared");
    return misses.get("shared");
  }

  private int hit(String[] keys) {
    return hits.getIfPresent(keys[distributionIndex++ % keys.length]) != null ? 1 : 0;
  }

  private int hit(Operation[] values) {
    Operation operation = values[distributionIndex++ % values.length];
    return hits.getIfPresent(operation.key) != null ? 1 : 0;
  }

  private Operation next() { return operations[index++ % operations.length]; }
  private static final class FakeTicker extends Ticker {
    long nanos;
    @Override public long read() { return nanos; }
  }
  public static final class Corpus {
    public int version;
    public long seed;
    public int capacity;
    public String inputDistribution;
    public List<Operation> operations;
    public List<String> zipfKeys;
    public List<String> scanKeys;
    public List<String> phaseKeys;
  }
  public static final class Operation {
    public String op;
    public String key;
    public String value;
  }
}

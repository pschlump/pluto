/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package pluto.bench;

import com.github.benmanes.caffeine.cache.Cache;
import com.github.benmanes.caffeine.cache.Caffeine;
import org.openjdk.jmh.annotations.Benchmark;
import org.openjdk.jmh.annotations.Level;
import org.openjdk.jmh.annotations.Scope;
import org.openjdk.jmh.annotations.Setup;
import org.openjdk.jmh.annotations.State;
import org.openjdk.jmh.annotations.Threads;

/** Matched workload for tiny_lfu/bench_test.go. */
@State(Scope.Benchmark)
public class CaffeineBenchmark {
  public static final int CAPACITY = 32768;
  public static final int KEYS = 2 * CAPACITY;

  Cache<Integer, Boolean> cache;
  Integer[] keys;

  @State(Scope.Thread)
  public static class ThreadState {
    int index;
  }

  @Setup(Level.Trial)
  public void setup() {
    keys = new Integer[KEYS];
    for (int i = 0; i < KEYS; i++) {
      keys[i] = i;
    }
    cache = Caffeine.newBuilder()
        .initialCapacity(CAPACITY)
        .maximumSize(CAPACITY)
        .executor(Runnable::run)
        .build();
    for (int i = 0; i < CAPACITY; i++) {
      cache.put(keys[i], Boolean.TRUE);
    }
    cache.cleanUp();
  }

  @Benchmark @Threads(1)
  public Boolean get1(ThreadState state) {
    return cache.getIfPresent(keys[state.index++ & (CAPACITY - 1)]);
  }

  @Benchmark @Threads(1)
  public void putUpdate1(ThreadState state) {
    cache.put(keys[state.index++ & (CAPACITY - 1)], Boolean.TRUE);
  }

  @Benchmark @Threads(8)
  public Boolean get8(ThreadState state) {
    return cache.getIfPresent(keys[state.index++ & (CAPACITY - 1)]);
  }

  @Benchmark @Threads(8)
  public void putUpdate8(ThreadState state) {
    cache.put(keys[state.index++ & (CAPACITY - 1)], Boolean.TRUE);
  }

  @Benchmark @Threads(8)
  public Boolean mixed8(ThreadState state) {
    int keyIndex = state.index++ & (CAPACITY - 1);
    Integer key = keys[keyIndex];
    if (keyIndex % 20 == 0) {
      cache.put(key, Boolean.TRUE);
      return Boolean.TRUE;
    }
    return cache.getIfPresent(key);
  }

  @Benchmark @Threads(1)
  public void putEvict1(ThreadState state) {
    cache.put(keys[state.index++ & (KEYS - 1)], Boolean.TRUE);
  }
}

/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.cache.CacheLoader;
import com.google.common.cache.CacheStats;
import com.google.common.cache.LoadingCache;
import com.google.common.cache.CacheBuilder;
import com.google.common.base.Ticker;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/** Executes the Phase 4 loading/expiration trace against Guava LocalCache. */
public final class CacheOracle {
  private CacheOracle() {}

  public static void main(String[] arguments) throws Exception {
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    FakeTicker ticker = new FakeTicker();
    LoadingCache<String, String> cache = CacheBuilder.newBuilder()
        .expireAfterAccess(10, TimeUnit.NANOSECONDS)
        .recordStats()
        .ticker(ticker)
        .build(new CacheLoader<String, String>() {
          @Override public String load(String key) throws Exception {
            if (key.equals("bad")) throw new Exception("load");
            return key + "!";
          }
        });
    List<Object> results = new ArrayList<>();
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "put": cache.put(operation.key, operation.value); results.add(null); break;
        case "getIfPresent": results.add(cache.getIfPresent(operation.key)); break;
        case "get":
          try { results.add(cache.get(operation.key)); } catch (Exception error) { results.add("error"); }
          break;
        case "size": results.add(cache.size()); break;
        case "advance": ticker.nanos += operation.nanos; results.add(null); break;
        case "invalidate": cache.invalidate(operation.key); results.add(null); break;
        case "invalidateAll": cache.invalidateAll(); results.add(null); break;
        default: throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }
    Map<String, Object> output = new LinkedHashMap<>();
    output.put("version", 1);
    output.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    output.put("results", results);
    output.put("finalSize", cache.size());
    CacheStats stats = cache.stats();
    Map<String, Object> statValues = new LinkedHashMap<>();
    statValues.put("hits", stats.hitCount());
    statValues.put("misses", stats.missCount());
    statValues.put("loads", stats.loadCount());
    statValues.put("successes", stats.loadSuccessCount());
    statValues.put("failures", stats.loadExceptionCount());
    statValues.put("evictions", stats.evictionCount());
    output.put("stats", statValues);
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(output) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static final class FakeTicker extends Ticker {
    long nanos;
    @Override public long read() { return nanos; }
  }

  public static final class Trace {
    public int version;
    public List<Operation> operations;
  }
  public static final class Operation {
    public String op;
    public String key;
    public String value;
    public long nanos;
  }
}

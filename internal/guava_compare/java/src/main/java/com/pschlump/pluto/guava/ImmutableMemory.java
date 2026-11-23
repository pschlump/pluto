/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ImmutableBiMap;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableListMultimap;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableMultiset;
import com.google.common.collect.ImmutableSet;
import com.google.common.collect.ImmutableSortedMap;
import com.google.common.collect.ImmutableSortedSet;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.openjdk.jol.info.GraphLayout;

/** JOL retained-footprint driver for Phase 2 immutable collections. */
public final class ImmutableMemory {
  private ImmutableMemory() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: ImmutableMemory CORPUS OUTPUT");
    }
    Corpus corpus = new ObjectMapper().readValue(new File(arguments[0]), Corpus.class);
    Integer[] values = corpus.values.toArray(new Integer[0]);
    String[] keys = corpus.keys.toArray(new String[0]);
    List<Object> results = new ArrayList<>();
    results.add(measure("list", ImmutableList.copyOf(Arrays.asList(values)), corpus.size));
    results.add(measure("set", ImmutableSet.copyOf(Arrays.asList(values)), corpus.size));
    results.add(measure("map", map(keys, values), corpus.size));
    results.add(measure("sortedSet", ImmutableSortedSet.copyOf(Arrays.asList(values)), corpus.size));
    results.add(measure("sortedMap", sortedMap(keys, values), corpus.size));
    results.add(measure("multiset", ImmutableMultiset.copyOf(Arrays.asList(values)), corpus.size));
    results.add(measure("multimap", multimap(keys, values), corpus.size));
    results.add(measure("bimap", bimap(keys, values), corpus.size));
    Map<String, Object> output = new LinkedHashMap<>();
    output.put("version", 1);
    output.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    output.put("method", "JOL GraphLayout totalSize; includes reachable shared input objects");
    output.put("results", results);
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(output) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> measure(String name, Object value, int size) {
    long bytes = GraphLayout.parseInstance(value).totalSize();
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("name", name);
    result.put("bytes", bytes);
    result.put("bytesPerInput", (double) bytes / size);
    return result;
  }

  private static ImmutableMap<String, Integer> map(String[] keys, Integer[] values) {
    ImmutableMap.Builder<String, Integer> builder = ImmutableMap.builder();
    for (int i = 0; i < keys.length; i++) builder.put(keys[i], values[i]);
    return builder.build();
  }

  private static ImmutableSortedMap<String, Integer> sortedMap(String[] keys, Integer[] values) {
    ImmutableSortedMap.Builder<String, Integer> builder = ImmutableSortedMap.naturalOrder();
    for (int i = 0; i < keys.length; i++) builder.put(keys[i], values[i]);
    return builder.build();
  }

  private static ImmutableListMultimap<String, Integer> multimap(String[] keys, Integer[] values) {
    ImmutableListMultimap.Builder<String, Integer> builder = ImmutableListMultimap.builder();
    for (int i = 0; i < keys.length; i++) builder.put(keys[i % 64], values[i]);
    return builder.build();
  }

  private static ImmutableBiMap<String, Integer> bimap(String[] keys, Integer[] values) {
    ImmutableBiMap.Builder<String, Integer> builder = ImmutableBiMap.builder();
    for (int i = 0; i < keys.length; i++) builder.put(keys[i], i);
    return builder.build();
  }

  public static final class Corpus {
    public int version;
    public long seed;
    public int size;
    public String inputDistribution;
    public List<Integer> values;
    public List<String> keys;
  }
}

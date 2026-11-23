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
import java.util.Arrays;
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

/** Matched Phase 2 immutable construction and lookup workloads. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class ImmutableBenchmark {
  private Integer[] values;
  private String[] keys;
  private int size;
  private int index;
  private ImmutableList<Integer> list;
  private ImmutableSet<Integer> set;
  private ImmutableMap<String, Integer> map;
  private ImmutableSortedSet<Integer> sortedSet;
  private ImmutableSortedMap<String, Integer> sortedMap;
  private ImmutableMultiset<Integer> multiset;
  private ImmutableListMultimap<String, Integer> multimap;
  private ImmutableBiMap<String, Integer> bimap;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "immutableCorpus", "internal/guava_compare/testdata/immutable.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    size = corpus.size;
    values = corpus.values.toArray(new Integer[0]);
    keys = corpus.keys.toArray(new String[0]);
    list = ImmutableList.copyOf(Arrays.asList(values));
    set = ImmutableSet.copyOf(Arrays.asList(values));
    map = buildMap();
    sortedSet = ImmutableSortedSet.copyOf(Arrays.asList(values));
    sortedMap = buildSortedMap();
    multiset = ImmutableMultiset.copyOf(Arrays.asList(values));
    multimap = buildMultimap();
    bimap = buildBiMap();
  }

  @Benchmark public ImmutableList<Integer> immutableListBuild() {
    return ImmutableList.copyOf(Arrays.asList(values));
  }
  @Benchmark public ImmutableList<Integer> immutableListCopyOf() { return ImmutableList.copyOf(list); }
  @Benchmark public int immutableListAt() { return list.get(index++ % size); }
  @Benchmark public int immutableListIteration() {
    int result = 0;
    for (int value : list) result += value;
    return result;
  }

  @Benchmark public ImmutableSet<Integer> immutableSetBuild() {
    return ImmutableSet.copyOf(Arrays.asList(values));
  }
  @Benchmark public boolean immutableSetContains() { return set.contains(values[index++ % size]); }

  @Benchmark public ImmutableMap<String, Integer> immutableMapBuild() { return buildMap(); }
  @Benchmark public int immutableMapGet() { return map.get(keys[index++ % size]); }

  @Benchmark public ImmutableSortedSet<Integer> immutableSortedSetBuild() {
    return ImmutableSortedSet.copyOf(Arrays.asList(values));
  }
  @Benchmark public boolean immutableSortedSetContains() { return sortedSet.contains(values[index++ % size]); }
  @Benchmark public int immutableSortedSetSubRange() {
    return sortedSet.subSet(size / 4, true, size * 3 / 4, true).size();
  }

  @Benchmark public ImmutableSortedMap<String, Integer> immutableSortedMapBuild() { return buildSortedMap(); }
  @Benchmark public int immutableSortedMapGet() { return sortedMap.get(keys[index++ % size]); }

  @Benchmark public ImmutableMultiset<Integer> immutableMultisetBuild() {
    return ImmutableMultiset.copyOf(Arrays.asList(values));
  }
  @Benchmark public int immutableMultisetCount() { return multiset.count(values[index++ % size]); }

  @Benchmark public ImmutableListMultimap<String, Integer> immutableMultimapBuild() { return buildMultimap(); }
  @Benchmark public int immutableMultimapGet() { return multimap.get(keys[index++ % 64]).size(); }

  @Benchmark public ImmutableBiMap<String, Integer> immutableBiMapBuild() { return buildBiMap(); }
  @Benchmark public int immutableBiMapGet() { return bimap.get(keys[index++ % size]); }

  private ImmutableMap<String, Integer> buildMap() {
    ImmutableMap.Builder<String, Integer> builder = ImmutableMap.builder();
    for (int i = 0; i < size; i++) builder.put(keys[i], values[i]);
    return builder.build();
  }

  private ImmutableSortedMap<String, Integer> buildSortedMap() {
    ImmutableSortedMap.Builder<String, Integer> builder = ImmutableSortedMap.naturalOrder();
    for (int i = 0; i < size; i++) builder.put(keys[i], values[i]);
    return builder.build();
  }

  private ImmutableListMultimap<String, Integer> buildMultimap() {
    ImmutableListMultimap.Builder<String, Integer> builder = ImmutableListMultimap.builder();
    for (int i = 0; i < size; i++) builder.put(keys[i % 64], values[i]);
    return builder.build();
  }

  private ImmutableBiMap<String, Integer> buildBiMap() {
    ImmutableBiMap.Builder<String, Integer> builder = ImmutableBiMap.builder();
    for (int i = 0; i < size; i++) builder.put(keys[i], i);
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

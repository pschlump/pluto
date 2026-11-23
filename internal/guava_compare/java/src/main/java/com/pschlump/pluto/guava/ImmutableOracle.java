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
import com.google.common.collect.ImmutableTable;
import com.google.common.collect.Multiset;
import com.google.common.collect.Table;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Executes the Phase 2 immutable-collection fixture against Guava. */
public final class ImmutableOracle {
  private ImmutableOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: ImmutableOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    result.put("list", list(trace.list));
    result.put("set", set(trace.set));
    result.put("map", map(trace.map));
    result.put("sortedSet", sortedSet(trace.sortedSet));
    result.put("sortedMap", sortedMap(trace.sortedMap));
    result.put("multiset", multiset(trace.multiset));
    result.put("multimap", multimap(trace.multimap));
    result.put("bimap", bimap(trace.bimap));
    result.put("table", table(trace.table));
    Files.write(new File(arguments[1]).toPath(),
        (new ObjectMapper().writeValueAsString(result) + "\n").getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> list(List<Integer> input) {
    ImmutableList<Integer> value = ImmutableList.copyOf(input);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("elements", value);
    result.put("size", value.size());
    result.put("at", value.get(1));
    result.put("subList", value.subList(1, 3));
    result.put("reverse", value.reverse());
    return result;
  }

  private static Map<String, Object> set(List<String> input) {
    ImmutableSet<String> value = ImmutableSet.copyOf(input);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("elements", value.asList());
    result.put("containsA", value.contains("a"));
    result.put("containsZ", value.contains("z"));
    return result;
  }

  private static Map<String, Object> map(List<Entry> input) {
    ImmutableMap.Builder<String, Integer> builder = ImmutableMap.builder();
    for (Entry entry : input) builder.put(entry.key, entry.value);
    ImmutableMap<String, Integer> value = builder.build();
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("entries", entries(value.entrySet()));
    result.put("size", value.size());
    result.put("getA", value.get("a"));
    result.put("getZ", value.get("z"));
    return result;
  }

  private static Map<String, Object> sortedSet(List<Integer> input) {
    ImmutableSortedSet<Integer> value = ImmutableSortedSet.copyOf(input);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("elements", value.asList());
    result.put("subRange", value.subSet(2, true, 4, true).asList());
    result.put("higher", value.higher(2));
    result.put("lower", value.lower(4));
    result.put("reverse", value.descendingSet().asList());
    return result;
  }

  private static Map<String, Object> sortedMap(List<Entry> input) {
    ImmutableSortedMap.Builder<String, Integer> builder = ImmutableSortedMap.naturalOrder();
    for (Entry entry : input) builder.put(entry.key, entry.value);
    ImmutableSortedMap<String, Integer> value = builder.build();
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("entries", entries(value.entrySet()));
    result.put("head", entries(value.headMap("c", false).entrySet()));
    result.put("getZ", value.get("z"));
    return result;
  }

  private static Map<String, Object> multiset(List<String> input) {
    ImmutableMultiset<String> value = ImmutableMultiset.copyOf(input);
    List<Object> entries = new ArrayList<>();
    for (Multiset.Entry<String> entry : value.entrySet()) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("value", entry.getElement());
      item.put("count", entry.getCount());
      entries.add(item);
    }
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("entries", entries);
    result.put("size", value.size());
    result.put("countB", value.count("b"));
    result.put("all", new ArrayList<>(value));
    return result;
  }

  private static Map<String, Object> multimap(List<Entry> input) {
    ImmutableListMultimap.Builder<String, Integer> builder = ImmutableListMultimap.builder();
    for (Entry entry : input) builder.put(entry.key, entry.value);
    ImmutableListMultimap<String, Integer> value = builder.build();
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("entries", entries(value.entries()));
    result.put("size", value.size());
    result.put("keyLen", value.keySet().size());
    result.put("getA", value.get("a"));
    return result;
  }

  private static Map<String, Object> bimap(List<Entry> input) {
    ImmutableBiMap.Builder<String, Integer> builder = ImmutableBiMap.builder();
    for (Entry entry : input) builder.put(entry.key, entry.value);
    ImmutableBiMap<String, Integer> value = builder.build();
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("entries", entries(value.entrySet()));
    result.put("getA", value.get("a"));
    result.put("inverseOne", value.inverse().get(1));
    return result;
  }

  private static Map<String, Object> table(List<Cell> input) {
    ImmutableTable.Builder<String, String, Integer> builder = ImmutableTable.builder();
    for (Cell cell : input) builder.put(cell.row, cell.column, cell.value);
    ImmutableTable<String, String, Integer> value = builder.build();
    List<Object> cells = new ArrayList<>();
    for (Table.Cell<String, String, Integer> cell : value.cellSet()) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("row", cell.getRowKey());
      item.put("column", cell.getColumnKey());
      item.put("value", cell.getValue());
      cells.add(item);
    }
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("cells", cells);
    result.put("size", value.size());
    result.put("getAY", value.get("a", "y"));
    return result;
  }

  private static List<Object> entries(Iterable<Map.Entry<String, Integer>> input) {
    List<Object> result = new ArrayList<>();
    for (Map.Entry<String, Integer> entry : input) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("key", entry.getKey());
      item.put("value", entry.getValue());
      result.add(item);
    }
    return result;
  }

  public static final class Trace {
    public int version;
    public List<Integer> list;
    public List<String> set;
    public List<Entry> map;
    public List<Integer> sortedSet;
    public List<Entry> sortedMap;
    public List<String> multiset;
    public List<Entry> multimap;
    public List<Entry> bimap;
    public List<Cell> table;
  }

  public static final class Entry {
    public String key;
    public int value;
  }

  public static final class Cell {
    public String row;
    public String column;
    public int value;
  }
}

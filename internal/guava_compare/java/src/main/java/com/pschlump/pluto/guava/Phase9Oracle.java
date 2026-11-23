/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.collect.Interner;
import com.google.common.collect.Interners;
import com.google.common.collect.LinkedHashMultimap;
import com.google.common.collect.LinkedHashMultiset;
import com.google.common.collect.LinkedListMultimap;
import com.google.common.collect.Multiset;
import java.io.File;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Generates the Phase 9 insertion-order and strong-interning golden files. */
public final class Phase9Oracle {
  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: Phase9Oracle output-directory");
    File directory = new File(args[0]);
    if (!directory.isDirectory() && !directory.mkdirs()) {
      throw new IllegalArgumentException("cannot create " + directory);
    }
    ObjectMapper mapper = new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT);
    interning(mapper, new File(directory, "guava_intern.json"));
    linkedMap(mapper, new File(directory, "guava_linked_map.json"));
    linkedMultiset(mapper, new File(directory, "guava_linked_multiset.json"));
    linkedMultimap(mapper, new File(directory, "guava_linked_multimap.json"));
  }

  private static void interning(ObjectMapper mapper, File output) throws Exception {
    Interner<String> interner = Interners.newStrongInterner();
    String first = new String("pluto");
    String canonical = interner.intern(first);
    boolean same = canonical == first;
    List<String> values = new ArrayList<>();
    for (int i = 0; i < 100; i++) {
      String value = interner.intern(new String("pluto"));
      same &= value == canonical;
      if (i < 3) values.add(value);
    }
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("firstRetained", same);
    root.put("values", values);
    mapper.writeValue(output, root);
  }

  private static void linkedMap(ObjectMapper mapper, File output) throws Exception {
    Map<String, Integer> map = new LinkedHashMap<>();
    map.put("b", 2);
    map.put("a", 1);
    map.put("c", 3);
    map.put("a", 10);
    map.remove("a");
    map.put("a", 11);
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("keys", new ArrayList<>(map.keySet()));
    root.put("values", new ArrayList<>(map.values()));
    mapper.writeValue(output, root);
  }

  private static void linkedMultiset(ObjectMapper mapper, File output) throws Exception {
    Multiset<String> multiset = LinkedHashMultiset.create();
    multiset.add("b", 2);
    multiset.add("a", 1);
    multiset.add("c", 3);
    multiset.add("a", 2);
    multiset.setCount("a", 0);
    multiset.add("a", 1);
    List<Map<String, Object>> entries = new ArrayList<>();
    for (Multiset.Entry<String> entry : multiset.entrySet()) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("value", entry.getElement());
      item.put("count", entry.getCount());
      entries.add(item);
    }
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("entries", entries);
    root.put("all", new ArrayList<>(multiset));
    mapper.writeValue(output, root);
  }

  private static void linkedMultimap(ObjectMapper mapper, File output) throws Exception {
    LinkedListMultimap<String, Integer> list = LinkedListMultimap.create();
    list.put("b", 2);
    list.put("a", 1);
    list.put("b", 3);
    list.put("a", 1);
    List<Map<String, Object>> initialListEntries = entries(list.entries());
    list.removeAll("b");
    list.put("b", 4);
    LinkedHashMultimap<String, Integer> set = LinkedHashMultimap.create();
    set.putAll("b", Arrays.asList(3, 1, 3, 2));
    set.put("a", 9);
    set.put("b", 1);
    List<Map<String, Object>> initialSetEntries = entries(set.entries());
    set.remove("b", 1);
    set.put("b", 1);
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("listKeys", new ArrayList<>(list.keySet()));
    root.put("initialListEntries", initialListEntries);
    root.put("listEntries", entries(list.entries()));
    root.put("setKeys", new ArrayList<>(set.keySet()));
    root.put("initialSetEntries", initialSetEntries);
    root.put("setEntries", entries(set.entries()));
    mapper.writeValue(output, root);
  }

  private static List<Map<String, Object>> entries(Iterable<Map.Entry<String, Integer>> input) {
    List<Map<String, Object>> result = new ArrayList<>();
    for (Map.Entry<String, Integer> entry : input) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("key", entry.getKey());
      item.put("value", entry.getValue());
      result.add(item);
    }
    return result;
  }
}

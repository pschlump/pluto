/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ArrayListMultimap;
import com.google.common.collect.HashMultimap;
import com.google.common.collect.Multimap;
import com.google.common.collect.TreeMultimap;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/** Executes the Phase 1b trace against all three Guava multimap variants. */
public final class MultimapOracle {
  private static final String COMMIT = "36e3995499d6c3b74b1207d362fa9e19915d2589";

  private MultimapOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: MultimapOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    if (trace.version != 1) {
      throw new IllegalArgumentException("unsupported trace");
    }
    Map<String, Object> fixtures = new LinkedHashMap<>();
    fixtures.put("list", replay(ArrayListMultimap.create(), trace.operations, true));
    fixtures.put("set", replay(HashMultimap.create(), trace.operations, false));
    fixtures.put("sortedSet", replay(TreeMultimap.create(), trace.operations, false));
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", COMMIT);
    result.put("fixtures", fixtures);
    String encoded = new ObjectMapper().writeValueAsString(result) + "\n";
    Files.write(new File(arguments[1]).toPath(), encoded.getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> replay(
      Multimap<String, String> multimap, List<Operation> operations, boolean listOrder) {
    List<Object> results = new ArrayList<>();
    for (Operation operation : operations) {
      switch (operation.op) {
        case "put":
          results.add(multimap.put(operation.key, operation.value));
          break;
        case "putAll":
          int before = multimap.size();
          multimap.putAll(operation.key, operation.values);
          results.add(multimap.size() - before);
          break;
        case "get":
          results.add(values(multimap.get(operation.key), listOrder));
          break;
        case "containsEntry":
          results.add(multimap.containsEntry(operation.key, operation.value));
          break;
        case "remove":
          results.add(multimap.remove(operation.key, operation.value));
          break;
        case "replaceValues":
          results.add(values(multimap.replaceValues(operation.key, operation.values), listOrder));
          break;
        case "removeAll":
          results.add(values(multimap.removeAll(operation.key), listOrder));
          break;
        case "size":
          results.add(multimap.size());
          break;
        case "keySize":
          results.add(multimap.keySet().size());
          break;
        default:
          throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }

    List<Map<String, Object>> entries = new ArrayList<>();
    for (String key : new TreeSet<>(multimap.keySet())) {
      Map<String, Object> entry = new LinkedHashMap<>();
      entry.put("key", key);
      entry.put("values", values(multimap.get(key), listOrder));
      entries.add(entry);
    }
    Map<String, Object> finalState = new LinkedHashMap<>();
    finalState.put("size", multimap.size());
    finalState.put("keySize", multimap.keySet().size());
    finalState.put("asMap", entries);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("results", results);
    result.put("final", finalState);
    return result;
  }

  private static List<String> values(Collection<String> source, boolean preserveOrder) {
    List<String> result = new ArrayList<>(source);
    if (!preserveOrder) {
      Collections.sort(result);
    }
    return result;
  }

  public static final class Trace {
    public int version;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String key;
    public String value;
    public List<String> values;
  }
}

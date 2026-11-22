/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.HashBiMap;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/** Executes the Phase 1c trace against Guava HashBiMap. */
public final class BimapOracle {
  private BimapOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: BimapOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    HashBiMap<String, String> bimap = HashBiMap.create();
    List<Object> results = new ArrayList<>();
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "put":
          try {
            bimap.put(operation.key, operation.value);
            results.add(true);
          } catch (IllegalArgumentException conflict) {
            results.add(false);
          }
          break;
        case "forcePut":
          bimap.forcePut(operation.key, operation.value);
          results.add(true);
          break;
        case "getValue":
          results.add(bimap.get(operation.key));
          break;
        case "getKey":
          results.add(bimap.inverse().get(operation.value));
          break;
        case "deleteKey":
          results.add(bimap.remove(operation.key));
          break;
        case "deleteValue":
          results.add(bimap.inverse().remove(operation.value));
          break;
        case "size":
          results.add(bimap.size());
          break;
        default:
          throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }
    List<Map<String, String>> entries = new ArrayList<>();
    for (String key : new TreeSet<>(bimap.keySet())) {
      Map<String, String> entry = new LinkedHashMap<>();
      entry.put("key", key);
      entry.put("value", bimap.get(key));
      entries.add(entry);
    }
    Map<String, Object> finalState = new LinkedHashMap<>();
    finalState.put("size", bimap.size());
    finalState.put("entries", entries);
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    result.put("results", results);
    result.put("final", finalState);
    String encoded = new ObjectMapper().writeValueAsString(result) + "\n";
    Files.write(new File(arguments[1]).toPath(), encoded.getBytes(StandardCharsets.UTF_8));
  }

  public static final class Trace {
    public int version;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String key;
    public String value;
  }
}

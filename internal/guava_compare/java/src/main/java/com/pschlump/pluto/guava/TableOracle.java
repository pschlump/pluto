/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ArrayTable;
import com.google.common.collect.HashBasedTable;
import com.google.common.collect.Table;
import com.google.common.collect.TreeBasedTable;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/** Executes the Phase 1d trace against Guava's three mutable table variants. */
public final class TableOracle {
  private TableOracle() {}

  public static void main(String[] arguments) throws Exception {
    if (arguments.length != 2) {
      throw new IllegalArgumentException("usage: TableOracle TRACE OUTPUT");
    }
    Trace trace = new ObjectMapper().readValue(new File(arguments[0]), Trace.class);
    Map<String, Object> variants = new LinkedHashMap<>();
    variants.put("hash", execute(trace, HashBasedTable.create(), null));
    variants.put("tree", execute(trace, TreeBasedTable.create(), null));
    variants.put("array", execute(trace, HashBasedTable.create(), ArrayTable.create(
        Arrays.asList("a", "b", "c"), Arrays.asList("x", "y", "z"))));
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("version", 1);
    result.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    result.put("variants", variants);
    String encoded = new ObjectMapper().writeValueAsString(result) + "\n";
    Files.write(new File(arguments[1]).toPath(), encoded.getBytes(StandardCharsets.UTF_8));
  }

  private static Map<String, Object> execute(
      Trace trace, Table<String, String, String> table, Table<String, String, String> universe) {
    List<Object> results = new ArrayList<>();
    for (Operation operation : trace.operations) {
      switch (operation.op) {
        case "put":
          results.add(table.put(operation.row, operation.column, operation.value));
          break;
        case "get":
          results.add(table.get(operation.row, operation.column));
          break;
        case "remove":
          results.add(table.remove(operation.row, operation.column));
          break;
        case "containsRow":
          results.add(universe == null ? table.containsRow(operation.row) : universe.containsRow(operation.row));
          break;
        case "containsColumn":
          results.add(universe == null ? table.containsColumn(operation.column) : universe.containsColumn(operation.column));
          break;
        case "size":
          results.add(table.size());
          break;
        case "row":
          List<Table.Cell<String, String, String>> row = new ArrayList<>();
          for (Map.Entry<String, String> entry : table.row(operation.row).entrySet()) {
            row.add(cell(operation.row, entry.getKey(), entry.getValue()));
          }
          results.add(canonicalCells(row));
          break;
        case "column":
          List<Table.Cell<String, String, String>> column = new ArrayList<>();
          for (Map.Entry<String, String> entry : table.column(operation.column).entrySet()) {
            column.add(cell(entry.getKey(), operation.column, entry.getValue()));
          }
          results.add(canonicalCells(column));
          break;
        default:
          throw new IllegalArgumentException("unknown operation " + operation.op);
      }
    }
    Map<String, Object> finalState = new LinkedHashMap<>();
    finalState.put("size", table.size());
    finalState.put("cells", canonicalCells(table.cellSet()));
    Table<String, String, String> keySource = universe == null ? table : universe;
    finalState.put("rowKeys", new ArrayList<>(new TreeSet<>(keySource.rowKeySet())));
    finalState.put("columnKeys", new ArrayList<>(new TreeSet<>(keySource.columnKeySet())));
    Map<String, Object> variant = new LinkedHashMap<>();
    variant.put("results", results);
    variant.put("final", finalState);
    return variant;
  }

  private static List<Map<String, String>> canonicalCells(
      Collection<Table.Cell<String, String, String>> input) {
    List<Table.Cell<String, String, String>> cells = new ArrayList<>(input);
    cells.sort(Comparator.comparing(Table.Cell<String, String, String>::getRowKey)
        .thenComparing(Table.Cell<String, String, String>::getColumnKey));
    List<Map<String, String>> result = new ArrayList<>();
    for (Table.Cell<String, String, String> cell : cells) {
      Map<String, String> entry = new LinkedHashMap<>();
      entry.put("row", cell.getRowKey());
      entry.put("column", cell.getColumnKey());
      entry.put("value", cell.getValue());
      result.add(entry);
    }
    return result;
  }

  private static Table.Cell<String, String, String> cell(String row, String column, String value) {
    return new Table.Cell<String, String, String>() {
      @Override public String getRowKey() { return row; }
      @Override public String getColumnKey() { return column; }
      @Override public String getValue() { return value; }
    };
  }

  public static final class Trace {
    public int version;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String row;
    public String column;
    public String value;
  }
}

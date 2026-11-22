/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.ArrayTable;
import com.google.common.collect.HashBasedTable;
import com.google.common.collect.Table;
import com.google.common.collect.Tables;
import com.google.common.collect.TreeBasedTable;
import java.io.File;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
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

/** Matched Phase 1d workloads for table/bench_test.go. */
@State(Scope.Benchmark)
@BenchmarkMode(Mode.Throughput)
@OutputTimeUnit(TimeUnit.SECONDS)
@Warmup(iterations = 2, time = 1)
@Measurement(iterations = 3, time = 1)
@Fork(1)
@Threads(1)
public class TableBenchmark {
  private Table<String, String, String> hashTable;
  private Table<String, String, String> treeTable;
  private ArrayTable<String, String, String> arrayTable;
  private Map<String, Map<String, String>> nested;
  private String[] flat;
  private Table<String, String, String> hashFull;
  private Table<String, String, String> treeFull;
  private ArrayTable<String, String, String> arrayFull;
  private Operation[] operations;
  private int columns;
  private int index;

  @Setup(Level.Trial)
  public void setup() throws Exception {
    String path = System.getProperty(
        "tableCorpus", "internal/guava_compare/testdata/table.corpus.json");
    Corpus corpus = new ObjectMapper().readValue(new File(path), Corpus.class);
    columns = corpus.columns;
    List<String> rowKeys = keys("row", corpus.rows);
    List<String> columnKeys = keys("column", corpus.columns);
    hashTable = HashBasedTable.create();
    treeTable = TreeBasedTable.create();
    arrayTable = ArrayTable.create(rowKeys, columnKeys);
    nested = new HashMap<>();
    flat = new String[corpus.rows * corpus.columns];
    hashFull = full(HashBasedTable.create(), rowKeys, columnKeys);
    treeFull = full(TreeBasedTable.create(), rowKeys, columnKeys);
    arrayFull = ArrayTable.create(rowKeys, columnKeys);
    full(arrayFull, rowKeys, columnKeys);
    operations = corpus.operations.toArray(new Operation[0]);
  }

  @Benchmark public int hashTable() { return apply(hashTable, next()); }
  @Benchmark public int treeTable() { return apply(treeTable, next()); }
  @Benchmark public int nestedMap() { return applyNested(next()); }
  @Benchmark public int arrayTable() { return applyArray(next()); }
  @Benchmark public int flatMatrix() { return applyFlat(next()); }

  @Benchmark public int hashTableRow() { return hashFull.row("row-032").size(); }
  @Benchmark public int hashTableColumn() { return hashFull.column("column-032").size(); }
  @Benchmark public int treeTableRow() { return treeFull.row("row-032").size(); }
  @Benchmark public int treeTableColumn() { return treeFull.column("column-032").size(); }
  @Benchmark public int arrayTableRow() { return arrayFull.row("row-032").size(); }
  @Benchmark public int arrayTableColumn() { return arrayFull.column("column-032").size(); }

  @Benchmark public int hashTableTranspose() {
    return HashBasedTable.create(Tables.transpose(hashFull)).size();
  }

  @Benchmark public int treeTableTranspose() {
    return copyTree(Tables.transpose(treeFull)).size();
  }

  @Benchmark public int arrayTableTranspose() {
    ArrayTable<String, String, String> result = ArrayTable.create(
        arrayFull.columnKeyList(), arrayFull.rowKeyList());
    for (Table.Cell<String, String, String> cell : arrayFull.cellSet()) {
      result.put(cell.getColumnKey(), cell.getRowKey(), cell.getValue());
    }
    return result.size();
  }

  private static int apply(Table<String, String, String> table, Operation operation) {
    switch (operation.op) {
      case "put":
        return table.put(operation.row, operation.column, operation.value) == null ? 0 : 1;
      case "remove":
        return table.remove(operation.row, operation.column) == null ? 0 : 1;
      case "get":
        return table.get(operation.row, operation.column) == null ? 0 : 1;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private int applyArray(Operation operation) {
    switch (operation.op) {
      case "put":
        return arrayTable.put(operation.row, operation.column, operation.value) == null ? 0 : 1;
      case "remove":
        return arrayTable.set(operation.rowIndex, operation.columnIndex, null) == null ? 0 : 1;
      case "get":
        return arrayTable.get(operation.row, operation.column) == null ? 0 : 1;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private int applyNested(Operation operation) {
    Map<String, String> row = nested.get(operation.row);
    switch (operation.op) {
      case "put":
        if (row == null) {
          row = new HashMap<>();
          nested.put(operation.row, row);
        }
        return row.put(operation.column, operation.value) == null ? 0 : 1;
      case "remove":
        if (row == null) {
          return 0;
        }
        int result = row.remove(operation.column) == null ? 0 : 1;
        if (row.isEmpty()) {
          nested.remove(operation.row);
        }
        return result;
      case "get":
        return row != null && row.get(operation.column) != null ? 1 : 0;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
  }

  private int applyFlat(Operation operation) {
    int position = operation.rowIndex * columns + operation.columnIndex;
    String old = flat[position];
    switch (operation.op) {
      case "put":
        flat[position] = operation.value;
        break;
      case "remove":
        flat[position] = null;
        break;
      case "get":
        break;
      default:
        throw new IllegalArgumentException("unknown operation " + operation.op);
    }
    return old == null ? 0 : 1;
  }

  private Operation next() {
    return operations[index++ % operations.length];
  }

  private static Table<String, String, String> copyTree(Table<String, String, String> source) {
    Table<String, String, String> result = TreeBasedTable.create();
    for (Table.Cell<String, String, String> cell : source.cellSet()) {
      result.put(cell.getRowKey(), cell.getColumnKey(), cell.getValue());
    }
    return result;
  }

  private static Table<String, String, String> full(
      Table<String, String, String> table, List<String> rows, List<String> columns) {
    for (String row : rows) {
      for (String column : columns) {
        table.put(row, column, "value");
      }
    }
    return table;
  }

  private static List<String> keys(String prefix, int count) {
    List<String> result = new ArrayList<>();
    for (int i = 0; i < count; i++) {
      result.add(String.format("%s-%03d", prefix, i));
    }
    return result;
  }

  public static final class Corpus {
    public int version;
    public long seed;
    public int rows;
    public int columns;
    public String inputDistribution;
    public List<Operation> operations;
  }

  public static final class Operation {
    public String op;
    public String row;
    public String column;
    public String value;
    public int rowIndex;
    public int columnIndex;
  }
}

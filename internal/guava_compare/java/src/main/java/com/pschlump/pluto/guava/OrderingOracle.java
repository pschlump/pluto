package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.collect.Ordering;
import java.io.File;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class OrderingOracle {
  public static final class Item {
    public int key;
    public int id;
    Item(int key, int id) { this.key = key; this.id = id; }
  }

  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: OrderingOracle output.json");
    List<Integer> values = Arrays.asList(4, 1, 7, 1, 9, 3, 5);
    Ordering<Integer> natural = Ordering.natural();
    List<List<Integer>> lexInput = Arrays.asList(
        Arrays.asList(1, 2), Arrays.asList(1), Arrays.asList(1, 2, 0),
        Arrays.asList(), Arrays.asList(0, 9));
    List<Item> items = Arrays.asList(
        new Item(1, 0), new Item(0, 1), new Item(1, 2), new Item(0, 3), new Item(1, 4));
    Ordering<Item> byKey = Ordering.natural().onResultOf((Item item) -> item.key);
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("sortedCopy", natural.sortedCopy(values));
    root.put("reverse", natural.reverse().sortedCopy(values));
    root.put("leastOf", natural.leastOf(values, 3));
    root.put("greatestOf", natural.greatestOf(values, 3));
    root.put("lexicographical", natural.lexicographical().sortedCopy(lexInput));
    root.put("min", natural.min(values));
    root.put("max", natural.max(values));
    root.put("ordered", Arrays.asList(
        natural.isOrdered(Arrays.asList(1, 1, 2)),
        natural.isOrdered(Arrays.asList(2, 1))));
    root.put("strictlyOrdered", Arrays.asList(
        natural.isStrictlyOrdered(Arrays.asList(1, 1, 2)),
        natural.isStrictlyOrdered(Arrays.asList(1, 2, 3))));
    root.put("stableByKey", byKey.sortedCopy(items));
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT)
        .writeValue(new File(args[0]), root);
  }
}

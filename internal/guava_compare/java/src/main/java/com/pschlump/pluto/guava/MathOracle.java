package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.math.LongMath;
import com.google.common.primitives.Ints;
import java.io.File;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class MathOracle {
  private interface Operation { long run(); }

  private static void vector(List<Map<String, Object>> output, String name, long a, long b,
      String mode, Operation operation) {
    Map<String, Object> item = new LinkedHashMap<>();
    item.put("operation", name);
    item.put("a", Long.toString(a));
    item.put("b", Long.toString(b));
    item.put("mode", mode);
    try {
      item.put("result", Long.toString(operation.run()));
      item.put("ok", true);
    } catch (ArithmeticException | IllegalArgumentException exception) {
      item.put("result", "0");
      item.put("ok", false);
    }
    output.add(item);
  }

  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: MathOracle output.json");
    List<Map<String, Object>> vectors = new ArrayList<>();
    long[][] pairs = {
        {Long.MAX_VALUE, 1}, {Long.MIN_VALUE, -1}, {123456789, 98765},
        {-123456789, 98765}, {Long.MAX_VALUE, Long.MAX_VALUE}, {0, -1}};
    for (long[] pair : pairs) {
      long a = pair[0], b = pair[1];
      vector(vectors, "checkedAdd", a, b, "", () -> LongMath.checkedAdd(a, b));
      vector(vectors, "checkedSubtract", a, b, "", () -> LongMath.checkedSubtract(a, b));
      vector(vectors, "checkedMultiply", a, b, "", () -> LongMath.checkedMultiply(a, b));
      vector(vectors, "saturatingAdd", a, b, "", () -> LongMath.saturatedAdd(a, b));
      vector(vectors, "saturatingSubtract", a, b, "", () -> LongMath.saturatedSubtract(a, b));
      vector(vectors, "saturatingMultiply", a, b, "", () -> LongMath.saturatedMultiply(a, b));
    }
    for (long base : new long[] {-4, -2, 2, 10}) {
      for (int exponent : new int[] {0, 1, 3, 4, 10, 63}) {
        vector(vectors, "checkedPow", base, exponent, "", () -> LongMath.checkedPow(base, exponent));
        vector(vectors, "saturatingPow", base, exponent, "", () -> LongMath.saturatedPow(base, exponent));
      }
    }
    RoundingMode[] modes = {
        RoundingMode.FLOOR, RoundingMode.CEILING, RoundingMode.DOWN, RoundingMode.UP,
        RoundingMode.HALF_EVEN, RoundingMode.HALF_UP, RoundingMode.HALF_DOWN,
        RoundingMode.UNNECESSARY};
    long[][] dividends = {{7, 3}, {-7, 3}, {7, -3}, {-7, -3}, {5, 2}, {7, 2}, {6, 3}};
    for (long[] pair : dividends) {
      for (RoundingMode mode : modes) {
        vector(vectors, "divide", pair[0], pair[1], mode.name(),
            () -> LongMath.divide(pair[0], pair[1], mode));
      }
    }
    for (long value : new long[] {1, 2, 3, 4, 5, 6, 10, 31, 32, 100, Long.MAX_VALUE}) {
      for (RoundingMode mode : modes) {
        vector(vectors, "log2", value, 0, mode.name(), () -> LongMath.log2(value, mode));
        vector(vectors, "log10", value, 0, mode.name(), () -> LongMath.log10(value, mode));
        vector(vectors, "sqrt", value, 0, mode.name(), () -> LongMath.sqrt(value, mode));
      }
      vector(vectors, "isPrime", value, 0, "", () -> LongMath.isPrime(value) ? 1 : 0);
    }
    vector(vectors, "isPrime", 3825123056546413051L, 0, "",
        () -> LongMath.isPrime(3825123056546413051L) ? 1 : 0);
    vector(vectors, "mod", -7, 3, "", () -> LongMath.mod(-7, 3));
    vector(vectors, "mod", 7, 3, "", () -> LongMath.mod(7, 3));
    vector(vectors, "gcd", 54, 24, "", () -> LongMath.gcd(54, 24));
    vector(vectors, "factorial", 20, 0, "", () -> LongMath.factorial(20));
    vector(vectors, "binomial", 30, 15, "", () -> LongMath.binomial(30, 15));
    vector(vectors, "mean", Long.MIN_VALUE, Long.MAX_VALUE, "",
        () -> LongMath.mean(Long.MIN_VALUE, Long.MAX_VALUE));
    vector(vectors, "checkedConvert", Integer.MAX_VALUE + 1L, 32, "",
        () -> Ints.checkedCast(Integer.MAX_VALUE + 1L));
    vector(vectors, "saturatingConvert", Integer.MAX_VALUE + 1L, 32, "",
        () -> Ints.saturatedCast(Integer.MAX_VALUE + 1L));
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("vectors", vectors);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT)
        .writeValue(new File(args[0]), root);
  }
}

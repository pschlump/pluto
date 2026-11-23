package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.math.LinearTransformation;
import com.google.common.math.PairedStatsAccumulator;
import com.google.common.math.Quantiles;
import com.google.common.math.Stats;
import com.google.common.math.StatsAccumulator;
import java.io.File;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class StatsOracle {
  private static Map<String, Object> stats(Stats stats) {
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("count", stats.count());
    result.put("sum", Double.toString(stats.sum()));
    result.put("mean", Double.toString(stats.mean()));
    result.put("min", Double.toString(stats.min()));
    result.put("max", Double.toString(stats.max()));
    result.put("populationVariance", Double.toString(stats.populationVariance()));
    result.put("sampleVariance", Double.toString(stats.sampleVariance()));
    result.put("populationStandardDeviation", Double.toString(stats.populationStandardDeviation()));
    result.put("sampleStandardDeviation", Double.toString(stats.sampleStandardDeviation()));
    return result;
  }

  private static List<String> strings(Iterable<Double> values) {
    List<String> result = new ArrayList<>();
    for (double value : values) result.add(Double.toString(value));
    return result;
  }

  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: StatsOracle output.json");
    StatsAccumulator basic = new StatsAccumulator();
    basic.addAll(1, 2, 3, 4, 5);
    StatsAccumulator left = new StatsAccumulator();
    StatsAccumulator right = new StatsAccumulator();
    StatsAccumulator onePass = new StatsAccumulator();
    for (int i = 0; i < 1000; i++) {
      double value = 1e12 + (i * 7919) % 101;
      onePass.add(value);
      (i % 2 == 0 ? left : right).add(value);
    }
    left.addAll(right.snapshot());

    PairedStatsAccumulator paired = new PairedStatsAccumulator();
    for (int x = 0; x < 10; x++) paired.add(x, 3 * x + 2);
    LinearTransformation fit = paired.leastSquaresFit();
    Map<String, Object> pairedResult = new LinkedHashMap<>();
    pairedResult.put("correlation", Double.toString(paired.pearsonsCorrelationCoefficient()));
    pairedResult.put("slope", Double.toString(fit.slope()));
    pairedResult.put("transform4", Double.toString(fit.transform(4)));
    pairedResult.put("inverse14", Double.toString(fit.inverse().transform(14)));

    PairedStatsAccumulator horizontalAccumulator = new PairedStatsAccumulator();
    horizontalAccumulator.add(1, 7);
    horizontalAccumulator.add(2, 7);
    horizontalAccumulator.add(3, 7);
    Map<String, Object> horizontal = new LinkedHashMap<>();
    horizontal.put("isHorizontal", horizontalAccumulator.leastSquaresFit().isHorizontal());
    horizontal.put("transform99", Double.toString(horizontalAccumulator.leastSquaresFit().transform(99)));

    double[] quantileInput = {0, 10, 20, 30};
    int[] indexes = {0, 1, 2, 3, 4, 2};
    Map<String, Object> quantiles = new LinkedHashMap<>();
    quantiles.put("scale", 4);
    quantiles.put("indexes", Arrays.asList(0, 1, 2, 3, 4, 2));
    quantiles.put("values", strings(Quantiles.scale(4).indexes(indexes).compute(quantileInput).values()));

    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("basic", stats(basic.snapshot()));
    root.put("merged", stats(left.snapshot()));
    root.put("onePass", stats(onePass.snapshot()));
    root.put("paired", pairedResult);
    root.put("horizontal", horizontal);
    root.put("quantiles", quantiles);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT)
        .writeValue(new File(args[0]), root);
  }
}

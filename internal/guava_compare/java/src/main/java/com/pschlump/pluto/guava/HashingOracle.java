package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.hash.BloomFilter;
import com.google.common.hash.Funnels;
import com.google.common.hash.HashCode;
import com.google.common.hash.HashFunction;
import com.google.common.hash.Hashing;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class HashingOracle {
  private static Map<String, Object> vector(String algorithm, long seed, byte[] input, HashCode code) {
    Map<String, Object> item = new LinkedHashMap<>();
    item.put("algorithm", algorithm);
    item.put("seed", seed);
    item.put("input", Base64.getEncoder().encodeToString(input));
    item.put("hash", code.toString());
    return item;
  }

  private static byte[] sequence(int length, int bias) {
    byte[] data = new byte[length];
    for (int i = 0; i < length; i++) data[i] = (byte) (i * 131 + bias);
    return data;
  }

  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: HashingOracle output.json");
    List<Map<String, Object>> vectors = new ArrayList<>();
    int[] lengths = {0, 1, 2, 3, 4, 7, 8, 15, 16, 17, 31, 32, 33, 63, 64, 65, 127, 128, 129, 255};
    List<byte[]> inputs = new ArrayList<>();
    for (int length : lengths) inputs.add(sequence(length, length));
    inputs.add(new byte[] {0, 127, (byte) 128, (byte) 255, -1});
    inputs.add("Grüße, 世界 😀".getBytes(StandardCharsets.UTF_8));
    inputs.add("value-0000".getBytes(StandardCharsets.UTF_8));
    for (byte[] input : inputs) {
      vectors.add(vector("murmur3_32", 0, input, Hashing.murmur3_32().hashBytes(input)));
      vectors.add(vector("murmur3_32", 1, input, Hashing.murmur3_32(1).hashBytes(input)));
      vectors.add(vector("murmur3_32", 4294967295L, input, Hashing.murmur3_32(-1).hashBytes(input)));
      vectors.add(vector("murmur3_128", 0, input, Hashing.murmur3_128().hashBytes(input)));
      vectors.add(vector("murmur3_128", 1, input, Hashing.murmur3_128(1).hashBytes(input)));
      vectors.add(vector("sipHash24", 0, input, Hashing.sipHash24().hashBytes(input)));
      vectors.add(vector("farmHashFingerprint64", 0, input, Hashing.farmHashFingerprint64().hashBytes(input)));
      vectors.add(vector("fingerprint2011", 0, input, Hashing.fingerprint2011().hashBytes(input)));
    }

    List<Map<String, Object>> combined = new ArrayList<>();
    HashCode a = Hashing.murmur3_128().hashString("a", StandardCharsets.UTF_8);
    HashCode b = Hashing.murmur3_128().hashString("b", StandardCharsets.UTF_8);
    combined.add(vector("ordered", 0, new byte[0], Hashing.combineOrdered(java.util.Arrays.asList(a, b))));
    combined.add(vector("unordered", 0, new byte[0], Hashing.combineUnordered(java.util.Arrays.asList(a, b))));

    List<Integer> buckets = new ArrayList<>();
    for (int n : new int[] {1, 2, 3, 7, 8, 100, 1024}) buckets.add(Hashing.consistentHash(a, n));
    BloomFilter<String> first = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 1000, 0.03);
    first.put("value-0000");
    ByteArrayOutputStream firstOutput = new ByteArrayOutputStream();
    first.writeTo(firstOutput);
    BloomFilter<String> filter = BloomFilter.create(Funnels.stringFunnel(StandardCharsets.UTF_8), 1000, 0.03);
    for (int i = 0; i < 1000; i++) filter.put(String.format("value-%04d", i));
    ByteArrayOutputStream output = new ByteArrayOutputStream();
    filter.writeTo(output);
    Map<String, Object> bloom = new LinkedHashMap<>();
    bloom.put("expectedInsertions", 1000);
    bloom.put("fpp", 0.03);
    bloom.put("guavaEncoding", Base64.getEncoder().encodeToString(output.toByteArray()));
    bloom.put("firstEncoding", Base64.getEncoder().encodeToString(firstOutput.toByteArray()));
    bloom.put("expectedFpp", filter.expectedFpp());
    bloom.put("approximateElementCount", filter.approximateElementCount());

    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("guavaCommit", "36e3995499d6c3b74b1207d362fa9e19915d2589");
    root.put("vectors", vectors);
    root.put("combined", combined);
    root.put("buckets", buckets);
    root.put("bloom", bloom);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT).writeValue(new File(args[0]), root);
  }
}

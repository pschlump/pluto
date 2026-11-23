/*
 * Copyright 2026 Philip Schlump
 * BSD 3-Clause Licensed.
 */
package com.pschlump.pluto.guava;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.google.common.collect.Interner;
import com.google.common.collect.Interners;
import java.io.File;
import java.util.LinkedHashMap;
import java.util.Map;
import org.openjdk.jol.info.GraphLayout;

/** JOL retained-footprint measurement for the Phase 9 strong interner. */
public final class Phase9Memory {
  public static void main(String[] args) throws Exception {
    if (args.length != 1) throw new IllegalArgumentException("usage: Phase9Memory output.json");
    final int count = 10000;
    Interner<String> interner = Interners.newStrongInterner();
    for (int i = 0; i < count; i++) {
      interner.intern(String.format("value-%05d", i));
    }
    long bytes = GraphLayout.parseInstance(interner).totalSize();
    Map<String, Object> root = new LinkedHashMap<>();
    root.put("version", 1);
    root.put("method", "JOL GraphLayout totalSize");
    root.put("values", count);
    root.put("guavaStrongInternerBytes", bytes);
    new ObjectMapper().enable(SerializationFeature.INDENT_OUTPUT)
        .writeValue(new File(args[0]), root);
    if (interner.intern("value-00000") == null) throw new AssertionError();
  }
}

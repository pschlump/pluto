/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCounterGolden(t *testing.T) {
	trace, err := ReadTrace(filepath.Join("testdata", "counter.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := ExecuteTrace(trace)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "counter.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "actual-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := WriteTraceResult(path, actual); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(expected) {
		t.Fatalf("canonical result mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func TestGeneratedFilesAreDeterministic(t *testing.T) {
	trace, err := ReadTrace(filepath.Join("testdata", "counter.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := ReadCorpus(filepath.Join("testdata", "counter.corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if generated := GenerateTrace(trace.Seed, len(trace.Operations)); !reflect.DeepEqual(generated, trace) {
		t.Fatal("committed trace differs from deterministic generator")
	}
	if generated := GenerateCorpus(corpus.Seed, len(corpus.Operations), corpus.Capacity); !reflect.DeepEqual(generated, corpus) {
		t.Fatal("committed corpus differs from deterministic generator")
	}
	multimapCorpus, err := ReadMultimapCorpus(filepath.Join("testdata", "multimap.corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if generated := GenerateMultimapCorpus(multimapCorpus.Seed, len(multimapCorpus.Operations), multimapCorpus.Capacity, multimapCorpus.ValuesPerKey); !reflect.DeepEqual(generated, multimapCorpus) {
		t.Fatal("committed multimap corpus differs from deterministic generator")
	}
}

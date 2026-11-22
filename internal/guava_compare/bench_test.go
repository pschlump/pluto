/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"path/filepath"
	"testing"
)

var benchmarkResult int

func BenchmarkCounter(b *testing.B) {
	corpus, err := ReadCorpus(filepath.Join("testdata", "counter.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	counter := counter{counts: make(map[string]int, corpus.Capacity)}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := counter.apply(corpus.Operations[index%len(corpus.Operations)])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkResult = result
	}
}

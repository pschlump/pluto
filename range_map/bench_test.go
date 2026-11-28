/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_map

import (
	"path/filepath"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/ranges"
)

var benchmarkRangeMapResult int

func BenchmarkRangeMap(b *testing.B) {
	corpus := readRangeMapCorpus(b)
	mapped := NewRangeMap[int, string]()
	// Count into a local and publish once, as the JMH method returns its value:
	// a package-global increment inside the loop is extra work Java never does.
	hits := 0
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "add":
			mapped.Put(ranges.ClosedOpen(operation.Low, operation.High), operation.Value)
		case "remove":
			mapped.Remove(ranges.ClosedOpen(operation.Low, operation.High))
		default:
			if _, found := mapped.Get(operation.Point); found {
				hits++
			}
		}
	}
	benchmarkRangeMapResult = hits
}

func BenchmarkRangeMapIteration(b *testing.B) {
	mapped := NewRangeMap[int, string]()
	for index := 0; index < 1024; index += 2 {
		mapped.Put(ranges.ClosedOpen(index, index+1), "value")
	}
	b.ReportAllocs()
	for b.Loop() {
		count := 0
		for range mapped.Entries() {
			count++
		}
		benchmarkRangeMapResult = count
	}
}

func readRangeMapCorpus(b *testing.B) guava_compare.RangeCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadRangeCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "range.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

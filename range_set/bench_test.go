/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_set

import (
	"path/filepath"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/ranges"
)

var benchmarkRangeSetResult int

func BenchmarkRangeSet(b *testing.B) {
	corpus := readRangeSetCorpus(b)
	set := NewRangeSet[int]()
	// Count into a local and publish once, as the JMH method returns its value:
	// a package-global increment inside the loop is extra work Java never does.
	hits := 0
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "add":
			set.Add(ranges.ClosedOpen(operation.Low, operation.High))
		case "remove":
			set.Remove(ranges.ClosedOpen(operation.Low, operation.High))
		default:
			if set.Contains(operation.Point) {
				hits++
			}
		}
	}
	benchmarkRangeSetResult = hits
}

func BenchmarkRangeSetIteration(b *testing.B) {
	set := NewRangeSet[int]()
	for index := 0; index < 1024; index += 2 {
		set.Add(ranges.ClosedOpen(index, index+1))
	}
	b.ReportAllocs()
	for b.Loop() {
		count := 0
		for range set.All() {
			count++
		}
		benchmarkRangeSetResult = count
	}
}

func readRangeSetCorpus(b *testing.B) guava_compare.RangeCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadRangeCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "range.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

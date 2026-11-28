/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pschlump/pluto/bimap"
	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/multimap"
)

var immutableBenchmarkResult any
var immutableBenchmarkInt int

func BenchmarkImmutableListBuild(b *testing.B) {
	corpus := readImmutableCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = NewList(corpus.Values)
	}
}

func BenchmarkImmutableListCopyOf(b *testing.B) {
	list := NewList(readImmutableCorpus(b).Values)
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = CopyOfList(list)
	}
}

func BenchmarkImmutableListAt(b *testing.B) {
	corpus := readImmutableCorpus(b)
	list := NewList(corpus.Values)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		immutableBenchmarkInt, _ = list.At(index % corpus.Size)
	}
}

func BenchmarkImmutableListIteration(b *testing.B) {
	list := NewList(readImmutableCorpus(b).Values)
	b.ReportAllocs()
	// A plain b.N loop, not b.Loop: inside a b.Loop body the compiler keeps
	// every assignment alive and stops inlining the range-over-func body, so
	// each element pays an indirect yield call plus a memory round trip that
	// JMH's JIT (and ordinary Go code) does not. Accumulate locally and publish
	// once per pass, as the JMH method does (int result = 0; ... return result).
	for range b.N {
		result := 0
		for value := range list.All() {
			result += value
		}
		immutableBenchmarkInt = result
	}
}

func BenchmarkImmutableSetBuild(b *testing.B) {
	corpus := readImmutableCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = NewSet(corpus.Values)
	}
}

func BenchmarkImmutableSetContains(b *testing.B) {
	corpus := readImmutableCorpus(b)
	set := NewSet(corpus.Values)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if set.Contains(corpus.Values[index%corpus.Size]) {
			immutableBenchmarkInt++
		}
	}
}

func BenchmarkImmutableMapBuild(b *testing.B) {
	entries := immutableMapEntries(readImmutableCorpus(b))
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult, _ = NewMap(entries)
	}
}

func BenchmarkImmutableMapGet(b *testing.B) {
	corpus := readImmutableCorpus(b)
	value, err := NewMap(immutableMapEntries(corpus))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		immutableBenchmarkInt, _ = value.Get(corpus.Keys[index%corpus.Size])
	}
}

func BenchmarkImmutableSortedSetBuild(b *testing.B) {
	corpus := readImmutableCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = NewSortedSet(corpus.Values)
	}
}

func BenchmarkImmutableSortedSetContains(b *testing.B) {
	corpus := readImmutableCorpus(b)
	set := NewSortedSet(corpus.Values)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if set.Contains(corpus.Values[index%corpus.Size]) {
			immutableBenchmarkInt++
		}
	}
}

func BenchmarkImmutableSortedSetSubRange(b *testing.B) {
	corpus := readImmutableCorpus(b)
	set := NewSortedSet(corpus.Values)
	low, high := corpus.Size/4, corpus.Size*3/4
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkInt = set.SubRange(low, true, high, true).Len()
	}
}

func BenchmarkImmutableSortedMapBuild(b *testing.B) {
	entries := immutableMapEntries(readImmutableCorpus(b))
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult, _ = NewSortedMap(entries)
	}
}

func BenchmarkImmutableSortedMapGet(b *testing.B) {
	corpus := readImmutableCorpus(b)
	value, err := NewSortedMap(immutableMapEntries(corpus))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		immutableBenchmarkInt, _ = value.Get(corpus.Keys[index%corpus.Size])
	}
}

func BenchmarkImmutableMultisetBuild(b *testing.B) {
	corpus := readImmutableCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = NewMultiset(corpus.Values)
	}
}

func BenchmarkImmutableMultisetCount(b *testing.B) {
	corpus := readImmutableCorpus(b)
	value := NewMultiset(corpus.Values)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		immutableBenchmarkInt = value.Count(corpus.Values[index%corpus.Size])
	}
}

func BenchmarkImmutableMultimapBuild(b *testing.B) {
	entries := immutableMultimapEntries(readImmutableCorpus(b))
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult = NewMultimap(entries)
	}
}

func BenchmarkImmutableMultimapGet(b *testing.B) {
	corpus := readImmutableCorpus(b)
	value := NewMultimap(immutableMultimapEntries(corpus))
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		values, _ := value.Get(corpus.Keys[index%64])
		immutableBenchmarkInt = len(values)
	}
}

func BenchmarkImmutableBiMapBuild(b *testing.B) {
	entries := immutableBiMapEntries(readImmutableCorpus(b))
	b.ReportAllocs()
	for b.Loop() {
		immutableBenchmarkResult, _ = NewBiMap(entries)
	}
}

func BenchmarkImmutableBiMapGet(b *testing.B) {
	corpus := readImmutableCorpus(b)
	value, err := NewBiMap(immutableBiMapEntries(corpus))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		immutableBenchmarkInt, _ = value.GetValue(corpus.Keys[index%corpus.Size])
	}
}

func BenchmarkImmutableMemory(b *testing.B) {
	corpus := readImmutableCorpus(b)
	mapEntries := immutableMapEntries(corpus)
	multimapEntries := immutableMultimapEntries(corpus)
	bimapEntries := immutableBiMapEntries(corpus)
	for _, name := range []string{"list", "set", "map", "sortedSet", "sortedMap", "multiset", "multimap", "bimap"} {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				runtime.GC()
				var base runtime.MemStats
				runtime.ReadMemStats(&base)
				var value any
				switch name {
				case "list":
					value = NewList(corpus.Values)
				case "set":
					value = NewSet(corpus.Values)
				case "map":
					value, _ = NewMap(mapEntries)
				case "sortedSet":
					value = NewSortedSet(corpus.Values)
				case "sortedMap":
					value, _ = NewSortedMap(mapEntries)
				case "multiset":
					value = NewMultiset(corpus.Values)
				case "multimap":
					value = NewMultimap(multimapEntries)
				case "bimap":
					value, _ = NewBiMap(bimapEntries)
				}
				runtime.GC()
				var populated runtime.MemStats
				runtime.ReadMemStats(&populated)
				overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
				b.ReportMetric(float64(overhead), "retained-bytes")
				b.ReportMetric(float64(overhead)/float64(corpus.Size), "retained-bytes/input")
				runtime.KeepAlive(value)
			}
		})
	}
}

func immutableMapEntries(corpus guava_compare.ImmutableCorpus) []Entry[string, int] {
	entries := make([]Entry[string, int], corpus.Size)
	for index, key := range corpus.Keys {
		entries[index] = Entry[string, int]{Key: key, Value: corpus.Values[index]}
	}
	return entries
}

func immutableMultimapEntries(corpus guava_compare.ImmutableCorpus) []multimap.Association[string, int] {
	entries := make([]multimap.Association[string, int], corpus.Size)
	for index := range entries {
		entries[index] = multimap.Association[string, int]{Key: corpus.Keys[index%64], Value: corpus.Values[index]}
	}
	return entries
}

func immutableBiMapEntries(corpus guava_compare.ImmutableCorpus) []bimap.Association[string, int] {
	entries := make([]bimap.Association[string, int], corpus.Size)
	for index, key := range corpus.Keys {
		entries[index] = bimap.Association[string, int]{Key: key, Value: index}
	}
	return entries
}

func readImmutableCorpus(b *testing.B) guava_compare.ImmutableCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadImmutableCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "immutable.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

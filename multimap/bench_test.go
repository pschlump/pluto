/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"path/filepath"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

var multimapBenchmarkResult int

type benchmarkMultimap interface {
	Put(string, string) bool
	PutAll(string, []string) int
	Get(string) ([]string, bool)
	Remove(string, string) bool
	ContainsEntry(string, string) bool
}

func BenchmarkListMultimap(b *testing.B) {
	benchmarkPlutoMultimap(b, NewListMultimap[string, string]())
}

func BenchmarkSetMultimap(b *testing.B) {
	benchmarkPlutoMultimap(b, NewSetMultimap[string, string]())
}

func BenchmarkSortedSetMultimap(b *testing.B) {
	benchmarkPlutoMultimap(b, NewSortedSetMultimap[string, string]())
}

func benchmarkPlutoMultimap(b *testing.B, set benchmarkMultimap) {
	corpus := readMultimapBenchmarkCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		multimapBenchmarkResult = applyMultimapBenchmark(set, corpus.Operations[index%len(corpus.Operations)])
	}
}

func applyMultimapBenchmark(set benchmarkMultimap, operation guava_compare.MultimapOperation) int {
	switch operation.Op {
	case "put":
		if set.Put(operation.Key, operation.Value) {
			return 1
		}
	case "putAll":
		return set.PutAll(operation.Key, operation.Values)
	case "get":
		values, _ := set.Get(operation.Key)
		return len(values)
	case "remove":
		if set.Remove(operation.Key, operation.Value) {
			return 1
		}
	case "containsEntry":
		if set.ContainsEntry(operation.Key, operation.Value) {
			return 1
		}
	}
	return 0
}

func BenchmarkListMap(b *testing.B) {
	corpus := readMultimapBenchmarkCorpus(b)
	values := make(map[string][]string, corpus.Capacity)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "put":
			values[operation.Key] = append(values[operation.Key], operation.Value)
			multimapBenchmarkResult = 1
		case "putAll":
			values[operation.Key] = append(values[operation.Key], operation.Values...)
			multimapBenchmarkResult = len(operation.Values)
		case "get":
			multimapBenchmarkResult = len(values[operation.Key])
		case "remove":
			multimapBenchmarkResult = 0
			for valueIndex, value := range values[operation.Key] {
				if value == operation.Value {
					items := values[operation.Key]
					items = append(items[:valueIndex], items[valueIndex+1:]...)
					if len(items) == 0 {
						delete(values, operation.Key)
					} else {
						values[operation.Key] = items
					}
					multimapBenchmarkResult = 1
					break
				}
			}
		case "containsEntry":
			multimapBenchmarkResult = 0
			for _, value := range values[operation.Key] {
				if value == operation.Value {
					multimapBenchmarkResult = 1
					break
				}
			}
		}
	}
}

func BenchmarkSetMap(b *testing.B) {
	corpus := readMultimapBenchmarkCorpus(b)
	values := make(map[string]map[string]struct{}, corpus.Capacity)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "put", "putAll":
			if values[operation.Key] == nil {
				values[operation.Key] = make(map[string]struct{})
			}
			items := []string{operation.Value}
			if operation.Op == "putAll" {
				items = operation.Values
			}
			multimapBenchmarkResult = 0
			for _, value := range items {
				if _, found := values[operation.Key][value]; !found {
					values[operation.Key][value] = struct{}{}
					multimapBenchmarkResult++
				}
			}
		case "get":
			multimapBenchmarkResult = len(values[operation.Key])
		case "remove":
			if _, found := values[operation.Key][operation.Value]; found {
				delete(values[operation.Key], operation.Value)
				if len(values[operation.Key]) == 0 {
					delete(values, operation.Key)
				}
				multimapBenchmarkResult = 1
			} else {
				multimapBenchmarkResult = 0
			}
		case "containsEntry":
			_, found := values[operation.Key][operation.Value]
			if found {
				multimapBenchmarkResult = 1
			} else {
				multimapBenchmarkResult = 0
			}
		}
	}
}

func readMultimapBenchmarkCorpus(b *testing.B) guava_compare.MultimapCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadMultimapCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "multimap.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

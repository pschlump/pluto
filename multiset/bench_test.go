/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

var benchmarkResult int

func BenchmarkHashMultiset(b *testing.B) {
	corpus := benchmarkCorpus(b)
	set := NewHashMultiset[string]()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		applyHashBenchmark(set, operation)
	}
}

func BenchmarkTreeMultiset(b *testing.B) {
	corpus := benchmarkCorpus(b)
	set := NewTreeMultiset[string]()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		applyTreeBenchmark(set, operation)
	}
}

func BenchmarkMapMultiset(b *testing.B) {
	corpus := benchmarkCorpus(b)
	counts := make(map[string]int, corpus.Capacity)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		applyMapBenchmark(counts, operation)
	}
}

func benchmarkCorpus(b *testing.B) guava_compare.Corpus {
	b.Helper()
	corpus, err := guava_compare.ReadCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "counter.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

func applyHashBenchmark(set *HashMultiset[string], operation guava_compare.Operation) {
	switch operation.Op {
	case "add":
		set.Add(operation.Value, operation.Count)
	case "remove":
		set.Remove(operation.Value, operation.Count)
	case "setCount":
		set.SetCount(operation.Value, operation.Count)
	}
	benchmarkResult = set.Count(operation.Value)
}

func applyTreeBenchmark(set *TreeMultiset[string], operation guava_compare.Operation) {
	switch operation.Op {
	case "add":
		set.Add(operation.Value, operation.Count)
	case "remove":
		set.Remove(operation.Value, operation.Count)
	case "setCount":
		set.SetCount(operation.Value, operation.Count)
	}
	benchmarkResult = set.Count(operation.Value)
}

func applyMapBenchmark(counts map[string]int, operation guava_compare.Operation) {
	switch operation.Op {
	case "add":
		counts[operation.Value] += operation.Count
	case "remove":
		count := counts[operation.Value] - operation.Count
		if count <= 0 {
			delete(counts, operation.Value)
		} else {
			counts[operation.Value] = count
		}
	case "setCount":
		if operation.Count == 0 {
			delete(counts, operation.Value)
		} else {
			counts[operation.Value] = operation.Count
		}
	}
	benchmarkResult = counts[operation.Value]
}

func BenchmarkMultisetCount(b *testing.B) {
	for _, miss := range []bool{false, true} {
		name := "hit"
		probe := 0
		if miss {
			name = "miss"
			probe = 10000
		}
		b.Run(name, func(b *testing.B) {
			hash := NewHashMultiset[int]()
			tree := NewTreeMultiset[int]()
			counts := make(map[int]int)
			for value := 0; value < 4096; value++ {
				hash.Add(value, 2)
				tree.Add(value, 2)
				counts[value] = 2
			}
			b.Run("hash", func(b *testing.B) {
				for range b.N {
					benchmarkResult = hash.Count(probe)
				}
			})
			b.Run("tree", func(b *testing.B) {
				for range b.N {
					benchmarkResult = tree.Count(probe)
				}
			})
			b.Run("map", func(b *testing.B) {
				for range b.N {
					benchmarkResult = counts[probe]
				}
			})
		})
	}
}

func BenchmarkMultisetIteration(b *testing.B) {
	for _, occurrences := range []int{1, 4, 16} {
		b.Run("duplicate-"+strconv.Itoa(occurrences), func(b *testing.B) {
			hash := NewHashMultiset[int]()
			tree := NewTreeMultiset[int]()
			for value := 0; value < 1024; value++ {
				hash.Add(value, occurrences)
				tree.Add(value, occurrences)
			}
			b.Run("entries/hash", func(b *testing.B) {
				for range b.N {
					for entry := range hash.Entries() {
						benchmarkResult += entry.Count
					}
				}
			})
			b.Run("entries/tree", func(b *testing.B) {
				for range b.N {
					for entry := range tree.Entries() {
						benchmarkResult += entry.Count
					}
				}
			})
			b.Run("occurrences/hash", func(b *testing.B) {
				for range b.N {
					for range hash.All() {
						benchmarkResult++
					}
				}
			})
			b.Run("occurrences/tree", func(b *testing.B) {
				for range b.N {
					for range tree.All() {
						benchmarkResult++
					}
				}
			})
		})
	}
}

func BenchmarkMultisetAlgebra(b *testing.B) {
	for _, occurrences := range []int{1, 4, 16} {
		b.Run("duplicate-"+strconv.Itoa(occurrences), func(b *testing.B) {
			hashA, hashB := NewHashMultiset[int](), NewHashMultiset[int]()
			treeA, treeB := NewTreeMultiset[int](), NewTreeMultiset[int]()
			for value := 0; value < 1024; value++ {
				hashA.Add(value, occurrences)
				hashB.Add(value/2, occurrences)
				treeA.Add(value, occurrences)
				treeB.Add(value/2, occurrences)
			}
			b.Run("sum/hash", func(b *testing.B) {
				for range b.N {
					var result HashMultiset[int]
					result.Sum(hashA, hashB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("sum/tree", func(b *testing.B) {
				for range b.N {
					var result TreeMultiset[int]
					result.Sum(treeA, treeB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("intersection/hash", func(b *testing.B) {
				for range b.N {
					var result HashMultiset[int]
					result.Intersection(hashA, hashB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("intersection/tree", func(b *testing.B) {
				for range b.N {
					var result TreeMultiset[int]
					result.Intersection(treeA, treeB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("union/hash", func(b *testing.B) {
				for range b.N {
					var result HashMultiset[int]
					result.Union(hashA, hashB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("union/tree", func(b *testing.B) {
				for range b.N {
					var result TreeMultiset[int]
					result.Union(treeA, treeB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("minus/hash", func(b *testing.B) {
				for range b.N {
					var result HashMultiset[int]
					result.Minus(hashA, hashB)
					benchmarkResult = result.Len()
				}
			})
			b.Run("minus/tree", func(b *testing.B) {
				for range b.N {
					var result TreeMultiset[int]
					result.Minus(treeA, treeB)
					benchmarkResult = result.Len()
				}
			})
		})
	}
}

func BenchmarkMultisetMemory(b *testing.B) {
	for _, occurrences := range []int{1, 4, 16} {
		b.Run("duplicate-"+strconv.Itoa(occurrences), func(b *testing.B) {
			for _, implementation := range []string{"hash", "tree", "map"} {
				b.Run(implementation, func(b *testing.B) {
					for range b.N {
						runtime.GC()
						var base runtime.MemStats
						runtime.ReadMemStats(&base)
						var hash *HashMultiset[int]
						var tree *TreeMultiset[int]
						var counts map[int]int
						switch implementation {
						case "hash":
							hash = NewHashMultiset[int]()
						case "tree":
							tree = NewTreeMultiset[int]()
						case "map":
							counts = make(map[int]int)
						}
						for value := 0; value < 4096; value++ {
							switch implementation {
							case "hash":
								hash.Add(value, occurrences)
							case "tree":
								tree.Add(value, occurrences)
							case "map":
								counts[value] = occurrences
							}
						}
						runtime.GC()
						var populated runtime.MemStats
						runtime.ReadMemStats(&populated)
						overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
						b.ReportMetric(float64(overhead), "retained-bytes")
						b.ReportMetric(float64(overhead)/(4096*float64(occurrences)), "retained-bytes/occurrence")
						runtime.KeepAlive(hash)
						runtime.KeepAlive(tree)
						runtime.KeepAlive(counts)
					}
				})
			}
		})
	}
}

func BenchmarkLinkedHashMultiset(b *testing.B) {
	corpus := benchmarkCorpus(b)
	set := NewLinkedHashMultiset[string]()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "add":
			set.Add(operation.Value, operation.Count)
		case "remove":
			set.Remove(operation.Value, operation.Count)
		case "setCount":
			set.SetCount(operation.Value, operation.Count)
		}
		benchmarkResult = set.Count(operation.Value)
	}
}

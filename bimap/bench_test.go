/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

var bimapBenchmarkResult int

func BenchmarkHashBiMap(b *testing.B) {
	corpus := readBimapCorpus(b)
	set := NewHashBiMap[string, string]()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "put":
			if set.Put(operation.Key, operation.Value) == nil {
				bimapBenchmarkResult = 1
			} else {
				bimapBenchmarkResult = 0
			}
		case "forcePut":
			set.ForcePut(operation.Key, operation.Value)
			bimapBenchmarkResult = 1
		case "getValue":
			_, found := set.GetValue(operation.Key)
			bimapBenchmarkResult = boolResult(found)
		case "getKey":
			_, found := set.GetKey(operation.Value)
			bimapBenchmarkResult = boolResult(found)
		case "deleteKey":
			_, found := set.DeleteKey(operation.Key)
			bimapBenchmarkResult = boolResult(found)
		case "deleteValue":
			_, found := set.DeleteValue(operation.Value)
			bimapBenchmarkResult = boolResult(found)
		}
	}
}

func BenchmarkMapBiMap(b *testing.B) {
	corpus := readBimapCorpus(b)
	forward := make(map[string]string, corpus.Capacity)
	reverse := make(map[string]string, corpus.Capacity)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		switch operation.Op {
		case "put":
			oldKey, valueFound := reverse[operation.Value]
			if valueFound && oldKey != operation.Key {
				bimapBenchmarkResult = 0
				break
			}
			if oldValue, found := forward[operation.Key]; found && oldValue != operation.Value {
				delete(reverse, oldValue)
			}
			forward[operation.Key] = operation.Value
			reverse[operation.Value] = operation.Key
			bimapBenchmarkResult = 1
		case "forcePut":
			if oldKey, found := reverse[operation.Value]; found && oldKey != operation.Key {
				delete(forward, oldKey)
			}
			if oldValue, found := forward[operation.Key]; found && oldValue != operation.Value {
				delete(reverse, oldValue)
			}
			forward[operation.Key] = operation.Value
			reverse[operation.Value] = operation.Key
			bimapBenchmarkResult = 1
		case "getValue":
			_, found := forward[operation.Key]
			bimapBenchmarkResult = boolResult(found)
		case "getKey":
			_, found := reverse[operation.Value]
			bimapBenchmarkResult = boolResult(found)
		case "deleteKey":
			value, found := forward[operation.Key]
			if found {
				delete(forward, operation.Key)
				delete(reverse, value)
			}
			bimapBenchmarkResult = boolResult(found)
		case "deleteValue":
			key, found := reverse[operation.Value]
			if found {
				delete(reverse, operation.Value)
				delete(forward, key)
			}
			bimapBenchmarkResult = boolResult(found)
		}
	}
}

func BenchmarkBiMapInverseIteration(b *testing.B) {
	set := NewHashBiMap[string, string]()
	for index := 0; index < 4096; index++ {
		set.ForcePut(fmt.Sprintf("key-%04d", index), fmt.Sprintf("value-%04d", index))
	}
	inverse := set.Inverse()
	b.ResetTimer()
	for range b.N {
		for value, key := range inverse.Entries() {
			bimapBenchmarkResult = len(value) + len(key)
		}
	}
}

func BenchmarkBiMapMemory(b *testing.B) {
	for _, implementation := range []string{"bimap", "maps"} {
		b.Run(implementation, func(b *testing.B) {
			for range b.N {
				runtime.GC()
				var base runtime.MemStats
				runtime.ReadMemStats(&base)
				set := NewHashBiMap[string, string]()
				forward := make(map[string]string)
				reverse := make(map[string]string)
				for index := 0; index < 4096; index++ {
					key, value := fmt.Sprintf("key-%04d", index), fmt.Sprintf("value-%04d", index)
					if implementation == "bimap" {
						set.ForcePut(key, value)
					} else {
						forward[key] = value
						reverse[value] = key
					}
				}
				runtime.GC()
				var populated runtime.MemStats
				runtime.ReadMemStats(&populated)
				overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
				b.ReportMetric(float64(overhead), "retained-bytes")
				b.ReportMetric(float64(overhead)/4096, "retained-bytes/association")
				runtime.KeepAlive(set)
				runtime.KeepAlive(forward)
				runtime.KeepAlive(reverse)
			}
		})
	}
}

func boolResult(found bool) int {
	if found {
		return 1
	}
	return 0
}

func readBimapCorpus(b *testing.B) guava_compare.MultimapCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadMultimapCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "bimap.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

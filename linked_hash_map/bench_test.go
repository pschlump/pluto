/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map

import (
	"fmt"
	"testing"
)

func BenchmarkMapPutGet(b *testing.B) {
	m := New[int, int]()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		key := index % 10000
		m.Put(key, index)
		m.Get(key)
	}
}

func BenchmarkBuiltinMapPutGet(b *testing.B) {
	m := make(map[int]int)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		key := index % 10000
		m[key] = index
		_ = m[key]
	}
}

func BenchmarkMapIteration(b *testing.B) {
	m := New[int, int]()
	for index := range 10000 {
		m.Put(index, index)
	}
	b.ReportAllocs()
	b.ResetTimer()
	sum := 0
	for range b.N {
		for _, value := range m.All() {
			sum += value
		}
	}
	linkedMapBenchmarkResult = sum
}

var linkedMapBenchmarkValues = func() []string {
	values := make([]string, 10000)
	for index := range values {
		values[index] = fmt.Sprintf("value-%05d", index)
	}
	return values
}()

func BenchmarkLinkedHashMap(b *testing.B) {
	m := New[string, int]()
	b.ReportAllocs()
	b.ResetTimer()
	// JMH consumes each returned value through its Blackhole; Go accumulates
	// into a local and publishes once, instead of storing to a package global on
	// every operation.
	sum := 0
	for index := 0; index < b.N; index++ {
		slot := index % len(linkedMapBenchmarkValues)
		value := linkedMapBenchmarkValues[slot]
		m.Put(value, slot)
		got, _ := m.Get(value)
		sum += got
	}
	linkedMapBenchmarkResult = sum
}

var linkedMapBenchmarkResult int

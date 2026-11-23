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
	b.ResetTimer()
	for range b.N {
		sum := 0
		for _, value := range m.All() {
			sum += value
		}
		_ = sum
	}
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
	for index := 0; index < b.N; index++ {
		slot := index % len(linkedMapBenchmarkValues)
		value := linkedMapBenchmarkValues[slot]
		m.Put(value, slot)
		linkedMapBenchmarkResult = m.mustGet(value)
	}
}

var linkedMapBenchmarkResult int

func (m *Map[K, V]) mustGet(key K) V {
	value, _ := m.Get(key)
	return value
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set

import "testing"

var benchmarkRangeResult int

func benchmarkRangeSet() *TreeSet[int] {
	values := make([]int, 4096)
	for index := range values {
		values[index] = index
	}
	return FromSlice(values)
}

func BenchmarkTreeSetRangePositioned(b *testing.B) {
	set := benchmarkRangeSet()
	b.ReportAllocs()
	for b.Loop() {
		for value := range set.Range(1024, 1035) {
			benchmarkRangeResult += value
		}
	}
}

func BenchmarkTreeSetRangeFiltered(b *testing.B) {
	set := benchmarkRangeSet()
	b.ReportAllocs()
	for b.Loop() {
		for value := range set.All() {
			if value > 1035 {
				break
			}
			if value >= 1024 {
				benchmarkRangeResult += value
			}
		}
	}
}

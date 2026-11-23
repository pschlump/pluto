/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package intern

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var benchmarkValues = func() []string {
	values := make([]string, 10000)
	for index := range values {
		values[index] = fmt.Sprintf("value-%05d", index)
	}
	return values
}()

func BenchmarkInterner(b *testing.B) {
	interner := New[string]()
	for index, value := range benchmarkValues {
		if index < len(benchmarkValues)/2 {
			interner.Intern(value)
		}
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		interner.Intern(strings.Clone(benchmarkValues[index%len(benchmarkValues)]))
	}
}

func BenchmarkMapMutex(b *testing.B) {
	values := make(map[string]string, len(benchmarkValues)/2)
	var mutex sync.RWMutex
	for index, value := range benchmarkValues {
		if index < len(benchmarkValues)/2 {
			values[value] = value
		}
	}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		value := strings.Clone(benchmarkValues[index%len(benchmarkValues)])
		mutex.RLock()
		_, found := values[value]
		mutex.RUnlock()
		if !found {
			mutex.Lock()
			if _, found = values[value]; !found {
				values[value] = value
			}
			mutex.Unlock()
		}
	}
}

func BenchmarkInternerBuild(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		interner := New[string]()
		for _, value := range benchmarkValues {
			interner.Intern(value)
		}
	}
}

func BenchmarkInternerRetained(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		interner := New[string]()
		for _, value := range benchmarkValues {
			interner.Intern(value)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc)/float64(len(benchmarkValues)), "retained-B/value")
		runtime.KeepAlive(interner)
	}
}

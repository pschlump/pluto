package bloom_filter_ts

import (
	"fmt"
	"testing"

	"github.com/pschlump/pluto/hashing"
)

var benchmarkBool bool

// The twin's benchmarks mirror the plain package's Put and membership
// workloads (see bloom_filter/bench_test.go) plus a parallel-reader run.
func BenchmarkBloomFilterPut(b *testing.B) {
	values := make([]string, 1000)
	for i := range values {
		values[i] = fmt.Sprintf("value-%04d", i)
	}
	b.ReportAllocs()
	for b.Loop() {
		filter := New(100_000, 0.01, hashing.StringFunnel)
		for _, value := range values {
			benchmarkBool = filter.Put(value)
		}
	}
}

func BenchmarkBloomFilterMightContain(b *testing.B) {
	filter := New(1000, 0.01, hashing.StringFunnel)
	values := make([]string, 1000)
	absent := make([]string, 1000)
	for i := range 1000 {
		values[i] = fmt.Sprintf("value-%04d", i)
		absent[i] = fmt.Sprintf("absent-%04d", i)
		filter.Put(values[i])
	}
	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			benchmarkBool = filter.MightContain(values[i%1000])
		}
	})
	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; b.Loop(); i++ {
			benchmarkBool = filter.MightContain(absent[i%1000])
		}
	})
	b.Run("parallel-hit", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				if !filter.MightContain(values[i%1000]) {
					b.Error("false negative")
					return
				}
			}
		})
	})
}

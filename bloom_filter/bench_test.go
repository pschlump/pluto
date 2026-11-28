package bloom_filter

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/pschlump/pluto/hashing"
)

var benchmarkBool bool
var benchmarkBytes []byte

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
}

func BenchmarkBloomFilterMerge(b *testing.B) {
	left := New(1000, 0.01, hashing.StringFunnel)
	right := New(1000, 0.01, hashing.StringFunnel)
	for i := range 1000 {
		left.Put(fmt.Sprintf("left-%d", i))
		right.Put(fmt.Sprintf("right-%d", i))
	}
	b.ReportAllocs()
	for b.Loop() {
		copyFilter := left.Copy()
		if err := copyFilter.Merge(right); err != nil {
			b.Fatal(err)
		}
		// Guava's bloomFilterMerge returns copy.mightContain(values[0]).
		benchmarkBool = copyFilter.MightContain("value-0000")
	}
}

func BenchmarkBloomFilterSerialization(b *testing.B) {
	filter := New(1000, 0.01, hashing.StringFunnel)
	for i := range 1000 {
		filter.Put(fmt.Sprintf("value-%04d", i))
	}
	b.ReportAllocs()
	for b.Loop() {
		var output bytes.Buffer
		if err := filter.WriteGuavaTo(&output); err != nil {
			b.Fatal(err)
		}
		// Guava's bloomFilterSerialization returns output.toByteArray(), a copy.
		benchmarkBytes = bytes.Clone(output.Bytes())
	}
}

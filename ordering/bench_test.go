package ordering_test

import (
	"slices"
	"testing"

	"github.com/pschlump/pluto/ordering"
)

const (
	orderingBenchmarkSize = 10000
	orderingBenchmarkK    = 32
)

var orderingBenchmarkSink []int

func orderingValues() []int {
	values := make([]int, orderingBenchmarkSize)
	for index := range values {
		values[index] = int((int64(index)*1103515245+12345)&0x7fffffff) % 100000
	}
	return values
}

func BenchmarkOrderingSortedCopy(b *testing.B) {
	values := orderingValues()
	order := ordering.Natural[int]()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		orderingBenchmarkSink = order.SortedCopy(values)
	}
}

func BenchmarkOrderingLeastOf(b *testing.B) {
	values := orderingValues()
	order := ordering.Natural[int]()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		orderingBenchmarkSink = order.LeastOf(slices.Values(values), orderingBenchmarkK)
	}
}

func BenchmarkOrderingGreatestOf(b *testing.B) {
	values := orderingValues()
	order := ordering.Natural[int]()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		orderingBenchmarkSink = order.GreatestOf(slices.Values(values), orderingBenchmarkK)
	}
}

func BenchmarkOrderingFullSort(b *testing.B) {
	values := orderingValues()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result := append([]int(nil), values...)
		slices.Sort(result)
		orderingBenchmarkSink = result
	}
}

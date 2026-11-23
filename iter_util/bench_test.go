package iter_util_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/pschlump/pluto/iter_util"
)

var (
	iterBenchmarkSink   int
	iterBenchmarkValues = orderingValuesForIter()
)

func iterValues() iter.Seq[int] {
	return slices.Values(iterBenchmarkValues)
}

func orderingValuesForIter() []int {
	values := make([]int, 10000)
	for index := range values {
		values[index] = int((int64(index)*1103515245+12345)&0x7fffffff) % 100000
	}
	return values
}

func BenchmarkIterConcat(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for value := range iter_util.Concat(iterValues(), iterValues()) {
			iterBenchmarkSink += value
		}
	}
}

func BenchmarkIterFilter(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for value := range iter_util.Filter(iterValues(), func(value int) bool { return value%3 == 0 }) {
			iterBenchmarkSink += value
		}
	}
}

func BenchmarkIterPartition(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for part := range iter_util.Partition(iterValues(), 32) {
			iterBenchmarkSink += len(part)
		}
	}
}

func BenchmarkIterPeeking(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		cursor := iter_util.NewPeeking(iterValues())
		for {
			value, ok := cursor.Peek()
			if !ok {
				break
			}
			iterBenchmarkSink += value
			cursor.Next()
		}
		cursor.Close()
	}
}

func BenchmarkIterConcatBaseline(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for _, values := range [][]int{iterBenchmarkValues, iterBenchmarkValues} {
			for _, value := range values {
				iterBenchmarkSink += value
			}
		}
	}
}

func BenchmarkIterFilterBaseline(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for _, value := range iterBenchmarkValues {
			if value%3 == 0 {
				iterBenchmarkSink += value
			}
		}
	}
}

func BenchmarkIterPartitionBaseline(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for start := 0; start < len(iterBenchmarkValues); start += 32 {
			part := append([]int(nil), iterBenchmarkValues[start:min(start+32, len(iterBenchmarkValues))]...)
			iterBenchmarkSink += len(part)
		}
	}
}

func BenchmarkIterPeekingBaseline(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		next, stop := iter.Pull(iterValues())
		for {
			value, ok := next()
			if !ok {
				break
			}
			iterBenchmarkSink += value
		}
		stop()
	}
}

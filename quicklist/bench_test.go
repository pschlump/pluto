package quicklist

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Benchmarks and the memory comparison: bytes/element for a 1M-element
// list of small strings in a QuickList vs dll vs dqueue vs a plain
// packed []string (the goal: within 1.5x of the slice, several times
// better than the linked structures), plus end-op and At throughput.

import (
	"runtime"
	"testing"

	"github.com/pschlump/pluto/dll"
	"github.com/pschlump/pluto/dqueue"
)

// heapUsed returns the HeapAlloc delta around build, with a GC on both
// sides, keeping the built structure alive via keep.
func heapUsed(keep []any, build func() any) (uint64, []any) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	x := build()
	runtime.GC()
	runtime.ReadMemStats(&after)
	return after.HeapAlloc - before.HeapAlloc, append(keep, x)
}

func TestMemoryPerElement(t *testing.T) {
	const n = 1_000_000
	const val = "abcdefgh" // 8-byte string; 16-byte header per element
	var keep []any

	var sliceBytes, qlBytes, dllBytes, dqBytes uint64
	sliceBytes, keep = heapUsed(keep, func() any {
		s := make([]string, 0, n)
		for i := 0; i < n; i++ {
			s = append(s, val)
		}
		return s
	})
	qlBytes, keep = heapUsed(keep, func() any {
		q := NewQuickList[string]()
		for i := 0; i < n; i++ {
			q.PushTail(val)
		}
		return q
	})
	dllBytes, keep = heapUsed(keep, func() any {
		l := dll.NewDll[string]()
		for i := 0; i < n; i++ {
			l.Push(val)
		}
		return l
	})
	dqBytes, keep = heapUsed(keep, func() any {
		var d dqueue.Deque[string]
		for i := 0; i < n; i++ {
			d.PushBack(val)
		}
		return d
	})
	runtime.KeepAlive(keep)

	t.Logf("bytes/element for %d small strings:", n)
	t.Logf("  packed []string : %.1f", float64(sliceBytes)/n)
	t.Logf("  quicklist       : %.1f (%.2fx slice)", float64(qlBytes)/n, float64(qlBytes)/float64(sliceBytes))
	t.Logf("  dll             : %.1f (%.2fx slice)", float64(dllBytes)/n, float64(dllBytes)/float64(sliceBytes))
	t.Logf("  dqueue          : %.1f (%.2fx slice)", float64(dqBytes)/n, float64(dqBytes)/float64(sliceBytes))

	if qlBytes > sliceBytes*3/2 {
		t.Fatalf("quicklist uses %.2fx the memory of a packed slice, goal is <= 1.5x",
			float64(qlBytes)/float64(sliceBytes))
	}
	if qlBytes >= dllBytes || qlBytes >= dqBytes {
		t.Fatal("quicklist should beat both linked structures on memory")
	}
}

func BenchmarkPushTail(b *testing.B) {
	q := NewQuickList[int]()
	for i := 0; b.Loop(); i++ {
		q.PushTail(i)
	}
}

func BenchmarkPushHead(b *testing.B) {
	q := NewQuickList[int]()
	for i := 0; b.Loop(); i++ {
		q.PushHead(i)
	}
}

func BenchmarkDllPush(b *testing.B) {
	l := dll.NewDll[int]()
	for i := 0; b.Loop(); i++ {
		l.Push(i)
	}
}

func BenchmarkDqueuePushBack(b *testing.B) {
	var d dqueue.Deque[int]
	for i := 0; b.Loop(); i++ {
		d.PushBack(i)
	}
}

func BenchmarkPopHead(b *testing.B) {
	q := NewQuickList[int]()
	for i := 0; i < b.N; i++ {
		q.PushTail(i)
	}
	b.ResetTimer()
	for b.Loop() {
		q.PopHead()
	}
}

func BenchmarkDqueuePopFront(b *testing.B) {
	var d dqueue.Deque[int]
	for i := 0; i < b.N; i++ {
		d.PushBack(i)
	}
	b.ResetTimer()
	for b.Loop() {
		_, _ = d.PopFront()
	}
}

func BenchmarkAt(b *testing.B) {
	const n = 100_000
	q := NewQuickList[int]()
	for i := 0; i < n; i++ {
		q.PushTail(i)
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.At(i % n)
	}
}

func BenchmarkAtCompressed(b *testing.B) {
	const n = 100_000
	q := NewQuickList(
		WithSegmentFill[string](128),
		WithCompression[string](LZWCodec(), 2, EncodeStringSegment, DecodeStringSegment))
	for i := 0; i < n; i++ {
		q.PushTail("some reasonably repetitive string value for compression")
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.At(i % n)
	}
}

package multiway_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"testing"

	"github.com/pschlump/pluto/heap"
)

// The backlog's explicit ask: benchmark d ∈ {2,4,8,16} (plus the pluto
// binary heap as the d=2 baseline) at heap size 4096, matching
// heap/bench_test.go's shape.

const benchmarkHeapSize = 4096

var benchDs = []int{2, 4, 8, 16}

func BenchmarkPush(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if dh.Len() >= benchmarkHeapSize {
					dh.Truncate()
				}
				dh.Push(i)
			}
		})
	}
}

func BenchmarkPop(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			for i := range benchmarkHeapSize {
				dh.Push(i * 7 % benchmarkHeapSize)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, found := dh.Pop(); !found {
					b.StopTimer()
					for j := range benchmarkHeapSize {
						dh.Push(j * 7 % benchmarkHeapSize)
					}
					b.StartTimer()
				}
			}
		})
	}
}

// BenchmarkPushPop measures the combined insert+extract cycle.
func BenchmarkPushPop(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dh.Push(i)
				if _, found := dh.Pop(); !found {
					b.Fatalf("Pop: unexpectedly empty")
				}
			}
		})
	}
}

// BenchmarkFixDecrease measures the sift-UP path (Fix to a smaller
// value) — the operation a larger d makes cheaper.
func BenchmarkFixDecrease(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			for i := range benchmarkHeapSize {
				dh.Push(i)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// A leaf slot: the decrease-to-minimum rises the full
				// log_d n depth to the root (a uniformly random slot is a
				// leaf on average — O(1) depth — and would measure nothing).
				idx := i%halfOf(benchmarkHeapSize) + halfOf(benchmarkHeapSize)
				old, _ := dh.GetValue(idx)
				dh.Fix(idx, old-benchmarkHeapSize)
			}
		})
	}
}

// BenchmarkFixIncrease measures the sift-DOWN path (Fix to a larger
// value) — the operation a larger d makes dearer (d-1 compares per
// level).  The root is always fixed: the increase-to-maximum sinks the
// full depth.
func BenchmarkFixIncrease(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			for i := range benchmarkHeapSize {
				dh.Push(i)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				old, _ := dh.GetValue(0)
				dh.Fix(0, old+benchmarkHeapSize)
			}
		})
	}
}

// halfOf returns n/2 (n is a power of two in these benchmarks).
func halfOf(n int) int { return n / 2 }

// BenchmarkHeapify measures the bulk-build (AppendHeap + bottom-up
// rebuild) of a scrambled 4096-element batch.
func BenchmarkHeapify(b *testing.B) {
	var batch []int
	for i := range benchmarkHeapSize {
		batch = append(batch, i*7%benchmarkHeapSize)
	}
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dh.Truncate()
				dh.AppendHeap(batch)
				for j := (dh.Len() - 2) / dh.D(); j >= 0; j-- {
					dh.Heapify(dh.Len(), j)
				}
			}
		})
	}
}

// The large-heap variants (2^20 elements, 8 MiB — well past L2) are
// where the d-ary trade-off actually shows: at 4096 elements the heap
// is L1/L2 resident and the sift compares are nearly free, but at a
// million elements every sift level is a cache miss and the shallower
// log_d n up-sift path of a larger d pays for real.

const benchmarkLargeSize = 1 << 20

func BenchmarkFixDecreaseLarge(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			for i := range benchmarkLargeSize {
				dh.Push(i)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx := i%halfOf(benchmarkLargeSize) + halfOf(benchmarkLargeSize) // a leaf slot
				old, _ := dh.GetValue(idx)
				dh.Fix(idx, old-benchmarkLargeSize)
			}
		})
	}
}

func BenchmarkPopLarge(b *testing.B) {
	for _, d := range benchDs {
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			dh := NewDHeap[int](d)
			for i := range benchmarkLargeSize {
				dh.Push(i * 7 % benchmarkLargeSize)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, found := dh.Pop(); !found {
					b.StopTimer()
					for j := range benchmarkLargeSize {
						dh.Push(j * 7 % benchmarkLargeSize)
					}
					b.StartTimer()
				}
			}
		})
	}
}

// The pluto binary-heap baseline (the d=2 reference implementation) at
// the same size, so the README table compares apples to apples.

func BenchmarkBinaryHeapPush(b *testing.B) {
	hp := heap.NewHeap[int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if hp.Len() >= benchmarkHeapSize {
			hp.Truncate()
		}
		hp.Push(i)
	}
}

func BenchmarkBinaryHeapPop(b *testing.B) {
	hp := heap.NewHeap[int]()
	for i := range benchmarkHeapSize {
		hp.Push(i * 7 % benchmarkHeapSize)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found := hp.Pop(); !found {
			b.StopTimer()
			for j := range benchmarkHeapSize {
				hp.Push(j * 7 % benchmarkHeapSize)
			}
			b.StartTimer()
		}
	}
}

func BenchmarkBinaryHeapFixDecrease(b *testing.B) {
	hp := heap.NewHeap[int]()
	for i := range benchmarkHeapSize {
		hp.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i%halfOf(benchmarkHeapSize) + halfOf(benchmarkHeapSize) // a leaf slot
		old, _ := hp.GetValue(idx)
		hp.Fix(idx, old-benchmarkHeapSize)
	}
}

func BenchmarkBinaryHeapFixDecreaseLarge(b *testing.B) {
	hp := heap.NewHeap[int]()
	for i := range benchmarkLargeSize {
		hp.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i%halfOf(benchmarkLargeSize) + halfOf(benchmarkLargeSize) // a leaf slot
		old, _ := hp.GetValue(idx)
		hp.Fix(idx, old-benchmarkLargeSize)
	}
}

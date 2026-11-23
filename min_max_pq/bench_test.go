/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq

import (
	stdheap "container/heap"
	"path/filepath"
	"testing"

	plutoheap "github.com/pschlump/pluto/heap"
	"github.com/pschlump/pluto/internal/guava_compare"
)

var minMaxBenchmarkResult int

func benchmarkMinMaxStream(b *testing.B, oneEnded bool, apply func(guava_compare.SmallOperation) int) {
	b.Helper()
	corpus := readMinMaxCorpus(b)
	operations := corpus.Operations
	if oneEnded {
		operations = corpus.OneEnded
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		minMaxBenchmarkResult = apply(operations[index%len(operations)])
	}
}

func applyMinMax(queue *MinMaxPriorityQueue[int], operation guava_compare.SmallOperation) int {
	switch operation.Op {
	case "push":
		queue.Push(operation.Value)
	case "popMin":
		return minMaxBoolResult(func() bool { _, found := queue.PopMin(); return found }())
	case "popMax":
		return minMaxBoolResult(func() bool { _, found := queue.PopMax(); return found }())
	case "peekMin":
		return minMaxBoolResult(func() bool { _, found := queue.PeekMin(); return found }())
	case "peekMax":
		return minMaxBoolResult(func() bool { _, found := queue.PeekMax(); return found }())
	}
	return 0
}

func BenchmarkMinMaxPriorityQueue(b *testing.B) {
	queue := NewMinMaxPriorityQueue[int]()
	benchmarkMinMaxStream(b, false, func(operation guava_compare.SmallOperation) int {
		return applyMinMax(queue, operation)
	})
}

func BenchmarkBoundedMinMaxPriorityQueue(b *testing.B) {
	corpus := readMinMaxCorpus(b)
	queue := NewBoundedMinMaxPriorityQueue[int](corpus.MaximumSize)
	benchmarkMinMaxStream(b, false, func(operation guava_compare.SmallOperation) int {
		return applyMinMax(queue, operation)
	})
}

func BenchmarkMinMaxOneEnd(b *testing.B) {
	queue := NewMinMaxPriorityQueue[int]()
	benchmarkMinMaxStream(b, true, func(operation guava_compare.SmallOperation) int {
		return applyMinMax(queue, operation)
	})
}

func BenchmarkPriorityQueue(b *testing.B) {
	queue := plutoheap.NewHeap[int]()
	benchmarkMinMaxStream(b, true, func(operation guava_compare.SmallOperation) int {
		switch operation.Op {
		case "push":
			queue.Push(operation.Value)
		case "popMin":
			return minMaxBoolResult(func() bool { _, found := queue.Pop(); return found }())
		case "peekMin":
			return minMaxBoolResult(func() bool { _, found := queue.Peek(); return found }())
		}
		return 0
	})
}

type benchmarkEntry struct {
	value int
	id    int
}

type benchmarkHeap struct {
	entries []benchmarkEntry
	maximum bool
}

func (h benchmarkHeap) Len() int { return len(h.entries) }
func (h benchmarkHeap) Less(i, j int) bool {
	if h.entries[i].value != h.entries[j].value {
		return h.entries[i].value < h.entries[j].value != h.maximum
	}
	return h.entries[i].id < h.entries[j].id
}
func (h benchmarkHeap) Swap(i, j int)   { h.entries[i], h.entries[j] = h.entries[j], h.entries[i] }
func (h *benchmarkHeap) Push(value any) { h.entries = append(h.entries, value.(benchmarkEntry)) }
func (h *benchmarkHeap) Pop() any {
	last := len(h.entries) - 1
	value := h.entries[last]
	h.entries[last] = benchmarkEntry{}
	h.entries = h.entries[:last]
	return value
}

type benchmarkTwoHeap struct {
	minimum   benchmarkHeap
	maximum   benchmarkHeap
	discarded map[int]struct{}
	nextID    int
}

func newBenchmarkTwoHeap() *benchmarkTwoHeap {
	return &benchmarkTwoHeap{maximum: benchmarkHeap{maximum: true}, discarded: make(map[int]struct{})}
}

func (q *benchmarkTwoHeap) offer(value int) {
	entry := benchmarkEntry{value: value, id: q.nextID}
	q.nextID++
	stdheap.Push(&q.minimum, entry)
	stdheap.Push(&q.maximum, entry)
}

func (q *benchmarkTwoHeap) clean(maximum bool) {
	heap := &q.minimum
	if maximum {
		heap = &q.maximum
	}
	for len(heap.entries) > 0 {
		id := heap.entries[0].id
		if _, found := q.discarded[id]; !found {
			return
		}
		stdheap.Pop(heap)
		delete(q.discarded, id)
	}
}

func (q *benchmarkTwoHeap) pop(maximum bool) (int, bool) {
	q.clean(maximum)
	heap := &q.minimum
	if maximum {
		heap = &q.maximum
	}
	if heap.Len() == 0 {
		return 0, false
	}
	entry := stdheap.Pop(heap).(benchmarkEntry)
	q.discarded[entry.id] = struct{}{}
	return entry.value, true
}

func (q *benchmarkTwoHeap) peek(maximum bool) (int, bool) {
	q.clean(maximum)
	heap := &q.minimum
	if maximum {
		heap = &q.maximum
	}
	if heap.Len() == 0 {
		return 0, false
	}
	return heap.entries[0].value, true
}

func BenchmarkTwoHeap(b *testing.B) {
	queue := newBenchmarkTwoHeap()
	benchmarkMinMaxStream(b, false, func(operation guava_compare.SmallOperation) int {
		switch operation.Op {
		case "push":
			queue.offer(operation.Value)
		case "popMin":
			return minMaxBoolResult(func() bool { _, found := queue.pop(false); return found }())
		case "popMax":
			return minMaxBoolResult(func() bool { _, found := queue.pop(true); return found }())
		case "peekMin":
			return minMaxBoolResult(func() bool { _, found := queue.peek(false); return found }())
		case "peekMax":
			return minMaxBoolResult(func() bool { _, found := queue.peek(true); return found }())
		}
		return 0
	})
}

func minMaxBoolResult(found bool) int {
	if found {
		return 1
	}
	return 0
}

func readMinMaxCorpus(b *testing.B) guava_compare.MinMaxCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadMinMaxCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "min_max_pq.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

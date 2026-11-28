/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue

import (
	"path/filepath"
	"testing"

	"github.com/pschlump/pluto/dqueue"
	"github.com/pschlump/pluto/internal/guava_compare"
)

var queueBenchmarkResult int

func benchmarkQueueStream(b *testing.B, apply func(guava_compare.SmallOperation) int) {
	b.Helper()
	corpus := readQueueCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		queueBenchmarkResult = apply(corpus.Operations[index%len(corpus.Operations)])
	}
}

func BenchmarkEvictingQueue(b *testing.B) {
	corpus := readQueueCorpus(b)
	queue := NewQueue[int](corpus.Capacity)
	benchmarkQueueStream(b, func(operation guava_compare.SmallOperation) int {
		switch operation.Op {
		case "push":
			queue.Push(operation.Value)
		case "pushAll":
			queue.PushAll(operation.Values)
		case "pop":
			return queueBoolResult(func() bool { _, found := queue.Pop(); return found }())
		case "peek":
			return queueBoolResult(func() bool { _, found := queue.Peek(); return found }())
		}
		return 0
	})
}

func BenchmarkDQueueEviction(b *testing.B) {
	corpus := readQueueCorpus(b)
	queue := &dqueue.Deque[int]{}
	push := func(value int) {
		if queue.Len() == corpus.Capacity {
			_, _ = queue.PopFront()
		}
		queue.PushBack(value)
	}
	benchmarkQueueStream(b, func(operation guava_compare.SmallOperation) int {
		switch operation.Op {
		case "push":
			push(operation.Value)
		case "pushAll":
			for _, value := range operation.Values {
				push(value)
			}
		case "pop":
			_, err := queue.PopFront()
			return queueBoolResult(err == nil)
		case "peek":
			_, err := queue.PeekFront()
			return queueBoolResult(err == nil)
		}
		return 0
	})
}

func BenchmarkEvictingQueueIteration(b *testing.B) {
	queue := NewQueue[int](guava_compare.EvictingQueueBenchmarkCapacity)
	for index := 0; index < queue.Capacity(); index++ {
		queue.Push(index)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		// Sum into a local and publish once per pass, as the Java benchmark
		// does (local int, returned). Accumulating into the package variable
		// adds a store and reload to every element.
		result := 0
		for value := range queue.All() {
			result += value
		}
		queueBenchmarkResult = result
	}
}

func BenchmarkDQueueIteration(b *testing.B) {
	queue := &dqueue.Deque[int]{}
	for index := 0; index < guava_compare.EvictingQueueBenchmarkCapacity; index++ {
		queue.PushBack(index)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result := 0
		for _, value := range queue.All() {
			result += value
		}
		queueBenchmarkResult = result
	}
}

func queueBoolResult(found bool) int {
	if found {
		return 1
	}
	return 0
}

func readQueueCorpus(b *testing.B) guava_compare.EvictingQueueCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadEvictingQueueCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "evicting_queue.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

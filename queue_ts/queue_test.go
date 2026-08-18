package queue

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestQueue001(t *testing.T) {
	type TestDemo struct {
		S string
	}

	var Que1 Queue[TestDemo]

	if !Que1.IsEmpty() {
		t.Errorf("Expected empty queue after declaration, failed to get one.")
	}

	Que1.Push(TestDemo{S: "hi"})

	if Que1.IsEmpty() {
		t.Errorf("Expected non-empty queue after 1st push, failed to get one.")
	}

	err := Que1.Pop()
	if err != nil {
		t.Errorf("Unexpected empty queue error after 1 pop")
	}
	err = Que1.Pop()
	if err == nil {
		t.Errorf("Unexpected lack of error after pop on empty queue")
	}
	if !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue, got %v", err)
	}

	Que1.Push(TestDemo{S: "hi2"})
	Que1.Push(TestDemo{S: "hi3"})

	got := Que1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Que1.Peek()
	if err != nil {
		t.Errorf("Unexpected error on non-empty queue")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected %s got %s", "hi2", ss.S)
	}

	_ = Que1.Pop()
	ss, err = Que1.Peek()
	if err != nil {
		t.Errorf("Unexpected error on non-empty queue")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	Que1.Truncate()
	if !Que1.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate, failed to get one.")
	}
}

func TestEnqueueDequeue(t *testing.T) {
	var q Queue[int]

	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue on empty Dequeue, got %v", err)
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue on empty Peek, got %v", err)
	}

	// FIFO order check.
	for i := 0; i < 10; i++ {
		q.Enqueue(i)
	}
	if q.Length() != 10 {
		t.Errorf("Expected length of 10 got %d", q.Length())
	}
	for i := 0; i < 10; i++ {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Unexpected error on Dequeue: %s", err)
		}
		if *v != i {
			t.Errorf("Expected %d got %d", i, *v)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all elements")
	}
}

// TestConcurrent runs producers and consumers against the queue in parallel.
// Run with `go test -race`; the race detector validates the locking.
func TestConcurrent(t *testing.T) {
	var q Queue[int]
	const producers = 4
	const perProducer = 250

	var wg sync.WaitGroup

	// Consumers: drain the queue, counting how many items they get.
	var cwg sync.WaitGroup
	var mu sync.Mutex
	consumed := 0
	done := make(chan struct{})
	for i := 0; i < 2; i++ {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			for {
				select {
				case <-done:
					// Drain whatever is left.
					for {
						if _, err := q.Dequeue(); err != nil {
							return
						}
						mu.Lock()
						consumed++
						mu.Unlock()
					}
				default:
					if _, err := q.Dequeue(); err == nil {
						mu.Lock()
						consumed++
						mu.Unlock()
					}
				}
			}
		}()
	}

	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.Enqueue(base*perProducer + i)
				_, _ = q.Peek()
				_ = q.IsEmpty()
				_ = q.Length()
			}
		}(p)
	}
	wg.Wait()
	close(done)
	cwg.Wait()

	if got, expect := consumed, producers*perProducer; got != expect {
		t.Errorf("Expected %d consumed elements got %d", expect, got)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after concurrent test")
	}
}

// TestIteratorsConcurrent verifies that All iterates a consistent snapshot
// even while another goroutine mutates the queue.  Run with -race.
func TestIteratorsConcurrent(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 100; i++ {
		q.Push(i)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 100; i < 200; i++ {
			q.Push(i)
			_ = q.Pop()
		}
	}()

	n := 0
	for i, v := range q.All() {
		if i != v {
			t.Errorf("All: snapshot expected index %d to match value %d", i, v)
		}
		n++
		// Calling queue methods from inside the loop must not deadlock.
		_ = q.Length()
	}
	if n != 100 {
		t.Errorf("All: expected a snapshot of 100 elements got %d", n)
	}
	wg.Wait()
}

func TestIterators(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 5; i++ {
		q.Push(i)
	}

	n := 0
	for i, v := range q.All() {
		if i != v {
			t.Errorf("All: expected index %d to match value %d", i, v)
		}
		n++
	}
	if n != 5 {
		t.Errorf("All: expected 5 elements got %d", n)
	}

	expect := 4
	n = 0
	for i, v := range q.Backward() {
		if v != expect || i != expect {
			t.Errorf("Backward: expected index/value %d got %d/%d", expect, i, v)
		}
		expect--
		n++
	}
	if n != 5 {
		t.Errorf("Backward: expected 5 elements got %d", n)
	}

	var empty Queue[int]
	for range empty.All() {
		t.Errorf("All: expected no elements on empty queue")
	}
	for range empty.Backward() {
		t.Errorf("Backward: expected no elements on empty queue")
	}
}

func BenchmarkPush(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
}

func BenchmarkPop(b *testing.B) {
	var q Queue[int]
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Pop(); err != nil {
			b.Fatalf("Unexpected error on Pop: %s", err)
		}
	}
}

func BenchmarkEnqueueDequeue(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
		if _, err := q.Dequeue(); err != nil {
			b.Fatalf("Unexpected error on Dequeue: %s", err)
		}
	}
}

func BenchmarkPeek(b *testing.B) {
	var q Queue[int]
	q.Push(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.Peek(); err != nil {
			b.Fatalf("Unexpected error on Peek: %s", err)
		}
	}
}

func ExampleQueue() {
	var q Queue[int]
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)
	for _, v := range q.All() {
		fmt.Println(v)
	}
	// Output:
	// 1
	// 2
	// 3
}

/* vim: set noai ts=4 sw=4: */

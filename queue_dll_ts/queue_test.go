package queue_dll_ts

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

type TestDemo struct {
	S string
}

var _ comparable.Equality = (*TestDemo)(nil)

func (aa TestDemo) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(TestDemo); ok {
		return aa.S == bb.S
	} else if bb, ok := x.(*TestDemo); ok {
		return aa.S == bb.S
	}
	panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
}

func TestQueue001(t *testing.T) {

	var Que1 Queue[TestDemo]

	if !Que1.IsEmpty() {
		t.Errorf("Expected empty queue after declaration, failed to get one.")
	}

	Que1.Push(&TestDemo{S: "hi"})

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

	Que1.Push(&TestDemo{S: "hi2"})
	Que1.Push(&TestDemo{S: "hi3"})

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
	var q Queue[TestDemo]

	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue on empty Dequeue, got %v", err)
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue on empty Peek, got %v", err)
	}

	// FIFO order check.
	for i := 0; i < 10; i++ {
		q.Enqueue(&TestDemo{S: fmt.Sprintf("v%d", i)})
	}
	if q.Length() != 10 {
		t.Errorf("Expected length of 10 got %d", q.Length())
	}
	for i := 0; i < 10; i++ {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Unexpected error on Dequeue: %s", err)
		}
		if expect := fmt.Sprintf("v%d", i); v.S != expect {
			t.Errorf("Expected %s got %s", expect, v.S)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all elements")
	}
}

func TestIterators(t *testing.T) {
	var q Queue[TestDemo]
	for i := 0; i < 5; i++ {
		q.Push(&TestDemo{S: fmt.Sprintf("v%d", i)})
	}

	// All: head to tail.
	n := 0
	for i, v := range q.All() {
		if expect := fmt.Sprintf("v%d", i); v.S != expect {
			t.Errorf("All: expected %s got %s", expect, v.S)
		}
		n++
	}
	if n != 5 {
		t.Errorf("All: expected 5 elements got %d", n)
	}

	// All with early break.
	n = 0
	for range q.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("All: expected early break after 2 elements, got %d", n)
	}

	// Backward: tail to head, indexes matching All.
	expect := 4
	n = 0
	for i, v := range q.Backward() {
		if i != expect {
			t.Errorf("Backward: expected index %d got %d", expect, i)
		}
		if want := fmt.Sprintf("v%d", expect); v.S != want {
			t.Errorf("Backward: expected %s got %s", want, v.S)
		}
		expect--
		n++
	}
	if n != 5 {
		t.Errorf("Backward: expected 5 elements got %d", n)
	}

	// Iterating an empty queue yields nothing.
	var empty Queue[TestDemo]
	for range empty.All() {
		t.Errorf("All: expected no elements on empty queue")
	}
	for range empty.Backward() {
		t.Errorf("Backward: expected no elements on empty queue")
	}
}

// TestConcurrent runs producers and consumers against the queue in parallel.
// Run with `go test -race`; the race detector validates the locking.
func TestConcurrent(t *testing.T) {
	var q Queue[TestDemo]
	const producers = 4
	const perProducer = 250

	var wg sync.WaitGroup

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
				q.Enqueue(&TestDemo{S: fmt.Sprintf("%d-%d", base, i)})
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

func BenchmarkPush(b *testing.B) {
	var q Queue[TestDemo]
	v := TestDemo{S: "hi"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(&v)
	}
}

func BenchmarkPop(b *testing.B) {
	var q Queue[TestDemo]
	v := TestDemo{S: "hi"}
	for i := 0; i < b.N; i++ {
		q.Push(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Pop(); err != nil {
			b.Fatalf("Unexpected error on Pop: %s", err)
		}
	}
}

func BenchmarkEnqueueDequeue(b *testing.B) {
	var q Queue[TestDemo]
	v := TestDemo{S: "hi"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(&v)
		if _, err := q.Dequeue(); err != nil {
			b.Fatalf("Unexpected error on Dequeue: %s", err)
		}
	}
}

func BenchmarkPeek(b *testing.B) {
	var q Queue[TestDemo]
	q.Push(&TestDemo{S: "hi"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.Peek(); err != nil {
			b.Fatalf("Unexpected error on Peek: %s", err)
		}
	}
}

func ExampleQueue() {
	var q Queue[TestDemo]
	q.Enqueue(&TestDemo{S: "a"})
	q.Enqueue(&TestDemo{S: "b"})
	q.Enqueue(&TestDemo{S: "c"})
	for _, v := range q.All() {
		fmt.Println(v.S)
	}
	// Output:
	// a
	// b
	// c
}

/* vim: set noai ts=4 sw=4: */

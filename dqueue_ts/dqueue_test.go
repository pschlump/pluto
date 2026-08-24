package dqueue_ts

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

func TestDeque001(t *testing.T) {

	var Que1 Deque[TestDemo]

	if !Que1.IsEmpty() {
		t.Errorf("Expected empty queue after declaration, failed to get one.")
	}

	Que1.PushFront(&TestDemo{S: "hi"})

	if Que1.IsEmpty() {
		t.Errorf("Expected non-empty queue after 1st push, failed to get one.")
	}

	_, err := Que1.PopFront()
	if err != nil {
		t.Errorf("Unexpected empty queue error after 1 pop")
	}
	_, err = Que1.PopFront()
	if err == nil {
		t.Errorf("Unexpected lack of error after pop on empty queue")
	}
	if !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque, got %v", err)
	}

	Que1.PushBack(&TestDemo{S: "hi2"})
	Que1.PushBack(&TestDemo{S: "hi3"})

	got := Que1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Que1.PeekFront()
	if err != nil {
		t.Errorf("Unexpected error on non-empty queue")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected %s got %s", "hi2", ss.S)
	}

	_, _ = Que1.PopFront()
	ss, err = Que1.PeekFront()
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

func TestPushPopOrder(t *testing.T) {
	var q Deque[TestDemo]

	// Empty queue errors.
	if _, err := q.PopFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque on empty PopFront, got %v", err)
	}
	if _, err := q.PopBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque on empty PopBack, got %v", err)
	}
	if _, err := q.PeekFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque on empty PeekFront, got %v", err)
	}
	if _, err := q.PeekBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque on empty PeekBack, got %v", err)
	}

	// PushFront at the front, PushBack at the back: front..back is f2,f1,b1,b2.
	q.PushBack(&TestDemo{S: "b1"})
	q.PushFront(&TestDemo{S: "f1"})
	q.PushFront(&TestDemo{S: "f2"})
	q.PushBack(&TestDemo{S: "b2"})

	if q.Length() != 4 {
		t.Errorf("Expected length of 4 got %d", q.Length())
	}

	v, err := q.PeekFront()
	if err != nil || v.S != "f2" {
		t.Errorf("PeekFront: expected f2 got %v, err %v", v, err)
	}
	v, err = q.PeekBack()
	if err != nil || v.S != "b2" {
		t.Errorf("PeekBack: expected b2 got %v, err %v", v, err)
	}

	// PopFront removes from the front.
	v, err = q.PopFront()
	if err != nil || v.S != "f2" {
		t.Errorf("PopFront: expected f2 got %v, err %v", v, err)
	}
	// PopBack removes from the back.
	v, err = q.PopBack()
	if err != nil || v.S != "b2" {
		t.Errorf("PopBack: expected b2 got %v, err %v", v, err)
	}

	// Remaining order front..back is f1,b1.
	for _, expect := range []string{"f1", "b1"} {
		v, err := q.PopFront()
		if err != nil {
			t.Fatalf("Unexpected error on PopFront: %s", err)
		}
		if v.S != expect {
			t.Errorf("Expected %s got %s", expect, v.S)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after popping all elements")
	}
}

func TestIterators(t *testing.T) {
	var q Deque[TestDemo]
	for i := 0; i < 5; i++ {
		q.PushBack(&TestDemo{S: fmt.Sprintf("v%d", i)})
	}

	// All: front to back.
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

	// Backward: back to front, indexes matching All.
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
	var empty Deque[TestDemo]
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
	var q Deque[TestDemo]
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
						if _, err := q.PopFront(); err != nil {
							return
						}
						mu.Lock()
						consumed++
						mu.Unlock()
					}
				default:
					if _, err := q.PopBack(); err == nil {
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
				if base%2 == 0 {
					q.PushFront(&TestDemo{S: fmt.Sprintf("%d-%d", base, i)})
				} else {
					q.PushBack(&TestDemo{S: fmt.Sprintf("%d-%d", base, i)})
				}
				_, _ = q.PeekFront()
				_, _ = q.PeekBack()
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

func BenchmarkPushFront(b *testing.B) {
	var q Deque[TestDemo]
	v := TestDemo{S: "hi"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushFront(&v)
	}
}

func BenchmarkPushBack(b *testing.B) {
	var q Deque[TestDemo]
	v := TestDemo{S: "hi"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushBack(&v)
	}
}

func BenchmarkPopFront(b *testing.B) {
	var q Deque[TestDemo]
	v := TestDemo{S: "hi"}
	for i := 0; i < b.N; i++ {
		q.PushBack(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.PopFront(); err != nil {
			b.Fatalf("Unexpected error on PopFront: %s", err)
		}
	}
}

func BenchmarkPopBack(b *testing.B) {
	var q Deque[TestDemo]
	v := TestDemo{S: "hi"}
	for i := 0; i < b.N; i++ {
		q.PushBack(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.PopBack(); err != nil {
			b.Fatalf("Unexpected error on PopBack: %s", err)
		}
	}
}

func BenchmarkPushBackPopFront(b *testing.B) {
	var q Deque[TestDemo]
	v := TestDemo{S: "hi"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushBack(&v)
		if _, err := q.PopFront(); err != nil {
			b.Fatalf("Unexpected error on PopFront: %s", err)
		}
	}
}

func BenchmarkPeekFront(b *testing.B) {
	var q Deque[TestDemo]
	q.PushBack(&TestDemo{S: "hi"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.PeekFront(); err != nil {
			b.Fatalf("Unexpected error on PeekFront: %s", err)
		}
	}
}

func ExampleDeque() {
	var q Deque[TestDemo]
	q.PushBack(&TestDemo{S: "b"})
	q.PushFront(&TestDemo{S: "a"})
	q.PushBack(&TestDemo{S: "c"})
	for _, v := range q.All() {
		fmt.Println(v.S)
	}
	// Output:
	// a
	// b
	// c
}

/* vim: set noai ts=4 sw=4: */

package queue

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
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

	// Interleaved push/pop.
	for i := 0; i < 100; i++ {
		q.Push(i)
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Unexpected error on Dequeue: %s", err)
		}
		if *v != i {
			t.Errorf("Expected %d got %d", i, *v)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after interleaved push/pop")
	}
}

func TestPopReleasesBackingArray(t *testing.T) {
	var q Queue[*int]
	a, b := 1, 2
	q.Push(&a)
	q.Push(&b)
	if err := q.Pop(); err != nil {
		t.Fatalf("Unexpected error on Pop: %s", err)
	}
	if q.IsEmpty() {
		t.Errorf("Expected non-empty queue after popping 1 of 2 elements")
	}
	v, err := q.Peek()
	if err != nil || *v != &b {
		t.Errorf("Expected head element to survive pop")
	}
	if err := q.Pop(); err != nil {
		t.Fatalf("Unexpected error on Pop: %s", err)
	}
	// Popping the last element must release the backing array.
	if q.data != nil {
		t.Errorf("Expected backing array to be released after popping last element")
	}
	// Queue must still be usable.
	q.Push(&a)
	if q.Length() != 1 {
		t.Errorf("Expected length of 1 got %d", q.Length())
	}
}

func TestTruncate(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 5; i++ {
		q.Push(i)
	}
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate")
	}
	if q.Length() != 0 {
		t.Errorf("Expected length of 0 got %d", q.Length())
	}
	if q.data != nil {
		t.Errorf("Expected backing array to be released after Truncate")
	}
	q.Push(42)
	v, err := q.Dequeue()
	if err != nil || *v != 42 {
		t.Errorf("Expected 42 after Truncate then Push, got %v, %v", v, err)
	}
}

func TestIterators(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 5; i++ {
		q.Push(i)
	}

	// All: head to tail.
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

	// Backward: tail to head.
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

	// Iterating an empty queue yields nothing.
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

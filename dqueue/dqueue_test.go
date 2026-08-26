package dqueue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDeque(t *testing.T) {
	var q Deque[string]

	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after declaration.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected length 0 after declaration, got %d/%d", q.Len(), q.Length())
	}

	if _, err := q.PopFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopFront on empty deque, got %v", err)
	}
	if _, err := q.PopBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopBack on empty deque, got %v", err)
	}
	if _, err := q.PeekFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekFront on empty deque, got %v", err)
	}
	if _, err := q.PeekBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekBack on empty deque, got %v", err)
	}

	q.PushBack("hi")

	if q.IsEmpty() {
		t.Errorf("Expected non-empty deque after push.")
	}

	x, err := q.PopFront()
	if err != nil {
		t.Errorf("Unexpected empty-deque error after 1 pop: %v", err)
	}
	if x != "hi" {
		t.Errorf("Expected %q got %q", "hi", x)
	}
	if _, err := q.PopBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque after draining, got %v", err)
	}

	q.PushBack("hi")
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after Truncate.")
	}

	q.PushBack("hi2")
	q.PushBack("hi3")

	if got, want := q.Length(), 2; got != want {
		t.Errorf("Expected length of %d got %d", want, got)
	}

	ss, err := q.PeekFront()
	if err != nil {
		t.Errorf("Unexpected error on non-empty deque")
	}
	if ss != "hi2" {
		t.Errorf("Expected %s got %s", "hi2", ss)
	}
}

// TestPushPopOrder verifies the two ends are independent: PushFront adds
// at the front, PushBack at the back, and the pops remove from their own
// end.
func TestPushPopOrder(t *testing.T) {
	var q Deque[int]

	// PushFront at the front, PushBack at the back: front..back is f2,f1,b1,b2.
	q.PushBack(11)  // b1
	q.PushFront(21) // f1
	q.PushFront(22) // f2
	q.PushBack(12)  // b2

	if q.Length() != 4 {
		t.Fatalf("Expected length of 4 got %d", q.Length())
	}

	if v, err := q.PeekFront(); err != nil || v != 22 {
		t.Errorf("PeekFront = (%v, %v), expected 22", v, err)
	}
	if v, err := q.PeekBack(); err != nil || v != 12 {
		t.Errorf("PeekBack = (%v, %v), expected 12", v, err)
	}

	// PopFront removes from the front.
	if v, err := q.PopFront(); err != nil || v != 22 {
		t.Errorf("PopFront = (%v, %v), expected 22", v, err)
	}
	// PopBack removes from the back.
	if v, err := q.PopBack(); err != nil || v != 12 {
		t.Errorf("PopBack = (%v, %v), expected 12", v, err)
	}

	// Remaining order front..back is f1,b1.
	for i, expect := range []int{21, 11} {
		v, err := q.PopFront()
		if err != nil {
			t.Fatalf("Unexpected error on PopFront: %s", err)
		}
		if v != expect {
			t.Errorf("Step %d: expected %d got %d", i, expect, v)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty deque after popping all elements")
	}
}

// TestDequeIterators covers All and Backward: order, indexes, early
// exit, and the empty deque.
func TestDequeIterators(t *testing.T) {
	var q Deque[int]
	for i := range 5 {
		q.PushBack(i)
	}

	// All: front to back.
	var got []int
	for i, v := range q.All() {
		if i != len(got) {
			t.Errorf("All: expected index %d, got %d", len(got), i)
		}
		got = append(got, v)
	}
	if want := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("All: expected %v, got %v", want, got)
	}

	// All with early break.
	n := 0
	for range q.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("All: expected early break after 2 elements, got %d", n)
	}

	// Backward with early break.
	n = 0
	for range q.Backward() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("Backward: expected early break after 2 elements, got %d", n)
	}

	// Backward: back to front, indexes matching All.
	got = got[:0]
	expect := 4
	for i, v := range q.Backward() {
		if i != expect {
			t.Errorf("Backward: expected index %d got %d", expect, i)
		}
		if v != expect {
			t.Errorf("Backward: expected value %d got %d", expect, v)
		}
		expect--
		got = append(got, v)
	}
	if want := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("Backward: expected %v, got %v", want, got)
	}

	// Iterating an empty deque yields nothing.
	var empty Deque[int]
	for range empty.All() {
		t.Errorf("All: expected no elements on empty deque")
	}
	for range empty.Backward() {
		t.Errorf("Backward: expected no elements on empty deque")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value and nil deque
// -------------------------------------------------------------------------------------------------------

// TestZeroValueSemantics verifies that the zero value is a fully usable
// empty deque — no constructor, no constraints on the element type.
func TestZeroValueSemantics(t *testing.T) {
	var q Deque[int]

	if !q.IsEmpty() {
		t.Errorf("Expected zero value deque to be empty.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected zero value deque to have length 0.")
	}
	if _, err := q.PopFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopFront on zero value deque, got %v", err)
	}
	if _, err := q.PopBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopBack on zero value deque, got %v", err)
	}
	if _, err := q.PeekFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekFront on zero value deque, got %v", err)
	}
	if _, err := q.PeekBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekBack on zero value deque, got %v", err)
	}

	q.PushFront(1)
	q.PushBack(2)
	if q.Length() != 2 {
		t.Errorf("Expected length 2 after pushes, got %d", q.Length())
	}
	if v, err := q.PopFront(); err != nil || v != 1 {
		t.Errorf("PopFront = (%v, %v), expected 1", v, err)
	}
	checkModel(t, &q, []int{2})
}

// TestNilDequeTolerated verifies that every operation except the pushes
// treats a nil deque as an empty deque, and that PushFront and PushBack
// panic with messages naming the method — the package's only panics.
func TestNilDequeTolerated(t *testing.T) {
	var q *Deque[int]

	if !q.IsEmpty() {
		t.Errorf("Expected nil deque to be empty.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected nil deque to have length 0.")
	}
	if _, err := q.PopFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopFront on nil deque, got %v", err)
	}
	if _, err := q.PopBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PopBack on nil deque, got %v", err)
	}
	if _, err := q.PeekFront(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekFront on nil deque, got %v", err)
	}
	if _, err := q.PeekBack(); !errors.Is(err, ErrEmptyDeque) {
		t.Errorf("Expected ErrEmptyDeque from PeekBack on nil deque, got %v", err)
	}
	q.Truncate() // no-op, must not panic
	for range q.All() {
		t.Errorf("Expected no values from All on nil deque.")
	}
	for range q.Backward() {
		t.Errorf("Expected no values from Backward on nil deque.")
	}

	for _, tc := range []struct {
		name string
		call func()
	}{
		{"PushFront", func() { q.PushFront(1) }},
		{"PushBack", func() { q.PushBack(1) }},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on nil deque to panic.", tc.name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, tc.name) {
					t.Errorf("%s: unexpected panic message: %v", tc.name, r)
				}
			}()
			tc.call()
		}()
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------------------------------------

const benchmarkDequeSize = 4096

func BenchmarkPushFront(b *testing.B) {
	var q Deque[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushFront(i)
	}
}

func BenchmarkPushBack(b *testing.B) {
	var q Deque[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushBack(i)
	}
}

func BenchmarkPopFront(b *testing.B) {
	var q Deque[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.IsEmpty() {
			for j := range benchmarkDequeSize {
				q.PushBack(j)
			}
		}
		_, _ = q.PopFront()
	}
}

func BenchmarkPopBack(b *testing.B) {
	var q Deque[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.IsEmpty() {
			for j := range benchmarkDequeSize {
				q.PushBack(j)
			}
		}
		_, _ = q.PopBack()
	}
}

func BenchmarkPushBackPopFront(b *testing.B) {
	var q Deque[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PushBack(i)
		if _, err := q.PopFront(); err != nil {
			b.Fatalf("PopFront: %v", err)
		}
	}
}

func BenchmarkPeekFront(b *testing.B) {
	var q Deque[int]
	for i := range benchmarkDequeSize {
		q.PushBack(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = q.PeekFront()
	}
}

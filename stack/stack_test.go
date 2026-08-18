package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"slices"
	"testing"
)

func TestStack(t *testing.T) {
	type TestDemo struct {
		S string
	}

	var Stk1 Stack[TestDemo]

	if !Stk1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Stk1.Push(TestDemo{S: "hi"})
	Stk1.Push(TestDemo{S: "there"})

	if Stk1.IsEmpty() {
		t.Errorf("Expected non-empty stack after 1st push, failed to get one.")
	}

	x, err := Stk1.Pop()
	if err != nil {
		t.Errorf("Unexpectd empty stack error after 1 pop")
	}
	if x.S != "there" {
		t.Errorf("Unexpectd value")
	}
	x, _ = Stk1.Pop()
	if x.S != "hi" {
		t.Errorf("Unexpectd value")
	}
	x, err = Stk1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}

	Stk1.Push(TestDemo{S: "hi"})
	Stk1.Truncate()
	if !Stk1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Stk1.Push(TestDemo{S: "hi2"})
	Stk1.Push(TestDemo{S: "hi3"})

	got := Stk1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Stk1.Peek()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty stack")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}
}

func TestStackPopZeroesSlot(t *testing.T) {
	a, b := 1, 2
	stk := Stack[*int]{&a, &b}

	rv, err := stk.Pop()
	if err != nil {
		t.Fatalf("Unexpected error on pop: %v", err)
	}
	if rv != &b {
		t.Errorf("Expected pop to return &b")
	}
	// The popped slot in the backing array must be zeroed so the GC can
	// reclaim the value.
	full := stk[:cap(stk)]
	if full[len(full)-1] != nil {
		t.Errorf("Expected popped slot to be zeroed, got %v", full[len(full)-1])
	}
}

func TestStackPopEmpty(t *testing.T) {
	var stk Stack[int]
	_, err := stk.Pop()
	if !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack, got %v", err)
	}
	_, err = stk.Peek()
	if !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack, got %v", err)
	}
	// Truncate on an empty stack must be a no-op.
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after Truncate")
	}
}

func TestStackTruncateReleases(t *testing.T) {
	var stk Stack[int]
	for i := 0; i < 10; i++ {
		stk.Push(i)
	}
	stk.Truncate()
	if !stk.IsEmpty() || stk.Length() != 0 {
		t.Errorf("Expected empty stack after Truncate")
	}
	if cap(stk) != 0 {
		t.Errorf("Expected Truncate to release the backing array")
	}
	// Stack must still be usable after a Truncate.
	stk.Push(42)
	if v, err := stk.Pop(); err != nil || v != 42 {
		t.Errorf("Expected 42, got %v, %v", v, err)
	}
}

func TestStackIterators(t *testing.T) {
	var stk Stack[int]
	for i := 1; i <= 3; i++ {
		stk.Push(i)
	}

	// All iterates top to bottom: 3, 2, 1.
	var got []int
	for i, v := range stk.All() {
		if i != len(got) {
			t.Errorf("Expected index %d, got %d", len(got), i)
		}
		got = append(got, v)
	}
	if want := []int{3, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("All: expected %v, got %v", want, got)
	}

	// Backward iterates bottom to top: 1, 2, 3.
	got = got[:0]
	for _, v := range stk.Backward() {
		got = append(got, v)
	}
	if want := []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("Backward: expected %v, got %v", want, got)
	}

	// Early exit must stop the iteration.
	n := 0
	for range stk.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early exit after 1 element, got %d", n)
	}

	// Iterating an empty stack yields nothing.
	var empty Stack[int]
	for range empty.All() {
		t.Errorf("Expected no elements from empty stack")
	}
	for range empty.Backward() {
		t.Errorf("Expected no elements from empty stack")
	}
}

func BenchmarkPush(b *testing.B) {
	var stk Stack[int]
	for i := 0; i < b.N; i++ {
		stk.Push(i)
	}
}

func BenchmarkPop(b *testing.B) {
	var stk Stack[int]
	for i := 0; i < b.N; i++ {
		stk.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := stk.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

package stack_ts

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

func TestStack(t *testing.T) {
	var stk Stack[string]

	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after declaration.")
	}
	if stk.Len() != 0 || stk.Length() != 0 {
		t.Errorf("Expected length 0 after declaration, got %d/%d", stk.Len(), stk.Length())
	}

	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Pop on empty stack, got %v", err)
	}
	if _, err := stk.Peek(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Peek on empty stack, got %v", err)
	}

	stk.Push("a")
	stk.Push("b")
	stk.Push("c")

	if stk.IsEmpty() {
		t.Errorf("Expected non-empty stack after pushes.")
	}
	if stk.Length() != 3 {
		t.Errorf("Expected length 3 after pushes, got %d", stk.Length())
	}

	// LIFO order: last pushed is first popped.
	for _, want := range []string{"c", "b", "a"} {
		if v, err := stk.Peek(); err != nil || v != want {
			t.Errorf("Peek = (%q, %v), expected %q", v, err, want)
		}
		v, err := stk.Pop()
		if err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if v != want {
			t.Errorf("Pop = %q, expected %q", v, want)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after draining.")
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack after draining, got %v", err)
	}
}

// TestStackPopZeroesSlot verifies that popping clears the vacated slot so
// the backing array does not keep the popped value alive.
func TestStackPopZeroesSlot(t *testing.T) {
	stk := &Stack[int]{}
	for i := range 4 {
		stk.Push(i)
	}
	// Keep a view of the backing array from before the pop; after the pop
	// the slot sits outside the stack's window but is still observable
	// through this view.  (Single-goroutine test: reading internals
	// without the lock is fine here.)
	before := stk.data
	if _, err := stk.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if got := before[3]; got != 0 {
		t.Errorf("Expected the vacated slot to be zeroed, got %d", got)
	}
	stk.lock.RLock()
	if len(stk.data) != 3 {
		t.Errorf("Expected length 3 after pop, got %d", len(stk.data))
	}
	stk.lock.RUnlock()
}

// TestPopReleasesBackingArray verifies that draining the stack releases
// the backing array entirely (data becomes nil), so a drained stack holds
// no reference to the popped elements.
func TestPopReleasesBackingArray(t *testing.T) {
	stk := &Stack[int]{}
	for i := range 10 {
		stk.Push(i)
	}
	for i := range 10 {
		if _, err := stk.Pop(); err != nil {
			t.Fatalf("Pop(%d): %v", i, err)
		}
	}
	stk.lock.RLock()
	defer stk.lock.RUnlock()
	if stk.data != nil {
		t.Errorf("Expected nil backing array after draining, got len=%d cap=%d", len(stk.data), cap(stk.data))
	}
}

func TestStackPopEmpty(t *testing.T) {
	stk := &Stack[int]{}
	for i := range 2 {
		if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
			t.Errorf("Pop on empty stack #%d: got %v, expected ErrEmptyStack", i, err)
		}
		if _, err := stk.Peek(); !errors.Is(err, ErrEmptyStack) {
			t.Errorf("Peek on empty stack #%d: got %v, expected ErrEmptyStack", i, err)
		}
	}
	// The empty stack is still usable.
	stk.Push(1)
	if v, err := stk.Pop(); err != nil || v != 1 {
		t.Errorf("Pop after empty-cycle = (%v, %v), expected 1", v, err)
	}
}

// TestStackTruncateReleases verifies that Truncate drops the backing
// array entirely and that the stack is reusable afterwards.
func TestStackTruncateReleases(t *testing.T) {
	stk := &Stack[int]{}
	for i := range 10 {
		stk.Push(i)
	}
	stk.Truncate()
	stk.lock.RLock()
	if stk.data != nil {
		t.Errorf("Expected nil backing array after Truncate, got len=%d cap=%d", len(stk.data), cap(stk.data))
	}
	stk.lock.RUnlock()
	if !stk.IsEmpty() || stk.Length() != 0 {
		t.Errorf("Expected empty stack after Truncate.")
	}

	// Reusable.
	stk.Push(7)
	if v, err := stk.Peek(); err != nil || v != 7 {
		t.Errorf("Peek after Truncate = (%v, %v), expected 7", v, err)
	}

	// Truncating an already-empty stack is fine.
	stk.Truncate()
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after double Truncate.")
	}
}

func TestStackIterators(t *testing.T) {
	stk := &Stack[string]{}
	for _, s := range []string{"a", "b", "c"} { // pushed a, then b, then c
		stk.Push(s)
	}

	// All: top (most recent) to bottom.
	var top []string
	for i, v := range stk.All() {
		if i != len(top) {
			t.Fatalf("All: unexpected index %d at step %d", i, len(top))
		}
		top = append(top, v)
	}
	if expect := []string{"c", "b", "a"}; !reflect.DeepEqual(top, expect) {
		t.Errorf("All got %v, expected %v", top, expect)
	}

	// Backward: bottom to top.
	var bottom []string
	for i, v := range stk.Backward() {
		if i != len(bottom) {
			t.Fatalf("Backward: unexpected index %d at step %d", i, len(bottom))
		}
		bottom = append(bottom, v)
	}
	if expect := []string{"a", "b", "c"}; !reflect.DeepEqual(bottom, expect) {
		t.Errorf("Backward got %v, expected %v", bottom, expect)
	}

	// Early break stops iteration.
	n := 0
	for range stk.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Iterating an empty stack yields nothing.
	empty := &Stack[int]{}
	for range empty.All() {
		t.Errorf("Expected no items from All on empty stack")
	}
	for range empty.Backward() {
		t.Errorf("Expected no items from Backward on empty stack")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value and nil stack
// -------------------------------------------------------------------------------------------------------

// TestZeroValueSemantics verifies that the zero value is a fully usable
// empty stack — no constructor needed, Push included.
func TestZeroValueSemantics(t *testing.T) {
	var stk Stack[int]

	if !stk.IsEmpty() {
		t.Errorf("Expected zero value stack to be empty.")
	}
	if stk.Len() != 0 || stk.Length() != 0 {
		t.Errorf("Expected zero value stack to have length 0.")
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Pop on zero value stack, got %v", err)
	}
	if _, err := stk.Peek(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Peek on zero value stack, got %v", err)
	}

	stk.Push(1)
	stk.Push(2)
	if stk.Length() != 2 {
		t.Errorf("Expected length 2 after pushes, got %d", stk.Length())
	}
	if v, err := stk.Pop(); err != nil || v != 2 {
		t.Errorf("Pop = (%v, %v), expected 2 (LIFO)", v, err)
	}
}

// TestNilStackTolerated verifies that every operation except Push treats
// a nil stack as an empty stack, and that Push panics with a message
// naming the method — the package's only panic.
func TestNilStackTolerated(t *testing.T) {
	var stk *Stack[int]

	if !stk.IsEmpty() {
		t.Errorf("Expected nil stack to be empty.")
	}
	if stk.Len() != 0 || stk.Length() != 0 {
		t.Errorf("Expected nil stack to have length 0.")
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Pop on nil stack, got %v", err)
	}
	if _, err := stk.Peek(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Peek on nil stack, got %v", err)
	}
	stk.Truncate() // no-op, must not panic
	for range stk.All() {
		t.Errorf("Expected no values from All on nil stack.")
	}
	for range stk.Backward() {
		t.Errorf("Expected no values from Backward on nil stack.")
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Push on nil stack to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Push") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		stk.Push(1)
	}()
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkStackSize = 4096

func BenchmarkPush(b *testing.B) {
	var stk Stack[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stk.Push(i)
	}
}

func BenchmarkPop(b *testing.B) {
	var stk Stack[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if stk.IsEmpty() {
			for j := range benchmarkStackSize {
				stk.Push(j)
			}
		}
		_, _ = stk.Pop()
	}
}

func BenchmarkPeek(b *testing.B) {
	var stk Stack[int]
	for i := range benchmarkStackSize {
		stk.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = stk.Peek()
	}
}

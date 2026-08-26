package stack_sll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

	stk.Push("hi")
	stk.Push("there")

	if stk.IsEmpty() {
		t.Errorf("Expected non-empty stack after pushes.")
	}

	x, err := stk.Pop()
	if err != nil {
		t.Errorf("Unexpected empty-stack error after 1 pop: %v", err)
	}
	if x != "there" {
		t.Errorf("Expected %q got %q", "there", x)
	}
	x, _ = stk.Pop()
	if x != "hi" {
		t.Errorf("Expected %q got %q", "hi", x)
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack after draining, got %v", err)
	}

	stk.Push("hi")
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after Truncate.")
	}

	stk.Push("hi2")
	stk.Push("hi3")

	if got, want := stk.Length(), 2; got != want {
		t.Errorf("Expected length of %d got %d", want, got)
	}

	ss, err := stk.Peek()
	if err != nil {
		t.Errorf("Unexpected error on non-empty stack")
	}
	if ss != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss)
	}
}

// TestLIFOOrder drains 100 pushes in exact reverse order.
func TestLIFOOrder(t *testing.T) {
	var stk Stack[int]
	for i := range 100 {
		stk.Push(i)
	}
	for i := 99; i >= 0; i-- {
		v, err := stk.Pop()
		if err != nil {
			t.Fatalf("Pop(%d): %v", i, err)
		}
		if v != i {
			t.Fatalf("Pop = %d, expected %d", v, i)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after draining.")
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
	if want := []int{3, 2, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("All: expected %v, got %v", want, got)
	}

	// Backward iterates bottom to top: 1, 2, 3.
	got = got[:0]
	for _, v := range stk.Backward() {
		got = append(got, v)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
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

	// Early exit must stop backward iteration too.
	n = 0
	for range stk.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected Backward early exit after 1 element, got %d", n)
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

// TestStackIterateSnapshot verifies that the All/Backward iterators
// operate on a snapshot taken when they are called: later modifications —
// even truncating the whole stack — are not observed, and mutating the
// stack from inside the loop is safe.
func TestStackIterateSnapshot(t *testing.T) {
	var stk Stack[int]
	for i := range 5 {
		stk.Push(i)
	}

	all := stk.All()
	backward := stk.Backward()

	stk.Truncate() // the iterators above must not observe this

	var fwd []int
	for _, v := range all {
		fwd = append(fwd, v)
	}
	if expect := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, fwd)
	}

	var bwd []int
	for _, v := range backward {
		bwd = append(bwd, v)
	}
	if expect := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward after Truncate error, expected %v got %v", expect, bwd)
	}

	// Popping from inside the loop is safe: the loop sees the snapshot.
	for i := range 3 {
		stk.Push(i)
	}
	visited := 0
	for _, v := range stk.All() {
		visited++
		_, _ = stk.Pop()
		_ = v
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while popping during iteration, got %d", visited)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping during iteration.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value and nil stack
// -------------------------------------------------------------------------------------------------------

// TestZeroValueSemantics verifies that the zero value is a fully usable
// empty stack — no constructor, no constraints on the element type.
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
	checkModel(t, &stk, []int{1})
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
// Concurrency
// -------------------------------------------------------------------------------------------------------

// TestStackConcurrent hammers one stack with concurrent pushers and
// poppers.  It is primarily a test for the race detector (`make race`);
// the accounting must balance: every push is popped exactly once.
func TestStackConcurrent(t *testing.T) {
	var stk Stack[int]

	const workers = 4
	const perWorker = 100
	const total = workers * perWorker

	var wg sync.WaitGroup

	// Concurrent pushers.
	for range workers {
		wg.Go(func() {
			for i := range perWorker {
				stk.Push(i)
			}
		})
	}
	wg.Wait()
	if got, want := stk.Length(), total; got != want {
		t.Fatalf("Expected length %d, got %d", want, got)
	}

	// A reader iterates snapshots while the poppers drain.
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range stk.All() {
			}
			_, _ = stk.Peek()
			_ = stk.Len()
		}
	}()

	// Concurrent poppers: every pop must either succeed or see an empty
	// stack, and the total number of successful pops must equal the
	// number of pushes.
	var popped atomic.Int64
	for range workers {
		wg.Go(func() {
			for {
				_, err := stk.Pop()
				if errors.Is(err, ErrEmptyStack) {
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				popped.Add(1)
			}
		})
	}
	wg.Wait()
	close(stop)

	if got := popped.Load(); got != total {
		t.Errorf("Expected %d successful pops, got %d", total, got)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after concurrent pops")
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------------------------------------

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

func BenchmarkEnqueueDequeue(b *testing.B) {
	var stk Stack[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stk.Push(i)
		if _, err := stk.Pop(); err != nil {
			b.Fatalf("Pop: %v", err)
		}
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

package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/sll_ts"
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
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
}

// Int is a simple comparable.Equality implementation for tests and benchmarks.
type Int int

func (aa Int) IsEqual(x comparable.Equality) bool {
	bb, ok := x.(Int)
	return ok && aa == bb
}

func TestStack(t *testing.T) {
	var Stk1 Stack[TestDemo]

	if !Stk1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Stk1.Push(&TestDemo{S: "hi"})
	Stk1.Push(&TestDemo{S: "there"})

	if Stk1.IsEmpty() {
		t.Errorf("Expected non-empty stack after 1st push, failed to get one.")
	}

	x, err := Stk1.Pop()
	if err != nil {
		t.Errorf("Unexpectd empty stack error after 1 pop")
	}
	if x.S != "there" {
		t.Errorf("Unexpectd value, of %q got %v", "there", x)
	}
	x, _ = Stk1.Pop()
	if x.S != "hi" {
		t.Errorf("Unexpectd value, of %q got %v", "hi", x)
	}
	_, err = Stk1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty stack")
	}

	Stk1.Push(&TestDemo{S: "hi"})
	Stk1.Truncate()
	if !Stk1.IsEmpty() {
		t.Errorf("Expected empty stack after decleration, failed to get one.")
	}

	Stk1.Push(&TestDemo{S: "hi2"})
	Stk1.Push(&TestDemo{S: "hi3"})

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

/* vim: set noai ts=4 sw=4: */

func TestStackIterators(t *testing.T) {
	var stk Stack[Int]
	for i := 1; i <= 3; i++ {
		v := Int(i)
		stk.Push(&v)
	}

	// All iterates top to bottom: 3, 2, 1.
	var got []int
	for i, v := range stk.All() {
		if i != len(got) {
			t.Errorf("Expected index %d, got %d", len(got), i)
		}
		got = append(got, int(*v))
	}
	if want := []int{3, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("All: expected %v, got %v", want, got)
	}

	// Backward iterates bottom to top: 1, 2, 3.
	got = got[:0]
	for _, v := range stk.Backward() {
		got = append(got, int(*v))
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
	var empty Stack[Int]
	for range empty.All() {
		t.Errorf("Expected no elements from empty stack")
	}
	for range empty.Backward() {
		t.Errorf("Expected no elements from empty stack")
	}
}

func TestStackPopEmptyError(t *testing.T) {
	var stk Stack[Int]
	_, err := stk.Pop()
	if !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll, got %v", err)
	}
	_, err = stk.Peek()
	if !errors.Is(err, sll_ts.ErrEmptySll) {
		t.Errorf("Expected sll_ts.ErrEmptySll, got %v", err)
	}
}

func TestStackConcurrent(t *testing.T) {
	var stk Stack[Int]
	const perWorker = 100
	var wg sync.WaitGroup

	// Concurrent pushers.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				v := Int(i)
				stk.Push(&v)
			}
		}()
	}
	wg.Wait()
	if got, want := stk.Length(), 4*perWorker; got != want {
		t.Fatalf("Expected length %d, got %d", want, got)
	}

	// Concurrent poppers: every pop must either succeed or see an empty
	// stack, and the total number of successful pops must equal the
	// number of pushes.
	var popped atomic.Int64
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, err := stk.Pop()
				if errors.Is(err, sll_ts.ErrEmptySll) {
					return
				}
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				popped.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := popped.Load(); got != 4*perWorker {
		t.Errorf("Expected %d successful pops, got %d", 4*perWorker, got)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after concurrent pops")
	}
}

func BenchmarkPush(b *testing.B) {
	var stk Stack[Int]
	v := Int(1)
	for i := 0; i < b.N; i++ {
		stk.Push(&v)
	}
}

func BenchmarkPop(b *testing.B) {
	var stk Stack[Int]
	v := Int(1)
	for i := 0; i < b.N; i++ {
		stk.Push(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := stk.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"math/rand"
	"slices"
	"testing"
)

// checkModel verifies the stack's observable state against a reference
// model (a plain slice where the top is the last element).
func checkModel[T comparable](t *testing.T, stk *Stack[T], model []T) {
	t.Helper()
	if got, want := stk.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := stk.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	if len(model) > 0 {
		top, err := stk.Peek()
		if err != nil {
			t.Fatalf("Peek on non-empty stack returned error: %v", err)
		}
		if *top != model[len(model)-1] {
			t.Fatalf("Peek: expected %v, got %v", model[len(model)-1], *top)
		}
	}
	// All must iterate top to bottom.
	var fwd []T
	for _, v := range stk.All() {
		fwd = append(fwd, v)
	}
	var wantFwd []T
	for i := len(model) - 1; i >= 0; i-- {
		wantFwd = append(wantFwd, model[i])
	}
	if !slices.Equal(fwd, wantFwd) {
		t.Fatalf("All: expected %v, got %v", wantFwd, fwd)
	}
	// Backward must iterate bottom to top.
	var bwd []T
	for _, v := range stk.Backward() {
		bwd = append(bwd, v)
	}
	if !slices.Equal(bwd, model) {
		t.Fatalf("Backward: expected %v, got %v", model, bwd)
	}
}

func TestZeroValueSemantics(t *testing.T) {
	// A nil (zero-value) stack must be fully usable without a constructor.
	var stk Stack[int]
	if stk.Length() != 0 {
		t.Errorf("Expected length 0 on zero value, got %d", stk.Length())
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected zero value to be empty")
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack from Pop on zero value, got %v", err)
	}
	if p, err := stk.Peek(); p != nil || !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected (nil, ErrEmptyStack) from Peek on zero value, got (%v, %v)", p, err)
	}
}

func TestSingleElement(t *testing.T) {
	var stk Stack[int]
	stk.Push(7)

	if stk.Length() != 1 {
		t.Errorf("Expected length 1, got %d", stk.Length())
	}
	if stk.IsEmpty() {
		t.Errorf("Expected non-empty stack")
	}

	// Peek twice: Peek must not remove the element.
	for i := 0; i < 2; i++ {
		p, err := stk.Peek()
		if err != nil {
			t.Fatalf("Unexpected Peek error: %v", err)
		}
		if *p != 7 {
			t.Errorf("Peek: expected 7, got %d", *p)
		}
		if stk.Length() != 1 {
			t.Errorf("Peek must not change the length; got %d", stk.Length())
		}
	}

	v, err := stk.Pop()
	if err != nil || v != 7 {
		t.Errorf("Pop: expected (7, nil), got (%d, %v)", v, err)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping the only element")
	}
	// Popping again must fail.
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack, got %v", err)
	}
}

func TestDuplicateValues(t *testing.T) {
	// A stack must happily hold duplicate values.
	var stk Stack[int]
	for i := 0; i < 5; i++ {
		stk.Push(9)
	}
	if stk.Length() != 5 {
		t.Fatalf("Expected length 5, got %d", stk.Length())
	}
	for i := 0; i < 5; i++ {
		v, err := stk.Pop()
		if err != nil || v != 9 {
			t.Errorf("Pop %d: expected (9, nil), got (%d, %v)", i, v, err)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping all duplicates")
	}
}

func TestPeekAliasesTopElement(t *testing.T) {
	// Peek returns a pointer that aliases the stored element; mutating
	// through it changes what a subsequent Pop returns.
	var stk Stack[int]
	stk.Push(1)
	p, err := stk.Peek()
	if err != nil {
		t.Fatalf("Unexpected Peek error: %v", err)
	}
	*p = 99
	v, err := stk.Pop()
	if err != nil {
		t.Fatalf("Unexpected Pop error: %v", err)
	}
	if v != 99 {
		t.Errorf("Expected mutation through Peek pointer to be visible, got %d", v)
	}
}

func TestLIFOOrder(t *testing.T) {
	// Push 0..n-1, expect pops in strictly reverse order.
	const n = 100
	var stk Stack[int]
	for i := 0; i < n; i++ {
		stk.Push(i)
	}
	for want := n - 1; want >= 0; want-- {
		v, err := stk.Pop()
		if err != nil {
			t.Fatalf("Unexpected Pop error: %v", err)
		}
		if v != want {
			t.Errorf("LIFO violation: expected %d, got %d", want, v)
		}
	}
}

func TestPushPopInterleaved(t *testing.T) {
	// Interleaved push/pop must never lose ordering of the survivors.
	var stk Stack[int]
	for i := 0; i < 10; i++ {
		stk.Push(i) // 0..9
	}
	for i := 0; i < 5; i++ {
		if _, err := stk.Pop(); err != nil { // removes 9..5
			t.Fatalf("Unexpected Pop error: %v", err)
		}
	}
	stk.Push(100) // stack: 0,1,2,3,4,100
	want := []int{100, 4, 3, 2, 1, 0}
	for _, w := range want {
		v, err := stk.Pop()
		if err != nil || v != w {
			t.Errorf("Expected %d, got (%d, %v)", w, v, err)
		}
	}
}

func TestBackwardEarlyBreak(t *testing.T) {
	var stk Stack[int]
	for i := 1; i <= 3; i++ {
		stk.Push(i)
	}
	n := 0
	first := -1
	for _, v := range stk.Backward() {
		n++
		first = v
		break
	}
	if n != 1 {
		t.Errorf("Expected early exit after 1 element, got %d", n)
	}
	if first != 1 {
		t.Errorf("Backward must start at the bottom element; expected 1, got %d", first)
	}

	// Also verify a partial (non-immediate) break sees the right prefix.
	var got []int
	for _, v := range stk.Backward() {
		got = append(got, v)
		if len(got) == 2 {
			break
		}
	}
	if want := []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("Backward partial: expected %v, got %v", want, got)
	}
}

func TestIteratorIndexSequences(t *testing.T) {
	var stk Stack[int]
	for i := 10; i < 15; i++ {
		stk.Push(i)
	}
	// All: index 0 is the top element.
	wantIdx := 0
	for i, v := range stk.All() {
		if i != wantIdx {
			t.Errorf("All: expected index %d, got %d", wantIdx, i)
		}
		if v != 14-wantIdx {
			t.Errorf("All: expected value %d, got %d", 14-wantIdx, v)
		}
		wantIdx++
	}
	if wantIdx != 5 {
		t.Errorf("All: expected 5 elements, got %d", wantIdx)
	}
	// Backward: index 0 is the bottom element.
	wantIdx = 0
	for i, v := range stk.Backward() {
		if i != wantIdx {
			t.Errorf("Backward: expected index %d, got %d", wantIdx, i)
		}
		if v != 10+wantIdx {
			t.Errorf("Backward: expected value %d, got %d", 10+wantIdx, v)
		}
		wantIdx++
	}
}

func TestIteratorReflectsLiveStack(t *testing.T) {
	// Backward captures the slice length when the loop starts, so an
	// element pushed during iteration is not visited.
	var stk Stack[int]
	stk.Push(1)
	stk.Push(2)

	var got []int
	for _, v := range stk.Backward() {
		got = append(got, v)
		if v == 2 {
			stk.Push(3) // appended after the captured length
		}
	}
	if want := []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("Backward live-push: expected %v, got %v", want, got)
	}

	// All re-evaluates the length on every step, so popping the element
	// just yielded leaves a consistent walk over the survivors.
	stk2 := Stack[int]{1, 2, 3} // top is 3
	got = got[:0]
	for _, v := range stk2.All() {
		got = append(got, v)
		if v == 3 {
			if _, err := stk2.Pop(); err != nil { // removes 3, just yielded
				t.Fatalf("Unexpected Pop error: %v", err)
			}
		}
	}
	// After popping 3, the remaining walk sees 2 then 1.
	if want := []int{3, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("All live-pop: expected %v, got %v", want, got)
	}
}

func TestTruncateThenIterate(t *testing.T) {
	var stk Stack[int]
	for i := 0; i < 5; i++ {
		stk.Push(i)
	}
	stk.Truncate()
	for range stk.All() {
		t.Errorf("Expected no elements from truncated stack")
	}
	for range stk.Backward() {
		t.Errorf("Expected no elements from truncated stack")
	}
	if _, err := stk.Pop(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack after Truncate, got %v", err)
	}
	if _, err := stk.Peek(); !errors.Is(err, ErrEmptyStack) {
		t.Errorf("Expected ErrEmptyStack after Truncate, got %v", err)
	}
}

// TestRandomizedAgainstModel runs a fixed-seed randomized sequence of mixed
// Push/Pop/Peek/Truncate operations, cross-checking every observable against
// a plain-slice reference model.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	var stk Stack[int]
	var model []int

	for op := 0; op < 2000; op++ {
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4: // Push (weighted common)
			v := rng.Intn(100)
			stk.Push(v)
			model = append(model, v)
		case 5, 6, 7: // Pop
			v, err := stk.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyStack) {
					t.Fatalf("op %d: Pop on empty: expected ErrEmptyStack, got %v", op, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: Pop on non-empty: unexpected error %v", op, err)
				}
				if want := model[len(model)-1]; v != want {
					t.Fatalf("op %d: Pop: expected %d, got %d", op, want, v)
				}
				model = model[:len(model)-1]
			}
		case 8: // Peek
			p, err := stk.Peek()
			if len(model) == 0 {
				if p != nil || !errors.Is(err, ErrEmptyStack) {
					t.Fatalf("op %d: Peek on empty: expected (nil, ErrEmptyStack), got (%v, %v)", op, p, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: Peek on non-empty: unexpected error %v", op, err)
				}
				if want := model[len(model)-1]; *p != want {
					t.Fatalf("op %d: Peek: expected %d, got %d", op, want, *p)
				}
			}
		case 9: // Truncate
			stk.Truncate()
			model = model[:0]
		}
		checkModel(t, &stk, model)
	}
}

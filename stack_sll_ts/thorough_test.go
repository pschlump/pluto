package stack_sll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, single-element and
// duplicate edge cases, value-copy semantics of Peek, truncate reuse, and
// a fixed-seed randomized property test.

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// checkModel verifies the stack's observable state against a reference
// model (a plain slice where the top is the last element).
func checkModel(t *testing.T, stk *Stack[int], model []int) {
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
		if top != model[len(model)-1] {
			t.Fatalf("Peek: expected %v, got %v", model[len(model)-1], top)
		}
	}
	// All must iterate top to bottom.
	var fwd []int
	for _, v := range stk.All() {
		fwd = append(fwd, v)
	}
	var wantFwd []int
	for _, m := range slices.Backward(model) {
		wantFwd = append(wantFwd, m)
	}
	if !reflect.DeepEqual(fwd, wantFwd) {
		t.Fatalf("All: expected %v, got %v", wantFwd, fwd)
	}
	// Backward must iterate bottom to top.
	var bwd []int
	for _, v := range stk.Backward() {
		bwd = append(bwd, v)
	}
	if !reflect.DeepEqual(bwd, model) {
		t.Fatalf("Backward: expected %v, got %v", model, bwd)
	}
}

func TestSingleElement(t *testing.T) {
	var stk Stack[string]

	stk.Push("only")
	if stk.IsEmpty() || stk.Length() != 1 {
		t.Errorf("Expected single-element stack, length %d", stk.Length())
	}
	if v, err := stk.Peek(); err != nil || v != "only" {
		t.Errorf("Peek = (%q, %v)", v, err)
	}
	if v, err := stk.Pop(); err != nil || v != "only" {
		t.Errorf("Pop = (%q, %v)", v, err)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping the only element.")
	}
	if stk.head != nil {
		t.Errorf("Expected nil head after draining.")
	}
}

func TestDuplicateValues(t *testing.T) {
	var stk Stack[int]
	for _, v := range []int{5, 5, 5, 7} {
		stk.Push(v)
	}
	for i, want := range []int{7, 5, 5, 5} {
		if v, err := stk.Pop(); err != nil || v != want {
			t.Errorf("Pop step %d = (%v, %v), expected %d", i, v, err, want)
		}
	}
}

// TestPeekReturnsValue verifies that Peek returns an independent value:
// mutating it cannot affect the stack.
func TestPeekReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var stk Stack[item]
	stk.Push(item{S: "a", N: 1})
	stk.Push(item{S: "b", N: 2})

	v, err := stk.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	v.N = 99 // mutate the returned value

	if got, err := stk.Peek(); err != nil || got.N != 2 {
		t.Errorf("Peek after mutation = (%v, %v), expected N=2 unaffected", got, err)
	}
	if stk.Length() != 2 {
		t.Errorf("Expected Peek to leave the length at 2, got %d", stk.Length())
	}
}

// TestPushPopInterleaved cross-checks against the model between every
// operation.
func TestPushPopInterleaved(t *testing.T) {
	var stk Stack[int]
	var model []int

	for _, v := range []int{1, 2, 3} {
		stk.Push(v)
		model = append(model, v)
		checkModel(t, &stk, model)
	}

	if v, err := stk.Pop(); err != nil || v != 3 {
		t.Fatalf("Pop = (%v, %v), expected 3", v, err)
	}
	model = model[:len(model)-1]
	checkModel(t, &stk, model)

	stk.Push(4)
	model = append(model, 4)
	checkModel(t, &stk, model)
}

// TestTruncateReuse verifies that the stack is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	var stk Stack[int]
	for i := range 10 {
		stk.Push(i)
	}
	stk.Truncate()

	if !stk.IsEmpty() || stk.Length() != 0 {
		t.Errorf("Expected empty stack after Truncate.")
	}
	checkModel(t, &stk, nil)

	// Reusable after the drain.
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

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 17))
	const ops = 4000
	const keySpace = 50

	var stk Stack[int]
	var model []int

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(3) {
		case 0, 1: // Push (weighted so the stack grows)
			stk.Push(v)
			model = append(model, v)
		case 2: // Pop
			got, err := stk.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyStack) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[len(model)-1] {
					t.Fatalf("step %d: Pop = (%v, %v), model top %d", step, got, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		}
		if step%50 == 0 {
			checkModel(t, &stk, model)
		}
	}
	checkModel(t, &stk, model)
}

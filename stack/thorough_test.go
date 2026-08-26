package stack

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: single-element and duplicate edge cases, value-copy
// semantics of Peek, iterator index sequences and live reflection, and a
// fixed-seed randomized property test cross-checked against a slice
// reference model.

import (
	"errors"
	"math/rand/v2"
	"reflect"
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
		if top != model[len(model)-1] {
			t.Fatalf("Peek: expected %v, got %v", model[len(model)-1], top)
		}
	}
	// All must iterate top to bottom.
	var fwd []T
	for _, v := range stk.All() {
		fwd = append(fwd, v)
	}
	var wantFwd []T
	for _, m := range slices.Backward(model) {
		wantFwd = append(wantFwd, m)
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
	if stk.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
	checkModel(t, &stk, nil)
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

func TestPushPopInterleaved(t *testing.T) {
	var stk Stack[string]
	var model []string

	ops := []struct {
		push string
	}{
		{push: "a"}, {push: "b"}, {push: "c"},
	}
	for _, op := range ops {
		stk.Push(op.push)
		model = append(model, op.push)
		checkModel(t, &stk, model)
	}

	if v, err := stk.Pop(); err != nil || v != "c" {
		t.Fatalf("Pop = (%q, %v), expected c", v, err)
	}
	model = model[:len(model)-1]
	checkModel(t, &stk, model)

	stk.Push("d")
	model = append(model, "d")
	checkModel(t, &stk, model)
}

func TestBackwardEarlyBreak(t *testing.T) {
	var stk Stack[int]
	for i := range 5 {
		stk.Push(i)
	}

	n := 0
	var first int
	for _, v := range stk.Backward() {
		n++
		first = v
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}
	if first != 0 {
		t.Errorf("Expected first Backward item to be the bottom (0), got %d", first)
	}

	// Take exactly two items, bottom to top.
	var got []int
	for _, v := range stk.Backward() {
		got = append(got, v)
		if len(got) == 2 {
			break
		}
	}
	if expect := []int{0, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Expected [0 1] from partial Backward, got %v", got)
	}
}

// TestIteratorIndexSequences verifies the index numbering: All numbers
// from 0 at the top, Backward from 0 at the bottom.
func TestIteratorIndexSequences(t *testing.T) {
	var stk Stack[int]
	for i := range 4 {
		stk.Push(i) // 0,1,2,3 pushed; top is 3
	}

	var allIdx, allVal []int
	for i, v := range stk.All() {
		allIdx = append(allIdx, i)
		allVal = append(allVal, v)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(allIdx, expect) {
		t.Errorf("All indices: expected %v got %v", expect, allIdx)
	}
	if expect := []int{3, 2, 1, 0}; !reflect.DeepEqual(allVal, expect) {
		t.Errorf("All values: expected %v got %v", expect, allVal)
	}

	var bwdIdx, bwdVal []int
	for i, v := range stk.Backward() {
		bwdIdx = append(bwdIdx, i)
		bwdVal = append(bwdVal, v)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(bwdIdx, expect) {
		t.Errorf("Backward indices: expected %v got %v", expect, bwdIdx)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(bwdVal, expect) {
		t.Errorf("Backward values: expected %v got %v", expect, bwdVal)
	}
}

// TestIteratorReflectsLiveStack verifies that the iterators reflect
// pushes and pops that happen between iterations.
func TestIteratorReflectsLiveStack(t *testing.T) {
	var stk Stack[int]
	stk.Push(1)
	stk.Push(2)

	var seen []int
	for _, v := range stk.All() {
		seen = append(seen, v)
		break // stop after the top element
	}
	if expect := []int{2}; !reflect.DeepEqual(seen, expect) {
		t.Fatalf("First visit: expected %v got %v", expect, seen)
	}

	// Mutate, then iterate again: the change is visible.
	stk.Push(3)
	seen = nil
	for _, v := range stk.All() {
		seen = append(seen, v)
	}
	if expect := []int{3, 2, 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("After push: expected %v got %v", expect, seen)
	}

	if _, err := stk.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	seen = nil
	for _, v := range stk.All() {
		seen = append(seen, v)
	}
	if expect := []int{2, 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("After pop: expected %v got %v", expect, seen)
	}
}

func TestTruncateThenIterate(t *testing.T) {
	var stk Stack[int]
	for i := range 5 {
		stk.Push(i)
	}
	stk.Truncate()

	n := 0
	for range stk.All() {
		n++
	}
	for range stk.Backward() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from iterators on truncated stack, got %d", n)
	}

	// The stack is reusable.
	stk.Push(9)
	if v, err := stk.Pop(); err != nil || v != 9 {
		t.Errorf("Pop after Truncate = (%v, %v), expected 9", v, err)
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 13))
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

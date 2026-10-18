package stack_sll_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization top down, deterministic rendering, order-sensitive
// equality, deep copy, the nil/zero-value read contract, a randomized
// model test against a slice reference, and the concurrency behavior of
// the locked methods (the real assertion is `go test -race`).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
)

// stackOf builds a stack holding items with each pushed in order (so the
// LAST item is the top).
func stackOf(items ...int) *Stack[int] {
	ns := &Stack[int]{}
	for _, v := range items {
		ns.Push(v)
	}
	return ns
}

// stackTopDown builds a stack whose top-down Slice is items (items[0] is
// the top).
func stackTopDown(items []int) *Stack[int] {
	ns := &Stack[int]{}
	for i := len(items) - 1; i >= 0; i-- {
		ns.Push(items[i])
	}
	return ns
}

// bracketOf renders items the way String/StringFunc must: fmt.Sprint per
// element, single spaces, square brackets.
func bracketOf(items ...int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range items {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, v)
	}
	sb.WriteByte(']')
	return sb.String()
}

// checkStackModel verifies the whole capability surface against the
// slice reference model (model holds the stack top down).
func checkStackModel(t *testing.T, ns *Stack[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := ns.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := ns.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	for i := range model {
		if got[i] != model[i] {
			t.Fatalf("%s: Slice[%d] = %d, want %d", where, i, got[i], model[i])
		}
	}
	if s := ns.String(); s != bracketOf(model...) {
		t.Errorf("%s: String = %q, want %q", where, s, bracketOf(model...))
	}
	dup := ns.Clone()
	if !dup.Equal(ns) || !ns.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver", where)
	}
	twin := stackTopDown(model)
	if !ns.Equal(twin) || !twin.Equal(ns) {
		t.Errorf("%s: stack must equal an independently built stack with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	ns := stackOf(1, 2, 3) // 3 is the top

	got := ns.Slice()
	if len(got) != 3 || got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Errorf("Slice = %v, want [3 2 1] top down", got)
	}

	// The slice is a copy: mutating it must not affect the stack, and
	// stack operations must not change the earlier slice.
	got[0] = 99
	if v, _ := ns.Peek(); v != 3 {
		t.Errorf("mutating the Slice result changed the stack: Peek = %d", v)
	}
	_, _ = ns.Pop()
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("stack operations changed the earlier Slice result: %v", got)
	}

	if empty := (&Stack[int]{}).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty stack should be empty non-nil, got %v", empty)
	}
	var nilS *Stack[int]
	if s := nilS.Slice(); s != nil {
		t.Errorf("Slice of a nil stack should be nil, got %v", s)
	}

	// A drained stack (nodes unlinked) yields an empty slice.
	drained := stackOf(1)
	_, _ = drained.Pop()
	if s := drained.Slice(); s == nil || len(s) != 0 {
		t.Errorf("Slice of a drained stack should be empty non-nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed stacks holding the same sequence must
	// render identically.
	a := stackOf(1, 2, 3)
	b := stackOf(1, 2, 3)
	if a.String() != b.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[3 2 1]"; got != want {
		t.Errorf("String = %q, want %q (top down)", got, want)
	}
	if s1, s2 := a.String(), a.String(); s1 != s2 {
		t.Errorf("String must be stable across calls")
	}

	if got := (&Stack[string]{}).String(); got != "[]" {
		t.Errorf("String of an empty stack = %q, want []", got)
	}
	var nilS *Stack[string]
	if got := nilS.String(); got != "[]" {
		t.Errorf("String of a nil stack = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	ns := stackOf(5, 8) // 8 is the top
	got := ns.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#8 #5]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := ns.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := stackOf(1, 2, 3)
	b := stackOf(1, 2, 3)
	c := stackOf(1, 2)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("stacks with the same sequence must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("stacks of different lengths must not be equal")
	}

	// ORDER SENSITIVITY — the sequence contract: same elements pushed in
	// a different order is a different stack.
	x := stackOf(1, 2) // [2 1] top down
	y := stackOf(2, 1) // [1 2] top down
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[2 1] and [1 2] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := stackOf(1, 1, 2)
	yy := stackOf(1, 2, 1)
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("top-down [2 1 1] and [1 2 1] must not be equal")
	}

	empty := &Stack[int]{}
	var nilS *Stack[int]
	if !nilS.Equal(empty) || !empty.Equal(nilS) {
		t.Errorf("nil and empty stacks must be equal, both directions")
	}
	if !nilS.Equal(nilS) {
		t.Errorf("nil must equal nil")
	}
	if nilS.Equal(a) || a.Equal(nilS) {
		t.Errorf("nil/empty must not equal a non-empty stack")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	ns := stackOf(1, 2, 3)

	dup := ns.Clone()
	if !dup.Equal(ns) || !ns.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// Deep copy: mutating the clone never affects the receiver.
	dup.Push(4)
	if got := ns.Slice(); fmt.Sprint(got) != "[3 2 1]" {
		t.Errorf("pushing to the clone changed the receiver: %v", got)
	}

	// And mutating the receiver never affects the clone.
	_, _ = ns.Pop()
	if got := dup.Slice(); fmt.Sprint(got) != "[4 3 2 1]" {
		t.Errorf("popping the receiver changed the clone: %v", got)
	}

	if dup := (&Stack[int]{}).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty stack should be a fresh empty stack")
	}
	var nilS *Stack[int]
	if dup := nilS.Clone(); dup != nil {
		t.Errorf("Clone of a nil stack should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := stackOf(7)
	for i, ss := range []*Stack[int]{nil, {}} {
		where := fmt.Sprintf("container %d", i)
		if got := ss.Len(); got != 0 {
			t.Errorf("%s: Len = %d, want 0", where, got)
		}
		if got := ss.String(); got != "[]" {
			t.Errorf("%s: String = %q, want []", where, got)
		}
		if got := ss.StringFunc(func(v int) string { return "x" }); got != "[]" {
			t.Errorf("%s: StringFunc = %q, want []", where, got)
		}
		other := &Stack[int]{}
		if !ss.Equal(other) || !other.Equal(ss) {
			t.Errorf("%s: must equal an empty stack, both directions", where)
		}
		if ss.Equal(nonEmpty) || nonEmpty.Equal(ss) {
			t.Errorf("%s: must not equal a non-empty stack", where)
		}
	}
	var nilS *Stack[int]
	if got := nilS.Slice(); got != nil {
		t.Errorf("nil stack Slice = %v, want nil", got)
	}
	if dup := nilS.Clone(); dup != nil {
		t.Errorf("nil stack Clone = %v, want nil", dup)
	}
	if dup := (&Stack[int]{}).Clone(); dup == nil || !dup.Equal(&Stack[int]{}) {
		t.Errorf("zero-value Clone should be a fresh empty stack")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	ns := &Stack[int]{}
	var model []int // top down: model[0] is the top
	for step := 0; step < 800; step++ {
		switch rng.IntN(4) {
		case 0, 1: // push
			v := rng.IntN(50)
			ns.Push(v)
			model = append([]int{v}, model...)
		case 2: // pop
			if len(model) > 0 {
				v, err := ns.Pop()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: Pop = %d,%v; want %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // occasional truncate
			if rng.IntN(8) == 0 {
				ns.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkStackModel(t, ns, model, step)
		}
	}
	checkStackModel(t, ns, model, -1)

	// The final stack must not equal a rotation of itself (order counts).
	if len(model) > 1 {
		rotated := stackTopDown(append(append([]int{}, model[1:]...), model[0]))
		if ns.Equal(rotated) || rotated.Equal(ns) {
			t.Errorf("a rotated stack must not be equal to the original")
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency (the _ts additions) — `go test -race` is the real
// assertion: no data race and no deadlock.
// ---------------------------------------------------------------------------

func TestCapabilitiesConcurrent(t *testing.T) {
	ns := &Stack[int]{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (w + i) % 8 {
				case 0, 1: // writes
					ns.Push(i)
				case 2:
					_, _ = ns.Pop()
				case 3: // reads
					ns.Len()
					ns.IsEmpty()
					_, _ = ns.Peek()
				case 4: // capability snapshot reads
					ns.Slice()
				case 5:
					_ = ns.String()
				case 6:
					ns.Clone().Len()
				case 7:
					ns.StringFunc(func(v int) string { return fmt.Sprint(v) })
				}
			}
		}(w)
	}
	wg.Wait()
	if ns.Len() == 0 {
		t.Errorf("after the hammer the stack should hold elements, Len = %d", ns.Len())
	}
	for {
		if _, err := ns.Pop(); err != nil {
			break
		}
	}
	if ns.Len() != 0 {
		t.Errorf("final Len = %d, the stack should be empty after draining", ns.Len())
	}
}

func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := &Stack[int]{}
	b := &Stack[int]{}
	for i := 0; i < 500; i++ {
		a.Push(i)
		b.Push(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
	if !a.Equal(b) {
		t.Errorf("identical stacks must be equal")
	}
}

package bag_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in insertion order, deterministic rendering,
// order-sensitive equality, deep copy, the nil/zero-value read contract,
// a randomized model test against a slice reference, and the concurrency
// behavior of the locked methods (the real assertion is `go test
// -race`).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
)

// bagOf builds a bag holding items in the order given (insertion order).
func bagOf(items ...int) *Bag[int] {
	b := &Bag[int]{}
	for _, v := range items {
		b.Add(v)
	}
	return b
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

// checkBagModel verifies the whole capability surface against the slice
// reference model (model is the insertion order).
func checkBagModel(t *testing.T, b *Bag[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := b.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := b.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	for i := range model {
		if got[i] != model[i] {
			t.Fatalf("%s: Slice[%d] = %d, want %d", where, i, got[i], model[i])
		}
	}
	if s := b.String(); s != bracketOf(model...) {
		t.Errorf("%s: String = %q, want %q", where, s, bracketOf(model...))
	}
	dup := b.Clone()
	if !dup.Equal(b) || !b.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver", where)
	}
	twin := bagOf(model...)
	if !b.Equal(twin) || !twin.Equal(b) {
		t.Errorf("%s: bag must equal an independently built bag with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	b := bagOf(1, 2, 3)

	got := b.Slice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Slice = %v, want [1 2 3] in insertion order", got)
	}

	// The slice is a copy: mutating it must not affect the bag, and bag
	// operations must not change the earlier slice.
	got[0] = 99
	if b.Slice()[0] != 1 {
		t.Errorf("mutating the Slice result changed the bag: %v", b.Slice())
	}
	b.Add(4)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("bag operations changed the earlier Slice result: %v", got)
	}

	if empty := (&Bag[int]{}).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty bag should be empty non-nil, got %v", empty)
	}
	var nilB *Bag[int]
	if s := nilB.Slice(); s != nil {
		t.Errorf("Slice of a nil bag should be nil, got %v", s)
	}

	// A truncated bag (nodes released) yields an empty slice.
	trunc := bagOf(1)
	trunc.Truncate()
	if s := trunc.Slice(); s == nil || len(s) != 0 {
		t.Errorf("Slice of a truncated bag should be empty non-nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed bags holding the same sequence must
	// render identically.
	a := bagOf(1, 2, 3)
	c := bagOf(1, 2, 3)
	if a.String() != c.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), c.String())
	}
	if got, want := a.String(), "[1 2 3]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if s1, s2 := a.String(), a.String(); s1 != s2 {
		t.Errorf("String must be stable across calls")
	}

	if got := (&Bag[string]{}).String(); got != "[]" {
		t.Errorf("String of an empty bag = %q, want []", got)
	}
	var nilB *Bag[string]
	if got := nilB.String(); got != "[]" {
		t.Errorf("String of a nil bag = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	b := bagOf(5, 8)
	got := b.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := b.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := bagOf(1, 2, 3)
	b := bagOf(1, 2, 3)
	c := bagOf(1, 2)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("bags with the same sequence must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("bags of different lengths must not be equal")
	}

	// ORDER SENSITIVITY — the sequence contract: the same multiset added
	// in a different order is a different bag.
	x := bagOf(1, 2)
	y := bagOf(2, 1)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 2] and [2 1] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := bagOf(1, 1, 2)
	yy := bagOf(1, 2, 1)
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("[1 1 2] and [1 2 1] must not be equal")
	}

	empty := &Bag[int]{}
	var nilB *Bag[int]
	if !nilB.Equal(empty) || !empty.Equal(nilB) {
		t.Errorf("nil and empty bags must be equal, both directions")
	}
	if !nilB.Equal(nilB) {
		t.Errorf("nil must equal nil")
	}
	if nilB.Equal(a) || a.Equal(nilB) {
		t.Errorf("nil/empty must not equal a non-empty bag")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	b := bagOf(1, 2, 3)

	dup := b.Clone()
	if !dup.Equal(b) || !b.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// Deep copy: mutating the clone never affects the receiver.
	dup.Add(4)
	if got := b.Slice(); fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("adding to the clone changed the receiver: %v", got)
	}

	// And mutating the receiver never affects the clone.
	b.Truncate()
	if got := dup.Slice(); fmt.Sprint(got) != "[1 2 3 4]" {
		t.Errorf("truncating the receiver changed the clone: %v", got)
	}

	if dup := (&Bag[int]{}).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty bag should be a fresh empty bag")
	}
	var nilB *Bag[int]
	if dup := nilB.Clone(); dup != nil {
		t.Errorf("Clone of a nil bag should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := bagOf(7)
	for i, bb := range []*Bag[int]{nil, {}} {
		where := fmt.Sprintf("container %d", i)
		if got := bb.Len(); got != 0 {
			t.Errorf("%s: Len = %d, want 0", where, got)
		}
		if got := bb.String(); got != "[]" {
			t.Errorf("%s: String = %q, want []", where, got)
		}
		if got := bb.StringFunc(func(v int) string { return "x" }); got != "[]" {
			t.Errorf("%s: StringFunc = %q, want []", where, got)
		}
		other := &Bag[int]{}
		if !bb.Equal(other) || !other.Equal(bb) {
			t.Errorf("%s: must equal an empty bag, both directions", where)
		}
		if bb.Equal(nonEmpty) || nonEmpty.Equal(bb) {
			t.Errorf("%s: must not equal a non-empty bag", where)
		}
	}
	var nilB *Bag[int]
	if got := nilB.Slice(); got != nil {
		t.Errorf("nil bag Slice = %v, want nil", got)
	}
	if dup := nilB.Clone(); dup != nil {
		t.Errorf("nil bag Clone = %v, want nil", dup)
	}
	if dup := (&Bag[int]{}).Clone(); dup == nil || !dup.Equal(&Bag[int]{}) {
		t.Errorf("zero-value Clone should be a fresh empty bag")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	b := &Bag[int]{}
	var model []int // insertion order
	for step := 0; step < 800; step++ {
		switch rng.IntN(3) {
		case 0, 1: // add
			v := rng.IntN(50)
			b.Add(v)
			model = append(model, v)
		case 2: // occasional truncate
			if rng.IntN(8) == 0 {
				b.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkBagModel(t, b, model, step)
		}
	}
	checkBagModel(t, b, model, -1)

	// The final bag must not equal a rotation of itself (order counts) —
	// unless every element is identical, when the rotation is the same
	// sequence.
	if len(model) > 1 {
		rotated := append(append([]int{}, model[len(model)-1]), model[:len(model)-1]...)
		if !slices.Equal(model, rotated) {
			rbag := bagOf(rotated...)
			if b.Equal(rbag) || rbag.Equal(b) {
				t.Errorf("a rotated bag must not be equal to the original")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency (the _ts additions) — `go test -race` is the real
// assertion: no data race and no deadlock.
// ---------------------------------------------------------------------------

func TestCapabilitiesConcurrent(t *testing.T) {
	b := &Bag[int]{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (w + i) % 8 {
				case 0, 1: // writes (a bag is add-only)
					b.Add(i)
				case 2: // reads
					b.Len()
					b.IsEmpty()
				case 3: // capability snapshot reads
					b.Slice()
				case 4:
					_ = b.String()
				case 5:
					b.Clone().Len()
				case 6:
					b.StringFunc(func(v int) string { return fmt.Sprint(v) })
				case 7:
					b.Backward()
				}
			}
		}(w)
	}
	wg.Wait()
	if b.Len() == 0 {
		t.Errorf("after the hammer the bag should hold elements, Len = %d", b.Len())
	}
	b.Truncate()
	if b.Len() != 0 {
		t.Errorf("final Len = %d, the bag should be empty after truncating", b.Len())
	}
}

func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := &Bag[int]{}
	c := &Bag[int]{}
	for i := 0; i < 500; i++ {
		a.Add(i)
		c.Add(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(c) }()
	go func() { defer wg.Done(); _ = c.Equal(a) }()
	wg.Wait()
	if !a.Equal(c) {
		t.Errorf("identical bags must be equal")
	}
}

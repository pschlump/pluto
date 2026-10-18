package heap

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in breadth-first backing order, rendering, MULTSET
// equality (same multiset, different arrangements, must be equal), deep
// copy preserving the exact layout, the nil/zero-value read contract,
// and a randomized model test against a sorted-slice reference.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// capHeapOf builds a min-heap of ints by pushing items in order.
func capHeapOf(items ...int) *Heap[int] {
	hp := NewHeap[int]()
	for _, v := range items {
		hp.Push(v)
	}
	return hp
}

// capBracketOf renders items the way String must when the heap's backing
// order equals items: fmt.Sprint per element, single spaces, square
// brackets.
func capBracketOf(items ...int) string {
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

// capSorted returns a sorted copy of items.
func capSorted(items []int) []int {
	out := slices.Clone(items)
	slices.Sort(out)
	return out
}

// checkCapModel verifies the whole capability surface against the
// multiset reference model.
func checkCapModel(t *testing.T, hp *Heap[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := hp.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := hp.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	if want := capSorted(model); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice multiset = %v, want %v", where, capSorted(got), want)
	}
	// String must render exactly the Slice order (backing order).
	if s, want := hp.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := hp.Clone()
	if !dup.Equal(hp) || !hp.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (multiset)", where)
	}
	twin := capHeapOf(model...)
	if !hp.Equal(twin) || !twin.Equal(hp) {
		t.Errorf("%s: must equal an independently built heap over the same multiset", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// The backing order is breadth-first tree order, deterministic for a
	// given insertion sequence: push 3, 1, 2 -> sift -> [1, 3, 2].
	hp := capHeapOf(3, 1, 2)
	got := hp.Slice()
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 2 {
		t.Errorf("Slice = %v, want [1 3 2] (breadth-first backing order)", got)
	}
	checkHeapInvariant(t, hp, Compare[int])

	// The slice is a copy: mutating it must not affect the heap, and
	// heap operations must not change the earlier slice.
	got[0] = 99
	if v, found := hp.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the heap: Peek = %d", v)
	}
	hp.Push(0)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("heap operations changed the earlier Slice result: %v", got)
	}
	checkHeapInvariant(t, hp, Compare[int])

	if empty := NewHeap[int]().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty heap should be empty non-nil, got %v", empty)
	}
	var nilH *Heap[int]
	if s := nilH.Slice(); s != nil {
		t.Errorf("Slice of a nil heap should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (backing) order, so two heaps
	// built by the same insertion sequence render identically.
	a := capHeapOf(5, 3, 1, 4, 2)
	b := capHeapOf(5, 3, 1, 4, 2)
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewHeap[string]().String(); got != "[]" {
		t.Errorf("String of an empty heap = %q, want []", got)
	}
	var nilH *Heap[string]
	if got := nilH.String(); got != "[]" {
		t.Errorf("String of a nil heap = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	hp := capHeapOf(1, 3, 2)
	got := hp.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	// Expected: the Slice (backing) order, each element rendered by the
	// format function.
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range hp.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "#%d", v)
	}
	sb.WriteByte(']')
	if got != sb.String() {
		t.Errorf("StringFunc = %q, want %q", got, sb.String())
	}
	if got := hp.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// THE MULTiset CONTRACT: same multiset, different insertion orders —
	// different internal arrangements, still equal.
	a := capHeapOf(1, 2, 3, 4, 5)
	b := capHeapOf(5, 4, 3, 2, 1)
	c := capHeapOf(3, 1, 4, 5, 2)
	if !a.Equal(b) || !b.Equal(a) || !a.Equal(c) || !c.Equal(a) {
		t.Errorf("heaps over the same multiset must be equal regardless of insertion order")
	}
	// ... and their arrangements genuinely differ (Equal must not depend
	// on them): ascending pushes keep the array sorted, descending pushes
	// sift everything below the root.
	if slices.Equal(a.Slice(), b.Slice()) {
		t.Errorf("expected different backing arrangements for the two insertion orders: %v", a.Slice())
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths are never equal.
	if a.Equal(capHeapOf(1, 2, 3, 4)) || capHeapOf(1, 2, 3, 4).Equal(a) {
		t.Errorf("heaps of different lengths must not be equal")
	}
	// Same length, different multisets ([1 1 2] vs [1 2 2]).
	x := capHeapOf(1, 1, 2)
	y := capHeapOf(1, 2, 2)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 1 2] and [1 2 2] are different multisets and must not be equal")
	}

	// Ties: heavy-duplicate multisets, inserted in different orders.
	p := capHeapOf(7, 7, 7, 2, 2, 9)
	q := capHeapOf(2, 9, 7, 2, 7, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-heavy heaps over the same multiset must be equal")
	}

	// A max-heap (reversed comparator) compares consistently too.
	rmax := NewHeapFunc[int](func(a, b int) int { return Compare(b, a) })
	smax := NewHeapFunc[int](func(a, b int) int { return Compare(b, a) })
	for _, v := range []int{4, 1, 3} {
		rmax.Push(v)
	}
	for _, v := range []int{3, 4, 1} {
		smax.Push(v)
	}
	if !rmax.Equal(smax) || !smax.Equal(rmax) {
		t.Errorf("reversed-comparator heaps over the same multiset must be equal")
	}

	// Equal is non-destructive: the receiver keeps its arrangement.
	before := a.Slice()
	_ = a.Equal(b)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}
	checkHeapInvariant(t, a, Compare[int])

	empty := NewHeap[int]()
	var nilH *Heap[int]
	if !nilH.Equal(empty) || !empty.Equal(nilH) {
		t.Errorf("nil and empty heaps must be equal, both directions")
	}
	if !nilH.Equal(nilH) {
		t.Errorf("nil must equal nil")
	}
	if nilH.Equal(a) || a.Equal(nilH) {
		t.Errorf("nil/empty must not equal a non-empty heap")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	hp := capHeapOf(5, 3, 1, 4, 2)

	dup := hp.Clone()
	if !dup.Equal(hp) || !hp.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkHeapInvariant(t, dup, Compare[int])

	// The clone preserves the exact backing layout (the array is copied).
	orig, cloned := hp.Slice(), dup.Slice()
	if !slices.Equal(orig, cloned) {
		t.Errorf("clone must preserve the backing order: %v vs %v", orig, cloned)
	}

	// Deep copy: mutating the clone never affects the receiver, and vice
	// versa — including the pop sequence.
	dup.Push(0)
	if got := hp.Len(); got != 5 {
		t.Errorf("pushing to the clone changed the receiver: Len = %d, want 5", got)
	}
	hp.Pop()
	if got := dup.Len(); got != 6 {
		t.Errorf("popping the receiver changed the clone: Len = %d, want 6", got)
	}

	if dup := NewHeap[int]().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty heap should be a fresh empty heap")
	}
	var nilH *Heap[int]
	if dup := nilH.Clone(); dup != nil {
		t.Errorf("Clone of a nil heap should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capHeapOf(3)
	for i, hh := range []*Heap[int]{nil, {}} {
		where := fmt.Sprintf("container %d", i)
		if got := hh.Len(); got != 0 {
			t.Errorf("%s: Len = %d, want 0", where, got)
		}
		if got := hh.String(); got != "[]" {
			t.Errorf("%s: String = %q, want []", where, got)
		}
		if got := hh.StringFunc(func(v int) string { return "x" }); got != "[]" {
			t.Errorf("%s: StringFunc = %q, want []", where, got)
		}
		other := NewHeap[int]()
		if !hh.Equal(other) || !other.Equal(hh) {
			t.Errorf("%s: must equal an empty heap, both directions", where)
		}
		if hh.Equal(nonEmpty) || nonEmpty.Equal(hh) {
			t.Errorf("%s: must not equal a non-empty heap", where)
		}
	}
	var nilH *Heap[int]
	if got := nilH.Slice(); got != nil {
		t.Errorf("nil heap Slice = %v, want nil", got)
	}
	if dup := nilH.Clone(); dup != nil {
		t.Errorf("nil heap Clone = %v, want nil", dup)
	}
	if dup := (&Heap[int]{}).Clone(); dup == nil || !dup.Equal(NewHeap[int]()) {
		t.Errorf("zero-value Clone should be a fresh empty heap")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	hp := NewHeap[int]()
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1: // push
			v := rng.IntN(20)
			hp.Push(v)
			model = append(model, v)
		case 2, 3: // pop the minimum
			if len(model) > 0 {
				v, found := hp.Pop()
				if !found {
					t.Fatalf("step %d: Pop not found, model has %d", step, len(model))
				}
				want := capSorted(model)[0]
				if v != want {
					t.Fatalf("step %d: Pop = %d, want %d", step, v, want)
				}
				model = slices.Delete(model, slices.Index(model, want), slices.Index(model, want)+1)
			}
		case 4: // peek
			if len(model) > 0 {
				if v, found := hp.Peek(); !found || v != capSorted(model)[0] {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 5: // occasional truncate
			if rng.IntN(8) == 0 {
				hp.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkHeapInvariant(t, hp, Compare[int])
			checkCapModel(t, hp, model, step)
		}
	}
	checkHeapInvariant(t, hp, Compare[int])
	checkCapModel(t, hp, model, -1)

	// A heap built from the final multiset in a SHUFFLED order must be
	// equal (multiset equality, the §4.5 contract).
	if len(model) > 0 {
		shuffled := slices.Clone(model)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		twin := capHeapOf(shuffled...)
		if !hp.Equal(twin) || !twin.Equal(hp) {
			t.Errorf("a shuffled-order rebuild of the same multiset must be equal")
		}
	}
}

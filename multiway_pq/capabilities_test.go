package multiway_pq

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in breadth-first backing order, rendering, MULTiset
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

// capDHeapOf builds a 4-ary min-heap of ints by pushing items in order.
func capDHeapOf(items ...int) *DHeap[int] {
	dh := NewDHeap[int](4)
	for _, v := range items {
		dh.Push(v)
	}
	return dh
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
func checkCapModel(t *testing.T, dh *DHeap[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := dh.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := dh.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	if want := capSorted(model); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice multiset = %v, want %v", where, capSorted(got), want)
	}
	// String must render exactly the Slice order (backing order).
	if s, want := dh.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := dh.Clone()
	if !dup.Equal(dh) || !dh.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (multiset)", where)
	}
	twin := capDHeapOf(model...)
	if !dh.Equal(twin) || !twin.Equal(dh) {
		t.Errorf("%s: must equal an independently built heap over the same multiset", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// The backing order is breadth-first tree order, deterministic for a
	// given insertion sequence: push 3, 1, 2 -> sift -> [1, 3, 2].
	dh := capDHeapOf(3, 1, 2)
	got := dh.Slice()
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 2 {
		t.Errorf("Slice = %v, want [1 3 2] (breadth-first backing order)", got)
	}
	checkHeapInvariant(t, dh, Compare[int])

	// The slice is a copy: mutating it must not affect the heap, and
	// heap operations must not change the earlier slice.
	got[0] = 99
	if v, found := dh.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the heap: Peek = %d", v)
	}
	dh.Push(0)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("heap operations changed the earlier Slice result: %v", got)
	}
	checkHeapInvariant(t, dh, Compare[int])

	if empty := NewDHeap[int](2).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty heap should be empty non-nil, got %v", empty)
	}
	var nilH *DHeap[int]
	if s := nilH.Slice(); s != nil {
		t.Errorf("Slice of a nil heap should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (backing) order, so two heaps
	// built by the same insertion sequence render identically.
	a := capDHeapOf(5, 3, 1, 4, 2)
	b := capDHeapOf(5, 3, 1, 4, 2)
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewDHeap[string](4).String(); got != "[]" {
		t.Errorf("String of an empty heap = %q, want []", got)
	}
	var nilH *DHeap[string]
	if got := nilH.String(); got != "[]" {
		t.Errorf("String of a nil heap = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	dh := capDHeapOf(1, 3, 2)
	got := dh.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	// Expected: the Slice (backing) order, each element rendered by the
	// format function.
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range dh.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "#%d", v)
	}
	sb.WriteByte(']')
	if got != sb.String() {
		t.Errorf("StringFunc = %q, want %q", got, sb.String())
	}
	if got := dh.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// THE MULTISET CONTRACT: same multiset, different insertion orders —
	// different internal arrangements, still equal.
	a := capDHeapOf(1, 2, 3, 4, 5)
	b := capDHeapOf(5, 4, 3, 2, 1)
	c := capDHeapOf(3, 1, 4, 5, 2)
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
	if a.Equal(capDHeapOf(1, 2, 3, 4)) || capDHeapOf(1, 2, 3, 4).Equal(a) {
		t.Errorf("heaps of different lengths must not be equal")
	}
	// Same length, different multisets ([1 1 2] vs [1 2 2]).
	x := capDHeapOf(1, 1, 2)
	y := capDHeapOf(1, 2, 2)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 1 2] and [1 2 2] are different multisets and must not be equal")
	}

	// Ties: heavy-duplicate multisets, inserted in different orders.
	p := capDHeapOf(7, 7, 7, 2, 2, 9)
	q := capDHeapOf(2, 9, 7, 2, 7, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-heavy heaps over the same multiset must be equal")
	}

	// Heaps of different branching factors over the same multiset are
	// equal too — d shapes the tree, not the content.
	bin := NewDHeap[int](2)
	oct := NewDHeap[int](8)
	for _, v := range []int{4, 1, 3, 5, 2} {
		bin.Push(v)
		oct.Push(v)
	}
	if !bin.Equal(oct) || !oct.Equal(bin) {
		t.Errorf("d=2 and d=8 heaps over the same multiset must be equal")
	}

	// A max-heap (reversed comparator) compares consistently too.
	rmax := NewDHeapFunc[int](4, func(a, b int) int { return Compare(b, a) })
	smax := NewDHeapFunc[int](4, func(a, b int) int { return Compare(b, a) })
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

	empty := NewDHeap[int](4)
	var nilH *DHeap[int]
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
	dh := capDHeapOf(5, 3, 1, 4, 2)

	dup := dh.Clone()
	if !dup.Equal(dh) || !dh.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkHeapInvariant(t, dup, Compare[int])

	// The clone preserves the exact backing layout (the array is copied)
	// and the branching factor.
	orig, cloned := dh.Slice(), dup.Slice()
	if !slices.Equal(orig, cloned) {
		t.Errorf("clone must preserve the backing order: %v vs %v", orig, cloned)
	}
	if dh.D() != dup.D() {
		t.Errorf("clone must keep the branching factor: D = %d, want %d", dup.D(), dh.D())
	}

	// Deep copy: mutating the clone never affects the receiver, and vice
	// versa.
	dup.Push(0)
	if got := dh.Len(); got != 5 {
		t.Errorf("pushing to the clone changed the receiver: Len = %d, want 5", got)
	}
	dh.Pop()
	if got := dup.Len(); got != 6 {
		t.Errorf("popping the receiver changed the clone: Len = %d, want 6", got)
	}

	if dup := NewDHeap[int](4).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty heap should be a fresh empty heap")
	}
	var nilH *DHeap[int]
	if dup := nilH.Clone(); dup != nil {
		t.Errorf("Clone of a nil heap should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capDHeapOf(3)
	for i, hh := range []*DHeap[int]{nil, {}} {
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
		other := NewDHeap[int](4)
		if !hh.Equal(other) || !other.Equal(hh) {
			t.Errorf("%s: must equal an empty heap, both directions", where)
		}
		if hh.Equal(nonEmpty) || nonEmpty.Equal(hh) {
			t.Errorf("%s: must not equal a non-empty heap", where)
		}
	}
	// The zero value's Slice is empty non-nil; the nil heap's is nil.
	if got := (&DHeap[int]{}).Slice(); got == nil || len(got) != 0 {
		t.Errorf("zero-value Slice should be empty non-nil, got %v", got)
	}
	var nilH *DHeap[int]
	if got := nilH.Slice(); got != nil {
		t.Errorf("nil heap Slice = %v, want nil", got)
	}
	if dup := nilH.Clone(); dup != nil {
		t.Errorf("nil heap Clone = %v, want nil", dup)
	}
	if dup := (&DHeap[int]{}).Clone(); dup == nil || !dup.Equal(NewDHeap[int](2)) {
		t.Errorf("zero-value Clone should be a fresh empty heap")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	dh := NewDHeap[int](4)
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(7) {
		case 0, 1: // push
			v := rng.IntN(20)
			dh.Push(v)
			model = append(model, v)
		case 2, 3: // pop the minimum
			if len(model) > 0 {
				v, found := dh.Pop()
				if !found {
					t.Fatalf("step %d: Pop not found, model has %d", step, len(model))
				}
				want := capSorted(model)[0]
				if v != want {
					t.Fatalf("step %d: Pop = %d, want %d", step, v, want)
				}
				model = slices.Delete(model, slices.Index(model, want), slices.Index(model, want)+1)
			}
		case 4: // delete a searched value (an arbitrary member, not the min)
			if len(model) > 0 {
				target := model[rng.IntN(len(model))]
				_, pos, found := dh.Search(target)
				if !found {
					t.Fatalf("step %d: Search(%d) not found, model has it", step, target)
				}
				got, ok := dh.Delete(pos)
				if !ok {
					t.Fatalf("step %d: Delete(%d) reported false", step, pos)
				}
				if got != target {
					t.Fatalf("step %d: Delete = %d, want %d", step, got, target)
				}
				model = slices.Delete(model, slices.Index(model, target), slices.Index(model, target)+1)
			}
		case 5: // peek
			if len(model) > 0 {
				if v, found := dh.Peek(); !found || v != capSorted(model)[0] {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 6: // occasional truncate
			if rng.IntN(8) == 0 {
				dh.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkHeapInvariant(t, dh, Compare[int])
			checkCapModel(t, dh, model, step)
		}
	}
	checkHeapInvariant(t, dh, Compare[int])
	checkCapModel(t, dh, model, -1)

	// A heap built from the final multiset in a SHUFFLED order must be
	// equal (multiset equality, the §4.5 contract).
	if len(model) > 0 {
		shuffled := slices.Clone(model)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		twin := capDHeapOf(shuffled...)
		if !dh.Equal(twin) || !twin.Equal(dh) {
			t.Errorf("a shuffled-order rebuild of the same multiset must be equal")
		}
	}
}

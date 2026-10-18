package index_pq

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in BACKING order (the breadth-first index-heap order,
// not priority order), rendering, BINDING equality (Equal compares the
// index→value bindings — a value swap between indices changes the
// content), deep copy, the nil/zero-value read contract, and a
// randomized model test against a map reference.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// capIPQFromModel builds a queue over n indices holding exactly the
// model's bindings, inserting in map iteration order (deliberately not
// deterministic — binding equality must not depend on the build order).
func capIPQFromModel(model map[int]int, n int) *IndexPQ[int] {
	q := NewIndexPQ[int](n)
	for k, v := range model {
		q.Insert(k, v)
	}
	return q
}

// capValues returns the model's values.
func capValues(model map[int]int) []int {
	out := make([]int, 0, len(model))
	for _, v := range model {
		out = append(out, v)
	}
	return out
}

// capMin returns the minimum value in the model.
func capMin(model map[int]int) int {
	min := 0
	first := true
	for _, v := range model {
		if first || v < min {
			min = v
			first = false
		}
	}
	return min
}

// capBracketOf renders items the way String must: fmt.Sprint per
// element, single spaces, square brackets.
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
// binding reference model.
func checkCapModel(t *testing.T, q *IndexPQ[int], model map[int]int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := q.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := q.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	if want := capSorted(capValues(model)); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice multiset = %v, want %v", where, capSorted(got), want)
	}
	if s, want := q.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	// All yields exactly the model's bindings, in priority order.
	drained := map[int]int{}
	for k, v := range q.All() {
		drained[k] = v
	}
	if !maps.Equal(drained, model) {
		t.Errorf("%s: All bindings = %v, want %v", where, drained, model)
	}
	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (bindings)", where)
	}
	twin := capIPQFromModel(model, 25)
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: must equal an independently built queue over the same bindings", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// Backing order: insert (0,5) (1,3) (2,4) — index 1 swims above
	// index 0, index 2 stays — so the index heap is [1 0 2] and Slice
	// yields the values in that order: [3 5 4].  NOT priority order.
	q := NewIndexPQ[int](6)
	q.Insert(0, 5)
	q.Insert(1, 3)
	q.Insert(2, 4)
	got := q.Slice()
	if want := []int{3, 5, 4}; !slices.Equal(got, want) {
		t.Errorf("Slice = %v, want %v (backing order)", got, want)
	}
	if slices.Equal(got, capSorted(got)) {
		t.Errorf("Slice should not be priority order here: %v", got)
	}

	// The slice is a copy: mutating it must not affect the queue.
	got[0] = 99
	if v, found := q.Value(1); !found || v != 3 {
		t.Errorf("mutating the Slice result changed the queue: Value(1) = %d", v)
	}

	if empty := NewIndexPQ[int](3).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *IndexPQ[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (backing) order: the same
	// bindings inserted in the same order render identically.
	build := func() *IndexPQ[int] {
		q := NewIndexPQ[int](6)
		q.Insert(0, 5)
		q.Insert(1, 3)
		q.Insert(2, 4)
		q.Insert(3, 1)
		return q
	}
	a, b := build(), build()
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewIndexPQ[string](3).String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *IndexPQ[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := NewIndexPQ[int](3)
	q.Insert(0, 5)
	q.Insert(1, 3)
	q.Insert(2, 4)
	got := q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	// Expected: the Slice (backing) order, each value rendered by the
	// format function.
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range q.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "#%d", v)
	}
	sb.WriteByte(']')
	if got != sb.String() {
		t.Errorf("StringFunc = %q, want %q", got, sb.String())
	}
	if got := q.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// THE BINDING CONTRACT: same bindings, different insertion orders —
	// still equal.
	a := NewIndexPQ[int](6)
	a.Insert(0, 5)
	a.Insert(1, 3)
	a.Insert(2, 4)
	b := NewIndexPQ[int](6)
	b.Insert(2, 4)
	b.Insert(1, 3)
	b.Insert(0, 5)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("queues over the same bindings must be equal regardless of insertion order")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// THE MAP-SHAPED CONTRACT: swapping the values of two indices keeps
	// the value multiset but CHANGES the content — not equal.
	s1 := NewIndexPQ[string](2)
	s1.Insert(0, "a")
	s1.Insert(1, "b")
	s2 := NewIndexPQ[string](2)
	s2.Insert(0, "b")
	s2.Insert(1, "a")
	if s1.Equal(s2) || s2.Equal(s1) {
		t.Errorf("a value swap between indices changes the bindings — must NOT be equal (multiset is the same)")
	}

	// Different lengths are never equal.
	c := NewIndexPQ[int](6)
	c.Insert(0, 5)
	c.Insert(1, 3)
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}
	// Same length, same indices, one different value.
	d := NewIndexPQ[int](6)
	d.Insert(0, 5)
	d.Insert(1, 3)
	d.Insert(2, 9)
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("a different value at the same index must not be equal")
	}

	// Ties: duplicate values bound to the same indices, inserted in
	// different orders.
	p := NewIndexPQ[int](3)
	p.Insert(0, 7)
	p.Insert(1, 7)
	p.Insert(2, 7)
	q := NewIndexPQ[int](3)
	q.Insert(2, 7)
	q.Insert(0, 7)
	q.Insert(1, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-valued queues over the same bindings must be equal")
	}

	// The index space is capacity, not content: different n, same
	// bindings — still equal.
	small := NewIndexPQ[int](4)
	small.Insert(1, 2)
	small.Insert(3, 4)
	big := NewIndexPQ[int](1000)
	big.Insert(3, 4)
	big.Insert(1, 2)
	if !small.Equal(big) || !big.Equal(small) {
		t.Errorf("queues over different index spaces but the same bindings must be equal")
	}

	// A max-first queue (reversed comparator) compares consistently too.
	rev := func(a, b int) int { return cmp.Compare(b, a) }
	rmax := NewIndexPQFunc[int](3, rev)
	rmax2 := NewIndexPQFunc[int](3, rev)
	for i, v := range []int{4, 1, 3} {
		rmax.Insert(i, v)
	}
	for _, kv := range [][2]int{{2, 3}, {1, 1}, {0, 4}} { // same bindings, reversed insertion order
		rmax2.Insert(kv[0], kv[1])
	}
	if !rmax.Equal(rmax2) || !rmax2.Equal(rmax) {
		t.Errorf("reversed-comparator queues over the same bindings must be equal")
	}

	// Equal is non-destructive (reads only).
	before := a.Slice()
	_ = a.Equal(b)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}

	empty := NewIndexPQ[int](3)
	var nilQ *IndexPQ[int]
	if !nilQ.Equal(empty) || !empty.Equal(nilQ) {
		t.Errorf("nil and empty queues must be equal, both directions")
	}
	if !nilQ.Equal(nilQ) {
		t.Errorf("nil must equal nil")
	}
	if nilQ.Equal(a) || a.Equal(nilQ) {
		t.Errorf("nil/empty must not equal a non-empty queue")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	q := NewIndexPQ[int](6)
	q.Insert(0, 5)
	q.Insert(1, 3)
	q.Insert(2, 4)

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// Deep copy, both directions: changing a binding in the clone never
	// affects the receiver, and vice versa.
	dup.Change(0, 99)
	if v, found := q.Value(0); !found || v != 5 {
		t.Errorf("changing the clone changed the receiver: Value(0) = %d", v)
	}
	q.Insert(3, 7)
	if got := dup.Len(); got != 3 {
		t.Errorf("inserting into the receiver changed the clone: Len = %d, want 3", got)
	}

	// The clone keeps the index space: an index that was never used in
	// the receiver is insertable in a clone of an EMPTY queue.
	empty := NewIndexPQ[int](4)
	edup := empty.Clone()
	if !edup.Insert(3, 1) {
		t.Errorf("a clone of an empty queue must keep the index space (Insert(3) refused)")
	}

	if dup := NewIndexPQ[int](4).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *IndexPQ[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
	if dup := (&IndexPQ[int]{}).Clone(); dup == nil || !dup.Equal(NewIndexPQ[int](3)) {
		t.Errorf("zero-value Clone should read as a fresh empty queue")
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := NewIndexPQ[int](2)
	nonEmpty.Insert(0, 3)
	for i, qq := range []*IndexPQ[int]{nil, {}} {
		where := fmt.Sprintf("container %d", i)
		if got := qq.Len(); got != 0 {
			t.Errorf("%s: Len = %d, want 0", where, got)
		}
		if got := qq.String(); got != "[]" {
			t.Errorf("%s: String = %q, want []", where, got)
		}
		if got := qq.StringFunc(func(v int) string { return "x" }); got != "[]" {
			t.Errorf("%s: StringFunc = %q, want []", where, got)
		}
		other := NewIndexPQ[int](3)
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *IndexPQ[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	if dup := (&IndexPQ[int]{}).Clone(); dup == nil || !dup.Equal(NewIndexPQ[int](3)) {
		t.Errorf("zero-value Clone should read as a fresh empty queue")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	const n = 25
	rng := rand.New(rand.NewPCG(1, 42))
	q := NewIndexPQ[int](n)
	model := map[int]int{}
	for step := 0; step < 800; step++ {
		switch rng.IntN(7) {
		case 0, 1: // insert (replaces a present index's value)
			k, v := rng.IntN(n), rng.IntN(20)
			if !q.Insert(k, v) {
				t.Fatalf("step %d: Insert(%d) refused", step, k)
			}
			model[k] = v
		case 2: // delete
			k := rng.IntN(n)
			q.Delete(k)
			delete(model, k)
		case 3: // change a present value
			k, v := rng.IntN(n), rng.IntN(20)
			if _, present := model[k]; present {
				if !q.Change(k, v) {
					t.Fatalf("step %d: Change(%d) refused a present index", step, k)
				}
				model[k] = v
			}
		case 4: // pop the minimum
			if len(model) > 0 {
				k, v, found := q.Pop()
				if !found {
					t.Fatalf("step %d: Pop not found, model has %d", step, len(model))
				}
				if mv, present := model[k]; !present || mv != v {
					t.Fatalf("step %d: Pop = (%d,%d), model holds (%d,%v)", step, k, v, k, mv)
				}
				if v != capMin(model) {
					t.Fatalf("step %d: Pop = %d, want the minimum %d", step, v, capMin(model))
				}
				delete(model, k)
			}
		case 5: // peek
			if len(model) > 0 {
				k, v, found := q.Peek()
				if !found || model[k] != v || v != capMin(model) {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 6: // occasional truncate
			if rng.IntN(8) == 0 {
				q.Truncate()
				clear(model)
			}
		}
		if step%37 == 0 {
			checkInvariants(t, q, Compare[int])
			checkCapModel(t, q, model, step)
		}
	}
	checkInvariants(t, q, Compare[int])
	checkCapModel(t, q, model, -1)

	// A queue built from the final bindings in a SHUFFLED order must be
	// equal (binding equality, the map-shaped §4.5 contract).
	if len(model) > 0 {
		keys := slices.Collect(maps.Keys(model))
		rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		twin := NewIndexPQ[int](n)
		for _, k := range keys {
			twin.Insert(k, model[k])
		}
		if !q.Equal(twin) || !twin.Equal(q) {
			t.Errorf("a shuffled-order rebuild of the same bindings must be equal")
		}
	}
}

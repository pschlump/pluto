package fib_heap

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in priority order (All's order), rendering, BINDING
// equality (the map-shaped §4.5 exception: same index→value
// associations, not just the same value multiset), deep copy carrying
// the index space, the nil/zero-value read contract, and a randomized
// model test against a map[int]int reference — wide value range, per
// the package note that comparator ties poison mirror oracles.

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

// capFibOf builds a queue over the index space 0..n-1 with the given
// (index, value) bindings, inserted in order.
func capFibOf(n int, pairs ...[2]int) *IndexFibonacciPQ[int] {
	q := NewIndexFibonacciPQ[int](n)
	for _, p := range pairs {
		q.Insert(p[0], p[1])
	}
	return q
}

// capBracketOf renders items the way String must when the queue's
// priority order equals items.
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

// capModelValues returns the model's values, sorted.
func capModelValues(model map[int]int) []int {
	out := make([]int, 0, len(model))
	for _, v := range model {
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

// capTwin builds a fresh queue from the model, inserting in sorted index
// order (the insertion order must not matter to Equal).
func capTwin(model map[int]int) *IndexFibonacciPQ[int] {
	keys := make([]int, 0, len(model))
	for k := range model {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	twin := NewIndexFibonacciPQ[int](64)
	for _, k := range keys {
		twin.Insert(k, model[k])
	}
	return twin
}

// checkCapModel verifies the whole capability surface against the
// binding reference model.
func checkCapModel(t *testing.T, q *IndexFibonacciPQ[int], model map[int]int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := q.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	// The VALUE multiset must match (Slice is in priority order, so
	// compare sorted copies).
	got := q.Slice()
	if want := capModelValues(model); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice value multiset = %v, want %v", where, capSorted(got), want)
	}
	// Every binding must match, index by index.
	for k, v := range model {
		if got, found := q.Value(k); !found || got != v {
			t.Errorf("%s: Value(%d) = (%d, %v), want (%d, true)", where, k, got, found, v)
		}
	}
	if s, want := q.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (bindings)", where)
	}
	twin := capTwin(model)
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: must equal an independently built queue over the same bindings", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// Slice is in priority order (minimum first — All's order), which
	// for distinct values is sorted order.
	q := capFibOf(8, [2]int{0, 3}, [2]int{1, 1}, [2]int{2, 2})
	got := q.Slice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Slice = %v, want [1 2 3] (priority order)", got)
	}

	// The slice is a copy: queue operations never change it, and vice
	// versa.
	got[0] = 99
	if _, v, found := q.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the queue: Peek value = %d", v)
	}
	q.Insert(3, 0)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("queue operations changed the earlier Slice result: %v", got)
	}

	if empty := NewIndexFibonacciPQ[int](4).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *IndexFibonacciPQ[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (priority) order; ties may render
	// in any order, so use distinct values for the determinism check.
	a := capFibOf(8, [2]int{0, 5}, [2]int{1, 3}, [2]int{2, 1}, [2]int{3, 4}, [2]int{4, 2})
	b := capFibOf(8, [2]int{0, 5}, [2]int{1, 3}, [2]int{2, 1}, [2]int{3, 4}, [2]int{4, 2})
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewIndexFibonacciPQ[string](4).String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *IndexFibonacciPQ[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := capFibOf(8, [2]int{0, 3}, [2]int{1, 1}, [2]int{2, 2})
	got := q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
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
	// THE BINDING CONTRACT (the map-shaped §4.5 exception): the content
	// is the index→value association, not the value multiset.  Swapping
	// the values of two present indices keeps the multiset but changes
	// the content — NOT equal.
	a := capFibOf(8, [2]int{0, 10}, [2]int{1, 20})
	swapped := capFibOf(8, [2]int{0, 20}, [2]int{1, 10})
	if a.Equal(swapped) || swapped.Equal(a) {
		t.Errorf("queues with the same value multiset but swapped bindings must NOT be equal")
	}
	// Different index SETS with the same value multiset are not equal
	// either.
	d := capFibOf(8, [2]int{0, 5}, [2]int{2, 5})
	e := capFibOf(8, [2]int{0, 5}, [2]int{1, 5})
	if d.Equal(e) || e.Equal(d) {
		t.Errorf("queues over different index sets must NOT be equal")
	}

	// Same bindings, different insertion ORDER — equal, both directions.
	c := capFibOf(8, [2]int{1, 20}, [2]int{0, 10})
	if !a.Equal(c) || !c.Equal(a) {
		t.Errorf("queues over the same bindings must be equal regardless of insertion order")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Equal values at the same indices (cmp == 0) are equal bindings.
	f := capFibOf(8, [2]int{0, 5}, [2]int{1, 5})
	g := capFibOf(8, [2]int{1, 5}, [2]int{0, 5})
	if !f.Equal(g) || !g.Equal(f) {
		t.Errorf("same bindings inserted in different orders must be equal")
	}

	// Different lengths are never equal.
	if a.Equal(capFibOf(8, [2]int{0, 10})) || capFibOf(8, [2]int{0, 10}).Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}
	// Same length, different value multisets.
	if a.Equal(capFibOf(8, [2]int{0, 10}, [2]int{1, 21})) || capFibOf(8, [2]int{0, 10}, [2]int{1, 21}).Equal(a) {
		t.Errorf("queues over different value multisets must not be equal")
	}

	// Equal never reorders either side.
	before := a.Slice()
	_ = a.Equal(c)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}

	empty := NewIndexFibonacciPQ[int](4)
	var nilQ *IndexFibonacciPQ[int]
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
	q := capFibOf(8, [2]int{0, 5}, [2]int{1, 3}, [2]int{2, 1}, [2]int{3, 4}, [2]int{4, 2})

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// The clone keeps the index space: the same inserts succeed and the
	// same out-of-range insert fails.
	if dup.Insert(7, 42) != true {
		t.Errorf("Insert(7) inside the cloned index space should succeed")
	}
	if dup.Insert(8, 42) != false {
		t.Errorf("Insert(8) outside the cloned index space should report false")
	}

	// Deep copy, both directions.
	if _, found := q.Value(7); found {
		t.Errorf("inserting into the clone changed the receiver")
	}
	q.Delete(0)
	if _, found := dup.Value(0); !found {
		t.Errorf("deleting from the receiver changed the clone")
	}
	if _, found := dup.Value(7); !found {
		t.Errorf("deleting from the receiver changed the clone's extra binding")
	}

	if dup := NewIndexFibonacciPQ[int](4).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *IndexFibonacciPQ[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capFibOf(4, [2]int{0, 3})
	for i, qq := range []*IndexFibonacciPQ[int]{nil, {}} {
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
		other := NewIndexFibonacciPQ[int](4)
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *IndexFibonacciPQ[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	// The zero value's Slice is empty non-nil and its Clone is a fresh
	// empty queue over the same (empty) index space.
	if got := (&IndexFibonacciPQ[int]{}).Slice(); got == nil || len(got) != 0 {
		t.Errorf("zero-value Slice should be empty non-nil, got %v", got)
	}
	if dup := (&IndexFibonacciPQ[int]{}).Clone(); dup == nil || !dup.Equal(NewIndexFibonacciPQ[int](1)) {
		t.Errorf("zero-value Clone should be a fresh empty queue")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	// Wide value range per the package note: comparator ties poison
	// mirror oracles, so the model only relies on ties being RARE and
	// checks the min VALUE (not the index) on Pop/Peek.
	rng := rand.New(rand.NewPCG(1, 42))
	q := NewIndexFibonacciPQ[int](64)
	model := map[int]int{}
	for step := 0; step < 800; step++ {
		switch rng.IntN(8) {
		case 0, 1: // insert / replace
			k, v := rng.IntN(64), rng.IntN(100000)
			if !q.Insert(k, v) {
				t.Fatalf("step %d: Insert(%d) reported false", step, k)
			}
			model[k] = v
		case 2: // change, either direction
			k, v := rng.IntN(64), rng.IntN(100000)
			if ok := q.Change(k, v); ok {
				model[k] = v
			} else if _, present := model[k]; present {
				t.Fatalf("step %d: Change(%d) false but the index is present", step, k)
			}
		case 3: // decrease-key, only when it really decreases
			k, v := rng.IntN(64), rng.IntN(100000)
			if ok := q.DecreaseKey(k, v); ok {
				old, present := model[k]
				if !present || v >= old {
					t.Fatalf("step %d: DecreaseKey(%d, %d) true but old was %d (present %v)", step, k, v, old, present)
				}
				model[k] = v
			}
		case 4: // delete
			k := rng.IntN(64)
			_, present := model[k]
			if ok := q.Delete(k); ok != present {
				t.Fatalf("step %d: Delete(%d) = %v disagrees with the model (present %v)", step, k, ok, present)
			}
			delete(model, k)
		case 5: // pop the minimum
			if len(model) > 0 {
				k, v, found := q.Pop()
				if !found {
					t.Fatalf("step %d: Pop not found, model has %d", step, len(model))
				}
				want := capModelValues(model)[0]
				if v != want {
					t.Fatalf("step %d: Pop value = %d, want %d", step, v, want)
				}
				if got, present := model[k]; !present || got != v {
					t.Fatalf("step %d: Pop index %d not bound to %d in the model", step, k, v)
				}
				delete(model, k)
			}
		case 6: // peek
			if len(model) > 0 {
				if _, v, found := q.Peek(); !found || v != capModelValues(model)[0] {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 7: // occasional truncate
			if rng.IntN(8) == 0 {
				q.Truncate()
				clear(model)
			}
		}
		if step%37 == 0 {
			checkInvariants(t, q)
			checkCapModel(t, q, model, step)
		}
	}
	checkInvariants(t, q)
	checkCapModel(t, q, model, -1)

	// A queue rebuilt from the final bindings in a SHUFFLED insertion
	// order must be equal (binding equality, the §4.5 contract).
	if len(model) > 0 {
		keys := make([]int, 0, len(model))
		for k := range model {
			keys = append(keys, k)
		}
		rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		twin := NewIndexFibonacciPQ[int](64)
		for _, k := range keys {
			twin.Insert(k, model[k])
		}
		if !q.Equal(twin) || !twin.Equal(q) {
			t.Errorf("a shuffled-order rebuild of the same bindings must be equal")
		}
	}
}

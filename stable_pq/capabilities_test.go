package stable_pq

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in (key, insertion) order, rendering, KEY-multiset
// equality (the insertion stamps are deliberately ignored), deep copy
// that preserves the FIFO tie order, the nil/zero-value read contract,
// and a randomized model test against a sorted-slice reference.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// capStableOf builds a stable min-PQ of ints by inserting items in
// order.
func capStableOf(items ...int) *StablePQ[int] {
	pq := NewStablePQ[int]()
	for _, v := range items {
		pq.Insert(v)
	}
	return pq
}

// capBracketOf renders items the way String must when the queue's
// (key, insertion) order equals items.
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
func checkCapModel(t *testing.T, pq *StablePQ[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := pq.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := pq.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	if want := capSorted(model); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice key multiset = %v, want %v", where, capSorted(got), want)
	}
	// String must render exactly the Slice order.
	if s, want := pq.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := pq.Clone()
	if !dup.Equal(pq) || !pq.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (key multiset)", where)
	}
	twin := capStableOf(model...)
	if !pq.Equal(twin) || !twin.Equal(pq) {
		t.Errorf("%s: must equal an independently built queue over the same key multiset", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// Slice is in (key, insertion) order — minimum key first, ties
	// oldest first — which for keys is sorted order, deterministic even
	// under ties: insert 2, 1, 2, 1 -> [1 1 2 2].
	pq := capStableOf(2, 1, 2, 1)
	got := pq.Slice()
	if len(got) != 4 || got[0] != 1 || got[1] != 1 || got[2] != 2 || got[3] != 2 {
		t.Errorf("Slice = %v, want [1 1 2 2] ((key, insertion) order)", got)
	}

	// The slice is a copy: queue operations never change it, and vice
	// versa.
	got[0] = 99
	if v, found := pq.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the queue: Peek = %d", v)
	}
	pq.Insert(0)
	if got[0] != 99 || len(got) != 4 {
		t.Errorf("queue operations changed the earlier Slice result: %v", got)
	}

	if empty := NewStablePQ[int]().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *StablePQ[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice order — stable, so even tie-heavy
	// sequences render deterministically.
	a := capStableOf(5, 3, 1, 4, 2, 1, 3)
	b := capStableOf(5, 3, 1, 4, 2, 1, 3)
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewStablePQ[string]().String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *StablePQ[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	pq := capStableOf(2, 1, 2, 1)
	got := pq.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range pq.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "#%d", v)
	}
	sb.WriteByte(']')
	if got != sb.String() {
		t.Errorf("StringFunc = %q, want %q", got, sb.String())
	}
	if got := pq.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// THE KEY-MULTISET CONTRACT, STAMPS IGNORED: the insertion stamps
	// are internal tie-breaking bookkeeping, not content — the same key
	// multiset inserted in different orders carries different stamps but
	// must be equal.
	a := capStableOf(1, 2, 3, 4, 5)
	b := capStableOf(5, 4, 3, 2, 1)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("queues over the same key multiset must be equal regardless of insertion order")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Tie-heavy multisets, inserted in different orders — the stamps
	// genuinely differ, the queues are still equal.
	p := capStableOf(7, 7, 7, 2, 2, 9)
	q := capStableOf(2, 9, 7, 2, 7, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-heavy queues over the same key multiset must be equal (stamps ignored)")
	}

	// Different lengths are never equal.
	if a.Equal(capStableOf(1, 2, 3, 4)) || capStableOf(1, 2, 3, 4).Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}
	// Same length, different multisets ([1 1 2] vs [1 2 2]).
	x := capStableOf(1, 1, 2)
	y := capStableOf(1, 2, 2)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 1 2] and [1 2 2] are different multisets and must not be equal")
	}

	// A max-order queue (reversed comparator) compares consistently too.
	rmax := NewStablePQFunc[int](func(a, b int) int { return cmp.Compare(b, a) })
	smax := NewStablePQFunc[int](func(a, b int) int { return cmp.Compare(b, a) })
	for _, v := range []int{4, 1, 3} {
		rmax.Insert(v)
	}
	for _, v := range []int{3, 4, 1} {
		smax.Insert(v)
	}
	if !rmax.Equal(smax) || !smax.Equal(rmax) {
		t.Errorf("reversed-comparator queues over the same multiset must be equal")
	}

	// Equal is non-destructive: the receiver keeps its order.
	before := a.Slice()
	_ = a.Equal(b)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}

	empty := NewStablePQ[int]()
	var nilQ *StablePQ[int]
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
	pq := capStableOf(5, 3, 1, 4, 2)

	dup := pq.Clone()
	if !dup.Equal(pq) || !pq.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// The clone preserves the exact materialization order.
	orig, cloned := pq.Slice(), dup.Slice()
	if !slices.Equal(orig, cloned) {
		t.Errorf("clone must preserve the (key, insertion) order: %v vs %v", orig, cloned)
	}

	// Deep copy, both directions.
	dup.Insert(0)
	if got := pq.Len(); got != 5 {
		t.Errorf("inserting into the clone changed the receiver: Len = %d, want 5", got)
	}
	pq.Pop()
	if got := dup.Len(); got != 6 {
		t.Errorf("popping the receiver changed the clone: Len = %d, want 6", got)
	}

	if dup := NewStablePQ[int]().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *StablePQ[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
}

// TestCapabilitiesClonePreservesFIFOOrder is the stability witness for
// Clone: the clone keeps every element's insertion stamp and the stamp
// counter, so draining the clone yields the receiver's exact pop
// sequence — FIFO among ties included — and later inserts on both sides
// keep interleaving consistently (task's comparator orders by prio
// only, so the tags observe the tie order).
func TestCapabilitiesClonePreservesFIFOOrder(t *testing.T) {
	build := func() *StablePQ[task] {
		pq := NewStablePQFunc(byPrio)
		for _, x := range []task{{2, "a"}, {1, "b"}, {2, "c"}, {1, "d"}} {
			pq.Insert(x)
		}
		return pq
	}
	drain := func(q *StablePQ[task]) []string {
		var tags []string
		for {
			x, found := q.Pop()
			if !found {
				return tags
			}
			tags = append(tags, x.tag)
		}
	}
	want := []string{"b", "d", "a", "c"} // ties FIFO: b<d, a<c, 1s before 2s

	pq := build()
	if got := drain(pq); !slices.Equal(got, want) {
		t.Fatalf("receiver drain = %v, want %v", got, want)
	}

	pq = build()
	dup := pq.Clone()
	pq.Insert(task{1, "e"})  // continues the RECEIVER's stamp counter
	dup.Insert(task{1, "e"}) // continues the CLONE's stamp counter
	if got := drain(pq); !slices.Equal(got, []string{"b", "d", "e", "a", "c"}) {
		t.Errorf("receiver drain after insert = %v", got)
	}
	if got := drain(dup); !slices.Equal(got, []string{"b", "d", "e", "a", "c"}) {
		t.Errorf("clone drain after insert = %v (the cloned counter must continue)", got)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capStableOf(3)
	for i, qq := range []*StablePQ[int]{nil, {}} {
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
		other := NewStablePQ[int]()
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *StablePQ[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	// The zero value's Slice is empty non-nil and its Clone is a fresh
	// empty queue.
	if got := (&StablePQ[int]{}).Slice(); got == nil || len(got) != 0 {
		t.Errorf("zero-value Slice should be empty non-nil, got %v", got)
	}
	if dup := (&StablePQ[int]{}).Clone(); dup == nil || !dup.Equal(NewStablePQ[int]()) {
		t.Errorf("zero-value Clone should be a fresh empty queue")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	pq := NewStablePQ[int]()
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1: // insert
			v := rng.IntN(20)
			pq.Insert(v)
			model = append(model, v)
		case 2, 3: // pop the minimum (ties: the oldest, but the key is
			// the minimum either way)
			if len(model) > 0 {
				v, found := pq.Pop()
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
				if v, found := pq.Peek(); !found || v != capSorted(model)[0] {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 5: // occasional truncate
			if rng.IntN(8) == 0 {
				pq.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkCapModel(t, pq, model, step)
		}
	}
	checkCapModel(t, pq, model, -1)

	// A queue built from the final multiset in a SHUFFLED order must be
	// equal (key-multiset equality, stamps ignored — the §4.5 contract).
	if len(model) > 0 {
		shuffled := slices.Clone(model)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		twin := capStableOf(shuffled...)
		if !pq.Equal(twin) || !twin.Equal(pq) {
			t.Errorf("a shuffled-order rebuild of the same multiset must be equal")
		}
	}
}

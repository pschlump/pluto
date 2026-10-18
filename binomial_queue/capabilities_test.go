package binomial_queue

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in internal forest order, rendering, MULTISET
// equality (same multiset, different forest shapes and merge
// histories, must be equal), deep copy by re-insertion (shape may
// differ, drain order survives), the nil/zero-value read contract, and
// a randomized model test against a sorted-slice reference.

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

// capBQOf builds a binomial queue of ints by inserting items in order.
func capBQOf(items ...int) *BinomialQueue[int] {
	q := NewBinomialQueue[int]()
	for _, v := range items {
		q.Insert(v)
	}
	return q
}

// capBracketOf renders items the way String must when the queue's
// forest order equals items.
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
func checkCapModel(t *testing.T, q *BinomialQueue[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := q.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := q.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	if want := capSorted(model); !slices.Equal(capSorted(got), want) {
		t.Errorf("%s: Slice multiset = %v, want %v", where, capSorted(got), want)
	}
	// String must render exactly the Slice order (forest order).
	if s, want := q.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (multiset)", where)
	}
	twin := capBQOf(model...)
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: must equal an independently built queue over the same multiset", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// The forest order is deterministic for a given insertion sequence:
	// insert 3, 1, 2 -> the forest is [B0:2, B1:(1 with child 3)] ->
	// pre-order [2, 1, 3].
	q := capBQOf(3, 1, 2)
	got := q.Slice()
	if len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 3 {
		t.Errorf("Slice = %v, want [2 1 3] (internal forest order)", got)
	}
	checkInvariants(t, q, Compare[int])

	// The slice is a copy: queue operations never change it, and vice
	// versa.
	got[0] = 99
	if v, found := q.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the queue: Peek = %d", v)
	}
	q.Insert(0)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("queue operations changed the earlier Slice result: %v", got)
	}
	checkInvariants(t, q, Compare[int])

	if empty := NewBinomialQueue[int]().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *BinomialQueue[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (forest) order, so two queues
	// built by the same insertion sequence render identically.
	a := capBQOf(5, 3, 1, 4, 2)
	b := capBQOf(5, 3, 1, 4, 2)
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewBinomialQueue[string]().String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *BinomialQueue[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := capBQOf(3, 1, 2)
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
	// THE MULTISET CONTRACT: same multiset, different insertion orders —
	// different forests, still equal.
	a := capBQOf(1, 2, 3, 4, 5)
	b := capBQOf(5, 4, 3, 2, 1)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("queues over the same multiset must be equal regardless of insertion order")
	}
	// ... and their forests genuinely differ (Equal must not depend on
	// the shape).
	if slices.Equal(a.Slice(), b.Slice()) {
		t.Errorf("expected different forest orders for the two insertion orders: %v", a.Slice())
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// A queue built by MERGING holds yet another forest shape — the
	// merge history is not content either.
	left, right := capBQOf(1), capBQOf(2, 3)
	left.Merge(right)
	if !left.Equal(capBQOf(3, 2, 1)) || !capBQOf(3, 2, 1).Equal(left) {
		t.Errorf("a merged queue over the same multiset must be equal")
	}

	// Different lengths are never equal.
	if a.Equal(capBQOf(1, 2, 3, 4)) || capBQOf(1, 2, 3, 4).Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}
	// Same length, different multisets ([1 1 2] vs [1 2 2]).
	x := capBQOf(1, 1, 2)
	y := capBQOf(1, 2, 2)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 1 2] and [1 2 2] are different multisets and must not be equal")
	}

	// Ties: heavy-duplicate multisets, inserted in different orders.
	p := capBQOf(7, 7, 7, 2, 2, 9)
	q := capBQOf(2, 9, 7, 2, 7, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-heavy queues over the same multiset must be equal")
	}

	// A max-first queue (reversed comparator) compares consistently too.
	rmax := NewBinomialQueueFunc[int](func(a, b int) int { return Compare(b, a) })
	smax := NewBinomialQueueFunc[int](func(a, b int) int { return Compare(b, a) })
	for _, v := range []int{4, 1, 3} {
		rmax.Insert(v)
	}
	for _, v := range []int{3, 4, 1} {
		smax.Insert(v)
	}
	if !rmax.Equal(smax) || !smax.Equal(rmax) {
		t.Errorf("reversed-comparator queues over the same multiset must be equal")
	}

	// Equal is non-destructive: the receiver keeps its forest.
	before := a.Slice()
	_ = a.Equal(b)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}
	checkInvariants(t, a, Compare[int])

	empty := NewBinomialQueue[int]()
	var nilQ *BinomialQueue[int]
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
	q := capBQOf(5, 3, 1, 4, 2)

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkInvariants(t, dup, Compare[int])

	// The clone's sorted drain is the receiver's (the forest shape may
	// differ — only the multiset and the drain are observable).
	var drainQ, drainDup []int
	for {
		v, found := q.DeleteMin()
		if !found {
			break
		}
		drainQ = append(drainQ, v)
	}
	for {
		v, found := dup.DeleteMin()
		if !found {
			break
		}
		drainDup = append(drainDup, v)
	}
	if !slices.Equal(drainQ, drainDup) || !slices.IsSorted(drainQ) {
		t.Errorf("clone drain = %v, receiver drain = %v (want identical sorted)", drainDup, drainQ)
	}

	// Deep copy, both directions (fresh queues — the drains above
	// emptied both).
	q, dup = capBQOf(2, 1), capBQOf(2, 1).Clone()
	dup.Insert(0)
	if got := q.Len(); got != 2 {
		t.Errorf("inserting into the clone changed the receiver: Len = %d, want 2", got)
	}
	q.DeleteMin()
	if got := dup.Len(); got != 3 {
		t.Errorf("deleting from the receiver changed the clone: Len = %d, want 3", got)
	}

	if dup := NewBinomialQueue[int]().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *BinomialQueue[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
}

func TestCapabilitiesCloneOfZeroValueIsAMergeTarget(t *testing.T) {
	// A zero-value queue clones to a fresh queue that is still without a
	// comparison function — like the receiver — but is still usable as a
	// Merge target, which adopts the operand's comparison function.
	dup := (&BinomialQueue[int]{}).Clone()
	if dup == nil || dup.Len() != 0 {
		t.Fatalf("zero-value Clone should be a fresh empty queue")
	}
	if !dup.Equal(NewBinomialQueue[int]()) {
		t.Errorf("zero-value Clone should equal an empty queue")
	}
	src := capBQOf(2, 1)
	dup.Merge(src)
	if got := dup.Len(); got != 2 {
		t.Fatalf("Merge into the zero-value clone: Len = %d, want 2", got)
	}
	if v, found := dup.Peek(); !found || v != 1 {
		t.Errorf("after adopting the operand's comparator, Peek = (%d, %v), want (1, true)", v, found)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capBQOf(3)
	for i, qq := range []*BinomialQueue[int]{nil, {}} {
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
		other := NewBinomialQueue[int]()
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *BinomialQueue[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	// The zero value's Slice is empty non-nil.
	if got := (&BinomialQueue[int]{}).Slice(); got == nil || len(got) != 0 {
		t.Errorf("zero-value Slice should be empty non-nil, got %v", got)
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	q := NewBinomialQueue[int]()
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(7) {
		case 0, 1: // insert
			v := rng.IntN(20)
			q.Insert(v)
			model = append(model, v)
		case 2, 3: // delete the minimum
			if len(model) > 0 {
				v, found := q.DeleteMin()
				if !found {
					t.Fatalf("step %d: DeleteMin not found, model has %d", step, len(model))
				}
				want := capSorted(model)[0]
				if v != want {
					t.Fatalf("step %d: DeleteMin = %d, want %d", step, v, want)
				}
				model = slices.Delete(model, slices.Index(model, want), slices.Index(model, want)+1)
			}
		case 4: // merge a small scratch queue in (a different build
			// history over the same growing multiset)
			scratch := NewBinomialQueue[int]()
			var vals []int
			for k := rng.IntN(3) + 1; k > 0; k-- {
				v := rng.IntN(20)
				scratch.Insert(v)
				vals = append(vals, v)
			}
			q.Merge(scratch)
			if scratch.Len() != 0 {
				t.Fatalf("step %d: Merge left %d elements in the operand", step, scratch.Len())
			}
			model = append(model, vals...)
		case 5: // peek
			if len(model) > 0 {
				if v, found := q.Peek(); !found || v != capSorted(model)[0] {
					t.Fatalf("step %d: Peek mismatch", step)
				}
			}
		case 6: // occasional truncate
			if rng.IntN(8) == 0 {
				q.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkInvariants(t, q, Compare[int])
			checkCapModel(t, q, model, step)
		}
	}
	checkInvariants(t, q, Compare[int])
	checkCapModel(t, q, model, -1)

	// A queue built from the final multiset in a SHUFFLED order must be
	// equal (multiset equality, the §4.5 contract).
	if len(model) > 0 {
		shuffled := slices.Clone(model)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		twin := capBQOf(shuffled...)
		if !q.Equal(twin) || !twin.Equal(q) {
			t.Errorf("a shuffled-order rebuild of the same multiset must be equal")
		}
	}
}

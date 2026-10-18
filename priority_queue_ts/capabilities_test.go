package priority_queue_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in PRIORITY order (this package's Slice/String order
// is sorted — All's order, the one §4.5 exception), rendering, MULTISET
// equality (same multiset, different insertion orders, must be equal),
// deep copy, the nil/zero-value read contract, a randomized model test
// against a sorted-slice reference, and the concurrency behavior of the
// locked methods (the real assertion is `go test -race`).

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
	"sync"
	"testing"
)

// capPQOf builds a priority queue of ints by inserting items in order
// (distinct from the pq_test.go helpers).
func capPQOf(items ...int) *PriorityQueue[int] {
	pq := NewPriorityQueue[int]()
	for _, v := range items {
		pq.Insert(v)
	}
	return pq
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
// multiset reference model.
func checkCapModel(t *testing.T, pq *PriorityQueue[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := pq.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := pq.Slice()
	if want := capSorted(model); !slices.Equal(got, want) {
		t.Errorf("%s: Slice = %v, want %v (priority order — sorted)", where, got, want)
	}
	if s, want := pq.String(), capBracketOf(got...); s != want {
		t.Errorf("%s: String = %q, want %q (Slice order)", where, s, want)
	}
	dup := pq.Clone()
	if !dup.Equal(pq) || !pq.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver (multiset)", where)
	}
	twin := capPQOf(model...)
	if !pq.Equal(twin) || !twin.Equal(pq) {
		t.Errorf("%s: must equal an independently built queue over the same multiset", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	// This package's observable order is PRIORITY order (its All is
	// sorted): insert 3, 1, 2 -> Slice [1 2 3], not the backing array.
	pq := capPQOf(3, 1, 2)
	got := pq.Slice()
	if want := []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("Slice = %v, want %v (priority order — the §4.5 exception)", got, want)
	}

	// The slice is a copy taken from All's private snapshot: mutating it
	// must not affect the queue, and queue operations must not change an
	// earlier slice.
	got[0] = 99
	if v, found := pq.Peek(); !found || v != 1 {
		t.Errorf("mutating the Slice result changed the queue: Peek = %d", v)
	}
	pq.Insert(0)
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("queue operations changed the earlier Slice result: %v", got)
	}

	if empty := NewPriorityQueue[int]().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *PriorityQueue[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// String renders exactly the Slice (priority) order, so two queues
	// built from the same multiset — even in different insertion orders —
	// render identically.
	a := capPQOf(5, 3, 1, 4, 2)
	b := capPQOf(5, 3, 1, 4, 2)
	if a.String() != b.String() {
		t.Errorf("same insertion sequence must render identically: %q vs %q", a.String(), b.String())
	}
	c := capPQOf(1, 2, 3, 4, 5)
	if a.String() != c.String() {
		t.Errorf("same multiset must render identically: %q vs %q", a.String(), c.String())
	}
	if s, want := a.String(), capBracketOf(a.Slice()...); s != want {
		t.Errorf("String = %q, want %q (the Slice order)", s, want)
	}

	if got := NewPriorityQueue[string]().String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *PriorityQueue[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	pq := capPQOf(1, 3, 2)
	got := pq.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	// Expected: the Slice (priority) order, each element rendered by the
	// format function.
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
	// THE MULTISET CONTRACT: same multiset, different insertion orders —
	// still equal.  Unlike the heap packages, Slice is sorted here, so
	// the two sides' arrangements are NOT observable through the public
	// surface; Equal's work is distinguishing multisets ([1 1 2] vs
	// [1 2 2] below), and Equal remains multiset equality by contract.
	a := capPQOf(1, 2, 3, 4, 5)
	b := capPQOf(5, 4, 3, 2, 1)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("queues over the same multiset must be equal regardless of insertion order")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	if a.Equal(capPQOf(1, 2, 3, 4)) || capPQOf(1, 2, 3, 4).Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}
	x := capPQOf(1, 1, 2)
	y := capPQOf(1, 2, 2)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 1 2] and [1 2 2] are different multisets and must not be equal")
	}

	// Ties: heavy-duplicate multisets, inserted in different orders.
	p := capPQOf(7, 7, 7, 2, 2, 9)
	q := capPQOf(2, 9, 7, 2, 7, 7)
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("tie-heavy queues over the same multiset must be equal")
	}

	// A max-first queue (reversed comparator) compares consistently too.
	rev := func(a, b int) int { return cmp.Compare(b, a) }
	rmax := NewPriorityQueueFunc[int](rev)
	smax := NewPriorityQueueFunc[int](rev)
	for _, v := range []int{4, 1, 3} {
		rmax.Insert(v)
	}
	for _, v := range []int{3, 4, 1} {
		smax.Insert(v)
	}
	if !rmax.Equal(smax) || !smax.Equal(rmax) {
		t.Errorf("reversed-comparator queues over the same multiset must be equal")
	}

	// Equal is non-destructive.
	before := a.Slice()
	_ = a.Equal(b)
	if after := a.Slice(); !slices.Equal(before, after) {
		t.Errorf("Equal reordered the receiver: %v -> %v", before, after)
	}

	empty := NewPriorityQueue[int]()
	var nilQ *PriorityQueue[int]
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
	pq := capPQOf(5, 3, 1, 4, 2)

	dup := pq.Clone()
	if !dup.Equal(pq) || !pq.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// The clone copies the underlying backing array, so the pop sequence
	// — the arrangement-dependent observable — is the receiver's.
	for {
		a, fa := pq.Pop()
		b, fb := dup.Pop()
		if !fa || !fb {
			if fa != fb {
				t.Fatalf("clone and receiver drained to different lengths")
			}
			break
		}
		if a != b {
			t.Fatalf("pop sequences diverge: %d vs %d", a, b)
		}
	}

	// Deep copy, both directions.
	dup.Insert(0)
	if got := pq.Len(); got != 0 {
		t.Errorf("inserting into the clone changed the receiver: Len = %d, want 0", got)
	}
	pq.Insert(9)
	if got := dup.Len(); got != 1 {
		t.Errorf("inserting into the receiver changed the clone: Len = %d, want 1", got)
	}

	if dup := NewPriorityQueue[int]().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *PriorityQueue[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
	if dup := (&PriorityQueue[int]{}).Clone(); dup == nil || !dup.Equal(NewPriorityQueue[int]()) {
		t.Errorf("zero-value Clone should be a fresh empty queue")
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capPQOf(3)
	for i, qq := range []*PriorityQueue[int]{nil, {}} {
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
		other := NewPriorityQueue[int]()
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *PriorityQueue[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	if dup := (&PriorityQueue[int]{}).Clone(); dup == nil || !dup.Equal(NewPriorityQueue[int]()) {
		t.Errorf("zero-value Clone should be a fresh empty queue")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	pq := NewPriorityQueue[int]()
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1: // insert
			v := rng.IntN(20)
			pq.Insert(v)
			model = append(model, v)
		case 2, 3: // pop the minimum
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
	// equal (multiset equality, the §4.5 contract).
	if len(model) > 0 {
		shuffled := slices.Clone(model)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		twin := capPQOf(shuffled...)
		if !pq.Equal(twin) || !twin.Equal(pq) {
			t.Errorf("a shuffled-order rebuild of the same multiset must be equal")
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency (the _ts additions) — `go test -race` is the real
// assertion: no data race and no deadlock.
// ---------------------------------------------------------------------------

func TestCapabilitiesConcurrent(t *testing.T) {
	pq := NewPriorityQueue[int]() // a zero-value queue cannot Insert (no underlying heap)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (w + i) % 8 {
				case 0, 1: // writes
					pq.Insert(i)
				case 2:
					pq.Pop()
				case 3: // reads
					pq.Len()
					pq.Peek()
				case 4: // capability snapshot reads
					pq.Slice()
				case 5:
					_ = pq.String()
				case 6:
					pq.Clone().Len()
				case 7:
					pq.StringFunc(func(v int) string { return fmt.Sprint(v) })
				}
			}
		}(w)
	}
	wg.Wait()
	if pq.Len() == 0 {
		t.Errorf("after the hammer the queue should hold elements, Len = %d", pq.Len())
	}
	for {
		if _, found := pq.Pop(); !found {
			break
		}
	}
	if pq.Len() != 0 {
		t.Errorf("final Len = %d, the queue should be empty after draining", pq.Len())
	}
}

func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := NewPriorityQueue[int]()
	b := NewPriorityQueue[int]()
	for i := 0; i < 500; i++ {
		a.Insert(i)
		b.Insert(499 - i) // same multiset, inserted in the opposite order
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
	if !a.Equal(b) {
		t.Errorf("queues over the same multiset must be equal")
	}
}

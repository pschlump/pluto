package sll_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization head to tail, deterministic rendering, order-sensitive
// equality through the list's equality function, deep copy, the
// nil/zero-value read contract, a randomized model test against a slice
// reference, and the concurrent-hammer/no-deadlock checks the twin owes.

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

// capSllOf builds a list holding items head to tail (items[0] is the
// head).
func capSllOf(items ...TestSllItem) *Sll[TestSllItem] {
	list := newTestSll()
	for _, it := range items {
		list.InsertAfterTail(it)
	}
	return list
}

// capBracketOf renders items the way String/StringFunc must: fmt.Sprint
// per element, single spaces, square brackets.
func capBracketOf(items ...TestSllItem) string {
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

// capCheckModel verifies the whole capability surface against the slice
// reference model (model[0] is the head).
func capCheckModel(t *testing.T, list *Sll[TestSllItem], model []TestSllItem, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := list.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := list.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	for i := range model {
		if got[i] != model[i] {
			t.Fatalf("%s: Slice[%d] = %v, want %v", where, i, got[i], model[i])
		}
	}
	if s := list.String(); s != capBracketOf(model...) {
		t.Errorf("%s: String = %q, want %q", where, s, capBracketOf(model...))
	}
	dup := list.Clone()
	if !dup.Equal(list) || !list.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver", where)
	}
	checkInvariants(t, list, where)
	checkInvariants(t, dup, where+"/clone")
	twin := capSllOf(model...)
	if !list.Equal(twin) || !twin.Equal(list) {
		t.Errorf("%s: list must equal an independently built list with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	list := newTestSll()
	list.InsertAfterTail(TestSllItem{S: "a"})
	list.InsertAfterTail(TestSllItem{S: "b"})
	list.InsertBeforeHead(TestSllItem{S: "c"}) // c becomes the new head

	got := list.Slice()
	if len(got) != 3 || got[0].S != "c" || got[1].S != "a" || got[2].S != "b" {
		t.Errorf("Slice = %v, want [c a b] head to tail", got)
	}

	// The slice is a copy: mutating it must not affect the list, and
	// list operations must not change the earlier slice.
	got[0] = TestSllItem{S: "zz"}
	if v, err := list.Peek(); err != nil || v.S != "c" {
		t.Errorf("mutating the Slice result changed the list: Peek = %v,%v", v, err)
	}
	if _, err := list.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if got[0].S != "zz" || len(got) != 3 {
		t.Errorf("list operations changed the earlier Slice result: %v", got)
	}

	if empty := newTestSll().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty list should be empty non-nil, got %v", empty)
	}
	var nilList *Sll[TestSllItem]
	if s := nilList.Slice(); s != nil {
		t.Errorf("Slice of a nil list should be nil, got %v", s)
	}

	// A truncated list yields an empty slice.
	emptied := capSllOf(TestSllItem{S: "x"})
	emptied.Truncate()
	if s := emptied.Slice(); s == nil || len(s) != 0 {
		t.Errorf("Slice of a truncated list should be empty non-nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed lists holding the same sequence must
	// render identically.
	a := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "c"})
	b := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "c"})
	if a.String() != b.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[{a} {b} {c}]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if s1, s2 := a.String(), a.String(); s1 != s2 {
		t.Errorf("String must be stable across calls")
	}

	if got := newTestSll().String(); got != "[]" {
		t.Errorf("String of an empty list = %q, want []", got)
	}
	var nilList *Sll[string]
	if got := nilList.String(); got != "[]" {
		t.Errorf("String of a nil list = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	list := NewSll[int]()
	list.InsertAfterTail(5)
	list.InsertAfterTail(8)
	got := list.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := list.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "c"})
	b := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "c"})
	c := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"})
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("lists with the same sequence must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("lists of different lengths must not be equal")
	}

	// ORDER SENSITIVITY — the sequence contract: same elements in a
	// different order is a different list.
	x := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"})
	y := capSllOf(TestSllItem{S: "b"}, TestSllItem{S: "a"})
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[a b] and [b a] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "a"}, TestSllItem{S: "b"})
	yy := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "a"})
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("[a a b] and [a b a] must not be equal")
	}

	// EQ SEMANTICS — Equal compares through the list's equality
	// function: elements the eq calls equal are equal, even when their
	// full values differ.
	type capSllItem struct {
		S string
		N int
	}
	byS := func(a, b capSllItem) bool { return a.S == b.S }
	p := NewSllFunc(byS)
	p.InsertAfterTail(capSllItem{S: "a", N: 1})
	p.InsertAfterTail(capSllItem{S: "b", N: 2})
	q := NewSllFunc(byS)
	q.InsertAfterTail(capSllItem{S: "a", N: 900})
	q.InsertAfterTail(capSllItem{S: "b", N: 800})
	if !p.Equal(q) || !q.Equal(p) {
		t.Errorf("elements the equality function calls equal must compare equal")
	}
	q2 := NewSllFunc(byS)
	q2.InsertAfterTail(capSllItem{S: "a", N: 900})
	q2.InsertAfterTail(capSllItem{S: "c", N: 800})
	if p.Equal(q2) || q2.Equal(p) {
		t.Errorf("a differing element must break equality")
	}

	empty := newTestSll()
	var nilList *Sll[TestSllItem]
	if !nilList.Equal(empty) || !empty.Equal(nilList) {
		t.Errorf("nil and empty lists must be equal, both directions")
	}
	if !nilList.Equal(nilList) {
		t.Errorf("nil must equal nil")
	}
	if nilList.Equal(a) || a.Equal(nilList) {
		t.Errorf("nil/empty must not equal a non-empty list")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	list := capSllOf(TestSllItem{S: "a"}, TestSllItem{S: "b"}, TestSllItem{S: "c"})

	dup := list.Clone()
	if !dup.Equal(list) || !list.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkInvariants(t, dup, "clone")

	// Deep copy: mutating the clone never affects the receiver.
	dup.InsertAfterTail(TestSllItem{S: "d"})
	if got := list.Slice(); fmt.Sprint(got) != "[{a} {b} {c}]" {
		t.Errorf("appending to the clone changed the receiver: %v", got)
	}

	// And mutating the receiver never affects the clone.
	if _, err := list.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if got := dup.Slice(); fmt.Sprint(got) != "[{a} {b} {c} {d}]" {
		t.Errorf("popping the receiver changed the clone: %v", got)
	}

	// The clone keeps the equality function: Search still finds by eq.
	if el, pos := dup.Search(TestSllItem{S: "b"}); el == nil || pos != 1 {
		t.Errorf("clone Search by the carried equality function = %v,%d; want found at 1", el != nil, pos)
	}

	if dup := newTestSll().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty list should be a fresh empty list")
	}
	var nilList *Sll[TestSllItem]
	if dup := nilList.Clone(); dup != nil {
		t.Errorf("Clone of a nil list should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := capSllOf(TestSllItem{S: "z"})
	for i, ll := range []*Sll[TestSllItem]{nil, {}} {
		where := fmt.Sprintf("container %d", i)
		if got := ll.Len(); got != 0 {
			t.Errorf("%s: Len = %d, want 0", where, got)
		}
		if got := ll.String(); got != "[]" {
			t.Errorf("%s: String = %q, want []", where, got)
		}
		if got := ll.StringFunc(func(v TestSllItem) string { return "x" }); got != "[]" {
			t.Errorf("%s: StringFunc = %q, want []", where, got)
		}
		other := newTestSll()
		if !ll.Equal(other) || !other.Equal(ll) {
			t.Errorf("%s: must equal an empty list, both directions", where)
		}
		if ll.Equal(nonEmpty) || nonEmpty.Equal(ll) {
			t.Errorf("%s: must not equal a non-empty list", where)
		}
	}
	var nilList *Sll[TestSllItem]
	if got := nilList.Slice(); got != nil {
		t.Errorf("nil list Slice = %v, want nil", got)
	}
	if dup := nilList.Clone(); dup != nil {
		t.Errorf("nil list Clone = %v, want nil", dup)
	}
	// A zero-value list clones to a fresh empty list (eq nil is carried,
	// the clone is empty so it is never consulted).
	if dup := (&Sll[TestSllItem]{}).Clone(); dup == nil || !dup.Equal(&Sll[TestSllItem]{}) {
		t.Errorf("zero-value Clone should be a fresh empty list")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	list := newTestSll()
	var model []TestSllItem // model[0] is the head
	keySpace := 25
	for step := 0; step < 800; step++ {
		switch rng.IntN(5) {
		case 0: // append at tail
			it := TestSllItem{S: fmt.Sprintf("k%02d", rng.IntN(keySpace))}
			list.InsertAfterTail(it)
			model = append(model, it)
		case 1: // push at head
			it := TestSllItem{S: fmt.Sprintf("k%02d", rng.IntN(keySpace))}
			list.InsertBeforeHead(it)
			model = append([]TestSllItem{it}, model...)
		case 2: // pop head
			if len(model) > 0 {
				v, err := list.Pop()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: Pop = %v,%v; want %v", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // delete first equal-from-head
			it := TestSllItem{S: fmt.Sprintf("k%02d", rng.IntN(keySpace))}
			err := list.Delete(it)
			found := -1
			for i := range model {
				if eqTestSllItem(model[i], it) {
					found = i
					break
				}
			}
			if found >= 0 && err != nil {
				t.Fatalf("step %d: Delete(%v) = %v; want nil", step, it, err)
			}
			if found < 0 && err == nil {
				t.Fatalf("step %d: Delete(%v) deleted a missing element", step, it)
			}
			if found >= 0 {
				model = append(model[:found], model[found+1:]...)
			}
		case 4: // occasional truncate
			if rng.IntN(8) == 0 {
				list.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			capCheckModel(t, list, model, step)
		}
	}
	capCheckModel(t, list, model, -1)

	// The final list must not equal a rotation of itself (order counts).
	if len(model) > 1 {
		rotated := append(append([]TestSllItem{}, model[1:]...), model[0])
		twin := capSllOf(rotated...)
		if list.Equal(twin) || twin.Equal(list) {
			t.Errorf("a rotated list must not be equal to the original")
		}
	}
}

// The concurrent hammer: writers mutate both ends while readers snapshot
// through the capability methods.  Equal runs against a live list (the
// lengths drift, so it mostly returns false — the point is that it does
// not deadlock or race while the other goroutines hammer the same lists).
func TestCapabilitiesConcurrent(t *testing.T) {
	list := NewSll[int]() // a zero-value list has no equality function: inserts panic
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (w + i) % 8 {
				case 0, 1: // writes
					list.InsertBeforeHead(i)
				case 2:
					list.InsertAfterTail(i)
					_, _ = list.Pop()
				case 3: // reads
					list.Len()
					_, _ = list.Peek()
				case 4: // capability snapshot reads
					list.Slice()
				case 5:
					_ = list.String() // vet: a discarded Stringer call must be explicit
				case 6:
					list.Clone().Len()
				case 7:
					list.StringFunc(func(v int) string { return fmt.Sprint(v) })
				}
			}
		}(w)
	}
	wg.Wait()
	if list.Len() == 0 {
		t.Errorf("after the hammer the list should hold elements, Len = %d", list.Len())
	}
	for {
		if _, err := list.Pop(); err != nil {
			break
		}
	}
	if list.Len() != 0 {
		t.Errorf("final Len = %d, the list should be empty after draining", list.Len())
	}
}

// Equal snapshots each list under its own lock and never holds two locks
// at once — two goroutines comparing the same pair in opposite directions
// must not deadlock.
func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := NewSll[int]()
	b := NewSll[int]()
	for i := 0; i < 500; i++ {
		a.InsertAfterTail(i)
		b.InsertAfterTail(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
	if !a.Equal(b) {
		t.Errorf("identical lists must be equal")
	}
}

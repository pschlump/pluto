package dqueue_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization front to back, deterministic rendering, order-sensitive
// equality, deep copy, the nil/zero-value read contract, a randomized
// model test against a slice reference, and the concurrency behavior of
// the locked methods (the real assertion is `go test -race`).  checkLinks
// (thorough_test.go) verifies the clone's chain stays structurally sound.

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

// dequeOf builds a deque holding items from front to back.
func dequeOf(items ...int) *Deque[int] {
	q := &Deque[int]{}
	for _, v := range items {
		q.PushBack(v)
	}
	return q
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

// checkDequeModel verifies the whole capability surface against the
// slice reference model (model[0] is the front).
func checkDequeModel(t *testing.T, q *Deque[int], model []int, step int) {
	t.Helper()
	where := fmt.Sprintf("step %d", step)
	if got := q.Len(); got != len(model) {
		t.Fatalf("%s: Len = %d, want %d", where, got, len(model))
	}
	got := q.Slice()
	if len(got) != len(model) {
		t.Fatalf("%s: Slice length = %d, want %d", where, len(got), len(model))
	}
	for i := range model {
		if got[i] != model[i] {
			t.Fatalf("%s: Slice[%d] = %d, want %d", where, i, got[i], model[i])
		}
	}
	if s := q.String(); s != bracketOf(model...) {
		t.Errorf("%s: String = %q, want %q", where, s, bracketOf(model...))
	}
	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("%s: clone must be equal to the receiver", where)
	}
	checkLinks(t, dup)
	twin := dequeOf(model...)
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: deque must equal an independently built deque with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	q := dequeOf(1, 2, 3)
	q.PushFront(0)

	got := q.Slice()
	if len(got) != 4 || got[0] != 0 || got[1] != 1 || got[2] != 2 || got[3] != 3 {
		t.Errorf("Slice = %v, want [0 1 2 3] front to back", got)
	}

	// The slice is a copy: mutating it must not affect the deque, and
	// deque operations must not change the earlier slice.
	got[0] = 99
	if v, _ := q.PeekFront(); v != 0 {
		t.Errorf("mutating the Slice result changed the deque: PeekFront = %d", v)
	}
	if _, err := q.PopFront(); err != nil {
		t.Fatalf("PopFront: %v", err)
	}
	if got[0] != 99 || len(got) != 4 {
		t.Errorf("deque operations changed the earlier Slice result: %v", got)
	}

	if empty := (&Deque[int]{}).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty deque should be empty non-nil, got %v", empty)
	}
	var nilD *Deque[int]
	if s := nilD.Slice(); s != nil {
		t.Errorf("Slice of a nil deque should be nil, got %v", s)
	}

	// A drained deque (nodes unlinked) yields an empty slice.
	drained := dequeOf(1)
	_, _ = drained.PopBack()
	if s := drained.Slice(); s == nil || len(s) != 0 {
		t.Errorf("Slice of a drained deque should be empty non-nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed deques holding the same sequence must
	// render identically.
	a := dequeOf(1, 2, 3)
	b := dequeOf(1, 2, 3)
	if a.String() != b.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[1 2 3]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if s1, s2 := a.String(), a.String(); s1 != s2 {
		t.Errorf("String must be stable across calls")
	}

	if got := (&Deque[string]{}).String(); got != "[]" {
		t.Errorf("String of an empty deque = %q, want []", got)
	}
	var nilD *Deque[string]
	if got := nilD.String(); got != "[]" {
		t.Errorf("String of a nil deque = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := dequeOf(5, 8)
	got := q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := q.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := dequeOf(1, 2, 3)
	b := dequeOf(1, 2, 3)
	c := dequeOf(1, 2)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("deques with the same sequence must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("deques of different lengths must not be equal")
	}

	// ORDER SENSITIVITY — the sequence contract: same elements in a
	// different order is a different deque.
	x := dequeOf(1, 2)
	y := dequeOf(2, 1)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 2] and [2 1] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := dequeOf(1, 1, 2)
	yy := dequeOf(1, 2, 1)
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("[1 1 2] and [1 2 1] must not be equal")
	}

	empty := &Deque[int]{}
	var nilD *Deque[int]
	if !nilD.Equal(empty) || !empty.Equal(nilD) {
		t.Errorf("nil and empty deques must be equal, both directions")
	}
	if !nilD.Equal(nilD) {
		t.Errorf("nil must equal nil")
	}
	if nilD.Equal(a) || a.Equal(nilD) {
		t.Errorf("nil/empty must not equal a non-empty deque")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	q := dequeOf(1, 2, 3)

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkLinks(t, dup)

	// Deep copy: mutating the clone never affects the receiver.
	dup.PushBack(4)
	if got := q.Slice(); fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("pushing to the clone changed the receiver: %v", got)
	}

	// And mutating the receiver never affects the clone.
	if _, err := q.PopFront(); err != nil {
		t.Fatalf("PopFront: %v", err)
	}
	if got := dup.Slice(); fmt.Sprint(got) != "[1 2 3 4]" {
		t.Errorf("popping the receiver changed the clone: %v", got)
	}
	checkLinks(t, dup)
	checkLinks(t, q)

	if dup := (&Deque[int]{}).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty deque should be a fresh empty deque")
	}
	var nilD *Deque[int]
	if dup := nilD.Clone(); dup != nil {
		t.Errorf("Clone of a nil deque should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := dequeOf(7)
	for i, qq := range []*Deque[int]{nil, {}} {
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
		other := &Deque[int]{}
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty deque, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty deque", where)
		}
	}
	var nilD *Deque[int]
	if got := nilD.Slice(); got != nil {
		t.Errorf("nil deque Slice = %v, want nil", got)
	}
	if dup := nilD.Clone(); dup != nil {
		t.Errorf("nil deque Clone = %v, want nil", dup)
	}
	if dup := (&Deque[int]{}).Clone(); dup == nil || !dup.Equal(&Deque[int]{}) {
		t.Errorf("zero-value Clone should be a fresh empty deque")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	q := &Deque[int]{}
	var model []int // model[0] is the front
	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1: // push back
			v := rng.IntN(50)
			q.PushBack(v)
			model = append(model, v)
		case 2: // push front
			v := rng.IntN(50)
			q.PushFront(v)
			model = append([]int{v}, model...)
		case 3: // pop front
			if len(model) > 0 {
				v, err := q.PopFront()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: PopFront = %d,%v; want %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 4: // pop back
			if len(model) > 0 {
				v, err := q.PopBack()
				if err != nil || v != model[len(model)-1] {
					t.Fatalf("step %d: PopBack = %d,%v; want %d", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case 5: // occasional truncate
			if rng.IntN(8) == 0 {
				q.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkDequeModel(t, q, model, step)
		}
	}
	checkDequeModel(t, q, model, -1)

	// The final deque must not equal a rotation of itself (order counts).
	if len(model) > 1 {
		rotated := dequeOf(model[1:]...)
		rotated.PushBack(model[0])
		if q.Equal(rotated) || rotated.Equal(q) {
			t.Errorf("a rotated deque must not be equal to the original")
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency (the _ts additions) — `go test -race` is the real
// assertion: no data race and no deadlock.
// ---------------------------------------------------------------------------

func TestCapabilitiesConcurrent(t *testing.T) {
	q := &Deque[int]{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch (w + i) % 8 {
				case 0, 1: // writes
					q.PushBack(i)
				case 2:
					q.PushFront(i)
				case 3:
					_, _ = q.PopFront()
					_, _ = q.PopBack()
				case 4: // reads
					q.Len()
					q.IsEmpty()
					_, _ = q.PeekFront()
					_, _ = q.PeekBack()
				case 5: // capability snapshot reads
					q.Slice()
				case 6:
					_ = q.String()
					q.Clone().Len()
				case 7:
					q.StringFunc(func(v int) string { return fmt.Sprint(v) })
				}
			}
		}(w)
	}
	wg.Wait()
	if q.Len() == 0 {
		t.Errorf("after the hammer the deque should hold elements, Len = %d", q.Len())
	}
	for {
		if _, err := q.PopFront(); err != nil {
			break
		}
	}
	if q.Len() != 0 {
		t.Errorf("final Len = %d, the deque should be empty after draining", q.Len())
	}
}

func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := &Deque[int]{}
	b := &Deque[int]{}
	for i := 0; i < 500; i++ {
		a.PushBack(i)
		b.PushBack(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
	if !a.Equal(b) {
		t.Errorf("identical deques must be equal")
	}
}

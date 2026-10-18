package queue

// Tests for the set-convenience capabilities (capabilities.go):
// materialization in FIFO order, deterministic rendering, order-sensitive
// equality, deep copy, the nil/zero-value read contract, and a randomized
// model test against a slice reference.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// queueOf builds a queue holding items in order (a small test-only
// constructor so the model comparisons can build expected queues).
func queueOf(items ...int) *Queue[int] {
	q := &Queue[int]{}
	for _, v := range items {
		q.Push(v)
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

// checkQueueModel verifies the whole capability surface against the
// slice reference model.
func checkQueueModel(t *testing.T, q *Queue[int], model []int, step int) {
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
	twin := queueOf(model...)
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: queue must equal an independently built queue with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	q := queueOf(1, 2, 3)

	got := q.Slice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Slice = %v, want [1 2 3] in FIFO order", got)
	}

	// The slice is a copy: mutating it must not affect the queue, and
	// queue operations must not change the earlier slice.
	got[0] = 99
	if v, _ := q.Peek(); v != 1 {
		t.Errorf("mutating the Slice result changed the queue: Peek = %d", v)
	}
	if _, err := q.Dequeue(); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got[0] != 99 || len(got) != 3 {
		t.Errorf("queue operations changed the earlier Slice result: %v", got)
	}

	if empty := (&Queue[int]{}).Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty queue should be empty non-nil, got %v", empty)
	}
	var nilQ *Queue[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil queue should be nil, got %v", s)
	}

	// A drained queue (backing array released) yields an empty slice.
	drained := queueOf(1)
	_ = drained.Pop()
	if s := drained.Slice(); s == nil || len(s) != 0 {
		t.Errorf("Slice of a drained queue should be empty non-nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed queues holding the same sequence must
	// render identically.
	a := queueOf(1, 2, 3)
	b := queueOf(1, 2, 3)
	if a.String() != b.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[1 2 3]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	if s1, s2 := a.String(), a.String(); s1 != s2 {
		t.Errorf("String must be stable across calls")
	}

	if got := (&Queue[string]{}).String(); got != "[]" {
		t.Errorf("String of an empty queue = %q, want []", got)
	}
	var nilQ *Queue[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil queue = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := queueOf(5, 8)
	got := q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := q.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := queueOf(1, 2, 3)
	b := queueOf(1, 2, 3)
	c := queueOf(1, 2)
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("queues with the same sequence must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("queues of different lengths must not be equal")
	}

	// ORDER SENSITIVITY — the sequence contract: same elements in a
	// different order is a different queue.
	x := queueOf(1, 2)
	y := queueOf(2, 1)
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[1 2] and [2 1] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := queueOf(1, 1, 2)
	yy := queueOf(1, 2, 1)
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("[1 1 2] and [1 2 1] must not be equal")
	}

	empty := &Queue[int]{}
	var nilQ *Queue[int]
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
	q := queueOf(1, 2, 3)

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}

	// Deep copy: mutating the clone never affects the receiver.
	dup.Push(4)
	if got := q.Slice(); fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("pushing to the clone changed the receiver: %v", got)
	}

	// And mutating the receiver never affects the clone.
	if _, err := q.Dequeue(); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got := dup.Slice(); fmt.Sprint(got) != "[1 2 3 4]" {
		t.Errorf("dequeuing the receiver changed the clone: %v", got)
	}

	if dup := (&Queue[int]{}).Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty queue should be a fresh empty queue")
	}
	var nilQ *Queue[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil queue should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := queueOf(7)
	for i, qq := range []*Queue[int]{nil, {}} {
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
		other := &Queue[int]{}
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty queue, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty queue", where)
		}
	}
	var nilQ *Queue[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil queue Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil queue Clone = %v, want nil", dup)
	}
	if dup := (&Queue[int]{}).Clone(); dup == nil || !dup.Equal(&Queue[int]{}) {
		t.Errorf("zero-value Clone should be a fresh empty queue")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	q := &Queue[int]{}
	var model []int
	for step := 0; step < 800; step++ {
		switch rng.IntN(4) {
		case 0, 1: // push
			v := rng.IntN(50)
			q.Push(v)
			model = append(model, v)
		case 2: // dequeue
			if len(model) > 0 {
				v, err := q.Dequeue()
				if err != nil || v != model[0] {
					t.Fatalf("step %d: Dequeue = %d,%v; want %d", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 3: // occasional truncate
			if rng.IntN(8) == 0 {
				q.Truncate()
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkQueueModel(t, q, model, step)
		}
	}
	checkQueueModel(t, q, model, -1)

	// The final queue must not equal a rotation of itself (order counts).
	if len(model) > 1 {
		rotated := queueOf(model[1:]...)
		rotated.Push(model[0])
		if q.Equal(rotated) || rotated.Equal(q) {
			t.Errorf("a rotated queue must not be equal to the original")
		}
	}
}

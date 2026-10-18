package quicklist

// Tests for the set-convenience capabilities (capabilities.go):
// materialization head to tail (compressed segments read through
// transparently), deterministic rendering, order-sensitive equality,
// deep copy with the configuration carried over, the nil/zero-value read
// contract, and a randomized model test against a slice reference.

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

// checkQuickModel verifies the whole capability surface against the
// slice reference model (model[0] is the head).
func checkQuickModel(t *testing.T, q *QuickList[int], model []int, step int) {
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
	twin := NewQuickList[int]()
	for _, v := range model {
		twin.PushTail(v)
	}
	if !q.Equal(twin) || !twin.Equal(q) {
		t.Errorf("%s: list must equal an independently built list with the same sequence", where)
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	q := build(5) // holds 0 1 2 3 4, head first

	got := q.Slice()
	if len(got) != 5 || got[0] != 0 || got[4] != 4 {
		t.Errorf("Slice = %v, want [0 1 2 3 4] head to tail", got)
	}

	// The slice is a copy: mutating it must not affect the list, and
	// list operations must not change the earlier slice.
	got[0] = 99
	if v, ok := q.At(0); !ok || v != 0 {
		t.Errorf("mutating the Slice result changed the list: At(0) = %d", v)
	}
	q.PushHead(-1)
	if got[0] != 99 || len(got) != 5 {
		t.Errorf("list operations changed the earlier Slice result: %v", got)
	}
	checkInvariants(t, q)

	if empty := NewQuickList[int]().Slice(); empty == nil || len(empty) != 0 {
		t.Errorf("Slice of an empty list should be empty non-nil, got %v", empty)
	}
	var nilQ *QuickList[int]
	if s := nilQ.Slice(); s != nil {
		t.Errorf("Slice of a nil list should be nil, got %v", s)
	}

	// Compressed segments are read through transparently: the compressed
	// build materializes the same sequence as the plain build.
	opts := []Option[string]{
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 1, EncodeStringSegment, DecodeStringSegment),
	}
	plain := strBuild(30)
	packed := strBuild(30, opts...)
	ps, qs := plain.Slice(), packed.Slice()
	if len(ps) != len(qs) {
		t.Fatalf("compressed Slice length = %d, want %d", len(qs), len(ps))
	}
	for i := range ps {
		if ps[i] != qs[i] {
			t.Fatalf("compressed Slice[%d] = %q, want %q", i, qs[i], ps[i])
		}
	}
	checkInvariants(t, packed)
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately built lists holding the same sequence must render
	// identically — including when one is stored compressed.
	a := build(5)
	b := NewQuickList[int]()
	for i := 4; i >= 0; i-- {
		b.PushHead(i)
	}
	if a.String() != b.String() {
		t.Errorf("same sequence must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[0 1 2 3 4]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewQuickList[string]().String(); got != "[]" {
		t.Errorf("String of an empty list = %q, want []", got)
	}
	var nilQ *QuickList[string]
	if got := nilQ.String(); got != "[]" {
		t.Errorf("String of a nil list = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	q := build(3)
	got := q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#0 #1 #2]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := q.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := build(5)
	b := build(5)
	c := build(3)
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
	x := build(2) // [0 1]
	y := NewQuickList[int]()
	y.PushHead(0)
	y.PushHead(1) // [1 0]
	if x.Equal(y) || y.Equal(x) {
		t.Errorf("[0 1] and [1 0] must not be equal: Equal is order-sensitive")
	}
	// Order matters among duplicates too.
	xx := NewQuickList[int]()
	xx.PushTail(1)
	xx.PushTail(1)
	xx.PushTail(2)
	yy := NewQuickList[int]()
	yy.PushTail(1)
	yy.PushTail(2)
	yy.PushTail(1)
	if xx.Equal(yy) || yy.Equal(xx) {
		t.Errorf("[1 1 2] and [1 2 1] must not be equal")
	}

	// A compressed list equals its plain twin with the same sequence.
	opts := []Option[string]{
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 1, EncodeStringSegment, DecodeStringSegment),
	}
	if !strBuild(30).Equal(strBuild(30, opts...)) {
		t.Errorf("a compressed list must equal the plain list with the same sequence")
	}

	empty := NewQuickList[int]()
	var nilQ *QuickList[int]
	if !nilQ.Equal(empty) || !empty.Equal(nilQ) {
		t.Errorf("nil and empty lists must be equal, both directions")
	}
	if !nilQ.Equal(nilQ) {
		t.Errorf("nil must equal nil")
	}
	if nilQ.Equal(a) || a.Equal(nilQ) {
		t.Errorf("nil/empty must not equal a non-empty list")
	}
}

func TestCapabilitiesClone(t *testing.T) {
	q := build(20, WithSegmentFill[int](8))

	dup := q.Clone()
	if !dup.Equal(q) || !q.Equal(dup) {
		t.Errorf("clone must be equal to the receiver, both directions")
	}
	checkInvariants(t, dup)

	// Deep copy: mutating the clone never affects the receiver.
	dup.PushTail(99)
	if got := q.Len(); got != 20 {
		t.Errorf("pushing to the clone changed the receiver: Len = %d, want 20", got)
	}

	// And mutating the receiver never affects the clone.
	q.PopHead()
	if got := dup.Len(); got != 21 {
		t.Errorf("popping the receiver changed the clone: Len = %d, want 21", got)
	}

	// The configuration carries over: the clone of a small-fill list
	// keeps splitting at the same fill.
	if got := dup.segCap(); got != 8 {
		t.Errorf("clone segCap = %d, want 8 (configuration not carried over)", got)
	}

	// The configuration carries over for compression too: the clone of a
	// compressed list still reads correctly and holds real windows.
	copts := []Option[string]{
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 1, EncodeStringSegment, DecodeStringSegment),
	}
	src := strBuild(30, copts...)
	cdup := src.Clone()
	if !cdup.Equal(src) {
		t.Errorf("compressed clone must equal the receiver")
	}
	checkInvariants(t, cdup)
	checkDepthWindow(t, cdup)

	if dup := NewQuickList[int]().Clone(); dup == nil || dup.Len() != 0 {
		t.Errorf("Clone of an empty list should be a fresh empty list")
	}
	var nilQ *QuickList[int]
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("Clone of a nil list should be nil, got %v", dup)
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	nonEmpty := build(3)
	for i, qq := range []*QuickList[int]{nil, {}} {
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
		other := NewQuickList[int]()
		if !qq.Equal(other) || !other.Equal(qq) {
			t.Errorf("%s: must equal an empty list, both directions", where)
		}
		if qq.Equal(nonEmpty) || nonEmpty.Equal(qq) {
			t.Errorf("%s: must not equal a non-empty list", where)
		}
	}
	var nilQ *QuickList[int]
	if got := nilQ.Slice(); got != nil {
		t.Errorf("nil list Slice = %v, want nil", got)
	}
	if dup := nilQ.Clone(); dup != nil {
		t.Errorf("nil list Clone = %v, want nil", dup)
	}
	if dup := (&QuickList[int]{}).Clone(); dup == nil || !dup.Equal(NewQuickList[int]()) {
		t.Errorf("zero-value Clone should be a fresh empty list")
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	q := NewQuickList[int](WithSegmentFill[int](8))
	var model []int // model[0] is the head
	for step := 0; step < 1500; step++ {
		switch rng.IntN(6) {
		case 0, 1: // push tail
			v := rng.IntN(50)
			q.PushTail(v)
			model = append(model, v)
		case 2: // push head
			v := rng.IntN(50)
			q.PushHead(v)
			model = append([]int{v}, model...)
		case 3: // pop head
			if len(model) > 0 {
				v, ok := q.PopHead()
				if !ok || v != model[0] {
					t.Fatalf("step %d: PopHead = %d,%v; want %d", step, v, ok, model[0])
				}
				model = model[1:]
			}
		case 4: // pop tail
			if len(model) > 0 {
				v, ok := q.PopTail()
				if !ok || v != model[len(model)-1] {
					t.Fatalf("step %d: PopTail = %d,%v; want %d", step, v, ok, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		case 5: // occasional clear
			if rng.IntN(8) == 0 {
				q.DeleteRange(0, -1)
				model = model[:0]
			}
		}
		if step%37 == 0 {
			checkInvariants(t, q)
			checkQuickModel(t, q, model, step)
		}
	}
	checkInvariants(t, q)
	checkQuickModel(t, q, model, -1)

	// The final list must not equal a rotation of itself (order counts).
	if len(model) > 1 {
		rotated := NewQuickList[int](WithSegmentFill[int](8))
		for _, v := range model[1:] {
			rotated.PushTail(v)
		}
		rotated.PushTail(model[0])
		if q.Equal(rotated) || rotated.Equal(q) {
			t.Errorf("a rotated list must not be equal to the original")
		}
	}
}

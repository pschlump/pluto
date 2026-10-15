package bag

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"strings"
	"testing"
)

func TestBag(t *testing.T) {
	var b Bag[int]
	if !b.IsEmpty() {
		t.Errorf("Expected empty bag at start.")
	}
	if b.Len() != 0 || b.Length() != 0 {
		t.Errorf("Expected length 0 at start, got %d/%d.", b.Len(), b.Length())
	}

	b.Add(1)
	b.Add(2)
	b.Add(3)
	if b.Length() != 3 {
		t.Errorf("Expected length 3, got %d.", b.Length())
	}
	if b.IsEmpty() {
		t.Errorf("Expected non-empty bag.")
	}

	var got []int
	for _, v := range b.All() {
		got = append(got, v)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("All out of order, expected [1 2 3], got %v.", got)
	}
}

// TestZeroValueSemantics verifies the constraint-free contract: the
// zero value is fully usable, including Add.
func TestZeroValueSemantics(t *testing.T) {
	var b Bag[string]

	if !b.IsEmpty() {
		t.Errorf("Expected zero value bag to be empty.")
	}
	if b.Len() != 0 || b.Length() != 0 {
		t.Errorf("Expected zero value bag to have length 0.")
	}
	for range b.All() {
		t.Errorf("Expected no values from All on zero value bag.")
	}
	for range b.Backward() {
		t.Errorf("Expected no values from Backward on zero value bag.")
	}

	b.Add("a")
	b.Add("b")
	if b.Length() != 2 {
		t.Errorf("Expected length 2 after adds, got %d.", b.Length())
	}
	var fwd []string
	for _, v := range b.All() {
		fwd = append(fwd, v)
	}
	if len(fwd) != 2 || fwd[0] != "a" || fwd[1] != "b" {
		t.Errorf("Expected [a b] in insertion order, got %v.", fwd)
	}

	b.Truncate()
	if !b.IsEmpty() || b.Length() != 0 {
		t.Errorf("Expected empty bag after Truncate.")
	}
	// The bag is reusable after Truncate.
	b.Add("c")
	if b.Length() != 1 {
		t.Errorf("Expected length 1 after Truncate + Add, got %d.", b.Length())
	}
}

// TestNilBagTolerated verifies that every operation except Add treats a
// nil bag as an empty bag, and that Add panics with a message naming
// the method — the package's only panic.
func TestNilBagTolerated(t *testing.T) {
	var b *Bag[int]

	if !b.IsEmpty() {
		t.Errorf("Expected nil bag to be empty.")
	}
	if b.Len() != 0 || b.Length() != 0 {
		t.Errorf("Expected nil bag to have length 0.")
	}
	b.Truncate() // no-op, must not panic
	for range b.All() {
		t.Errorf("Expected no values from All on nil bag.")
	}
	for range b.Backward() {
		t.Errorf("Expected no values from Backward on nil bag.")
	}
	if _, err := b.MarshalJSON(); err != nil {
		t.Errorf("MarshalJSON on nil bag: unexpected error %v.", err)
	}
	if err := b.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("UnmarshalJSON([]) on nil bag: unexpected error %v.", err)
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Add on nil bag to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Add") {
				t.Errorf("Add: unexpected panic message: %v.", r)
			}
		}()
		b.Add(1)
	}()

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements on nil bag to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") {
				t.Errorf("UnmarshalJSON: unexpected panic message: %v.", r)
			}
		}()
		_ = b.UnmarshalJSON([]byte(`[1]`))
	}()
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------------------------------------

const benchmarkBagSize = 4096

func BenchmarkAdd(b *testing.B) {
	var bg Bag[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bg.Add(i)
	}
}

func BenchmarkIterate(b *testing.B) {
	var bg Bag[int]
	for i := range benchmarkBagSize {
		bg.Add(i)
	}
	b.ResetTimer()
	for range b.N {
		for range bg.All() {
		}
	}
}

func BenchmarkIterateBackward(b *testing.B) {
	var bg Bag[int]
	for i := range benchmarkBagSize {
		bg.Add(i)
	}
	b.ResetTimer()
	for range b.N {
		for range bg.Backward() {
		}
	}
}

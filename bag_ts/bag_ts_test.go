package bag_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"strings"
	"sync"
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
}

// TestNilBagTolerated verifies that every operation except Add treats a
// nil bag as an empty bag, and that Add panics with a message naming
// the method — the package's only panic.  The nil guard runs before the
// lock is acquired (a nil-receiver lock would itself panic).
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

	for _, tc := range []struct {
		name string
		call func()
	}{
		{"Add", func() { b.Add(1) }},
		{"UnmarshalJSON", func() { _ = b.UnmarshalJSON([]byte(`[1]`)) }},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on nil bag to panic.", tc.name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, tc.name) {
					t.Errorf("%s: unexpected panic message: %v.", tc.name, r)
				}
			}()
			tc.call()
		}()
	}
}

// TestJSONRoundTrip verifies the JSON array encoding under the ts lock
// rules: snapshot under the read lock, decode before the write lock.
func TestJSONRoundTrip(t *testing.T) {
	var b Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)

	blob, err := json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(blob) != "[1,2,3]" {
		t.Fatalf("Marshal: expected [1,2,3], got %s", blob)
	}

	var b2 Bag[int]
	if err := json.Unmarshal(blob, &b2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b2.Length() != 3 {
		t.Fatalf("Expected 3 elements after decode, got %d", b2.Length())
	}
	var fwd []int
	for _, v := range b2.All() {
		fwd = append(fwd, v)
	}
	if len(fwd) != 3 || fwd[0] != 1 || fwd[1] != 2 || fwd[2] != 3 {
		t.Fatalf("Decoded order wrong: %v", fwd)
	}

	// A decode error leaves the bag untouched.
	if err := json.Unmarshal([]byte(`[4,"x"]`), &b2); err == nil {
		t.Fatalf("Expected a decode error for mixed types")
	}
	if b2.Length() != 3 {
		t.Fatalf("Decode error changed the bag: length %d", b2.Length())
	}

	// null and [] clear the bag.
	if err := json.Unmarshal([]byte(`null`), &b2); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if !b2.IsEmpty() {
		t.Fatalf("Expected empty bag after decoding null")
	}
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

func BenchmarkContendedAdd(b *testing.B) {
	var bg Bag[int]
	var wg sync.WaitGroup
	b.ResetTimer()
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				bg.Add(g)
			}
		}()
	}
	wg.Wait()
}

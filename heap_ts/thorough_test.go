// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package heap_ts

import (
	"bytes"
	"strings"
	"testing"
)

// TestZeroValueUsable verifies that a Heap declared without NewHeap works.
func TestZeroValueUsable(t *testing.T) {
	var h Heap[myHeap] // zero value, no constructor

	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("zero-value heap: expected length 0, got %d/%d", h.Len(), h.Length())
	}
	if got := h.Pop(); got != nil {
		t.Errorf("zero-value heap: Pop expected nil, got %v", *got)
	}
	if got := h.Peek(); got != nil {
		t.Errorf("zero-value heap: Peek expected nil, got %v", *got)
	}

	for i := 5; i >= 1; i-- {
		v := myHeap(i)
		h.Push(&v)
	}
	h.verify(t, 0)
	if got := h.Peek(); got == nil || int(*got) != 1 {
		t.Fatalf("zero-value heap: Peek expected 1, got %v", got)
	}

	h.Truncate()
	if h.Len() != 0 {
		t.Errorf("after Truncate: expected length 0, got %d", h.Len())
	}
	// Heap must be reusable after Truncate.
	v := myHeap(9)
	h.Push(&v)
	if got := h.Pop(); got == nil || int(*got) != 9 {
		t.Fatalf("after Truncate+Push: Pop expected 9, got %v", got)
	}
}

// TestNlMethods exercises the no-lock API as a batch under a single
// Lock/Unlock pair: NlPush, NlLen, NlGetValue, NlPop, NlDelete, NlFix.
func TestNlMethods(t *testing.T) {
	h := NewHeap[myHeap]()

	h.Lock()
	for i := 30; i >= 1; i-- {
		v := myHeap(i)
		h.NlPush(&v)
	}
	if got := h.NlLen(); got != 30 {
		t.Fatalf("NlLen: expected 30, got %d", got)
	}
	h.Unlock()
	h.verify(t, 0)

	// NlGetValue returns the root minimum.
	h.Lock()
	if got := h.NlGetValue(0); got == nil || int(*got) != 1 {
		t.Fatalf("NlGetValue(0): expected 1, got %v", got)
	}
	last := int(*h.NlGetValue(h.NlLen() - 1))

	// NlDelete of the last index (n == ii edge case).
	got := h.NlDelete(h.NlLen() - 1)
	if got == nil || int(*got) != last {
		t.Fatalf("NlDelete(last): expected %d, got %v", last, got)
	}
	// NlDelete of an interior index.
	got = h.NlDelete(5)
	if got == nil {
		t.Fatal("NlDelete(5) returned nil")
	}
	// NlDelete of a leaf-position index (no children inside the heap):
	// down() cannot move the element, so the up() fallback runs.
	got = h.NlDelete(h.NlLen() - 2)
	if got == nil {
		t.Fatal("NlDelete(leaf) returned nil")
	}
	if h.NlLen() != 27 {
		t.Fatalf("after three NlDelete: expected length 27, got %d", h.NlLen())
	}

	// NlFix: replace the root with a large value (must sink), then replace
	// an interior slot with a new minimum (must rise to the root).
	big := myHeap(1000)
	h.NlFix(0, &big)
	small := myHeap(0)
	h.NlFix(10, &small)
	if got := h.NlGetValue(0); got == nil || int(*got) != 0 {
		t.Fatalf("after NlFix: expected root 0, got %v", got)
	}

	// NlPop drains in sorted order.
	prev := -1
	for h.NlLen() > 0 {
		x := h.NlPop()
		if x == nil {
			t.Fatal("NlPop returned nil on a non-empty heap")
		}
		if int(*x) < prev {
			t.Fatalf("NlPop out of order: %d after %d", int(*x), prev)
		}
		prev = int(*x)
	}
	// NlPop on an empty heap returns nil.
	if got := h.NlPop(); got != nil {
		t.Fatalf("NlPop on empty heap: expected nil, got %v", *got)
	}
	h.Unlock()

	// The 1000 inserted by NlFix must have been part of the drain.
	if prev != 1000 {
		t.Errorf("drain max: expected 1000 (from NlFix), got %d", prev)
	}
}

// TestNlOutOfRangePanics verifies the Nl-prefixed methods panic on bad
// indexes, just like their locking counterparts.
func TestNlOutOfRangePanics(t *testing.T) {
	h := NewHeap[myHeap]()
	v := myHeap(1)
	h.Push(&v)
	nv := myHeap(2)

	for name, fn := range map[string]func(int){
		"NlGetValue": func(i int) { h.NlGetValue(i) },
		"NlDelete":   func(i int) { h.NlDelete(i) },
		"NlFix":      func(i int) { h.NlFix(i, &nv) },
	} {
		for _, idx := range []int{-1, 1, 100} {
			func() {
				h.Lock()
				defer func() {
					if recover() == nil {
						t.Errorf("%s(%d): expected panic, got none", name, idx)
					}
					h.Unlock() // release the lock taken before the panic
				}()
				fn(idx)
			}()
		}
	}

	// The heap must be untouched and still functional.
	if h.Len() != 1 {
		t.Fatalf("after panics: expected length 1, got %d", h.Len())
	}
	if got := h.Pop(); got == nil || int(*got) != 1 {
		t.Fatalf("after panics: Pop expected 1, got %v", got)
	}
}

// TestDump verifies Dump writes the heap contents to the given writer.
func TestDump(t *testing.T) {
	h := NewHeap[myHeap]()

	var buf bytes.Buffer
	h.Dump(&buf) // empty heap must not panic
	if buf.Len() == 0 {
		t.Error("Dump on empty heap wrote nothing")
	}

	for i := 1; i <= 5; i++ {
		v := myHeap(i)
		h.Push(&v)
	}
	buf.Reset()
	h.Dump(&buf)
	out := buf.String()
	if !strings.Contains(out, "[") {
		t.Errorf("Dump output does not look like a slice dump: %q", out)
	}
	for i := 1; i <= 5; i++ {
		if !strings.Contains(out, string(rune('0'+i))) {
			t.Errorf("Dump output missing value %d: %q", i, out)
		}
	}
}

// TestAllIteratorSnapshot verifies the documented snapshot semantics: the
// iterator walks a copy taken when iteration starts, so mutating the heap
// from inside the loop body is safe and does not change what is yielded.
func TestAllIteratorSnapshot(t *testing.T) {
	h := NewHeap[myHeap]()
	const n = 20
	for i := 1; i <= n; i++ {
		v := myHeap(i)
		h.Push(&v)
	}

	count := 0
	for range h.All() {
		count++
		if count == 1 {
			// Mutate heavily mid-iteration: the iterator must not notice.
			h.Truncate()
			v := myHeap(999)
			h.Push(&v)
		}
	}
	if count != n {
		t.Errorf("All over snapshot yielded %d elements, expected %d", count, n)
	}
	if h.Len() != 1 {
		t.Errorf("after in-loop mutation: expected length 1, got %d", h.Len())
	}
	if got := h.Peek(); got == nil || int(*got) != 999 {
		t.Fatalf("after in-loop mutation: Peek expected 999, got %v", got)
	}
}

// TestSearchEmpty verifies Search on an empty heap returns nil, -1, nil.
func TestSearchEmpty(t *testing.T) {
	h := NewHeap[myHeap]()
	needle := myHeap(1)
	rv, pos, err := h.Search(&needle)
	if rv != nil || pos != -1 || err != nil {
		t.Errorf("Search on empty heap: got rv=%v pos=%d err=%v, want nil/-1/nil", rv, pos, err)
	}
}

// TestTruncateIsRepeatable covers Truncate on an already-empty heap and
// repeated truncation.
func TestTruncateIsRepeatable(t *testing.T) {
	h := NewHeap[myHeap]()
	h.Truncate() // empty
	h.Truncate() // still empty
	v := myHeap(4)
	h.Push(&v)
	h.Truncate()
	h.Truncate()
	if h.Len() != 0 {
		t.Errorf("after repeated Truncate: expected length 0, got %d", h.Len())
	}
	if got := h.Pop(); got != nil {
		t.Errorf("after repeated Truncate: Pop expected nil, got %v", *got)
	}
}

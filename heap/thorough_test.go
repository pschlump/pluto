// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package heap

import (
	"bytes"
	"strings"
	"testing"
)

// TestZeroValueUsable verifies that a zero-value Heap (no NewHeap call)
// supports all basic operations.
func TestZeroValueUsable(t *testing.T) {
	var h Heap[myHeap] // zero value, no constructor

	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("zero-value heap: expected length 0, got %d/%d", h.Len(), h.Length())
	}
	if got := h.Pop(); got != nil {
		t.Errorf("zero-value heap Pop: expected nil, got %v", *got)
	}
	if got := h.Peek(); got != nil {
		t.Errorf("zero-value heap Peek: expected nil, got %v", *got)
	}

	v := myHeap(9)
	h.Push(&v)
	if h.Len() != 1 {
		t.Fatalf("zero-value heap after Push: expected length 1, got %d", h.Len())
	}
	if got := h.Peek(); got == nil || int(*got) != 9 {
		t.Fatalf("zero-value heap Peek: expected 9, got %v", got)
	}
	if got := h.Pop(); got == nil || int(*got) != 9 {
		t.Fatalf("zero-value heap Pop: expected 9, got %v", got)
	}
	if h.Len() != 0 {
		t.Errorf("zero-value heap after Pop: expected length 0, got %d", h.Len())
	}
}

// TestLockUnlockNoop verifies the API-compatibility no-op locks work and
// the heap remains fully usable across Lock/Unlock pairs.
func TestLockUnlockNoop(t *testing.T) {
	h := NewHeap[myHeap]()
	h.Lock()
	v := myHeap(4)
	h.Push(&v)
	h.Unlock()
	h.Lock()
	if got := h.Pop(); got == nil || int(*got) != 4 {
		t.Fatalf("Pop across Lock/Unlock: expected 4, got %v", got)
	}
	h.Unlock()
	if h.Len() != 0 {
		t.Errorf("expected length 0, got %d", h.Len())
	}
}

// TestNlMethods exercises the Nl-prefixed (no-lock) API-compatibility
// methods and checks they behave identically to the plain methods.
func TestNlMethods(t *testing.T) {
	h := NewHeap[myHeap]()

	if h.NlLen() != 0 {
		t.Fatalf("NlLen on empty heap: expected 0, got %d", h.NlLen())
	}
	if got := h.NlPop(); got != nil {
		t.Errorf("NlPop on empty heap: expected nil, got %v", *got)
	}

	for i := 20; i >= 1; i-- {
		v := myHeap(i)
		h.NlPush(&v)
	}
	if h.NlLen() != 20 {
		t.Fatalf("NlLen: expected 20, got %d", h.NlLen())
	}
	h.verify(t, 0)

	if got := h.NlGetValue(0); got == nil || int(*got) != 1 {
		t.Fatalf("NlGetValue(0): expected 1, got %v", got)
	}

	// NlFix: replace the min with the max; it must sink.
	nv := myHeap(100)
	h.NlFix(0, &nv)
	h.verify(t, 0)
	if got := h.Peek(); got == nil || int(*got) != 2 {
		t.Fatalf("after NlFix(0, 100): expected head 2, got %v", got)
	}

	// NlDelete: remove the element with value 100.
	needle := myHeap(100)
	_, pos, err := h.Search(&needle)
	if err != nil || pos < 0 {
		t.Fatalf("Search for 100 failed: pos=%d err=%v", pos, err)
	}
	got := h.NlDelete(pos)
	if got == nil || int(*got) != 100 {
		t.Fatalf("NlDelete(%d): expected 100, got %v", pos, got)
	}
	if h.NlLen() != 19 {
		t.Fatalf("NlLen after NlDelete: expected 19, got %d", h.NlLen())
	}
	h.verify(t, 0)

	// NlPop drains in sorted order.
	prev := 0
	for h.NlLen() > 0 {
		x := h.NlPop()
		if x == nil {
			t.Fatal("NlPop returned nil on a non-empty heap")
		}
		if int(*x) <= prev {
			t.Errorf("NlPop out of order: got %d after %d", int(*x), prev)
		}
		prev = int(*x)
		h.verify(t, 0)
	}
}

// TestSearchOnEmpty verifies Search on an empty heap returns nil, -1, nil.
func TestSearchOnEmpty(t *testing.T) {
	h := NewHeap[myHeap]()
	needle := myHeap(1)
	rv, pos, err := h.Search(&needle)
	if rv != nil || pos != -1 || err != nil {
		t.Errorf("Search on empty heap: got rv=%v pos=%d err=%v, want nil/-1/nil", rv, pos, err)
	}
}

// TestTruncateReuse verifies the heap is fully usable after Truncate.
func TestTruncateReuse(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 100; i >= 1; i-- {
		v := myHeap(i)
		h.Push(&v)
	}
	h.Truncate()
	if h.Len() != 0 {
		t.Fatalf("after Truncate: expected length 0, got %d", h.Len())
	}
	if got := h.Pop(); got != nil {
		t.Errorf("Pop after Truncate: expected nil, got %v", *got)
	}
	if got := h.Peek(); got != nil {
		t.Errorf("Peek after Truncate: expected nil, got %v", *got)
	}

	// Reuse after Truncate must rebuild a correct heap.
	for i := 10; i >= 1; i-- {
		v := myHeap(i)
		h.Push(&v)
	}
	h.verify(t, 0)
	for i := 1; i <= 10; i++ {
		got := h.Pop()
		if got == nil || int(*got) != i {
			t.Fatalf("after Truncate reuse: %d.th Pop got %v, want %d", i, got, i)
		}
	}
}

// TestDeleteZeroIsPop verifies that Delete(0) returns the minimum, matching
// Pop, as documented.
func TestDeleteZeroIsPop(t *testing.T) {
	h1 := NewHeap[myHeap]()
	h2 := NewHeap[myHeap]()
	for i := 50; i >= 1; i-- {
		v1 := myHeap(i)
		v2 := myHeap(i)
		h1.Push(&v1)
		h2.Push(&v2)
	}
	for i := 1; i <= 50; i++ {
		a := h1.Delete(0)
		b := h2.Pop()
		if a == nil || b == nil || int(*a) != int(*b) || int(*a) != i {
			t.Fatalf("%d.th: Delete(0)=%v Pop=%v, want %d", i, a, b, i)
		}
	}
}

// TestDump verifies Dump writes a representation of the heap contents.
func TestDump(t *testing.T) {
	h := NewHeap[myHeap]()
	var buf bytes.Buffer

	// Empty heap must still produce output without panicking.
	h.Dump(&buf)
	if buf.Len() == 0 {
		t.Error("Dump on empty heap produced no output")
	}

	buf.Reset()
	for i := 5; i >= 1; i-- {
		v := myHeap(i)
		h.Push(&v)
	}
	h.Dump(&buf)
	out := buf.String()
	if !strings.Contains(out, "1") {
		t.Errorf("Dump output %q does not contain the min element", out)
	}
	if buf.Len() == 0 {
		t.Error("Dump on non-empty heap produced no output")
	}
}

// TestAllIteratorMatchesInternalOrder verifies All yields exactly the
// backing-slice elements in order (it is documented as NOT sorted).
func TestAllIteratorMatchesInternalOrder(t *testing.T) {
	h := NewHeap[myHeap]()
	vals := []int{9, 3, 7, 1, 8, 2, 6}
	for _, x := range vals {
		v := myHeap(x)
		h.Push(&v)
	}
	h.verify(t, 0)

	i := 0
	for v := range h.All() {
		if v != h.data[i] {
			t.Fatalf("All element %d: got %v, want backing element %v", i, *v, *h.data[i])
		}
		i++
	}
	if i != len(vals) {
		t.Errorf("All yielded %d elements, want %d", i, len(vals))
	}
}

// TestPushSamePointerRepeatedly documents behavior when the same *T is
// pushed more than once: the heap holds duplicate references and remains a
// valid heap.
func TestPushSamePointerRepeatedly(t *testing.T) {
	h := NewHeap[myHeap]()
	v := myHeap(5)
	for i := 0; i < 10; i++ {
		h.Push(&v)
	}
	h.verify(t, 0)
	if h.Len() != 10 {
		t.Fatalf("expected length 10, got %d", h.Len())
	}
	for i := 0; i < 10; i++ {
		got := h.Pop()
		if got == nil || int(*got) != 5 {
			t.Fatalf("%d.th Pop: expected 5, got %v", i, got)
		}
	}
}

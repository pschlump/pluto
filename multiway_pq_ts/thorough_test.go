/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq_ts

import (
	"math/rand"
	"strings"
	"testing"
)

// newRand is the fixed-seed source shared by the randomized tests.
// fixed seed: deterministic run.
func newRand() *rand.Rand { return rand.New(rand.NewSource(42)) }

// expectPanic fails unless fn panics with a message containing want.
func expectPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic — wanted one containing %q", want)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T (%v), wanted a string containing %q", r, r, want)
		}
		if !strings.Contains(msg, want) {
			t.Fatalf("panic message %q does not contain %q", msg, want)
		}
	}()
	fn()
}

// checkHeapInvariant verifies the d-ary min-heap property over the
// whole backing array: every parent orders before or equal to each of
// its d children, through the heap's comparison function.
func checkHeapInvariant[T any](t *testing.T, h *DHeap[T]) {
	t.Helper()
	h.Lock()
	defer h.Unlock()
	n := h.NlLen()
	d := h.D()
	for i := range n {
		for c := d*i + 1; c <= d*i+d && c < n; c++ {
			parent, _ := h.NlGetValue(i)
			child, _ := h.NlGetValue(c)
			if h.compare(parent, child) > 0 {
				t.Fatalf("heap invariant broken: parent at %d (%v) orders after child at %d (%v)", i, parent, c, child)
			}
		}
	}
}

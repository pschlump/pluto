/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package symbol_graph

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

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4

import (
	"math/rand/v2"
	"testing"
)

// TestXXH32Incremental: the streaming xxh32s must equal the one-shot
// xxh32 for every input, however the bytes are split across write
// calls.  The two-argument split is exhaustive for every length up to
// 80 (both the sub-16 small path and the stripe/tail boundaries);
// large inputs get random chunk splits.
func TestXXH32Incremental(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))

	// Small sizes: exhaustive two-way splits (both the sub-16 small
	// path and the stripe/tail boundaries).
	for n := 0; n <= 80; n++ {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rng.IntN(256))
		}
		want := xxh32(b)
		for i := 0; i <= len(b); i++ {
			var s xxh32s
			s.reset()
			s.write(b[:i])
			s.write(b[i:])
			if got := s.sum(); got != want {
				t.Fatalf("n=%d split=%d: sum = %#08x, want %#08x", n, i, got, want)
			}
		}
	}

	// Large sizes: random many-way splits (the exhaustive split would
	// be quadratic).  Empty writes are covered by the i=0 split above.
	for _, n := range []int{1000, 65536, 1 << 20} {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rng.IntN(256))
		}
		want := xxh32(b)
		for trial := 0; trial < 8; trial++ {
			var s xxh32s
			s.reset()
			for pos := 0; pos < len(b); {
				c := min(1+rng.IntN(23), len(b)-pos) // 1..23: always progress
				s.write(b[pos : pos+c])
				pos += c
			}
			if got := s.sum(); got != want {
				t.Fatalf("n=%d trial=%d: sum = %#08x, want %#08x", n, trial, got, want)
			}
		}
	}
}

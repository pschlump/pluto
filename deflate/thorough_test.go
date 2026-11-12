/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate_test

import (
	"bytes"
	"compress/flate"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/deflate"
)

// genInput builds a deterministic pseudo-random input with the given
// shape.  shapes: 0 uniform bytes, 1 few distinct symbols (match-rich),
// 2 run-heavy, 3 text-like (skewed over a small alphabet with spaces),
// 4 alternating random and repeated segments.
func genInput(rng *rand.Rand, n int, shape int) []byte {
	data := make([]byte, n)
	switch shape {
	case 1:
		syms := []byte("abcdefgh")
		for i := range data {
			data[i] = syms[rng.IntN(len(syms))]
		}
	case 2:
		i := 0
		for i < n {
			b := byte(rng.IntN(256))
			run := 1 + rng.IntN(60)
			for j := 0; j < run && i < n; j++ {
				data[i] = b
				i++
			}
		}
	case 3:
		syms := []byte("etaoin shrdlu etaoin shrdlu\n")
		for i := range data {
			data[i] = syms[rng.IntN(len(syms))]
		}
	case 4:
		i := 0
		for i < n {
			switch rng.IntN(2) {
			case 0:
				for j := 0; j < 32 && i < n; j++ {
					data[i] = byte(rng.IntN(256))
					i++
				}
			default:
				for j := 0; j < 64 && i < n; j++ {
					data[i] = byte('A' + rng.IntN(4))
					i++
				}
			}
		}
	default:
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
	}
	return data
}

// TestRandomizedSeed42 is the property test: 400 random inputs across
// five shapes and sizes 0..60000, each round-tripped through
// Compress/Expand; every 8th is also cross-checked against the stdlib
// in BOTH directions (stdlib decodes our stream; we decode stdlib's
// streams at several levels).  The PCG seed is fixed at 42 — keep the
// test deterministic.
func TestRandomizedSeed42(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	levels := []int{flate.NoCompression, flate.BestSpeed, flate.DefaultCompression, flate.BestCompression, flate.HuffmanOnly}
	for step := 0; step < 400; step++ {
		n := rng.IntN(60001)
		shape := rng.IntN(5)
		data := genInput(rng, n, shape)
		if step%8 == 0 && n > 0 {
			n = n * 3 / 4 // vary sizes across the power-of-two boundaries too
			data = data[:n]
		}

		compressed := deflate.Compress(data)
		back, used := deflate.Expand(compressed)
		if !bytes.Equal(back, data) {
			t.Fatalf("step %d (shape %d, %d bytes): round trip mismatch (%d back)", step, shape, n, len(back))
		}
		if used != len(compressed) {
			t.Fatalf("step %d: consumed %d, want %d", step, used, len(compressed))
		}

		if step%8 == 0 {
			if out, ok := stdExpand(t, compressed); !ok || !bytes.Equal(out, data) {
				t.Fatalf("step %d: stdlib flate rejected or mismatched our stream", step)
			}
			level := levels[step/8%len(levels)]
			std := stdCompress(t, data, level)
			if out, used := deflate.Expand(std); !bytes.Equal(out, data) || used != len(std) {
				t.Fatalf("step %d: our Expand mismatched stdlib level %d", step, level)
			}
		}
	}
}

// TestRandomSizesSweep walks every length 0..1200 (crossing many
// token/lazy boundaries) with alternating shapes.
func TestRandomSizesSweep(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for n := 0; n <= 1200; n++ {
		data := genInput(rng, n, n%5)
		back, _ := deflate.Expand(deflate.Compress(data))
		if !bytes.Equal(back, data) {
			t.Fatalf("size %d shape %d: round trip mismatch", n, n%5)
		}
	}
}

// TestBitFlipCorruptionMustNotPanic flips every single bit of several
// compressed streams: Expand must return something without panicking
// (either nil for the now-corrupt stream, or a different byte string —
// corruption detection is the container's job via CRCs, not DEFLATE's).
func TestBitFlipCorruptionMustNotPanic(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 99))
	data := genInput(rng, 3000, 3)
	compressed := deflate.Compress(data)
	for _, pos := range []int{0, 1, len(compressed) / 2, len(compressed) - 1} {
		if pos >= len(compressed) {
			continue
		}
		for bit := 0; bit < 8; bit++ {
			mut := append([]byte{}, compressed...)
			mut[pos] ^= 1 << bit
			out, _ := deflate.Expand(mut) // must not panic
			_ = out
		}
	}
	// A dense sweep over one small stream.
	small := deflate.Compress(genInput(rng, 200, 1))
	for pos := 0; pos < len(small); pos++ {
		for bit := 0; bit < 8; bit++ {
			mut := append([]byte{}, small...)
			mut[pos] ^= 1 << bit
			_, _ = deflate.Expand(mut)
		}
	}
}

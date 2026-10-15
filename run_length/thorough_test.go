package run_length

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

// genRandom fills data with uniformly random bytes — runs are short,
// the worst case for run-length compression (the blob is typically
// larger than the input).
func genRandom(rng *rand.Rand, data []byte) {
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
}

// genRuns builds input from long same-byte runs of 0x00 and 0xFF —
// the regime the format targets, and the one that crosses the 255-bit
// saturation split constantly.
func genRuns(rng *rand.Rand, data []byte) {
	pos := 0
	v := byte(0x00)
	if rng.IntN(2) == 0 {
		v = 0xFF
	}
	for pos < len(data) {
		n := 1 + rng.IntN(80)
		if pos+n > len(data) {
			n = len(data) - pos
		}
		for i := pos; i < pos+n; i++ {
			data[i] = v
		}
		pos += n
		if v == 0x00 {
			v = 0xFF
		} else {
			v = 0x00
		}
	}
}

// genSparse sets single bits in a zero field — runs of zeros with
// isolated one-bits.
func genSparse(rng *rand.Rand, data []byte) {
	if len(data) == 0 {
		return
	}
	for i := range data {
		data[i] = 0
	}
	for range 1 + rng.IntN(len(data)) {
		data[rng.IntN(len(data))] |= 1 << rng.IntN(8)
	}
}

// genMixedBytes draws from {0x00, 0xFF} per byte — long runs in both
// polarities with sharp transitions.
func genMixedBytes(rng *rand.Rand, data []byte) {
	for i := range data {
		if rng.IntN(2) == 0 {
			data[i] = 0x00
		} else {
			data[i] = 0xFF
		}
	}
}

// TestRandomizedRoundTrip sweeps the input regimes at every size
// 0..600 (sizes 32 and 33 cross the 255-bit saturation split, sizes
// 64+ cross the 510 split, and so on up the gaps).  For each input:
// Compress must round-trip through Expand, and the blob's counts must
// sum to exactly 8·len(data) bits.  Fixed PCG seed for
// reproducibility.
func TestRandomizedRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	gens := []struct {
		name string
		gen  func(*rand.Rand, []byte)
	}{
		{"random", genRandom},
		{"runs", genRuns},
		{"sparse", genSparse},
		{"mixed-bytes", genMixedBytes},
	}

	for _, g := range gens {
		for size := 0; size <= 600; size++ {
			data := make([]byte, size)
			g.gen(rng, data)

			blob := Compress(data)
			back := Expand(blob)
			if !bytes.Equal(back, data) {
				t.Fatalf("%s size %d: round trip failed: %v -> %v -> %v", g.name, size, data, blob, back)
			}

			total := 0
			for _, c := range blob {
				total += int(c)
			}
			if total != 8*size {
				t.Fatalf("%s size %d: counts sum to %d bits, want %d", g.name, size, total, 8*size)
			}
		}
	}
}

// TestCanonicalForm checks the structural rules of Compress's output
// over the random regimes: a zero count appears only at index 0 (the
// alternation anchor for input that opens with a 1-bit) or
// immediately after a 255 (the second half of a saturation split).
func TestCanonicalForm(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for range 300 {
		data := make([]byte, 1+rng.IntN(600))
		genRuns(rng, data)

		blob := Compress(data)
		for i, c := range blob {
			if c == 0 && i > 0 && blob[i-1] != 255 {
				t.Fatalf("interior zero count at %d not part of a saturation split: %v", i, blob)
			}
		}
	}
}

// TestSingleByteSweep compresses every one of the 256 single bytes and
// verifies the round trip — the smallest inputs, where the anchor and
// the run boundary logic are most exposed.
func TestSingleByteSweep(t *testing.T) {
	for b := 0; b < 256; b++ {
		data := []byte{byte(b)}
		blob := Compress(data)
		back := Expand(blob)
		if !bytes.Equal(back, data) {
			t.Fatalf("byte %d: round trip failed: %v -> %v", b, blob, back)
		}
		total := 0
		for _, c := range blob {
			total += int(c)
		}
		if total != 8 {
			t.Fatalf("byte %d: counts sum to %d bits, want 8", b, total)
		}
	}
}

// TestMalformedSweep verifies over random blobs that Expand's answer
// depends only on the bit total: nil exactly when total % 8 != 0.
func TestMalformedSweep(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for range 2000 {
		blob := make([]byte, 1+rng.IntN(40))
		for i := range blob {
			blob[i] = byte(rng.IntN(256))
		}
		total := 0
		for _, c := range blob {
			total += int(c)
		}

		got := Expand(blob)
		if total%8 != 0 {
			if got != nil {
				t.Fatalf("blob %v sums to %d bits, Expand returned %v, want nil", blob, total, got)
			}
		} else {
			if got == nil && total > 0 {
				t.Fatalf("blob %v sums to %d bits, Expand returned nil, want %d bytes", blob, total, total/8)
			}
			if len(got) != total/8 {
				t.Fatalf("blob %v: Expand returned %d bytes, want %d", blob, len(got), total/8)
			}
		}
	}
}

func BenchmarkCompressRandom(b *testing.B) {
	base := make([]byte, 100_000)
	rng := uint32(42)
	for i := range base {
		rng = rng*1664525 + 1013904223
		base[i] = byte(rng >> 24)
	}
	b.SetBytes(int64(len(base)))
	b.ResetTimer()
	for range b.N {
		_ = Compress(base)
	}
}

// BenchmarkCompressRuns is the format's target regime: long runs, so
// the per-bit walk amortizes to roughly one run transition per count.
func BenchmarkCompressRuns(b *testing.B) {
	base := make([]byte, 100_000)
	for i := range base {
		if (i/64)%2 == 0 {
			base[i] = 0x00
		} else {
			base[i] = 0xFF
		}
	}
	b.SetBytes(int64(len(base)))
	b.ResetTimer()
	for range b.N {
		_ = Compress(base)
	}
}

func BenchmarkExpandRuns(b *testing.B) {
	base := make([]byte, 100_000)
	for i := range base {
		if (i/64)%2 == 0 {
			base[i] = 0x00
		} else {
			base[i] = 0xFF
		}
	}
	blob := Compress(base)
	b.SetBytes(int64(len(base)))
	b.ResetTimer()
	for range b.N {
		_ = Expand(blob)
	}
}

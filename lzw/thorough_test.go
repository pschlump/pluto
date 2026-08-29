package lzw

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Randomized round-trip property test (fixed seed)
// -------------------------------------------------------------------------------------------------------

// skewedData returns deterministic pseudo-random data of length n with
// one of three distributions, chosen by kind: uniform random bytes, a
// small alphabet (long matches, fast codebook growth), or runs of
// repeated bytes (very long matches).
func skewedData(rng *rand.Rand, n, kind int) []byte {
	data := make([]byte, 0, n)
	switch kind {
	case 0: // uniform random over all 256 byte values
		for len(data) < n {
			data = append(data, byte(rng.Intn(256)))
		}
	case 1: // a small alphabet: 2..8 distinct byte values
		alpha := make([]byte, 2+rng.Intn(7))
		for i := range alpha {
			alpha[i] = byte(rng.Intn(256))
		}
		for len(data) < n {
			data = append(data, alpha[rng.Intn(len(alpha))])
		}
	default: // runs of 1..200 copies of a small-alphabet byte
		const alphabet = "abcd"
		for len(data) < n {
			b := alphabet[rng.Intn(len(alphabet))]
			for range 1 + rng.Intn(200) {
				data = append(data, b)
			}
		}
	}
	return data[:n]
}

// TestRoundTripRandomizedModel is the property test: for every input,
// Expand(Compress(x)) == x.  Sizes ramp from 0 to 100000 over the three
// skewed distributions — the small-alphabet and run inputs grow the
// codebook well past the 9-bit and 10-bit width boundaries.
func TestRoundTripRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	for step := range 240 {
		var n int
		switch {
		case step < 100:
			n = rng.Intn(1000) // many small inputs, empty included
		case step < 180:
			n = rng.Intn(20000)
		default:
			n = rng.Intn(100001)
		}
		data := skewedData(rng, n, step%3)

		compressed := Compress(data)
		if compressed == nil {
			t.Fatalf("step %d: Compress returned nil for %d bytes", step, n)
		}
		back := Expand(compressed)
		if !bytes.Equal(back, data) {
			t.Fatalf("step %d: round trip failed for %d bytes (kind %d), got %d bytes back",
				step, n, step%3, len(back))
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Width transitions
// -------------------------------------------------------------------------------------------------------

// decodeCodewords re-walks a compressed stream, checking the codeword
// sequence and the width-widening rule, and reports the widest codeword
// width actually used.  It mirrors Expand's table growth without
// materializing the strings — an internal check that the widening of
// Compress and the widening a reader must apply stay in lockstep.
func decodeCodewords(t *testing.T, compressed []byte) (nCodes, maxW int) {
	t.Helper()
	rd := bitReader{data: compressed}
	width := minWidth
	maxW = width
	i := firstCode

	c, ok := rd.readBits(width)
	if !ok {
		t.Fatalf("stream too short for even one codeword")
	}
	nCodes++
	if c == eofCode {
		return nCodes, maxW
	}
	if c > eofCode {
		t.Fatalf("first codeword %d is out of range", c)
	}
	for {
		c, ok = rd.readBits(width)
		if !ok {
			t.Fatalf("stream has no EOF codeword")
		}
		nCodes++
		if c == eofCode {
			return nCodes, maxW
		}
		if c > i {
			t.Fatalf("codeword %d out of range (next assignable is %d)", c, i)
		}
		if i < maxCodes {
			i++
			if i == (1<<width)-1 && width < maxWidth {
				width++
				maxW = width
			}
		}
	}
}

// TestWidthTransitions compresses a >2MB low-entropy input — long runs
// over a 4-byte alphabet — which forces the codebook past 4096 entries
// and the codeword width to at least 12 bits, then verifies the round
// trip and that the widening actually happened.
func TestWidthTransitions(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	var buf bytes.Buffer
	for buf.Len() < 3<<20 {
		b := byte("abcd"[rng.Intn(4)])
		for range 1 + rng.Intn(200) {
			buf.WriteByte(b)
		}
	}
	data := buf.Bytes()

	compressed := checkRoundTrip(t, data)

	nCodes, maxW := decodeCodewords(t, compressed)
	if maxW < 12 {
		t.Fatalf("expected the codeword width to reach at least 12 bits, got %d (%d codes)", maxW, nCodes)
	}
}

// TestFullCodebook keeps feeding a high-entropy stream until the
// codebook fills all 65536 entries (width 16), verifying that
// compression keeps working — correctly, without widening past 16 and
// without resetting the table — after the codebook is full.
func TestFullCodebook(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	// Uniform random bytes are near-incompressible, so the codebook
	// grows by one entry per codeword emitted: ~70000 codes fill it.
	data := make([]byte, 600000)
	rng.Read(data)

	compressed := checkRoundTrip(t, data)

	_, maxW := decodeCodewords(t, compressed)
	if maxW != maxWidth {
		t.Fatalf("expected the codeword width to reach %d bits, got %d", maxWidth, maxW)
	}
}

// -------------------------------------------------------------------------------------------------------
// Malformed input: Expand reports nil, never panics
// -------------------------------------------------------------------------------------------------------

// TestMalformedTruncation cuts a valid stream at every byte boundary:
// decoding is prefix-deterministic, so every strict prefix lacks the
// EOF codeword and must be reported malformed.
func TestMalformedTruncation(t *testing.T) {
	compressed := Compress([]byte("TOBEORNOTTOBEORTOBEORNOT"))
	for cut := range len(compressed) {
		if got := Expand(compressed[:cut]); got != nil {
			t.Fatalf("Expand of the stream truncated to %d bytes = %q, expected nil", cut, got)
		}
	}
}

// TestMalformedMissingEOF builds streams with valid codewords but no
// EOF terminator.
func TestMalformedMissingEOF(t *testing.T) {
	var w bitWriter
	w.writeBits('a', minWidth)
	if got := Expand(w.bytes()); got != nil {
		t.Fatalf("Expand of one unterminated codeword = %q, expected nil", got)
	}

	w = bitWriter{}
	w.writeBits('a', minWidth)
	w.writeBits('b', minWidth)
	w.writeBits(257, minWidth) // "ab" — all valid, still no EOF
	if got := Expand(w.bytes()); got != nil {
		t.Fatalf("Expand of valid codewords without EOF = %q, expected nil", got)
	}
}

// TestMalformedOutOfRange feeds codewords no encoder can have emitted.
func TestMalformedOutOfRange(t *testing.T) {
	// The first codeword must be a literal (0..255) or EOF (256).
	var w bitWriter
	w.writeBits(300, minWidth)
	w.writeBits(eofCode, minWidth)
	if got := Expand(w.bytes()); got != nil {
		t.Fatalf("Expand of a stream starting at codeword 300 = %q, expected nil", got)
	}

	// A codeword more than one step ahead of the decoder's table: after
	// 'a' the decoder has assigned nothing, so 258 is out of range
	// (257 would be the legal KwKwK step).
	w = bitWriter{}
	w.writeBits('a', minWidth)
	w.writeBits(258, minWidth)
	w.writeBits(eofCode, minWidth)
	if got := Expand(w.bytes()); got != nil {
		t.Fatalf("Expand of a stream jumping to codeword 258 = %q, expected nil", got)
	}
}

// TestMalformedGarbage verifies that arbitrary bytes never panic
// Expand.  Whatever garbage decodes to a valid stream at all is
// harmless; the malformed cases must come back nil.
func TestMalformedGarbage(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	if got := Expand(nil); got != nil {
		t.Fatalf("Expand(nil) = %q, expected nil", got)
	}
	for range 500 {
		data := make([]byte, rng.Intn(64))
		rng.Read(data)
		Expand(data) // must not panic; the result is don't-care
	}
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test
// -------------------------------------------------------------------------------------------------------

// TestConcurrentUse hammers Compress and Expand from many goroutines.
// The functions are stateless, so concurrent use is the package's
// concurrency guarantee.  Run under -race.
func TestConcurrentUse(t *testing.T) {
	inputs := [][]byte{
		[]byte("TOBEORNOTTOBEORTOBEORNOT"),
		[]byte("ABABABABABABAB"),
		skewedData(rand.New(rand.NewSource(42)), 5000, 1),
	}
	compressed := make([][]byte, len(inputs))
	for i, data := range inputs {
		compressed[i] = Compress(data)
	}

	const workers = 8
	const rounds = 50
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := inputs[w%len(inputs)]
			want := compressed[w%len(compressed)]
			for range rounds {
				if got := Compress(data); !bytes.Equal(got, want) {
					t.Errorf("concurrent Compress disagrees with the sequential result")
					return
				}
				if got := Expand(want); !bytes.Equal(got, data) {
					t.Errorf("concurrent Expand disagrees with the sequential result")
					return
				}
			}
		}()
	}
	wg.Wait()
}

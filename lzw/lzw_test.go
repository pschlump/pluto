package lzw

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"math/rand"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Known vectors (pin the exact bit format against regressions)
// -------------------------------------------------------------------------------------------------------

// TestKnownVectorTOBEORNOT pins the compressed byte stream of the algs4
// §5.5 textbook example "TOBEORNOTTOBEORTOBEORNOT" to a hand-traced
// codeword sequence.  The trace:
//
//	remaining input        match  emitted  added to table
//	TOBEORNOTTOBEORTOBEORNOT
//	OBEORNOTTOBEORTOBEORNOT   T      84     TO=257
//	BEORNOTTOBEORTOBEORNOT    O      79     OB=258
//	EORNOTTOBEORTOBEORNOT     B      66     BE=259
//	ORNOTTOBEORTOBEORNOT      E      69     EO=260
//	RNOTTOBEORTOBEORNOT       O      79     OR=261
//	NOTTOBEORTOBEORNOT        R      82     RN=262
//	OTTOBEORTOBEORNOT         N      78     NO=263
//	TTOBEORTOBEORNOT          O      79     OT=264
//	TOBEORTOBEORNOT           T      84     TT=265
//	BEORTOBEORNOT             TO    257     TOB=266
//	ORTOBEORNOT               BE    259     BEO=267
//	TOBEORNOT                 OR    261     ORT=268
//	EORNOT                    TOB   266     TOBE=269
//	RNOT                      EO    260     EOR=270
//	OT                        RN    262     RNO=271
//	(end of input)            OT    264     (no entry: match reached the end)
//	                              256     EOF
//
// Every codeword is < 512, so the width stays 9 throughout.  The 17
// codewords pack MSB-first into 153 bits = 20 bytes (the final byte
// carries 1 code bit and 7 zero padding bits).
func TestKnownVectorTOBEORNOT(t *testing.T) {
	want := []byte{
		0x2a, 0x13, 0xc8, 0x44, 0x52, 0x79, 0x48, 0x9c, 0x4f, 0x2a,
		0x40, 0x60, 0x70, 0x58, 0x54, 0x12, 0x0d, 0x08, 0x80, 0x00,
	}
	got := Compress([]byte("TOBEORNOTTOBEORTOBEORNOT"))
	if !bytes.Equal(got, want) {
		t.Fatalf("Compress(TOBEORNOTTOBEORTOBEORNOT) = %x, expected %x", got, want)
	}
	if back := Expand(want); !bytes.Equal(back, []byte("TOBEORNOTTOBEORTOBEORNOT")) {
		t.Fatalf("Expand of the known vector = %q", back)
	}
}

// TestKnownVectorKwKwK pins the stream of "AAAAAA", whose third
// codeword (258, "AAA") is the KwKwK case: the encoder emits a codeword
// one step before the decoder can have it in its table.  The trace:
//
//	remaining input  match  emitted  added to table
//	AAAAAA
//	AAAAA             A       65     AA=257
//	AAA               AA     257     AAA=258
//	(end of input)    AAA    258     (no entry: match reached the end)
//	                           256   EOF
//
// Decoder side: after emitting "AA" for codeword 257 the next codeword
// read is 258 == i, so s = val+val[:1] = "AA"+"A" = "AAA".  All
// codewords are 9 bits (65 = 001000001, 257 = 100000001, ...), packed
// MSB-first:
//
//	001000001 100000001 100000010 100000000 -> 20 c0 60 50 00
func TestKnownVectorKwKwK(t *testing.T) {
	want := []byte{0x20, 0xc0, 0x60, 0x50, 0x00}
	got := Compress([]byte("AAAAAA"))
	if !bytes.Equal(got, want) {
		t.Fatalf("Compress(AAAAAA) = %x, expected %x", got, want)
	}
	if back := Expand(want); !bytes.Equal(back, []byte("AAAAAA")) {
		t.Fatalf("Expand of the KwKwK vector = %q", back)
	}
}

// TestKnownVectorEmpty pins the empty-input stream: just the 9-bit EOF
// codeword 256 = 0b100000000, packing to 0x80 then 7 padding zeros.
func TestKnownVectorEmpty(t *testing.T) {
	want := []byte{0x80, 0x00}
	if got := Compress(nil); !bytes.Equal(got, want) {
		t.Fatalf("Compress(nil) = %x, expected %x", got, want)
	}
	if got := Compress([]byte{}); !bytes.Equal(got, want) {
		t.Fatalf("Compress([]byte{}) = %x, expected %x", got, want)
	}
	back := Expand(want)
	if back == nil || len(back) != 0 {
		t.Fatalf("Expand of the empty stream = %v, expected an empty non-nil slice", back)
	}
}

// -------------------------------------------------------------------------------------------------------
// Round trips
// -------------------------------------------------------------------------------------------------------

// checkRoundTrip verifies Compress/Expand on data and returns the
// compressed stream for further poking.
func checkRoundTrip(t *testing.T, data []byte) []byte {
	t.Helper()
	compressed := Compress(data)
	if compressed == nil {
		t.Fatalf("Compress returned nil for input of length %d", len(data))
	}
	back := Expand(compressed)
	if !bytes.Equal(back, data) {
		t.Fatalf("round trip of %d bytes failed (got %d bytes back)", len(data), len(back))
	}
	return compressed
}

func TestRoundTripBasic(t *testing.T) {
	cases := [][]byte{
		{},                                 // empty
		{0},                                // single NUL byte
		{'a'},                              // single byte
		{0, 0, 0, 0, 0, 0, 0},              // a run of NULs (KwKwK on byte 0)
		[]byte("ABABABABABABAB"),           // the algs4 ababLZW pattern
		[]byte("TOBEORNOTTOBEORTOBEORNOT"), // the algs4 §5.5 example
		[]byte("the quick brown fox jumps over the lazy dog. " +
			"the quick brown fox jumps over the lazy dog."),
	}
	for i, data := range cases {
		compressed := checkRoundTrip(t, data)
		if len(compressed) == 0 {
			t.Fatalf("case %d: Compress produced an empty stream", i)
		}
	}
}

// TestRoundTripAllByteValues compresses every byte value 0..255 in
// order: the whole literal alphabet appears, in an order the TST never
// inserted as multi-byte keys before.
func TestRoundTripAllByteValues(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	checkRoundTrip(t, data)
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks (compress + expand 1MB of text-like data)
// -------------------------------------------------------------------------------------------------------

// benchmarkText returns a deterministic pseudo-English text of about n
// bytes built from a small word list — repetitive enough that the
// codebook grows well past the 9-bit boundary.
func benchmarkText(n int) []byte {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic text
	words := []string{
		"the ", "quick ", "brown ", "fox ", "jumps ", "over ", "the ",
		"lazy ", "dog ", "and ", "runs ", "through ", "forest ",
		"compression ", "algorithms ", "data ", "stream ",
	}
	var buf bytes.Buffer
	for buf.Len() < n {
		buf.WriteString(words[rng.Intn(len(words))])
	}
	return buf.Bytes()[:n]
}

func BenchmarkCompress(b *testing.B) {
	data := benchmarkText(1 << 20)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		Compress(data)
	}
}

func BenchmarkExpand(b *testing.B) {
	compressed := Compress(benchmarkText(1 << 20))
	b.SetBytes(int64(len(compressed)))
	b.ResetTimer()
	for range b.N {
		Expand(compressed)
	}
}

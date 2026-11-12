/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate_test

import (
	"bytes"
	"compress/flate"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/deflate"
)

// corpusEntry is one named corpus input shared by the round-trip,
// interop and truncation tests.
type corpusEntry struct {
	name string
	data []byte
}

// stdCompress compresses with the stdlib flate writer at the given
// level — the conformance oracle for our encoder.
func stdCompress(t *testing.T, data []byte, level int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		t.Fatalf("flate.NewWriter: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	return buf.Bytes()
}

// stdExpand decompresses with the stdlib flate reader — the
// conformance oracle for our decoder.
func stdExpand(t *testing.T, data []byte) ([]byte, bool) {
	t.Helper()
	r := flate.NewReader(bytes.NewReader(data))
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, false
	}
	return out, true
}

// buildCorpus returns the shared deterministic corpus.  It exercises
// every block type, match geometry and size boundary: empty and tiny
// inputs, long runs (max-length matches), overlapping matches, a
// match at exactly the 32768 window limit, incompressible data
// (stored blocks), inputs that straddle the 65535-byte stored-block
// cap, Fibonacci-skewed byte frequencies (code lengths past the
// 15-bit limit), and multi-block sizes.
func buildCorpus() []corpusEntry {
	rng := rand.New(rand.NewPCG(42, 42))

	text := strings.Repeat(
		"the quick brown fox jumps over the lazy dog. "+ //nolint:uselessly-nolint // deterministic text corpus
			"Call me Ishmael. Some years ago—never mind how long "+
			"precisely—having little or no money in my purse, and "+
			"nothing particular to interest me on shore, ", 700)

	far := make([]byte, 40000)
	for i := range far {
		far[i] = byte(rng.IntN(256))
	}
	for i := 32768; i < len(far); i++ {
		far[i] = far[i-32768] // distance exactly at the window limit
	}

	skew := make([]byte, 0, 120000)
	a, b := 1, 1
	for s := 0; s < 24; s++ {
		for i := 0; i < a; i++ {
			skew = append(skew, byte(s))
		}
		a, b = b, a+b // Fibonacci frequencies: optimal codes exceed 15 bits
	}
	for i := 0; i < 24; i++ {
		skew = append(skew, byte(rng.IntN(256)))
	}

	var pattern []byte
	for i := 0; i < 25600; i++ {
		pattern = append(pattern, byte(i*7+13))
	}

	random64k := make([]byte, 65536)
	for i := range random64k {
		random64k[i] = byte(rng.IntN(256))
	}

	mixed := make([]byte, 0, 300000)
	seg := make([]byte, 4096)
	for len(mixed) < 300000 {
		switch len(mixed) % 3 {
		case 0:
			for i := range seg {
				seg[i] = byte(rng.IntN(256))
			}
			mixed = append(mixed, seg...)
		case 1:
			mixed = append(mixed, bytes.Repeat([]byte("0123456789"), 100)...)
		default:
			mixed = append(mixed, text[:2000]...)
		}
	}

	return []corpusEntry{
		{"empty-nil", nil},
		{"empty", []byte{}},
		{"single", []byte("a")},
		{"two", []byte("ab")},
		{"to-be-or-not", []byte("TOBEORNOTTOBEORTOBEORNOT")},
		{"run-a", bytes.Repeat([]byte("a"), 100000)},
		{"overlap-abc", bytes.Repeat([]byte("abc"), 10000)},
		{"all-bytes-pattern", pattern},
		{"text", []byte(text)},
		{"random-64k", random64k},
		{"far-match-32768", far},
		{"skewed-fib", skew},
		{"size-65534", mixed[:65534]},
		{"size-65535", mixed[:65535]},
		{"size-65536", mixed[:65536]},
		{"size-65537", mixed[:65537]},
		{"mixed-300k", mixed},
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range buildCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			compressed := deflate.Compress(tc.data)
			if compressed == nil {
				t.Fatalf("Compress(%s) returned nil", tc.name)
			}
			if len(tc.data) == 0 && len(compressed) != 2 {
				t.Fatalf("empty input: got %d bytes, want the 2-byte final block 03 00", len(compressed))
			}
			back, n := deflate.Expand(compressed)
			if !bytes.Equal(back, tc.data) {
				t.Fatalf("round trip mismatch: got %d bytes, want %d", len(back), len(tc.data))
			}
			if n != len(compressed) {
				t.Fatalf("consumed = %d, want %d", n, len(compressed))
			}
		})
	}
}

func TestEmptyStreamGolden(t *testing.T) {
	got := deflate.Compress(nil)
	want := []byte{0x03, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("Compress(nil) = % x, want % x", got, want)
	}
	out, n := deflate.Expand(want)
	if len(out) != 0 || n != 2 {
		t.Fatalf("Expand(03 00) = (%d bytes, %d), want (0, 2)", len(out), n)
	}
}

// TestStdlibReadsOurs is the encoder conformance oracle: the stdlib
// flate reader must decode every stream we produce.
func TestStdlibReadsOurs(t *testing.T) {
	for _, tc := range buildCorpus() {
		compressed := deflate.Compress(tc.data)
		back, ok := stdExpand(t, compressed)
		if !ok {
			t.Fatalf("%s: stdlib flate rejected our stream", tc.name)
		}
		if !bytes.Equal(back, tc.data) {
			t.Fatalf("%s: stdlib flate decoded %d bytes, want %d", tc.name, len(back), len(tc.data))
		}
	}
}

// TestWeReadStdlib is the decoder conformance oracle: we must decode
// every stream the stdlib flate writer produces, at several levels
// (each level emits a different mix of block types; NoCompression
// emits stored, HuffmanOnly emits fixed, the rest dynamic).
func TestWeReadStdlib(t *testing.T) {
	levels := []int{
		flate.NoCompression,
		flate.BestSpeed,
		flate.DefaultCompression,
		flate.BestCompression,
		flate.HuffmanOnly,
	}
	for _, tc := range buildCorpus() {
		for _, level := range levels {
			compressed := stdCompress(t, tc.data, level)
			back, n := deflate.Expand(compressed)
			if !bytes.Equal(back, tc.data) {
				t.Fatalf("%s level %d: decoded %d bytes, want %d", tc.name, level, len(back), len(tc.data))
			}
			if n != len(compressed) {
				t.Fatalf("%s level %d: consumed %d, want %d", tc.name, level, n, len(compressed))
			}
		}
	}
}

func TestExpandTrailingInput(t *testing.T) {
	data := []byte("trailing bytes are not part of the stream")
	compressed := deflate.Compress(data)
	padded := append(append([]byte{}, compressed...), 0xFF, 0x00, 0x55)
	back, n := deflate.Expand(padded)
	if !bytes.Equal(back, data) {
		t.Fatalf("decoded %q, want %q", back, data)
	}
	if n != len(compressed) {
		t.Fatalf("consumed %d, want %d", n, len(compressed))
	}
}

func TestTruncatedStreamsAreMalformed(t *testing.T) {
	for _, tc := range buildCorpus() {
		compressed := deflate.Compress(tc.data)
		if len(compressed) < 2 {
			continue
		}
		for _, cut := range []int{0, 1, len(compressed) / 2, len(compressed) - 1} {
			if out, n := deflate.Expand(compressed[:cut]); out != nil || n != 0 {
				t.Fatalf("%s cut at %d: proper prefix of a stream decoded to (%d bytes, %d), want malformed",
					tc.name, cut, len(out), n)
			}
		}
	}
}

// TestMalformedStreams covers crafted bad streams, each annotated
// with the bytes' meaning.  All must report (nil, 0), never panic.
func TestMalformedStreams(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one-bit", []byte{0x01}},          // final+stored header, then nothing
		{"reserved-btype-3", []byte{0x07}}, // BFINAL=1, BTYPE=11
		{"stored-len-mismatch", []byte{0x01, 0x01, 0x00, 0x00, 0x00}},        // LEN=1, NLEN=0
		{"stored-truncated-data", []byte{0x01, 0x05, 0x00, 0xFA, 0xFF, 'x'}}, // LEN=5, 1 byte present
		{"dynamic-oversubscribed-cl", []byte{0x05, 0x00, 0xF8, 0x1F}},        // 4 one-bit CL codes
		{"match-into-empty-history", []byte{0x05, 0x02}},                     // fixed: length-3 match, dist 1, no output yet
		{"fixed-symbol-286", []byte{0x1D, 0x03}},                             // fixed code for reserved symbol 286
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, n := deflate.Expand(tc.in)
			if out != nil || n != 0 {
				t.Fatalf("Expand(% x) = (%d bytes, %d), want (nil, 0)", tc.in, len(out), n)
			}
		})
	}
}

func TestStoredBlockGolden(t *testing.T) {
	// A hand-built valid stored block: BFINAL=1, BTYPE=00, align,
	// LEN=1, NLEN=~1, one data byte 'A'.
	in := []byte{0x01, 0x01, 0x00, 0xFE, 0xFF, 'A'}
	out, n := deflate.Expand(in)
	if string(out) != "A" || n != len(in) {
		t.Fatalf("Expand = (%q, %d), want (A, %d)", out, n, len(in))
	}
}

func TestRoundTripSizes(t *testing.T) {
	// Sweep small sizes 0..300 — the token/lazy logic edges.
	rng := rand.New(rand.NewPCG(7, 7))
	for n := 0; n <= 300; n++ {
		data := make([]byte, n)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		back, _ := deflate.Expand(deflate.Compress(data))
		if !bytes.Equal(back, data) {
			t.Fatalf("size %d: round trip mismatch", n)
		}
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman

import (
	"bytes"
	"testing"
)

// The known vectors are hand-traced from the format: [trie preorder]
// [64-bit count] [codewords], bits MSB-first, final partial byte
// zero-padded.  Every byte below was derived on paper before it was
// run — see the per-case notes.
func TestKnownVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{
			// Two symbols, equal frequency: A (seq 0) pops first, so it
			// hangs LEFT — A = "0", B = "1".  Trie: 0 | 1 01000001 |
			// 1 01000010 (19 bits); count 2 (64 bits); codes 0,1.
			// 85 bits total → 11 bytes.
			name: "AB",
			in:   "AB",
			want: []byte{0x50, 0x68, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x48},
		},
		{
			// Single distinct symbol: the root is A's leaf, the codeword
			// is the empty string, and the message costs zero bits.
			// Trie: 1 01000001 (9 bits); count 3 (64 bits).  73 bits →
			// 10 bytes.
			name: "AAA",
			in:   "AAA",
			want: []byte{0xA0, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x80},
		},
		{
			// Three symbols' skew in miniature: freq A=2 (seq 0), B=1
			// (seq 1) — B pops first (smaller frequency wins before the
			// tie-break matters), so B hangs LEFT: B = "0", A = "1".
			// Trie: 0 | 1 01000010 | 1 01000001; count 3; codes
			// 1,1,0.  86 bits → 11 bytes.
			name: "AAB",
			in:   "AAB",
			want: []byte{0x50, 0xA8, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x78},
		},
		{
			// One byte: same single-leaf shape as AAA, count 1.  'X' =
			// 01011000.  73 bits → 10 bytes.
			name: "X",
			in:   "X",
			want: []byte{0xAC, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compress([]byte(tc.in))
			if !bytes.Equal(got, tc.want) {
				t.Errorf("Compress(%q) =\n % x\nwant\n % x", tc.in, got, tc.want)
			}
			back := Expand(tc.want)
			if string(back) != tc.in {
				t.Errorf("Expand(known vector) = %q, want %q", back, tc.in)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	if got := Compress(nil); got != nil {
		t.Errorf("Compress(nil) = % x, want nil", got)
	}
	if got := Compress([]byte{}); got != nil {
		t.Errorf("Compress(empty) = % x, want nil", got)
	}
	if got := Expand(nil); got != nil {
		t.Errorf("Expand(nil) = % x, want nil", got)
	}
	if got := Expand([]byte{}); got != nil {
		t.Errorf("Expand(empty) = % x, want nil", got)
	}
}

// A single-symbol message is exactly the 9-bit trie plus the 64-bit
// count — 73 bits, 10 bytes — no matter how long the input.  This is
// the zero-bits-per-symbol property, and it is why Huffman beats every
// other member of the §5.5 family on constant data.
func TestSingleSymbolTenBytes(t *testing.T) {
	for _, n := range []int{1, 2, 100, 1000, 1_000_000} {
		data := bytes.Repeat([]byte{'Q'}, n)
		blob := Compress(data)
		if len(blob) != 10 {
			t.Errorf("len(Compress(%d × 'Q')) = %d, want 10", n, len(blob))
		}
		if back := Expand(blob); !bytes.Equal(back, data) {
			t.Errorf("round trip of %d × 'Q' failed", n)
		}
	}
}

// With exactly two distinct symbols the trie can only be a root with
// two leaves — both codewords exactly one bit — so the blob length is
// a closed form: 19 (trie) + 64 (count) + n (codewords) bits.
func TestTwoSymbolExactLength(t *testing.T) {
	for _, n := range []int{2, 3, 10, 100, 1000} {
		data := make([]byte, n)
		for i := range data {
			data[i] = byte('A' + i%2)
		}
		want := (19 + 64 + n + 7) / 8
		if got := len(Compress(data)); got != want {
			t.Errorf("len(Compress(%d alternating)) = %d, want %d", n, got, want)
		}
	}
}

// The trie a blob carries is a full binary tree over at most 256
// leaves satisfying Kraft's equality — the prefix-free condition.  This
// is the structural invariant of the format, checked from the parsed
// blob (not from Compress's internals).
func TestBlobTrieInvariants(t *testing.T) {
	inputs := [][]byte{
		[]byte("AB"),
		[]byte("AAA"),
		bytes.Repeat([]byte("abracadabra"), 5),
		{0x00, 0xFF, 0x7F, 0x80, 0x00, 0xFF},
	}
	for _, in := range inputs {
		root, ok := parseTrie(Compress(in))
		if !ok {
			t.Fatalf("parseTrie of Compress(%q) failed", in)
		}
		checkTrieInvariants(t, root, symbolSet(in))
	}
}

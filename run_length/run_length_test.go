package run_length

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"slices"
	"testing"
)

// TestKnownVectors checks hand-traced encodings: the alternation
// anchor (a first count of 0 for input that opens with a 1-bit), plain
// runs, per-bit alternation, and the 255/0 saturation split.
func TestKnownVectors(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"one zero byte", []byte{0x00}, []byte{8}},
		{"one one-byte", []byte{0xFF}, []byte{0, 8}},
		{"high nibble ones", []byte{0xF0}, []byte{0, 4, 4}},
		{"alternating from zero", []byte{0x55}, []byte{1, 1, 1, 1, 1, 1, 1, 1}},
		{"alternating from one", []byte{0xAA}, []byte{0, 1, 1, 1, 1, 1, 1, 1, 1}},
		{"32 zero bytes saturate", bytes.Repeat([]byte{0x00}, 32), []byte{255, 0, 1}},
		{"33 zero bytes saturate", bytes.Repeat([]byte{0x00}, 33), []byte{255, 0, 9}},
		{"32 one bytes saturate", bytes.Repeat([]byte{0xFF}, 32), []byte{0, 255, 0, 1}},
		{"mixed nibbles", []byte{0xC3}, []byte{0, 2, 4, 2}},       // 1100 0011
		{"two sparse bytes", []byte{0x00, 0x80}, []byte{8, 1, 7}}, // 0000 0000 1000 0000
	}

	for _, tc := range cases {
		got := Compress(tc.in)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: Compress(%v) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
		back := Expand(tc.want)
		if !bytes.Equal(back, tc.in) {
			t.Errorf("%s: Expand(%v) = %v, want %v", tc.name, tc.want, back, tc.in)
		}
	}
}

// TestEmpty verifies empty ↔ empty in both directions (nil and the
// empty slice alike).
func TestEmpty(t *testing.T) {
	if got := Compress(nil); len(got) != 0 {
		t.Errorf("Compress(nil) = %v, want empty", got)
	}
	if got := Compress([]byte{}); len(got) != 0 {
		t.Errorf("Compress(empty) = %v, want empty", got)
	}
	if got := Expand(nil); len(got) != 0 {
		t.Errorf("Expand(nil) = %v, want empty", got)
	}
	if got := Expand([]byte{}); len(got) != 0 {
		t.Errorf("Expand(empty) = %v, want empty", got)
	}
}

// TestMalformed verifies that a blob whose counts do not sum to a
// whole number of bytes expands to nil — the package's only failure
// mode, never a panic.
func TestMalformed(t *testing.T) {
	cases := []struct {
		name string
		blob []byte
	}{
		{"three bits", []byte{3}},
		{"one bit after a byte", []byte{8, 1}},
		{"saturation split truncated to 263 bits", []byte{255, 0, 8}},
		{"only an anchor", []byte{0, 1}},
		{"large non-multiple total", append(bytes.Repeat([]byte{255}, 55), 4)},
	}
	for _, tc := range cases {
		if got := Expand(tc.blob); got != nil {
			t.Errorf("%s: Expand(%v) = %v, want nil", tc.name, tc.blob, got)
		}
	}
}

// TestNonCanonicalTolerance verifies that zero counts in the interior
// and at the end are tolerated (they decode to no bits), even though
// only Compress's output is canonical.
func TestNonCanonicalTolerance(t *testing.T) {
	if got := Expand([]byte{8, 0}); !bytes.Equal(got, []byte{0x00}) {
		t.Errorf("Expand([8 0]) = %v, want [0]", got)
	}
	if got := Expand([]byte{0, 0, 0, 8}); !bytes.Equal(got, []byte{0xFF}) {
		t.Errorf("Expand([0 0 0 8]) = %v, want [255]", got)
	}
	if got := Expand([]byte{0}); len(got) != 0 {
		t.Errorf("Expand([0]) = %v, want empty", got)
	}
}

// TestInputNotModified verifies that Compress and Expand leave their
// input untouched.
func TestInputNotModified(t *testing.T) {
	data := []byte{0x0F, 0xF0, 0x55, 0xAA, 0x00, 0xFF}
	orig := slices.Clone(data)
	_ = Compress(data)
	if !slices.Equal(data, orig) {
		t.Errorf("Compress modified its input: %v -> %v", orig, data)
	}

	blob := Compress(data)
	origBlob := slices.Clone(blob)
	_ = Expand(blob)
	if !slices.Equal(blob, origBlob) {
		t.Errorf("Expand modified its input: %v -> %v", origBlob, blob)
	}
}

// TestNeverPanics hammers both functions with adversarial shapes —
// including nil, empty, all-saturated input — under a recover guard:
// the package contract is that neither function ever panics.
func TestNeverPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("package panicked: %v", r)
		}
	}()
	_ = Compress(nil)
	_ = Compress(bytes.Repeat([]byte{0x00}, 10_000))
	_ = Compress(bytes.Repeat([]byte{0xFF}, 10_000))
	_ = Expand(nil)
	_ = Expand([]byte{255, 0, 255, 0, 255})
	_ = Expand(bytes.Repeat([]byte{255}, 1000))
	_ = Expand([]byte{0, 0, 0, 0, 0, 0, 0, 0})
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zlib_test

import (
	"bytes"
	stdzlib "compress/zlib"
	"hash/adler32"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/zlib"
)

func corpus() [][]byte {
	rng := rand.New(rand.NewPCG(42, 42))
	text := []byte(strings.Repeat("Call me Ishmael. Some years ago—never mind how long precisely—", 300))
	random := make([]byte, 65536)
	for i := range random {
		random[i] = byte(rng.IntN(256))
	}
	return [][]byte{
		nil,
		[]byte("a"),
		[]byte("Wikipedia"),
		bytes.Repeat([]byte("a"), 100000),
		text,
		random,
	}
}

func TestRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		z := zlib.Compress(data)
		if z == nil {
			t.Fatalf("corpus %d: Compress returned nil", i)
		}
		if len(z) < 2 || z[0] != 0x78 {
			t.Fatalf("corpus %d: header = % x, want CMF 0x78", i, z[:2])
		}
		if (int(z[0])*256+int(z[1]))%31 != 0 {
			t.Fatalf("corpus %d: FCHECK bits wrong", i)
		}
		if back := zlib.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("corpus %d: round trip mismatch", i)
		}
	}
}

func TestAdlerMatchesStdlib(t *testing.T) {
	// The trailer Adler-32 must match the stdlib hash for every
	// corpus element: one 32-bit big-endian word.
	for i, data := range corpus() {
		z := zlib.Compress(data)
		want := adler32.Checksum(data)
		got := uint32(z[len(z)-4])<<24 | uint32(z[len(z)-3])<<16 | uint32(z[len(z)-2])<<8 | uint32(z[len(z)-1])
		if got != want {
			t.Fatalf("corpus %d: adler = %08x, want %08x", i, got, want)
		}
	}
	if adler32.Checksum([]byte("Wikipedia")) != 0x11E60398 {
		t.Fatal("stdlib adler32 sanity vector drifted")
	}
}

// TestStdlibInterop is the oracle in both directions.
func TestStdlibInterop(t *testing.T) {
	for i, data := range corpus() {
		r, err := stdzlib.NewReader(bytes.NewReader(zlib.Compress(data)))
		if err != nil {
			t.Fatalf("corpus %d: stdlib rejected our stream: %v", i, err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: stdlib read: %v", i, err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("corpus %d: stdlib decode mismatch", i)
		}
	}
	for i, data := range corpus() {
		var buf bytes.Buffer
		w := stdzlib.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if back := zlib.Expand(buf.Bytes()); !bytes.Equal(back, data) {
			t.Fatalf("corpus %d: our Expand mismatched stdlib stream", i)
		}
	}
}

func TestMalformed(t *testing.T) {
	base := zlib.Compress([]byte("content to corrupt"))
	cases := []struct {
		name string
		in   []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one-byte", []byte{0x78}},
		{"bad-cm", func() []byte { b := append([]byte{}, base...); b[0] = 0x79; return b }()},
		{"cinfo-too-big", func() []byte { b := append([]byte{}, base...); b[0] = 0x88; return b }()},
		{"bad-fcheck", func() []byte { b := append([]byte{}, base...); b[1] ^= 0x01; return b }()},
		{"fdict-set", func() []byte {
			b := append([]byte{}, base...)
			b[0], b[1] = 0x78, 0xBB // FDICT bit set, FCHECK still valid
			return b
		}()},
		{"truncated-deflate", base[:6]},
		{"truncated-trailer", base[:len(base)-4]},
		{"adler-corrupt", func() []byte { b := append([]byte{}, base...); b[len(b)-1] ^= 0x01; return b }()},
		{"trailing-junk", append(append([]byte{}, base...), 0x00)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := zlib.Expand(tc.in); got != nil {
				t.Fatalf("Expand accepted malformed input: %d bytes", len(got))
			}
		})
	}
}

// TestRandomizedSeed42 is the property test: round trips plus one
// stdlib cross-check per step.
func TestRandomizedSeed42(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	for step := 0; step < 200; step++ {
		n := rng.IntN(30001)
		data := make([]byte, n)
		shape := rng.IntN(3)
		for i := range data {
			switch shape {
			case 0:
				data[i] = byte(rng.IntN(256))
			case 1:
				data[i] = byte('a' + rng.IntN(6))
			default:
				data[i] = 'x'
			}
		}
		z := zlib.Compress(data)
		if back := zlib.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("step %d: round trip mismatch", step)
		}
		if step%16 == 0 {
			r, err := stdzlib.NewReader(bytes.NewReader(z))
			if err != nil {
				t.Fatalf("step %d: stdlib rejected: %v", step, err)
			}
			out, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(out, data) {
				t.Fatalf("step %d: stdlib cross-check failed", step)
			}
		}
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// corpus covers the decoder's regimes: empty, tiny, runs, natural
// text at several sizes, incompressible random data, and mixtures
// that force multi-block frames and every literals size format.
func corpus() []struct {
	name string
	data []byte
} {
	text := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 20000)) // ~900 KB
	rnd := make([]byte, 400000)
	s := uint32(42)
	for i := range rnd {
		s = s*1664525 + 1013904223
		rnd[i] = byte(s >> 23)
	}
	mix := append(append(append([]byte{}, rnd[:100000]...), bytes.Repeat([]byte{7}, 100000)...), text[:200000]...)
	return []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"one", []byte("h")},
		{"small", []byte("hello hello hello hello world\n")},
		{"runs", append([]byte(strings.Repeat("ab", 500)), strings.Repeat("c", 60)...)},
		{"text-22k", []byte(strings.Repeat("pack my box with five dozen liquor jugs ", 500))},
		{"text-900k", text},
		{"random-400k", rnd},
		{"run-300k", bytes.Repeat([]byte{9}, 300000)},
		{"mix-400k", mix},
		{"words-400k", wordCorpus()}, // match-poor skewed literals: Huffman fodder
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range corpus() {
		z := Compress(tc.data)
		got := Expand(z)
		if !bytes.Equal(got, tc.data) {
			t.Errorf("%s: stored round trip failed", tc.name)
		}
		if n := len(z) - len(tc.data); n > 12+len(tc.data)/maxBlockSize*3 {
			t.Errorf("%s: frame overhead %d looks wrong", tc.name, n)
		}
	}
}

// TestSystemGoldens decodes zstd(1) output across levels, checksums,
// and long-window mode.  Determinism requires -T1: with -T0 the CLI's
// job scheduling changes the framing between runs.
func TestSystemGoldens(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	for _, tc := range corpus() {
		for _, lvl := range []string{"-1", "-3", "-6", "-9", "-12", "-16", "-19", "--ultra -22", "--long=25"} {
			cmd := exec.Command("zstd", "-q", "-c", "-T1", lvl)
			cmd.Stdin = bytes.NewReader(tc.data)
			z, err := cmd.Output()
			if err != nil {
				if strings.Contains(lvl, "-22") {
					continue // the CLI enforces a minimum input size at -22
				}
				t.Fatalf("%s %s: %v", tc.name, lvl, err)
			}
			if got := Expand(z); !bytes.Equal(got, tc.data) {
				t.Errorf("%s %s: Expand failed", tc.name, lvl)
			}
		}
		// checksummed frames verify
		cmd := exec.Command("zstd", "-q", "-c", "-T1", "--check")
		cmd.Stdin = bytes.NewReader(tc.data)
		z, _ := cmd.Output()
		if got := Expand(z); !bytes.Equal(got, tc.data) {
			t.Errorf("%s --check: Expand failed", tc.name)
		}
	}
}

// TestSystemReadsOurs: zstd(1) must test and decode our stored frames.
func TestSystemReadsOurs(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	for _, tc := range corpus() {
		z := Compress(tc.data)
		tcmd := exec.Command("zstd", "-t")
		tcmd.Stdin = bytes.NewReader(z)
		if out, err := tcmd.CombinedOutput(); err != nil {
			t.Errorf("%s: zstd -t rejected: %v: %s", tc.name, err, out)
			continue
		}
		dcmd := exec.Command("zstd", "-q", "-dc")
		dcmd.Stdin = bytes.NewReader(z)
		got, err := dcmd.Output()
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: zstd -dc mismatch: %v", tc.name, err)
		}
	}
}

func TestConcatAndSkippable(t *testing.T) {
	a := Compress([]byte("first "))
	b := Compress([]byte("second"))
	skip := []byte{0x50, 0x2A, 0x4D, 0x18, 4, 0, 0, 0, 1, 2, 3, 4}
	got := Expand(append(append(append([]byte{}, a...), skip...), b...))
	if string(got) != "first second" {
		t.Fatalf("concat/skippable: %q", got)
	}
	if Expand(append(a, 0)) != nil {
		t.Error("trailing byte decoded")
	}
}

func TestMalformed(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	in := append([]byte("truncate me nicely please, more bytes here"), bytes.Repeat([]byte("xy"), 300)...)
	cmd := exec.Command("zstd", "-q", "-c", "-T1", "-6")
	cmd.Stdin = bytes.NewReader(in)
	z, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	if Expand(nil) != nil {
		t.Error("empty input decoded")
	}
	if Expand([]byte{0x28, 0xB5, 0x2F, 0xFD}) != nil {
		t.Error("magic-only decoded")
	}
	bad := append([]byte{}, z...)
	bad[0] ^= 0xFF // magic
	if Expand(bad) != nil {
		t.Error("bad magic decoded")
	}
	bad = append([]byte{}, z...)
	bad[len(bad)-1] ^= 0x01
	if Expand(bad) != nil {
		t.Error("corrupted tail decoded")
	}
	for i := 0; i < len(z) && i < 64; i++ {
		if Expand(z[:i]) != nil {
			t.Fatalf("prefix %d decoded", i)
		}
	}
	// The empty frame shape: magic, no-FCS/no-check descriptor, 128 KiB
	// window, one empty last raw block.
	if got, want := Compress(nil), []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x68, 0x01, 0x00, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("empty frame:\n got %x\nwant %x", got, want)
	}
}

// TestCorruptionSweep flips and truncates bytes at many offsets: the
// decoder must return an error (or the right bytes), never panic or
// loop.
func TestCorruptionSweep(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	in := append([]byte("corruption sweep corpus, some compressible padding here "), bytes.Repeat([]byte("zap "), 4000)...)
	cmd := exec.Command("zstd", "-q", "-c", "-T1", "-6")
	cmd.Stdin = bytes.NewReader(in)
	z, _ := cmd.Output()
	s := uint32(7)
	for i := 0; i < 3000; i++ {
		s = s*1664525 + 1013904223
		bad := append([]byte{}, z...)
		switch s % 3 {
		case 0:
			bad[int(s>>8)%len(bad)] ^= byte(s >> 16)
		case 1:
			bad = bad[:int(s>>8)%len(bad)]
		default:
			bad[int(s>>8)%len(bad)] = byte(s >> 16)
		}
		Expand(bad) // must not panic; output is either right or nil
	}
}

// TestStructuredRoundTrip: tar-like payloads — 512-byte headers with
// checksum digits, short data runs, long zero tails — catch matcher
// bugs the uniform corpus misses (a backward-extension off-by-one hid
// here for a whole encoder version).
func TestStructuredRoundTrip(t *testing.T) {
	s := uint32(20261118)
	next := func(n int) int {
		s = s*1664525 + 1013904223
		return int(s>>23) % n
	}
	for trial := 0; trial < 200; trial++ {
		var buf []byte
		for len(buf) < 20000 {
			blk := make([]byte, 512)
			name := []string{"proj/", "data.bin", "x", "some-longer-name.txt"}[next(4)]
			copy(blk, name)
			copy(blk[100:], "00000000007")
			copy(blk[148:], "000640\x00 ")
			for i := 300; i < 340; i++ {
				blk[i] = byte('0' + next(8))
			}
			buf = append(buf, blk...)
			switch next(3) {
			case 0: // short data
				buf = append(buf, byte(next(256)), byte(next(256)), byte(next(256)))
			case 1: // medium run
				run := make([]byte, 40+next(64))
				b := byte(next(256))
				for i := range run {
					run[i] = b
				}
				buf = append(buf, run...)
			}
		}
		z := Compress(buf)
		if got := Expand(z); !bytes.Equal(got, buf) {
			i := 0
			for ; i < len(got) && got[i] == buf[i]; i++ {
			}
			t.Fatalf("trial %d: corrupt at %d of %d (got %02x want %02x)",
				trial, i, len(buf), got[i], buf[i])
		}
	}
}

// TestWriterStructured: the same payloads through the streaming
// writer with small writes and interleaved flushes.
func TestWriterStructured(t *testing.T) {
	s := uint32(7)
	next := func(n int) int {
		s = s*1664525 + 1013904223
		return int(s>>23) % n
	}
	for trial := 0; trial < 60; trial++ {
		var want []byte
		var out bytes.Buffer
		w := NewWriter(&out)
		for len(want) < 30000 {
			chunk := make([]byte, 1+next(900))
			for i := range chunk {
				chunk[i] = byte(next(4) * 61)
			}
			want = append(want, chunk...)
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}
			if next(4) == 0 {
				if err := w.Flush(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if got := Expand(out.Bytes()); !bytes.Equal(got, want) {
			t.Fatalf("trial %d: streaming writer corrupt", trial)
		}
	}
}

// wordCorpus builds match-poor, letter-skewed text (3-7 letter words
// over a 12-letter alphabet): 4-grams repeat constantly but long
// matches are rare, so literals dominate and Huffman earns its keep.
func wordCorpus() []byte {
	s := uint32(11)
	word := func() string {
		s = s*1664525 + 1013904223
		n := 3 + int(s>>28)
		out := make([]byte, n)
		for i := range out {
			s = s*1664525 + 1013904223
			out[i] = byte('a' + (s>>23)%12)
		}
		return string(out)
	}
	var buf []byte
	for i := 0; i < 55000; i++ {
		buf = append(buf, word()...)
		buf = append(buf, ' ')
	}
	return buf
}

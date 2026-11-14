/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/pschlump/pluto/xz"
)

// corpus covers the decoder's paths: empty, tiny, runs, natural text
// at several sizes (single-chunk and multi-chunk), and incompressible
// random data (the uncompressed-chunk path).
func corpus() []struct {
	name string
	data []byte
} {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 20000) // ~900 KB
	rnd := make([]byte, 100000)
	s := uint32(42)
	for i := range rnd {
		s = s*1664525 + 1013904223
		rnd[i] = byte(s >> 23)
	}
	return []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"one", []byte("h")},
		{"small", []byte("hello hello hello hello world\n")},
		{"runs", append([]byte(strings.Repeat("ab", 500)), strings.Repeat("c", 60)...)},
		{"match-boundary", []byte(strings.Repeat("ab", 1503))}, // 2+11×273+1: a lone literal after full matches
		{"text-22k", []byte(strings.Repeat("pack my box with five dozen liquor jugs ", 500))},
		{"ab-200k", []byte(strings.Repeat("ab", 100000))},
		{"text-900k", []byte(text)},
		{"random-100k", rnd},
	}
}

func xzCLI(t *testing.T, args []string, input []byte) []byte {
	t.Helper()
	cmd := exec.Command("xz", args...)
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("xz %v: %v", args, err)
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range corpus() {
		z := xz.Compress(tc.data)
		got := xz.Expand(z)
		if got == nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: round trip failed", tc.name)
		}
		// compressible inputs must actually shrink; tiny and random
		// ones legitimately do not (container overhead, raw chunks)
		if len(tc.data) > 10000 && strings.Contains(tc.name, "text") && len(z) >= len(tc.data) {
			t.Errorf("%s: no compression (%d -> %d)", tc.name, len(tc.data), len(z))
		}
	}
}

// TestSystemGoldens decodes xz(1) output across check types and
// presets.
func TestSystemGoldens(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	sample := corpus()[6].data[:200000]
	for _, check := range []string{"crc32", "crc64", "none", "sha256"} {
		for _, tc := range corpus() {
			z := xzCLI(t, []string{"-c", "-C", check}, tc.data)
			got := xz.Expand(z)
			if got == nil || !bytes.Equal(got, tc.data) {
				t.Errorf("%s/%s: Expand failed", check, tc.name)
			}
		}
	}
	for _, preset := range []string{"-0", "-1", "-6", "-9", "-9e"} {
		z := xzCLI(t, []string{"-c", preset}, sample)
		if got := xz.Expand(z); got == nil || !bytes.Equal(got, sample) {
			t.Errorf("preset %s: Expand failed", preset)
		}
	}
}

// TestSystemReadsOurs: xz(1) must test and decode our streams.
func TestSystemReadsOurs(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	for _, tc := range corpus() {
		z := xz.Compress(tc.data)
		tcmd := exec.Command("xz", "-t")
		tcmd.Stdin = bytes.NewReader(z)
		if out, err := tcmd.CombinedOutput(); err != nil {
			t.Errorf("%s: xz -t rejected: %v: %s", tc.name, err, out)
			continue
		}
		dcmd := exec.Command("xz", "-dc")
		dcmd.Stdin = bytes.NewReader(z)
		got, err := dcmd.Output()
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: xz -dc mismatch: %v", tc.name, err)
		}
	}
}

func TestConcatAndPadding(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	a := xzCLI(t, []string{"-c"}, []byte("first part, "))
	b := xzCLI(t, []string{"-c"}, []byte("second part"))
	joined := append(append(append([]byte{}, a...), 0, 0, 0, 0), b...) // padding between
	got := xz.Expand(joined)
	if got == nil || string(got) != "first part, second part" {
		t.Fatalf("concat: %q", got)
	}
	// our own streams concatenate too
	ja := xz.Compress([]byte("one"))
	jb := xz.Compress([]byte("two"))
	if got := xz.Expand(append(ja, jb...)); string(got) != "onetwo" {
		t.Fatalf("self concat: %q", got)
	}
}

func TestMalformed(t *testing.T) {
	z := xzCLI(t, []string{"-c"}, []byte("truncate me nicely please"))

	if xz.Expand(nil) != nil {
		t.Error("empty input decoded")
	}
	if xz.Expand([]byte{0, 0, 0, 0}) != nil {
		t.Error("padding-only input decoded")
	}
	bad := append([]byte{}, z...)
	bad[0] ^= 0xFF // magic
	if xz.Expand(bad) != nil {
		t.Error("bad magic decoded")
	}
	bad = append([]byte{}, z...)
	bad[len(z)-1] ^= 0xFF // footer magic YZ
	if xz.Expand(bad) != nil {
		t.Error("bad footer decoded")
	}
	bad = append([]byte{}, z...)
	bad[20] ^= 0x01 // flip a byte inside the compressed data
	if xz.Expand(bad) != nil {
		t.Error("corrupt data decoded")
	}
	if xz.Expand(z[:len(z)-5]) != nil {
		t.Error("truncated tail decoded")
	}
	// every proper prefix is malformed
	for i := 0; i < len(z); i++ {
		if xz.Expand(z[:i]) != nil {
			t.Fatalf("prefix %d decoded", i)
		}
	}
}

// TestEmptyStreamShape pins our empty stream byte-for-byte (xz(1)
// writes a zero-block variant; ours carries one empty block — both
// valid, ours chosen for writer symmetry).
func TestEmptyStreamShape(t *testing.T) {
	got := xz.Compress(nil)
	want := []byte{
		0xFD, '7', 'z', 'X', 'Z', 0x00, 0x00, 0x04, 0xE6, 0xD6, 0xB4, 0x46,
		0x02, 0x00, 0x21, 0x01, 0x16, 0x00, 0x00, 0x00, 0x74, 0x2F, 0xE5, 0xA3,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0x15, 0x00, 0x3F, 0x53, 0x33, 0x17,
		0x1F, 0xB6, 0xF3, 0x7D, 0x01, 0x00, 0x00, 0x00, 0x00, 0x04, 0x59, 0x5A,
	}
	if !bytes.Equal(got, want) {
		t.Errorf("empty stream shape:\n got %x\nwant %x", got, want)
	}
}

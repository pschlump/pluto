/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lzma_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/pschlump/pluto/lzma"
)

func corpus() []struct {
	name string
	data []byte
} {
	return []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"one", []byte("h")},
		{"small", []byte("hello hello hello hello world\n")},
		{"runs", []byte(strings.Repeat("ab", 3000))},
		{"text", []byte(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 800))},
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range corpus() {
		z := lzma.Compress(tc.data)
		got := lzma.Expand(z)
		if got == nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: round trip failed", tc.name)
		}
	}
}

func TestSystemGoldens(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	for _, tc := range corpus() {
		cmd := exec.Command("xz", "-c", "--format=lzma")
		cmd.Stdin = bytes.NewReader(tc.data)
		z, err := cmd.Output()
		if err != nil {
			t.Fatalf("xz: %v", err)
		}
		got := lzma.Expand(z)
		if got == nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: golden decode failed", tc.name)
		}
	}
}

func TestSystemReadsOurs(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	for _, tc := range corpus() {
		z := lzma.Compress(tc.data)
		cmd := exec.Command("xz", "-dc")
		cmd.Stdin = bytes.NewReader(z)
		got, err := cmd.Output()
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: xz -dc rejected ours: %v", tc.name, err)
		}
	}
	// the lzma tool itself if present
	if _, err := exec.LookPath("lzma"); err == nil {
		z := lzma.Compress(corpus()[3].data)
		cmd := exec.Command("lzma", "-dc")
		cmd.Stdin = bytes.NewReader(z)
		if out, err := cmd.Output(); err != nil || !bytes.Equal(out, corpus()[3].data) {
			t.Errorf("lzma -dc rejected ours: %v", err)
		}
	}
}

// TestKnownSizeGolden: a header with the exact size (no end marker) —
// produced by Python's FORMAT_ALONE writers — must decode.
func TestKnownSizeGolden(t *testing.T) {
	out, err := exec.Command("python3", "-I", "-c",
		`import lzma, sys
data = b"known size hello " * 200
sys.stdout.buffer.write(lzma.compress(data, format=lzma.FORMAT_ALONE))`).Output()
	if err != nil {
		t.Skip("python3 unavailable")
	}
	got := lzma.Expand(out)
	if got == nil || !bytes.Contains(got, []byte("known size hello")) {
		t.Fatalf("known-size golden failed (%d bytes)", len(out))
	}
}

func TestMalformed(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	cmd := exec.Command("xz", "-c", "--format=lzma")
	cmd.Stdin = strings.NewReader("truncate me")
	z, _ := cmd.Output()
	if lzma.Expand(nil) != nil || lzma.Expand(z[:5]) != nil {
		t.Error("short input decoded")
	}
	bad := append([]byte{}, z...)
	bad[0] = 0xF0 // invalid properties
	if lzma.Expand(bad) != nil {
		t.Error("bad props decoded")
	}
	for i := 13; i < len(z); i++ {
		if lzma.Expand(z[:i]) != nil {
			t.Fatalf("prefix %d decoded", i)
		}
	}
}

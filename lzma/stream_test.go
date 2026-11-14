/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lzma_test

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	"github.com/pschlump/pluto/lzma"
)

func TestStreamRoundTrip(t *testing.T) {
	for _, tc := range corpus() {
		var buf bytes.Buffer
		w := lzma.NewWriter(&buf)
		if _, err := w.Write(tc.data); err != nil {
			t.Fatalf("%s: write: %v", tc.name, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("%s: close: %v", tc.name, err)
		}
		if got := lzma.Expand(buf.Bytes()); got == nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: whole-buffer read of stream output failed", tc.name)
		}
		r := lzma.NewReader(bytes.NewReader(buf.Bytes()))
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: stream read of ours: %v", tc.name, err)
		}
		if _, err := exec.LookPath("xz"); err == nil {
			cmd := exec.Command("xz", "-dc")
			cmd.Stdin = bytes.NewReader(buf.Bytes())
			if out, err := cmd.Output(); err != nil || !bytes.Equal(out, tc.data) {
				t.Errorf("%s: xz -dc of ours: %v", tc.name, err)
			}
		}
	}
}

func TestStreamReadGoldens(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	for _, tc := range corpus() {
		cmd := exec.Command("xz", "-c", "--format=lzma")
		cmd.Stdin = bytes.NewReader(tc.data)
		z, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		r := lzma.NewReader(bytes.NewReader(z))
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: stream read of golden: %v", tc.name, err)
		}
	}
	out, err := exec.Command("python3", "-I", "-c",
		`import lzma, sys
data = b"sized " * 50000
sys.stdout.buffer.write(lzma.compress(data, format=lzma.FORMAT_ALONE))`).Output()
	if err == nil {
		r := lzma.NewReader(bytes.NewReader(out))
		got, err := io.ReadAll(r)
		if err != nil || len(got) != 6*50000 {
			t.Errorf("known-size stream read: %v (%d bytes)", err, len(got))
		}
	}
}

// TestStreamIncrementalDelivery: the Reader delivers bytes before the
// stream ends (no whole-stream buffering).
func TestStreamIncrementalDelivery(t *testing.T) {
	data := bytes.Repeat([]byte("incremental delivery matters "), 20000)
	cmd := exec.Command("xz", "-c", "--format=lzma")
	cmd.Stdin = bytes.NewReader(data)
	z, err := cmd.Output()
	if err != nil {
		t.Skip("xz unavailable")
	}
	r := lzma.NewReader(bytes.NewReader(z))
	buf := make([]byte, 100)
	if n, err := io.ReadFull(r, buf); err != nil || n != 100 {
		t.Fatalf("first read: %d bytes, %v", n, err)
	}
	if !bytes.Equal(buf, data[:100]) {
		t.Fatal("first 100 bytes differ")
	}
	rest, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(append(buf, rest...), data) {
		t.Fatal("remainder mismatch")
	}
}

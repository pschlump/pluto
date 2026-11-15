/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// TestReaderGoldens streams zstd(1) output through the Reader one
// byte at a time — the harshest delivery pattern — and matches
// whole-buffer Expand.
func TestReaderGoldens(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	for _, tc := range corpus() {
		for _, lvl := range []string{"-3", "-9", "-19"} {
			cmd := exec.Command("zstd", "-q", "-c", "-T1", lvl)
			cmd.Stdin = bytes.NewReader(tc.data)
			z, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s %s: %v", tc.name, lvl, err)
			}
			r := NewReader(bytes.NewReader(z))
			var got []byte
			one := make([]byte, 1)
			for {
				n, err := r.Read(one)
				got = append(got, one[:n]...)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("%s %s: read: %v", tc.name, lvl, err)
				}
			}
			if !bytes.Equal(got, tc.data) {
				t.Errorf("%s %s: streamed mismatch (%d bytes)", tc.name, lvl, len(got))
			}
		}
	}
}

// TestReaderMultiFrame streams several frames (one checksummed) plus
// a skippable frame.
func TestReaderMultiFrame(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	cmd := exec.Command("zstd", "-q", "-c", "-T1", "--check")
	cmd.Stdin = strings.NewReader("checked frame")
	a, _ := cmd.Output()
	b := Compress([]byte("stored frame"))
	skip := []byte{0x5F, 0x2A, 0x4D, 0x18, 2, 0, 0, 0, 0xAA, 0xBB}
	all := append(append(append([]byte{}, a...), skip...), b...)

	r := NewReader(bytes.NewReader(all))
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "checked framestored frame" {
		t.Fatalf("multi-frame: %q", got)
	}
	// after EOF the error is sticky
	if _, err := r.Read(nil); err != io.EOF {
		t.Fatalf("sticky EOF: %v", err)
	}
}

// TestReaderMalformed: truncation anywhere is an error, never a
// silent short read.
func TestReaderMalformed(t *testing.T) {
	z := Compress(bytes.Repeat([]byte("streaming truncation check "), 9000))
	for _, cut := range []int{1, 3, 7, 100, len(z) / 2, len(z) - 1} {
		r := NewReader(bytes.NewReader(z[:cut]))
		_, err := io.ReadAll(r)
		if err == nil {
			t.Errorf("prefix %d decoded without error", cut)
		}
	}
}

func TestWriterRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	pattern := []byte(strings.Repeat("writer block boundary check ", 9000)) // spans blocks
	if _, err := w.Write(pattern); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("tail")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
	if got := Expand(buf.Bytes()); !bytes.Equal(got, append(pattern, []byte("tail")...)) {
		t.Fatal("writer round trip failed")
	}
	// empty writer: header + one empty block
	var ebuf bytes.Buffer
	ew := NewWriter(&ebuf)
	if err := ew.Close(); err != nil {
		t.Fatal(err)
	}
	if got := Expand(ebuf.Bytes()); len(got) != 0 {
		t.Fatal("empty writer produced output")
	}
	if _, err := ew.Write([]byte("x")); err == nil {
		t.Fatal("write after close accepted")
	}
}

// TestWriterSystemInterop: zstd(1) decodes our streamed frames.
func TestWriterSystemInterop(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	data := bytes.Repeat([]byte("streamed interop "), 40000)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	dcmd := exec.Command("zstd", "-q", "-dc")
	dcmd.Stdin = bytes.NewReader(buf.Bytes())
	got, err := dcmd.Output()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("zstd -dc our stream: %v", err)
	}
}

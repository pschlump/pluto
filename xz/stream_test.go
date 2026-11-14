/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz_test

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/pschlump/pluto/xz"
)

// TestStreamRoundTrip: the Writer's output must satisfy Expand, our
// own streaming Reader, and xz(1).
func TestStreamRoundTrip(t *testing.T) {
	for _, tc := range corpus() {
		var buf bytes.Buffer
		w := xz.NewWriter(&buf)
		if _, err := w.Write(tc.data); err != nil {
			t.Fatalf("%s: write: %v", tc.name, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("%s: close: %v", tc.name, err)
		}
		if got := xz.Expand(buf.Bytes()); got == nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: Expand of stream-writer output failed", tc.name)
		}
		r := xz.NewReader(bytes.NewReader(buf.Bytes()))
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, tc.data) {
			t.Errorf("%s: stream read of own output: %v", tc.name, err)
		}
		if _, err := exec.LookPath("xz"); err == nil {
			cmd := exec.Command("xz", "-t")
			cmd.Stdin = bytes.NewReader(buf.Bytes())
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s: xz -t: %v: %s", tc.name, err, out)
			}
		}
	}
}

// TestStreamReadGoldens: the Reader decodes xz(1) streams.
func TestStreamReadGoldens(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	for _, check := range []string{"crc32", "crc64", "none", "sha256"} {
		for _, tc := range corpus() {
			z := xzCLI(t, []string{"-c", "-C", check}, tc.data)
			r := xz.NewReader(bytes.NewReader(z))
			got, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(got, tc.data) {
				t.Errorf("%s/%s: stream read: %v", check, tc.name, err)
			}
		}
	}
}

// TestStreamFlushIncremental: bytes are decodable after each Flush
// (chunks end early, everything before them is complete).
func TestStreamFlushIncremental(t *testing.T) {
	var buf bytes.Buffer
	w := xz.NewWriter(&buf)
	var expect []byte
	for i := 0; i < 5; i++ {
		chunk := []byte(strings.Repeat("flush me early ", 100) + strings.Repeat(string(rune('a'+i)), 70000))
		expect = append(expect, chunk...)
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := xz.Expand(buf.Bytes()); got == nil || !bytes.Equal(got, expect) {
		t.Fatal("flushed stream does not round trip")
	}
}

// TestStreamTruncation: every prefix of a stream is truncated
// (io.ErrUnexpectedEOF) or malformed — never a silent short read.
func TestStreamTruncation(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz not installed")
	}
	z := xzCLI(t, []string{"-c"}, []byte(strings.Repeat("the quick brown fox ", 300)))
	for i := 0; i < len(z); i++ {
		r := xz.NewReader(bytes.NewReader(z[:i]))
		out, err := io.ReadAll(r)
		if err == nil {
			t.Fatalf("prefix %d: decoded %d bytes without error", i, len(out))
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, xz.ErrMalformed) {
			t.Fatalf("prefix %d: unexpected error %v", i, err)
		}
	}
}

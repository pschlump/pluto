/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2_test

import (
	"bytes"
	stdbzip2 "compress/bzip2"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/bzip2"
)

// bzStreamWrite compresses data through the streaming Writer in
// chunks of the given size.
func bzStreamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := bzip2.NewWriter(&buf)
	for len(data) > 0 {
		n := min(chunk, len(data))
		if _, err := w.Write(data[:n]); err != nil {
			t.Fatalf("stream write: %v", err)
		}
		data = data[n:]
	}
	if err := w.Close(); err != nil {
		t.Fatalf("stream close: %v", err)
	}
	return buf.Bytes()
}

// bzStreamRead decompresses through the streaming Reader in chunks of
// the given size.
func bzStreamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := bzip2.NewReader(bytes.NewReader(data))
	defer r.Close()
	var out bytes.Buffer
	p := make([]byte, chunk)
	for {
		n, err := r.Read(p)
		out.Write(p[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream read: %v", err)
		}
	}
	return out.Bytes()
}

// TestBz2StreamRoundTrip runs the corpus through Writer and Reader:
// every entry at a mid-size geometry, the small ones also one byte at
// a time (the worst case for the writer's block buffering).
func TestBz2StreamRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		bz := bzStreamWrite(t, data, 4096)
		if got := bzStreamRead(t, bz, 4096); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: round trip mismatch (%d in, %d out)", i, len(data), len(got))
		}
	}
	small := corpus()[:4] // nil, "a", TOBEORNOT, 300k 'a's
	for i, data := range small {
		for _, chunk := range []int{1, 17} {
			bz := bzStreamWrite(t, data, chunk)
			if got := bzStreamRead(t, bz, chunk); !bytes.Equal(got, data) {
				t.Errorf("corpus %d chunk %d: round trip mismatch", i, chunk)
			}
		}
	}
}

// TestBz2StreamLevelsRoundTrip exercises NewWriterLevel 1..9 and its
// panic on out-of-range levels.
func TestBz2StreamLevelsRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("level me up, "), 30000) // 417k, multi-block at low levels
	for level := 1; level <= 9; level++ {
		var buf bytes.Buffer
		w := bzip2.NewWriterLevel(&buf, level)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if got := bzStreamRead(t, buf.Bytes(), 70000); !bytes.Equal(got, data) {
			t.Errorf("level %d: round trip mismatch", level)
		}
	}
	for _, bad := range []int{0, 10} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("level %d: NewWriterLevel did not panic", bad)
				}
			}()
			bzip2.NewWriterLevel(io.Discard, bad)
		}()
	}
}

// TestBz2StreamEmptyGolden: an unused Writer closed immediately must
// produce exactly Compress(nil) (and the CLI empty golden).
func TestBz2StreamEmptyGolden(t *testing.T) {
	var buf bytes.Buffer
	w := bzip2.NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), bzip2.Compress(nil)) {
		t.Errorf("empty stream = % x, want Compress(nil)", buf.Bytes())
	}
	if !bytes.Equal(buf.Bytes(), hexBytes(t, goldenEmptyHex)) {
		t.Errorf("empty stream = % x, want CLI golden", buf.Bytes())
	}
	if got := bzStreamRead(t, buf.Bytes(), 4); len(got) != 0 {
		t.Errorf("empty decode = %q", got)
	}
}

// TestBz2StreamCLIGoldens: the streaming Reader decodes bzip2(1)
// output (the empty golden is covered above; TOBEORNOT here).
func TestBz2StreamCLIGoldens(t *testing.T) {
	got := bzStreamRead(t, hexBytes(t, goldenTobeHex), 3)
	if !bytes.Equal(got, []byte("TOBEORNOTTOBEORTOBEORNOT")) {
		t.Errorf("CLI golden decode = %q", got)
	}
}

// TestBz2StreamStdlibInterop: the stdlib reader decodes the streaming
// Writer's output at every level.
func TestBz2StreamStdlibInterop(t *testing.T) {
	for level := 1; level <= 9; level += 4 {
		data := bytes.Repeat([]byte("stdlib interop, "), 30000)
		var buf bytes.Buffer
		w := bzip2.NewWriterLevel(&buf, level)
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		sr := stdbzip2.NewReader(bytes.NewReader(buf.Bytes()))
		back, err := io.ReadAll(sr)
		if err != nil {
			t.Fatalf("level %d: stdlib read: %v", level, err)
		}
		if !bytes.Equal(back, data) {
			t.Errorf("level %d: stdlib decoded streaming output wrong", level)
		}
	}
}

// TestBz2StreamMultiMember: concatenated members concatenate content,
// as Expand and bzip2(1) -d do.
func TestBz2StreamMultiMember(t *testing.T) {
	a := bzip2.Compress([]byte("first member "))
	b := bzip2.Compress(bytes.Repeat([]byte("second member, rather longer. "), 5000))
	both := append(append([]byte{}, a...), b...)
	want := append([]byte("first member "), bytes.Repeat([]byte("second member, rather longer. "), 5000)...)
	if got := bzStreamRead(t, both, 1000); !bytes.Equal(got, want) {
		t.Error("multi-member mismatch")
	}
}

// TestBz2StreamOneByteReader feeds the Reader through
// iotest.OneByteReader so every refill boundary is exercised a byte at
// a time.
func TestBz2StreamOneByteReader(t *testing.T) {
	for i, data := range corpus()[:6] {
		bz := bzip2.Compress(data)
		r := bzip2.NewReader(iotest.OneByteReader(bytes.NewReader(bz)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("corpus %d: one-byte-reader mismatch", i)
		}
	}
}

// TestBz2StreamFlush delivers early: after Flush the prefix is
// decodable through a pipe before anything more is written.
func TestBz2StreamFlush(t *testing.T) {
	data := []byte(strings.Repeat("flush me early, ", 2000))
	half := len(data) / 2
	pr, pw := io.Pipe()
	resume := make(chan struct{})
	go func() {
		w := bzip2.NewWriter(pw)
		if _, err := w.Write(data[:half]); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := w.Flush(); err != nil {
			pw.CloseWithError(err)
			return
		}
		<-resume
		if _, err := w.Write(data[half:]); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := w.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	r := bzip2.NewReader(pr)
	got := make([]byte, half)
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("decode after Flush: %v", err)
	}
	if !bytes.Equal(got, data[:half]) {
		t.Fatal("flushed prefix mismatch")
	}
	close(resume)
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rest, data[half:]) {
		t.Fatal("post-flush remainder mismatch")
	}
}

// TestBz2StreamTruncated: sampled proper prefixes must error, never
// return short silently.
func TestBz2StreamTruncated(t *testing.T) {
	full := bzip2.Compress(bytes.Repeat([]byte("truncate me "), 8000))
	for n := 1; n < len(full); n += 97 {
		r := bzip2.NewReader(bytes.NewReader(full[:n]))
		_, err := io.ReadAll(r)
		if err == nil {
			t.Errorf("prefix %d decoded without error", n)
		} else if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("prefix %d: err = %v, want ErrUnexpectedEOF", n, err)
		}
	}
	if _, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(nil))); err != nil {
		t.Errorf("empty source: err = %v, want nil", err)
	}
}

// TestBz2StreamErrTypes: a corrupted stream CRC and trailing garbage
// wrap ErrMalformed.
func TestBz2StreamErrTypes(t *testing.T) {
	full := bzip2.Compress(bytes.Repeat([]byte("check me "), 5000))

	// The combined CRC is the last 4 bytes before the final pad bit;
	// flip a bit inside it.
	bad := append([]byte{}, full...)
	bad[len(bad)-4] ^= 0x01
	r := bzip2.NewReader(bytes.NewReader(bad))
	_, err := io.ReadAll(r)
	if !errors.Is(err, bzip2.ErrMalformed) {
		t.Errorf("bad stream CRC: err = %v, want ErrMalformed", err)
	}

	// Trailing garbage after a complete member.
	tail := append(append([]byte{}, full...), "junk"...)
	r2 := bzip2.NewReader(bytes.NewReader(tail))
	_, err = io.ReadAll(r2)
	if !errors.Is(err, bzip2.ErrMalformed) {
		t.Errorf("trailing garbage: err = %v, want ErrMalformed", err)
	}
}

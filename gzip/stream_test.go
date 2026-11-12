/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package gzip_test

import (
	"bytes"
	stdgzip "compress/gzip"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/gzip"
)

// gzStreamWrite compresses data through the streaming Writer in
// chunks of the given size.
func gzStreamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
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

// gzStreamRead decompresses through the streaming Reader in chunks of
// the given size.
func gzStreamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := gzip.NewReader(bytes.NewReader(data))
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

// TestGzStreamRoundTrip runs the corpus through Writer and Reader at
// several chunk geometries.
func TestGzStreamRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		for _, chunk := range []int{1, 7, 1024, 33333, 1 << 20} {
			gz := gzStreamWrite(t, data, chunk)
			if got := gzStreamRead(t, gz, chunk); !bytes.Equal(got, data) {
				t.Errorf("corpus %d chunk %d: round trip mismatch (%d in, %d out)",
					i, chunk, len(data), len(got))
			}
		}
	}
}

// TestGzStreamEmptyGolden: an unused Writer closed immediately must
// produce exactly the whole-buffer Compress(nil) bytes.
func TestGzStreamEmptyGolden(t *testing.T) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), gzip.Compress(nil)) {
		t.Errorf("empty stream = % x, want Compress(nil)", buf.Bytes())
	}
	if got := gzStreamRead(t, buf.Bytes(), 4); len(got) != 0 {
		t.Errorf("empty decode = %q", got)
	}
}

// TestGzStreamOneByteReader feeds the Reader through
// iotest.OneByteReader so every refill boundary is exercised a byte at
// a time.
func TestGzStreamOneByteReader(t *testing.T) {
	for i, data := range corpus() {
		gz := gzStreamWrite(t, data, 4096)
		r := gzip.NewReader(iotest.OneByteReader(bytes.NewReader(gz)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("corpus %d: one-byte-reader mismatch", i)
		}
	}
}

// TestGzStreamStdlibInterop: stdlib gzip reads the streaming Writer's
// member, and the streaming Reader reads a stdlib member carrying
// Name/Comment/Extra.
func TestGzStreamStdlibInterop(t *testing.T) {
	for i, data := range corpus() {
		gz := gzStreamWrite(t, data, 5000)
		sr, err := stdgzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			t.Fatalf("corpus %d: stdlib NewReader: %v", i, err)
		}
		back, err := io.ReadAll(sr)
		if err != nil {
			t.Fatalf("corpus %d: stdlib read: %v", i, err)
		}
		if !bytes.Equal(back, data) {
			t.Errorf("corpus %d: stdlib decoded streaming output wrong", i)
		}

		var buf bytes.Buffer
		w := stdgzip.NewWriter(&buf)
		w.Name = "corpus.bin"
		w.Comment = "pluto test"
		w.Extra = []byte{1, 2, 3, 4}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if got := gzStreamRead(t, buf.Bytes(), 4096); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: streaming Reader mismatched stdlib member", i)
		}
	}
}

// TestGzStreamMultiMember: concatenated members concatenate content
// through the streaming Reader, matching Expand.
func TestGzStreamMultiMember(t *testing.T) {
	a := gzip.Compress([]byte("first member "))
	b := gzip.Compress(bytes.Repeat([]byte("second member, rather longer. "), 5000))
	both := append(append([]byte{}, a...), b...)
	if got := gzStreamRead(t, both, 1000); !bytes.Equal(got,
		append([]byte("first member "), bytes.Repeat([]byte("second member, rather longer. "), 5000)...)) {
		t.Error("multi-member mismatch")
	}
}

// TestGzStreamFlush delivers early: after Flush the prefix is
// decodable through a pipe before anything more is written.
func TestGzStreamFlush(t *testing.T) {
	data := []byte(strings.Repeat("flush me early, ", 4000))
	half := len(data) / 2
	pr, pw := io.Pipe()
	resume := make(chan struct{})
	go func() {
		w := gzip.NewWriter(pw)
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

	r := gzip.NewReader(pr)
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

// TestGzStreamTruncated: sampled proper prefixes of a streaming
// member must error (the empty prefix is the one valid "no members"
// file, like Expand of nothing), never return short silently.
func TestGzStreamTruncated(t *testing.T) {
	full := gzStreamWrite(t, bytes.Repeat([]byte("truncate me "), 5000), 4096)
	for n := 1; n < len(full); n += 53 {
		r := gzip.NewReader(bytes.NewReader(full[:n]))
		_, err := io.ReadAll(r)
		if err == nil {
			t.Errorf("prefix %d decoded without error", n)
		}
	}
	// An empty source is a valid empty stream.
	if _, err := io.ReadAll(gzip.NewReader(bytes.NewReader(nil))); err != nil {
		t.Errorf("empty source: err = %v, want nil", err)
	}
}

// TestGzStreamErrTypes: a wrong CRC-32 wraps ErrMalformed, truncation
// reports io.ErrUnexpectedEOF, and trailing bytes that start but do
// not complete another member are a shortage, as in stdlib gzip.
func TestGzStreamErrTypes(t *testing.T) {
	full := gzStreamWrite(t, bytes.Repeat([]byte("check me "), 5000), 4096)

	bad := append([]byte{}, full...)
	bad[len(bad)-5] ^= 0xFF // inside the CRC-32 trailer word
	r := gzip.NewReader(bytes.NewReader(bad))
	_, err := io.ReadAll(r)
	if !errors.Is(err, gzip.ErrMalformed) {
		t.Errorf("bad CRC: err = %v, want ErrMalformed", err)
	}

	r2 := gzip.NewReader(bytes.NewReader(full[:len(full)/2]))
	_, err = io.ReadAll(r2)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated: err = %v, want ErrUnexpectedEOF", err)
	}

	tail := append(append([]byte{}, full...), 'x')
	r3 := gzip.NewReader(bytes.NewReader(tail))
	_, err = io.ReadAll(r3)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("trailing byte: err = %v, want ErrUnexpectedEOF", err)
	}

	// A complete trailing header with bad magic is corruption.
	badMagic := append(append([]byte{}, full...), 0x1f, 0x8c, 8, 0, 0, 0, 0, 0, 0, 3)
	r4 := gzip.NewReader(bytes.NewReader(badMagic))
	_, err = io.ReadAll(r4)
	if !errors.Is(err, gzip.ErrMalformed) {
		t.Errorf("bad trailing magic: err = %v, want ErrMalformed", err)
	}
}

// TestGzStreamCorruptNoPanic flips bits; the reader must error, never
// panic.
func TestGzStreamCorruptNoPanic(t *testing.T) {
	full := gzStreamWrite(t, bytes.Repeat([]byte("corruption target — "), 3000), 1000)
	for i := 0; i < len(full); i += 17 {
		mut := append([]byte{}, full...)
		mut[i] ^= 0x40
		r := gzip.NewReader(bytes.NewReader(mut))
		_, _ = io.ReadAll(r) // must not panic
		r.Close()
	}
}

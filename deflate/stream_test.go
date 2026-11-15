/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate_test

import (
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/deflate"
)

// streamWrite compresses data through the streaming Writer, feeding it
// in chunks of the given size (1 mimics the worst case for the window
// logic).
func streamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := deflate.NewWriter(&buf)
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

// streamRead decompresses data through the streaming Reader, reading
// in chunks of the given size.
func streamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := deflate.NewReader(bytes.NewReader(data))
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

// TestStreamRoundTrip runs the shared corpus through Writer and Reader
// at several chunk geometries: the chunk boundaries land on different
// window slide points each time.
func TestStreamRoundTrip(t *testing.T) {
	for _, tc := range buildCorpus() {
		for _, chunk := range []int{1, 7, 1024, 33333, 1 << 20} {
			compressed := streamWrite(t, tc.data, chunk)
			if got := streamRead(t, compressed, chunk); !bytes.Equal(got, tc.data) {
				t.Errorf("%s chunk %d: round trip mismatch (%d bytes in, %d out)",
					tc.name, chunk, len(tc.data), len(got))
			}
		}
	}
}

// TestStreamOneByteReader feeds the Reader through iotest.OneByteReader
// so every refill boundary is exercised one byte at a time.
func TestStreamOneByteReader(t *testing.T) {
	for _, tc := range buildCorpus() {
		compressed := streamWrite(t, tc.data, 4096)
		r := deflate.NewReader(iotest.OneByteReader(bytes.NewReader(compressed)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("%s: read: %v", tc.name, err)
		}
		if !bytes.Equal(got, tc.data) {
			t.Errorf("%s: one-byte-reader mismatch", tc.name)
		}
	}
}

// TestStreamCrossWholeBuffer checks the streaming halves against the
// stateless pair in both directions: the whole-buffer Expand decodes
// streaming output, and the streaming Reader decodes whole-buffer
// output.
func TestStreamCrossWholeBuffer(t *testing.T) {
	for _, tc := range buildCorpus() {
		streamed := streamWrite(t, tc.data, 8192)
		if back, n := deflate.Expand(streamed); n == 0 || !bytes.Equal(back, tc.data) {
			t.Errorf("%s: whole-buffer Expand cannot read streaming output", tc.name)
		}
		whole := deflate.Compress(tc.data)
		if got := streamRead(t, whole, 1000); !bytes.Equal(got, tc.data) {
			t.Errorf("%s: streaming Reader cannot read whole-buffer output", tc.name)
		}
	}
}

// TestStreamStdlibInterop is the conformance oracle for both streaming
// halves: stdlib flate decodes what the streaming Writer produces, and
// the streaming Reader decodes what stdlib produces at every level.
func TestStreamStdlibInterop(t *testing.T) {
	for _, tc := range buildCorpus() {
		got := streamRead(t, streamWrite(t, tc.data, 5000), 5000)
		if !bytes.Equal(got, tc.data) {
			t.Fatalf("%s: self check", tc.name)
		}

		var buf bytes.Buffer
		fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(tc.data); err != nil {
			t.Fatal(err)
		}
		if err := fw.Close(); err != nil {
			t.Fatal(err)
		}
		if got := streamRead(t, buf.Bytes(), 4096); !bytes.Equal(got, tc.data) {
			t.Errorf("%s: streaming Reader cannot decode stdlib output", tc.name)
		}

		streamed := streamWrite(t, tc.data, 4096)
		fr := flate.NewReader(bytes.NewReader(streamed))
		back, err := io.ReadAll(fr)
		if err != nil {
			t.Errorf("%s: stdlib cannot decode streaming output: %v", tc.name, err)
		} else if !bytes.Equal(back, tc.data) {
			t.Errorf("%s: stdlib decoded streaming output wrong", tc.name)
		}
	}
}

// TestStreamEmptyGolden pins the empty-stream bytes the streaming
// Writer must produce (same golden as Compress(nil)).
func TestStreamEmptyGolden(t *testing.T) {
	got := streamWrite(t, nil, 100)
	if !bytes.Equal(got, []byte{0x03, 0x00}) {
		t.Errorf("streaming empty = % x, want 03 00", got)
	}
	if back := streamRead(t, got, 4); len(back) != 0 {
		t.Errorf("streaming empty decode = %q", back)
	}
}

// TestStreamFlush delivers early: through a pipe, after Flush the
// written prefix is decodable before anything else is written (the
// writer side is held at the fence until the prefix is verified).
func TestStreamFlush(t *testing.T) {
	data := []byte(strings.Repeat("flush me early, ", 4000)) // crosses a slide
	half := len(data) / 2
	pr, pw := io.Pipe()
	resume := make(chan struct{})
	go func() {
		w := deflate.NewWriter(pw)
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

	// This ReadFull completes from the flushed block alone: the writer
	// cannot emit anything more until resume closes.
	r := deflate.NewReader(pr)
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

// TestStreamBuffered checks the trailer-recovery surface gzip and zlib
// rely on: bytes the reader staged past the end of the stream come
// back through Buffered.
func TestStreamBuffered(t *testing.T) {
	data := []byte("payload for the framing layers, repeated to be interesting. ")
	data = bytes.Repeat(data, 100)
	raw := deflate.Compress(data)
	framed := append(append([]byte{}, raw...), []byte("TRAILER")...)

	r := deflate.NewReader(bytes.NewReader(framed))
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("decode: %v", err)
	}
	if string(r.Buffered()) != "TRAILER" {
		t.Errorf("Buffered = %q, want TRAILER", r.Buffered())
	}
}

// TestStreamTruncated asserts that sampled proper prefixes of a
// streaming output error (never panic, never return short silently).
// The whole-buffer suite sweeps every prefix; here a stride keeps the
// O(n²) decode cost bounded.
func TestStreamTruncated(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	data := make([]byte, 30000)
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}
	full := streamWrite(t, data, 4096)
	for n := 0; n < len(full); n += 37 {
		r := deflate.NewReader(bytes.NewReader(full[:n]))
		got, err := io.ReadAll(r)
		if err == nil {
			// Only a prefix that still ends with a final block may
			// succeed; our single-stream output has one final block at
			// the very end, so no proper prefix may decode cleanly.
			t.Errorf("prefix %d decoded without error (%d bytes)", n, len(got))
		}
	}
}

// TestStreamCorruptNoPanic flips bits and truncates; the reader must
// error, never panic, on every mutation.
func TestStreamCorruptNoPanic(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 9))
	data := bytes.Repeat([]byte("corruption target — "), 3000)
	full := streamWrite(t, data, 1000)
	for i := 0; i < 300; i++ {
		mut := append([]byte{}, full...)
		mut[rng.IntN(len(mut))] ^= 1 << rng.IntN(8)
		r := deflate.NewReader(bytes.NewReader(mut[:rng.IntN(len(mut)+1)]))
		_, _ = io.ReadAll(r) // must not panic; errors are fine
		r.Close()
	}
}

// TestStreamMalformedErrType checks that corruption surfaces as
// ErrMalformed and truncation as ErrUnexpectedEOF.
func TestStreamMalformedErrType(t *testing.T) {
	// Reserved block type 11.
	r := deflate.NewReader(bytes.NewReader([]byte{0x07, 0x00}))
	_, err := io.ReadAll(r)
	if !errors.Is(err, deflate.ErrMalformed) {
		t.Errorf("reserved BTYPE: err = %v, want ErrMalformed", err)
	}
	// Truncation of a valid stream.
	full := streamWrite(t, bytes.Repeat([]byte("truncate me "), 5000), 4096)
	r2 := deflate.NewReader(bytes.NewReader(full[:len(full)/2]))
	_, err = io.ReadAll(r2)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated: err = %v, want ErrUnexpectedEOF", err)
	}
}

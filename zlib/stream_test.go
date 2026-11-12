/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zlib_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/zlib"
)

// zStreamWrite compresses data through the streaming Writer in chunks
// of the given size.
func zStreamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
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

// zStreamRead decompresses through the streaming Reader in chunks of
// the given size.
func zStreamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := zlib.NewReader(bytes.NewReader(data))
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

// TestZStreamRoundTrip runs the corpus through Writer and Reader at
// several chunk geometries.
func TestZStreamRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		for _, chunk := range []int{1, 7, 1024, 33333, 1 << 20} {
			z := zStreamWrite(t, data, chunk)
			if got := zStreamRead(t, z, chunk); !bytes.Equal(got, data) {
				t.Errorf("corpus %d chunk %d: round trip mismatch (%d in, %d out)",
					i, chunk, len(data), len(got))
			}
		}
	}
}

// TestZStreamEmptyGolden: an unused Writer closed immediately must
// produce exactly the whole-buffer Compress(nil) bytes.
func TestZStreamEmptyGolden(t *testing.T) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), zlib.Compress(nil)) {
		t.Errorf("empty stream = % x, want Compress(nil)", buf.Bytes())
	}
	if got := zStreamRead(t, buf.Bytes(), 4); len(got) != 0 {
		t.Errorf("empty decode = %q", got)
	}
}

// TestZStreamOneByteReader feeds the Reader through
// iotest.OneByteReader so every refill boundary is exercised a byte at
// a time.
func TestZStreamOneByteReader(t *testing.T) {
	for i, data := range corpus() {
		z := zStreamWrite(t, data, 4096)
		r := zlib.NewReader(iotest.OneByteReader(bytes.NewReader(z)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("corpus %d: one-byte-reader mismatch", i)
		}
	}
}

// TestZStreamFlush delivers early: after Flush the prefix is decodable
// through a pipe before anything more is written.
func TestZStreamFlush(t *testing.T) {
	data := []byte(strings.Repeat("flush me early, ", 4000))
	half := len(data) / 2
	pr, pw := io.Pipe()
	resume := make(chan struct{})
	go func() {
		w := zlib.NewWriter(pw)
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

	r := zlib.NewReader(pr)
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

// TestZStreamTruncated: sampled proper prefixes must error, never
// return short silently.
func TestZStreamTruncated(t *testing.T) {
	full := zStreamWrite(t, bytes.Repeat([]byte("truncate me "), 5000), 4096)
	for n := 1; n < len(full); n += 53 {
		r := zlib.NewReader(bytes.NewReader(full[:n]))
		_, err := io.ReadAll(r)
		if err == nil {
			t.Errorf("prefix %d decoded without error", n)
		}
	}
}

// TestZStreamErrTypes: a wrong Adler-32 and trailing bytes wrap
// ErrMalformed; truncation reports io.ErrUnexpectedEOF.
func TestZStreamErrTypes(t *testing.T) {
	full := zStreamWrite(t, bytes.Repeat([]byte("check me "), 5000), 4096)

	bad := append([]byte{}, full...)
	bad[len(bad)-2] ^= 0xFF // inside the Adler-32 trailer
	r := zlib.NewReader(bytes.NewReader(bad))
	_, err := io.ReadAll(r)
	if !errors.Is(err, zlib.ErrMalformed) {
		t.Errorf("bad adler: err = %v, want ErrMalformed", err)
	}

	r2 := zlib.NewReader(bytes.NewReader(full[:len(full)/2]))
	_, err = io.ReadAll(r2)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated: err = %v, want ErrUnexpectedEOF", err)
	}

	tail := append(append([]byte{}, full...), 'x')
	r3 := zlib.NewReader(bytes.NewReader(tail))
	_, err = io.ReadAll(r3)
	if !errors.Is(err, zlib.ErrMalformed) {
		t.Errorf("trailing byte: err = %v, want ErrMalformed", err)
	}

	// FDICT header: check bits valid (0x78 0xF9), dictionary required.
	r4 := zlib.NewReader(bytes.NewReader([]byte{0x78, 0xF9}))
	_, err = io.ReadAll(r4)
	if !errors.Is(err, zlib.ErrMalformed) {
		t.Errorf("FDICT: err = %v, want ErrMalformed", err)
	}
}

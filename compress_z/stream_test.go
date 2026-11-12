/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/compress_z"
)

// zcStreamWrite compresses data through the streaming Writer in
// chunks of the given size.
func zcStreamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := compress_z.NewWriter(&buf)
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

// zcStreamRead decompresses through the streaming Reader in chunks of
// the given size.
func zcStreamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := compress_z.NewReader(bytes.NewReader(data))
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

// TestZcStreamRoundTrip runs the corpus through Writer and Reader at
// several chunk geometries.
func TestZcStreamRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		for _, chunk := range []int{1, 7, 1024, 33333, 1 << 20} {
			z := zcStreamWrite(t, data, chunk)
			if got := zcStreamRead(t, z, chunk); !bytes.Equal(got, data) {
				t.Errorf("corpus %d chunk %d: round trip mismatch (%d in, %d out)",
					i, chunk, len(data), len(got))
			}
		}
	}
}

// TestZcStreamChunkIndependence: the greedy encoder is a pure
// function of the byte sequence (one byte of carry state, no window
// decisions), so the streaming output must equal CompressBits byte
// for byte however the input is chunked.
func TestZcStreamChunkIndependence(t *testing.T) {
	for i, data := range corpus() {
		want := compress_z.Compress(data)
		for _, chunk := range []int{1, 3, 4096} {
			if got := zcStreamWrite(t, data, chunk); !bytes.Equal(got, want) {
				t.Errorf("corpus %d chunk %d: streaming output differs from Compress", i, chunk)
			}
		}
	}
}

// TestZcStreamAllWidths exercises NewWriterBits 9..16 (including the
// three overshoot widths) and its panic on out-of-range values.
func TestZcStreamAllWidths(t *testing.T) {
	data := bytes.Repeat([]byte("width me, wider, widest. "), 4000)
	for width := 9; width <= 16; width++ {
		var buf bytes.Buffer
		w := compress_z.NewWriterBits(&buf, width)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("width %d: %v", width, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("width %d: %v", width, err)
		}
		if !bytes.Equal(buf.Bytes(), compress_z.CompressBits(data, width)) {
			t.Errorf("width %d: output differs from CompressBits", width)
		}
		if got := zcStreamRead(t, buf.Bytes(), 5000); !bytes.Equal(got, data) {
			t.Errorf("width %d: round trip mismatch", width)
		}
	}
	for _, bad := range []int{8, 17} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("width %d: NewWriterBits did not panic", bad)
				}
			}()
			compress_z.NewWriterBits(io.Discard, bad)
		}()
	}
}

// TestZcStreamEmptyGolden: an unused Writer closed immediately must
// produce exactly Compress(nil) — the 3-byte header alone.
func TestZcStreamEmptyGolden(t *testing.T) {
	var buf bytes.Buffer
	w := compress_z.NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), compress_z.Compress(nil)) {
		t.Errorf("empty stream = % x, want Compress(nil)", buf.Bytes())
	}
	if got := zcStreamRead(t, buf.Bytes(), 4); len(got) != 0 {
		t.Errorf("empty decode = %q", got)
	}
}

// TestZcStreamCrossWholeBuffer: each half reads the other's output.
func TestZcStreamCrossWholeBuffer(t *testing.T) {
	for i, data := range corpus() {
		streamed := zcStreamWrite(t, data, 8192)
		if got := compress_z.Expand(streamed); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: whole-buffer Expand cannot read streaming output", i)
		}
		whole := compress_z.Compress(data)
		if got := zcStreamRead(t, whole, 1000); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: streaming Reader cannot read whole-buffer output", i)
		}
	}
}

// TestZcStreamCLIGoldens: the streaming Reader decodes compress(1)
// output at several widths.
func TestZcStreamCLIGoldens(t *testing.T) {
	for name, hex := range map[string]string{
		"tobe9":  goldenTobe9Hex,
		"tobe12": goldenTobe12Hex,
		"tobe16": goldenTobe16Hex,
	} {
		if got := zcStreamRead(t, hexBytes(t, hex), 3); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
			t.Errorf("golden %s: got %q", name, got)
		}
	}
}

// TestZcStreamOneByteReader feeds the Reader through
// iotest.OneByteReader so every refill boundary is exercised a byte at
// a time.
func TestZcStreamOneByteReader(t *testing.T) {
	for i, data := range corpus() {
		z := compress_z.Compress(data)
		r := compress_z.NewReader(iotest.OneByteReader(bytes.NewReader(z)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("corpus %d: one-byte-reader mismatch", i)
		}
	}
}

// TestZcStreamPrefixDecodesPrefix: the format has no end framing —
// the stream simply stops — so any cut input decodes to a prefix of
// the full output (this is the streaming analogue of the whole-buffer
// suite's chop test, at a stride).
func TestZcStreamPrefixDecodesPrefix(t *testing.T) {
	data := bytes.Repeat([]byte("prefix property, "), 6000)
	full := compress_z.Compress(data)
	for n := 3; n < len(full); n += 61 {
		r := compress_z.NewReader(bytes.NewReader(full[:n]))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("cut %d: %v", n, err)
		}
		if !bytes.HasPrefix(data, got) {
			t.Fatalf("cut %d: output is not a prefix of the input data", n)
		}
	}
	// A header cut short is a shortage, not a clean prefix.
	r := compress_z.NewReader(bytes.NewReader([]byte{0x1f, 0x9d}))
	if _, err := io.ReadAll(r); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("short header: err = %v, want ErrUnexpectedEOF", err)
	}
}

// TestZcStreamErrTypes: a bad header wraps ErrMalformed; corruption
// beyond the live table surfaces as ErrMalformed too.
func TestZcStreamErrTypes(t *testing.T) {
	r := compress_z.NewReader(bytes.NewReader([]byte{0x1f, 0x9e, 0x90}))
	if _, err := io.ReadAll(r); !errors.Is(err, compress_z.ErrMalformed) {
		t.Errorf("bad magic: err = %v, want ErrMalformed", err)
	}

	// Code 257 as the very first code: beyond the live table (free is
	// 257, code > free is bad; code == free with oldcode < 0 is bad).
	stream := append([]byte{0x1f, 0x9d, 0x90}, pack([]uint32{258}, 9)...)
	r2 := compress_z.NewReader(bytes.NewReader(stream))
	if _, err := io.ReadAll(r2); !errors.Is(err, compress_z.ErrMalformed) {
		t.Errorf("bad first code: err = %v, want ErrMalformed", err)
	}
}

// TestZcStreamWriteAfterClose: writes after Close report an error and
// Close reports the sticky error rather than succeeding twice
// differently.
func TestZcStreamWriteAfterClose(t *testing.T) {
	var buf bytes.Buffer
	w := compress_z.NewWriter(&buf)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("more")); err == nil {
		t.Error("Write after Close: want error")
	}
	// The sticky Write-after-Close error surfaces from Close too, as
	// in the deflate/gzip/zlib/bzip2 writers.
	if err := w.Close(); err == nil {
		t.Error("second Close: want the sticky error")
	}
	if got := compress_z.Expand(buf.Bytes()); string(got) != "data" {
		t.Errorf("decode = %q", got)
	}
}

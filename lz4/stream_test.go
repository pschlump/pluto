/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/pschlump/pluto/lz4"
)

// lzStreamWrite compresses data through the streaming Writer in
// chunks of the given size.
func lzStreamWrite(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := lz4.NewWriter(&buf)
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

// lzStreamRead decompresses through the streaming Reader in chunks of
// the given size.
func lzStreamRead(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	r := lz4.NewReader(bytes.NewReader(data))
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

// TestLz4StreamRoundTrip runs the corpus through Writer and Reader at
// several chunk geometries.  Unlike .Z, the streaming output is NOT
// byte-identical to Compress: block boundaries fall wherever the 4 MB
// buffer happens to fill, so this is a value round trip only.
func TestLz4StreamRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		for _, chunk := range []int{1, 7, 1024, 33333, 1 << 20} {
			z := lzStreamWrite(t, data, chunk)
			if got := lzStreamRead(t, z, chunk); !bytes.Equal(got, data) {
				t.Errorf("corpus %d chunk %d: round trip mismatch (%d in, %d out)",
					i, chunk, len(data), len(got))
			}
		}
	}
}

// TestLz4StreamEmptyGolden: an unused Writer closed immediately must
// produce exactly Compress(nil) — the pinned 15-byte empty frame.
func TestLz4StreamEmptyGolden(t *testing.T) {
	var buf bytes.Buffer
	w := lz4.NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), lz4.Compress(nil)) {
		t.Errorf("empty stream = % x, want Compress(nil)", buf.Bytes())
	}
	if !bytes.Equal(buf.Bytes(), hexBytes(t, ourEmptyHex)) {
		t.Errorf("empty stream = % x, want the pinned golden", buf.Bytes())
	}
	if got := lzStreamRead(t, buf.Bytes(), 4); len(got) != 0 {
		t.Errorf("empty decode = %q", got)
	}
}

// TestLz4StreamCrossWholeBuffer: each half reads the other's output —
// the whole-buffer Expand (one-shot XXH32) verifies the streaming
// Writer's incremental content checksum, and the streaming Reader's
// incremental state is verified against Compress output.
func TestLz4StreamCrossWholeBuffer(t *testing.T) {
	for i, data := range corpus() {
		streamed := lzStreamWrite(t, data, 8192)
		if got := lz4.Expand(streamed); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: whole-buffer Expand cannot read streaming output", i)
		}
		whole := lz4.Compress(data)
		if got := lzStreamRead(t, whole, 1000); !bytes.Equal(got, data) {
			t.Errorf("corpus %d: streaming Reader cannot read whole-buffer output", i)
		}
	}
}

// TestLz4StreamCLIGoldens: the streaming Reader decodes lz4(1)
// frames, including the per-block-checksum (-BX) and content-size
// (--content-size) variants — the incremental content checksum against
// CLI-generated vectors.
func TestLz4StreamCLIGoldens(t *testing.T) {
	if got := lzStreamRead(t, hexBytes(t, goldenTobeHex), 3); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
		t.Errorf("golden tobe: got %q", got)
	}
	if got := lzStreamRead(t, hexBytes(t, goldenEmptyHex), 4); len(got) != 0 {
		t.Errorf("golden empty: got %d bytes", len(got))
	}
	dict := lz4.Expand(hexBytes(t, goldenDictHex))
	for name, hex := range map[string]string{
		"dict":     goldenDictHex,
		"dict -BX": goldenDictBCHex,
		"dict -CS": goldenDictCSHex,
	} {
		if got := lzStreamRead(t, hexBytes(t, hex), 37); !bytes.Equal(got, dict) {
			t.Errorf("golden %s: got %d bytes, want %d", name, len(got), len(dict))
		}
	}
}

// TestLz4StreamOneByteReader feeds the Reader through
// iotest.OneByteReader so every read boundary is exercised a byte at
// a time.
func TestLz4StreamOneByteReader(t *testing.T) {
	for i, data := range corpus() {
		z := lz4.Compress(data)
		r := lz4.NewReader(iotest.OneByteReader(bytes.NewReader(z)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("corpus %d: one-byte-reader mismatch", i)
		}
	}
}

// TestLz4StreamFrames: skippable frames are passed over, concatenated
// frames decode in sequence, and a stream of nothing but a skippable
// frame is a valid empty stream.
func TestLz4StreamFrames(t *testing.T) {
	a := []byte("TOBEORNOTTOBEORTOBEORNOT")
	b := bytes.Repeat([]byte("second frame, "), 500)

	// skip magic 0x184D2A50, size 5, junk payload.
	skip := append([]byte{0x50, 0x2A, 0x4D, 0x18, 0x05, 0x00, 0x00, 0x00}, "junky"...)
	stream := append(append(append([]byte{}, lz4.Compress(a)...), skip...), lz4.Compress(b)...)
	if got := lzStreamRead(t, stream, 61); !bytes.Equal(got, append(append([]byte{}, a...), b...)) {
		t.Errorf("skippable + concatenated: got %d bytes", len(got))
	}

	// Only a skippable frame: clean empty decode.
	r := lz4.NewReader(bytes.NewReader(skip))
	if got, err := io.ReadAll(r); err != nil || len(got) != 0 {
		t.Errorf("skippable-only: got %d bytes, err %v", len(got), err)
	}
}

// TestLz4StreamTruncated: every prefix of a valid frame is a shortage
// — never a clean end, never a different error.  (A prefix that ends
// at a frame boundary is a valid frame of its own only when the
// content checksum also landed, which for a single frame means the
// whole stream.)
func TestLz4StreamTruncated(t *testing.T) {
	full := lz4.Compress([]byte(bytes.Repeat([]byte("truncate me, "), 1500)))
	for n := 0; n < len(full); n += 97 {
		r := lz4.NewReader(bytes.NewReader(full[:n]))
		_, err := io.ReadAll(r)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("cut %d: err = %v, want ErrUnexpectedEOF", n, err)
		}
	}
	// The empty source is malformed the same way (a valid empty frame
	// is 15 bytes), matching Expand(nil).
	r := lz4.NewReader(bytes.NewReader(nil))
	if _, err := io.ReadAll(r); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("empty source: err = %v, want ErrUnexpectedEOF", err)
	}
}

// TestLz4StreamErrTypes: each corruption lands on the right side of
// the ErrMalformed / ErrUnexpectedEOF split, and no single-byte
// corruption panics or slips through the checksums unnoticed.
func TestLz4StreamErrTypes(t *testing.T) {
	read := func(z []byte) error {
		_, err := io.ReadAll(lz4.NewReader(bytes.NewReader(z)))
		return err
	}

	// Structural rejections: ErrMalformed.
	flip := func(hex string, i byte, v byte) []byte {
		b := hexBytes(t, hex)
		b[i] = v
		return b
	}
	for name, z := range map[string][]byte{
		"bad magic":     flip(goldenTobeHex, 2, 0x4E),
		"header check":  flip(goldenTobeHex, 6, 0x00),
		"wrong version": flip(goldenTobeHex, 4, 0xA4),
		"linked blocks": flip(goldenTobeHex, 4, 0x44),
		"dictID frame":  flip(goldenTobeHex, 4, 0x65),
	} {
		if err := read(z); !errors.Is(err, lz4.ErrMalformed) {
			t.Errorf("%s: err = %v, want ErrMalformed", name, err)
		}
	}

	// A block size word over the declared maximum (64 KB here): the
	// size is checked before the block is read.
	over := hexBytes(t, goldenTobeHex)
	over[7], over[8], over[9], over[10] = 0x22, 0x00, 0x01, 0x00
	if err := read(over); !errors.Is(err, lz4.ErrMalformed) {
		t.Errorf("block over max: err = %v, want ErrMalformed", err)
	}

	// A wrong content checksum.
	wrong := lz4.Compress([]byte("checksum me"))
	wrong[len(wrong)-1] ^= 0xFF
	if err := read(wrong); !errors.Is(err, lz4.ErrMalformed) {
		t.Errorf("content checksum: err = %v, want ErrMalformed", err)
	}

	// Trailing garbage: a complete bad magic is malformed; an
	// incomplete one is a shortage (the stdlib gzip rule).
	trail := append(lz4.Compress([]byte("x")), 0xFF, 0xFF, 0xFF, 0xFF)
	if err := read(trail); !errors.Is(err, lz4.ErrMalformed) {
		t.Errorf("trailing garbage: err = %v, want ErrMalformed", err)
	}
	short := append(lz4.Compress([]byte("x")), 0xFF)
	if err := read(short); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("short trailing: err = %v, want ErrUnexpectedEOF", err)
	}

	// No single-byte corruption of a CLI frame decodes cleanly: the
	// header, block and content checksums always catch something.
	good := hexBytes(t, goldenTobeHex)
	for i := range good {
		bad := append([]byte(nil), good...)
		bad[i] ^= 0x55
		if err := read(bad); err == nil {
			t.Errorf("flip byte %d: decoded without error", i)
		}
	}
}

// TestLz4StreamFlushBlocks: Flush between writes ends blocks early;
// the stream still decodes (with different block boundaries than
// Compress would draw).
func TestLz4StreamFlushBlocks(t *testing.T) {
	data := bytes.Repeat([]byte("flush between me, "), 3000)
	var buf bytes.Buffer
	w := lz4.NewWriter(&buf)
	for i := 0; i < len(data); i += 9000 {
		end := min(i+9000, len(data))
		if _, err := w.Write(data[i:end]); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := lz4.Expand(buf.Bytes()); !bytes.Equal(got, data) {
		t.Errorf("flushed blocks: Expand mismatch (%d bytes)", len(got))
	}
	if got := lzStreamRead(t, buf.Bytes(), 512); !bytes.Equal(got, data) {
		t.Errorf("flushed blocks: stream read mismatch")
	}
}

// TestLz4StreamFlushPipe: after a Flush the flushed bytes are
// decodable without the writer writing anything more (io.Pipe is
// synchronous, so the reader must make progress while the writer is
// parked past the Flush).
func TestLz4StreamFlushPipe(t *testing.T) {
	pr, pw := io.Pipe()
	w := lz4.NewWriter(pw)
	resume := make(chan struct{})
	go func() {
		if _, err := w.Write([]byte("flushed-")); err != nil {
			t.Errorf("write: %v", err)
		}
		if err := w.Flush(); err != nil {
			t.Errorf("flush: %v", err)
		}
		<-resume
		if _, err := w.Write([]byte("then-some")); err != nil {
			t.Errorf("write2: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
		if err := pw.Close(); err != nil {
			t.Errorf("pipe close: %v", err)
		}
	}()

	r := lz4.NewReader(pr)
	got := make([]byte, 8)
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read flushed: %v", err)
	}
	if string(got) != "flushed-" {
		t.Fatalf("flushed bytes = %q", got)
	}
	close(resume)
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read rest: %v", err)
	}
	if string(rest) != "then-some" {
		t.Errorf("rest = %q", rest)
	}
}

// TestLz4StreamWriteAfterClose: writes after Close report an error and
// Close reports the sticky error, as in the other codec writers.
func TestLz4StreamWriteAfterClose(t *testing.T) {
	var buf bytes.Buffer
	w := lz4.NewWriter(&buf)
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("more")); err == nil {
		t.Error("Write after Close: want error")
	}
	if err := w.Close(); err == nil {
		t.Error("second Close: want the sticky error")
	}
	if got := lz4.Expand(buf.Bytes()); string(got) != "data" {
		t.Errorf("decode = %q", got)
	}
}

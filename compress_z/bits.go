/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z

import "io"

// The .Z bit convention (ncompress compress42.c): LZW codes are
// packed least-significant-bit first — the first bit of a code lands
// in the 0x01 position of the next byte, higher bits climb within and
// across bytes, and the final partial byte of the stream is padded
// with zero bits.  This is the OPPOSITE end from pluto/lzw, which
// follows algs4's MSB-first BinaryStdOut convention — the two
// packages share an algorithm family, not a wire format.

// bitWriter accumulates LSB-first codes.  The streaming Writer of
// stream.go drains buf to its destination mid-stream; drained keeps
// the group-alignment math on an absolute bit scale that survives
// draining (it is 0 for the whole-buffer encoder, which never
// drains).
type bitWriter struct {
	buf     []byte
	acc     uint64
	n       uint
	base    int // absolute bit position where the current code group started
	drained int // bytes moved out of buf so far (streaming only)
}

// writeCode appends a code of n bits (n <= 16).
func (w *bitWriter) writeCode(code uint32, n uint) {
	w.acc |= uint64(code) << w.n
	w.n += n
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

// totalBits reports the absolute bit position just past all bits
// written so far.
func (w *bitWriter) totalBits() int { return (w.drained+len(w.buf))*8 + int(w.n) }

// bytes returns the output, zero-padding the final partial byte.
func (w *bitWriter) bytes() []byte {
	if w.n > 0 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc, w.n = 0, 0
	}
	return w.buf
}

// realign zero-pads up to the next group boundary: the .Z layout packs
// codes in groups of eight, so a width change or a CLEAR must land on
// a multiple of 8*n bits from the current segment's start.  Growth
// points are always naturally aligned (they occur after 2^n-256
// codes, a multiple of 8); CLEAR is the case that observably pads.
func (w *bitWriter) realign(n uint) {
	rel := w.totalBits() - w.base
	m := 8 * int(n)
	if pad := (m - rel%m) % m; pad > 0 {
		w.writeCode(0, uint(pad))
	}
	w.base = w.totalBits()
}

// bitReader is the matching LSB-first reader.  It has two modes, as
// deflate's bitReader does: whole-buffer (Expand — buf is the whole
// input) and streaming (the Reader of stream.go — a source is
// attached and the stage refills through fill()).  Compaction drops
// consumed bytes from the head of buf; dropped keeps the
// group-alignment math on an absolute bit scale that survives it
// (0 in whole-buffer mode, which never compacts).
type bitReader struct {
	buf  []byte
	pos  int
	acc  uint32
	n    uint
	base int // absolute bit position where the current code group started

	src     io.Reader // streaming mode when non-nil
	srcEOF  bool
	fillErr error
	dropped int // bytes dropped by compaction (streaming only)
}

// realign skips the zero padding up to the next 8*n-bit group
// boundary (the mirror of bitWriter.realign).  Bits in the final
// partial group past the last code are never read.
func (r *bitReader) realign(n uint) {
	rel := r.consumedBits() - r.base
	m := 8 * int(n)
	if pad := (m - rel%m) % m; pad > 0 {
		for i := 0; i < pad; i++ {
			if _, ok := r.readCode(1); !ok {
				return // stream ends inside the padding — clean end
			}
		}
	}
	r.base = r.consumedBits()
}

// consumedBits reports the absolute bit position of the reader
// through all bits consumed so far.
func (r *bitReader) consumedBits() int { return (r.dropped+r.pos)*8 - int(r.n) }

// readCode reads an n-bit code.  ok is false when fewer than n bits
// remain — the .Z end-of-stream condition (there is no EOF code; the
// stream simply stops).
func (r *bitReader) readCode(n uint) (uint32, bool) {
	for r.n < n {
		if r.pos >= len(r.buf) {
			if !r.fill() {
				return 0, false
			}
		}
		r.acc |= uint32(r.buf[r.pos]) << r.n
		r.pos++
		r.n += 8
	}
	code := r.acc & (1<<n - 1)
	r.acc >>= n
	r.n -= n
	return code, true
}

// fill stages more source bytes in streaming mode; in whole-buffer
// mode there is nothing to fill — the slice is the whole input and
// its end is the end.  Compaction keeps the stage bounded however
// long the stream runs; a (0, nil) read is retried a bounded number
// of times and then reported as no progress.
func (r *bitReader) fill() bool {
	if r.src == nil || r.srcEOF || r.fillErr != nil {
		return false
	}
	const chunk = 32 << 10
	if r.pos > chunk {
		n := copy(r.buf, r.buf[r.pos:])
		r.buf = r.buf[:n]
		r.dropped += r.pos
		r.pos = 0
	}
	tail := len(r.buf)
	if cap(r.buf)-tail < chunk {
		grow := make([]byte, tail, tail+2*chunk)
		copy(grow, r.buf)
		r.buf = grow
	}
	for tries := 0; tries < 100; tries++ {
		r.buf = r.buf[:cap(r.buf)]
		n, err := r.src.Read(r.buf[tail:cap(r.buf)])
		r.buf = r.buf[:tail+n]
		if n > 0 {
			return true
		}
		if err == io.EOF {
			r.srcEOF = true
			return false
		}
		if err != nil {
			r.fillErr = err
			return false
		}
	}
	r.fillErr = io.ErrNoProgress
	return false
}

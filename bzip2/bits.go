/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2

import "io"

// The bzip2 bit convention: everything is written MOST-significant-bit
// first — the first bit of a field lands in the 0x80 position of the
// next byte, and Huffman codes are emitted MSB-first too (unlike
// DEFLATE, whose base stream is LSB-first and whose codes are
// therefore pre-reversed).  The final partial byte of the stream is
// padded with zero bits.

// msbWriter accumulates MSB-first fields.
type msbWriter struct {
	buf []byte
	acc uint64
	n   uint
}

// writeBits appends the low n bits of v (n <= 48), MSB first.
func (w *msbWriter) writeBits(v uint64, n uint) {
	v &= (1 << n) - 1
	w.acc = w.acc<<n | v
	w.n += n
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acc>>(w.n-8)))
		w.n -= 8
	}
}

// bytes returns the output, zero-padding the final partial byte.
func (w *msbWriter) bytes() []byte {
	if w.n > 0 {
		w.buf = append(w.buf, byte(w.acc<<(8-w.n)))
		w.acc, w.n = 0, 0
	}
	return w.buf
}

// msbReader is the matching MSB-first reader.  It has two modes, as
// deflate's bitReader does: whole-buffer (Expand — buf is the whole
// input, its end is the end) and streaming (the Reader of stream.go —
// a source is attached and the stage refills through fill()).
type msbReader struct {
	buf []byte
	pos int
	acc uint64
	n   uint

	src        io.Reader // streaming mode when non-nil
	srcEOF     bool
	fillErr    error
	readFailed bool // a bit read hit end-of-input (streaming mode)
}

// readBits reads an n-bit field (n <= 48).  ok is false when fewer
// than n bits remain.
func (r *msbReader) readBits(n uint) (uint64, bool) {
	for r.n < n {
		if r.pos >= len(r.buf) {
			if !r.fill() {
				r.readFailed = true
				return 0, false
			}
		}
		r.acc = r.acc<<8 | uint64(r.buf[r.pos])
		r.pos++
		r.n += 8
	}
	v := r.acc >> (r.n - n)
	if n < 64 {
		v &= (1 << n) - 1
	}
	r.n -= n
	return v, true
}

// fill stages more source bytes in streaming mode; in whole-buffer
// mode there is nothing to fill.  Compaction keeps the stage bounded
// however long the stream runs; a (0, nil) read is retried a bounded
// number of times and then reported as no progress.
func (r *msbReader) fill() bool {
	if r.src == nil || r.srcEOF || r.fillErr != nil {
		return false
	}
	const chunk = 32 << 10
	if r.pos > chunk {
		n := copy(r.buf, r.buf[r.pos:])
		r.buf = r.buf[:n]
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

// readBit reads one bit.
func (r *msbReader) readBit() (uint64, bool) {
	v, ok := r.readBits(1)
	return v, ok
}

// alignToByte discards the bits of the final partial byte, if any.
func (r *msbReader) alignToByte() {
	r.n -= r.n % 8
}

// bytePos is the byte offset of the reader into its buffer (call
// after alignToByte to land on a member boundary).
func (r *msbReader) bytePos() int {
	return r.pos - int(r.n/8)
}

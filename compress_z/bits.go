/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z

// The .Z bit convention (ncompress compress42.c): LZW codes are
// packed least-significant-bit first — the first bit of a code lands
// in the 0x01 position of the next byte, higher bits climb within and
// across bytes, and the final partial byte of the stream is padded
// with zero bits.  This is the OPPOSITE end from pluto/lzw, which
// follows algs4's MSB-first BinaryStdOut convention — the two
// packages share an algorithm family, not a wire format.

// bitWriter accumulates LSB-first codes.
type bitWriter struct {
	buf  []byte
	acc  uint64
	n    uint
	base int // bit position where the current code group started
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
	total := len(w.buf)*8 + int(w.n)
	rel := total - w.base
	m := 8 * int(n)
	if pad := (m - rel%m) % m; pad > 0 {
		w.writeCode(0, uint(pad))
	}
	w.base = len(w.buf)*8 + int(w.n)
}

// bitReader is the matching LSB-first reader.
type bitReader struct {
	buf  []byte
	pos  int
	acc  uint32
	n    uint
	base int // bit position where the current code group started
}

// realign skips the zero padding up to the next 8*n-bit group
// boundary (the mirror of bitWriter.realign).  Bits in the final
// partial group past the last code are never read.
func (r *bitReader) realign(n uint) {
	consumed := r.pos*8 - int(r.n)
	rel := consumed - r.base
	m := 8 * int(n)
	if pad := (m - rel%m) % m; pad > 0 {
		for i := 0; i < pad; i++ {
			if _, ok := r.readCode(1); !ok {
				return // stream ends inside the padding — clean end
			}
		}
	}
	r.base = r.pos*8 - int(r.n)
}

// readCode reads an n-bit code.  ok is false when fewer than n bits
// remain — the .Z end-of-stream condition (there is no EOF code; the
// stream simply stops).
func (r *bitReader) readCode(n uint) (uint32, bool) {
	for r.n < n {
		if r.pos >= len(r.buf) {
			return 0, false
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

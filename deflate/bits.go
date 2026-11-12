/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate

// DEFLATE packs every field — block headers, extra bits, and Huffman
// codes alike — into bytes least-significant-bit first (RFC 1951
// §3.1.1).  Huffman *codes* are still emitted most-significant-bit
// first; encoders bridge the two conventions by pre-reversing each
// canonical code (see reverseBits in huff.go) so that the reversed
// integer can go through the same LSB-first writer as everything else.

// bitWriter accumulates bits LSB-first into an internal byte slice.
// It is always safe to inspect buf between calls; pending partial
// bits live in acc/n only.
type bitWriter struct {
	buf []byte
	acc uint64 // pending bits; the LOWEST bit is the OLDEST
	n   uint   // number of pending bits in acc, 0..7 after any call
}

// writeBits appends the low n bits of v, least significant bit first.
func (w *bitWriter) writeBits(v uint, n uint) {
	w.acc |= uint64(v&((1<<n)-1)) << w.n
	w.n += n
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

// writeCode appends a Huffman code held in write order (already
// bit-reversed so the code's most significant bit goes out first).
func (w *bitWriter) writeCode(c huffCode) {
	w.writeBits(uint(c.bits), uint(c.len))
}

// alignBytes pads with zero bits up to the next byte boundary.
func (w *bitWriter) alignBytes() {
	if w.n > 0 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc, w.n = 0, 0
	}
}

// bytes flushes any partial byte (zero-padded) and returns the output.
func (w *bitWriter) bytes() []byte {
	w.alignBytes()
	return w.buf
}

// bitReader is the matching LSB-first reader.
type bitReader struct {
	buf []byte
	pos int    // index of the next byte to load
	acc uint32 // loaded bits; the LOWEST bit is the OLDEST unread one
	n   uint   // number of unread bits in acc
}

// readBit returns the next single bit.  ok is false at end of input.
func (r *bitReader) readBit() (uint, bool) {
	if r.n == 0 {
		if r.pos >= len(r.buf) {
			return 0, false
		}
		r.acc = uint32(r.buf[r.pos])
		r.pos++
		r.n = 8
	}
	b := r.acc & 1
	r.acc >>= 1
	r.n--
	return uint(b), true
}

// readBits reads n bits (n <= 16), least significant bit first.
func (r *bitReader) readBits(n uint) (uint, bool) {
	var v uint
	for i := uint(0); i < n; i++ {
		b, ok := r.readBit()
		if !ok {
			return 0, false
		}
		v |= b << i
	}
	return v, true
}

// align discards the remaining bits of the current partial byte
// (used to reach the byte-aligned LEN/NLEN of a stored block).
func (r *bitReader) align() {
	r.acc = 0
	r.n = 0
}

// consumed reports how many bytes of buf have been consumed, where a
// byte holding only trailing padding bits of the final block counts as
// consumed.  Containers layered on top (gzip, zlib) use this to find
// where their trailer starts.
func (r *bitReader) consumed() int { return r.pos }

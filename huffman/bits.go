/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman

// This file holds the bit-level I/O of the huffman blob format.  It
// replicates the algs4 BinaryStdOut / BinaryStdIn conventions exactly:
// a value written with writeBits(x, r) emits the r low bits of x from
// bit r-1 down to bit 0, and the byte buffer fills MSB-first (the first
// bit written lands in the 0x80 position of the first byte).  The final
// partial byte is padded with 0 bits (BinaryStdOut.close).
//
// The file is a deliberate duplicate of lzw/bits.go (the backlog's
// "duplicate small" option, decided once): g_lib's charter is stateless
// pure functions, so a stateful bit writer does not belong there, and
// the repository has no internal/ sharing mechanism — two ~60-line
// copies for lzw, run_length-adjacent bit order and this package beat a
// new coupling.  If a third consumer appears, lift it into an internal
// package instead.

// bitWriter packs bits into a byte slice, MSB-first.
type bitWriter struct {
	out   []byte // the bytes flushed so far
	buf   byte   // the not-yet-full byte being accumulated
	nbits int    // the number of bits accumulated in buf
}

// writeBit appends one bit (0 or 1) to the stream.
func (w *bitWriter) writeBit(bit uint) {
	w.buf = w.buf<<1 | byte(bit)
	w.nbits++
	if w.nbits == 8 {
		w.out = append(w.out, w.buf)
		w.buf = 0
		w.nbits = 0
	}
}

// writeBits appends the r low bits of x, from bit r-1 down to bit 0 —
// exactly BinaryStdOut.write(x, r).  x must fit in r bits.
func (w *bitWriter) writeBits(x, r int) {
	for i := r - 1; i >= 0; i-- {
		w.writeBit(uint(x>>i) & 1)
	}
}

// writeCode appends a codeword held as a string of '0'/'1' characters
// (the form buildCode produces).
func (w *bitWriter) writeCode(code string) {
	for i := 0; i < len(code); i++ {
		w.writeBit(uint(code[i] - '0'))
	}
}

// bytes flushes the final partial byte, padding its low bits with 0s
// (BinaryStdOut.close), and returns the packed stream.
func (w *bitWriter) bytes() []byte {
	if w.nbits > 0 {
		w.out = append(w.out, w.buf<<(8-w.nbits))
		w.buf = 0
		w.nbits = 0
	}
	return w.out
}

// bitReader unpacks an MSB-first bit stream produced by bitWriter.
type bitReader struct {
	data []byte
	pos  int // the next bit to read: pos>>3 is the byte, 7-pos&7 the bit within it
}

// readBits reads the next r bits, MSB-first — exactly
// BinaryStdIn.readInt(r).  ok is false when fewer than r bits remain.
func (r *bitReader) readBits(n int) (x int, ok bool) {
	if r.pos+n > len(r.data)*8 {
		return 0, false
	}
	for range n {
		x <<= 1
		if r.data[r.pos>>3]&(0x80>>(r.pos&7)) != 0 {
			x |= 1
		}
		r.pos++
	}
	return x, true
}

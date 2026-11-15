/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

// Backward and forward bit readers, the two bit orders the zstd
// format uses.  FSE and Huffman streams are read backward from the
// last byte (whose highest set bit is a marker, not data); FSE table
// descriptions are read forward, LSB-first.

// bitReader consumes bits from the end of a stream toward its
// beginning, most-significant bit first within each byte — the order
// the entropy-coded streams are written in.  Reads past the beginning
// return zero bits and raise the overflow flag rather than panicking,
// because the Huffman-weights FSE decode relies on exactly that
// zero-fill to find its end.
type bitReader struct {
	b    []byte
	pos  int    // bytes in b[0:pos] not yet loaded; loading walks pos down
	acc  uint64 // pending bits, right-aligned
	n    int    // number of valid bits in acc
	over bool   // a read needed more bits than the stream had
}

func newBitReader(b []byte) (*bitReader, bool) {
	if len(b) == 0 || b[len(b)-1] == 0 {
		return nil, false // the last byte carries the marker; zero is corrupt
	}
	last := b[len(b)-1]
	nb := bitLen8(last) - 1 // bits below the marker
	r := &bitReader{b: b, pos: len(b) - 1}
	r.acc = uint64(last) & ((1 << nb) - 1)
	r.n = nb
	r.reload()
	return r, true
}

// reload tops the accumulator up to at least 57 bits while bytes
// remain, so ordinary reads never straddle a reload.
func (r *bitReader) reload() {
	for r.n <= 56 && r.pos > 0 {
		r.pos--
		r.acc = r.acc<<8 | uint64(r.b[r.pos])
		r.n += 8
	}
}

// read takes the next n bits (n ≤ 31).  Past the end it returns
// zero-filled values and flags the overflow.
func (r *bitReader) read(n int) uint32 {
	if n == 0 {
		return 0
	}
	if r.n < n {
		v := uint32(r.acc<<uint(n-r.n)) & mask(n)
		r.n = 0
		r.over = true
		return v
	}
	r.n -= n
	v := uint32(r.acc >> uint(r.n))
	r.reload()
	return v & mask(n)
}

// peek looks at the next n bits without consuming them (n ≤ 31); it
// zero-pads past the end like read.
func (r *bitReader) peek(n int) uint32 {
	if n == 0 {
		return 0
	}
	if r.n < n {
		return uint32(r.acc<<uint(n-r.n)) & mask(n)
	}
	return uint32(r.acc>>uint(r.n-n)) & mask(n)
}

// skip consumes n bits already peeked.
func (r *bitReader) skip(n int) {
	if r.n < n {
		r.n = 0
		r.over = true
		return
	}
	r.n -= n
	r.reload()
}

// done reports whether the stream is exactly consumed.
func (r *bitReader) done() bool { return r.pos == 0 && r.n == 0 }

func mask(n int) uint32 { return 1<<uint(n) - 1 }

func bitLen8(b byte) int {
	n := 0
	for b != 0 {
		n++
		b >>= 1
	}
	return n
}

func highbit32(v uint32) int {
	n := -1
	for v != 0 {
		n++
		v >>= 1
	}
	return n
}

// fwdBits reads a stream forward, least-significant bit first — the
// order FSE table descriptions (normalized counts) are written in.
// peek loads lazily, so long runs of small fields never read stale
// accumulator bits; consumed counts bits taken, not bytes loaded.
type fwdBits struct {
	b        []byte
	pos      int
	acc      uint32
	n        int
	consumed int
}

func (f *fwdBits) load() bool {
	for f.n <= 24 {
		if f.pos >= len(f.b) {
			return f.n > 0
		}
		f.acc |= uint32(f.b[f.pos]) << uint(f.n)
		f.pos++
		f.n += 8
	}
	return true
}

func (f *fwdBits) peek(n int) uint32 {
	for f.n < n && f.pos < len(f.b) {
		f.acc |= uint32(f.b[f.pos]) << uint(f.n)
		f.pos++
		f.n += 8
	}
	return f.acc & mask(n)
}

func (f *fwdBits) skip(n int) {
	f.acc >>= uint(n)
	f.n -= n
	f.consumed += n
}

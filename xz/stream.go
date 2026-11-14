/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"github.com/pschlump/pluto/internal/lzmacore"
	"hash"
	"io"

	"github.com/pschlump/pluto/crc"
)

// The streaming API: the same .xz format as Compress/Expand, driven
// through io.Reader and io.Writer.  The Reader decodes one LZMA2
// chunk at a time — a chunk's compressed size is a 16-bit field, so
// memory is one chunk's data plus a dictionary window, never the
// whole stream.  The Writer buffers input up to the chunk size and
// encodes a chunk whenever the buffer is full; Flush ends the current
// chunk early, and everything written so far is decodable once the
// chunk lands.

// ErrMalformed marks a structurally invalid stream: a bad magic,
// wrong header/footer CRCs, a reserved flag, a filter that is not
// LZMA2, bad filter properties, a corrupt chunk sequence, a wrong
// block check, an index that disagrees with the blocks, or trailing
// garbage.  Truncation — input ending anywhere the format needs more
// bytes — is reported as io.ErrUnexpectedEOF.  Both surface from
// Read, sticky.
var ErrMalformed = errors.New("xz: malformed stream")

// maxDictWindow is the dictionary tail the streaming Reader retains
// (the 8 MiB property our writer declares; streams declaring more
// still decode, distances just cannot reach further back than this).
const maxDictWindow = 8 << 20

// Reader states.
const (
	stMagic  = iota // at a stream header (or padding, or the clean end)
	stBlocks        // inside a stream's block loop
	stIndex         // the index and footer
)

// blockInfo tracks one block for the index verification.
type blockInfo struct {
	unpadded, uncomp int64
	data             int64 // compressed bytes, including the end marker
}

// Reader decodes .xz streams — one, several concatenated, with stream
// padding between — from src.
type Reader struct {
	src  io.Reader
	br   *bufio.Reader
	err  error
	out  []byte // decoded but undelivered bytes; doubles as the dictionary
	base int64  // absolute stream position of out[0]
	path int
	n    int64 // src bytes consumed

	check    int
	st       *lzmaState
	dictBase int64 // absolute position of the last dictionary reset
	block    *blockInfo

	blocks   []blockInfo
	idxBytes []byte // the index as read (indicator through padding), for its CRC
	sawOne   bool   // at least one stream has been decoded
	c32      uint32
	c64      uint64
	sha      hash.Hash
}

// NewReader starts decoding src.
func NewReader(src io.Reader) *Reader {
	return &Reader{src: src, br: bufio.NewReaderSize(src, 64*1024)}
}

// Read delivers decoded bytes.
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	for len(r.out) == 0 && r.err == nil {
		switch r.path {
		case stMagic:
			r.atMagic()
		case stBlocks:
			r.atBlocks()
		case stIndex:
			r.atIndex()
		}
	}
	if n := copy(p, r.out); n > 0 {
		r.out = r.out[n:]
		r.base += int64(n)
		if drop := int64(len(r.out)) - maxDictWindow; drop > 0 {
			r.out = r.out[drop:]
			r.base += drop
		}
		return n, nil
	}
	return 0, r.err
}

// Close releases the reader (nothing held beyond memory).
func (r *Reader) Close() error { return nil }

func (r *Reader) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *Reader) malformed() { r.fail(ErrMalformed) }

// readFull fills b, mapping EOF to io.ErrUnexpectedEOF.
func (r *Reader) readFull(b []byte) bool {
	if _, err := io.ReadFull(r.br, b); err != nil {
		if err == io.EOF {
			r.fail(io.ErrUnexpectedEOF)
		} else {
			r.fail(err)
		}
		return false
	}
	r.n += int64(len(b))
	return true
}

// atMagic consumes stream padding and then a stream header, or ends
// cleanly at EOF once at least one stream has been decoded.
func (r *Reader) atMagic() {
	var zeros [4]byte
	for {
		magic, err := r.br.Peek(6)
		if err == io.EOF {
			if len(magic) == 0 && r.sawOne {
				r.fail(io.EOF) // clean end, possibly after padding
				return
			}
			if len(magic) == 0 {
				r.fail(io.ErrUnexpectedEOF)
			} else {
				r.malformed() // a short non-empty tail
			}
			return
		}
		if err != nil {
			r.fail(err)
			return
		}
		if magic[0]|magic[1]|magic[2]|magic[3] != 0 {
			break // not padding
		}
		if !r.sawOne {
			r.malformed() // padding before any stream
			return
		}
		if !r.readFull(zeros[:]) {
			return
		}
	}
	var hdr [12]byte
	if !r.readFull(hdr[:]) {
		return
	}
	if string(hdr[:6]) != "\xFD7zXZ\x00" ||
		crc32Bytes(hdr[6:8]) != le32(hdr[8:12]) ||
		hdr[6] != 0 || hdr[7]&0xF0 != 0 || !validCheck(int(hdr[7]&0x0F)) {
		r.malformed()
		return
	}
	r.check = int(hdr[7] & 0x0F)
	r.blocks = nil
	r.path = stBlocks
}

// atBlocks parses one block header and its chunks, or hands off to
// the index when the block loop ends.
func (r *Reader) atBlocks() {
	first, err := r.br.ReadByte()
	if err != nil {
		r.fail(io.ErrUnexpectedEOF)
		return
	}
	r.n++
	if first == 0x00 { // index indicator
		r.path = stIndex
		return
	}
	hdrSize := (int(first) + 1) * 4
	if hdrSize < 8 || hdrSize > 1024 {
		r.malformed()
		return
	}
	hdr := make([]byte, hdrSize)
	hdr[0] = first
	if !r.readFull(hdr[1:]) {
		return
	}
	if crc32Bytes(hdr[:hdrSize-4]) != le32(hdr[hdrSize-4:]) {
		r.malformed()
		return
	}
	flags := hdr[1]
	if flags&0x3C != 0 || flags&3 != 0 {
		r.malformed()
		return
	}
	p := 2
	if flags&0x40 != 0 { // declared compressed size (accepted; the index settles it)
		_, np, ok := readVLI(hdr, p)
		if !ok {
			r.malformed()
			return
		}
		p = np
	}
	if flags&0x80 != 0 { // declared uncompressed size
		_, np, ok := readVLI(hdr, p)
		if !ok {
			r.malformed()
			return
		}
		p = np
	}
	fid, np, ok := readVLI(hdr, p)
	if !ok || fid != lzma2FilterID {
		r.malformed()
		return
	}
	psize, np, ok := readVLI(hdr, np)
	if !ok || psize != 1 || hdr[np] > 40 {
		r.malformed()
		return
	}

	r.block = &blockInfo{unpadded: int64(hdrSize)}
	r.st = nil
	r.dictBase = r.base + int64(len(r.out))
	r.c32, r.c64, r.sha = 0, 0, sha256.New()
	r.decodeChunks()
}

// decodeChunks runs one block's LZMA2 chunk loop to completion.
// Decoded bytes land in r.out; Read delivers them even after the
// loop has moved on to later blocks.
func (r *Reader) decodeChunks() {
	b := r.block
	var dataBytes int64
	for {
		ctl, err := r.br.ReadByte()
		if err != nil {
			r.fail(io.ErrUnexpectedEOF)
			return
		}
		r.n++
		dataBytes++
		switch ctl {
		case 0x00: // end of the block's data
			b.data = dataBytes
			b.unpadded += dataBytes + int64(checkSize[r.check])
			r.blocks = append(r.blocks, *b)
			for dataBytes%4 != 0 { // block padding
				pad, err := r.br.ReadByte()
				if err != nil {
					r.fail(io.ErrUnexpectedEOF)
					return
				}
				r.n++
				dataBytes++
				if pad != 0 {
					r.malformed()
					return
				}
			}
			var want [32]byte
			if !r.readFull(want[:checkSize[r.check]]) {
				return
			}
			if !r.checkOK(want[:checkSize[r.check]]) {
				r.malformed()
				return
			}
			return // stay in stBlocks (or the next byte is the index)

		case 0x01, 0x02: // uncompressed chunk
			var sz [2]byte
			if !r.readFull(sz[:]) {
				return
			}
			size := int(sz[0])<<8 | int(sz[1])
			dataBytes += 2
			if ctl == 0x01 {
				r.dictBase = r.base + int64(len(r.out))
				r.st = nil
			}
			start := len(r.out)
			r.out = append(r.out, make([]byte, size+1)...)
			if !r.readFull(r.out[start:]) {
				return
			}
			dataBytes += int64(size) + 1
			b.uncomp += int64(size) + 1
			r.addData(r.out[start:])

		default: // an LZMA chunk (bits 4-0 of ctl are the size, none reserved)
			reset := int(ctl>>5) & 3
			var sizes [4]byte
			if !r.readFull(sizes[:]) {
				return
			}
			unpacked := (int(ctl&0x1F)<<16 | int(sizes[0])<<8 | int(sizes[1])) + 1
			comp := (int(sizes[2])<<8 | int(sizes[3])) + 1
			dataBytes += 4
			if reset == 3 {
				comp++ // the properties byte rides ahead of the data
			}
			data := make([]byte, comp)
			if !r.readFull(data) {
				return
			}
			dataBytes += int64(comp)
			switch reset {
			case 3:
				if len(data) < 1 || data[0] > 224 {
					r.malformed()
					return
				}
				r.st = newLzmaState(data[0])
				data = data[1:]
			case 2:
				if r.st == nil {
					r.malformed()
					return
				}
				r.st.ResetProbs()
			case 1:
				if r.st == nil {
					r.malformed()
					return
				}
				r.dictBase = r.base + int64(len(r.out))
			case 0:
				if r.st == nil {
					r.malformed()
					return
				}
			}
			before := len(r.out)
			dictRel := r.dictBase - r.base
			if dictRel < 0 {
				dictRel = 0 // the window has trimmed past the reset point
			}
			rc := newRangeDecoder(data)
			if !lzmaDecode(rc, r.st, &r.out, before+unpacked, int(dictRel)) {
				r.malformed()
				return
			}
			b.uncomp += int64(unpacked)
			r.addData(r.out[before:])
		}
	}
}

// checkOK verifies the current block's check against the running
// hashes of its decoded data.
func (r *Reader) checkOK(want []byte) bool {
	switch r.check {
	case 0:
		return true
	case 1:
		return r.c32 == le32(want)
	case 4:
		return r.c64 == le64(want)
	case 10:
		return string(r.sha.Sum(nil)) == string(want)
	}
	return false
}

// addData folds freshly decoded bytes into the running check hashes.
func (r *Reader) addData(b []byte) {
	switch r.check {
	case 1:
		r.c32 = crc.Update32(r.c32, crc.IEEETable, b)
	case 4:
		r.c64 = crc.Update64(r.c64, crc.ECMATable, b)
	case 10:
		r.sha.Write(b)
	}
}

// atIndex reads the index and footer and verifies them against the
// blocks just decoded.
func (r *Reader) atIndex() {
	r.idxBytes = []byte{0x00} // the indicator atBlocks consumed
	count, ok := r.readVLIStream()
	if !ok || count != int64(len(r.blocks)) {
		r.malformed()
		return
	}
	for i := int64(0); i < count; i++ {
		unp, ok := r.readVLIStream()
		if !ok {
			r.malformed()
			return
		}
		unc, ok := r.readVLIStream()
		if !ok {
			r.malformed()
			return
		}
		if r.blocks[i].unpadded != unp || r.blocks[i].uncomp != unc {
			r.malformed()
			return
		}
	}
	for (len(r.idxBytes)+4)%4 != 0 { // index padding
		pad, err := r.br.ReadByte()
		if err != nil || pad != 0 {
			r.malformed()
			return
		}
		r.n++
		r.idxBytes = append(r.idxBytes, pad)
	}
	var crcb [4]byte
	if !r.readFull(crcb[:]) {
		return
	}
	if crc32Bytes(r.idxBytes) != le32(crcb[:]) {
		r.malformed()
		return
	}
	var ftr [12]byte
	if !r.readFull(ftr[:]) {
		return
	}
	if crc32Bytes(ftr[4:10]) != le32(ftr[0:4]) ||
		int64(le32(ftr[4:8])+1)*4 != int64(len(r.idxBytes))+4 ||
		string(ftr[10:12]) != "YZ" {
		r.malformed()
		return
	}
	r.sawOne = true
	r.path = stMagic
}

// readVLIStream reads a variable-length integer, tracking the bytes
// for the index CRC.
func (r *Reader) readVLIStream() (int64, bool) {
	var v uint64
	for i := 0; i < 9; i++ {
		b, err := r.br.ReadByte()
		if err != nil {
			return 0, false
		}
		r.n++
		r.idxBytes = append(r.idxBytes, b)
		v |= uint64(b&0x7F) << uint(7*i)
		if b&0x80 == 0 {
			return int64(v), true
		}
	}
	return 0, false
}

// ----------------------------------------------------------------------------
// Writer

// Writer encodes one .xz stream (a single block, like Compress).
type Writer struct {
	dst     io.Writer
	err     error
	pend    []byte     // buffered input, flushed a chunk at a time
	window  []byte     // the dictionary tail for cross-chunk distances
	st      *lzmaState // the probability model, carried across chunks
	abs     int64      // absolute position of the next input byte
	started bool
	fresh   bool // no chunk yet: the first carries the properties
	closed  bool

	dataBytes int64 // compressed bytes of the current block so far
	uncomp    int64
	unpadded  int64
	c64       uint64
}

// NewWriter starts a stream on dst.
func NewWriter(dst io.Writer) *Writer {
	return &Writer{dst: dst, fresh: true}
}

func (w *Writer) write(b []byte) bool {
	if w.err != nil {
		return false
	}
	if _, err := w.dst.Write(b); err != nil {
		w.err = err
		return false
	}
	return true
}

func (w *Writer) start() bool {
	var hdr [12]byte
	hdr[6], hdr[7] = 0x00, 0x04
	putLE32(hdr[8:12], crc32Bytes(hdr[6:8]))
	hdr[0], hdr[1], hdr[2], hdr[3], hdr[4], hdr[5] = 0xFD, '7', 'z', 'X', 'Z', 0x00
	if !w.write(hdr[:]) {
		return false
	}
	bh := []byte{0x02, 0x00, lzma2FilterID, 0x01, dictProp8MiB, 0, 0, 0}
	bh = appendLE32(bh, crc32Bytes(bh))
	if !w.write(bh) {
		return false
	}
	w.started = true
	w.dataBytes = 0
	w.uncomp = 0
	w.unpadded = int64(len(bh))
	return true
}

// Write buffers input, encoding a chunk whenever the buffer reaches
// the chunk size.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	total := len(p)
	for len(w.pend)+len(p) >= chunkMaxUnpacked {
		if !w.started && !w.start() {
			return 0, w.err
		}
		take := chunkMaxUnpacked - len(w.pend)
		w.pend = append(w.pend, p[:take]...)
		p = p[take:]
		w.flushChunk()
		if w.err != nil {
			return 0, w.err
		}
	}
	w.pend = append(w.pend, p...)
	return total, nil
}

// Flush ends the current chunk early: everything written so far
// becomes decodable.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if len(w.pend) == 0 {
		return nil
	}
	if !w.started && !w.start() {
		return w.err
	}
	w.flushChunk()
	return w.err
}

// flushChunk encodes the pending input as one LZMA2 chunk (or
// uncompressed chunks when compression does not pay).
func (w *Writer) flushChunk() {
	if len(w.pend) == 0 {
		return
	}
	cut := w.pend
	enc, st := lzmacore.LzmaEncode(nil, w.window, cut, w.st, w.abs)
	w.st = st
	w.abs += int64(len(cut))
	if len(enc)+6 < len(cut) {
		ctl := byte(0x80)
		if w.fresh {
			ctl = 0xE0 // state reset + new properties
		}
		unpacked := len(cut) - 1
		compField := len(enc) - 1
		head := []byte{ctl | byte(unpacked>>16), byte(unpacked >> 8), byte(unpacked), byte(compField >> 8), byte(compField)}
		if w.fresh {
			head = append(head, defaultProps)
		}
		if !w.write(head) || !w.write(enc) {
			return
		}
		w.dataBytes += int64(len(head)) + int64(len(enc))
	} else {
		src := cut
		for len(src) > 0 {
			n := len(src)
			if n > 1<<16 {
				n = 1 << 16
			}
			ctl := byte(0x02)
			if w.fresh {
				ctl = 0x01
			}
			if !w.write([]byte{ctl, byte((n - 1) >> 8), byte(n - 1)}) || !w.write(src[:n]) {
				return
			}
			w.dataBytes += int64(3 + n)
			src = src[n:]
			w.fresh = false
		}
	}
	w.window = append(w.window, cut...)
	if len(w.window) > maxDictWindow {
		w.window = w.window[len(w.window)-maxDictWindow:]
	}
	w.uncomp += int64(len(cut))
	w.c64 = crc.Update64(w.c64, crc.ECMATable, cut)
	w.pend = w.pend[:0]
	w.fresh = false
}

// Close ends the block (check, index, footer).  It does not close
// the underlying writer.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	if len(w.pend) > 0 {
		if !w.started && !w.start() {
			return w.err
		}
		w.flushChunk()
		if w.err != nil {
			return w.err
		}
	}
	if !w.started { // an empty stream: a block with no chunks
		if !w.start() {
			return w.err
		}
	}
	if !w.write([]byte{0x00}) { // end of block data
		return w.err
	}
	w.dataBytes++
	unpaddedTotal := w.unpadded + w.dataBytes + 8 // data (no padding) + header + check
	for w.dataBytes%4 != 0 {                      // block padding
		if !w.write([]byte{0x00}) {
			return w.err
		}
		w.dataBytes++
	}
	var chk [8]byte
	putLE64(chk[:], w.c64)
	if !w.write(chk[:]) {
		return w.err
	}
	idx := []byte{0x00, 0x01}
	idx = writeVLI(idx, unpaddedTotal)
	idx = writeVLI(idx, w.uncomp)
	for (len(idx)+4)%4 != 0 {
		idx = append(idx, 0x00)
	}
	idx = appendLE32(idx, crc32Bytes(idx))
	if !w.write(idx) {
		return w.err
	}
	ftr := make([]byte, 0, 6)
	ftr = appendLE32(ftr, uint32(len(idx)/4-1))
	ftr = append(ftr, 0x00, 0x04)
	tail := appendLE32(nil, crc32Bytes(ftr))
	tail = append(tail, ftr...)
	tail = append(tail, 'Y', 'Z')
	if !w.write(tail) {
		return w.err
	}
	w.closed = true
	return nil
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func putLE64(b []byte, v uint64) {
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * uint(i)))
	}
}

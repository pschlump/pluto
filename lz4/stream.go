/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4

import (
	"errors"
	"fmt"
	"io"
)

// The streaming API: the same LZ4 frame format as Compress/Expand,
// driven through io.Reader and io.Writer.  The frame is a chain of
// length-prefixed, independent, byte-aligned blocks, so streaming is
// natural: the Reader reads one block at a time (memory is one
// block's expansion plus a small read buffer) and the Writer buffers
// input up to the block size and encodes a block whenever the buffer
// is full — Flush ends the current block early, and because blocks
// are independent everything written so far is decodable at once.
// The content checksum runs incrementally (xxh32State below).

// ErrMalformed marks a structurally invalid frame: a bad magic, a
// wrong FLG version, linked blocks, a dictID frame, a wrong header
// checksum, block sizes over the declared maximum, a corrupt block
// sequence, a wrong per-block or content checksum, a wrong declared
// content size, or trailing bytes that are not another frame.
// Truncation — input ending anywhere the format needs more bytes — is
// reported as io.ErrUnexpectedEOF.  Both surface from Read, sticky.
var ErrMalformed = errors.New("lz4: malformed stream")

// Reader states: between frames the next bytes are a magic (or the
// clean end of the stream); inside a frame the header, then the block
// loop; the current block's expansion is delivered before decoding
// continues.
const (
	stLMagic = iota
	stLHeader
	stLBlocks
)

// Reader decodes LZ4 frames — one, several concatenated, with
// skippable frames between them — from src.
type Reader struct {
	src        io.Reader
	state      int
	sawFrame   bool // a frame (or skippable) completed: end is clean
	flg        byte
	blockMax   int
	contentLen int64  // running decoded length, for the C.Size check
	contentCap int64  // -1 when the frame declares no C.Size
	xx         xxh32s // content checksum over the current frame's output
	out        []byte // current block: delivered as read
	err        error
}

// NewReader returns a Reader decoding the LZ4 frame stream in src.
// An empty src is reported as io.ErrUnexpectedEOF — a valid empty
// frame is 15 bytes, and Expand of nothing is malformed the same way.
// A clean io.EOF comes only after at least one frame (or skippable
// frame) has completed.
func NewReader(src io.Reader) *Reader {
	return &Reader{src: src, state: stLMagic}
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for r.err == nil {
		if n := len(r.out); n > 0 {
			c := copy(p, r.out)
			r.out = r.out[c:]
			return c, nil
		}
		switch r.state {
		case stLMagic:
			var magic [4]byte
			if _, err := io.ReadFull(r.src, magic[:]); err != nil {
				if err == io.EOF && r.sawFrame {
					r.err = io.EOF // clean end of the stream
				} else {
					r.err = frameErr(err) // nothing-yet, or short garbage
				}
				continue
			}
			m := readLE32(magic[:])
			if m&0xFFFFFFF0 == skipMagicHi&0xFFFFFFF0 {
				var sz [4]byte
				if _, err := io.ReadFull(r.src, sz[:]); err != nil {
					r.err = frameErr(err)
					continue
				}
				skip := int64(readLE32(sz[:]))
				if n, err := io.CopyN(io.Discard, r.src, skip); err != nil || n != skip {
					r.err = frameErr(err)
					continue
				}
				r.sawFrame = true
				continue // stay in stLMagic: what follows is another frame
			}
			if m != frameMagic {
				r.err = fmt.Errorf("%w: bad frame magic", ErrMalformed)
				continue
			}
			r.state = stLHeader
		case stLHeader:
			var hb [2]byte
			if _, err := io.ReadFull(r.src, hb[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			r.flg = hb[0]
			flg, bd := hb[0], hb[1]
			switch {
			case flg>>6 != 1:
				r.err = fmt.Errorf("%w: wrong version", ErrMalformed)
				continue
			case flg&0x02 != 0:
				r.err = fmt.Errorf("%w: reserved FLG bit", ErrMalformed)
				continue
			case flg&flgIndep == 0:
				r.err = fmt.Errorf("%w: linked blocks unsupported", ErrMalformed)
				continue
			case flg&0x01 != 0:
				r.err = fmt.Errorf("%w: dictID frame", ErrMalformed)
				continue
			case bd&0x80 != 0 || bd&0x0F != 0:
				r.err = fmt.Errorf("%w: reserved BD bits", ErrMalformed)
				continue
			case bd>>4&7 < 4:
				r.err = fmt.Errorf("%w: block size under 64KB", ErrMalformed)
				continue
			}
			r.blockMax = 1 << (8 + 2*(bd>>4&7))
			r.contentLen = 0
			r.contentCap = -1
			r.xx.reset()
			// Optional C.Size (8 bytes LE), then the header checksum over
			// FLG+BD(+C.Size) — the magic is NOT included.
			hdr := []byte{flg, bd}
			if flg&0x08 != 0 {
				var cs [8]byte
				if _, err := io.ReadFull(r.src, cs[:]); err != nil {
					r.err = frameErr(err)
					continue
				}
				r.contentCap = int64(uint64(readLE32(cs[:])) | uint64(readLE32(cs[4:]))<<32)
				hdr = append(hdr, cs[:]...)
			}
			var hc [1]byte
			if _, err := io.ReadFull(r.src, hc[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			if hc[0] != byte(xxh32(hdr)>>8) {
				r.err = fmt.Errorf("%w: header checksum", ErrMalformed)
				continue
			}
			r.state = stLBlocks
		case stLBlocks:
			var sw [4]byte
			if _, err := io.ReadFull(r.src, sw[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			szWord := readLE32(sw[:])
			if szWord == 0 { // EndMark
				if r.flg&flgContentSum != 0 {
					var cc [4]byte
					if _, err := io.ReadFull(r.src, cc[:]); err != nil {
						r.err = frameErr(err)
						continue
					}
					if readLE32(cc[:]) != r.xx.sum() {
						r.err = fmt.Errorf("%w: content checksum", ErrMalformed)
						continue
					}
				}
				if r.contentCap >= 0 && r.contentCap != r.contentLen {
					r.err = fmt.Errorf("%w: content size", ErrMalformed)
					continue
				}
				r.sawFrame = true
				r.state = stLMagic // a concatenated frame may follow
				continue
			}
			raw := szWord&0x80000000 != 0
			sz := int(szWord & 0x7FFFFFFF)
			if sz == 0 || sz > r.blockMax {
				r.err = fmt.Errorf("%w: block size over maximum", ErrMalformed)
				continue
			}
			block := make([]byte, sz)
			if _, err := io.ReadFull(r.src, block); err != nil {
				r.err = frameErr(err)
				continue
			}
			if r.flg&0x10 != 0 { // per-block checksum over the STORED bytes
				var bc [4]byte
				if _, err := io.ReadFull(r.src, bc[:]); err != nil {
					r.err = frameErr(err)
					continue
				}
				if readLE32(bc[:]) != xxh32(block) {
					r.err = fmt.Errorf("%w: block checksum", ErrMalformed)
					continue
				}
			}
			if raw {
				r.out = block
			} else {
				chunk := expandBlock(block, 1<<20)
				if chunk == nil {
					r.err = fmt.Errorf("%w: corrupt block", ErrMalformed)
					continue
				}
				r.out = chunk
			}
			r.xx.write(r.out)
			r.contentLen += int64(len(r.out))
		}
	}
	return 0, r.err
}

// Close reports the sticky error, if any (io.EOF is not an error).
// It is provided for io.ReadCloser symmetry.
func (r *Reader) Close() error {
	if r.err == io.EOF {
		return nil
	}
	return r.err
}

func frameErr(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// Writer incrementally encodes what is written to it as one LZ4 frame
// on dst: the 7-byte header on the first write (or at Close for an
// empty frame), 4 MB independent blocks (a full buffer encodes as one
// block, storing raw when compression does not help, exactly as
// Compress), and at Close the end mark and the incremental XXH32
// content checksum.  Flush ends the current block early; blocks are
// independent, so everything written is decodable at a Flush or
// block boundary.  Errors are sticky.
type Writer struct {
	dst    io.Writer
	buf    []byte // pending uncompressed input, <= defaultBlock after each Write
	xx     xxh32s // content checksum over everything written
	hdr    bool
	closed bool
	err    error
}

// NewWriter returns a Writer encoding into dst.  Close must be called
// to finish the frame.
func NewWriter(dst io.Writer) *Writer {
	w := &Writer{dst: dst}
	w.xx.reset() // seed the accumulators (a zero value is not the seed)
	return w
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("lz4: Write after Close")
		return 0, w.err
	}
	if err := w.start(); err != nil {
		return 0, w.err
	}
	w.buf = append(w.buf, p...)
	for len(w.buf) >= defaultBlock && w.err == nil {
		w.encodeBlock(w.buf[:defaultBlock])
		rest := copy(w.buf, w.buf[defaultBlock:])
		w.buf = w.buf[:rest]
	}
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

// Flush ends the current block so a reader can decode everything
// written so far.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if w.hdr && !w.closed && len(w.buf) > 0 {
		w.encodeBlock(w.buf)
		w.buf = w.buf[:0]
	}
	return w.err
}

// Close writes the pending block (if any), the end mark, and the
// content checksum.  Calling it twice reports the sticky error.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	if err := w.start(); err != nil {
		return w.err
	}
	w.closed = true
	if len(w.buf) > 0 {
		w.encodeBlock(w.buf)
		w.buf = w.buf[:0]
	}
	if w.err != nil {
		return w.err
	}
	if _, err := w.dst.Write([]byte{0, 0, 0, 0}); err != nil && w.err == nil {
		w.err = err
	}
	if w.err != nil {
		return w.err
	}
	v := w.xx.sum()
	var cc [4]byte
	cc[0], cc[1], cc[2], cc[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	if _, err := w.dst.Write(cc[:]); err != nil && w.err == nil {
		w.err = err
	}
	return w.err
}

// start writes the frame header once, lazily, so an unused Writer
// still produces a valid empty frame at Close.
func (w *Writer) start() error {
	if w.hdr {
		return nil
	}
	w.hdr = true
	hdr := []byte{0x04, 0x22, 0x4D, 0x18, flgVersion | flgIndep | flgContentSum, bdBlockBits}
	hdr = append(hdr, byte(xxh32(hdr[4:])>>8))
	if _, err := w.dst.Write(hdr); err != nil {
		w.err = err
		return err
	}
	return nil
}

// encodeBlock encodes one independent block from src and writes it
// with its size word, storing raw when compression does not help.
func (w *Writer) encodeBlock(src []byte) {
	w.xx.write(src)
	block := compressBlock(src)
	if len(block) < len(src) {
		w.writeWord(uint32(len(block)))
		if w.err == nil {
			_, err := w.dst.Write(block)
			if err != nil && w.err == nil {
				w.err = err
			}
		}
		return
	}
	w.writeWord(uint32(len(src)) | 0x80000000)
	if w.err == nil {
		_, err := w.dst.Write(src)
		if err != nil && w.err == nil {
			w.err = err
		}
	}
}

func (w *Writer) writeWord(v uint32) {
	if w.err != nil {
		return
	}
	var b [4]byte
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	if _, err := w.dst.Write(b[:]); err != nil && w.err == nil {
		w.err = err
	}
}

// xxh32s is the incremental form of xxh32: bytes are written in any
// number of chunks; sum() returns the same value xxh32 would over the
// concatenation.  Full 16-byte stripes fold into the four
// accumulators as they complete; the tail (fewer than 16 bytes) is
// buffered until sum() mixes it.
type xxh32s struct {
	acc1, acc2, acc3, acc4 uint32
	buf                    [16]byte
	nbuf                   int
	total                  uint64
}

func (s *xxh32s) reset() {
	// The reference seeds: p1+p2 and -p1 computed in uint32 wraparound
	// (the untyped constants themselves overflow uint32).
	var p1, p2 uint32 = xxp1, xxp2
	s.acc1, s.acc2, s.acc3, s.acc4 = p1+p2, p2, 0, -p1
	s.nbuf = 0
	s.total = 0
}

// write folds p into the state, completing stripes through the buffer.
func (s *xxh32s) write(p []byte) {
	if len(p) == 0 {
		return // never disturbs a buffered partial stripe
	}
	s.total += uint64(len(p))
	// Fill the partial stripe first.
	if s.nbuf > 0 {
		c := copy(s.buf[s.nbuf:], p)
		s.nbuf += c
		p = p[c:]
		if s.nbuf < 16 {
			return
		}
		s.stripe(s.buf[:])
		s.nbuf = 0
	}
	for len(p) >= 16 {
		s.stripe(p)
		p = p[16:]
	}
	s.nbuf = copy(s.buf[:], p)
}

func (s *xxh32s) stripe(b []byte) {
	s.acc1 = xxhRound(s.acc1, readLE32(b))
	s.acc2 = xxhRound(s.acc2, readLE32(b[4:]))
	s.acc3 = xxhRound(s.acc3, readLE32(b[8:]))
	s.acc4 = xxhRound(s.acc4, readLE32(b[12:]))
}

// sum finishes the checksum: the small-input construction when the
// whole input stayed under one stripe, otherwise the accumulator
// convergence plus the tail mix, then the avalanche.
func (s *xxh32s) sum() uint32 {
	n := uint32(s.total)
	var h uint32
	if s.total < 16 {
		b := s.buf[:s.nbuf]
		h = n + xxp5
		i := 0
		for ; i+4 <= len(b); i += 4 {
			h += readLE32(b[i:]) * xxp3
			h = rotl32(h, 17) * xxp4
		}
		for ; i < len(b); i++ {
			h += uint32(b[i]) * xxp5
			h = rotl32(h, 11) * xxp1
		}
	} else {
		h = rotl32(s.acc1, 1) + rotl32(s.acc2, 7) + rotl32(s.acc3, 12) + rotl32(s.acc4, 18) + n
		b := s.buf[:s.nbuf]
		i := 0
		for ; i+4 <= len(b); i += 4 {
			h += readLE32(b[i:]) * xxp3
			h = rotl32(h, 17) * xxp4
		}
		for ; i < len(b); i++ {
			h += uint32(b[i]) * xxp5
			h = rotl32(h, 11) * xxp1
		}
	}
	h ^= h >> 15
	h *= xxp2
	h ^= h >> 13
	h *= xxp3
	h ^= h >> 16
	return h
}

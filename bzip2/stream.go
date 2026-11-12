/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2

import (
	"errors"
	"fmt"
	"io"
)

// The streaming API: the same bzip2 format as Compress/Expand, driven
// through io.Reader and io.Writer.  The format is block-oriented
// (100k..900k by level), so the natural streaming shape is
// block-at-a-time: the Reader decodes one block at a time through the
// refilling msbReader (decodeBlock runs verbatim; memory is one block
// plus a bounded input stage), and the Writer buffers input up to the
// block size and encodes a block whenever the buffer is full.  The
// Reader decodes concatenated members exactly as Expand does; bytes
// the msbReader staged past a member's final padding come back to feed
// the next member's header — nothing is read twice or lost.

// ErrMalformed marks a structurally invalid stream: a bad header, a
// bad block or end-of-stream magic, the deprecated randomized bit,
// out-of-range tree counts, code lengths or selectors, an over-long
// zero run, a block that overruns its declared size, a mismatched
// block or stream CRC, or trailing garbage after the final member.
// Truncation — input ending anywhere the format needs more bytes — is
// reported as io.ErrUnexpectedEOF.  Both surface from Read, sticky.
var ErrMalformed = errors.New("bzip2: malformed stream")

// Reader states: between members the next bytes are a BZh<level>
// header (or the clean end of the file); inside a member the
// block/final magic loop runs; the current block's bytes are
// delivered before decoding continues.
const (
	stBzHeader = iota
	stBzBlocks
)

// Reader decodes a bzip2 stream — one member or many concatenated —
// from src.
type Reader struct {
	in    seqReader // leftover bytes (from the previous member) first, then src
	br    msbReader // streaming mode, over in
	state int
	head  [4]byte // member header under construction

	blockSize int
	fileCRC   uint32
	out       []byte // current block: out[:delivered] delivered, rest pending
	delivered int

	err error
}

// NewReader returns a Reader decoding the bzip2 stream in src.  An
// empty src is a valid empty stream.
func NewReader(src io.Reader) *Reader {
	r := &Reader{state: stBzHeader}
	r.in.src = src
	r.br.src = &r.in
	return r
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for r.err == nil {
		if n := len(r.out) - r.delivered; n > 0 {
			c := copy(p, r.out[r.delivered:])
			r.delivered += c
			if r.delivered == len(r.out) {
				r.out, r.delivered = nil, 0 // release the block
			}
			return c, nil
		}
		switch r.state {
		case stBzHeader:
			// The first header byte alone separates a clean end (empty
			// stream, or no more members) from a header that starts but
			// cannot finish.
			if _, err := io.ReadFull(&r.in, r.head[:1]); err != nil {
				if err == io.EOF {
					r.err = io.EOF
				} else {
					r.err = err
				}
				continue
			}
			if _, err := io.ReadFull(&r.in, r.head[1:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			if r.head[0] != 'B' || r.head[1] != 'Z' || r.head[2] != 'h' {
				r.err = fmt.Errorf("%w: bad member magic", ErrMalformed)
				continue
			}
			if r.head[3] < '1' || r.head[3] > '9' {
				r.err = fmt.Errorf("%w: bad level %q", ErrMalformed, r.head[3])
				continue
			}
			r.blockSize = 100000 * int(r.head[3]-'0')
			r.fileCRC = 0
			r.br = msbReader{src: &r.in}
			r.state = stBzBlocks
		case stBzBlocks:
			magic, ok := r.br.readBits(48)
			if !ok {
				r.starved()
				continue
			}
			switch magic {
			case blockMagic:
				block, crc, ok := decodeBlock(&r.br, r.blockSize)
				if !ok {
					r.starved()
					continue
				}
				r.fileCRC = combineCRC(r.fileCRC, crc)
				r.out = block
			case finalMagic:
				want, ok := r.br.readBits(32)
				if !ok {
					r.starved()
					continue
				}
				if r.fileCRC != uint32(want) {
					r.err = fmt.Errorf("%w: stream CRC mismatch", ErrMalformed)
					continue
				}
				// The member ends inside its final partial byte; bytes
				// staged past it belong to the next member.
				r.br.alignToByte()
				r.in.reset(r.br.buf[r.br.bytePos():])
				r.state = stBzHeader
			default:
				r.err = fmt.Errorf("%w: bad block magic", ErrMalformed)
			}
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

// starved classifies a decode failure: if a bit read failed for want
// of input it is truncation, otherwise the stream itself is
// malformed.
func (r *Reader) starved() {
	if r.br.readFailed {
		if r.br.fillErr != nil {
			r.err = r.br.fillErr
			return
		}
		r.err = io.ErrUnexpectedEOF
		return
	}
	r.err = fmt.Errorf("%w: bad block", ErrMalformed)
}

func frameErr(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// seqReader serves bytes left over from a previous member first, then
// reads the original source.  It never reads ahead.
type seqReader struct {
	buf []byte
	pos int
	src io.Reader
}

func (s *seqReader) Read(p []byte) (int, error) {
	if s.pos < len(s.buf) {
		n := copy(p, s.buf[s.pos:])
		s.pos += n
		return n, nil
	}
	if s.src == nil {
		return 0, io.EOF
	}
	return s.src.Read(p)
}

func (s *seqReader) reset(leftover []byte) {
	s.buf = leftover
	s.pos = 0
}

// Writer incrementally encodes what is written to it as one bzip2
// stream on dst.  Input is buffered until a full block (blockSize
// bytes by level) is at hand, then encoded and drained, so memory is
// one block's worth of input plus one block's encoder state
// regardless of stream length.  Flush and Close end the current block
// early (a short block is as legal as a full one); Close writes the
// end-of-stream magic and combined CRC, and an empty stream is
// exactly Compress(nil).  Errors are sticky.
type Writer struct {
	dst       io.Writer
	mw        msbWriter
	buf       []byte // pending uncompressed input, < blockSize after each Write
	blockSize int
	fileCRC   uint32
	hdr       bool
	closed    bool
	err       error
}

// NewWriter returns a Writer encoding into dst at level 9 (bzip2's
// default: 900k blocks).  Close must be called to finish the stream.
func NewWriter(dst io.Writer) *Writer {
	return NewWriterLevel(dst, 9)
}

// NewWriterLevel returns a Writer encoding into dst with the given
// block size level 1..9.  It panics on an out-of-range level, as
// CompressLevel does.
func NewWriterLevel(dst io.Writer, level int) *Writer {
	if level < 1 || level > 9 {
		panic(fmt.Sprintf("bzip2.NewWriterLevel: level %d out of range, want 1..9", level))
	}
	return &Writer{dst: dst, blockSize: 100000 * level}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("bzip2: Write after Close")
		return 0, w.err
	}
	if err := w.start(); err != nil {
		return 0, w.err
	}
	w.buf = append(w.buf, p...)
	for len(w.buf) >= w.blockSize && w.err == nil {
		w.encodePending()
	}
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

// Flush ends the current member and starts a new one, so a reader can
// decode everything written so far.  bzip2 is one continuous bit
// stream with no mid-member byte-alignment point (a block's final
// Huffman symbol can end mid-byte, and those tail bits cannot be
// emitted until more of the stream exists), so ending the member —
// final magic, combined CRC, zero pad to the byte boundary, then a
// fresh BZh<level> header — is the only shape a flush can take.  The
// cost is ~15 bytes per Flush; the decoded content is unchanged
// (concatenated members concatenate).
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if !w.hdr || w.closed {
		return w.err
	}
	if len(w.buf) > 0 {
		w.encodePending()
	}
	if w.err != nil {
		return w.err
	}
	w.mw.writeBits(finalMagic, 48)
	w.mw.writeBits(uint64(w.fileCRC), 32)
	w.drain(w.mw.bytes()) // bytes() zero-pads the member's final partial byte
	w.fileCRC = 0
	w.mw.buf = append(w.mw.buf[:0], 'B', 'Z', 'h', byte('0'+w.blockSize/100000))
	w.drain(nil)
	return w.err
}

// Close writes the final block (if any input is pending), the
// end-of-stream magic, and the combined CRC.  Calling it twice
// reports the sticky error.
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
		w.encodePending()
	}
	if w.err != nil {
		return w.err
	}
	w.mw.writeBits(finalMagic, 48)
	w.mw.writeBits(uint64(w.fileCRC), 32)
	w.drain(w.mw.bytes()) // bytes() zero-pads the final partial byte
	return w.err
}

// start writes the BZh<level> header once, lazily, so an unused
// Writer still produces a valid empty stream at Close.
func (w *Writer) start() error {
	if w.hdr {
		return nil
	}
	w.hdr = true
	w.mw.buf = append(w.mw.buf[:0], 'B', 'Z', 'h', byte('0'+w.blockSize/100000))
	w.drain(nil)
	if w.err != nil {
		return w.err
	}
	return nil
}

// encodePending encodes one block from the head of buf (up to
// blockSize bytes of input) and slides the remainder down.
func (w *Writer) encodePending() {
	rle, consumed := rleFill(w.buf, w.blockSize)
	blockCRC := updateCRC(0, w.buf[:consumed])
	encodeBlock(rle, blockCRC, &w.mw)
	w.fileCRC = combineCRC(w.fileCRC, blockCRC)
	rest := copy(w.buf, w.buf[consumed:])
	w.buf = w.buf[:rest]
	w.drain(nil)
}

// drain moves completed output bytes to dst.  Only whole bytes go out
// (pad is nil); Close passes mw.bytes() to flush the zero-padded
// final partial byte.
func (w *Writer) drain(padded []byte) {
	out := w.mw.buf
	if len(padded) > 0 {
		out = padded
	}
	if len(out) == 0 {
		return
	}
	if _, err := w.dst.Write(out); err != nil && w.err == nil {
		w.err = err
	}
	w.mw.buf = w.mw.buf[:0]
}

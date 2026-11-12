/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zlib

import (
	"errors"
	"fmt"
	"io"

	"github.com/pschlump/pluto/deflate"
)

// The streaming API: the same zlib framing as Compress/Expand, driven
// through io.Reader and io.Writer.  Memory is bounded regardless of
// stream length.  As in gzip's streaming layer, the DEFLATE body runs
// through a deflate.Reader over the source, and the bytes that reader
// staged past its final block come back (deflate.Reader.Buffered) to
// feed the Adler-32 trailer — nothing is read twice or lost.

// ErrMalformed marks a structurally invalid stream: a bad header
// (reserved CM, CINFO > 7, wrong check bits, FDICT set), a wrong
// Adler-32, or trailing bytes after the trailer.  Truncation is
// reported as io.ErrUnexpectedEOF.  Both surface from Read, sticky.
var ErrMalformed = errors.New("zlib: malformed stream")

// Reader states: the 2-byte header, the delegated DEFLATE body, the
// 4-byte trailer, and the end-of-source check.
const (
	stZHeader = iota
	stZBody
	stZTrailer
	stZEof
)

// Reader decodes one zlib stream from src.
type Reader struct {
	in    seqReader // leftover bytes (from the DEFLATE reader) first, then src
	state int
	dr    *deflate.Reader
	s1    uint32 // running Adler-32
	s2    uint32
	err   error
}

// NewReader returns a Reader decoding the zlib stream in src.
func NewReader(src io.Reader) *Reader {
	r := &Reader{state: stZHeader, s1: 1}
	r.in.src = src
	return r
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for r.err == nil {
		switch r.state {
		case stZHeader:
			var head [2]byte
			if _, err := io.ReadFull(&r.in, head[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			cmf, flg := head[0], head[1]
			switch {
			case cmf&0x0F != 8:
				r.err = fmt.Errorf("%w: reserved compression method", ErrMalformed)
			case cmf>>4 > 7:
				r.err = fmt.Errorf("%w: window larger than 32K", ErrMalformed)
			case (int(cmf)*256+int(flg))%31 != 0:
				r.err = fmt.Errorf("%w: header check bits", ErrMalformed)
			case flg&0x20 != 0:
				r.err = fmt.Errorf("%w: FDICT preset dictionary", ErrMalformed)
			}
			if r.err != nil {
				continue
			}
			r.dr = deflate.NewReader(&r.in)
			r.state = stZBody
		case stZBody:
			n, err := r.dr.Read(p)
			if n > 0 {
				r.s1, r.s2 = adler32(r.s1, r.s2, p[:n])
				return n, nil
			}
			if err == io.EOF {
				r.in.reset(r.dr.Buffered())
				r.state = stZTrailer
				continue
			}
			if err != nil {
				r.err = err // wraps deflate.ErrMalformed or is ErrUnexpectedEOF
				continue
			}
		case stZTrailer:
			var tr [4]byte
			if _, err := io.ReadFull(&r.in, tr[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			want := r.s2<<16 | r.s1
			got := uint32(tr[0])<<24 | uint32(tr[1])<<16 | uint32(tr[2])<<8 | uint32(tr[3])
			if got != want {
				r.err = fmt.Errorf("%w: Adler-32 mismatch", ErrMalformed)
				continue
			}
			r.state = stZEof
		case stZEof:
			// Expand rejects trailing bytes after the stream; the same
			// check here must consult both the leftover stage and the
			// source before declaring the stream complete.
			var one [1]byte
			n, err := r.in.Read(one[:])
			if n > 0 {
				r.err = fmt.Errorf("%w: trailing bytes after stream", ErrMalformed)
				continue
			}
			if err == io.EOF {
				r.err = io.EOF
				continue
			}
			r.err = err
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

// seqReader serves bytes left over from the DEFLATE reader first,
// then reads the original source.  It never reads ahead.
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

// Writer incrementally encodes what is written to it as one zlib
// stream on dst: the canonical 0x78 0x9C header on the first write or
// at Close for an empty stream, a raw DEFLATE stream through a
// deflate.Writer, and at Close the big-endian Adler-32 trailer
// accumulated over everything written.  Errors are sticky.
type Writer struct {
	dst    io.Writer
	dw     *deflate.Writer
	s1     uint32 // running Adler-32
	s2     uint32
	hdr    bool
	closed bool
	err    error
}

// NewWriter returns a Writer encoding into dst.  Close must be called
// to finish the stream.
func NewWriter(dst io.Writer) *Writer {
	return &Writer{dst: dst, s1: 1}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("zlib: Write after Close")
		return 0, w.err
	}
	if err := w.start(); err != nil {
		return 0, w.err
	}
	if _, err := w.dw.Write(p); err != nil {
		w.err = err
		return 0, w.err
	}
	w.s1, w.s2 = adler32(w.s1, w.s2, p)
	return len(p), nil
}

// Flush ends the current DEFLATE block so a reader can decode
// everything written so far.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if w.dw == nil {
		return nil
	}
	return w.dw.Flush()
}

// Close writes the trailer and finishes the stream.  Calling it twice
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
	if err := w.dw.Close(); err != nil {
		w.err = err
		return w.err
	}
	v := w.s2<<16 | w.s1
	tr := [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	if _, err := w.dst.Write(tr[:]); err != nil {
		w.err = err
		return w.err
	}
	return nil
}

// start writes the 2-byte header once, lazily, so an unused Writer
// still produces a valid empty stream at Close.
func (w *Writer) start() error {
	if w.hdr {
		return nil
	}
	w.hdr = true
	cmf := byte(0x78)
	flg := byte(0x80)
	if rem := (int(cmf)*256 + int(flg)) % 31; rem != 0 {
		flg += byte(31 - rem)
	}
	if _, err := w.dst.Write([]byte{cmf, flg}); err != nil {
		w.err = err
		return err
	}
	w.dw = deflate.NewWriter(w.dst)
	return nil
}

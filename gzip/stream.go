/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package gzip

import (
	"errors"
	"fmt"
	"io"

	"github.com/pschlump/pluto/crc"
	"github.com/pschlump/pluto/deflate"
)

// The streaming API: the same gzip member format as Compress/Expand,
// driven through io.Reader and io.Writer.  The Reader decodes
// concatenated members exactly as Expand does; the Writer produces one
// member with the same minimal header.  Memory is bounded regardless
// of stream length.  The framing layers over pluto/deflate streaming:
// each member's DEFLATE body runs through a deflate.Reader over the
// remaining source, and bytes that reader staged past its final block
// come back (deflate.Reader.Buffered) to feed the trailer and the next
// member's header — nothing is ever read twice or lost.

// ErrMalformed marks a structurally invalid file: bad magic or method,
// reserved flags, a broken optional field, a bad FHCRC, or a wrong
// CRC-32 or ISIZE.  Truncation — anywhere a shortage of bytes is the
// problem, including trailing bytes that start but do not complete
// another member — is reported as io.ErrUnexpectedEOF so a caller can
// tell corruption from a short file.  Both surface from Read, sticky.
var ErrMalformed = errors.New("gzip: malformed stream")

// Reader states: between members the next bytes are a header (or the
// clean end of the file); a member body is delegated to a
// deflate.Reader; the trailer follows its final block.
const (
	stGzHeader = iota
	stGzBody
	stGzTrailer
)

// Reader decodes a gzip stream — one member or many concatenated —
// from src.
type Reader struct {
	in    seqReader // leftover bytes (from the previous member's reader) first, then src
	state int
	head  [10]byte // member header under construction
	opt   []byte   // header bytes accumulated for FHCRC
	crc   uint32   // running CRC-32 of the current member
	size  uint32   // running ISIZE (mod 2^32) of the current member
	dr    *deflate.Reader
	err   error
}

// NewReader returns a Reader decoding the gzip stream in src.  An
// empty src is a valid empty stream (Expand of nothing is nothing).
func NewReader(src io.Reader) *Reader {
	r := &Reader{state: stGzHeader}
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
		case stGzHeader:
			// Read the first header byte alone: a clean EOF there is the
			// end of the file (empty stream, or no more members), while
			// a header that starts but cannot finish is a shortage.
			if _, err := io.ReadFull(&r.in, r.head[:1]); err != nil {
				if err == io.EOF {
					r.err = io.EOF // empty stream, or no more members
				} else {
					r.err = err
				}
				continue
			}
			if _, err := io.ReadFull(&r.in, r.head[1:]); err != nil {
				r.err = headerErr(err)
				continue
			}
			if err := r.parseHeader(); err != nil {
				r.err = err
				continue
			}
			r.crc, r.size = 0, 0
			r.dr = deflate.NewReader(&r.in)
			r.state = stGzBody
		case stGzBody:
			n, err := r.dr.Read(p)
			if n > 0 {
				r.crc = crc.Update32(r.crc, crc.IEEETable, p[:n])
				r.size += uint32(n)
				return n, nil
			}
			if err == io.EOF {
				// Bytes the DEFLATE reader staged past its final block
				// are the trailer (and perhaps the next member).
				r.in.reset(r.dr.Buffered())
				r.state = stGzTrailer
				continue
			}
			if err != nil {
				r.err = err // wraps deflate.ErrMalformed or is ErrUnexpectedEOF
				continue
			}
		case stGzTrailer:
			var tr [8]byte
			if _, err := io.ReadFull(&r.in, tr[:]); err != nil {
				r.err = trailerErr(err)
				continue
			}
			if r.crc != le32at(tr[:], 0) {
				r.err = fmt.Errorf("%w: CRC-32 mismatch", ErrMalformed)
				continue
			}
			if r.size != le32at(tr[:], 4) {
				r.err = fmt.Errorf("%w: ISIZE mismatch", ErrMalformed)
				continue
			}
			r.state = stGzHeader
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

// parseHeader validates r.head and consumes the optional fields,
// returning an error wrapping ErrMalformed on any framing fault.
func (r *Reader) parseHeader() error {
	if r.head[0] != 0x1f || r.head[1] != 0x8b || r.head[2] != 0x08 {
		return fmt.Errorf("%w: bad magic or method", ErrMalformed)
	}
	flags := r.head[3]
	if flags&^(ftext|fhcrc|fextra|fname|fcomment) != 0 {
		return fmt.Errorf("%w: reserved flag bits set", ErrMalformed)
	}
	r.opt = append(r.opt[:0], r.head[:]...)
	if flags&fextra != 0 {
		var xl [2]byte
		if _, err := io.ReadFull(&r.in, xl[:]); err != nil {
			return headerErr(err)
		}
		r.opt = append(r.opt, xl[:]...)
		xlen := int(xl[0]) | int(xl[1])<<8
		extra := make([]byte, xlen)
		if _, err := io.ReadFull(&r.in, extra); err != nil {
			return headerErr(err)
		}
		r.opt = append(r.opt, extra...)
	}
	for _, f := range []byte{fname, fcomment} {
		if flags&f == 0 {
			continue
		}
		for {
			var one [1]byte
			if _, err := io.ReadFull(&r.in, one[:]); err != nil {
				return headerErr(err)
			}
			r.opt = append(r.opt, one[0])
			if one[0] == 0 {
				break
			}
		}
	}
	if flags&fhcrc != 0 {
		var hc [2]byte
		if _, err := io.ReadFull(&r.in, hc[:]); err != nil {
			return headerErr(err)
		}
		want := uint16(hc[0]) | uint16(hc[1])<<8
		if want != uint16(crc.Checksum32(r.opt, crc.IEEETable)&0xFFFF) {
			return fmt.Errorf("%w: FHCRC mismatch", ErrMalformed)
		}
	}
	return nil
}

func headerErr(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return io.ErrUnexpectedEOF
	}
	return fmt.Errorf("%w: %v", ErrMalformed, err)
}

func trailerErr(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// seqReader serves bytes left over from a previous member's DEFLATE
// reader first, then reads the original source.  It never reads
// ahead: pass-through both ways.
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

// reset installs leftover bytes to serve before the source continues.
func (s *seqReader) reset(leftover []byte) {
	s.buf = leftover
	s.pos = 0
}

// Writer incrementally encodes what is written to it as one gzip
// member on dst: the minimal header (as Compress writes) on the first
// write or at Close for an empty member, a raw DEFLATE stream through
// a deflate.Writer, and at Close the CRC-32 and ISIZE trailer
// accumulated over everything written.  Errors are sticky.
type Writer struct {
	dst    io.Writer
	dw     *deflate.Writer
	crc    uint32
	size   uint32
	hdr    bool
	closed bool
	err    error
}

// NewWriter returns a Writer encoding into dst.  Close must be called
// to finish the member.
func NewWriter(dst io.Writer) *Writer {
	return &Writer{dst: dst}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("gzip: Write after Close")
		return 0, w.err
	}
	if err := w.start(); err != nil {
		return 0, w.err
	}
	if _, err := w.dw.Write(p); err != nil {
		w.err = err
		return 0, w.err
	}
	w.crc = crc.Update32(w.crc, crc.IEEETable, p)
	w.size += uint32(len(p))
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

// Close writes the trailer and finishes the member.  Calling it twice
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
	var tr [8]byte
	tr[0], tr[1], tr[2], tr[3] = byte(w.crc), byte(w.crc>>8), byte(w.crc>>16), byte(w.crc>>24)
	tr[4], tr[5], tr[6], tr[7] = byte(w.size), byte(w.size>>8), byte(w.size>>16), byte(w.size>>24)
	if _, err := w.dst.Write(tr[:]); err != nil {
		w.err = err
		return w.err
	}
	return nil
}

// start writes the member header once, lazily, so an unused Writer
// still produces a valid empty member at Close.
func (w *Writer) start() error {
	if w.hdr {
		return nil
	}
	w.hdr = true
	if _, err := w.dst.Write([]byte{
		0x1f, 0x8b, // ID1 ID2
		0x08,       // CM: deflate
		0x00,       // FLG: no optional fields
		0, 0, 0, 0, // MTIME: 0
		0x00, // XFL
		0xff, // OS: unknown
	}); err != nil {
		w.err = err
		return err
	}
	w.dw = deflate.NewWriter(w.dst)
	return nil
}

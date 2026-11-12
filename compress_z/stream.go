/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z

import (
	"errors"
	"fmt"
	"io"
)

// The streaming API: the same .Z format as Compress/Expand, driven
// through io.Reader and io.Writer.  Memory is bounded regardless of
// stream length (the decoder's dictionary is bounded by maxbits, the
// encoder's by its hash map of at most 2^maxbits entries; the input
// stage is a bounded refill buffer).  The decoder is the shared
// decoder of expand.go driven one code at a time over the refilling
// bitReader; LZW output is never re-referenced later, so decoded
// bytes are delivered and forgotten — no history window.

// ErrMalformed marks a structurally invalid stream: a bad header, a
// code beyond the live table, a non-literal first code, or a cyclic
// chain.  Because the format has no end-of-stream framing (the stream
// simply stops), there is no truncation error: input ending anywhere
// is a clean end, exactly as Expand reads a prefix.
var ErrMalformed = errors.New("compress_z: malformed stream")

// Reader states: the 3-byte header, then the code stream.
const (
	stZHeader = iota
	stZCodes
)

// Reader decodes one .Z stream from src.
type Reader struct {
	src   io.Reader
	d     *decoder
	state int
	head  [3]byte
	err   error
}

// NewReader returns a Reader decoding the .Z stream in src.
func NewReader(src io.Reader) *Reader {
	return &Reader{src: src, state: stZHeader}
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if r.d != nil && len(r.d.out) > 0 {
			c := copy(p, r.d.out)
			r.d.out = r.d.out[c:]
			return c, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		switch r.state {
		case stZHeader:
			if _, err := io.ReadFull(r.src, r.head[:]); err != nil {
				r.err = frameErr(err)
				continue
			}
			if r.head[0] != magic0 || r.head[1] != magic1 {
				r.err = fmt.Errorf("%w: bad header magic", ErrMalformed)
				continue
			}
			maxBits := int(r.head[2] & bitMask)
			block := r.head[2]&blockMode != 0
			if maxBits < 9 || maxBits > 16 {
				r.err = fmt.Errorf("%w: maxbits %d out of range", ErrMalformed, maxBits)
				continue
			}
			r.d = newDecoder(block, maxBits)
			r.d.r.src = r.src
			r.state = stZCodes
		case stZCodes:
			switch r.d.step() {
			case stData, stClear:
				// output (if any) is delivered at the top of the loop
			case stEnd:
				if r.d.r.fillErr != nil {
					r.err = r.d.r.fillErr
				} else {
					r.err = io.EOF // the stream simply stops
				}
			case stBad:
				r.err = fmt.Errorf("%w: bad code", ErrMalformed)
			}
		}
	}
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

// Writer incrementally encodes what is written to it as one .Z stream
// on dst: the 3-byte header on the first write (or at Close for an
// empty stream), then the same greedy LZW as CompressBits — the same
// width-growth order and the same never-emit-CLEAR policy — with
// completed output bytes drained to dst as they form.  The greedy
// matcher keeps one input byte of state (the pending chain start)
// across Write calls; compression equals the whole-buffer result
// regardless of chunking.  Errors are sticky.
type Writer struct {
	dst     io.Writer
	bw      bitWriter
	maxBits int
	maxmax  uint32
	free    uint32
	nBits   uint
	maxcode uint32
	tab     map[uint64]uint32
	ent     uint32
	haveEnt bool
	hdr     bool
	closed  bool
	err     error
}

// NewWriter returns a Writer encoding into dst with maxBits 16 (the
// compress(1) default).  Close must be called to finish the stream.
func NewWriter(dst io.Writer) *Writer {
	return NewWriterBits(dst, 16)
}

// NewWriterBits returns a Writer encoding into dst with the given
// maximum code width (9..16, the compress -b range).  It panics on an
// out-of-range width, as CompressBits does.
func NewWriterBits(dst io.Writer, maxBits int) *Writer {
	if maxBits < 9 || maxBits > 16 {
		panic(fmt.Sprintf("compress_z.NewWriterBits: maxBits %d out of range, want 9..16", maxBits))
	}
	return &Writer{
		dst:     dst,
		maxBits: maxBits,
		maxmax:  uint32(1) << maxBits,
		nBits:   initBits,
		maxcode: 1<<initBits - 1,
		free:    firstEntry,
		tab:     make(map[uint64]uint32, 4096),
	}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("compress_z: Write after Close")
		return 0, w.err
	}
	if err := w.start(); err != nil {
		return 0, w.err
	}
	for _, c := range p {
		if !w.haveEnt {
			w.ent = uint32(c)
			w.haveEnt = true
			continue
		}
		key := uint64(w.ent)<<8 | uint64(c)
		if code, ok := w.tab[key]; ok {
			w.ent = code
			continue
		}
		w.bw.writeCode(w.ent, w.nBits)
		// Width change after the write and before the add — the
		// reference encoder's order (see CompressBits).
		if w.free > w.maxcode {
			w.bw.realign(w.nBits)
			w.nBits++
			if w.nBits == uint(w.maxBits) {
				w.maxcode = w.maxmax
			} else {
				w.maxcode = 1<<w.nBits - 1
			}
		}
		if w.free < w.maxmax {
			w.tab[key] = w.free
			w.free++
		}
		w.ent = uint32(c)
	}
	w.drain(nil)
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

// Flush moves completed output bytes to dst.  .Z has no mid-stream
// alignment point (codes pack continuously and there is no block or
// member boundary), so this cannot make the tail decodable — it only
// pushes out the whole bytes that already exist.  A final partial
// byte is zero-padded and emitted by Close alone.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if w.hdr && !w.closed {
		w.drain(nil)
	}
	return w.err
}

// Close writes the pending chain-start code, pads the final partial
// byte, and finishes the stream.  Calling it twice reports the sticky
// error.
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
	if w.haveEnt {
		w.bw.writeCode(w.ent, w.nBits)
	}
	w.drain(w.bw.bytes())
	return w.err
}

// start writes the 3-byte header once, lazily, so an unused Writer
// still produces a valid empty stream at Close (header only, as
// Compress(nil)).
func (w *Writer) start() error {
	if w.hdr {
		return nil
	}
	w.hdr = true
	w.bw.buf = append(w.bw.buf[:0], magic0, magic1, byte(w.maxBits)|blockMode)
	w.bw.base = w.bw.totalBits()
	w.drain(nil)
	if w.err != nil {
		return w.err
	}
	return nil
}

// drain moves completed output bytes to dst, recording them in
// bw.drained so the group-alignment math stays absolute.  Only whole
// bytes go out (padded is nil); Close passes bw.bytes() to emit the
// zero-padded final partial byte.
func (w *Writer) drain(padded []byte) {
	out := w.bw.buf
	if len(padded) > 0 {
		out = padded
	}
	if len(out) == 0 {
		return
	}
	if _, err := w.dst.Write(out); err != nil && w.err == nil {
		w.err = err
	}
	w.bw.drained += len(out)
	w.bw.buf = w.bw.buf[:0]
}

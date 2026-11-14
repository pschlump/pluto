/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lzma

import (
	"errors"
	"io"

	"github.com/pschlump/pluto/internal/lzmacore"
)

// The streaming API.  The Reader is fully streaming: the .lzma format
// is one continuous range-coded stream with no block structure, so
// the decoder pulls its input incrementally and decodes in slices
// (memory is the delivery buffer, never the whole input).  The Writer
// accumulates and encodes once at Close — an incremental LZMA writer
// needs a persistent match finder, which the whole-buffer encoder
// does not carry; the asymmetry is deliberate and documented.

// ErrMalformed marks a structurally invalid stream: a short header, a
// bad properties byte, corrupt LZMA data, or a wrong known size.
// Truncation is io.ErrUnexpectedEOF.  Both surface from Read, sticky.
var ErrMalformed = errors.New("lzma: malformed stream")

// readWindow is how much the streaming Reader decodes per fill;
// historyWindow is how much decoded history it retains behind the
// delivery point (distances may reach back into it).
const (
	readWindow    = 256 * 1024
	historyWindow = 8 << 20
)

// Reader decodes one .lzma stream from src.
type Reader struct {
	src  io.Reader
	rc   *lzmacore.RangeDecoder
	st   *lzmacore.LzmaState
	err  error
	out  []byte // decoded history + pending delivery
	roff int    // delivery offset into out
	size int64  // known uncompressed size, or -1
	done bool   // the end marker was seen (bytes may still be pending)
}

// NewReader starts decoding src.
func NewReader(src io.Reader) *Reader {
	return &Reader{src: src}
}

// Read delivers decoded bytes.
func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.st == nil && !r.start() {
		return 0, r.err
	}
	// Decode until there is something deliverable past the history
	// reserve (or the stream ends): pending must exceed the window.
	for len(r.out)-r.roff <= historyWindow && !r.done && r.err == nil {
		target := len(r.out) + readWindow
		if r.size >= 0 {
			if left := int(r.size) - len(r.out); left < readWindow {
				target = len(r.out) + left
			}
			if len(r.out) >= int(r.size) {
				r.done = true // size-terminated: no marker follows
				break
			}
		}
		done, ok := lzmacore.LzmaDecodeSome(r.rc, r.st, &r.out, target)
		if !ok {
			r.err = ErrMalformed
			break
		}
		if done {
			r.done = true
			if r.size >= 0 && int64(len(r.out)) != r.size {
				r.err = ErrMalformed // the marker disagrees with the header
			}
		}
	}
	// Deliver from the front, but keep a history reserve behind the
	// delivery point — matches in the next fill may reach back into
	// it.  Once the stream is done there is no next fill and
	// everything drains.
	avail := len(r.out) - r.roff
	if !r.done {
		if avail > historyWindow {
			avail -= historyWindow
		} else {
			avail = 0 // hold the history reserve back
		}
	}
	if avail > len(p) {
		avail = len(p)
	}
	if avail > 0 {
		n := copy(p, r.out[r.roff:r.roff+avail])
		r.roff += n
		if r.roff > historyWindow {
			// compact: the tail becomes the buffer
			r.out = r.out[r.roff-historyWindow:]
			r.roff = historyWindow
		}
		return n, nil
	}
	if r.done {
		r.err = io.EOF
		return 0, r.err
	}
	if r.err == nil {
		r.err = io.ErrUnexpectedEOF
	}
	return 0, r.err
}

// Close releases the reader.
func (r *Reader) Close() error { return nil }

func (r *Reader) start() bool {
	var hdr [13]byte
	if _, err := io.ReadFull(io.LimitReader(r.src, 13), hdr[:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		r.err = err
		return false
	}
	if hdr[0] > 224 {
		r.err = ErrMalformed
		return false
	}
	size := int64(uint64(hdr[5]) | uint64(hdr[6])<<8 | uint64(hdr[7])<<16 | uint64(hdr[8])<<24 |
		uint64(hdr[9])<<32 | uint64(hdr[10])<<40 | uint64(hdr[11])<<48 | uint64(hdr[12])<<56)
	if size == -1 {
		r.size = -1
	} else if size > maxOutput {
		r.err = ErrMalformed
		return false
	} else {
		r.size = size
	}
	r.st = lzmacore.NewLzmaState(hdr[0])
	r.rc = lzmacore.NewRangeDecoderReader(r.src)
	return true
}

// Writer buffers input and writes the .lzma stream at Close.  It
// never holds the decoded form — only the pending input — and Flush
// is a no-op (the format has no flush point short of the end).
type Writer struct {
	dst   io.Writer
	pend  []byte
	err   error
	start bool
}

// NewWriter starts a stream on dst.
func NewWriter(dst io.Writer) *Writer { return &Writer{dst: dst} }

// Write buffers p; the encoding happens once, at Close.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	w.pend = append(w.pend, p...)
	return len(p), nil
}

// Flush is a no-op: the .lzma format has a single stream with no
// intermediate boundary to flush to.
func (w *Writer) Flush() error { return w.err }

// Close writes the header, the LZMA stream, and the end marker.  It
// does not close the underlying writer.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.start {
		return nil
	}
	w.start = true
	out := make([]byte, 0, len(w.pend)/2+64)
	out = append(out, lzmacore.DefaultProps)
	var hdr [12]byte
	d := uint32(dictSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = byte(d), byte(d>>8), byte(d>>16), byte(d>>24)
	for i := 4; i < 12; i++ {
		hdr[i] = 0xFF // size unknown: end-marker terminated
	}
	out = append(out, hdr[:]...)
	e, st := lzmacore.LzmaEncodeNoFlush(nil, nil, w.pend, nil, 0)
	lzmacore.EncodeEndMarker(e, st, len(w.pend)&(1<<uint(st.Pb)-1))
	e.Flush()
	out = append(out, e.Out...)
	if _, err := w.dst.Write(out); err != nil {
		w.err = err
	}
	return w.err
}

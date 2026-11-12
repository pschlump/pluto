/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate

import (
	"errors"
	"fmt"
	"io"
)

// The streaming API: the same DEFLATE codec as Compress/Expand, driven
// through io.Reader and io.Writer so callers with pipes, sockets, or
// files larger than memory can use it.  Memory is bounded regardless
// of stream length: the Reader keeps a 32 KiB decode history plus a
// bounded input stage, the Writer a 64 KiB match window plus bounded
// output.  The whole-buffer functions remain the simple API; these
// types are the plumbing for gzip/zlib framing (stream.go in those
// packages) and for archive tools that stream.
//
// Like everything else in the package the types are stateful but not
// shared: use one Reader or Writer per goroutine, no _ts twin (the
// lzw precedent).

// ErrMalformed marks a structurally invalid stream; truncated input is
// reported as io.ErrUnexpectedEOF so a caller can tell corruption from
// a short read.  Both surface from Read.
var ErrMalformed = errors.New("deflate: malformed stream")

// Reader states: between blocks the next three bits are BFINAL/BTYPE;
// a stored block stages its LEN/NLEN then copies raw bytes; a
// huffman block runs expandBlock until its end-of-block symbol.
const (
	stHeader = iota
	stStoredLen
	stStoredCopy
	stBlock
)

const (
	histKeep = windowSize // delivered history retained for match distances
)

// Reader decodes one raw DEFLATE stream from src.  Reads return the
// decompressed bytes; io.EOF marks the end of the final block.  A
// malformed stream yields an error wrapping ErrMalformed, a truncated
// one io.ErrUnexpectedEOF; both are sticky.  Read never panics.
type Reader struct {
	br bitReader // streaming mode: refills its stage from src as bits are consumed

	out       []byte // decoded bytes: out[:delivered] history, out[delivered:] pending
	delivered int
	state     int
	final     bool
	lit       *decTree
	dist      *decTree
	stored    int // stored-block bytes still to copy

	err  error
	done bool
}

// NewReader returns a Reader decoding the raw DEFLATE stream in src.
// src is read only as far as decoding demands; after the final block
// no further read happens, and whatever was staged but not consumed is
// available through Buffered.
func NewReader(src io.Reader) *Reader {
	return &Reader{br: bitReader{src: src}, state: stHeader}
}

// Read implements io.Reader.  Buffered bytes are delivered before a
// sticky error, per io convention.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.out)-r.delivered == 0 {
		if r.err != nil {
			return 0, r.err
		}
		if r.done {
			r.err = io.EOF
			return 0, io.EOF
		}
		r.step(len(p))
	}
	n := copy(p, r.out[r.delivered:])
	r.delivered += n
	r.trim()
	return n, nil
}

// Close releases nothing (there is nothing to release) and reports the
// sticky error, if any.  It is provided for io.ReadCloser symmetry.
func (r *Reader) Close() error { return r.err }

// Buffered returns source bytes that were staged but lie past the end
// of the DEFLATE stream.  Framing layers (gzip, zlib) take their
// trailers from here — in order — before reading the original source
// again.  The result aliases internal state; use it before the next
// Read (a finished stream has none anyway).
func (r *Reader) Buffered() []byte { return r.br.buf[r.br.pos:] }

// step advances decoding until out holds at least one byte (or the
// stream ends or fails).
func (r *Reader) step(target int) {
	switch r.state {
	case stHeader:
		b, ok := r.br.readBit()
		if !ok {
			r.truncated()
			return
		}
		r.final = b == 1
		t, ok := r.br.readBits(2)
		if !ok {
			r.truncated()
			return
		}
		switch t {
		case 0:
			r.br.align()
			r.state = stStoredLen
		case 1:
			r.lit, r.dist = fixedLitTree, fixedDistTree
			r.state = stBlock
		case 2:
			lit, dist, ok := readDynamicHeader(&r.br)
			if !ok {
				r.starved(ErrMalformed)
				return
			}
			r.lit, r.dist = lit, dist
			r.state = stBlock
		default:
			r.fail(fmt.Errorf("%w: reserved block type 11", ErrMalformed))
		}
	case stStoredLen:
		// align() left acc empty, so each 16-bit read consumes exactly
		// two staged bytes and pos lands on the stored data.
		length, ok := r.br.readBits(16)
		if !ok {
			r.truncated()
			return
		}
		nlen, ok := r.br.readBits(16)
		if !ok {
			r.truncated()
			return
		}
		if length != ^nlen&0xFFFF {
			r.fail(fmt.Errorf("%w: stored block LEN/NLEN mismatch", ErrMalformed))
			return
		}
		r.stored = int(length)
		r.state = stStoredCopy
	case stStoredCopy:
		if r.br.pos >= len(r.br.buf) && !r.br.fill() {
			if r.stored > 0 {
				r.truncated()
			} else {
				r.state = stHeader
			}
			return
		}
		c := min(r.stored, len(r.br.buf)-r.br.pos)
		r.out = append(r.out, r.br.buf[r.br.pos:r.br.pos+c]...)
		r.br.pos += c
		r.stored -= c
		if r.stored == 0 {
			r.state = stHeader
			if r.final {
				r.done = true
			}
		}
	case stBlock:
		// The output limit is in pending bytes (the undelivered tail);
		// expandBlock counts the whole buffer, history included.
		limit := r.delivered + max(len(r.out)-r.delivered, target)
		hitEnd, ok := expandBlock(&r.br, &r.out, r.lit, r.dist, limit)
		if !ok {
			r.starved(fmt.Errorf("%w: bad huffman block", ErrMalformed))
			return
		}
		if hitEnd {
			r.state = stHeader
			if r.final {
				r.done = true
			}
		}
	}
}

// trim slides the output buffer down so the retained history — bytes
// already delivered that a later match distance can still reach —
// never exceeds the 32 KiB window.  Matches copy out of that history
// (expandBlock indexes the whole buffer), so it must cover every
// legal distance, or the entire output while the stream is shorter
// than that; delivered bytes are never re-delivered.
func (r *Reader) trim() {
	if r.delivered > histKeep {
		drop := r.delivered - histKeep
		n := copy(r.out, r.out[drop:])
		r.out = r.out[:n]
		r.delivered = histKeep
	}
}

// starved classifies a decode failure: if a bit read actually failed
// for want of input it is truncation, otherwise the stream itself is
// malformed.
func (r *Reader) starved(malformed error) {
	if r.br.readFailed {
		r.truncated()
		return
	}
	r.fail(malformed)
}

func (r *Reader) truncated() {
	if r.br.fillErr != nil {
		r.err = r.br.fillErr
		return
	}
	r.err = io.ErrUnexpectedEOF
}

func (r *Reader) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Writer incrementally encodes what is written to it as one raw
// DEFLATE stream on dst.  The LZ77 matcher is the same hash-chain
// search with lazy evaluation as Compress, run over a sliding 64 KiB
// window: positions are absolute int64 chain entries, and matches are
// clamped to data still in the window, so compression equals the
// whole-buffer result except within a window of a Write-chunk
// boundary.  Blocks flush at the same 16384-symbol threshold; Flush
// and Close end a block early (Close writes the final block and pads
// to a byte boundary).  Neither method panics; a write error is
// sticky and reported by every later call.
type Writer struct {
	dst    io.Writer
	enc    encoder // tokens + bit output (head/prev unused: the window has its own chains)
	head   []int64 // hash -> most recent absolute position
	prev   []int64 // absolute position&windowMask -> previous position
	win    []byte  // uncompressed window, win[0] is absolute base
	base   int64   // absolute position of win[0]
	tok    int64   // absolute tokenize cursor
	link   int64   // absolute insert cursor (positions < link are chained)
	start  int64   // absolute block start
	err    error
	closed bool
}

// NewWriter returns a Writer encoding into dst.  Close must be called
// to finish the stream.
func NewWriter(dst io.Writer) *Writer {
	w := &Writer{
		dst:  dst,
		head: make([]int64, hashSize),
		prev: make([]int64, windowSize),
	}
	for i := range w.head {
		w.head[i] = -1
	}
	for i := range w.prev {
		w.prev[i] = -1
	}
	return w
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.closed {
		w.err = errors.New("deflate: Write after Close")
		return 0, w.err
	}
	w.win = append(w.win, p...)
	w.pump()
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

// Flush ends the current block (non-final) so a reader can decode
// everything written so far.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	end := w.base + int64(len(w.win))
	w.tokenize(end)
	w.flushTo(end, false)
	return w.err
}

// Close writes the final block and finishes the stream.  Calling it
// twice reports the sticky error.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	w.closed = true
	end := w.base + int64(len(w.win))
	w.tokenize(end)
	w.flushTo(end, true)
	w.enc.w.alignBytes()
	w.drain()
	return w.err
}

// pump tokenizes and slides while the window holds a full extra
// window's worth of data, leaving at most 64 KiB buffered.
func (w *Writer) pump() {
	for w.err == nil && len(w.win) >= 2*windowSize {
		mid := w.base + windowSize
		w.tokenize(mid)
		w.flushTo(mid, false)
		if w.err != nil {
			return
		}
		n := copy(w.win, w.win[windowSize:])
		w.win = w.win[:n]
		w.base += windowSize
	}
}

// tokenize runs the lazy-match LZ77 loop from the cursor up to upto,
// flushing a block whenever the token threshold is reached.
func (w *Writer) tokenize(upto int64) {
	for w.err == nil && w.tok < upto {
		w.linkUpto(w.tok + 1)
		length, dist := w.findMatch(w.tok)
		if length >= minMatch && length < maxMatch && w.tok+1 < w.base+int64(len(w.win)) {
			// Lazy evaluation, as in compress: prefer a longer match at
			// the next position.
			w.linkUpto(w.tok + 2)
			if l2, d2 := w.findMatch(w.tok + 1); l2 > length {
				w.enc.emit(token{lit: uint16(w.at(w.tok))})
				w.tok++
				length, dist = l2, d2
			}
		}
		if length >= minMatch {
			w.enc.emit(token{length: uint16(length), dist: uint16(dist)})
			w.linkUpto(w.tok + int64(length))
			w.tok += int64(length)
		} else {
			w.enc.emit(token{lit: uint16(w.at(w.tok))})
			w.tok++
		}
		if len(w.enc.tokens) >= blockTokens {
			w.flushTo(w.tok, false)
		}
	}
	w.linkUpto(upto)
}

// at returns the window byte at absolute position p (p must be inside
// the window).
func (w *Writer) at(p int64) byte { return w.win[p-w.base] }

// linkUpto chains every unlinked position below upto.  Positions too
// close to the window's end to hash three bytes are deferred — the
// clamp keeps the cursor behind them so later data can hash them.
func (w *Writer) linkUpto(upto int64) {
	if max := w.base + int64(len(w.win)) - minMatch + 1; upto > max {
		upto = max
	}
	for ; w.link < upto; w.link++ {
		h := hash3(w.win[w.link-w.base:])
		w.prev[w.link&windowMask] = w.head[h]
		w.head[h] = w.link
	}
}

// findMatch walks the hash chain of the position i (absolute),
// returning the longest match whose distance stays inside the window.
func (w *Writer) findMatch(i int64) (int, int) {
	if i+minMatch > w.base+int64(len(w.win)) {
		return 0, 0
	}
	limit := i - windowSize
	if limit < w.base {
		limit = w.base // data below the window base has been slid away
	}
	rel := i - w.base
	j := w.head[hash3(w.win[rel:])]
	for j >= i {
		j = w.prev[j&windowMask]
	}
	best, bestDist := 0, 0
	for chain := 0; j >= limit && chain < maxChain; chain++ {
		if best == 0 || (i+int64(best) < w.base+int64(len(w.win)) && w.at(j+int64(best)) == w.at(i+int64(best))) {
			if l := matchLen(w.win, int(j-w.base), int(rel), maxMatch); l > best {
				best, bestDist = l, int(i-j)
				if best == maxMatch {
					break
				}
			}
		}
		j = w.prev[j&windowMask]
	}
	if best < minMatch {
		return 0, 0
	}
	return best, bestDist
}

// flushTo writes the tokens covering [start, upto) as one block,
// reusing the whole-buffer block emitter.  A final flush with no
// pending bytes still emits the final (possibly empty) block.
func (w *Writer) flushTo(upto int64, final bool) {
	if w.start < upto {
		block := w.win[w.start-w.base : upto-w.base]
		w.enc.flushBlock(block, final)
		w.start = upto
	} else if final {
		w.enc.flushBlock(nil, true)
	}
	w.drain()
}

// drain moves completed output bytes to dst.  Only full bytes go out;
// a partial byte stays in the accumulator so consecutive blocks share
// byte boundaries exactly as the whole-buffer encoder lays them out.
func (w *Writer) drain() {
	if len(w.enc.w.buf) == 0 {
		return
	}
	if _, err := w.dst.Write(w.enc.w.buf); err != nil && w.err == nil {
		w.err = err
	}
	w.enc.w.buf = w.enc.w.buf[:0]
}

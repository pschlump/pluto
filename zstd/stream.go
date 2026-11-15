/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import (
	"errors"
	"io"
)

// The streaming forms.  Reader decodes incrementally block by block
// (the format's blocks are the natural streaming unit — at most
// 128 KiB regenerated each), holding the current frame's undelivered
// bytes as match history and trimming behind the window.  Writer
// emits the stored form of Compress: raw blocks under a 128 KiB
// window.

type Reader struct {
	src   io.Reader
	in    []byte // buffered input, consumed from the front
	eof   bool
	err   error
	state int // 0 between frames, 1 inside a frame

	d        decoder // holds the frame's decoded-but-undelivered bytes
	window   int64
	blockMax int
	check    bool
	xx       *xxh64
}

// NewReader wraps r; Read decompresses on demand.
func NewReader(r io.Reader) *Reader {
	return &Reader{src: r, d: decoder{rep: [3]uint64{1, 4, 8}}}
}

func (r *Reader) fill(n int) bool {
	for len(r.in) < n && !r.eof {
		var tmp [8192]byte
		k, err := r.src.Read(tmp[:])
		r.in = append(r.in, tmp[:k]...)
		if err != nil {
			r.eof = true
			if !errors.Is(err, io.EOF) {
				r.err = err
			}
		}
	}
	return len(r.in) >= n
}

// deliverable is how many buffered bytes may leave: inside a frame
// the last window bytes stay back as match history.
func (r *Reader) deliverable() int {
	if r.state == 1 {
		if over := int64(len(r.d.out)) - r.window; over > 0 {
			return int(over)
		}
		return 0
	}
	return len(r.d.out)
}

// Read delivers decoded bytes, reading and decoding whole blocks as
// they become available.  The error is sticky.
func (r *Reader) Read(p []byte) (int, error) {
	for r.deliverable() == 0 {
		if r.err != nil {
			return 0, r.err
		}
		if r.state == 0 {
			if len(r.in) == 0 && r.eof {
				r.err = io.EOF
				continue
			}
			r.stepFrame()
			continue
		}
		r.stepBlock()
	}
	n := copy(p, r.d.out[:r.deliverable()])
	r.d.out = r.d.out[n:]
	return n, nil
}

// stepFrame starts a new frame or skips a skippable one.  Errors are
// sticky in r.err.  A clean exhaustion of the input at a frame
// boundary is not an error: Read reports io.EOF for it.
func (r *Reader) stepFrame() {
	if !r.fill(4) {
		if r.err == nil && len(r.in) == 0 && r.eof {
			return
		}
		if r.err == nil {
			r.err = errCorrupt
		}
		return
	}
	switch m := le32x(r.in[:4]); {
	case m == frameMagic:
		// The largest header is 18 bytes (magic, descriptor, window,
		// dictionary id, 8-byte content size); fill waits for the
		// bytes or end of input, whichever comes first.
		r.fill(18)
		fi, hlen, err := parseFrameHeader(r.in)
		if err != nil {
			r.err = err
			return
		}
		r.in = r.in[hlen:]
		r.d.out = nil
		r.d.huffOK, r.d.llOK, r.d.ofOK, r.d.mlOK = false, false, false, false
		r.d.rep = [3]uint64{1, 4, 8}
		r.window = fi.window
		r.blockMax = int(fi.blockMax)
		r.check = fi.checksum
		r.xx = nil
		if r.check {
			r.xx = newXXH64()
		}
		r.state = 1
	case m&0xFFFFFFF0 == skipMagicLo:
		if !r.fill(8) {
			r.err = errCorrupt
			return
		}
		sz := int(le32x(r.in[4:8]))
		if !r.fill(8 + sz) {
			r.err = errCorrupt
			return
		}
		r.in = r.in[8+sz:]
	default:
		r.err = errCorrupt
	}
}

// stepBlock decodes one block of the current frame.
func (r *Reader) stepBlock() {
	if !r.fill(3) {
		if r.err == nil {
			r.err = errCorrupt
		}
		return
	}
	bh := le24x(r.in[:3])
	r.in = r.in[3:]
	last := bh&1 != 0
	typ := (bh >> 1) & 3
	size := int(bh >> 3)
	content := size
	if typ == 1 {
		content = 1
	}
	if typ > 2 || int64(size) > int64(r.blockMax) {
		r.err = errCorrupt
		return
	}
	if !r.fill(content) {
		r.err = errCorrupt
		return
	}
	body := r.in[:content]
	start := len(r.d.out)
	switch typ {
	case 0:
		r.d.out = append(r.d.out, body...)
	case 1:
		for i := 0; i < size; i++ {
			r.d.out = append(r.d.out, body[0])
		}
	case 2:
		if err := r.d.block(body, r.blockMax); err != nil {
			r.err = err
			return
		}
		if len(r.d.out)-start > r.blockMax {
			r.err = errCorrupt
			return
		}
	}
	r.in = r.in[content:]
	if r.xx != nil {
		r.xx.write(r.d.out[start:])
	}
	if last {
		if r.check {
			if !r.fill(4) {
				r.err = errCorrupt
				return
			}
			if le32x(r.in[:4]) != uint32(r.xx.sum()) {
				r.err = errCorrupt
				return
			}
			r.in = r.in[4:]
		}
		r.state = 0
	}
}

func le32x(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func le24x(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

// Writer buffers writes and flushes them as raw blocks of a stored
// frame; Close writes the final block.  Closing twice is a no-op.
type Writer struct {
	w    io.Writer
	buf  []byte
	hdr  bool
	done bool
	err  error
}

// NewWriter wraps w; nothing is written until a block fills or Close.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.done {
		return 0, errors.New("zstd: write after close")
	}
	if !w.hdr {
		if _, err := w.w.Write([]byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x38}); err != nil {
			w.err = err
			return 0, err
		}
		w.hdr = true
	}
	n := len(p)
	w.buf = append(w.buf, p...)
	for len(w.buf) >= maxBlockSize {
		if err := w.rawBlock(w.buf[:maxBlockSize], false); err != nil {
			w.err = err
			return 0, err
		}
		w.buf = w.buf[maxBlockSize:]
	}
	return n, nil
}

func (w *Writer) rawBlock(b []byte, last bool) error {
	h := uint32(len(b)) << 3
	if last {
		h |= 1
	}
	if _, err := w.w.Write([]byte{byte(h), byte(h >> 8), byte(h >> 16)}); err != nil {
		return err
	}
	_, err := w.w.Write(b)
	return err
}

// Close flushes the final block (an empty one for empty input).
func (w *Writer) Close() error {
	if w.done {
		return w.err
	}
	w.done = true
	if w.err != nil {
		return w.err
	}
	if !w.hdr {
		if _, err := w.w.Write([]byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x38}); err != nil {
			w.err = err
			return err
		}
		w.hdr = true
	}
	if err := w.rawBlock(w.buf, true); err != nil {
		w.err = err
		return err
	}
	w.buf = nil
	return nil
}

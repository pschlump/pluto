/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package zstd implements the Zstandard format (RFC 8878) with no
// dependencies beyond the standard library.  Expand reads everything
// the reference zstd(1) writes — every preset and level, Huffman
// literals (1 or 4 streams, direct or FSE-coded weights, treeless
// repeats), FSE or predefined or RLE sequence tables, raw/RLE/compressed
// blocks, window sizes to 128 MiB, content checksums, concatenated
// frames, and skippable frames.  Compress writes the legal minimal
// frame: raw blocks, no checksum — stored, not compressed (see the
// README for the honest consequence).  nil from Expand means
// malformed.
package zstd

import "encoding/binary"

const (
	frameMagic   = 0xFD2FB528
	skipMagicLo  = 0x184D2A50 // ..0x184D2A5F identify skippable frames
	maxWindow    = 1 << 27    // 128 MiB, beyond any preset's reach
	maxBlockSize = 128 << 10
	maxOutput    = 1 << 30 // bounds expansion bombs, like the other codecs
)

// decoder holds one frame's mutable state: the output built so far
// (which is also the match history), the Huffman table and sequence
// tables that persist between blocks for Treeless/Repeat modes, and
// the repeat offsets that carry across compressed blocks.
type decoder struct {
	out          []byte
	huff         *huffTable
	huffOK       bool
	fseLL, fseOF fseTable
	llOK, ofOK   bool
	fseML        fseTable
	mlOK         bool
	rep          [3]uint64
}

// frameWidth is the window the compressor declares: 8 MiB lets
// matches reach well back while keeping the streaming reader's hold
// bounded.
const frameWidth = 1 << 23

// Compress writes data as one zstd frame: magic, an 8 MiB-window
// header, then 128 KiB blocks — compressed (greedy matches through
// the predefined FSE tables, raw or RLE literals) when that shrinks
// the block, stored verbatim otherwise.
func Compress(data []byte) []byte {
	out := make([]byte, 0, len(data)/2+64)
	out = binary.LittleEndian.AppendUint32(out, frameMagic)
	out = append(out, 0x00)        // frame header descriptor: no FCS, no checksum
	out = append(out, byte(13)<<3) // window: 8 MiB, no mantissa

	// the hash table seeds as each block's scan passes a position,
	// so candidates are always history (positions the decoder has
	// already produced)
	f := newMatchFinder()
	for off := 0; ; off += maxBlockSize {
		n := len(data) - off
		if n > maxBlockSize {
			n = maxBlockSize
		}
		last := uint32(0)
		if off+n >= len(data) {
			last = 1
		}
		var h uint32
		if body := compressBlockBody(f, data, off, off+n, frameWidth); body != nil {
			h = last | 2<<1 | uint32(len(body))<<3
			out = append(out, byte(h), byte(h>>8), byte(h>>16))
			out = append(out, body...)
		} else {
			h = last | uint32(n)<<3
			out = append(out, byte(h), byte(h>>8), byte(h>>16))
			out = append(out, data[off:off+n]...)
		}
		if last == 1 {
			return out
		}
	}
}

// Expand decodes a zstd stream — one frame, many concatenated frames,
// or frames mixed with skippable ones — and returns the joined
// content, or nil when malformed.
func Expand(z []byte) []byte {
	var out []byte
	pos := 0
	for pos < len(z) {
		if len(z)-pos < 4 {
			return nil
		}
		m := binary.LittleEndian.Uint32(z[pos:])
		switch {
		case m == frameMagic:
			d := &decoder{rep: [3]uint64{1, 4, 8}}
			n, err := d.frame(z, pos)
			if err != nil {
				return nil
			}
			if len(out)+len(d.out) > maxOutput {
				return nil
			}
			out = append(out, d.out...)
			pos += n
		case m&0xFFFFFFF0 == skipMagicLo:
			if len(z)-pos < 8 {
				return nil
			}
			sz := int(binary.LittleEndian.Uint32(z[pos+4:]))
			if sz < 0 || 8+sz > len(z)-pos {
				return nil
			}
			pos += 8 + sz
		default:
			return nil
		}
	}
	if len(out) > maxOutput {
		return nil
	}
	return out
}

// frameInfo is a parsed frame header.
type frameInfo struct {
	window   int64
	blockMax int64
	checksum bool
	fcs      uint64 // ^uint64(0) when not stated
	fcsSet   bool
}

// parseFrameHeader decodes the header of the frame starting at b[0]
// (magic included), returning its parameters and total length.
func parseFrameHeader(b []byte) (frameInfo, int, error) {
	var fi frameInfo
	if len(b) < 5 {
		return fi, 0, errCorrupt
	}
	fhd := b[4]
	if fhd&(1<<3) != 0 || fhd&(1<<4) != 0 {
		return fi, 0, errCorrupt // reserved and (this version's) unused bits
	}
	single := fhd&(1<<5) != 0
	fi.checksum = fhd&(1<<2) != 0
	didSize := []int{0, 1, 2, 4}[fhd&3]
	p := 5
	if p+didSize > len(b) {
		return fi, 0, errCorrupt
	}
	var did uint32
	for i := 0; i < didSize; i++ {
		did |= uint32(b[p+i]) << (8 * i)
	}
	if did != 0 {
		return fi, 0, errCorrupt // dictionaries are out of scope
	}
	p += didSize
	fcsSize := 0
	switch fhd >> 6 {
	case 0:
		if single {
			fcsSize = 1
		}
	case 1:
		fcsSize = 2
	case 2:
		fcsSize = 4
	default:
		fcsSize = 8
	}
	if p+fcsSize > len(b) {
		return fi, 0, errCorrupt
	}
	var content uint64
	switch fcsSize {
	case 0:
	case 1:
		content = uint64(b[p])
	case 2:
		content = uint64(binary.LittleEndian.Uint16(b[p:])) + 256
	case 4:
		content = uint64(binary.LittleEndian.Uint32(b[p:]))
	case 8:
		content = binary.LittleEndian.Uint64(b[p:])
	}
	p += fcsSize
	var window uint64
	if single {
		window = content
		if content > maxOutput {
			return fi, 0, errCorrupt
		}
	} else {
		if p >= len(b) {
			return fi, 0, errCorrupt
		}
		wd := b[p]
		p++
		exp := uint64(wd >> 3)
		if 10+exp > 27 {
			return fi, 0, errCorrupt
		}
		base := uint64(1) << (10 + exp)
		window = base + base/8*uint64(wd&7)
	}
	if window > maxWindow {
		return fi, 0, errCorrupt
	}
	fi.window = int64(window)
	fi.blockMax = int64(min64(window, maxBlockSize))
	fi.fcs, fi.fcsSet = content, fcsSize > 0
	return fi, p, nil

}

func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// frame decodes one frame starting at pos, returning the bytes it
// occupies on disk.
func (d *decoder) frame(b []byte, pos int) (int, error) {
	fi, hlen, err := parseFrameHeader(b[pos:])
	if err != nil {
		return 0, err
	}
	p := pos + hlen
	d.out = d.out[:0]

	var xx *xxh64
	if fi.checksum {
		xx = newXXH64()
	}
	for {
		if len(b)-p < 3 {
			return 0, errCorrupt
		}
		bh := uint32(b[p]) | uint32(b[p+1])<<8 | uint32(b[p+2])<<16
		p += 3
		last := bh&1 != 0
		typ := (bh >> 1) & 3
		size := int(bh >> 3)
		if int64(size) > fi.blockMax {
			return 0, errCorrupt
		}
		start := len(d.out)
		switch typ {
		case 0: // raw
			if p+size > len(b) {
				return 0, errCorrupt
			}
			if len(d.out)+size > maxOutput {
				return 0, errCorrupt
			}
			d.out = append(d.out, b[p:p+size]...)
			p += size
		case 1: // RLE
			if p+1 > len(b) {
				return 0, errCorrupt
			}
			if len(d.out)+size > maxOutput {
				return 0, errCorrupt
			}
			c := b[p]
			p++
			for i := 0; i < size; i++ {
				d.out = append(d.out, c)
			}
		case 2: // compressed
			if p+size > len(b) {
				return 0, errCorrupt
			}
			if len(d.out)+int(fi.blockMax) > maxOutput {
				return 0, errCorrupt
			}
			if err := d.block(b[p:p+size], int(fi.blockMax)); err != nil {
				return 0, err
			}
			p += size
		default:
			return 0, errCorrupt
		}
		if xx != nil {
			xx.write(d.out[start:])
		}
		if last {
			break
		}
	}
	if fi.checksum {
		if len(b)-p < 4 {
			return 0, errCorrupt
		}
		if binary.LittleEndian.Uint32(b[p:p+4]) != uint32(xx.sum()) {
			return 0, errCorrupt
		}
		p += 4
	}
	if fi.fcsSet && fi.fcs != uint64(len(d.out)) {
		return 0, errCorrupt
	}
	return p - pos, nil
}

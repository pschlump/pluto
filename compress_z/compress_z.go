/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package compress_z implements the historical UNIX compress/.Z
// format (the compress(1) "zopen" wire format that .Z files and
// .tar.Z archives use): a 3-byte header followed by a stream of
// least-significant-bit-first LZW codes packed in groups of eight,
// widening from 9 bits as the dictionary grows, with code 256
// reserved as a table-CLEAR marker in block mode.
//
// This is a different wire format from pluto/lzw, which follows the
// algs4 MSB-first fixed-12-bit teaching variant; the two packages
// share the LZW algorithm family, not bytes.
//
// Complexity is O(n) for both Compress and Expand (amortized; the
// encoder's dictionary is a hash map, the decoder's walks are bounded
// by the chain lengths actually encoded).
//
// Not safe for concurrent mutation — the two whole-buffer functions
// are pure and the streaming Reader/Writer of stream.go carry their
// own per-stream state, so concurrent use of separate streams is
// fine and there is no _ts twin.
package compress_z

import "fmt"

// Header and code-space constants (zopen.c).
const (
	magic0     = 0x1F // first header byte
	magic1     = 0x9D // second header byte
	bitMask    = 0x1F // maxbits field within the third header byte
	blockMode  = 0x80 // block-mode flag within the third header byte
	initBits   = 9    // codes start at 9 bits
	clearCode  = 256  // table-reset code (block mode only)
	firstEntry = 257  // first free dictionary slot in block mode
)

// Compress encodes data as a .Z stream: block mode, maxBits 16.
// It never returns nil and never panics.
func Compress(data []byte) []byte {
	return CompressBits(data, 16)
}

// CompressBits encodes data as a .Z stream with the given maximum
// code width (9..16, the compress -b range).  Smaller widths cap the
// dictionary sooner, trading ratio for decode cost.  For maxBits 9,
// 10 and 11 the width legitimately overshoots to maxBits+1 when the
// table fills — the format's own quirk, mirrored by Expand.
//
// The encoder never emits CLEAR (compress(1) clears the table when
// its compression ratio drops at a 10000-byte checkpoint; skipping
// that heuristic keeps streams deterministic and the decoder accepts
// clear-free streams without reservation).
//
// Panics if maxBits is outside [9, 16] — the fix is to pass a value
// in that range (the format has no other widths).
func CompressBits(data []byte, maxBits int) []byte {
	if maxBits < 9 || maxBits > 16 {
		panic(fmt.Sprintf("compress_z.CompressBits: maxBits %d out of range, want 9..16", maxBits))
	}
	out := make([]byte, 0, len(data)/2+8)
	out = append(out, magic0, magic1, byte(maxBits)|blockMode)
	if len(data) == 0 {
		return out // header only, a valid empty stream
	}

	w := &bitWriter{buf: out, base: len(out) * 8} // groups start after the header
	maxmax := uint32(1) << maxBits
	free := uint32(firstEntry)
	nBits := uint(initBits)
	maxcode := uint32(1)<<initBits - 1
	tab := make(map[uint64]uint32, 4096)

	ent := uint32(data[0])
	for _, c := range data[1:] {
		key := uint64(ent)<<8 | uint64(c)
		if code, ok := tab[key]; ok {
			ent = code
			continue
		}
		w.writeCode(ent, nBits)
		// Width change after the write and before the add — the
		// reference encoder's order — landing on the next group
		// boundary.  The decoder's counter lags one add behind, and
		// this ordering is what absorbs the lag.
		if free > maxcode {
			w.realign(nBits)
			nBits++
			if nBits == uint(maxBits) {
				maxcode = maxmax
			} else {
				maxcode = 1<<nBits - 1
			}
		}
		if free < maxmax {
			tab[key] = free
			free++
		}
		ent = uint32(c)
	}
	w.writeCode(ent, nBits)
	return w.bytes()
}

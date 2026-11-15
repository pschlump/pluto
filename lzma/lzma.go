/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lzma implements the .lzma "alone" format (the lzma(1)
// legacy container: a 13-byte header and one LZMA stream) over byte
// slices: Compress produces streams xz --format=lzma and Python's
// FORMAT_ALONE read, and Expand reads what they write — known or
// unknown uncompressed size, end-marker or size-terminated.  The LZMA
// model itself lives in pluto/internal/lzmacore, shared with the xz
// codec.
//
// Complexity is O(n) per direction.  Not safe for concurrent mutation
// of one stream; separate streams are independent (the xz precedent).
package lzma

import (
	"encoding/binary"

	"github.com/pschlump/pluto/internal/lzmacore"
)

// maxOutput bounds whole-buffer expansion (a decompression-bomb
// guard; 1 GiB, the xz precedent).
const maxOutput = 1 << 30

// dictSize is the dictionary size our header declares (8 MiB, the
// xz -6 convention).
const dictSize = 8 << 20

// Compress writes data as one .lzma stream: the 13-byte header
// (default properties 0x5D, 8 MiB dictionary, unknown uncompressed
// size) and the LZMA stream closed by the end marker — the shape
// xz --format=lzma writes.  It never fails.
func Compress(data []byte) []byte {
	out := make([]byte, 0, len(data)/2+64)
	out = append(out, lzmacore.DefaultProps)
	var hdr [12]byte
	binary.LittleEndian.PutUint32(hdr[0:4], dictSize)
	binary.LittleEndian.PutUint64(hdr[4:12], ^uint64(0)) // size unknown: end-marker terminated
	out = append(out, hdr[:]...)

	e, st := lzmacore.LzmaEncodeNoFlush(nil, nil, data, nil, 0)
	lzmacore.EncodeEndMarker(e, st, len(data)&(1<<uint(st.Pb)-1))
	e.Flush()
	return append(out, e.Out...)
}

// Expand decodes a .lzma stream and returns the uncompressed bytes.
// nil means malformed: a short header, a bad properties byte, a
// corrupt stream, a wrong known size, or truncation.  Never panics.
func Expand(z []byte) []byte {
	if len(z) < 13 {
		return nil
	}
	if z[0] > 224 {
		return nil
	}
	size := int64(binary.LittleEndian.Uint64(z[5:13]))
	if size > maxOutput {
		return nil
	}
	rc := lzmacore.NewRangeDecoder(z[13:])
	st := lzmacore.NewLzmaState(z[0])
	out := make([]byte, 0, capHint(size))
	switch {
	case size >= 0:
		if !lzmacore.LzmaDecode(rc, st, &out, int(size), 0) {
			return nil
		}
	default: // unknown size: decode to the end marker
		if !lzmacore.LzmaDecodeStream(rc, st, &out, maxOutput) {
			return nil
		}
	}
	return out
}

func capHint(size int64) int {
	if size <= 0 || size > 1<<24 {
		return 0
	}
	return int(size)
}

// DecodeRaw decodes a headerless LZMA stream — the form 7z stores —
// from the coder properties it carries ([props byte, dictionary size
// LE32]) and a known output size.  outSize <0 means the stream runs
// to an end marker instead.  nil means malformed.
func DecodeRaw(data, props []byte, outSize int64) []byte {
	if len(props) != 5 || props[0] > 224 || outSize > maxOutput {
		return nil
	}
	rc := lzmacore.NewRangeDecoder(data)
	st := lzmacore.NewLzmaState(props[0])
	out := make([]byte, 0, capHint(outSize))
	if outSize >= 0 {
		if !lzmacore.LzmaDecode(rc, st, &out, int(outSize), 0) {
			return nil
		}
		return out
	}
	if !lzmacore.LzmaDecodeStream(rc, st, &out, maxOutput) {
		return nil
	}
	return out
}

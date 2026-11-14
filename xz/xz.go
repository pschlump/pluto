/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package xz implements the .xz container (the xz file format: stream
// header, LZMA2-compressed blocks with CRC32/CRC64/SHA256 checks, an
// index, a stream footer) over byte slices: Compress produces streams
// xz(1) reads, and Expand reads streams xz(1) writes — any preset,
// any check type, concatenated streams, and stream padding.  The LZMA
// coder (both directions) is from scratch after Igor Pavlov's
// public-domain reference.
//
// Complexity is O(n) per direction (the encoder's match finder is a
// greedy hash chain with bounded walks).
//
// Not safe for concurrent mutation — the two whole-buffer functions
// are pure and the streaming Reader/Writer of stream.go carry their
// own per-stream state, so concurrent use of separate streams is fine
// (the bzip2/lz4 precedent).
package xz

// Compress writes data as one .xz stream: a single LZMA2-filter block
// with the CRC64 check (xz's default), an index, and the footer.  It
// never fails; empty input produces the canonical 32-byte empty
// stream.
func Compress(data []byte) []byte {
	out := make([]byte, 0, len(data)/2+128)

	// Stream header: magic, flags (CRC64 check), CRC32 of the flags.
	out = append(out, 0xFD, '7', 'z', 'X', 'Z', 0x00, 0x00, 0x04)
	out = appendLE32(out, crc32Bytes([]byte{0x00, 0x04}))

	// Block: header (flags 0x00 = one filter, no declared sizes; the
	// LZMA2 filter with the 8 MiB dictionary property), then chunks.
	bh := []byte{0x02, 0x00, lzma2FilterID, 0x01, dictProp8MiB, 0, 0, 0}
	bh = appendLE32(bh, crc32Bytes(bh[:8]))
	out = append(out, bh...)

	blockDataStart := len(out)
	chunks := encodeLZMA2Chunks(nil, nil, data)
	out = append(out, chunks...)
	out = append(out, 0x00)                        // end of block data
	unpadded := len(out) - blockDataStart + 12 + 8 // data (no padding) + header + check (CRC64)
	for (len(out)-blockDataStart)%4 != 0 {
		out = append(out, 0x00)
	}
	out = appendLE64(out, crc64Bytes(data))

	// Index: indicator, count, records, padding to 4, CRC32.
	idxStart := len(out)
	out = append(out, 0x00, 0x01)
	out = writeVLI(out, int64(unpadded))
	out = writeVLI(out, int64(len(data)))
	for (len(out)-idxStart+4)%4 != 0 {
		out = append(out, 0x00)
	}
	out = appendLE32(out, crc32Bytes(out[idxStart:]))

	// Footer: CRC32 (over backward size + flags), backward size,
	// flags, YZ.
	ftr := make([]byte, 0, 6)
	ftr = appendLE32(ftr, uint32(len(out)-idxStart)/4-1)
	ftr = append(ftr, 0x00, 0x04)
	out = appendLE32(out, crc32Bytes(ftr))
	out = append(out, ftr...)
	out = append(out, 'Y', 'Z')
	return out
}

func appendLE32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendLE64(dst []byte, v uint64) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24), byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

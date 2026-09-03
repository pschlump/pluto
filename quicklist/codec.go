/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package quicklist

import (
	"encoding/binary"

	"github.com/pschlump/pluto/lzw"
)

// Codec is the byte-level compression hook for WithCompression: the
// segment wire bytes produced by the caller's encoder are stored as
// Compress(encoder(...)) and restored with Decompress.  The functions
// must round-trip exactly; they are called with the list lock held (in
// quicklist_ts), so a Codec need not be concurrency-safe — pluto/lzw's
// stateless functions are.
type Codec interface {
	Compress(b []byte) []byte
	Decompress(b []byte) []byte
}

// lzwCodec adapts pluto/lzw to Codec.
type lzwCodec struct{}

// LZWCodec returns a Codec built on pluto/lzw (the LZF stand-in of the
// Redis quicklist design).  It is stateless and safe for concurrent use.
func LZWCodec() Codec { return lzwCodec{} }

func (lzwCodec) Compress(b []byte) []byte   { return lzw.Compress(b) }
func (lzwCodec) Decompress(b []byte) []byte { return lzw.Expand(b) }

// EncodeStringSegment serializes a string segment for WithCompression:
// each string as a uvarint length followed by its bytes.  Pair it with
// DecodeStringSegment.
func EncodeStringSegment(items []string) []byte {
	total := 0
	for _, s := range items {
		total += binary.MaxVarintLen64 + len(s)
	}
	buf := make([]byte, 0, total)
	for _, s := range items {
		buf = binary.AppendUvarint(buf, uint64(len(s)))
		buf = append(buf, s...)
	}
	return buf
}

// DecodeStringSegment restores the n strings written by
// EncodeStringSegment.  Malformed input yields what decoded cleanly —
// a Codec that round-trips never produces it.
func DecodeStringSegment(b []byte, n int) []string {
	out := make([]string, 0, n)
	for len(out) < n && len(b) > 0 {
		l, k := binary.Uvarint(b)
		if k <= 0 || uint64(len(b)-k) < l {
			break
		}
		out = append(out, string(b[k:k+int(l)]))
		b = b[k+int(l):]
	}
	return out
}

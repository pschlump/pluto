/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package zlib implements the zlib stream format of RFC 1950 over
// pluto/deflate — the DEFLATE framing used inside zip entries'
// "traditional" relatives, PNG chunks, and HTTP gzip content when a
// checksum without a filename header is wanted: a 2-byte CMF/FLG
// header (with its %31 check bits), a raw DEFLATE stream, and a
// big-endian Adler-32 trailer.  Two stateless functions are the whole
// API:
//
//	z := zlib.Compress(data)   // CMF 0x78, FCHECK-correct FLG
//	back := zlib.Expand(z)     // == data, or nil if malformed
//
// Neither function panics; both are stateless, and there is no _ts
// twin (the lzw precedent).
package zlib

import "github.com/pschlump/pluto/deflate"

// Compress returns data as one zlib stream.  CMF is 0x78 (CM 8
// deflate, CINFO 7 = 32K window); FLG carries FLEVEL 2 (bits 6-7) with
// FDICT 0 and the FCHECK bits that make CMF*256+FLG a multiple of 31 —
// yielding the canonical 0x78 0x9C header.  Any input is accepted,
// including nil and empty; the result is never nil.  Complexity is the
// cost of deflate.Compress plus O(n) for the Adler-32.
func Compress(data []byte) []byte {
	cmf := byte(0x78)
	flg := byte(0x80) // FLEVEL 2, FDICT 0; FCHECK filled below
	rem := (int(cmf)*256 + int(flg)) % 31
	if rem != 0 {
		flg += byte(31 - rem)
	}
	z := make([]byte, 0, len(data)/2+16)
	z = append(z, cmf, flg)
	z = append(z, deflate.Compress(data)...)
	s1, s2 := adler32(1, 0, data)
	v := s2<<16 | s1 // the 32-bit Adler-32, big-endian in the trailer
	return append(z, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// adler32 folds data into an in-progress Adler-32 (RFC 1950 §8.2):
// s1 = 1 + sum of bytes, s2 = sum of running s1 values, both mod
// 65521.  Complexity is O(n).
func adler32(s1, s2 uint32, data []byte) (uint32, uint32) {
	for _, b := range data {
		s1 = (s1 + uint32(b)) % 65521
		s2 = (s2 + s1) % 65521
	}
	return s1, s2
}

// Expand decodes one zlib stream.  The header must name deflate with
// a window of at most 32K and carry correct check bits; FDICT streams
// (preset dictionaries) are rejected — they cannot be decoded without
// the dictionary.  The Adler-32 trailer is always verified.  A
// malformed stream returns nil: bad header, reserved CM, CINFO > 7,
// wrong check bits, FDICT set, a malformed DEFLATE stream, a wrong
// Adler-32, a truncated trailer, or trailing bytes after the stream.
// Complexity is the cost of deflate.Expand plus O(n) for the Adler-32.
func Expand(z []byte) []byte {
	if len(z) < 2 {
		return nil
	}
	cmf, flg := z[0], z[1]
	if cmf&0x0F != 8 { // CM: only deflate is defined
		return nil
	}
	if cmf>>4 > 7 { // CINFO: window larger than 32K
		return nil
	}
	if (int(cmf)*256+int(flg))%31 != 0 {
		return nil // FCHECK
	}
	if flg&0x20 != 0 {
		return nil // FDICT: needs a preset dictionary we do not have
	}
	data, n := deflate.Expand(z[2:])
	if n == 0 {
		return nil
	}
	if len(z) < n+6 { // 2 header + n deflate + 4 adler
		return nil
	}
	s1, s2 := adler32(1, 0, data)
	v := s2<<16 | s1
	if uint32(z[n+2])<<24|uint32(z[n+3])<<16|uint32(z[n+4])<<8|uint32(z[n+5]) != v {
		return nil // Adler-32 mismatch
	}
	if len(z) != n+6 {
		return nil // trailing bytes after the stream
	}
	return data
}

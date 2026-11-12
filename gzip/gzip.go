/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package gzip implements the gzip member format of RFC 1952 over
// pluto/deflate — the framing a `.gz` file (and a `.tar.gz` / `.tgz`
// archive) is made of: a 10-byte header, a raw DEFLATE stream, and an
// 8-byte CRC-32 + ISIZE trailer.  Two stateless functions are the
// whole API:
//
//	gz := gzip.Compress(data)   // one member, minimal header
//	back := gzip.Expand(gz)     // == data, or nil if malformed
//
// Expand reads every member of a multi-member file (concatenated
// members are concatenated content) and parses — without needing —
// the FHCRC, FEXTRA, FNAME and FCOMMENT header fields, verifying
// FHCRC when present and both trailer words always.  Neither function
// panics; both are stateless, and there is no _ts twin (the lzw
// precedent).
package gzip

import (
	"github.com/pschlump/pluto/crc"
	"github.com/pschlump/pluto/deflate"
)

// Header flag bits (RFC 1952 §2.3).
const (
	ftext    = 1 << 0
	fhcrc    = 1 << 1
	fextra   = 1 << 2
	fname    = 1 << 3
	fcomment = 1 << 4
)

// Compress returns data as a single gzip member.  The header is the
// minimal one: no flags (no FNAME/FEXTRA/FHCRC — the tar or zip layer
// above carries names), MTIME 0, XFL 0, OS 255 (unknown).  The trailer
// is the CRC-32 and the length mod 2^32 of data, both little-endian.
// Any input is accepted, including nil and empty; the result is never
// nil.  Complexity is the cost of deflate.Compress plus O(n) for the
// CRC.
func Compress(data []byte) []byte {
	gz := make([]byte, 0, len(data)/2+32)
	gz = append(gz,
		0x1f, 0x8b, // ID1 ID2
		0x08,       // CM: deflate
		0x00,       // FLG: no optional fields
		0, 0, 0, 0, // MTIME: 0 (no timestamp)
		0x00, // XFL
		0xff, // OS: unknown
	)
	gz = append(gz, deflate.Compress(data)...)
	gz = le32(gz, crc.Checksum32(data, crc.IEEETable))
	gz = le32(gz, uint32(len(data)))
	return gz
}

// le32 appends v in little-endian order.
func le32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// le32at reads the little-endian uint32 at i.
func le32at(b []byte, i int) uint32 {
	return uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
}

// Expand decodes a gzip file: one member, or several concatenated
// (their contents concatenate — what `gzip -d` produces for a
// multi-member stream).  The FHCRC header checksum is verified when
// present; the CRC-32 and ISIZE trailer words are always verified.
// Every other header field is parsed and skipped.  A malformed file
// returns nil: bad magic or method, reserved flag bits, an unterminated
// or overrunning optional field, a bad FHCRC, a malformed DEFLATE
// stream, a wrong CRC-32 or ISIZE, a truncated trailer, or trailing
// bytes that are not another member.  Complexity is the cost of
// deflate.Expand plus O(n) for the CRCs.
func Expand(gz []byte) []byte {
	var out []byte
	for len(gz) > 0 {
		data, rest, ok := expandMember(gz)
		if !ok {
			return nil
		}
		out = append(out, data...)
		gz = rest
	}
	return out
}

// expandMember decodes the first member of gz, returning its
// decompressed data and the remaining input.
func expandMember(gz []byte) (data, rest []byte, ok bool) {
	if len(gz) < 10 {
		return nil, nil, false
	}
	if gz[0] != 0x1f || gz[1] != 0x8b || gz[2] != 0x08 {
		return nil, nil, false // bad magic or not deflate
	}
	flags := gz[3]
	if flags&^(ftext|fhcrc|fextra|fname|fcomment) != 0 {
		return nil, nil, false // reserved flag bits set
	}
	p := 10 // past MTIME/XFL/OS, which carry no decodable information here
	if flags&fextra != 0 {
		if p+2 > len(gz) {
			return nil, nil, false
		}
		xlen := int(gz[p]) | int(gz[p+1])<<8
		p += 2 + xlen
		if p > len(gz) {
			return nil, nil, false
		}
	}
	if flags&fname != 0 {
		p = skipZeroString(gz, p)
		if p < 0 {
			return nil, nil, false
		}
	}
	if flags&fcomment != 0 {
		p = skipZeroString(gz, p)
		if p < 0 {
			return nil, nil, false
		}
	}
	if flags&fhcrc != 0 {
		if p+2 > len(gz) {
			return nil, nil, false
		}
		want := uint16(gz[p]) | uint16(gz[p+1])<<8
		if want != uint16(crc.Checksum32(gz[:p], crc.IEEETable)&0xFFFF) {
			return nil, nil, false
		}
		p += 2
	}

	data, n := deflate.Expand(gz[p:])
	if n == 0 {
		return nil, nil, false
	}
	p += n
	if p+8 > len(gz) {
		return nil, nil, false // truncated trailer
	}
	if got := crc.Checksum32(data, crc.IEEETable); got != le32at(gz, p) {
		return nil, nil, false // CRC-32 mismatch
	}
	if uint32(len(data)) != le32at(gz, p+4) {
		return nil, nil, false // ISIZE mismatch
	}
	return data, gz[p+8:], true
}

// skipZeroString advances past a zero-terminated string starting at i,
// returning the index after the terminator, or -1 if none exists.
func skipZeroString(b []byte, i int) int {
	for ; i < len(b); i++ {
		if b[i] == 0 {
			return i + 1
		}
	}
	return -1
}

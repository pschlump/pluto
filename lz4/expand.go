/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4

// Expand decodes an LZ4 frame (or a concatenation of frames, and any
// skippable frames between them).  It returns nil for malformed
// input: empty input, a bad magic, a wrong FLG version, linked
// (non-independent) blocks — this package decodes the
// independent-block form only, the form every default lz4(1)
// invocation writes — dictID frames (they need the external
// dictionary), a wrong header checksum, block sizes over the frame's
// declared maximum, a corrupt block sequence, a wrong per-block or
// content checksum, a missing end mark, a wrong declared content
// size, or trailing garbage.  Never panics.  Complexity is O(n).
func Expand(z []byte) []byte {
	if len(z) == 0 {
		return nil // a valid empty frame is 15 bytes, not 0
	}
	out := make([]byte, 0, len(z)*3+16)
	pos := 0
	for pos < len(z) {
		if pos+4 > len(z) {
			return nil
		}
		magic := readLE32(z[pos:])
		if magic&0xFFFFFFF0 == skipMagicHi&0xFFFFFFF0 {
			// Skippable frame: 4-byte magic + 4-byte size + size bytes.
			if pos+8 > len(z) {
				return nil
			}
			sz := int(readLE32(z[pos+4:]))
			if pos+8+sz > len(z) {
				return nil
			}
			pos += 8 + sz
			continue
		}
		if magic != frameMagic {
			return nil
		}
		if pos+7 > len(z) {
			return nil
		}
		flg, bd := z[pos+4], z[pos+5]
		if flg>>6 != 1 {
			return nil // version 01 only
		}
		if flg&0x02 != 0 {
			return nil // reserved bit
		}
		if flg&flgIndep == 0 {
			return nil // linked blocks unsupported (never default)
		}
		if flg&0x01 != 0 {
			return nil // dictID frame: needs the external dictionary
		}
		if bd&0x80 != 0 || bd&0x0F != 0 {
			return nil // reserved bits
		}
		blockMax := 1 << (8 + 2*(bd>>4&7)) // 4=64KB .. 7=8MB
		if bd>>4&7 < 4 {
			return nil
		}
		// Optional C.Size, then the mandatory one-byte header checksum
		// over FLG+BD (+C.Size) — the magic is NOT included.
		hdrEnd := pos + 6
		contentSize := int64(-1)
		if flg&0x08 != 0 { // C.Size
			if hdrEnd+8 > len(z) {
				return nil
			}
			contentSize = int64(uint64(readLE32(z[hdrEnd:])) | uint64(readLE32(z[hdrEnd+4:]))<<32)
			hdrEnd += 8
		}
		if hdrEnd >= len(z) {
			return nil
		}
		if z[hdrEnd] != byte(xxh32(z[pos+4:hdrEnd])>>8) {
			return nil
		}
		pos = hdrEnd + 1

		member := make([]byte, 0, len(z))
		for {
			if pos+4 > len(z) {
				return nil
			}
			szWord := readLE32(z[pos:])
			pos += 4
			if szWord == 0 { // EndMark
				break
			}
			raw := szWord&0x80000000 != 0
			sz := int(szWord & 0x7FFFFFFF)
			if sz == 0 || sz > blockMax || pos+sz > len(z) {
				return nil
			}
			if flg&0x10 != 0 { // per-block checksum, over the STORED bytes
				if pos+sz+4 > len(z) {
					return nil
				}
				if readLE32(z[pos+sz:]) != xxh32(z[pos:pos+sz]) {
					return nil
				}
			}
			var chunk []byte
			if raw {
				chunk = z[pos : pos+sz]
			} else {
				chunk = expandBlock(z[pos:pos+sz], len(member)+1<<20)
				if chunk == nil {
					return nil
				}
			}
			pos += sz
			if flg&0x10 != 0 {
				pos += 4
			}
			member = append(member, chunk...)
		}

		if flg&flgContentSum != 0 {
			if pos+4 > len(z) {
				return nil
			}
			if readLE32(z[pos:]) != xxh32(member) {
				return nil
			}
			pos += 4
		}
		if contentSize >= 0 && contentSize != int64(len(member)) {
			return nil
		}
		out = append(out, member...)
	}
	return out
}

// expandBlock decodes one independent LZ4 block.  capHint is the
// caller's guess for preallocation.  nil is returned for malformed
// sequences: a truncated token run, an offset of 0 or past the
// output so far, or output past n.
func expandBlock(b []byte, capHint int) []byte {
	const (
		minMatch  = 4
		maxOffset = 65535
	)
	out := make([]byte, 0, min(capHint, 1<<22))
	pos := 0
	for pos < len(b) {
		token := b[pos]
		pos++
		litLen := int(token >> 4)
		if litLen == 15 {
			for {
				if pos >= len(b) {
					return nil
				}
				c := b[pos]
				pos++
				litLen += int(c)
				if c != 255 {
					break
				}
			}
		}
		if pos+litLen > len(b) {
			return nil
		}
		out = append(out, b[pos:pos+litLen]...)
		pos += litLen
		if pos == len(b) {
			break // last sequence: literals only
		}
		if pos+2 > len(b) {
			return nil
		}
		offset := int(b[pos]) | int(b[pos+1])<<8
		pos += 2
		if offset < 1 || offset > len(out) || offset > maxOffset {
			return nil
		}
		matchLen := int(token&0xF) + minMatch
		if token&0xF == 15 {
			for {
				if pos >= len(b) {
					return nil
				}
				c := b[pos]
				pos++
				matchLen += int(c)
				if c != 255 {
					break
				}
			}
		}
		start := len(out) - offset
		for i := 0; i < matchLen; i++ {
			if len(out) >= 1<<28 {
				return nil // absurd expansion
			}
			out = append(out, out[start+i])
		}
	}
	return out
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

import (
	"crypto/sha256"

	"github.com/pschlump/pluto/crc"
)

// maxOutput bounds whole-buffer expansion (a decompression-bomb
// guard; 1 GiB).
const maxOutput = 1 << 30

// check sizes by check id (0 = none, 1 = CRC32, 4 = CRC64, 10 = SHA256).
var checkSize = [...]int{0, 4, -1, -1, 8, -1, -1, -1, -1, -1, 32}

func validCheck(id int) bool {
	return id >= 0 && id < len(checkSize) && checkSize[id] >= 0
}

func crc32Bytes(b []byte) uint32 { return crc.Checksum32(b, crc.IEEETable) }
func crc64Bytes(b []byte) uint64 { return crc.Checksum64(b, crc.ECMATable) }

// block is one parsed block's bookkeeping for the index.
type block struct {
	unpaddedSize int64
	uncompSize   int64
}

// Expand decodes an .xz stream — one, or several concatenated with
// stream padding between them — and returns the concatenated
// uncompressed bytes.  nil means malformed: a bad magic, a wrong
// header/footer CRC, a reserved flag, a filter that is not LZMA2, bad
// filter properties, a corrupt LZMA2 chunk sequence, a bad block
// check (CRC32/CRC64/SHA256 as the stream flags demand), an index
// that disagrees with the blocks, or a wrong footer.  Truncation also
// yields nil.  Never panics.  Complexity is O(n).
func Expand(z []byte) []byte {
	sawStream := false
	out := make([]byte, 0, len(z)*2+64)
	pos := 0
	for {
		// stream padding: whole 4-byte zero words between streams
		for pos+4 <= len(z) && z[pos] == 0 && z[pos+1] == 0 && z[pos+2] == 0 && z[pos+3] == 0 {
			pos += 4
		}
		if pos == len(z) {
			if pos == 0 {
				return nil // xz(1) rejects an empty file outright
			}
			if !sawStream {
				return nil // padding only, no stream — xz(1) rejects that too
			}
			return out
		}
		sawStream = true
		var more bool
		var err bool
		out, pos, more, err = expandStream(z, pos, out)
		if err {
			return nil
		}
		if !more && pos != len(z) {
			return nil // trailing garbage
		}
		_ = more
	}
}

// expandStream decodes one stream starting at pos, appending to out.
// more reports whether more streams may follow.
func expandStream(z []byte, pos int, out []byte) ([]byte, int, bool, bool) {
	// Stream header: magic (6), flags (2), CRC32 (4).
	if pos+12 > len(z) {
		return nil, 0, false, true
	}
	hdr := z[pos : pos+12]
	if string(hdr[:6]) != "\xFD7zXZ\x00" {
		return nil, 0, false, true
	}
	if crc32Bytes(hdr[6:8]) != le32(hdr[8:12]) {
		return nil, 0, false, true
	}
	if hdr[6] != 0 || hdr[7]&0xF0 != 0 || !validCheck(int(hdr[7]&0x0F)) {
		return nil, 0, false, true
	}
	check := int(hdr[7] & 0x0F)
	pos += 12

	var blocks []block
	for {
		if pos >= len(z) {
			return nil, 0, false, true
		}
		if z[pos] == 0 {
			break // index indicator
		}
		var b block
		var err bool
		out, pos, b, err = expandBlock(z, pos, out, check)
		if err {
			return nil, 0, false, true
		}
		blocks = append(blocks, b)
	}

	// Index: 0x00, count VLI, records, padding, CRC32.
	idxStart := pos
	pos++
	count, pos, ok := readVLI(z, pos)
	if !ok || count != int64(len(blocks)) || count > 1<<31 {
		return nil, 0, false, true
	}
	for i := int64(0); i < count; i++ {
		unp, pos2, ok := readVLI(z, pos)
		if !ok {
			return nil, 0, false, true
		}
		unc, pos3, ok := readVLI(z, pos2)
		if !ok {
			return nil, 0, false, true
		}
		pos = pos3
		if blocks[i].unpaddedSize != unp || blocks[i].uncompSize != unc {
			return nil, 0, false, true
		}
	}
	for (pos-idxStart+4)%4 != 0 {
		if pos >= len(z) || z[pos] != 0 {
			return nil, 0, false, true
		}
		pos++
	}
	if pos+4 > len(z) || crc32Bytes(z[idxStart:pos]) != le32(z[pos:pos+4]) {
		return nil, 0, false, true
	}
	indexSize := int64(pos + 4 - idxStart)

	// Stream footer: CRC32 (over the next 6), backward size, flags, YZ.
	pos += 4
	if pos+12 > len(z) {
		return nil, 0, false, true
	}
	ftr := z[pos : pos+12]
	if crc32Bytes(ftr[4:10]) != le32(ftr[0:4]) {
		return nil, 0, false, true
	}
	if int64(le32(ftr[4:8])+1)*4 != indexSize {
		return nil, 0, false, true
	}
	if ftr[8] != hdr[6] || ftr[9] != hdr[7] || string(ftr[10:12]) != "YZ" {
		return nil, 0, false, true
	}
	return out, pos + 12, true, false
}

// expandBlock decodes one block (header, LZMA2 data, padding, check).
func expandBlock(z []byte, pos int, out []byte, check int) ([]byte, int, block, bool) {
	if pos >= len(z) {
		return nil, 0, block{}, true
	}
	hdrSize := (int(z[pos]) + 1) * 4
	if pos+hdrSize > len(z) || hdrSize < 8 || hdrSize > 1024 {
		return nil, 0, block{}, true
	}
	hdr := z[pos : pos+hdrSize]
	if crc32Bytes(hdr[:hdrSize-4]) != le32(hdr[hdrSize-4:]) {
		return nil, 0, block{}, true
	}
	flags := hdr[1]
	if flags&0x3C != 0 || flags&3 != 0 {
		return nil, 0, block{}, true // reserved bits, or not the single LZMA2 filter
	}
	p := 2
	var declaredComp, declaredUncomp int64
	if flags&0x40 != 0 {
		v, np, ok := readVLI(hdr, p)
		if !ok {
			return nil, 0, block{}, true
		}
		declaredComp, p = v, np
	}
	if flags&0x80 != 0 {
		v, np, ok := readVLI(hdr, p)
		if !ok {
			return nil, 0, block{}, true
		}
		declaredUncomp, p = v, np
	}
	// The one filter: id, property size, property.
	fid, np, ok := readVLI(hdr, p)
	if !ok || fid != lzma2FilterID {
		return nil, 0, block{}, true
	}
	psize, np, ok := readVLI(hdr, np)
	if !ok || psize != 1 || hdr[np] > 40 {
		return nil, 0, block{}, true
	}
	_ = int(hdr[np]) // distances are bounded by produced bytes; the property only bounds the window
	pos += hdrSize

	dataStart := len(out)
	blockDataStart := pos
	dictBase := len(out) // where the LZMA2 dictionary began (last dict reset)
	var st *lzmaState

	for {
		if pos >= len(z) {
			return nil, 0, block{}, true
		}
		ctl := z[pos]
		pos++
		switch ctl {
		case 0x00: // end of the block's compressed data
			b := block{
				unpaddedSize: int64(hdrSize + pos - blockDataStart + checkSize[check]),
				uncompSize:   int64(len(out) - dataStart),
			}
			if declaredComp != 0 && declaredComp != int64(pos-blockDataStart) {
				return nil, 0, block{}, true
			}
			if declaredUncomp != 0 && declaredUncomp != b.uncompSize {
				return nil, 0, block{}, true
			}
			// block padding to a 4-byte multiple
			for (pos-blockDataStart)%4 != 0 {
				if pos >= len(z) || z[pos] != 0 {
					return nil, 0, block{}, true
				}
				pos++
			}
			if pos+checkSize[check] > len(z) {
				return nil, 0, block{}, true
			}
			if !checkOK(check, out[dataStart:], z[pos:pos+checkSize[check]]) {
				return nil, 0, block{}, true
			}
			pos += checkSize[check]
			return out, pos, b, false

		case 0x01, 0x02: // uncompressed chunk
			if ctl == 0x01 {
				dictBase = len(out)
				st = nil // a dict reset makes the LZMA state's history moot; the next LZMA chunk must restate itself
			}
			if pos+2 > len(z) {
				return nil, 0, block{}, true
			}
			size := int(z[pos])<<8 | int(z[pos+1]) + 1
			pos += 2
			if pos+size > len(z) || len(out)+size > maxOutput {
				return nil, 0, block{}, true
			}
			out = append(out, z[pos:pos+size]...)
			pos += size

		default: // an LZMA chunk (ctl >= 0x80): bit 7 the flag, bits
			// 6-5 the reset mode (0 none, 1 dict reset, 2 state reset,
			// 3 state reset + new properties), bits 4-0 the high bits
			// of the unpacked size.  The 2-byte compressed size counts
			// the LZMA data only — the properties byte, when the reset
			// mode brings one, is extra.
			reset := int(ctl>>5) & 3
			if pos+4 > len(z) {
				return nil, 0, block{}, true
			}
			unpacked := (int(ctl&0x1F)<<16 | int(z[pos])<<8 | int(z[pos+1])) + 1
			comp := (int(z[pos+2])<<8 | int(z[pos+3])) + 1
			pos += 4
			hasProps := reset == 3
			if hasProps {
				comp++ // the properties byte rides ahead of the data
			}
			if pos+comp > len(z) {
				return nil, 0, block{}, true
			}
			data := z[pos : pos+comp]
			pos += comp
			switch reset {
			case 3: // new properties
				if len(data) < 1 || data[0] > 224 {
					return nil, 0, block{}, true
				}
				st = newLzmaState(data[0])
				data = data[1:]
			case 2: // state reset
				if st == nil {
					return nil, 0, block{}, true
				}
				st.resetProbs()
			case 1: // dictionary reset
				if st == nil {
					return nil, 0, block{}, true
				}
				dictBase = len(out)
			case 0:
				if st == nil {
					return nil, 0, block{}, true
				}
			}
			if len(out)+unpacked > maxOutput {
				return nil, 0, block{}, true
			}
			rc := newRangeDecoder(data)
			if !lzmaDecode(rc, st, &out, len(out)+unpacked, dictBase) {
				return nil, 0, block{}, true
			}
		}
	}
}

func checkOK(check int, data, want []byte) bool {
	switch check {
	case 0:
		return true
	case 1:
		return crc32Bytes(data) == le32(want)
	case 4:
		return crc64Bytes(data) == le64(want)
	case 10:
		sum := sha256.Sum256(data)
		return string(sum[:]) == string(want)
	}
	return false
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func le64(b []byte) uint64 {
	return uint64(le32(b)) | uint64(le32(b[4:]))<<32
}

// readVLI reads a variable-length integer (7 bits per byte,
// little-endian; the high bit continues).  Max 9 bytes.
func readVLI(z []byte, pos int) (int64, int, bool) {
	var v uint64
	for i := 0; i < 9; i++ {
		if pos+i >= len(z) {
			return 0, 0, false
		}
		b := z[pos+i]
		v |= uint64(b&0x7F) << uint(7*i)
		if b&0x80 == 0 {
			if v > 1<<62 {
				return 0, 0, false
			}
			return int64(v), pos + i + 1, true
		}
	}
	return 0, 0, false
}

// writeVLI appends a variable-length integer.
func writeVLI(dst []byte, v int64) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		dst = append(dst, b)
		if v == 0 {
			return dst
		}
	}
}

const (
	lzma2FilterID = 0x21
	defaultProps  = 0x5D // lc=3, lp=0, pb=2
	dictProp8MiB  = 22   // (2|0) << (11+11) = 8 MiB
)

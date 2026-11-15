/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

// DecodeLZMA2 decodes a bare LZMA2 chunk sequence — the form 7z
// stores — under a known total output size (folder sizes always are
// there).  prop is the one-byte dictionary-size property.  A final
// 0x00 chunk terminator may be present or absent; anything else left
// over is malformed.  nil means malformed.
func DecodeLZMA2(data []byte, prop byte, outSize int64) []byte {
	if prop > 40 {
		return nil
	}
	out := make([]byte, 0, outSize)
	pos := 0
	st := newLzmaState(0)
	dictBase := 0
	for len(out) < int(outSize) {
		if pos >= len(data) {
			return nil
		}
		ctl := data[pos]
		pos++
		switch {
		case ctl == 0x00:
			return nil // terminator before the declared size
		case ctl <= 0x02: // uncompressed chunk
			if ctl == 0x01 {
				dictBase, st = len(out), nil
			}
			if pos+2 > len(data) {
				return nil
			}
			size := int(data[pos])<<8 | int(data[pos+1]) + 1
			pos += 2
			if pos+size > len(data) || len(out)+size > int(outSize) {
				return nil
			}
			out = append(out, data[pos:pos+size]...)
			pos += size
		default: // an LZMA chunk (ctl >= 0x80)
			reset := int(ctl>>5) & 3
			if pos+4 > len(data) {
				return nil
			}
			unpacked := (int(ctl&0x1F)<<16 | int(data[pos])<<8 | int(data[pos+1])) + 1
			comp := (int(data[pos+2])<<8 | int(data[pos+3])) + 1
			pos += 4
			if reset == 3 {
				comp++ // the properties byte rides ahead of the data
			}
			if pos+comp > len(data) || len(out)+unpacked > int(outSize) {
				return nil
			}
			chunk := data[pos : pos+comp]
			pos += comp
			switch reset {
			case 3:
				if len(chunk) < 1 || chunk[0] > 224 {
					return nil
				}
				st = newLzmaState(chunk[0])
				chunk = chunk[1:]
			case 2:
				if st == nil {
					return nil
				}
				st.ResetProbs()
			case 1:
				if st == nil {
					return nil
				}
				dictBase = len(out)
			default:
				if st == nil {
					return nil
				}
			}
			rc := newRangeDecoder(chunk)
			if !lzmaDecode(rc, st, &out, len(out)+unpacked, dictBase) {
				return nil
			}
		}
	}
	// Bytes after the declared size are ignored: 7z's AES pads its
	// plaintext to the block size, so the chunk stream can trail into
	// decryptor padding.
	_ = pos
	return out
}

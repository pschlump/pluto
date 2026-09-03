/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import "encoding/binary"

// BitPos returns the position of the first bit equal to bit (0 or 1)
// inside the range [start, end] of buf, indexed in the chosen unit — the
// BITPOS command.  The returned position is always a bit offset from the
// start of buf, whatever the range unit.  found is false exactly where
// Redis replies -1.
//
// Range semantics are bitposCommand's: negative indexes measure from the
// end in the chosen unit, start clamps up to 0, end clamps down to the
// last element, and an empty range (start past end after clamping) is
// not-found.  When endGiven is false the range extends to the last
// element and the end argument is ignored — and the difference matters:
// looking for a 0 that is not in an explicitly-ended range is not-found,
// while without an explicit end the answer is the first bit just past
// the buffer (Redis treats the right side as zero-padded; e.g. an all-0xFF
// buffer with BitPos(b, 0, 0, 0, ByteUnit, false) reports len*8).  That
// past-the-end answer is returned with found true — render it as the
// integer Redis replies with.
//
// A nil buffer is a missing key — an infinite array of 0 bits — so bit 0
// reports position 0 and bit 1 reports not-found, regardless of the range
// arguments.  An empty non-nil buffer is an empty string: not-found for
// both bits.
//
// Panics (the package's whole panic contract) if bit is not 0 or 1, or if
// unit is neither ByteUnit nor BitUnit — no sane answer exists for either.
//
// Complexity is O(n) over the range, word-skipping (full 8-byte words
// that are all ones or all zeros are skipped, the redisBitpos fast path).
func BitPos(buf []byte, bit int, start, end int64, unit Unit, endGiven bool) (pos int64, found bool) {
	if bit != 0 && bit != 1 {
		panic("bitmap.BitPos: the bit argument must be 1 or 0")
	}
	checkUnit(unit, "BitPos")
	if buf == nil {
		// A missing key is an infinite zero array from our point of view.
		if bit == 1 {
			return -1, false
		}
		return 0, true
	}
	if !endGiven {
		// No explicit end: the range reaches the last element (the C
		// computes bit-end as totlen*8+7 and lets the clamp fold it back
		// to totlen-1 — the same destination).
		if unit == BitUnit {
			end = int64(len(buf))*8 - 1
		} else {
			end = int64(len(buf)) - 1
		}
	}
	totlen := int64(len(buf))
	if unit == BitUnit {
		totlen <<= 3
	}
	if start < 0 {
		start += totlen
	}
	if end < 0 {
		end += totlen
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end >= totlen {
		end = totlen - 1
	}
	var firstMask, lastMask byte
	if unit == BitUnit && start <= end {
		firstMask = 0xFF ^ (byte(0xFF) >> (start & 7))
		lastMask = 0xFF ^ (byte(0xFF) << (7 - (end & 7)))
		start >>= 3
		end >>= 3
	}
	if start > end {
		return -1, false
	}
	bytes := end - start + 1

	// First partial byte: force the out-of-range bits to the value we
	// are NOT looking for, so the one-byte scan cannot match them.
	var pos1 int64
	have := false
	if firstMask != 0 {
		tmp := buf[start]
		if bit == 1 {
			tmp &^= firstMask
		} else {
			tmp |= firstMask
		}
		if lastMask != 0 && bytes == 1 {
			if bit == 1 {
				tmp &^= lastMask
			} else {
				tmp |= lastMask
			}
		}
		pos1 = bitposByte(tmp, bit)
		if bytes == 1 || (pos1 != -1 && pos1 != 8) {
			have = true
		} else {
			start++
			bytes--
		}
	}
	if !have {
		// Middle: the full bytes between any partial edges.
		curbytes := bytes
		if lastMask != 0 {
			curbytes--
		}
		if curbytes > 0 {
			pos1 = bitposRange(buf[start:start+curbytes], bit)
			if bytes == curbytes || (pos1 != -1 && pos1 != int64(curbytes)<<3) {
				have = true
			} else {
				start += curbytes
				bytes -= curbytes
			}
		}
	}
	if !have {
		// Last partial byte, masked like the first.
		tmp := buf[end]
		if bit == 1 {
			tmp &^= lastMask
		} else {
			tmp |= lastMask
		}
		pos1 = bitposByte(tmp, bit)
	}
	pos = pos1

	// Looking for a 0 with an explicit end, the right of the range is
	// NOT zero-padded: "first bit past the range" means there is no 0
	// in the range at all.
	if endGiven && bit == 0 && pos == int64(bytes)<<3 {
		return -1, false
	}
	if pos != -1 {
		pos += start << 3
	}
	return pos, pos != -1
}

// bitposByte scans one byte MSB-first: the position of the first bit
// equal to bit, 8 when the byte holds no such bit (a zero bit is assumed
// past the byte), or -1 when bit is 1 and the byte holds no 1.
func bitposByte(b byte, bit int) int64 {
	for k := 7; k >= 0; k-- {
		if int((b>>k)&1) == bit {
			return int64(7 - k)
		}
	}
	if bit == 1 {
		return -1
	}
	return 8
}

// bitposRange scans s MSB-first, byte 0 bit 0 first.  Full words that are
// all ones (bit 0) or all zeros (bit 1) are skipped in one comparison
// each — the redisBitpos fast path.  Returns -1 when bit is 1 and no 1
// exists; when bit is 0 and no 0 exists it returns len(s)*8, the caller's
// "first bit past the range" signal.
func bitposRange(s []byte, bit int) int64 {
	skip := uint64(0)
	if bit == 0 {
		skip = ^uint64(0)
	}
	i := 0
	for ; i+8 <= len(s); i += 8 {
		if binary.LittleEndian.Uint64(s[i:]) != skip {
			break
		}
	}
	for j := i; j < len(s); j++ {
		b := s[j]
		if bit == 0 {
			if b == 0xFF {
				continue
			}
			for k := 7; k >= 0; k-- {
				if b&(1<<k) == 0 {
					return int64(j)*8 + int64(7-k)
				}
			}
		} else {
			if b == 0 {
				continue
			}
			for k := 7; k >= 0; k-- {
				if b&(1<<k) != 0 {
					return int64(j)*8 + int64(7-k)
				}
			}
		}
	}
	if bit == 1 {
		return -1
	}
	return int64(len(s)) * 8
}

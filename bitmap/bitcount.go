/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import (
	"encoding/binary"
	"math/bits"
)

// BitCount returns the number of set bits in the range [start, end] of
// buf, indexed in the chosen unit — the BITCOUNT command.  A nil buffer
// (a missing key) counts as 0.
//
// Range semantics, exactly bitcountCommand's: indexes may be negative,
// measured from the end in the chosen unit (-1 is the last byte or bit);
// start below the beginning clamps to 0 and end past the last element
// clamps to it; a start past the end (after clamping) counts 0.  With
// BitUnit a sub-byte range masks the partial first/last bytes — the bits
// outside [start, end] are not counted.  The whole buffer is
// BitCount(buf, 0, -1, ByteUnit).
//
// Complexity is O(n) over the range, word-wise (math/bits.OnesCount64
// over 8-byte words, four accumulators, the Go equivalent of Redis's
// popcount loop).
func BitCount(buf []byte, start, end int64, unit Unit) int64 {
	checkUnit(unit, "BitCount")
	if buf == nil {
		return 0
	}
	totlen := int64(len(buf))
	// Shortcut mirroring the C: both negative and inverted means empty.
	if start < 0 && end < 0 && start > end {
		return 0
	}
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
		// Bits of the first/last byte that fall outside the range,
		// subtracted after the popcount.  In-range bits of a byte are
		// 0xFF >> (start&7) from the top and 0xFF << (7-(end&7)).
		firstMask = 0xFF ^ (byte(0xFF) >> (start & 7))
		lastMask = 0xFF ^ (byte(0xFF) << (7 - (end & 7)))
		start >>= 3
		end >>= 3
	}
	if start > end {
		return 0
	}
	count := popcount(buf[start : end+1])
	if firstMask != 0 {
		count -= int64(bits.OnesCount8(buf[start] & firstMask))
	}
	if lastMask != 0 {
		count -= int64(bits.OnesCount8(buf[end] & lastMask))
	}
	return count
}

// popcount counts the set bits of b, 32 bytes per iteration through four
// independent accumulators (the dependency-breaking shape of Redis's
// redisPopcount) and the tail byte-wise.
func popcount(b []byte) int64 {
	var c0, c1, c2, c3 int
	for len(b) >= 32 {
		c0 += bits.OnesCount64(binary.LittleEndian.Uint64(b))
		c1 += bits.OnesCount64(binary.LittleEndian.Uint64(b[8:]))
		c2 += bits.OnesCount64(binary.LittleEndian.Uint64(b[16:]))
		c3 += bits.OnesCount64(binary.LittleEndian.Uint64(b[24:]))
		b = b[32:]
	}
	count := int64(c0 + c1 + c2 + c3)
	for len(b) >= 8 {
		count += int64(bits.OnesCount64(binary.LittleEndian.Uint64(b)))
		b = b[8:]
	}
	for _, x := range b {
		count += int64(bits.OnesCount8(x))
	}
	return count
}

// checkUnit rejects a Unit that is neither ByteUnit nor BitUnit — a range
// in an unknown unit has no interpretation at all (the one programmer
// error the read paths panic on; see the README's panic contract).
func checkUnit(unit Unit, method string) {
	if unit != ByteUnit && unit != BitUnit {
		panic("bitmap." + method + ": unit must be bitmap.ByteUnit or bitmap.BitUnit")
	}
}

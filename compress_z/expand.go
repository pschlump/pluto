/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z

// Expand decodes a .Z stream.  It returns nil for malformed input.
//
// The format as implemented by the historical compress(1) family
// (verified byte-exact against compress(1) output at every -b width):
//
//   - Codes are LSB-first, starting at 9 bits.  The width grows when
//     the free-slot counter passes maxcode, which is 2^n-1 — EXCEPT
//     that a growth landing exactly on maxbits clamps maxcode to
//     maxmaxcode instead.  The one visible consequence: at -b9, -b10
//     and -b11 the table fills while maxcode is still 2^maxbits-1, so
//     the width grows one bit PAST maxbits and stays there (a -b9
//     stream ends in 10-bit codes with values below 512).  This
//     overshoot is why the BSD tools reject their own -b9..-b11
//     output; pluto's decoder reads all widths 9..16.
//
//   - Codes are packed in groups of eight: a group of n-bit codes is
//     exactly 8*n bits.  A width change — and a CLEAR — takes effect
//     only at the next group boundary; the encoder zero-pads any
//     partial group.  Growth points are always naturally group-aligned
//     (they occur after 2^n-256 codes, a multiple of 8), so only
//     CLEARs observably pad.
//
//   - CLEAR (code 256, block mode) resets to a fresh dictionary:
//     widths restart at 9 bits, free restarts at 257 with the
//     previous-code state cleared, so the next code is a literal with
//     no table add (the classic adaptive reset when the compression
//     ratio drops at a checkpoint).
//
// A code equal to the free-slot counter is the KwKwK special case (the
// entry being defined right now); a code above it — or a non-literal
// first code — is malformed.
//
// There is no end-of-stream code: the encoder simply stops, the final
// partial byte zero-padded.  Trailing bits too few to form a code are
// a clean end, matching uncompress(1).
func Expand(z []byte) []byte {
	if len(z) < 3 || z[0] != magic0 || z[1] != magic1 {
		return nil
	}
	maxBits := int(z[2] & bitMask)
	block := z[2]&blockMode != 0
	if maxBits < 9 || maxBits > 16 {
		return nil
	}
	maxmax := uint32(1) << maxBits

	r := &bitReader{buf: z[3:]}
	prefix := make([]uint32, maxmax)
	suffix := make([]byte, maxmax)
	stack := make([]byte, 0, 1024)
	out := make([]byte, 0, len(z)*3+16)

	nBits := uint(initBits)
	maxcode := uint32(1)<<initBits - 1 // no init clamp — see the overshoot note
	free := uint32(firstEntry)
	if !block {
		free = 256
	}
	oldcode := int64(-1)
	finchar := byte(0)

	for {
		code, ok := r.readCode(nBits)
		if !ok {
			return out // clean end: stream just stops
		}
		if block && code == clearCode {
			// Fresh dictionary at the next group boundary.
			r.realign(nBits)
			nBits = initBits
			maxcode = 1<<initBits - 1
			free = firstEntry
			oldcode = -1
			continue
		}
		incode := code
		stack = stack[:0]
		if code >= free {
			if code > free || oldcode < 0 {
				return nil // beyond the live table, or a non-literal first code
			}
			stack = append(stack, finchar) // KwKwK: finish with the prior byte
			code = uint32(oldcode)
		}
		for code >= 256 {
			if len(stack) >= int(maxmax) {
				return nil // cyclic chain — only malformed input gets here
			}
			stack = append(stack, suffix[code])
			code = prefix[code]
		}
		finchar = byte(code)
		stack = append(stack, finchar)
		for i := len(stack) - 1; i >= 0; i-- {
			out = append(out, stack[i])
		}
		if oldcode >= 0 && free < maxmax {
			prefix[free] = uint32(oldcode)
			suffix[free] = finchar
			free++
		}
		if free > maxcode {
			r.realign(nBits) // growth lands on the next group boundary
			nBits++
			if nBits == uint(maxBits) {
				maxcode = maxmax
			} else {
				maxcode = 1<<nBits - 1
			}
		}
		oldcode = int64(incode)
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2

// Expand decodes a bzip2 stream.  It returns nil for malformed
// input: a bad header, a bad block or end-of-stream magic, the
// deprecated randomized bit, out-of-range tree counts, code lengths
// or selectors, an over-long zero run, a block that overruns its
// declared size, a mismatched block or stream CRC, or trailing
// garbage after the final member.  Concatenated members (bzip2
// streams glued together, the `bzip2` of two files) are decoded to
// their concatenation, matching bzip2(1) -d.
func Expand(bz []byte) []byte {
	out := make([]byte, 0, len(bz)*4+16)
	pos := 0
	for {
		if pos+4 > len(bz) {
			return nil
		}
		if bz[pos] != 'B' || bz[pos+1] != 'Z' || bz[pos+2] != 'h' {
			return nil
		}
		level := bz[pos+3]
		if level < '1' || level > '9' {
			return nil
		}
		blockSize := 100000 * int(level-'0')
		pos += 4

		r := &msbReader{buf: bz[pos:]}
		fileCRC := uint32(0)
		done := false
		for !done {
			magic, ok := r.readBits(48)
			if !ok {
				return nil
			}
			switch magic {
			case blockMagic:
				block, crc, ok := decodeBlock(r, blockSize)
				if !ok {
					return nil
				}
				fileCRC = combineCRC(fileCRC, crc)
				out = append(out, block...)
			case finalMagic:
				want, ok := r.readBits(32)
				if !ok || fileCRC != uint32(want) {
					return nil
				}
				done = true
			default:
				return nil
			}
		}

		// Skip the zero padding of the final partial byte, then look
		// for another member (BZh...) — or a clean end of input.
		r.alignToByte()
		pos += r.bytePos()
		if pos == len(bz) {
			return out
		}
		if pos+4 > len(bz) || bz[pos] != 'B' || bz[pos+1] != 'Z' {
			return nil
		}
	}
}

// decodeBlock decodes one block (magic already consumed).  It
// returns the block's plaintext and the transmitted block CRC for
// the stream-CRC combine.
func decodeBlock(r *msbReader, blockSize int) ([]byte, uint32, bool) {
	want, ok := r.readBits(32)
	if !ok {
		return nil, 0, false
	}
	wantCRC := uint32(want)
	randomized, ok := r.readBit()
	if !ok {
		return nil, 0, false
	}
	if randomized != 0 {
		return nil, 0, false // deprecated randomized files
	}
	origPtr, ok := r.readBits(24)
	if !ok {
		return nil, 0, false
	}

	// The two-level 16x16 symbol bitmap.
	rangeBits, ok := r.readBits(16)
	if !ok {
		return nil, 0, false
	}
	var present [256]bool
	nUsed := 0
	for rg := uint(0); rg < 16; rg++ {
		if rangeBits&(1<<(15-rg)) == 0 {
			continue
		}
		bits, ok := r.readBits(16)
		if !ok {
			return nil, 0, false
		}
		for s := uint(0); s < 16; s++ {
			if bits&(1<<(15-s)) != 0 {
				present[16*rg+s] = true
				nUsed++
			}
		}
	}
	if nUsed == 0 {
		return nil, 0, false // there must be an EOF symbol
	}

	nGroups, ok := r.readBits(3)
	if !ok || nGroups < 2 || nGroups > 6 {
		return nil, 0, false
	}
	nSelectors, ok := r.readBits(15)
	if !ok {
		return nil, 0, false
	}
	treeIdx := make([]uint8, nSelectors)
	treeMTF := make([]byte, nGroups)
	for i := range treeMTF {
		treeMTF[i] = byte(i)
	}
	for i := range treeIdx {
		c := 0
		for {
			b, ok := r.readBit()
			if !ok {
				return nil, 0, false
			}
			if b == 0 {
				break
			}
			c++
			if c >= int(nGroups) {
				return nil, 0, false // tree index too large
			}
		}
		treeIdx[i] = mtfDecodeByte(treeMTF, c)
	}
	if nSelectors == 0 {
		return nil, 0, false
	}

	// Code lengths for each tree, delta-coded from a 5-bit first
	// value: "1 0" increments, "1 1" decrements, "0" ends a symbol.
	alpha := nUsed + 2
	lengths := make([]uint8, alpha)
	trees := make([]*decTree, nGroups)
	for g := range trees {
		length, ok := r.readBits(5)
		if !ok {
			return nil, 0, false
		}
		for j := 0; j < alpha; j++ {
			for {
				if length < 1 || length > 20 {
					return nil, 0, false
				}
				b, ok := r.readBit()
				if !ok {
					return nil, 0, false
				}
				if b == 0 {
					break
				}
				d, ok := r.readBit()
				if !ok {
					return nil, 0, false
				}
				if d == 1 {
					length--
				} else {
					length++
				}
			}
			lengths[j] = uint8(length)
		}
		trees[g], ok = buildDecTree(lengths)
		if !ok {
			return nil, 0, false
		}
	}

	// MTF list: the present bytes in ascending order.
	symbols := make([]byte, 0, nUsed)
	for b := 0; b < 256; b++ {
		if present[b] {
			symbols = append(symbols, byte(b))
		}
	}
	mtfList := make([]byte, len(symbols))
	copy(mtfList, symbols)

	// Entropy decode: Huffman symbols, zero-run (RUNA/RUNB) expansion,
	// inverse MTF — building the BWT last column in tt and the counts
	// in c on the fly.
	var c [256]uint
	tt := make([]uint32, 0, blockSize)
	tree := trees[treeIdx[0]]
	selIdx, decoded := 1, 0
	repeat, power := 0, 0
	for {
		if decoded == maxSymsPerSelector {
			if selIdx >= len(treeIdx) {
				return nil, 0, false // insufficient selectors
			}
			tree = trees[treeIdx[selIdx]]
			selIdx++
			decoded = 0
		}
		v, ok := tree.decodeSym(r)
		if !ok {
			return nil, 0, false
		}
		decoded++
		if v < 2 {
			// RUNA or RUNB: bijective base-2 zero-run digits.
			if repeat == 0 {
				power = 1
			}
			repeat += power << v
			power <<= 1
			if repeat > 2*1024*1024 {
				return nil, 0, false // repeat count too large
			}
			continue
		}
		if repeat > 0 {
			if repeat > blockSize-len(tt) {
				return nil, 0, false // repeats past end of block
			}
			for i := 0; i < repeat; i++ {
				b := mtfList[0]
				tt = append(tt, uint32(b))
				c[b]++
			}
			repeat = 0
		}
		if int(v) == alpha-1 {
			break // EOB
		}
		if len(tt) >= blockSize {
			return nil, 0, false // data exceeds block size
		}
		b := mtfDecodeByte(mtfList, int(v-1))
		tt = append(tt, uint32(b))
		c[b]++
	}
	if int(origPtr) >= len(tt) {
		return nil, 0, false
	}

	// Inverse BWT (the reference single-array construction): turn tt
	// into a linked walk, then follow it, applying the RLE1 state
	// machine as bytes come out (four equal output bytes are followed
	// by a count byte of extra repeats).
	sum := 0
	for i := 0; i < 256; i++ {
		sum += int(c[i])
		c[i] = uint(sum - int(c[i]))
	}
	for i := range tt {
		b := tt[i] & 0xff
		tt[c[b]] |= uint32(i) << 8
		c[b]++
	}

	block := make([]byte, 0, len(tt))
	tPos := tt[origPtr] >> 8
	lastByte, byteRepeats, repeats := -1, 0, 0
	for i := 0; i < len(tt); i++ {
		tPos = tt[tPos]
		b := byte(tPos)
		tPos >>= 8
		for repeats > 0 {
			block = append(block, byte(lastByte))
			repeats--
			if repeats == 0 {
				lastByte = -1
			}
		}
		if byteRepeats == 3 {
			repeats = int(b)
			byteRepeats = 0
			continue
		}
		if lastByte == int(b) {
			byteRepeats++
		} else {
			byteRepeats = 0
		}
		lastByte = int(b)
		block = append(block, b)
	}
	for repeats > 0 {
		block = append(block, byte(lastByte))
		repeats--
	}

	if updateCRC(0, block) != wantCRC {
		return nil, 0, false
	}
	return block, wantCRC, true
}

// mtfDecodeByte moves list[pos] to the front and returns it.
func mtfDecodeByte(list []byte, pos int) byte {
	b := list[pos]
	copy(list[1:pos+1], list[:pos])
	list[0] = b
	return b
}

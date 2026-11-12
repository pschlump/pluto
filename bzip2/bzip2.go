/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bzip2 implements the bzip2 compression format (BWT + MTF +
// RLE + multi-table Huffman) over byte slices: Compress produces a
// stream that the standard library's compress/bzip2 (and bzip2(1))
// can read, and Expand reads streams written by bzip2(1).
//
// Complexity is O(n log² n) per block for the Burrows-Wheeler sort
// (cyclic suffix doubling) and O(n) for everything else.
//
// Not safe for concurrent mutation — the two whole-buffer functions
// are pure and the streaming Reader/Writer of stream.go carry their
// own per-stream state, so concurrent use of separate streams is
// fine and there is no _ts twin (the lzw/deflate precedent).
package bzip2

import (
	"cmp"
	"fmt"
	"slices"
)

// Magic values and format constants.
const (
	blockMagic         = 0x314159265359 // 48 bits — pi digits, start of block
	finalMagic         = 0x177245385090 // 48 bits — sqrt(2) digits, end of stream
	maxSymsPerSelector = 50
)

// Compress encodes data as a bzip2 stream at level 9 (the default of
// bzip2(1): 900k blocks).  It never returns nil and never panics.
func Compress(data []byte) []byte {
	return CompressLevel(data, 9)
}

// CompressLevel encodes data as a bzip2 stream with the given block
// size level 1..9 (bzip2's -1..-9: blocks of 100k..900k bytes).  The
// level changes only the block size — the codec is identical.
//
// Panics if level is outside [1, 9] — the fix is to pass a value in
// that range (the format defines no other levels).
func CompressLevel(data []byte, level int) []byte {
	if level < 1 || level > 9 {
		panic(fmt.Sprintf("bzip2.CompressLevel: level %d out of range, want 1..9", level))
	}
	blockSize := 100000 * level

	w := &msbWriter{}
	w.buf = append(w.buf, 'B', 'Z', 'h', byte('0'+level))

	fileCRC := uint32(0)
	for pos := 0; pos < len(data); {
		rle, next := rleFill(data[pos:], blockSize)
		blockCRC := updateCRC(0, data[pos:pos+next])
		encodeBlock(rle, blockCRC, w)
		fileCRC = combineCRC(fileCRC, blockCRC)
		pos += next
	}

	w.writeBits(finalMagic, 48)
	w.writeBits(uint64(fileCRC), 32)
	return w.bytes()
}

// rleFill applies bzip2's initial run-length encoding (runs of 4..259
// equal bytes become 4 copies plus a count byte; runs are chunked at
// 259 and shorter tails stay literal) and fills rle up to blockSize
// bytes, returning the RLE1 form of data[:consumed].
func rleFill(data []byte, blockSize int) (rle []byte, consumed int) {
	rle = make([]byte, 0, blockSize)
	i := 0
	for i < len(data) && len(rle) < blockSize {
		b := data[i]
		j := i + 1
		for j < len(data) && data[j] == b {
			j++
		}
		run := j - i
		for run > 0 {
			if run >= 4 {
				if len(rle)+5 > blockSize {
					return rle, i
				}
				chunk := min(run, 259)
				rle = append(rle, b, b, b, b, byte(chunk-4))
				i += chunk
				run -= chunk
			} else {
				if len(rle)+run > blockSize {
					return rle, i
				}
				for ; run > 0; run-- {
					rle = append(rle, b)
					i++
				}
			}
		}
	}
	return rle, i
}

// sortRotations returns the cyclic rotations of s sorted by content,
// ties (equal rotations, possible for periodic data) broken by
// starting position — the order the inverse BWT in the reference
// decoder reconstructs.  Cyclic suffix doubling, O(n log² n).
func sortRotations(s []byte) []int32 {
	n := len(s)
	idx := make([]int32, n)
	rank := make([]int32, n)
	newRank := make([]int32, n)
	for i := range idx {
		idx[i] = int32(i)
		rank[i] = int32(s[i])
	}
	key2 := func(i, k int) int32 { return rank[(i+k)%n] }
	for k := 1; n > 1; k *= 2 {
		slices.SortFunc(idx, func(a, b int32) int {
			if rank[a] != rank[b] {
				return cmp.Compare(rank[a], rank[b])
			}
			ra, rb := key2(int(a), k), key2(int(b), k)
			if ra != rb {
				return cmp.Compare(ra, rb)
			}
			return cmp.Compare(a, b)
		})
		newRank[idx[0]] = 0
		r := int32(0)
		for i := 1; i < n; i++ {
			prev, cur := int(idx[i-1]), int(idx[i])
			if rank[prev] != rank[cur] || key2(prev, k) != key2(cur, k) {
				r++
			}
			newRank[cur] = r
		}
		rank, newRank = newRank, rank
		if int(r)+1 == n || k >= n {
			break
		}
	}
	return idx
}

// bwt returns the last column of the sorted-rotation matrix and the
// row index of the original string.
func bwt(s []byte) (last []byte, origPtr int) {
	n := len(s)
	idx := sortRotations(s)
	last = make([]byte, n)
	for i, start := range idx {
		last[i] = s[(int(start)+n-1)%n]
		if start == 0 {
			origPtr = i
		}
	}
	return last, origPtr
}

// encodeBlock emits one block for the RLE1 data rle.  blockCRC is
// the CRC of the block's pre-RLE1 input, computed by the caller.
func encodeBlock(rle []byte, blockCRC uint32, w *msbWriter) {
	n := len(rle)
	last, origPtr := bwt(rle)

	// The distinct bytes of the block, ascending — the initial MTF
	// list and the symbol bitmap.
	var present [256]bool
	for _, b := range rle {
		present[b] = true
	}
	symbols := make([]byte, 0, 256)
	for b := 0; b < 256; b++ {
		if present[b] {
			symbols = append(symbols, byte(b))
		}
	}
	nUsed := len(symbols)

	// MTF + zero-run (RLE2) encoding.  Symbol values: 0 RUNA, 1 RUNB,
	// v = MTF position + 1 for positions >= 1, EOB = nUsed+1.
	list := slices.Clone(symbols)
	syms := make([]uint16, 0, n+8)
	zeroRun := 0
	for _, b := range last {
		pos := 0
		for list[pos] != b {
			pos++
		}
		if pos == 0 {
			zeroRun++
			continue
		}
		if zeroRun > 0 {
			emitZeroRun(&syms, zeroRun)
			zeroRun = 0
		}
		syms = append(syms, uint16(pos+1))
		copy(list[1:pos+1], list[:pos])
		list[0] = b
	}
	if zeroRun > 0 {
		emitZeroRun(&syms, zeroRun)
	}
	syms = append(syms, uint16(nUsed+1)) // EOB

	// One Huffman table, transmitted twice (the format demands 2..6
	// groups; two identical tables with all-zero selectors is the
	// smallest legal shape).
	alpha := nUsed + 2
	freqs := make([]int, alpha)
	for _, v := range syms {
		freqs[v]++
	}
	lengths := buildLengths(freqs, 20)
	codes := canonicalCodes(lengths)

	w.writeBits(blockMagic, 48)
	w.writeBits(uint64(blockCRC), 32)
	w.writeBits(0, 1) // not randomized (deprecated)
	w.writeBits(uint64(origPtr), 24)

	// Symbol bitmap: 16 range bits, then 16 bits per present range.
	var rangeBits uint32
	for _, b := range symbols {
		rangeBits |= 1 << (15 - b/16)
	}
	w.writeBits(uint64(rangeBits), 16)
	for r := 0; r < 16; r++ {
		if rangeBits&(1<<(15-r)) == 0 {
			continue
		}
		var bits uint32
		for s := 0; s < 16; s++ {
			if present[16*r+s] {
				bits |= 1 << (15 - s)
			}
		}
		w.writeBits(uint64(bits), 16)
	}

	nGroups := 2
	nSelectors := (len(syms) + maxSymsPerSelector - 1) / maxSymsPerSelector
	w.writeBits(uint64(nGroups), 3)
	w.writeBits(uint64(nSelectors), 15)
	// All selectors 0: unary position 0 in the MTF tree list is one
	// 0 bit each.
	for i := 0; i < nSelectors; i++ {
		w.writeBits(0, 1)
	}

	// Code lengths, delta-coded from a 5-bit first value: "1 0"
	// increments, "1 1" decrements, "0" ends each symbol.
	for g := 0; g < nGroups; g++ {
		w.writeBits(uint64(lengths[0]), 5)
		prev := int(lengths[0])
		for _, l := range lengths {
			for d := int(l) - prev; d > 0; d-- {
				w.writeBits(0b10, 2)
			}
			for d := int(l) - prev; d < 0; d++ {
				w.writeBits(0b11, 2)
			}
			w.writeBits(0, 1)
			prev = int(l)
		}
	}

	for _, v := range syms {
		w.writeBits(uint64(codes[v]), uint(lengths[v]))
	}
}

// emitZeroRun encodes a run of n references to the MTF front symbol
// in bzip2's bijective base-2: each RUNA digit contributes 1 at its
// power, each RUNB 2.
func emitZeroRun(syms *[]uint16, n int) {
	for n > 0 {
		if n&1 == 1 {
			*syms = append(*syms, 0) // RUNA
			n = (n - 1) >> 1
		} else {
			*syms = append(*syms, 1) // RUNB
			n = (n - 2) >> 1
		}
	}
}

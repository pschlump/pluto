/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lz4 implements the LZ4 frame format (v1.6.5) over byte
// slices: Compress produces a frame that lz4(1) reads, and Expand
// reads frames written by lz4(1) — magic 184D2204, independent
// blocks, XXH32 header and content checksums, skippable frames and
// concatenated frames on decode.  The block codec underneath is the
// spec's sequence format (token / literals / offset / match) with
// hash-chain match search.
//
// Complexity is O(n) for both directions amortized (the encoder's
// match chains are bounded, like lz4's own acceleration levels).
//
// Not safe for concurrent mutation — the package is stateless (two
// pure functions), so concurrent use is fine and there is no _ts
// twin (the lzw/deflate precedent).
package lz4

// Frame-format constants (the v1.6.5 spec).
const (
	frameMagic    = 0x184D2204
	skipMagicHi   = 0x184D2A50 // 0x184D2A50..0x184D2A5F skip frames
	flgVersion    = 0x40       // version 01 in bits 7-6
	flgIndep      = 0x20       // block independence
	flgContentSum = 0x04       // content checksum present
	bdBlockBits   = 0x70       // 4 MB max block size in bits 6-4
	defaultBlock  = 4 << 20
)

// Compress encodes data as an LZ4 frame: 4 MB independent blocks,
// XXH32 header checksum, XXH32 content checksum.  It never returns
// nil and never panics.
func Compress(data []byte) []byte {
	out := make([]byte, 0, len(data)/2+32)
	out = append(out, 0x04, 0x22, 0x4D, 0x18)
	out = append(out, flgVersion|flgIndep|flgContentSum, bdBlockBits)
	// Header checksum: XXH32 over FLG+BD (no magic), second byte, always written.
	out = append(out, byte(xxh32(out[4:])>>8))

	for pos := 0; pos < len(data); pos += defaultBlock {
		end := min(pos+defaultBlock, len(data))
		chunk := data[pos:end]
		block := compressBlock(chunk)
		if len(block) < len(chunk) {
			out = appendLE32(out, uint32(len(block)))
			out = append(out, block...)
		} else {
			// No gain (or incompressible): store the block raw,
			// flagged by bit 31 of the size word.
			out = appendLE32(out, uint32(len(chunk))|0x80000000)
			out = append(out, chunk...)
		}
	}

	out = appendLE32(out, 0) // EndMark
	out = appendLE32(out, xxh32(data))
	return out
}

// compressBlock encodes src as one independent LZ4 block.  The last
// sequence is literals-only and matches never start within the final
// 12 bytes, per the spec's encoder constraints.
func compressBlock(src []byte) []byte {
	const (
		minMatch   = 4
		mfLimit    = 12 // no match may start in the last 12 bytes
		lastLits   = 5  // the final 5 bytes are always literals
		maxOffset  = 65535
		tableBits  = 16
		tableSize  = 1 << tableBits
		chainTries = 16 // match candidates examined per position
	)
	n := len(src)
	if n == 0 {
		return nil
	}
	var table [tableSize]int32
	for i := range table {
		table[i] = -1
	}

	out := make([]byte, 0, n+n/8+16)
	hash := func(i int) uint32 { return (readLE32(src[i:]) * 2654435761 >> (32 - tableBits)) & (tableSize - 1) }

	anchor := 0
	i := 0
	for i+mfLimit <= n {
		h := hash(i)
		bestLen, bestOff := 0, 0
		cand := table[h]
		tries := chainTries
		for cand >= 0 && tries > 0 && i-int(cand) <= maxOffset {
			c := int(cand)
			if src[c+bestLen] == src[i+bestLen] {
				l := 0
				for i+l < n-mfLimit+lastLits && src[c+l] == src[i+l] {
					l++
				}
				if l > bestLen {
					bestLen, bestOff = l, i-int(cand)
					if bestLen >= 64 {
						break // long enough
					}
				}
			}
			next := table[(h+1)&(tableSize-1)] // one-step chain
			if next >= cand {
				break
			}
			cand = next
			tries--
		}
		_ = h
		if bestLen < minMatch {
			table[h] = int32(i)
			i++
			continue
		}
		// Emit one sequence: literals [anchor, i), match at i.
		litLen := i - anchor
		token := len(out)
		out = append(out, 0)
		if litLen >= 15 {
			out[token] |= 0xF0
			r := litLen - 15
			for r >= 255 {
				out = append(out, 255)
				r -= 255
			}
			out = append(out, byte(r))
		} else {
			out[token] |= byte(litLen) << 4
		}
		out = append(out, src[anchor:i]...)

		// Wire order in a sequence: token, [extra litLen], literals,
		// offset (2 bytes LE), [extra matchLen] — the offset comes
		// BEFORE the extra match-length bytes.
		out = append(out, byte(bestOff), byte(bestOff>>8))
		matchLen := bestLen - minMatch
		if matchLen >= 15 {
			out[token] |= 0x0F
			r := matchLen - 15
			for r >= 255 {
				out = append(out, 255)
				r -= 255
			}
			out = append(out, byte(r))
		} else {
			out[token] |= byte(matchLen)
		}

		// Insert positions of the match into the hash table so later
		// matches can find them.
		end := i + bestLen
		for j := i; j < end; j++ {
			if j+4 <= n {
				table[hash(j)] = int32(j)
			}
		}
		i = end
		anchor = i
	}

	// Final literals-only sequence.
	litLen := n - anchor
	token := len(out)
	out = append(out, 0)
	if litLen >= 15 {
		out[token] |= 0xF0
		r := litLen - 15
		for r >= 255 {
			out = append(out, 255)
			r -= 255
		}
		out = append(out, byte(r))
	} else {
		out[token] |= byte(litLen) << 4
	}
	out = append(out, src[anchor:]...)
	return out
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

// The LZMA encoder: the range encoder (the decoder's mirror image —
// same 11-bit probabilities, same adaptation) plus a greedy
// hash-chain match finder with one-position lookahead.  Symbols are
// literals and simple matches (repeated distances are never emitted,
// which every decoder accepts and only costs some ratio); the model
// state transitions mirror the decoder's exactly, since its literal
// decoding keys off them.

// rcEncoder is the binary range encoder.
type rcEncoder struct {
	out   []byte
	low   uint64
	rng   uint32
	cache byte
	csz   int64
}

func newRcEncoder() *rcEncoder {
	return &rcEncoder{rng: 0xFFFFFFFF, csz: 1}
}

// shiftLow emits buffered bytes, propagating carries.
func (e *rcEncoder) shiftLow() {
	if uint32(e.low) < 0xFF000000 || e.low>>32 != 0 {
		temp := e.cache
		carry := byte(e.low >> 32)
		for e.csz > 0 {
			e.out = append(e.out, temp+carry) // buffered 0xFF + carry wraps to 0x00: the propagation
			temp = 0xFF
			e.csz--
		}
		e.cache = byte(e.low >> 24)
	}
	e.csz++
	e.low = uint64(uint32(e.low) << 8)
}

func (e *rcEncoder) normalize() {
	for e.rng < 1<<24 {
		e.rng <<= 8
		e.shiftLow()
	}
}

// bit encodes one bit through probability slot p, updating the model
// exactly as the decoder does.
func (e *rcEncoder) bit(p []uint16, i int, v int) {
	prob := p[i]
	bound := (e.rng >> numBitLevels) * uint32(prob)
	if v == 0 {
		e.rng = bound
		p[i] = prob + uint16((1<<numBitLevels-prob)>>5)
	} else {
		e.low += uint64(bound)
		e.rng -= bound
		p[i] = prob - uint16(prob>>5)
	}
	e.normalize()
}

// direct encodes n fixed-probability bits.
func (e *rcEncoder) direct(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		e.rng >>= 1
		if v>>uint(i)&1 != 0 {
			e.low += uint64(e.rng)
		}
		e.normalize()
	}
}

// tree encodes an n-bit symbol MSB-first through a tree from index 1.
func (e *rcEncoder) tree(p []uint16, n int, v int) {
	m := 1
	for i := n - 1; i >= 0; i-- {
		b := v >> uint(i) & 1
		e.bit(p, m, b)
		m = m<<1 | b
	}
}

// reverse encodes v's low n bits LSB-first through a tree at offset off.
func (e *rcEncoder) reverse(p []uint16, off, n int, v uint32) {
	m := 1
	for i := 0; i < n; i++ {
		b := int(v >> uint(i) & 1)
		e.bit(p, off+m, b)
		m = m<<1 | b
	}
}

// flush writes the five closing bytes.
func (e *rcEncoder) flush() {
	for i := 0; i < 5; i++ {
		e.shiftLow()
	}
}

// encodeLen writes a match length through one of the length coders.
func (e *rcEncoder) encodeLen(choice *[2]uint16, low, mid [][numLenProbs]uint16, high []uint16, posState, length int) {
	length -= matchMinLen
	if length < 8 {
		e.bit(choice[:], 0, 0)
		e.tree(low[posState][:], 3, length)
		return
	}
	e.bit(choice[:], 0, 1)
	if length < 16 {
		e.bit(choice[:], 1, 0)
		e.tree(mid[posState][:], 3, length-8)
		return
	}
	e.bit(choice[:], 1, 1)
	e.tree(high, highLenBits, length-16)
}

// distSlot returns the 6-bit slot, the slot's base distance, and the
// number of footer bits, for a 0-based distance.
func distSlot(dist uint32) (slot int, base uint32, n int) {
	if dist < startPosModel {
		return int(dist), 0, 0
	}
	hb := 31 - leadingZeros(dist)
	slot = hb*2 + int(dist>>uint(hb-1)&1)
	n = slot>>1 - 1
	base = (2 | uint32(slot&1)) << uint(n)
	return slot, base, n
}

func leadingZeros(v uint32) int {
	n := 0
	for i := 31; i >= 0 && v>>uint(i)&1 == 0; i-- {
		n++
	}
	return n
}

// posSlotTree encodes the distance slot with the length context the
// decoder keys on.
func posSlotTree(e *rcEncoder, st *lzmaState, length, slot int) {
	ctx := length - matchMinLen
	if ctx >= numLenToPos {
		ctx = numLenToPos - 1
	}
	m := 1
	for i := 5; i >= 0; i-- {
		b := slot >> uint(i) & 1
		e.bit(st.posSlot, ctx<<6+m, b)
		m = m<<1 | b
	}
}

// chunkMaxUnpacked is how much data one LZMA chunk carries.  The
// format ceiling is 2 MiB minus one, but a chunk's compressed size is
// a 16-bit field — cutting at 64 KiB keeps a compressed chunk under
// it (incompressible cuts fall back to uncompressed chunks anyway).
const chunkMaxUnpacked = 1 << 16

// encodeLZMA2Chunks appends src as LZMA2 chunks: each cut of at most
// chunkMaxUnpacked bytes becomes one LZMA chunk (falling back to an
// uncompressed chunk when compression does not pay), with the
// dictionary continuing across chunks.  fresh marks the block's first
// chunk (full reset); the returned int is the total chunk bytes.
func encodeLZMA2Chunks(dst []byte, out, src []byte) []byte {
	fresh := true
	var st *lzmaState
	for len(src) > 0 {
		n := len(src)
		if n > chunkMaxUnpacked {
			n = chunkMaxUnpacked
		}
		cut := src[:n]
		var enc []byte
		enc, st = encodeLZMAChunk(nil, out, cut, st, 0)
		if len(enc)+6 < n {
			ctl := byte(0x80)
			if fresh {
				ctl = 0xE0 // state reset + new properties
			}
			unpacked := n - 1
			compField := len(enc) - 1 // the LZMA data length minus one; the properties byte is extra
			dst = append(dst, ctl|byte(unpacked>>16), byte(unpacked>>8), byte(unpacked), byte(compField>>8), byte(compField))
			if fresh {
				dst = append(dst, defaultProps)
			}
			dst = append(dst, enc...)
		} else {
			dst = lzma2Uncompressed(dst, cut, fresh)
		}
		out = append(out, cut...)
		src = src[n:]
		fresh = false
	}
	return dst
}

// encodeLZMAChunk encodes src into the LZMA data of one chunk against
// the history in out (the dictionary so far, which the chunk may
// reference for distances).  st carries the probability model across
// chunks (nil starts fresh); base is the absolute stream position of
// out[0] so position contexts line up after window trims.
func encodeLZMAChunk(dst, out, src []byte, st *lzmaState, base int64) ([]byte, *lzmaState) {
	if st == nil {
		st = newLzmaState(defaultProps)
	}
	e := newRcEncoder()
	posStateMask := 1<<uint(st.pb) - 1

	// The match finder: a 4-byte hash over the dictionary tail plus
	// the new data, chaining through previous positions.
	const hashBits = 16
	head := make([]int32, 1<<hashBits)
	for i := range head {
		head[i] = -1
	}
	full := make([]byte, 0, len(out)+len(src))
	full = append(full, out...)
	full = append(full, src...)
	window := len(full) - 1<<22 // chains stop at a 4 MiB history
	if window < 0 {
		window = 0
	}
	prev := make([]int32, len(full))
	for i := range prev {
		prev[i] = -1
	}
	hashAt := func(i int) uint32 {
		v := uint32(full[i]) | uint32(full[i+1])<<8 | uint32(full[i+2])<<16 | uint32(full[i+3])<<24
		return v * 2654435761 >> (32 - hashBits)
	}
	if len(out) >= 4 {
		for i := len(out) - 1<<22; i+4 <= len(out); i++ {
			if i < 0 {
				i = -1
				continue
			}
			h := hashAt(i)
			prev[i] = head[h]
			head[h] = int32(i)
		}
	}

	insert := func(from, to int) {
		// full already holds the whole cut; positions below the
		// encoder's cursor are fair game as candidates the moment
		// all four of their bytes exist (the cand >= i walk guard
		// keeps future positions out of matches).
		for j := from; j < to && j+4 <= len(full); j++ {
			if j < 0 {
				continue
			}
			h := hashAt(j)
			prev[j] = head[h]
			head[h] = int32(j)
		}
	}

	bestAt := func(i int) (off, length int) {
		maxLen := len(full) - i
		if maxLen > 273 {
			maxLen = 273
		}
		if maxLen < 4 {
			return 0, 0
		}
		cand := int(head[hashAt(i)])
		best, bestOff := 0, 0
		for tries := 0; cand >= window && tries < 16; tries++ {
			if cand >= i { // seeded ahead of the encoder: not a backward distance
				cand = int(prev[cand])
				continue
			}
			if cand+best < len(full) && i+best < len(full) && full[cand+best] == full[i+best] {
				l := 0
				for l < maxLen && full[cand+l] == full[i+l] {
					l++
				}
				if l > best {
					best, bestOff = l, i-cand-1 // the 0-based LZMA distance
					if best >= 128 {
						break
					}
				}
			}
			cand = int(prev[cand])
		}
		if best >= 4 || best == 3 && bestOff < 1<<14 {
			return bestOff, best
		}
		return 0, 0
	}

	i := len(out)
	abs := base + int64(i) // absolute position for the position contexts
	end := len(full)
	nextInsert := i
	for i < end {
		off, length := bestAt(i)
		if length > 0 && i+1 < end {
			if _, l2 := bestAt(i + 1); l2 > length {
				length = 0 // one-position lookahead wins
			}
		}
		if length == 0 {
			c := full[i]
			e.bit(st.isMatch, st.state<<uint(st.pb)+int(abs&int64(posStateMask)), 0)
			var prevB byte
			if i > 0 {
				prevB = full[i-1]
			}
			litState := (uint32(i)&uint32(1<<uint(st.lp)-1))<<uint(st.lc) | uint32(prevB)>>uint(8-st.lc)
			probs := st.litProbs[uint(litState)*0x300:]
			var symbol uint32 = 1
			if st.state >= 7 {
				var matchByte = uint32(full[i-int(st.reps[0])-1]) // the decoder keys on rep0
				for symbol < 0x100 {
					matchBit := matchByte >> 7 & 1
					matchByte = matchByte << 1 & 0xFF
					b := int(c >> 7)
					c = c << 1 & 0xFF
					e.bit(probs, int((1+matchBit)<<8+symbol), b)
					symbol = symbol<<1 | uint32(b)
					if matchBit != uint32(b) {
						break
					}
				}
			}
			for symbol < 0x100 {
				b := int(c >> 7)
				c = c << 1 & 0xFF
				e.bit(probs, int(symbol), b)
				symbol = symbol<<1 | uint32(b)
			}
			if st.state < 4 {
				st.state = 0
			} else if st.state < 10 {
				st.state -= 3
			} else {
				st.state -= 6
			}
			insert(nextInsert, i+1)
			nextInsert = i + 1
			i++
			abs++
			continue
		}

		e.bit(st.isMatch, st.state<<uint(st.pb)+int(abs&int64(posStateMask)), 1)
		e.bit(st.isRep, st.state, 0)
		st.reps[3], st.reps[2], st.reps[1] = st.reps[2], st.reps[1], st.reps[0]
		st.reps[0] = uint32(off)
		e.encodeLen(&st.lenChoice, st.lenLow[:], st.lenMid[:], st.lenHigh[:], int(abs&int64(posStateMask)), length)
		slot, base, n := distSlot(uint32(off))
		posSlotTree(e, st, length, slot)
		if slot >= startPosModel {
			footer := uint32(off) - base
			if slot < endPosModel {
				e.reverse(st.specPos, int(base)-slot, n, footer)
			} else {
				e.direct(footer>>alignBits, n-alignBits)
				e.reverse(st.align, 0, alignBits, footer&(1<<alignBits-1))
			}
		}
		st.state = matchStateEnc(st.state)
		insert(nextInsert, i+length)
		nextInsert = i + length
		i += length
		abs += int64(length)
	}
	// positions inside the final match/literal run were not hashed;
	// seed them so a following chunk (sharing this dictionary) finds them
	insert(nextInsert, len(full))
	e.flush()
	return append(dst, e.out...), st
}

func matchStateEnc(st int) int {
	if st < 7 {
		return 7
	}
	return 10
}

// lzma2Uncompressed appends src as LZMA2 uncompressed chunks (the
// incompressible-data fallback, and any encoder's legal floor).
func lzma2Uncompressed(dst []byte, src []byte, dictReset bool) []byte {
	for len(src) > 0 {
		n := len(src)
		if n > 1<<16 {
			n = 1 << 16
		}
		ctl := byte(0x02)
		if dictReset {
			ctl = 0x01
			dictReset = false
		}
		dst = append(dst, ctl, byte((n-1)>>8), byte(n-1))
		dst = append(dst, src[:n]...)
		src = src[n:]
	}
	return dst
}

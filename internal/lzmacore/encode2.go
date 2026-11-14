package lzmacore

// (moved from xz; the generic LZMA data encoder, shared with the
// lzma-alone codec)

const DefaultProps = 0x5D

// encodeLZMAChunk encodes src into the LZMA data of one chunk against
// the history in out (the dictionary so far, which the chunk may
// reference for distances).  st carries the probability model across
// chunks (nil starts fresh); base is the absolute stream position of
// out[0] so position contexts line up after window trims.
// LzmaEncodeNoFlush is LzmaEncode without the range coder's closing
// flush — for callers that continue the same coder stream (the
// lzma-alone end marker).
func LzmaEncodeNoFlush(dst, out, src []byte, st *LzmaState, base int64) (*RcEncoder, *LzmaState) {
	e, st := lzmaEncodeBody(dst, out, src, st, base)
	return e, st
}

func LzmaEncode(dst, out, src []byte, st *LzmaState, base int64) ([]byte, *LzmaState) {
	e, st := lzmaEncodeBody(dst, out, src, st, base)
	e.Flush()
	return e.Out, st
}

func lzmaEncodeBody(dst, out, src []byte, st *LzmaState, base int64) (*RcEncoder, *LzmaState) {
	if st == nil {
		st = NewLzmaState(DefaultProps)
	}
	e := NewRcEncoder()
	posStateMask := 1<<uint(st.Pb) - 1

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
			e.Bit(st.IsMatch, st.State<<uint(st.Pb)+int(abs&int64(posStateMask)), 0)
			var prevB byte
			if i > 0 {
				prevB = full[i-1]
			}
			litState := (uint32(i)&uint32(1<<uint(st.Lp)-1))<<uint(st.Lc) | uint32(prevB)>>uint(8-st.Lc)
			probs := st.LitProbs[uint(litState)*0x300:]
			var symbol uint32 = 1
			if st.State >= 7 {
				var matchByte = uint32(full[i-int(st.Reps[0])-1]) // the decoder keys on rep0
				for symbol < 0x100 {
					matchBit := matchByte >> 7 & 1
					matchByte = matchByte << 1 & 0xFF
					b := int(c >> 7)
					c = c << 1 & 0xFF
					e.Bit(probs, int((1+matchBit)<<8+symbol), b)
					symbol = symbol<<1 | uint32(b)
					if matchBit != uint32(b) {
						break
					}
				}
			}
			for symbol < 0x100 {
				b := int(c >> 7)
				c = c << 1 & 0xFF
				e.Bit(probs, int(symbol), b)
				symbol = symbol<<1 | uint32(b)
			}
			if st.State < 4 {
				st.State = 0
			} else if st.State < 10 {
				st.State -= 3
			} else {
				st.State -= 6
			}
			insert(nextInsert, i+1)
			nextInsert = i + 1
			i++
			abs++
			continue
		}

		e.Bit(st.IsMatch, st.State<<uint(st.Pb)+int(abs&int64(posStateMask)), 1)
		e.Bit(st.IsRep, st.State, 0)
		st.Reps[3], st.Reps[2], st.Reps[1] = st.Reps[2], st.Reps[1], st.Reps[0]
		st.Reps[0] = uint32(off)
		e.EncodeLen(&st.LenChoice, st.LenLow[:], st.LenMid[:], st.LenHigh[:], int(abs&int64(posStateMask)), length)
		slot, base, n := DistSlot(uint32(off))
		PosSlotTree(e, st, length, slot)
		if slot >= StartPosModel {
			footer := uint32(off) - base
			if slot < EndPosModel {
				e.Reverse(st.SpecPos, int(base)-slot, n, footer)
			} else {
				e.Direct(footer>>AlignBits, n-AlignBits)
				e.Reverse(st.Align, 0, AlignBits, footer&(1<<AlignBits-1))
			}
		}
		st.State = MatchState(st.State)
		insert(nextInsert, i+length)
		nextInsert = i + length
		i += length
		abs += int64(length)
	}
	// positions inside the final match/literal run were not hashed;
	// seed them so a following chunk (sharing this dictionary) finds them
	insert(nextInsert, len(full))
	e.Out = append(dst, e.Out...)
	return e, st
}

// EncodeEndMarker writes the end-of-stream marker (a simple match
// with distance 0xFFFFFFFF) through the same range coder as the
// data — the lzma-alone unknown-size terminator.
func EncodeEndMarker(e *RcEncoder, st *LzmaState, posState int) {
	e.Bit(st.IsMatch, st.State<<uint(st.Pb)+posState, 1)
	e.Bit(st.IsRep, st.State, 0)
	length := MatchMinLen
	e.EncodeLen(&st.LenChoice, st.LenLow[:], st.LenMid[:], st.LenHigh[:], posState, length)
	PosSlotTree(e, st, length, 63)
	slot, base, n := DistSlot(0xFFFFFFFF)
	_ = slot
	footer := 0xFFFFFFFF - base
	e.Direct(footer>>AlignBits, n-AlignBits)
	e.Reverse(st.Align, 0, AlignBits, footer&(1<<AlignBits-1))
}

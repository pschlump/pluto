/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package xz

// The LZMA decoder core: the 11-bit probability model, the range
// decoder, and the symbol loop — LZMA as the xz container drives it
// (LZMA2 chunks carry the unpacked size, so the end marker never
// appears).  The model follows Igor Pavlov's public-domain reference.

const (
	numStates    = 12
	numBitLevels = 11
	probInit     = 1 << 10 // half of the 11-bit probability space

	startPosModel = 4
	endPosModel   = 14
	numLenToPos   = 4 // distance-slot contexts (by match length)
	alignBits     = 4
	matchMinLen   = 2
	numLenProbs   = 8
	highLenBits   = 8
)

// lzmaState is one decoder's mutable model: the properties, the
// probability tables, the repetition distances, and the state machine.
type lzmaState struct {
	lc, lp, pb int
	litProbs   []uint16 // 0x300 << (lc+lp)
	isMatch    []uint16 // 12 << pb
	isRep      []uint16 // 12
	isRepG0    []uint16 // 12
	isRepG1    []uint16 // 12
	isRepG2    []uint16 // 12
	isRep0Long []uint16 // 12 << pb
	posSlot    []uint16 // 4 << 6
	specPos    []uint16 // footer trees for slots 4..13
	align      []uint16 // 16
	lenChoice  [2]uint16
	lenLow     [16][numLenProbs]uint16
	lenMid     [16][numLenProbs]uint16
	lenHigh    [1 << highLenBits]uint16
	repChoice  [2]uint16
	repLenLow  [16][numLenProbs]uint16
	repLenMid  [16][numLenProbs]uint16
	repLenHigh [1 << highLenBits]uint16
	state      int
	reps       [4]uint32
}

// newLzmaState builds the model for an LZMA properties byte
// ((pb*5 + lp)*9 + lc; all components 0..4).
func newLzmaState(props byte) *lzmaState {
	s := &lzmaState{
		pb: int(props / 45),
		lp: int(props / 9 % 5),
		lc: int(props % 9),
	}
	s.resetProbs()
	return s
}

// resetProbs reinitializes every probability to its half point (the
// LZMA2 "reset state" chunk modes).
func (s *lzmaState) resetProbs() {
	np := func(n int) []uint16 {
		p := make([]uint16, n)
		for i := range p {
			p[i] = probInit
		}
		return p
	}
	s.litProbs = np(0x300 << uint(s.lc+s.lp))
	s.isMatch = np(numStates << uint(s.pb))
	s.isRep = np(numStates)
	s.isRepG0 = np(numStates)
	s.isRepG1 = np(numStates)
	s.isRepG2 = np(numStates)
	s.isRep0Long = np(numStates << uint(s.pb))
	s.posSlot = np(numLenToPos << 6)
	s.specPos = np(256)
	s.align = np(1 << alignBits)
	s.lenChoice = [2]uint16{probInit, probInit}
	s.repChoice = [2]uint16{probInit, probInit}
	for st := 0; st < 16; st++ {
		for j := 0; j < numLenProbs; j++ {
			s.lenLow[st][j] = probInit
			s.lenMid[st][j] = probInit
			s.repLenLow[st][j] = probInit
			s.repLenMid[st][j] = probInit
		}
	}
	for i := range s.lenHigh {
		s.lenHigh[i] = probInit
		s.repLenHigh[i] = probInit
	}
	s.state = 0
	s.reps = [4]uint32{}
}

// rangeDecoder is the binary range decoder (big-endian byte stream,
// top-11-bit probability bins).
type rangeDecoder struct {
	in   []byte
	pos  int
	code uint32
	rng  uint32
	err  bool // out of input
}

func (rc *rangeDecoder) next() byte {
	if rc.pos >= len(rc.in) {
		rc.err = true
		return 0
	}
	b := rc.in[rc.pos]
	rc.pos++
	return b
}

func newRangeDecoder(in []byte) *rangeDecoder {
	rc := &rangeDecoder{in: in, rng: 0xFFFFFFFF}
	rc.next() // the first byte must be zero; consumed either way
	for i := 0; i < 4; i++ {
		rc.code = rc.code<<8 | uint32(rc.next())
	}
	return rc
}

// normalize refills the range.  Past the end of the chunk's data it
// shifts in zeros instead of failing: range decoders legitimately
// read a few bytes past the semantically-needed point (the encoder's
// flush leaves slack), and genuine truncation loses — the block's
// check field catches it.
func (rc *rangeDecoder) normalize() {
	if rc.rng < 1<<24 {
		rc.rng <<= 8
		rc.code = rc.code << 8
		if rc.pos < len(rc.in) {
			rc.code |= uint32(rc.in[rc.pos])
			rc.pos++
		}
	}
}

// bit decodes one bit through probability slot p, updating the model
// exactly as the reference does.
func (rc *rangeDecoder) bit(p []uint16, i int) int {
	prob := p[i]
	bound := (rc.rng >> numBitLevels) * uint32(prob)
	var v int
	if rc.code < bound {
		rc.rng = bound
		p[i] = prob + uint16((1<<numBitLevels-prob)>>5)
		v = 0
	} else {
		rc.code -= bound
		rc.rng -= bound
		p[i] = prob - uint16(prob>>5)
		v = 1
	}
	rc.normalize()
	return v
}

// direct decodes n fixed-probability bits.
func (rc *rangeDecoder) direct(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		rc.rng >>= 1
		var b uint32
		if rc.code >= rc.rng {
			rc.code -= rc.rng
			b = 1
		}
		v = v<<1 | b
		rc.normalize()
	}
	return v
}

// reverse reads numBits LSB-first through a tree whose root index is
// off+1 (the spec-pos footers and the align bits).
func (rc *rangeDecoder) reverse(probs []uint16, off, numBits int) uint32 {
	var m, symbol uint32
	m = 1
	for i := 0; i < numBits; i++ {
		b := rc.bit(probs, off+int(m))
		m = m<<1 | uint32(b)
		symbol |= uint32(b) << uint(i)
	}
	return symbol
}

// tree reads an n-bit tree MSB-first from index 1 (pos slots, the
// high length bits), returning the symbol with the implicit 1<<n
// subtracted.
func (rc *rangeDecoder) tree(probs []uint16, n int) int {
	m := 1
	for i := 0; i < n; i++ {
		m = m<<1 | rc.bit(probs, m)
	}
	return m - 1<<uint(n)
}

// decodeLen reads a match length through one of the two length coders.
func (rc *rangeDecoder) decodeLen(choice *[2]uint16, low, mid [][numLenProbs]uint16, high []uint16, posState int) int {
	if rc.bit(choice[:], 0) == 0 {
		return matchMinLen + rc.tree(low[posState][:], 3)
	}
	if rc.bit(choice[:], 1) == 0 {
		return matchMinLen + 8 + rc.tree(mid[posState][:], 3)
	}
	return matchMinLen + 16 + rc.tree(high, highLenBits)
}

// literal decodes one byte.  The position context uses lp bits of the
// output position and lc bits of the previous byte; after a match
// (state ≥ 7) the match-byte path runs until a bit differs from the
// byte at distance rep0 — real encoders' favorite trick.
func (s *lzmaState) literal(rc *rangeDecoder, out []byte, rep0 uint32) byte {
	var prev byte
	if len(out) > 0 {
		prev = out[len(out)-1]
	}
	litState := (uint32(len(out))&uint32(1<<uint(s.lp)-1))<<uint(s.lc) | uint32(prev)>>uint(8-s.lc)
	probs := s.litProbs[uint(litState)*0x300:]
	var symbol uint32 = 1
	if s.state >= 7 {
		var matchByte = uint32(out[uint32(len(out))-rep0-1])
		for symbol < 0x100 {
			matchBit := matchByte >> 7 & 1
			matchByte = matchByte << 1 & 0xFF
			b := uint32(rc.bit(probs, int((1+matchBit)<<8+symbol)))
			symbol = symbol<<1 | b
			if matchBit != b {
				break
			}
		}
	}
	for symbol < 0x100 {
		symbol = symbol<<1 | uint32(rc.bit(probs, int(symbol)))
	}
	return byte(symbol)
}

// decodeSlot decodes the 6-bit distance slot; the context is the
// match length (4 buckets).
func (s *lzmaState) decodeSlot(rc *rangeDecoder, length int) int {
	ctx := length - matchMinLen
	if ctx >= numLenToPos {
		ctx = numLenToPos - 1
	}
	m := 1
	for i := 0; i < 6; i++ {
		m = m<<1 | rc.bit(s.posSlot, ctx<<6+m)
	}
	return m - 64
}

// decodeDist decodes the full distance after its slot.
func (s *lzmaState) decodeDist(rc *rangeDecoder, slot int) uint32 {
	if slot < startPosModel {
		return uint32(slot)
	}
	n := slot>>1 - 1
	dist := uint32(2|slot&1) << uint(n)
	if slot < endPosModel {
		return dist + rc.reverse(s.specPos, int(dist)-slot, n)
	}
	dist += rc.direct(n-alignBits) << alignBits
	return dist + rc.reverse(s.align, 0, alignBits)
}

// lzmaDecode decodes exactly want bytes into *out (appending) from
// one LZMA2 chunk's compressed data — the unpacked size is known, so
// no end marker exists.  dictBase is where the current dictionary
// began (the last dict reset): distances may not reach past it.
func lzmaDecode(rc *rangeDecoder, s *lzmaState, out *[]byte, want, dictBase int) bool {
	posStateMask := 1<<uint(s.pb) - 1
	buf := *out
	for len(buf) < want {
		posState := len(buf) & posStateMask
		if rc.bit(s.isMatch, s.state<<uint(s.pb)+posState) == 0 {
			c := s.literal(rc, buf, s.reps[0])
			buf = append(buf, c)
			if s.state < 4 {
				s.state = 0
			} else if s.state < 10 {
				s.state -= 3
			} else {
				s.state -= 6
			}
			continue
		}

		var length int
		if rc.bit(s.isRep, s.state) != 0 {
			// a repeated distance
			if rc.bit(s.isRepG0, s.state) == 0 {
				if rc.bit(s.isRep0Long, s.state<<uint(s.pb)+posState) == 0 {
					// short rep: one byte at distance rep0 (0-based:
					// zero repeats the last byte, which is why the
					// initial rep0 is valid)
					if int(s.reps[0])+1 > len(buf)-dictBase {
						return false
					}
					buf = append(buf, buf[len(buf)-int(s.reps[0])-1])
					s.state = shortRepState(s.state)
					continue
				}
			} else {
				var dist uint32
				if rc.bit(s.isRepG1, s.state) == 0 {
					dist = s.reps[1]
				} else {
					if rc.bit(s.isRepG2, s.state) == 0 {
						dist = s.reps[2]
					} else {
						dist = s.reps[3]
						s.reps[3] = s.reps[2]
					}
					s.reps[2] = s.reps[1]
				}
				s.reps[1] = s.reps[0]
				s.reps[0] = dist
			}
			length = rc.decodeLen(&s.repChoice, s.repLenLow[:], s.repLenMid[:], s.repLenHigh[:], posState)
			s.state = repState(s.state)
		} else {
			s.reps[3], s.reps[2], s.reps[1] = s.reps[2], s.reps[1], s.reps[0]
			length = rc.decodeLen(&s.lenChoice, s.lenLow[:], s.lenMid[:], s.lenHigh[:], posState)
			slot := s.decodeSlot(rc, length)
			s.reps[0] = s.decodeDist(rc, slot)
			s.state = matchState(s.state)
		}
		if int(s.reps[0])+1 > len(buf)-dictBase {
			return false
		}
		// the overlap copy is the RLE: byte-by-byte, never copy()
		src := len(buf) - int(s.reps[0]) - 1
		for i := 0; i < length; i++ {
			buf = append(buf, buf[src])
			src++
		}
	}
	*out = buf
	return true
}

func matchState(st int) int {
	if st < 7 {
		return 7
	}
	return 10
}

func repState(st int) int {
	if st < 7 {
		return 8
	}
	return 11
}

func shortRepState(st int) int {
	if st < 7 {
		return 9
	}
	return 11
}

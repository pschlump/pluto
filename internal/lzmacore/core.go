/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lzmacore

import "io"

// Package lzmacore is the LZMA model shared by the xz and lzma
// codecs: the 11-bit probability model, the range coder (both
// directions), and the symbol loop.  Container packages (LZMA2
// chunks, the .lzma-alone header) drive it; the model follows Igor
// Pavlov's public-domain reference.

const (
	NumStates    = 12
	NumBitLevels = 11
	ProbInit     = 1 << 10 // half of the 11-bit probability space

	StartPosModel = 4
	EndPosModel   = 14
	NumLenToPos   = 4 // distance-slot contexts (by match length)
	AlignBits     = 4
	MatchMinLen   = 2
	NumLenProbs   = 8
	HighLenBits   = 8
)

// LzmaState is one decoder's mutable model: the properties, the
// probability tables, the repetition distances, and the state machine.
type LzmaState struct {
	Lc, Lp, Pb int
	LitProbs   []uint16 // 0x300 << (lc+lp)
	IsMatch    []uint16 // 12 << pb
	IsRep      []uint16 // 12
	IsRepG0    []uint16 // 12
	IsRepG1    []uint16 // 12
	IsRepG2    []uint16 // 12
	IsRep0Long []uint16 // 12 << pb
	PosSlot    []uint16 // 4 << 6
	SpecPos    []uint16 // footer trees for slots 4..13
	Align      []uint16 // 16
	LenChoice  [2]uint16
	LenLow     [16][NumLenProbs]uint16
	LenMid     [16][NumLenProbs]uint16
	LenHigh    [1 << HighLenBits]uint16
	repChoice  [2]uint16
	repLenLow  [16][NumLenProbs]uint16
	repLenMid  [16][NumLenProbs]uint16
	repLenHigh [1 << HighLenBits]uint16
	State      int
	Reps       [4]uint32

	// Pos is the absolute stream position of the NEXT byte the
	// decoder emits (posState/litState contexts key on it).  Buffers
	// that carry history across calls set it once; drivers advance
	// it, so compaction of the buffer cannot desync the contexts.
	Pos int64
}

// NewLzmaState builds the model for an LZMA properties byte
// ((pb*5 + lp)*9 + lc; all components 0..4).
func NewLzmaState(props byte) *LzmaState {
	s := &LzmaState{
		Pb: int(props / 45),
		Lp: int(props / 9 % 5),
		Lc: int(props % 9),
	}
	s.ResetProbs()
	return s
}

// resetProbs reinitializes every probability to its half point (the
// LZMA2 "reset state" chunk modes).
func (s *LzmaState) ResetProbs() {
	np := func(n int) []uint16 {
		p := make([]uint16, n)
		for i := range p {
			p[i] = ProbInit
		}
		return p
	}
	s.LitProbs = np(0x300 << uint(s.Lc+s.Lp))
	s.IsMatch = np(NumStates << uint(s.Pb))
	s.IsRep = np(NumStates)
	s.IsRepG0 = np(NumStates)
	s.IsRepG1 = np(NumStates)
	s.IsRepG2 = np(NumStates)
	s.IsRep0Long = np(NumStates << uint(s.Pb))
	s.PosSlot = np(NumLenToPos << 6)
	s.SpecPos = np(256)
	s.Align = np(1 << AlignBits)
	s.LenChoice = [2]uint16{ProbInit, ProbInit}
	s.repChoice = [2]uint16{ProbInit, ProbInit}
	for st := 0; st < 16; st++ {
		for j := 0; j < NumLenProbs; j++ {
			s.LenLow[st][j] = ProbInit
			s.LenMid[st][j] = ProbInit
			s.repLenLow[st][j] = ProbInit
			s.repLenMid[st][j] = ProbInit
		}
	}
	for i := range s.LenHigh {
		s.LenHigh[i] = ProbInit
		s.repLenHigh[i] = ProbInit
	}
	s.State = 0
	s.Reps = [4]uint32{}
}

// RangeDecoder is the binary range decoder (big-endian byte stream,
// top-11-bit probability bins).  Input is either a slice (In) or a
// pull source (src, the streaming form); past the end the decoder
// zero-fills — decoders legitimately read a few bytes past the
// semantically-needed point, and the container's check catches
// genuine truncation.
type RangeDecoder struct {
	In   []byte
	Pos  int
	Code uint32
	Rng  uint32
	src  io.Reader // when set, In refills from here
	Err  bool

	// starved counts reads past the end of input (zero-fills).  A
	// little slack is normal — decoders read a few bytes past the
	// semantically-needed point — but an end-marker stream that keeps
	// needing more is truncated.
	starved int
}

func (rc *RangeDecoder) Next() byte {
	if rc.Pos >= len(rc.In) {
		if rc.src != nil {
			buf := make([]byte, 64*1024)
			n, _ := rc.src.Read(buf)
			if n > 0 {
				rc.In = buf[:n]
				rc.Pos = 0
			} else {
				rc.src = nil // exhausted: zero-fill from here
			}
		}
		if rc.Pos >= len(rc.In) {
			rc.starved++
			if rc.starved > 8 {
				rc.Err = true
			}
			return 0
		}
	}
	b := rc.In[rc.Pos]
	rc.Pos++
	return b
}

func NewRangeDecoder(in []byte) *RangeDecoder {
	rc := &RangeDecoder{In: in, Rng: 0xFFFFFFFF}
	rc.Next() // the first byte must be zero; consumed either way
	for i := 0; i < 4; i++ {
		rc.Code = rc.Code<<8 | uint32(rc.Next())
	}
	return rc
}

// normalize refills the range.  Past the end of the chunk's data it
// shifts in zeros instead of failing: range decoders legitimately
// read a few bytes past the semantically-needed point (the encoder's
// flush leaves slack), and genuine truncation loses — the block's
// check field catches it.
func (rc *RangeDecoder) Normalize() {
	if rc.Rng < 1<<24 {
		rc.Rng <<= 8
		rc.Code = rc.Code << 8
		if rc.Pos < len(rc.In) {
			rc.Code |= uint32(rc.In[rc.Pos])
			rc.Pos++
		} else {
			rc.starved++
			if rc.starved > 8 {
				rc.Err = true
			}
		}
	}
}

// bit decodes one bit through probability slot p, updating the model
// exactly as the reference does.
func (rc *RangeDecoder) Bit(p []uint16, i int) int {
	prob := p[i]
	bound := (rc.Rng >> NumBitLevels) * uint32(prob)
	var v int
	if rc.Code < bound {
		rc.Rng = bound
		p[i] = prob + uint16((1<<NumBitLevels-prob)>>5)
		v = 0
	} else {
		rc.Code -= bound
		rc.Rng -= bound
		p[i] = prob - uint16(prob>>5)
		v = 1
	}
	rc.Normalize()
	return v
}

// direct decodes n fixed-probability bits.
func (rc *RangeDecoder) Direct(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		rc.Rng >>= 1
		var b uint32
		if rc.Code >= rc.Rng {
			rc.Code -= rc.Rng
			b = 1
		}
		v = v<<1 | b
		rc.Normalize()
	}
	return v
}

// reverse reads numBits LSB-first through a tree whose root index is
// off+1 (the spec-pos footers and the align bits).
func (rc *RangeDecoder) Reverse(probs []uint16, off, numBits int) uint32 {
	var m, symbol uint32
	m = 1
	for i := 0; i < numBits; i++ {
		b := rc.Bit(probs, off+int(m))
		m = m<<1 | uint32(b)
		symbol |= uint32(b) << uint(i)
	}
	return symbol
}

// tree reads an n-bit tree MSB-first from index 1 (pos slots, the
// high length bits), returning the symbol with the implicit 1<<n
// subtracted.
func (rc *RangeDecoder) Tree(probs []uint16, n int) int {
	m := 1
	for i := 0; i < n; i++ {
		m = m<<1 | rc.Bit(probs, m)
	}
	return m - 1<<uint(n)
}

// decodeLen reads a match length through one of the two length coders.
func (rc *RangeDecoder) DecodeLen(choice *[2]uint16, low, mid [][NumLenProbs]uint16, high []uint16, posState int) int {
	if rc.Bit(choice[:], 0) == 0 {
		return MatchMinLen + rc.Tree(low[posState][:], 3)
	}
	if rc.Bit(choice[:], 1) == 0 {
		return MatchMinLen + 8 + rc.Tree(mid[posState][:], 3)
	}
	return MatchMinLen + 16 + rc.Tree(high, HighLenBits)
}

// literal decodes one byte.  The position context uses lp bits of the
// output position and lc bits of the previous byte; after a match
// (state ≥ 7) the match-byte path runs until a bit differs from the
// byte at distance rep0 — real encoders' favorite trick.
func (s *LzmaState) Literal(rc *RangeDecoder, out []byte, rep0 uint32) byte {
	var prev byte
	if len(out) > 0 {
		prev = out[len(out)-1]
	}
	litState := (uint32(s.Pos)&uint32(1<<uint(s.Lp)-1))<<uint(s.Lc) | uint32(prev)>>uint(8-s.Lc)
	probs := s.LitProbs[uint(litState)*0x300:]
	var symbol uint32 = 1
	if s.State >= 7 {
		var matchByte = uint32(out[uint32(len(out))-rep0-1])
		for symbol < 0x100 {
			matchBit := matchByte >> 7 & 1
			matchByte = matchByte << 1 & 0xFF
			b := uint32(rc.Bit(probs, int((1+matchBit)<<8+symbol)))
			symbol = symbol<<1 | b
			if matchBit != b {
				break
			}
		}
	}
	for symbol < 0x100 {
		symbol = symbol<<1 | uint32(rc.Bit(probs, int(symbol)))
	}
	return byte(symbol)
}

// decodeSlot decodes the 6-bit distance slot; the context is the
// match length (4 buckets).
func (s *LzmaState) DecodeSlot(rc *RangeDecoder, length int) int {
	ctx := length - MatchMinLen
	if ctx >= NumLenToPos {
		ctx = NumLenToPos - 1
	}
	m := 1
	for i := 0; i < 6; i++ {
		m = m<<1 | rc.Bit(s.PosSlot, ctx<<6+m)
	}
	return m - 64
}

// decodeDist decodes the full distance after its slot.
func (s *LzmaState) DecodeDist(rc *RangeDecoder, slot int) uint32 {
	if slot < StartPosModel {
		return uint32(slot)
	}
	n := slot>>1 - 1
	dist := uint32(2|slot&1) << uint(n)
	if slot < EndPosModel {
		return dist + rc.Reverse(s.SpecPos, int(dist)-slot, n)
	}
	dist += rc.Direct(n-AlignBits) << AlignBits
	return dist + rc.Reverse(s.Align, 0, AlignBits)
}

// LzmaDecode decodes exactly want bytes into *out (appending) from
// one LZMA2 chunk's compressed data — the unpacked size is known, so
// no end marker exists.  dictBase is where the current dictionary
// began (the last dict reset): distances may not reach past it.
func LzmaDecode(rc *RangeDecoder, s *LzmaState, out *[]byte, want, dictBase int) bool {
	posStateMask := 1<<uint(s.Pb) - 1
	buf := *out
	for len(buf) < want {
		posState := len(buf) & posStateMask
		if rc.Bit(s.IsMatch, s.State<<uint(s.Pb)+posState) == 0 {
			c := s.Literal(rc, buf, s.Reps[0])
			buf = append(buf, c)
			if s.State < 4 {
				s.State = 0
			} else if s.State < 10 {
				s.State -= 3
			} else {
				s.State -= 6
			}
			continue
		}

		var length int
		if rc.Bit(s.IsRep, s.State) != 0 {
			// a repeated distance
			if rc.Bit(s.IsRepG0, s.State) == 0 {
				if rc.Bit(s.IsRep0Long, s.State<<uint(s.Pb)+posState) == 0 {
					// short rep: one byte at distance rep0 (0-based:
					// zero repeats the last byte, which is why the
					// initial rep0 is valid)
					if int(s.Reps[0])+1 > len(buf)-dictBase {
						return false
					}
					buf = append(buf, buf[len(buf)-int(s.Reps[0])-1])
					s.State = ShortRepState(s.State)
					continue
				}
			} else {
				var dist uint32
				if rc.Bit(s.IsRepG1, s.State) == 0 {
					dist = s.Reps[1]
				} else {
					if rc.Bit(s.IsRepG2, s.State) == 0 {
						dist = s.Reps[2]
					} else {
						dist = s.Reps[3]
						s.Reps[3] = s.Reps[2]
					}
					s.Reps[2] = s.Reps[1]
				}
				s.Reps[1] = s.Reps[0]
				s.Reps[0] = dist
			}
			length = rc.DecodeLen(&s.repChoice, s.repLenLow[:], s.repLenMid[:], s.repLenHigh[:], posState)
			s.State = RepState(s.State)
		} else {
			s.Reps[3], s.Reps[2], s.Reps[1] = s.Reps[2], s.Reps[1], s.Reps[0]
			length = rc.DecodeLen(&s.LenChoice, s.LenLow[:], s.LenMid[:], s.LenHigh[:], posState)
			slot := s.DecodeSlot(rc, length)
			s.Reps[0] = s.DecodeDist(rc, slot)
			s.State = MatchState(s.State)
		}
		if int(s.Reps[0])+1 > len(buf)-dictBase {
			return false
		}
		// the overlap copy is the RLE: byte-by-byte, never copy()
		src := len(buf) - int(s.Reps[0]) - 1
		for i := 0; i < length; i++ {
			buf = append(buf, buf[src])
			src++
		}
	}
	*out = buf
	return true
}

// LzmaDecodeStream decodes until the end marker (a match whose
// distance is 0xFFFFFFFF) or limit bytes, whichever comes first —
// the lzma-alone unknown-size form.  dictBase semantics as LzmaDecode.
// decodeSymbol decodes one symbol into buf: a literal, a match, a
// rep, or the end marker (marker reports it).  ok false means
// malformed.
func decodeSymbol(rc *RangeDecoder, s *LzmaState, buf *[]byte) (marker, ok bool) {
	b := *buf
	posState := int(s.Pos) & (1<<uint(s.Pb) - 1)
	if rc.Bit(s.IsMatch, s.State<<uint(s.Pb)+posState) == 0 {
		c := s.Literal(rc, b, s.Reps[0])
		b = append(b, c)
		s.Pos++
		if s.State < 4 {
			s.State = 0
		} else if s.State < 10 {
			s.State -= 3
		} else {
			s.State -= 6
		}
		*buf = b
		return false, true
	}
	var length int
	if rc.Bit(s.IsRep, s.State) != 0 {
		// a repeated distance (the end marker cannot ride one)
		if rc.Bit(s.IsRepG0, s.State) == 0 {
			if rc.Bit(s.IsRep0Long, s.State<<uint(s.Pb)+posState) == 0 {
				// short rep: one byte at distance rep0 (0-based:
				// zero repeats the last byte)
				if len(b) == 0 || int(s.Reps[0])+1 > len(b) {
					return false, false
				}
				b = append(b, b[len(b)-int(s.Reps[0])-1])
				s.State = ShortRepState(s.State)
				s.Pos++
				*buf = b
				return false, true
			}
		} else {
			var dist uint32
			if rc.Bit(s.IsRepG1, s.State) == 0 {
				dist = s.Reps[1]
			} else {
				if rc.Bit(s.IsRepG2, s.State) == 0 {
					dist = s.Reps[2]
				} else {
					dist = s.Reps[3]
					s.Reps[3] = s.Reps[2]
				}
				s.Reps[2] = s.Reps[1]
			}
			s.Reps[1] = s.Reps[0]
			s.Reps[0] = dist
		}
		length = rc.DecodeLen(&s.repChoice, s.repLenLow[:], s.repLenMid[:], s.repLenHigh[:], posState)
		s.State = RepState(s.State)
	} else {
		s.Reps[3], s.Reps[2], s.Reps[1] = s.Reps[2], s.Reps[1], s.Reps[0]
		length = rc.DecodeLen(&s.LenChoice, s.LenLow[:], s.LenMid[:], s.LenHigh[:], posState)
		slot := s.DecodeSlot(rc, length)
		s.Reps[0] = s.DecodeDist(rc, slot)
		s.State = MatchState(s.State)
		if s.Reps[0] == 0xFFFFFFFF {
			if rc.starved > 0 {
				return false, false // a marker synthesized from zero-fill garbage
			}
			return true, true
		}
	}
	if int(s.Reps[0])+1 > len(b) {
		return false, false
	}
	// the overlap copy is the RLE: byte-by-byte, never copy()
	src := len(b) - int(s.Reps[0]) - 1
	for i := 0; i < length; i++ {
		b = append(b, b[src])
		src++
	}
	s.Pos += int64(length)
	*buf = b
	return false, true
}

// LzmaDecodeStream decodes until the end marker or limit bytes,
// whichever comes first — the lzma-alone unknown-size form.
func LzmaDecodeStream(rc *RangeDecoder, s *LzmaState, out *[]byte, limit int) bool {
	for len(*out) < limit && !rc.Err {
		marker, ok := decodeSymbol(rc, s, out)
		if !ok {
			return false
		}
		if marker {
			return true
		}
	}
	return false
}

// LzmaDecodeSome decodes until the end marker is seen or out grows by
// at least one byte past target — the incremental driver for the
// streaming lzma-alone Reader.  done reports the marker.
func LzmaDecodeSome(rc *RangeDecoder, s *LzmaState, out *[]byte, target int) (done, ok bool) {
	for len(*out) < target && !rc.Err {
		marker, ok := decodeSymbol(rc, s, out)
		if !ok {
			return false, false
		}
		if marker {
			return true, true
		}
	}
	return false, true
}

func MatchState(st int) int {
	if st < 7 {
		return 7
	}
	return 10
}

func RepState(st int) int {
	if st < 7 {
		return 8
	}
	return 11
}

func ShortRepState(st int) int {
	if st < 7 {
		return 9
	}
	return 11
}

// NewRangeDecoderReader is NewRangeDecoder over a refillable source:
// when the buffered bytes run out, more are pulled from src (past
// its end the decoder zero-fills, the whole-buffer semantic).  This
// is what lets the lzma-alone codec stream: its format has no block
// structure, so the range coder must be fed incrementally.
func NewRangeDecoderReader(src io.Reader) *RangeDecoder {
	rc := &RangeDecoder{Rng: 0xFFFFFFFF, src: src}
	rc.Next() // the first byte must be zero
	for i := 0; i < 4; i++ {
		rc.Code = rc.Code<<8 | uint32(rc.Next())
	}
	return rc
}

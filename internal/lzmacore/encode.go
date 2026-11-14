package lzmacore

// RcEncoder is the binary range encoder.
type RcEncoder struct {
	Out   []byte
	Low   uint64
	Rng   uint32
	Cache byte
	Csz   int64
}

func NewRcEncoder() *RcEncoder {
	return &RcEncoder{Rng: 0xFFFFFFFF, Csz: 1}
}

// ShiftLow emits buffered bytes, propagating carries.
func (e *RcEncoder) ShiftLow() {
	if uint32(e.Low) < 0xFF000000 || e.Low>>32 != 0 {
		temp := e.Cache
		carry := byte(e.Low >> 32)
		for e.Csz > 0 {
			e.Out = append(e.Out, temp+carry) // buffered 0xFF + carry wraps to 0x00: the propagation
			temp = 0xFF
			e.Csz--
		}
		e.Cache = byte(e.Low >> 24)
	}
	e.Csz++
	e.Low = uint64(uint32(e.Low) << 8)
}

func (e *RcEncoder) Normalize() {
	for e.Rng < 1<<24 {
		e.Rng <<= 8
		e.ShiftLow()
	}
}

// Bit encodes one Bit through probability slot p, updating the model
// exactly as the decoder does.
func (e *RcEncoder) Bit(p []uint16, i int, v int) {
	prob := p[i]
	bound := (e.Rng >> NumBitLevels) * uint32(prob)
	if v == 0 {
		e.Rng = bound
		p[i] = prob + uint16((1<<NumBitLevels-prob)>>5)
	} else {
		e.Low += uint64(bound)
		e.Rng -= bound
		p[i] = prob - uint16(prob>>5)
	}
	e.Normalize()
}

// Direct encodes n fixed-probability bits.
func (e *RcEncoder) Direct(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		e.Rng >>= 1
		if v>>uint(i)&1 != 0 {
			e.Low += uint64(e.Rng)
		}
		e.Normalize()
	}
}

// Tree encodes an n-Bit symbol MSB-first through a Tree from index 1.
func (e *RcEncoder) Tree(p []uint16, n int, v int) {
	m := 1
	for i := n - 1; i >= 0; i-- {
		b := v >> uint(i) & 1
		e.Bit(p, m, b)
		m = m<<1 | b
	}
}

// Reverse encodes v's low n bits LSB-first through a Tree at offset off.
func (e *RcEncoder) Reverse(p []uint16, off, n int, v uint32) {
	m := 1
	for i := 0; i < n; i++ {
		b := int(v >> uint(i) & 1)
		e.Bit(p, off+m, b)
		m = m<<1 | b
	}
}

// Flush writes the five closing bytes.
func (e *RcEncoder) Flush() {
	for i := 0; i < 5; i++ {
		e.ShiftLow()
	}
}

// EncodeLen writes a match length through one of the length coders.
func (e *RcEncoder) EncodeLen(choice *[2]uint16, low, mid [][NumLenProbs]uint16, high []uint16, posState, length int) {
	length -= MatchMinLen
	if length < 8 {
		e.Bit(choice[:], 0, 0)
		e.Tree(low[posState][:], 3, length)
		return
	}
	e.Bit(choice[:], 0, 1)
	if length < 16 {
		e.Bit(choice[:], 1, 0)
		e.Tree(mid[posState][:], 3, length-8)
		return
	}
	e.Bit(choice[:], 1, 1)
	e.Tree(high, HighLenBits, length-16)
}

// DistSlot returns the 6-Bit slot, the slot's base distance, and the
// number of footer bits, for a 0-based distance.
func DistSlot(dist uint32) (slot int, base uint32, n int) {
	if dist < StartPosModel {
		return int(dist), 0, 0
	}
	hb := 31 - LeadingZeros(dist)
	slot = hb*2 + int(dist>>uint(hb-1)&1)
	n = slot>>1 - 1
	base = (2 | uint32(slot&1)) << uint(n)
	return slot, base, n
}

func LeadingZeros(v uint32) int {
	n := 0
	for i := 31; i >= 0 && v>>uint(i)&1 == 0; i-- {
		n++
	}
	return n
}

// PosSlotTree encodes the distance slot with the length context the
// decoder keys on.
func PosSlotTree(e *RcEncoder, st *LzmaState, length, slot int) {
	ctx := length - MatchMinLen
	if ctx >= NumLenToPos {
		ctx = NumLenToPos - 1
	}
	m := 1
	for i := 5; i >= 0; i-- {
		b := slot >> uint(i) & 1
		e.Bit(st.PosSlot, ctx<<6+m, b)
		m = m<<1 | b
	}
}

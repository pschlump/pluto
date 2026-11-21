/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd8

// Subbotin's carryless range coder (1999), the coder zip's PPMd uses.
// Both sides track Low: renormalization shifts while the top bytes of
// Low and Low+Range agree (no carry can escape) or Range has fallen
// under the bottom guard and gets re-anchored to Low's alignment.
// The decoder subtracts the same start*Range the encoder adds.

const (
	rcTop8 = 1 << 24
	rcBot8 = 1 << 15
)

type rcState struct {
	low  uint32
	rng  uint32
	code uint32

	// decode side
	data []byte
	pos  int
	over bool // a read ran past the end: the stream is truncated

	// encode side
	out []byte
}

func (r *rcState) readByte() byte {
	if r.pos >= len(r.data) {
		r.over = true
		return 0
	}
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *rcState) initDec() bool {
	r.code = 0
	r.rng = 0xFFFFFFFF
	r.low = 0
	for i := 0; i < 4; i++ {
		r.code = r.code<<8 | uint32(r.readByte())
	}
	return !r.over && r.code < 0xFFFFFFFF
}

// normDec renormalizes on the decode side: consume a byte per shift.
func (r *rcState) normDec() {
	for {
		if (r.low ^ (r.low + r.rng)) >= rcTop8 {
			if r.rng >= rcBot8 {
				break
			}
			r.rng = (0 - r.low) & (rcBot8 - 1)
		}
		r.code = r.code<<8 | uint32(r.readByte())
		r.rng <<= 8
		r.low <<= 8
	}
}

func (m *model) rcDecode(start, size uint32) {
	start *= m.rc.rng
	m.rc.low += start
	m.rc.code -= start
	m.rc.rng *= size
}

func (m *model) rcThreshold(total uint32) uint32 {
	m.rc.rng /= total
	return m.rc.code / m.rc.rng
}

// getBinSumm mirrors the Ppmd8_GetBinSumm macro: the row is a
// frequency bucket, the column mixes prevSuccess, runLength, the
// suffix's size class, and the context's own Flags.
func (m *model) getBinSumm() *uint16 {
	mc := m.minContext
	idx := m.prevSuccess + uint32(m.runLength>>26)&0x20 +
		uint32(m.ns2BSIndx[m.ctxNS(m.ctxSuffix(mc))]) +
		m.ctxFlags(mc)
	return &m.binSumm[m.ns2Indx[m.stFreq(m.oneState(mc))-1]][idx]
}

// decodeSymbol decodes one symbol: >= 0 a byte, -1 the end-of-payload
// marker, -2 a data error.
func (m *model) decodeSymbol() int {
	var charMask [256]byte

	if m.ctxNS(m.minContext) != 0 {
		s := m.ctxStats(m.minContext)
		summFreq := m.ctxSummFreq(m.minContext)
		if summFreq > m.rc.rng {
			summFreq = m.rc.rng
		}

		count := m.rcThreshold(summFreq)
		hiCnt := count

		count -= m.stFreq(s)
		if int32(count) < 0 {
			m.rcDecode(0, m.stFreq(s))
			m.rc.normDec()
			m.foundState = s
			sym := m.stSymbol(s)
			m.update1_0()
			return int(sym)
		}

		m.prevSuccess = 0
		i := m.ctxNS(m.minContext)
		for {
			s += 6
			count -= m.stFreq(s)
			if int32(count) < 0 {
				m.rcDecode(hiCnt-count-m.stFreq(s), m.stFreq(s))
				m.rc.normDec()
				m.foundState = s
				sym := m.stSymbol(s)
				m.update1()
				return int(sym)
			}
			i--
			if i == 0 {
				break
			}
		}

		if hiCnt >= summFreq {
			return -2
		}
		hiCnt -= count
		m.rcDecode(hiCnt, summFreq-hiCnt)

		for i := range charMask {
			charMask[i] = 0xFF
		}
		charMask[m.stSymbol(s)] = 0
		for s2 := m.ctxStats(m.minContext); ; {
			sym0 := m.stSymbol(s2)
			sym1 := m.stSymbol(s2 + 6)
			s2 += 12
			charMask[sym0] = 0
			charMask[sym1] = 0
			if s2 >= s {
				break
			}
		}
	} else {
		s := m.oneState(m.minContext)
		prob := m.getBinSumm()
		pr := uint32(*prob)
		size0 := (m.rc.rng >> 14) * pr
		pr -= (pr + (1 << (periodBits - 2))) >> periodBits

		if m.rc.code < size0 {
			*prob = uint16(pr + 1<<intBits)

			m.rc.rng = size0
			m.rc.normDec()

			freq := m.stFreq(s)
			c := m.stSucc(s)
			sym := m.stSymbol(s)
			m.foundState = s
			m.prevSuccess = 1
			m.runLength++
			m.stSetFreq(s, freq+b2u(freq < 196))
			if m.orderFall == 0 && c >= m.unitsStart {
				m.maxContext = c
				m.minContext = c
			} else {
				m.updateModel()
			}
			return int(sym)
		}

		*prob = uint16(pr)
		m.initEsc = uint32(m.expEscape[pr>>10])

		m.rc.low += size0
		m.rc.code -= size0
		m.rc.rng = (m.rc.rng &^ (binScale - 1)) - size0

		for i := range charMask {
			charMask[i] = 0xFF
		}
		charMask[m.stSymbol(m.oneState(m.minContext))] = 0
		m.prevSuccess = 0
	}

	for {
		m.rc.normDec()
		mc := m.minContext
		numMasked := m.ctxNS(mc)

		var i uint32
		for {
			m.orderFall++
			if m.ctxSuffix(mc) == 0 {
				return -1 // the end-of-payload marker
			}
			mc = m.ctxSuffix(mc)
			i = m.ctxNS(mc)
			if i != numMasked {
				break
			}
		}

		s := m.ctxStats(mc)
		num := i + 1
		num2 := num / 2
		num &= 1
		var hiCnt uint32
		if num == 1 {
			hiCnt = m.stFreq(s) & uint32(charMask[m.stSymbol(s)])
		}
		s += num * 6
		m.minContext = mc

		for ; num2 > 0; num2-- {
			sym0 := m.stSymbol(s)
			sym1 := m.stSymbol(s + 6)
			s += 12
			hiCnt += m.stFreq(s-12) & uint32(charMask[sym0])
			hiCnt += m.stFreq(s-6) & uint32(charMask[sym1])
		}

		see, escFreq := m.makeEscFreq(numMasked)
		freqSum := escFreq + hiCnt
		freqSum2 := freqSum
		if freqSum2 > m.rc.rng {
			freqSum2 = m.rc.rng
		}

		count := m.rcThreshold(freqSum2)

		if count < hiCnt {
			s = m.ctxStats(m.minContext)
			count2 := int32(count)
			for {
				count2 -= int32(m.stFreq(s) & uint32(charMask[m.stSymbol(s)]))
				s += 6
				if count2 < 0 {
					break
				}
			}
			s -= 6
			m.rcDecode(uint32(int32(count)-count2)-m.stFreq(s), m.stFreq(s))
			m.rc.normDec()

			see.update()
			m.foundState = s
			sym := m.stSymbol(s)
			m.update2()
			return int(sym)
		}

		if count >= freqSum2 {
			return -2
		}

		m.rcDecode(hiCnt, freqSum2-hiCnt)
		see.summ = uint16(uint32(see.summ) + freqSum)

		s2 := m.ctxStats(m.minContext)
		end := s2 + (m.ctxNS(m.minContext)+1)*6
		for s2 != end {
			charMask[m.stSymbol(s2)] = 0
			s2 += 6
		}
	}
}

// Props are the two leading bytes of every zip method-98 member:
// order-1 in bits 0..3, memMB-1 in bits 4..11, restore method in
// bits 12+.
type Props struct {
	Order  int // 2..16
	MemMB  int // 1..256
	Restor int // 0 restart, 1 cut-off
}

func ParseProps(b []byte) (Props, bool) {
	if len(b) < 2 {
		return Props{}, false
	}
	v := uint32(b[0]) | uint32(b[1])<<8
	pr := Props{Order: int(v&15) + 1, MemMB: int(v>>4&0xFF) + 1, Restor: int(v >> 12)}
	if pr.Order < minOrder || pr.Order > maxOrder || pr.MemMB < 1 || pr.MemMB > 256 || pr.Restor > 2 {
		return Props{}, false
	}
	return pr, true
}

func (p Props) Bytes() [2]byte {
	v := uint32(p.Order-1) + uint32(p.MemMB-1)<<4 + uint32(p.Restor)<<12
	return [2]byte{byte(v), byte(v >> 8)}
}

func (m *model) init(order, memMB, restor int) bool {
	if order < minOrder || order > maxOrder || memMB < 1 || memMB > 256 || restor < 0 || restor > 2 {
		return false
	}
	m.construct()
	m.alloc(uint32(memMB) << 20)
	m.maxOrder = uint32(order)
	m.restoreMethod = uint32(restor)
	m.restartModel()
	return true
}

// Decode reads a zip method-98 PPMd stream (the bytes after the
// two-byte props) until its end marker.  outSize, when >= 0, must
// match the decoded length.  nil means malformed.
func Decode(data []byte, pr Props, outSize int64) []byte {
	if !pr.valid() {
		return nil
	}
	m := &model{}
	if !m.init(pr.Order, pr.MemMB, pr.Restor) {
		return nil
	}
	m.rc.data = data
	if !m.rc.initDec() {
		return nil
	}
	out := make([]byte, 0, 16) // non-nil even for empty payloads
	for {
		sym := m.decodeSymbol()
		if m.rc.over {
			return nil // truncated
		}
		if sym < 0 {
			if sym != -1 {
				return nil
			}
			break
		}
		out = append(out, byte(sym))
		if outSize >= 0 && int64(len(out)) > outSize {
			return nil
		}
	}
	if outSize >= 0 && int64(len(out)) != outSize {
		return nil
	}
	return out
}

func (p Props) valid() bool {
	return p.Order >= minOrder && p.Order <= maxOrder && p.MemMB >= 1 && p.MemMB <= 256 && p.Restor <= 1
}

/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd

// The decoder side: the carryless range coder 7-Zip pairs with the
// PPMd-H model, the per-symbol decode driver, and the whole-stream
// entry point.

type rangeDec struct {
	data []byte
	pos  int
	rng  uint32
	code uint32
}

func (r *rangeDec) readByte() byte {
	if r.pos >= len(r.data) {
		r.pos++  // a read past the end yields zeros (the model may
		return 0 // still terminate on the padding it implies)
	}
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *rangeDec) init() bool {
	r.code = 0
	r.rng = 0xFFFFFFFF
	if r.readByte() != 0 {
		return false
	}
	for i := 0; i < 4; i++ {
		r.code = r.code<<8 | uint32(r.readByte())
	}
	return r.code < 0xFFFFFFFF
}

const rcTop = 1 << 24

// norm shifts the range up (consuming a byte) until it reaches the
// top; the reference applies it twice per call site (RC_NORM) or
// once (RC_NORM_1).  Call sites are part of the format: they decide
// when bytes are consumed.
func (r *rangeDec) norm() {
	if r.rng < rcTop {
		r.code = r.code<<8 | uint32(r.readByte())
		r.rng <<= 8
		if r.rng < rcTop {
			r.code = r.code<<8 | uint32(r.readByte())
			r.rng <<= 8
		}
	}
}

func (r *rangeDec) norm1() {
	if r.rng < rcTop {
		r.code = r.code<<8 | uint32(r.readByte())
		r.rng <<= 8
	}
}

func (r *rangeDec) decode(start, size uint32) {
	r.code -= start * r.rng
	r.rng *= size
}

func (r *rangeDec) threshold(total uint32) uint32 {
	r.rng /= total
	return r.code / r.rng
}

// getBinSumm mirrors the Ppmd7_GetBinSumm macro.
func (m *model) getBinSumm() *uint16 {
	mc := m.minContext
	suffix := m.ctxSuffix(mc)
	idx := m.prevSuccess + uint32((m.runLength>>26)&0x20) +
		uint32(m.ns2BSIndx[m.ctxNumStats(suffix)-1]) +
		hiBitsFlag4(m.stSymbol(m.oneState(mc))) +
		hiBitsFlag3(m.stSymbol(m.foundState))
	m.hiBitsFlag = hiBitsFlag3(m.stSymbol(m.foundState))
	return &m.binSumm[m.stFreq(m.oneState(mc))-1][idx]
}

// makeEscFreq prices the escape for a masked context through SEE.
func (m *model) makeEscFreq(numMasked uint32) (*see, uint32) {
	mc := m.minContext
	numStats := m.ctxNumStats(mc)
	if numStats != 256 {
		nonMasked := numStats - numMasked
		s := &m.see[m.ns2Indx[nonMasked-1]]
		idx := nonMasked < m.ctxNumStats(m.ctxSuffix(mc))-numStats
		off := uint32(0)
		if idx {
			off = 1
		}
		if m.ctxSummFreq(mc) < 11*numStats {
			off += 2
		}
		if numMasked > nonMasked {
			off += 4
		}
		off += m.hiBitsFlag
		see := &s[off]
		summ := uint32(see.summ)
		r := summ >> see.shift
		see.summ = uint16(summ - r)
		if r == 0 {
			r = 1
		}
		return see, r
	}
	return &m.dummySee, 1
}

// nextContext advances after a successful non-masked symbol.
func (m *model) nextContext() {
	c := m.stSucc(m.foundState)
	if m.orderFall == 0 && c > m.text {
		m.maxContext = c
		m.minContext = c
	} else {
		m.updateModel()
	}
}

func (m *model) update1() {
	s := m.foundState
	freq := m.stFreq(s) + 4
	m.ctxSetSummFreq(m.minContext, m.ctxSummFreq(m.minContext)+4)
	m.stSetFreq(s, freq)
	if freq > m.stFreq(s-6) {
		m.swapStates(s)
		s -= 6
		m.foundState = s
		if freq > maxFreq {
			m.rescale()
		}
	}
	m.nextContext()
}

func (m *model) update1_0() {
	s := m.foundState
	mc := m.minContext
	freq := m.stFreq(s)
	summFreq := m.ctxSummFreq(mc)
	if 2*freq > summFreq {
		m.prevSuccess = 1
		m.runLength++
	} else {
		m.prevSuccess = 0
	}
	m.ctxSetSummFreq(mc, summFreq+4)
	m.stSetFreq(s, freq+4)
	if freq+4 > maxFreq {
		m.rescale()
	}
	m.nextContext()
}

func (m *model) update2() {
	s := m.foundState
	freq := m.stFreq(s) + 4
	m.runLength = m.initRL
	m.ctxSetSummFreq(m.minContext, m.ctxSummFreq(m.minContext)+4)
	m.stSetFreq(s, freq)
	if freq > maxFreq {
		m.rescale()
	}
	m.updateModel()
}

// DecodeSymbol decodes one symbol: >= 0 a byte, -1 end of payload,
// -2 data error.
func (m *model) decodeSymbol() int {
	var charMask [256]byte
	for i := range charMask {
		charMask[i] = 0xFF
	}

	if m.ctxNumStats(m.minContext) != 1 {
		s := m.ctxStats(m.minContext)
		summFreq := m.ctxSummFreq(m.minContext)

		count := m.dec.threshold(summFreq)
		hiCnt := int32(count)

		if hiCnt-int32(m.stFreq(s)) < 0 {
			m.dec.decode(0, m.stFreq(s))
			m.dec.norm()
			m.foundState = s
			sym := m.stSymbol(s)
			m.update1_0()
			return int(sym)
		}

		m.prevSuccess = 0
		i := m.ctxNumStats(m.minContext) - 1
		counted := hiCnt - int32(m.stFreq(s))
		for {
			s += 6
			counted -= int32(m.stFreq(s))
			if counted < 0 {
				m.dec.decode(uint32(hiCnt-counted)-m.stFreq(s), m.stFreq(s))
				m.dec.norm()
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

		if hiCnt >= int32(summFreq) {
			return -2
		}
		hiCnt -= counted
		m.dec.decode(uint32(hiCnt), summFreq-uint32(hiCnt))

		m.hiBitsFlag = hiBitsFlag3(m.stSymbol(m.foundState))
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
		size0 := (m.dec.rng >> 14) * pr
		// PPMD_UPDATE_PROB_1
		pr -= (pr + (1 << (periodBits - 2))) >> periodBits

		if m.dec.code < size0 {
			*prob = uint16(pr + 1<<intBits)

			m.dec.rng = size0
			m.dec.norm1()

			sym := m.stSymbol(s)
			freq := m.stFreq(s)
			c := m.stSucc(s)
			m.foundState = s
			m.prevSuccess = 1
			m.runLength++
			if freq < 128 {
				m.stSetFreq(s, freq+1)
			}
			if m.orderFall == 0 && c > m.text {
				m.maxContext = c
				m.minContext = c
			} else {
				m.updateModel()
			}
			return int(sym)
		}

		*prob = uint16(pr)
		m.initEsc = uint32(m.expEscape[pr>>10])

		m.dec.code -= size0
		m.dec.rng -= size0

		charMask[m.stSymbol(m.oneState(m.minContext))] = 0
		m.prevSuccess = 0
	}

	for {
		m.dec.norm()
		mc := m.minContext
		numMasked := m.ctxNumStats(mc)

		for {
			m.orderFall++
			if m.ctxSuffix(mc) == 0 {
				return -1
			}
			mc = m.ctxSuffix(mc)
			if m.ctxNumStats(mc) != numMasked {
				break
			}
		}
		s := m.ctxStats(mc)

		num := m.ctxNumStats(mc)
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

		count := m.dec.threshold(freqSum)

		if count < hiCnt {
			s = m.ctxStats(m.minContext)
			counted := int32(count)
			for {
				counted -= int32(m.stFreq(s) & uint32(charMask[m.stSymbol(s)]))
				s += 6
				if counted < 0 {
					break
				}
			}
			s -= 6
			m.dec.decode(uint32(int32(count)-counted)-m.stFreq(s), m.stFreq(s))
			m.dec.norm()

			see.update()
			m.foundState = s
			sym := m.stSymbol(s)
			m.update2()
			return int(sym)
		}

		if count >= freqSum {
			return -2
		}

		m.dec.decode(hiCnt, freqSum-hiCnt)

		see.summ = uint16(uint32(see.summ) + freqSum)

		s = m.ctxStats(m.minContext)
		s2 := s + m.ctxNumStats(m.minContext)*6
		for s != s2 {
			charMask[m.stSymbol(s)] = 0
			s += 6
		}
	}
}

// Decode decodes a PPMd-H stream (the 7-Zip coder form) with the
// given order and memory size (bytes), producing exactly outSize
// bytes.  nil means malformed.
func Decode(data []byte, order int, memSize uint32, outSize int64) []byte {
	if order < minOrder || order > maxOrderLim || memSize < minMemSize || outSize < 0 {
		return nil
	}
	m := &model{}
	m.construct()
	m.alloc(memSize)
	m.maxOrder = uint32(order)
	m.restart()
	m.dec = rangeDec{data: data}
	if !m.dec.init() {
		return nil
	}
	out := make([]byte, 0, outSize)
	for int64(len(out)) < outSize {
		sym := m.decodeSymbol()
		if sym < 0 {
			return nil
		}
		out = append(out, byte(sym))
	}
	return out
}

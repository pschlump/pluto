/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd8

// The encoder side of the carryless coder: Low accumulates
// start*Range products, renormalization writes Low's top byte while
// no carry can escape it, and four final shifts flush the tail.  The
// end marker is the escape chain walked all the way to the root —
// symbol -1 encodes no interval of its own.

func (m *model) initEnc() {
	m.rc.low = 0
	m.rc.rng = 0xFFFFFFFF
}

func (r *rcState) writeByte() {
	r.out = append(r.out, byte(r.low>>24))
}

func (r *rcState) normEnc() {
	for {
		if (r.low ^ (r.low + r.rng)) >= rcTop8 {
			if r.rng >= rcBot8 {
				break
			}
			r.rng = (0 - r.low) & (rcBot8 - 1)
		}
		r.writeByte()
		r.rng <<= 8
		r.low <<= 8
	}
}

func (m *model) rcEncode(start, size, total uint32) {
	m.rc.rng /= total
	m.rc.low += start * m.rc.rng
	m.rc.rng *= size
}

func (m *model) flushEnc() {
	for i := 0; i < 4; i++ {
		m.rc.writeByte()
		m.rc.low <<= 8
	}
}

// encodeSymbol encodes one byte (or -1 for the end marker).
func (m *model) encodeSymbol(symbol int) {
	var charMask [256]byte

	if m.ctxNS(m.minContext) != 0 {
		s := m.ctxStats(m.minContext)
		summFreq := m.ctxSummFreq(m.minContext)
		if summFreq > m.rc.rng {
			summFreq = m.rc.rng
		}

		if int(m.stSymbol(s)) == symbol {
			m.rcEncode(0, m.stFreq(s), summFreq)
			m.rc.normEnc()
			m.foundState = s
			m.update1_0()
			return
		}
		m.prevSuccess = 0
		sum := m.stFreq(s)
		i := m.ctxNS(m.minContext)
		for {
			s += 6
			if int(m.stSymbol(s)) == symbol {
				m.rcEncode(sum, m.stFreq(s), summFreq)
				m.rc.normEnc()
				m.foundState = s
				m.update1()
				return
			}
			sum += m.stFreq(s)
			i--
			if i == 0 {
				break
			}
		}

		m.rcEncode(sum, summFreq-sum, summFreq)

		for j := range charMask {
			charMask[j] = 0xFF
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
		prob := m.getBinSumm()
		s := m.oneState(m.minContext)
		pr := uint32(*prob)
		bound := (m.rc.rng >> 14) * pr
		pr -= (pr + (1 << (periodBits - 2))) >> periodBits

		if int(m.stSymbol(s)) == symbol {
			*prob = uint16(pr + 1<<intBits)

			m.rc.rng = bound
			m.rc.normEnc()

			freq := m.stFreq(s)
			c := m.stSucc(s)
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
			return
		}

		*prob = uint16(pr)
		m.initEsc = uint32(m.expEscape[pr>>10])

		m.rc.low += bound
		m.rc.rng = (m.rc.rng &^ (binScale - 1)) - bound

		for j := range charMask {
			charMask[j] = 0xFF
		}
		charMask[m.stSymbol(s)] = 0
		m.prevSuccess = 0
	}

	for {
		m.rc.normEnc()
		mc := m.minContext
		numMasked := m.ctxNS(mc)

		var i uint32
		for {
			m.orderFall++
			if m.ctxSuffix(mc) == 0 {
				return // the end marker: nothing to encode
			}
			mc = m.ctxSuffix(mc)
			i = m.ctxNS(mc)
			if i != numMasked {
				break
			}
		}
		m.minContext = mc

		see, escFreq := m.makeEscFreq(numMasked)

		s := m.ctxStats(mc)
		sum := uint32(0)
		i = m.ctxNS(mc) + 1
		var low, freq uint32

		for {
			if int(m.stSymbol(s)) == symbol {
				low = sum
				freq = m.stFreq(s)
				see.update()
				m.foundState = s
				sum += escFreq

				num2 := i / 2
				odd := i & 1
				if odd == 1 {
					sum += freq
				}
				if num2 != 0 {
					s += odd * 6
					for {
						sym0 := m.stSymbol(s)
						sym1 := m.stSymbol(s + 6)
						s += 12
						sum += m.stFreq(s-12) & uint32(charMask[sym0])
						sum += m.stFreq(s-6) & uint32(charMask[sym1])
						num2--
						if num2 == 0 {
							break
						}
					}
				}
				if sum > m.rc.rng {
					sum = m.rc.rng
				}
				m.rcEncode(low, freq, sum)
				m.rc.normEnc()
				m.update2()
				return
			}
			sum += m.stFreq(s) & uint32(charMask[m.stSymbol(s)])
			s += 6
			i--
			if i == 0 {
				break
			}
		}

		total := sum + escFreq
		see.summ = uint16(uint32(see.summ) + total)
		if total > m.rc.rng {
			total = m.rc.rng
		}
		m.rcEncode(sum, total-sum, total)

		s -= 6
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
	}
}

// Encode compresses data as a zip method-98 PPMd stream (without the
// two-byte props header).  pr chooses the model parameters.  nil
// means the props are out of range.
func Encode(data []byte, pr Props) []byte {
	if !pr.valid() {
		return nil
	}
	m := &model{}
	if !m.init(pr.Order, pr.MemMB, pr.Restor) {
		return nil
	}
	m.initEnc()
	for _, b := range data {
		m.encodeSymbol(int(b))
	}
	m.encodeSymbol(-1) // the end marker
	m.flushEnc()
	return m.rc.out
}

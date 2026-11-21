/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd

// The encoder side: a carry-propagating range coder (64-bit Low with
// the cache/0xFF-run carry flush — LZMA-style) paired with the same
// model the decoder drives.  Its first emitted byte is the zero the
// decoder's init consumes, and five shiftLows at the end flush the
// tail.  Normalization sites mirror the decoder's exactly: the
// double norm after every final encode, a single norm after a binary
// hit, none after a plain encode (the escape intervals inside one
// symbol walk).

type rangeEnc struct {
	low       uint64
	rng       uint32
	cache     byte
	cacheSize uint32
	out       []byte
}

func (r *rangeEnc) init() {
	r.low = 0
	r.rng = 0xFFFFFFFF
	r.cache = 0 // the first byte of the stream
	r.cacheSize = 1
}

// shiftLow flushes the top byte of Low, propagating a carry into the
// pending 0xFF run.
func (r *rangeEnc) shiftLow() {
	if uint32(r.low) < 0xFF000000 || r.low>>32 != 0 {
		temp := r.cache
		for {
			r.out = append(r.out, temp+byte(r.low>>32))
			temp = 0xFF
			r.cacheSize--
			if r.cacheSize == 0 {
				break
			}
		}
		r.cache = byte(uint32(r.low) >> 24)
	}
	r.cacheSize++
	r.low = uint64(uint32(r.low) << 8)
}

func (r *rangeEnc) norm() {
	if r.rng < rcTop {
		r.rng <<= 8
		r.shiftLow()
		if r.rng < rcTop {
			r.rng <<= 8
			r.shiftLow()
		}
	}
}

func (r *rangeEnc) norm1() {
	if r.rng < rcTop {
		r.rng <<= 8
		r.shiftLow()
	}
}

// encode narrows to [start, start+size) of the current total — no
// normalization (the reference's LOCAL norm is empty).
func (r *rangeEnc) encode(start, size uint32) {
	r.low += uint64(start) * uint64(r.rng)
	r.rng *= size
}

func (r *rangeEnc) encodeFinal(start, size uint32) {
	r.encode(start, size)
	r.norm()
}

// encodeSymbol encodes one byte; the model walk mirrors the decoder's
// path for path so both stay bit-identical.
func (m *model) encodeSymbol(symbol int) {
	var charMask [256]byte
	for i := range charMask {
		charMask[i] = 0xFF
	}

	if m.ctxNumStats(m.minContext) != 1 {
		s := m.ctxStats(m.minContext)
		summFreq := m.ctxSummFreq(m.minContext)
		m.enc.rng /= summFreq

		if int(m.stSymbol(s)) == symbol {
			m.enc.encodeFinal(0, m.stFreq(s))
			m.foundState = s
			m.update1_0()
			return
		}
		m.prevSuccess = 0
		sum := m.stFreq(s)
		i := m.ctxNumStats(m.minContext) - 1
		for {
			s += 6
			if int(m.stSymbol(s)) == symbol {
				m.enc.encodeFinal(sum, m.stFreq(s))
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
		m.enc.encode(sum, summFreq-sum)

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
		prob := m.getBinSumm()
		s := m.oneState(m.minContext)
		pr := uint32(*prob)
		bound := (m.enc.rng >> 14) * pr
		pr -= (pr + (1 << (periodBits - 2))) >> periodBits

		if int(m.stSymbol(s)) == symbol {
			*prob = uint16(pr + 1<<intBits)
			m.enc.rng = bound
			m.enc.norm1()

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
			return
		}

		*prob = uint16(pr)
		m.initEsc = uint32(m.expEscape[pr>>10])

		m.enc.low += uint64(bound)
		m.enc.rng -= bound

		charMask[m.stSymbol(s)] = 0
		m.prevSuccess = 0
	}

	for {
		m.enc.norm()
		mc := m.minContext
		numMasked := m.ctxNumStats(mc)

		var i uint32
		for {
			m.orderFall++
			if m.ctxSuffix(mc) == 0 {
				return // the end marker: unreachable, every root state exists
			}
			mc = m.ctxSuffix(mc)
			i = m.ctxNumStats(mc)
			if i != numMasked {
				break
			}
		}
		m.minContext = mc

		// the escape price through SEE (makeEscFreq inlined; i is
		// MinContext's NumStats)
		var see *see
		var escFreq uint32
		if i != 256 {
			nonMasked := i - numMasked
			off := m.hiBitsFlag
			if nonMasked < m.ctxNumStats(m.ctxSuffix(mc))-i {
				off++
			}
			if m.ctxSummFreq(mc) < 11*i {
				off += 2
			}
			if numMasked > nonMasked {
				off += 4
			}
			see = &m.see[m.ns2Indx[nonMasked-1]][off]
			summ := uint32(see.summ)
			r := summ >> see.shift
			see.summ = uint16(summ - r)
			if r == 0 {
				r = 1
			}
			escFreq = r
		} else {
			see = &m.dummySee
			escFreq = 1
		}

		s := m.ctxStats(mc)
		sum := uint32(0)
		var low, freq uint32
		for {
			if int(m.stSymbol(s)) == symbol {
				low = sum
				freq = m.stFreq(s)
				see.update()
				m.foundState = s
				sum += escFreq

				// the remaining masked total from this state on: the
				// found freq enters through the odd-first term or the
				// first pair, never both
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
				m.enc.rng /= sum
				m.enc.encodeFinal(low, freq)
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
		m.enc.rng /= total
		m.enc.encode(sum, escFreq)

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

// Encode compresses data as a PPMd-H stream in the 7-Zip coder form
// with the given order and memory size (bytes).  nil means the
// parameters are out of range.
func Encode(data []byte, order int, memSize uint32) []byte {
	if order < minOrder || order > maxOrderLim || memSize < minMemSize {
		return nil
	}
	m := &model{}
	m.construct()
	m.alloc(memSize)
	m.maxOrder = uint32(order)
	m.restart()
	m.enc.init()
	for _, b := range data {
		m.encodeSymbol(int(b))
	}
	for i := 0; i < 5; i++ {
		m.enc.shiftLow()
	}
	return m.enc.out
}

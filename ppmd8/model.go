/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd8

// The model dynamics.  Everything here runs identically on the encode
// and decode sides — the compressed stream is only meaningful because
// both walks mutate the model in lockstep.

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// refresh redistributes a context's frequencies after symbols were
// removed (restore rollback), raising the escape frequency by the
// amount the removed symbols carried.
func (m *model) refresh(c, oldNU, scale uint32) {
	i := m.ctxNS(c)
	s := m.shrinkUnits(m.ctxStats(c), oldNU, (i+2)>>1)
	m.ctxSetStats(c, s)

	if m.ctxSummFreq(c) >= 1<<15 {
		scale |= 1
	}

	flags := hibitsPrepare(m.stSymbol(s))
	{
		freq := m.stFreq(s)
		escFreq := m.ctxSummFreq(c) - freq
		freq = (freq + scale) >> scale // scale is 0 or 1: addend and shift
		sumFreq := freq
		m.stSetFreq(s, freq)
		for ; i > 0; i-- {
			s += 6
			f := m.stFreq(s)
			escFreq -= f
			f = (f + scale) >> scale
			sumFreq += f
			m.stSetFreq(s, f)
			flags |= hibitsPrepare(m.stSymbol(s))
		}
		m.ctxSetSummFreq(c, sumFreq+((escFreq+scale)>>scale))
	}
	m.ctxSetFlags(c, m.ctxFlags(c)&(flagPrevHigh+flagRescaled*scale)+hibits3of(flags))
}

func hibits3of(flags uint32) uint32 { return (flags >> hibits3Shift) & (1 << 3) }

// cutOff is the cut-off restore walk: converts max-order successors
// to NULL, drops raw and NULL successors below max order, removes
// emptied contexts, and moves stats vectors away from the text area.
// Returns the context ref, or 0 if the context was freed.
func (m *model) cutOff(c, order uint32) uint32 {
	ns := int32(m.ctxNS(c))
	if ns == 0 {
		s := m.oneState(c)
		successor := m.stSucc(s)
		if successor >= m.unitsStart {
			if order < m.maxOrder {
				successor = m.cutOff(successor, order+1)
			} else {
				successor = 0
			}
			m.stSetSucc(s, successor)
			if successor != 0 || order <= 9 { // O_BOUND
				return c
			}
		}
		m.specialFreeUnit(c)
		return 0
	}

	nu := (uint32(ns) + 2) >> 1
	indx := m.u2i(nu)
	stats := m.ctxStats(c)
	if stats-m.unitsStart <= 1<<14 && m.ctxStats(c) <= m.freeList[indx] {
		ptr := m.removeNode(indx)
		m.copyUnits(ptr, stats, nu)
		if stats != m.unitsStart {
			m.insertNode(stats, indx)
		} else {
			m.unitsStart += m.u2b(m.i2u(indx))
		}
		m.ctxSetStats(c, ptr)
		stats = ptr
	}

	{
		s := stats + uint32(ns)*6
		for {
			successor := m.stSucc(s)
			if successor < m.unitsStart {
				s2 := stats + uint32(ns)*6
				ns--
				if order != 0 {
					if s != s2 {
						m.copyState(s, s2)
					}
				} else {
					m.swapStatesAB(s, s2)
					m.stSetSucc(s2, 0)
				}
			} else {
				if order < m.maxOrder {
					m.stSetSucc(s, m.cutOff(successor, order+1))
				} else {
					m.stSetSucc(s, 0)
				}
			}
			if s == stats {
				break
			}
			s -= 6
		}
	}

	if ns != int32(m.ctxNS(c)) && order != 0 {
		if ns < 0 {
			m.freeUnits(stats, nu)
			m.specialFreeUnit(c)
			return 0
		}
		m.ctxSetNS(c, uint32(ns))
		if ns == 0 {
			sym := m.stSymbol(stats)
			m.ctxSetFlags(c, m.ctxFlags(c)&flagPrevHigh+hibits3(sym))
			// *ONE_STATE(c) = *stats, with the frequency rescaled
			m.arena[c+2] = sym
			m.arena[c+3] = byte((m.stFreq(stats) + 11) >> 3)
			m.setU16(c+4, m.u16(stats+2))
			m.setU16(c+6, m.u16(stats+4))
			m.freeUnits(stats, nu)
		} else {
			m.refresh(c, nu, b2u(m.ctxSummFreq(c) > 16*uint32(ns)))
		}
	}

	return c
}

// restoreModel is the memory-exhaustion recovery: roll back the
// symbol the update was adding (from MaxContext down to ctxError),
// raise escapes along the rest of the chain, then restart the model
// — or, under the cut-off method when enough memory remains, prune it
// and keep the accumulated statistics.
func (m *model) restoreModel(ctxError uint32) {
	m.text = 0

	for c := m.maxContext; c != ctxError; c = m.ctxSuffix(c) {
		ns := m.ctxNS(c) - 1
		m.ctxSetNS(c, ns)
		if ns == 0 {
			s := m.ctxStats(c)
			m.ctxSetFlags(c, m.ctxFlags(c)&flagPrevHigh+hibits3(m.stSymbol(s)))
			m.arena[c+2] = m.stSymbol(s)
			m.arena[c+3] = byte((m.stFreq(s) + 11) >> 3)
			m.setU16(c+4, m.u16(s+2))
			m.setU16(c+6, m.u16(s+4))
			m.specialFreeUnit(s)
		} else {
			m.refresh(c, (ns+3)>>1, 0)
		}
	}

	for c := ctxError; c != m.minContext; c = m.ctxSuffix(c) {
		if m.ctxNS(c) == 0 {
			f := uint32(m.arena[c+3])
			m.arena[c+3] = byte((f + 1) >> 1)
		} else {
			sum := m.ctxSummFreq(c) + 4
			m.ctxSetSummFreq(c, sum)
			if sum > 128+4*m.ctxNS(c) {
				m.refresh(c, (m.ctxNS(c)+2)>>1, 1)
			}
		}
	}

	if m.restoreMethod == restoreRestart || m.getUsedMemory() < m.size>>1 {
		m.restartModel()
	} else {
		for m.ctxSuffix(m.maxContext) != 0 {
			m.maxContext = m.ctxSuffix(m.maxContext)
		}
		for {
			m.cutOff(m.maxContext, 0)
			m.expandTextArea()
			if m.getUsedMemory() <= 3*(m.size>>2) {
				break
			}
		}
		m.glueCount = 0
		m.orderFall = m.maxOrder
	}
	m.minContext = m.maxContext
}

// createSuccessors materializes context records for FoundState's raw
// successor and every identical raw successor up the suffix chain.
// skip: don't push FoundState itself (called after the order-0 fall);
// s1: a state to use for the first suffix step (its freq was already
// bumped), 0 otherwise.
func (m *model) createSuccessors(skip bool, s1 uint32, c uint32) uint32 {
	upBranch := m.stSucc(m.foundState)
	var newSym, newFreq byte
	numPs := 0
	var ps [maxOrder + 1]uint32

	if !skip {
		ps[numPs] = m.foundState
		numPs++
	}

	for m.ctxSuffix(c) != 0 {
		var s uint32
		c = m.ctxSuffix(c)
		if s1 != 0 {
			s = s1
			s1 = 0
		} else if m.ctxNS(c) != 0 {
			sym := m.stSymbol(m.foundState)
			s = m.ctxStats(c)
			for m.stSymbol(s) != sym {
				s += 6
			}
			if m.stFreq(s) < maxFreq-9 {
				m.stSetFreq(s, m.stFreq(s)+1)
				m.ctxSetSummFreq(c, m.ctxSummFreq(c)+1)
			}
		} else {
			temp := b2u(m.ctxNS(m.ctxSuffix(c)) == 0)
			s = m.oneState(c)
			f := m.stFreq(s)
			m.stSetFreq(s, f+temp*b2u(f < 24))
		}
		if m.stSucc(s) != upBranch {
			c = m.stSucc(s)
			if numPs == 0 {
				return c
			}
			break
		}
		ps[numPs] = s
		numPs++
	}

	newSym = m.arena[upBranch]
	upBranch++
	flags := byte(hibits4(m.stSymbol(m.foundState)) + hibits3(newSym))

	if m.ctxNS(c) == 0 {
		newFreq = m.arena[c+3]
	} else {
		var cf, s0 uint32
		s := m.ctxStats(c)
		for m.stSymbol(s) != newSym {
			s += 6
		}
		cf = m.stFreq(s) - 1
		s0 = m.ctxSummFreq(c) - m.ctxNS(c) - cf
		if 2*cf <= s0 {
			if 5*cf > s0 {
				newFreq = 2
			} else {
				newFreq = 1
			}
		} else {
			newFreq = byte(1 + (cf+2*s0-3)/s0)
		}
	}

	for {
		var c1 uint32
		if m.hiUnit != m.loUnit {
			m.hiUnit -= unitSize
			c1 = m.hiUnit
		} else if m.freeList[0] != 0 {
			c1 = m.removeNode(0)
		} else {
			c1 = m.allocUnitsRare(0)
			if c1 == 0 {
				return 0
			}
		}
		m.ctxSetFlags(c1, uint32(flags))
		m.ctxSetNS(c1, 0)
		m.arena[c1+2] = newSym
		m.arena[c1+3] = newFreq
		m.stSetSucc(m.oneState(c1), upBranch)
		m.ctxSetSuffix(c1, c)
		numPs--
		m.stSetSucc(ps[numPs], c1)
		c = c1
		if numPs == 0 {
			break
		}
	}
	return c
}

// reduceOrder walks suffix contexts when FoundState's successor is
// NULL, pointing states at the raw text so the model can descend.
func (m *model) reduceOrder(s1 uint32, c uint32) uint32 {
	var s uint32
	c1 := c
	upBranch := m.text

	m.stSetSucc(m.foundState, upBranch)
	m.orderFall++

	for {
		if s1 != 0 {
			c = m.ctxSuffix(c)
			s = s1
			s1 = 0
		} else {
			if m.ctxSuffix(c) == 0 {
				return c
			}
			c = m.ctxSuffix(c)
			if m.ctxNS(c) != 0 {
				s = m.ctxStats(c)
				if m.stSymbol(s) != m.stSymbol(m.foundState) {
					for {
						s += 6
						if m.stSymbol(s) == m.stSymbol(m.foundState) {
							break
						}
					}
				}
				if m.stFreq(s) < maxFreq-9 {
					m.stSetFreq(s, m.stFreq(s)+2)
					m.ctxSetSummFreq(c, m.ctxSummFreq(c)+2)
				}
			} else {
				s = m.oneState(c)
				f := m.stFreq(s)
				m.stSetFreq(s, f+b2u(f < 32))
			}
		}
		if m.stSucc(s) != 0 {
			break
		}
		m.stSetSucc(s, upBranch)
		m.orderFall++
	}

	if m.stSucc(s) <= upBranch {
		s2 := m.foundState
		m.foundState = s
		successor := m.createSuccessors(false, 0, c)
		if successor == 0 {
			m.stSetSucc(s, 0)
		} else {
			m.stSetSucc(s, successor)
		}
		m.foundState = s2
	}

	successor := m.stSucc(s)
	if m.orderFall == 1 && c1 == m.maxContext {
		m.stSetSucc(m.foundState, successor)
		m.text--
	}
	if successor == 0 {
		return 0
	}
	return successor
}

func (m *model) updateModel() {
	minSuccessor := m.stSucc(m.foundState)
	fFreq := m.stFreq(m.foundState)
	fSymbol := m.stSymbol(m.foundState)
	var s1 uint32

	if fFreq < maxFreq/4 && m.ctxSuffix(m.minContext) != 0 {
		c := m.ctxSuffix(m.minContext)
		if m.ctxNS(c) == 0 {
			s1 = m.oneState(c)
			if m.stFreq(s1) < 32 {
				m.stSetFreq(s1, m.stFreq(s1)+1)
			}
		} else {
			s1 = m.ctxStats(c)
			if m.stSymbol(s1) != fSymbol {
				for {
					s1 += 6
					if m.stSymbol(s1) == fSymbol {
						break
					}
				}
				if m.stFreq(s1) >= m.stFreq(s1-6) {
					m.swapStates(s1)
					s1 -= 6
				}
			}
			if m.stFreq(s1) < maxFreq-9 {
				m.stSetFreq(s1, m.stFreq(s1)+2)
				m.ctxSetSummFreq(c, m.ctxSummFreq(c)+2)
			}
		}
	}

	c := m.maxContext
	if m.orderFall == 0 && minSuccessor != 0 {
		cs := m.createSuccessors(true, s1, m.minContext)
		if cs == 0 {
			m.stSetSucc(m.foundState, 0)
			m.restoreModel(c)
			return
		}
		m.stSetSucc(m.foundState, cs)
		m.minContext = cs
		m.maxContext = cs
		return
	}

	text := m.text
	m.arena[text] = fSymbol
	text++
	m.text = text
	if text >= m.unitsStart {
		m.restoreModel(c)
		return
	}
	maxSuccessor := text

	if minSuccessor == 0 {
		cs := m.reduceOrder(s1, m.minContext)
		if cs == 0 {
			m.restoreModel(c)
			return
		}
		minSuccessor = cs
	} else if minSuccessor < m.unitsStart {
		cs := m.createSuccessors(false, s1, m.minContext)
		if cs == 0 {
			m.restoreModel(c)
			return
		}
		minSuccessor = cs
	}

	m.orderFall--
	if m.orderFall == 0 {
		maxSuccessor = minSuccessor
		if m.maxContext != m.minContext {
			m.text--
		}
	}

	flag := hibits3(fSymbol)
	ns := m.ctxNS(m.minContext)
	s0 := m.ctxSummFreq(m.minContext) - ns - fFreq

	for ; c != m.minContext; c = m.ctxSuffix(c) {
		ns1 := m.ctxNS(c)
		var sum uint32
		if ns1 != 0 {
			if ns1&1 != 0 {
				oldNU := (ns1 + 1) >> 1
				i := m.u2i(oldNU)
				if i != m.u2i(oldNU+1) {
					ptr := m.allocUnits(i + 1)
					if ptr == 0 {
						m.restoreModel(c)
						return
					}
					oldPtr := m.ctxStats(c)
					m.copyUnits(ptr, oldPtr, oldNU)
					m.insertNode(oldPtr, i)
					m.ctxSetStats(c, ptr)
				}
			}
			sum = m.ctxSummFreq(c)
			if 3*ns1+1 < ns {
				sum++
			}
		} else {
			s := m.allocUnits(0)
			if s == 0 {
				m.restoreModel(c)
				return
			}
			freq := uint32(m.arena[c+3])
			m.arena[s] = m.arena[c+2]
			m.setU16(s+2, m.u16(c+4))
			m.setU16(s+4, m.u16(c+6))
			m.ctxSetStats(c, s)
			if freq < maxFreq/4-1 {
				freq <<= 1
			} else {
				freq = maxFreq - 4
			}
			m.stSetFreq(s, freq)
			sum = freq + m.initEsc
			if ns > 2 {
				sum++
			}
		}

		s := m.ctxStats(c) + (ns1+1)*6
		cf := 2 * (sum + 6) * fFreq
		sf := s0 + sum
		m.stSetSymbol(s, fSymbol)
		m.ctxSetNS(c, ns1+1)
		m.stSetSucc(s, maxSuccessor)
		m.ctxOrFlags(c, flag)

		if cf < 6*sf {
			v := uint32(1)
			if cf > sf {
				v++
			}
			if cf >= 4*sf {
				v++
			}
			cf = v
			sum += 4
		} else {
			v := uint32(4)
			if cf > 9*sf {
				v++
			}
			if cf > 12*sf {
				v++
			}
			if cf > 15*sf {
				v++
			}
			cf = v
			sum += cf
		}
		m.ctxSetSummFreq(c, sum)
		m.stSetFreq(s, cf)
	}

	m.maxContext = minSuccessor
	m.minContext = minSuccessor
}

func (m *model) rescale() {
	stats := m.ctxStats(m.minContext)
	s := m.foundState

	// move the found state to the front
	if s != stats {
		var tmp [6]byte
		copy(tmp[:], m.arena[s:s+6])
		for s > stats {
			copy(m.arena[s:s+6], m.arena[s-6:s])
			s -= 6
		}
		copy(m.arena[stats:stats+6], tmp[:])
	}

	sumFreq := m.stFreq(s)
	escFreq := m.ctxSummFreq(m.minContext) - sumFreq
	adder := b2u(m.orderFall != 0)

	sumFreq = (sumFreq + 4 + adder) >> 1
	i := m.ctxNS(m.minContext)
	m.stSetFreq(s, sumFreq)

	for ; i > 0; i-- {
		s += 6
		freq := m.stFreq(s)
		escFreq -= freq
		freq = (freq + adder) >> 1
		sumFreq += freq
		m.stSetFreq(s, freq)
		if freq > m.stFreq(s-6) {
			var tmp [6]byte
			copy(tmp[:], m.arena[s:s+6])
			s1 := s
			for {
				copy(m.arena[s1:s1+6], m.arena[s1-6:s1])
				s1 -= 6
				if s1 == stats || freq <= m.stFreq(s1-6) {
					break
				}
			}
			copy(m.arena[s1:s1+6], tmp[:])
		}
	}

	if m.stFreq(s) == 0 {
		i := uint32(0)
		for {
			i++
			s -= 6
			if m.stFreq(s) != 0 {
				break
			}
		}
		escFreq += i
		mc := m.minContext
		numStats := m.ctxNS(mc)
		numStatsNew := numStats - i
		m.ctxSetNS(mc, numStatsNew)
		n0 := (numStats + 2) >> 1

		if numStatsNew == 0 {
			freq := (2*m.stFreq(stats) + escFreq - 1) / escFreq
			if freq > maxFreq/3 {
				freq = maxFreq / 3
			}
			m.ctxSetFlags(mc, m.ctxFlags(mc)&flagPrevHigh+hibits3(m.stSymbol(stats)))
			os := m.oneState(mc)
			m.copyState(os, stats)
			m.stSetFreq(os, freq)
			m.foundState = os
			m.insertNode(stats, m.u2i(n0))
			return
		}

		n1 := (numStatsNew + 2) >> 1
		if n0 != n1 {
			m.ctxSetStats(mc, m.shrinkUnits(stats, n0, n1))
		}
	}

	mc := m.minContext
	m.ctxSetSummFreq(mc, sumFreq+escFreq-(escFreq>>1))
	m.ctxOrFlags(mc, flagRescaled)
	m.foundState = m.ctxStats(mc)
}

// makeEscFreq prices the escape for a masked context through SEE.
func (m *model) makeEscFreq(numMasked uint32) (*see, uint32) {
	mc := m.minContext
	numStats := m.ctxNS(mc)
	if numStats != 0xFF {
		off := b2u(m.ctxSummFreq(mc) > 11*(numStats+1)) +
			2*b2u(2*numStats < m.ctxNS(m.ctxSuffix(mc))+numMasked) +
			m.ctxFlags(mc)
		s := &m.see[m.ns2Indx[numStats+2]-3][off]
		summ := uint32(s.summ)
		r := summ >> s.shift
		s.summ = uint16(summ - r)
		if r == 0 {
			r = 1
		}
		return s, r
	}
	return &m.dummySee, 1
}

func (m *model) nextContext() {
	c := m.stSucc(m.foundState)
	if m.orderFall == 0 && c >= m.unitsStart {
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
	m.prevSuccess = b2u(2*freq >= summFreq) // var.I: >=, not >
	m.runLength += int32(m.prevSuccess)
	m.ctxSetSummFreq(mc, summFreq+4)
	freq += 4
	m.stSetFreq(s, freq)
	if freq > maxFreq {
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

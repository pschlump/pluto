/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package ppmd

// The model dynamics: context creation along suffix chains, the
// per-symbol updates, and rescaling.  Translated literally from the
// reference; every arithmetic detail matters for bit-exact decode
// (the model state IS the compressed stream's meaning).

// copyStates copies n 12-byte units (2 states each) of stats.
func (m *model) copyStates(dst, src, nUnits uint32) {
	copy(m.arena[dst:dst+nUnits*unitSize], m.arena[src:src+nUnits*unitSize])
}

// createStateSucc chain: called when FoundState's successor is raw
// (points into the text); builds real context records for it and
// every identical raw successor up the suffix chain.  Returns the
// ref of the new (or existing) highest context, or 0 on exhaustion.
func (m *model) createSuccessors() uint32 {
	c := m.minContext
	upBranch := m.stSucc(m.foundState)
	var newSym, newFreq byte
	numPs := 0
	var ps [maxOrderLim]uint32

	if m.orderFall != 0 {
		ps[numPs] = m.foundState
		numPs++
	}

	for m.ctxSuffix(c) != 0 {
		c = m.ctxSuffix(c)
		var s uint32
		if m.ctxNumStats(c) != 1 {
			sym := m.stSymbol(m.foundState)
			s = m.ctxStats(c)
			for m.stSymbol(s) != sym {
				s += 6
			}
		} else {
			s = m.oneState(c)
		}
		successor := m.stSucc(s)
		if successor != upBranch {
			c = successor
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

	if m.ctxNumStats(c) == 1 {
		newFreq = byte(m.stFreq(m.oneState(c)))
	} else {
		var cf, s0 uint32
		s := m.ctxStats(c)
		for m.stSymbol(s) != newSym {
			s += 6
		}
		cf = m.stFreq(s) - 1
		s0 = m.ctxSummFreq(c) - m.ctxNumStats(c) - cf
		// the reference folds a leading 1 into both arms: 1..2 in the
		// first, (2cf+s0-1)/(2s0)+2 in the second
		if 2*cf <= s0 {
			if 5*cf > s0 {
				newFreq = 2
			} else {
				newFreq = 1
			}
		} else {
			newFreq = byte((2*cf+s0-1)/(2*s0) + 2)
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
		m.ctxSetNumStats(c1, 1)
		m.stSetSymbol(m.oneState(c1), newSym)
		m.stSetFreq(m.oneState(c1), uint32(newFreq))
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

// swapStates swaps s with its predecessor (s is a state ref).
func (m *model) swapStates(s uint32) {
	var tmp [6]byte
	copy(tmp[:], m.arena[s:s+6])
	copy(m.arena[s:s+6], m.arena[s-6:s])
	copy(m.arena[s-6:s-6+6], tmp[:])
}

// updateModel grows contexts after a symbol decode when the context
// fell short of max order.  Memory exhaustion restarts the model —
// the format's own mechanism.
func (m *model) updateModel() {
	if m.stFreq(m.foundState) < maxFreq/4 && m.ctxSuffix(m.minContext) != 0 {
		c := m.ctxSuffix(m.minContext)
		if m.ctxNumStats(c) == 1 {
			s := m.oneState(c)
			if m.stFreq(s) < 32 {
				m.stSetFreq(s, m.stFreq(s)+1)
			}
		} else {
			s := m.ctxStats(c)
			sym := m.stSymbol(m.foundState)
			if m.stSymbol(s) != sym {
				for {
					s += 6
					if m.stSymbol(s) == sym {
						break
					}
				}
				if m.stFreq(s) >= m.stFreq(s-6) {
					m.swapStates(s)
					s -= 6
				}
			}
			if m.stFreq(s) < maxFreq-9 {
				m.stSetFreq(s, m.stFreq(s)+2)
				m.ctxSetSummFreq(c, m.ctxSummFreq(c)+2)
			}
		}
	}

	if m.orderFall == 0 {
		m.maxContext = m.createSuccessors()
		m.minContext = m.maxContext
		if m.minContext == 0 {
			m.restart()
			return
		}
		m.stSetSucc(m.foundState, m.minContext)
		return
	}

	text := m.text
	m.arena[text] = m.stSymbol(m.foundState)
	text++
	m.text = text
	if text >= m.unitsStart {
		m.restart()
		return
	}
	maxSuccessor := text

	minSuccessor := m.stSucc(m.foundState)

	if minSuccessor != 0 {
		if minSuccessor <= maxSuccessor {
			cs := m.createSuccessors()
			if cs == 0 {
				m.restart()
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
	} else {
		m.stSetSucc(m.foundState, maxSuccessor)
		minSuccessor = m.minContext
	}

	mc := m.minContext
	c := m.maxContext
	m.maxContext = minSuccessor
	m.minContext = minSuccessor

	if c == mc {
		return
	}

	s0 := m.ctxSummFreq(mc) - m.ctxNumStats(mc) - (m.stFreq(m.foundState) - 1)
	ns := m.ctxNumStats(mc)

	for {
		ns1 := m.ctxNumStats(c)
		var sum uint32
		if ns1 != 1 {
			if ns1&1 == 0 {
				oldNU := ns1 >> 1
				i := m.u2i(oldNU)
				if i != m.u2i(oldNU+1) {
					ptr := m.allocUnits(i + 1)
					if ptr == 0 {
						m.restart()
						return
					}
					oldPtr := m.ctxStats(c)
					m.copyStates(ptr, oldPtr, oldNU)
					m.insertNode(oldPtr, i)
					m.ctxSetStats(c, ptr)
				}
			}
			sum = m.ctxSummFreq(c)
			inc := uint32(0)
			if 2*ns1 < ns {
				inc++
			}
			if 4*ns1 <= ns && sum <= 8*ns1 {
				inc += 2
			}
			sum += inc
		} else {
			s := m.allocUnits(0)
			if s == 0 {
				m.restart()
				return
			}
			freq := uint32(m.arena[c+3]) // oneState freq overlays byte 3
			m.stSetSymbol(s, m.arena[c+2])
			m.setU16(s+2, m.u16(c+4))
			m.setU16(s+4, m.u16(c+6))
			m.ctxSetStats(c, s)
			if freq < maxFreq/4-1 {
				freq <<= 1
			} else {
				freq = maxFreq - 4
			}
			m.stSetFreq(s, freq)
			// InitEsc carries an escape frequency value (assigned
			// ExpEscape[pr>>10] by the decoder)
			sum = freq + m.initEsc
			if ns > 3 {
				sum++
			}
		}

		s := m.ctxStats(c) + ns1*6
		cf := 2 * (sum + 6) * m.stFreq(m.foundState)
		sf := s0 + sum
		m.stSetSymbol(s, m.stSymbol(m.foundState))
		m.ctxSetNumStats(c, ns1+1)
		m.stSetSucc(s, maxSuccessor)

		if cf < 6*sf {
			v := uint32(1)
			if cf > sf {
				v++
			}
			if cf >= 4*sf {
				v++
			}
			cf = v
			sum += 3
		} else {
			v := uint32(4)
			if cf >= 9*sf {
				v++
			}
			if cf >= 12*sf {
				v++
			}
			if cf >= 15*sf {
				v++
			}
			cf = v
			sum += cf
		}
		m.ctxSetSummFreq(c, sum)
		m.stSetFreq(s, cf)

		c = m.ctxSuffix(c)
		if c == mc {
			break
		}
	}
}

// rescale halves every frequency in MinContext after MAX_FREQ is
// exceeded, re-sorts, drops zeroed symbols, and may collapse to a
// binary context.
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
		m.foundState = stats
	}

	sumFreq := m.stFreq(s)
	escFreq := m.ctxSummFreq(m.minContext) - sumFreq
	adder := uint32(0)
	if m.orderFall != 0 {
		adder = 1
	}

	sumFreq = (sumFreq + 4 + adder) >> 1
	i := m.ctxNumStats(m.minContext) - 1
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
		numStats := m.ctxNumStats(mc)
		numStatsNew := numStats - i
		m.ctxSetNumStats(mc, numStatsNew)
		n0 := (numStats + 1) >> 1

		if numStatsNew == 1 {
			freq := m.stFreq(stats)
			for { // the C is a do-while: always halves at least once
				escFreq >>= 1
				freq = (freq + 1) >> 1
				if escFreq <= 1 {
					break
				}
			}
			os := m.oneState(mc)
			copy(m.arena[os:os+6], m.arena[stats:stats+6])
			m.stSetFreq(os, freq)
			m.foundState = os
			m.insertNode(stats, m.u2i(n0))
			return
		}

		n1 := (numStatsNew + 1) >> 1
		if n0 != n1 {
			i0 := m.u2i(n0)
			i1 := m.u2i(n1)
			if i0 != i1 {
				if m.freeList[i1] != 0 {
					ptr := m.removeNode(i1)
					m.ctxSetStats(mc, ptr)
					m.copyStates(ptr, stats, n1)
					m.insertNode(stats, i0)
				} else {
					m.splitBlock(stats, i0, i1)
				}
			}
		}
	}
	mc := m.minContext
	m.ctxSetSummFreq(mc, sumFreq+escFreq-(escFreq>>1))
	m.foundState = m.ctxStats(mc)
}

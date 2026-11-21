/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

// Package ppmd8 implements the PPMd variant-I codec (Dmitry Shkarin's
// PPMd var.I, 2002, public domain) in the form zip method 98 uses:
// the same context model over one byte arena with Subbotin's
// carryless range coder, a two-byte property header, and a
// self-terminating stream (the end marker is encoded, unlike the 7z
// PPMd form which is count-driven).  The implementation follows the
// published algorithm; it is a fresh Go translation, not ported code.
//
// Variant I differs from variant H beyond the range coder: NumStats
// is a byte holding count-1, contexts carry a Flags byte that feeds
// the binary probability table, the SEE and BinSumm tables are shaped
// differently, several update constants changed, and memory
// exhaustion no longer restarts the model unconditionally — the
// restore method (restart or cut-off) rebuilds enough state to keep
// going, which is part of the wire format.
package ppmd8

import "encoding/binary"

const (
	minOrder = 2
	maxOrder = 16 // PPMD8_MAX_ORDER: the props field is 4 bits

	unitSize = 12
	maxFreq  = 124

	intBits    = 7
	periodBits = 7
	binScale   = 1 << (intBits + periodBits)

	n1, n2, n3 = 4, 4, 4
	n4         = (128 + 3 - 1*n1 - 2*n2 - 3*n3) / 4
	numIndexes = n1 + n2 + n3 + n4

	restoreRestart = 0
	restoreCutOff  = 1
)

var kExpEscape = [16]byte{25, 14, 9, 7, 5, 5, 4, 4, 4, 3, 3, 3, 2, 2, 2, 2}

var kInitBinEsc = [8]uint16{0x3CDD, 0x1F3F, 0x59BF, 0x48F3, 0x64A1, 0x5ABC, 0x6632, 0x6051}

// Context Flags bits.
const (
	flagRescaled = 1 << 2
	flagPrevHigh = 1 << 4
	hibits3Shift = 8 - 3
	hibits4Shift = 8 - 4
)

// hibitsPrepare is HiBits_Prepare: the symbol's high-bit state as a
// word whose conversion extracts bit 3 or bit 4.
func hibitsPrepare(sym byte) uint32 { return uint32(sym) + 0xC0 }

func hibits3(sym byte) uint32 { return (hibitsPrepare(sym) >> hibits3Shift) & (1 << 3) }
func hibits4(sym byte) uint32 { return (hibitsPrepare(sym) >> hibits4Shift) & (1 << 4) }

// see is one secondary-escape-estimation cell (same shape as var.H).
type see struct {
	summ  uint16
	shift uint8
	count uint8
}

func (s *see) update() {
	if s.shift < periodBits {
		s.count--
		if s.count == 0 {
			s.summ <<= 1
			s.count = 3 << s.shift // the C shifts by the OLD shift
			s.shift++
		}
	}
}

// model is the PPMd-I state machine over one arena.
type model struct {
	minContext, maxContext uint32
	foundState             uint32
	orderFall, initEsc     uint32
	prevSuccess            uint32
	maxOrder               uint32
	restoreMethod          uint32
	runLength, initRL      int32

	size       uint32
	glueCount  uint32
	text       uint32 // refs are byte offsets into arena
	loUnit     uint32
	hiUnit     uint32
	unitsStart uint32
	arena      []byte

	rc rcState

	indx2Units [numIndexes + 2]byte
	units2Indx [128]byte
	freeList   [numIndexes]uint32
	stamps     [numIndexes]uint32

	ns2BSIndx [256]byte
	ns2Indx   [260]byte
	expEscape [16]byte
	dummySee  see
	see       [24][32]see
	binSumm   [25][64]uint16
}

// Arena accessors.  A context is 12 bytes: NumStats (count-1), Flags,
// the SummFreq/State2 overlay, the Stats/State4 overlay, Suffix.
func (m *model) u16(off uint32) uint16 { return binary.LittleEndian.Uint16(m.arena[off:]) }
func (m *model) setU16(off uint32, v uint16) {
	binary.LittleEndian.PutUint16(m.arena[off:], v)
}
func (m *model) u32(off uint32) uint32 { return binary.LittleEndian.Uint32(m.arena[off:]) }
func (m *model) setU32(off uint32, v uint32) {
	binary.LittleEndian.PutUint32(m.arena[off:], v)
}

func (m *model) ctxNS(c uint32) uint32       { return uint32(m.arena[c]) }
func (m *model) ctxSetNS(c uint32, n uint32) { m.arena[c] = byte(n) }
func (m *model) ctxFlags(c uint32) uint32    { return uint32(m.arena[c+1]) }
func (m *model) ctxSetFlags(c uint32, f uint32) {
	m.arena[c+1] = byte(f)
}
func (m *model) ctxOrFlags(c uint32, f uint32) { m.arena[c+1] |= byte(f) }
func (m *model) ctxSummFreq(c uint32) uint32   { return uint32(m.u16(c + 2)) }
func (m *model) ctxSetSummFreq(c uint32, v uint32) {
	m.setU16(c+2, uint16(v))
}
func (m *model) ctxStats(c uint32) uint32       { return m.u32(c + 4) }
func (m *model) ctxSetStats(c uint32, v uint32) { m.setU32(c+4, v) }
func (m *model) ctxSuffix(c uint32) uint32      { return m.u32(c + 8) }
func (m *model) ctxSetSuffix(c uint32, v uint32) {
	m.setU32(c+8, v)
}

// oneState of a binary context overlays bytes 2..7: Symbol, Freq,
// and the successor halves at +4/+6 (before Suffix at +8).
func (m *model) oneState(c uint32) uint32 { return c + 2 }

func (m *model) stSymbol(s uint32) byte       { return m.arena[s] }
func (m *model) stFreq(s uint32) uint32       { return uint32(m.arena[s+1]) }
func (m *model) stSetSymbol(s uint32, v byte) { m.arena[s] = v }
func (m *model) stSetFreq(s uint32, v uint32) { m.arena[s+1] = byte(v) }
func (m *model) stSucc(s uint32) uint32       { return m.u32(s + 2) }
func (m *model) stSetSucc(s, v uint32)        { m.setU32(s+2, v) }

func (m *model) construct() {
	k := 0
	for i := 0; i < numIndexes; i++ {
		step := 4
		if i < 12 {
			step = (i >> 2) + 1
		}
		for j := 0; j < step; j++ {
			m.units2Indx[k] = byte(i)
			k++
		}
		m.indx2Units[i] = byte(k)
	}
	m.ns2BSIndx[0] = 0 << 1
	m.ns2BSIndx[1] = 1 << 1
	for i := 2; i < 11; i++ {
		m.ns2BSIndx[i] = 2 << 1
	}
	for i := 11; i < 256; i++ {
		m.ns2BSIndx[i] = 3 << 1
	}
	for i := 0; i < 5; i++ {
		m.ns2Indx[i] = byte(i)
	}
	mm, k2 := 5, 1
	for i := 5; i < 260; i++ {
		m.ns2Indx[i] = byte(mm)
		k2--
		if k2 == 0 {
			mm++
			k2 = mm - 4
		}
	}
	copy(m.expEscape[:], kExpEscape[:])
}

func (m *model) u2b(nu uint32) uint32 { return nu * unitSize }
func (m *model) u2i(nu uint32) uint32 { return uint32(m.units2Indx[nu-1]) }
func (m *model) i2u(i uint32) uint32  { return uint32(m.indx2Units[i]) }

// ---------- the unit sub-allocator ----------

// Node fields at a unit boundary: Stamp u32, Next u32, NU u32.
const emptyNode = 0xFFFFFFFF

func (m *model) nodeStamp(n uint32) uint32       { return m.u32(n) }
func (m *model) nodeSetStamp(n uint32, v uint32) { m.setU32(n, v) }
func (m *model) nodeNU(n uint32) uint32          { return m.u32(n + 8) }
func (m *model) nodeSetNU(n, v uint32)           { m.setU32(n+8, v) }
func (m *model) nodeNext(n uint32) uint32        { return m.u32(n + 4) }
func (m *model) nodeSetNext(n, v uint32)         { m.setU32(n+4, v) }

func (m *model) insertNode(node uint32, indx uint32) {
	m.nodeSetStamp(node, emptyNode)
	m.nodeSetNext(node, m.freeList[indx])
	m.nodeSetNU(node, m.i2u(indx))
	m.freeList[indx] = node
	m.stamps[indx]++
}

func (m *model) removeNode(indx uint32) uint32 {
	node := m.freeList[indx]
	m.freeList[indx] = m.nodeNext(node)
	m.stamps[indx]--
	return node
}

func (m *model) splitBlock(ptr, oldIndx, newIndx uint32) {
	nu := m.i2u(oldIndx) - m.i2u(newIndx)
	ptr += m.u2b(m.i2u(newIndx))
	i := m.u2i(nu)
	if m.i2u(i) != nu {
		i--
		k := m.i2u(i)
		m.insertNode(ptr+m.u2b(k), nu-k-1)
	}
	m.insertNode(ptr, i)
}

// copyUnits copies n whole 12-byte units.
func (m *model) copyUnits(dst, src, nUnits uint32) {
	copy(m.arena[dst:dst+nUnits*unitSize], m.arena[src:src+nUnits*unitSize])
}

func (m *model) copyState(dst, src uint32) {
	copy(m.arena[dst:dst+6], m.arena[src:src+6])
}

func (m *model) swapStates(s uint32) {
	var tmp [6]byte
	copy(tmp[:], m.arena[s:s+6])
	copy(m.arena[s:s+6], m.arena[s-6:s])
	copy(m.arena[s-6:s], tmp[:])
}

func (m *model) swapStatesAB(a, b uint32) {
	var tmp [6]byte
	copy(tmp[:], m.arena[a:a+6])
	copy(m.arena[a:a+6], m.arena[b:b+6])
	copy(m.arena[b:b+6], tmp[:])
}

// glueFreeBlocks coalesces the free lists into maximal runs and
// refills them.  A node is free iff its Stamp field is EMPTY; the
// guard at LoUnit (stamp 0) bounds the address-adjacency walk.
func (m *model) glueFreeBlocks() {
	m.glueCount = 1 << 13
	m.stamps = [numIndexes]uint32{}
	if m.loUnit != m.hiUnit {
		m.nodeSetStamp(m.loUnit, 0) // guard
	}

	// Build the surviving-node list in traversal order, absorbing
	// address-adjacent free blocks (absorbed nodes get NU 0 and drop).
	var list []uint32
	for i := uint32(0); i < numIndexes; i++ {
		next := m.freeList[i]
		m.freeList[i] = 0
		for next != 0 {
			node := next
			nu := m.nodeNU(node)
			next = m.nodeNext(node)
			if nu != 0 {
				for {
					node2 := node + m.u2b(nu)
					if m.nodeStamp(node2) != emptyNode {
						break
					}
					nu += m.nodeNU(node2)
					m.nodeSetNU(node2, 0)
					m.nodeSetNU(node, nu)
				}
				list = append(list, node)
			}
		}
	}

	for _, node := range list {
		nu := m.nodeNU(node)
		if nu == 0 {
			continue // absorbed by a later node after it was listed
		}
		for ; nu > 128; nu -= 128 {
			m.insertNode(node, numIndexes-1)
			node += m.u2b(128)
		}
		i := m.u2i(nu)
		if m.i2u(i) != nu {
			i--
			k := m.i2u(i)
			m.insertNode(node+m.u2b(k), nu-k-1)
		}
		m.insertNode(node, i)
	}
}

func (m *model) allocUnitsRare(indx uint32) uint32 {
	if m.glueCount == 0 {
		m.glueFreeBlocks()
		if m.freeList[indx] != 0 {
			return m.removeNode(indx)
		}
	}
	i := indx
	for {
		i++
		if i == numIndexes {
			numBytes := m.u2b(m.i2u(indx))
			us := m.unitsStart
			m.glueCount--
			if us-m.text > numBytes {
				m.unitsStart = us - numBytes
				return m.unitsStart
			}
			return 0
		}
		if m.freeList[i] != 0 {
			break
		}
	}
	block := m.removeNode(i)
	m.splitBlock(block, i, indx)
	return block
}

func (m *model) allocUnits(indx uint32) uint32 {
	if m.freeList[indx] != 0 {
		return m.removeNode(indx)
	}
	numBytes := m.u2b(m.i2u(indx))
	lo := m.loUnit
	if m.hiUnit-lo >= numBytes {
		m.loUnit = lo + numBytes
		return lo
	}
	return m.allocUnitsRare(indx)
}

func (m *model) shrinkUnits(oldPtr, oldNU, newNU uint32) uint32 {
	i0 := m.u2i(oldNU)
	i1 := m.u2i(newNU)
	if i0 == i1 {
		return oldPtr
	}
	if m.freeList[i1] != 0 {
		ptr := m.removeNode(i1)
		m.copyUnits(ptr, oldPtr, newNU)
		m.insertNode(oldPtr, i0)
		return ptr
	}
	m.splitBlock(oldPtr, i0, i1)
	return oldPtr
}

func (m *model) freeUnits(ptr, nu uint32) {
	m.insertNode(ptr, m.u2i(nu))
}

// specialFreeUnit frees a context-sized unit, folding it into the
// text area when it sits at its start.
func (m *model) specialFreeUnit(ptr uint32) {
	if ptr != m.unitsStart {
		m.insertNode(ptr, 0)
	} else {
		m.unitsStart += unitSize
	}
}

// expandTextArea absorbs the free blocks sitting directly above
// UnitsStart into the text area, unlinking them from their lists.
func (m *model) expandTextArea() {
	var count [numIndexes]uint32
	if m.loUnit != m.hiUnit {
		m.nodeSetStamp(m.loUnit, 0) // guard
	}
	node := m.unitsStart
	for m.nodeStamp(node) == emptyNode {
		nu := m.nodeNU(node)
		m.nodeSetStamp(node, 0)
		count[m.u2i(nu)]++
		node += m.u2b(nu)
	}
	m.unitsStart = node

	for i := uint32(0); i < numIndexes; i++ {
		cnt := count[i]
		if cnt == 0 {
			continue
		}
		m.stamps[i] -= cnt
		n := m.freeList[i]
		prevIsHead := true
		var prevNode uint32
		for {
			node := n
			n = m.nodeNext(node)
			if m.nodeStamp(node) != 0 {
				prevIsHead = false
				prevNode = node
				continue
			}
			if prevIsHead {
				m.freeList[i] = n
			} else {
				m.nodeSetNext(prevNode, n)
			}
			cnt--
			if cnt == 0 {
				break
			}
		}
	}
}

// getUsedMemory reports allocated bytes (for the restore heuristic).
func (m *model) getUsedMemory() uint32 {
	v := uint32(0)
	for i := uint32(0); i < numIndexes; i++ {
		v += m.stamps[i] * m.i2u(i)
	}
	return m.size - (m.hiUnit - m.loUnit) - (m.unitsStart - m.text) - m.u2b(v)
}

func (m *model) restartModel() {
	m.freeList = [numIndexes]uint32{}
	m.stamps = [numIndexes]uint32{}
	m.text = 0
	m.hiUnit = m.size
	m.loUnit = m.hiUnit - m.size/8/unitSize*7*unitSize
	m.unitsStart = m.loUnit
	m.glueCount = 0

	m.orderFall = m.maxOrder
	m.initRL = -int32(min32(m.maxOrder, 12)) - 1
	m.runLength = m.initRL
	m.prevSuccess = 0

	mc := m.hiUnit - unitSize
	m.hiUnit = mc
	s := m.loUnit
	m.loUnit += m.u2b(256 / 2)
	m.maxContext, m.minContext = mc, mc
	m.foundState = s

	m.ctxSetFlags(mc, 0)
	m.ctxSetNS(mc, 256-1)
	m.ctxSetSummFreq(mc, 256+1)
	m.ctxSetStats(mc, s)
	m.ctxSetSuffix(mc, 0)
	for i := uint32(0); i < 256; i++ {
		m.stSetSymbol(s+i*6, byte(i))
		m.stSetFreq(s+i*6, 1)
		m.stSetSucc(s+i*6, 0)
	}

	// BinSumm rows are frequency buckets; the bucket width feeds the
	// initial value (note: /(i+1), not var.H's /(i+2)).
	i := uint32(0)
	for mm := uint32(0); mm < 25; mm++ {
		for m.ns2Indx[i] == byte(mm) {
			i++
		}
		for k := uint32(0); k < 8; k++ {
			val := uint16(uint32(binScale) - uint32(kInitBinEsc[k])/(i+1))
			for r := uint32(0); r < 64; r += 8 {
				m.binSumm[mm][k+r] = val
			}
		}
	}

	i = 0
	for mm := uint32(0); mm < 24; mm++ {
		for m.ns2Indx[i+3] == byte(mm+3) {
			i++
		}
		summ := uint16((2*i + 5) << (periodBits - 4))
		for k := 0; k < 32; k++ {
			m.see[mm][k].summ = summ
			m.see[mm][k].shift = periodBits - 4
			m.see[mm][k].count = 7
		}
	}

	m.dummySee.summ = 0
	m.dummySee.shift = periodBits
	m.dummySee.count = 64
}

func (m *model) alloc(size uint32) {
	m.arena = make([]byte, size)
	m.size = size
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

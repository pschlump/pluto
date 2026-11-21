/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

// Package ppmd implements the PPMd variant-H context-modeling codec
// (Dmitry Shkarin's PPMd var.H, 2001, public domain) in the form
// 7-Zip uses for native .7z PPMd members: the same model with the
// carryless range coder.  The implementation follows the published
// algorithm; it is a fresh Go translation, not ported code.
//
// The model keeps a tree of contexts (12-byte units in one arena,
// every reference a byte offset): each context holds a statistics
// vector of (symbol, frequency, successor) states plus a suffix link
// to its shorter context.  Coding descends from the longest context;
// an escape (priced by secondary escape estimation) drops to the
// suffix.  Memory exhaustion restarts the model — that restart is
// part of the format, not an error.
package ppmd

import "encoding/binary"

// Wire-observed constants from the reference.
const (
	minOrder    = 2
	maxOrderLim = 64
	minMemSize  = 1 << 11

	unitSize = 12 // one context record, or two states
	maxFreq  = 124

	intBits    = 7
	periodBits = 7
	binScale   = 1 << (intBits + periodBits)

	n1, n2, n3 = 4, 4, 4
	n4         = (128 + 3 - 1*n1 - 2*n2 - 3*n3) / 4
	numIndexes = n1 + n2 + n3 + n4
)

var kExpEscape = [16]byte{25, 14, 9, 7, 5, 5, 4, 4, 4, 3, 3, 3, 2, 2, 2, 2}

var kInitBinEsc = [8]uint16{0x3CDD, 0x1F3F, 0x59BF, 0x48F3, 0x64A1, 0x5ABC, 0x6632, 0x6051}

// see is one secondary-escape-estimation cell.
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

// model is the PPMd-H state machine over one arena.
type model struct {
	minContext, maxContext uint32 // context record refs
	foundState             uint32 // state ref (into a stats vector)
	orderFall, initEsc     uint32
	prevSuccess            uint32
	maxOrder               uint32
	hiBitsFlag             uint32
	runLength, initRL      int32

	size       uint32
	glueCount  uint32
	text       uint32 // refs are byte offsets into arena
	loUnit     uint32
	hiUnit     uint32
	unitsStart uint32
	arena      []byte
	dec        rangeDec
	enc        rangeEnc

	indx2Units [numIndexes + 2]byte
	units2Indx [128]byte
	freeList   [numIndexes]uint32

	ns2BSIndx [256]byte
	ns2Indx   [256]byte
	expEscape [16]byte
	dummySee  see
	see       [25][16]see
	binSumm   [128][64]uint16
}

// Arena accessors: contexts (12 bytes) and states (6 bytes) live at
// byte offsets; successors are stored as two little-endian halves.

func (m *model) u16(off uint32) uint16 { return binary.LittleEndian.Uint16(m.arena[off:]) }
func (m *model) setU16(off uint32, v uint16) {
	binary.LittleEndian.PutUint16(m.arena[off:], v)
}
func (m *model) u32(off uint32) uint32 { return binary.LittleEndian.Uint32(m.arena[off:]) }
func (m *model) setU32(off uint32, v uint32) {
	binary.LittleEndian.PutUint32(m.arena[off:], v)
}

func (m *model) ctxNumStats(c uint32) uint32       { return uint32(m.u16(c)) }
func (m *model) ctxSetNumStats(c uint32, n uint32) { m.setU16(c, uint16(n)) }
func (m *model) ctxSummFreq(c uint32) uint32       { return uint32(m.u16(c + 2)) }
func (m *model) ctxSetSummFreq(c uint32, v uint32) { m.setU16(c+2, uint16(v)) }
func (m *model) ctxStats(c uint32) uint32          { return m.u32(c + 4) }
func (m *model) ctxSetStats(c uint32, v uint32)    { m.setU32(c+4, v) }
func (m *model) ctxSuffix(c uint32) uint32         { return m.u32(c + 8) }
func (m *model) ctxSetSuffix(c uint32, v uint32)   { m.setU32(c+8, v) }

// oneState of a binary context overlays bytes 2..7 of the context.
func (m *model) oneState(c uint32) uint32 { return c + 2 }

func (m *model) stSymbol(s uint32) byte       { return m.arena[s] }
func (m *model) stFreq(s uint32) uint32       { return uint32(m.arena[s+1]) }
func (m *model) stSetSymbol(s uint32, v byte) { m.arena[s] = v }
func (m *model) stSetFreq(s uint32, v uint32) { m.arena[s+1] = byte(v) }
func (m *model) stSucc(s uint32) uint32 {
	return uint32(m.u16(s+2)) | uint32(m.u16(s+4))<<16
}
func (m *model) stSetSucc(s, v uint32) {
	m.setU16(s+2, uint16(v))
	m.setU16(s+4, uint16(v>>16))
}

// hiBitsFlag of a symbol: set for symbols ≥ 0x40.
func hiBitsFlag3(sym byte) uint32 {
	if sym >= 0x40 {
		return 8
	}
	return 0
}

func hiBitsFlag4(sym byte) uint32 {
	if sym >= 0x40 {
		return 16
	}
	return 0
}

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
	for i := 0; i < 3; i++ {
		m.ns2Indx[i] = byte(i)
	}
	mm, k2 := 3, 1
	for i := 3; i < 256; i++ {
		m.ns2Indx[i] = byte(mm)
		k2--
		if k2 == 0 {
			mm++
			k2 = mm - 2
		}
	}
	copy(m.expEscape[:], kExpEscape[:])
}

func (m *model) u2b(nu uint32) uint32 { return nu * unitSize }
func (m *model) u2i(nu uint32) uint32 { return uint32(m.units2Indx[nu-1]) }
func (m *model) i2u(i uint32) uint32  { return uint32(m.indx2Units[i]) }

// alloc allocates the arena.
func (m *model) alloc(size uint32) {
	m.arena = make([]byte, size)
	m.size = size
}

// ---------- the unit sub-allocator ----------

// node fields at a unit boundary: stamp u16, nu u16, next u32.
func (m *model) nodeStamp(n uint32) uint32       { return uint32(m.u16(n)) }
func (m *model) nodeSetStamp(n uint32, v uint32) { m.setU16(n, uint16(v)) }
func (m *model) nodeNU(n uint32) uint32          { return uint32(m.u16(n + 2)) }
func (m *model) nodeSetNU(n, v uint32)           { m.setU16(n+2, uint16(v)) }
func (m *model) nodeNext(n uint32) uint32        { return m.u32(n + 4) }
func (m *model) nodeSetNext(n, v uint32)         { m.setU32(n+4, v) }

const emptyNode = 0

func (m *model) insertNode(node uint32, indx uint32) {
	m.setU32(node, m.freeList[indx])
	m.freeList[indx] = node
}

func (m *model) removeNode(indx uint32) uint32 {
	node := m.freeList[indx]
	m.freeList[indx] = m.u32(node)
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

// glueFreeBlocks coalesces the free lists into maximal runs, then
// refills them.  Stamps at unit starts distinguish free blocks (0)
// from live records (nonzero: a state's Freq or a context's
// NumStats leads at offset 0 for states... a state's Symbol at
// offset 0 could be 0!  The reference relies on the stamp only for
// blocks already in the free lists, never for live ones.)
func (m *model) glueFreeBlocks() {
	m.glueCount = 255
	if m.loUnit != m.hiUnit {
		m.nodeSetStamp(m.loUnit, 1) // guard
	}

	var n uint32
	for i := uint32(0); i < numIndexes; i++ {
		nu := m.i2u(i)
		next := m.freeList[i]
		m.freeList[i] = 0
		for next != 0 {
			tmp := next
			next = m.u32(tmp) // the free-list link lives at offset 0
			m.nodeSetStamp(tmp, emptyNode)
			m.nodeSetNU(tmp, nu)
			m.nodeSetNext(tmp, n)
			n = tmp
		}
	}

	head := n
	// glue adjacent free blocks; prev tracks the predecessor's Next
	// link (the head or a node's field)
	prevIsHead, prevNode := true, uint32(0)
	for n != 0 {
		node := n
		nu := m.nodeNU(n)
		n = m.nodeNext(n)
		if nu == 0 {
			if prevIsHead {
				head = n
			} else {
				m.nodeSetNext(prevNode, n)
			}
			continue
		}
		prevIsHead, prevNode = false, node
		for {
			node2 := node + m.u2b(nu)
			nu += m.nodeNU(node2)
			if m.nodeStamp(node2) != emptyNode || nu >= 0x10000 {
				break
			}
			m.nodeSetNU(node, nu)
			m.nodeSetNU(node2, 0)
		}
	}

	// refill the free lists
	for n := head; n != 0; {
		node := n
		nu := m.nodeNU(n)
		n = m.nodeNext(n)
		if nu == 0 {
			continue
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

func (m *model) restart() {
	m.freeList = [numIndexes]uint32{}
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

	m.ctxSetNumStats(mc, 256)
	m.ctxSetSummFreq(mc, 256+1)
	m.ctxSetStats(mc, s)
	m.ctxSetSuffix(mc, 0)
	for i := uint32(0); i < 256; i++ {
		m.stSetSymbol(s+i*6, byte(i))
		m.stSetFreq(s+i*6, 1)
		m.stSetSucc(s+i*6, 0)
	}

	for i := uint32(0); i < 128; i++ {
		for k := uint32(0); k < 8; k++ {
			val := uint16(uint32(binScale) - uint32(kInitBinEsc[k])/(i+2))
			for mm := uint32(0); mm < 64; mm += 8 {
				m.binSumm[i][k+mm] = val
			}
		}
	}
	for i := uint32(0); i < 25; i++ {
		summ := uint16((5*i + 10) << (periodBits - 4))
		for k := 0; k < 16; k++ {
			m.see[i][k].summ = summ
			m.see[i][k].shift = periodBits - 4
			m.see[i][k].count = 4
		}
	}
	m.dummySee.summ = 0
	m.dummySee.shift = periodBits
	m.dummySee.count = 64
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

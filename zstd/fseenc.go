/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

// The FSE encoder side: a forward bit writer (bits accumulate
// LSB-first, bytes flush little-endian — the inverse of the backward
// reader the decoder uses), the compression-table build from a
// normalized distribution (translated literally from the reference
// FSE_buildCTable_wksp), and the state primitives (initCState2,
// encodeSymbol, flushCState) that pair with it.

// bitWriter is BIT_CStream: first bit written is the last bit read.
type bitWriter struct {
	out []byte
	acc uint64 // pending bits, LSB-first
	n   uint   // number of valid bits in acc
}

func (w *bitWriter) add(v uint32, n uint) {
	if n == 0 {
		return
	}
	v &= 1<<n - 1
	w.acc |= uint64(v) << w.n
	w.n += n
	for w.n >= 8 {
		w.out = append(w.out, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

// close appends the end marker (a single 1 bit) and pads the final
// byte with zeros — the backward reader starts at that marker.
func (w *bitWriter) close() {
	w.add(1, 1)
	if w.n > 0 {
		w.out = append(w.out, byte(w.acc))
		w.acc, w.n = 0, 0
	}
}

// cTransform is one symbol's encoding rule.
type cTransform struct {
	deltaFindState int
	deltaNbBits    uint32
}

// cTable is an FSE compression table: the state transition table plus
// per-symbol transforms.  state values live in [tableSize, 2*tableSize).
type cTable struct {
	log        uint
	stateTable []uint16
	tt         []cTransform
}

// buildCTable mirrors FSE_buildCTable_wksp: the same spread the decode
// table uses (stride, skipping the low-probability tail), states
// assigned to symbols in natural order as tableSize+u, and the
// deltaNbBits / deltaFindState pair per symbol.
func buildCTable(norm []int16, tableLog uint) *cTable {
	size := 1 << tableLog
	mask := size - 1
	step := (size >> 1) + (size >> 3) + 3
	high := size - 1

	cumul := make([]int, len(norm)+1)
	tableSymbol := make([]byte, size)
	cumul[0] = 0
	for s, c := range norm {
		if c == -1 { // low-probability symbols take the table's tail
			tableSymbol[high] = byte(s)
			high--
			cumul[s+1] = cumul[s] + 1
		} else {
			cumul[s+1] = cumul[s] + int(c)
		}
	}
	if high == size-1 {
		// no low-probability symbols: plain stride spread
		position := 0
		for s, c := range norm {
			for i := 0; i < int(c); i++ {
				tableSymbol[position] = byte(s)
				position = (position + step) & mask
			}
		}
	} else {
		position := 0
		for s, c := range norm {
			for i := 0; i < int(c); i++ {
				tableSymbol[position] = byte(s)
				position = (position + step) & mask
				for position > high {
					position = (position + step) & mask
				}
			}
		}
	}

	t := &cTable{log: tableLog, stateTable: make([]uint16, size), tt: make([]cTransform, len(norm))}
	// states, sorted by symbol: decode state u belongs to tableSymbol[u]
	for u := 0; u < size; u++ {
		s := tableSymbol[u]
		t.stateTable[cumul[s]] = uint16(size + u)
		cumul[s]++
	}
	// per-symbol transforms
	total := 0
	for s, c := range norm {
		switch c {
		case 0:
			t.tt[s].deltaNbBits = uint32(tableLog+1)<<16 - uint32(1)<<tableLog
		case -1, 1:
			t.tt[s].deltaNbBits = uint32(tableLog)<<16 - uint32(1)<<tableLog
			t.tt[s].deltaFindState = total - 1
			total++
		default:
			maxBitsOut := tableLog - uint(highbit32(uint32(c)-1))
			minStatePlus := uint32(c) << maxBitsOut
			t.tt[s].deltaNbBits = uint32(maxBitsOut)<<16 - minStatePlus
			t.tt[s].deltaFindState = total - int(c)
			total += int(c)
		}
	}
	return t
}

// cState is one FSE encoding state.
type cState struct {
	t *cTable
	v uint32
}

// init2 seeds the state for a known symbol (the first the encoder
// processes, i.e. the last the decoder reads), taking the smallest
// state value that decodes it.
func (s *cState) init2(t *cTable, symbol int) {
	s.t = t
	tt := t.tt[symbol]
	nb := (tt.deltaNbBits + 1<<15) >> 16
	v := nb<<16 - tt.deltaNbBits
	s.v = uint32(t.stateTable[(v>>nb)+uint32(tt.deltaFindState)])
}

// encode writes the current state's bits and transitions to a state
// decoding symbol.
func (s *cState) encode(w *bitWriter, symbol int) {
	tt := s.t.tt[symbol]
	nb := (s.v + tt.deltaNbBits) >> 16
	w.add(s.v, uint(nb))
	s.v = uint32(s.t.stateTable[(s.v>>nb)+uint32(tt.deltaFindState)])
}

// flush writes the final state value — the decoder's init read.
func (s *cState) flush(w *bitWriter) {
	w.add(s.v, s.t.log)
}

// The per-family symbol maxima and table-log maxima the format fixes.
const (
	llMaxSym = 35
	ofMaxSym = 31
	mlMaxSym = 52
)

// The predefined distributions' compression tables.
var (
	llCTab = buildCTable(llNorm, 6)
	mlCTab = buildCTable(mlNorm, 6)
	ofCTab = buildCTable(ofNorm, 5)
)

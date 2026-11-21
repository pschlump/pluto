/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package zstd

import (
	"math"
	"sort"
)

// Custom FSE sequence tables: normalize the block's actual code
// statistics, write the table description (the forward LSB-first
// normalized-count header readNCount parses), and pick per table
// between predefined, RLE, fresh-FSE, and repeat modes by an
// estimated bit cost.

// fwdBitWriter is the forward LSB-first accumulator the table
// descriptions use.
type fwdBitWriter struct {
	out []byte
	acc uint32
	n   uint
}

func (w *fwdBitWriter) add(v uint32, n uint) {
	w.acc |= v << w.n
	w.n += n
	for w.n >= 8 {
		w.out = append(w.out, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
	}
}

func (w *fwdBitWriter) flush() []byte {
	if w.n > 0 {
		w.out = append(w.out, byte(w.acc))
		w.acc, w.n = 0, 0
	}
	return w.out
}

// writeNCount serializes a normalized distribution exactly as
// readNCount parses it: the accuracy log in four low bits of the
// first byte, then per-symbol counts in a width that shrinks as the
// remaining budget does (counts below the threshold one bit short),
// with a zero symbol's entry followed by 2-bit repeat fields for the
// run of further zeros.  The stream ends after the last non-zero
// symbol — trailing zeros are never written.
func writeNCount(norm []int16, log uint) []byte {
	w := &fwdBitWriter{}
	w.add(uint32(log-5), 4)
	remaining := int32(1)<<log + 1
	threshold := int32(1) << log
	nbBits := uint(log + 1)
	prev0 := false
	sym := 0
	for remaining > 1 {
		if prev0 {
			// zeros between the last written symbol and the next
			// non-zero one: three-at-a-time 2-bit fields, then the
			// tail (a field below 3 ends the run)
			start := sym
			for sym < len(norm) && norm[sym] == 0 {
				sym++
			}
			for run := sym - start; run >= 3; run -= 3 {
				w.add(3, 2)
			}
			w.add(uint32(sym-start)%3, 2)
		}
		if sym >= len(norm) {
			break // unreachable for well-formed distributions
		}
		c := norm[sym]
		sym++
		max := (2*threshold - 1) - remaining
		if c < 0 {
			remaining += int32(c)
		} else {
			remaining -= int32(c)
		}
		v := int32(c) + 1
		if v >= threshold {
			v += max
		}
		if v < max {
			w.add(uint32(v), nbBits-1)
		} else {
			w.add(uint32(v), nbBits)
		}
		prev0 = c == 0
		for remaining < threshold {
			nbBits--
			threshold >>= 1
		}
	}
	return w.flush()
}

// optimalTableLog follows libzstd's heuristic: the accuracy a sample
// of nbSeq symbols can support (highbit minus two), at least enough
// bits to index every present symbol, clamped to the format's per-
// table maximum.
func optimalTableLog(nbSeq, maxSym int, maxLog uint) uint {
	log := maxLog
	if nbSeq > 1 {
		if mb := uint(highbit32(uint32(nbSeq - 1))); mb >= 2 && mb-2 < log {
			log = mb - 2
		}
	}
	if mb := uint(highbit32(uint32(maxSym))) + 2; mb > log {
		log = mb
	}
	if log < 5 {
		log = 5
	}
	if log > maxLog {
		log = maxLog
	}
	return log
}

// normalizeCounts scales raw counts to a distribution summing to
// exactly 2^log: proportional floors, the deficit to the largest
// remainders, and every present symbol guaranteed at least one cell.
// nil means the statistics do not fit (caller falls back).
func normalizeCounts(counts []uint32, maxSym int, maxLog uint) ([]int16, uint) {
	total, present := 0, 0
	for _, c := range counts {
		total += int(c)
		if c > 0 {
			present++
		}
	}
	if total == 0 || present < 2 {
		return nil, 0
	}
	log := optimalTableLog(total, maxSym, maxLog)
	for present > 1<<log && log < maxLog {
		log++
	}
	if present > 1<<log {
		return nil, 0
	}
	size := uint64(1) << log

	norm := make([]int16, len(counts))
	type rem struct {
		s int
		r uint64
	}
	var rems []rem
	assigned := 0
	for s, c := range counts {
		if c == 0 {
			continue
		}
		v := uint64(c) * size / uint64(total)
		norm[s] = int16(v)
		assigned += int(v)
		rems = append(rems, rem{s, uint64(c) * size % uint64(total)})
	}
	sort.Slice(rems, func(i, j int) bool { return rems[i].r > rems[j].r })
	for i := 0; i < len(rems) && assigned < int(size); i++ {
		norm[rems[i].s]++
		assigned++
	}
	for s, c := range counts {
		if c == 0 || norm[s] != 0 {
			continue
		}
		norm[s] = 1
		best := -1
		for s2, v := range norm {
			if v > 1 && (best < 0 || v > norm[best]) {
				best = s2
			}
		}
		if best < 0 {
			return nil, 0
		}
		norm[best]--
	}
	if assigned != int(size) {
		return nil, 0
	}
	return norm, log
}

// normCost estimates the FSE bit cost of coding the counted symbols
// under a distribution (log2 of the table share per symbol; a -1
// cell codes at the full accuracy, the reset rule).
func normCost(counts []uint32, norm []int16, log uint) float64 {
	size := float64(uint32(1) << log)
	bits := 0.0
	for s, c := range counts {
		if c == 0 {
			continue
		}
		switch p := norm[s]; {
		case p == 0:
			return math.Inf(1)
		case p < 0:
			bits += float64(c) * float64(log)
		default:
			bits += float64(c) * math.Log2(size/float64(p))
		}
	}
	return bits
}

// rleCTable is the one-state table of RLE_Mode: log 0, no bits ever.
func rleCTable(sym int) *cTable {
	t := &cTable{log: 0, stateTable: []uint16{0}, tt: make([]cTransform, sym+1)}
	return t
}

// famState remembers one family's table between blocks of a frame,
// which is what repeat mode reuses.
type famState struct {
	custom bool // the remembered table is not the predefined one
	norm   []int16
	log    uint
	tab    *cTable
}

// seqCodec carries the three families' state across the blocks of
// one frame.
type seqCodec struct {
	ll, of, ml famState
}

// famChoice is the per-block, per-family decision.
type famChoice struct {
	mode   int // 0 predefined, 1 RLE, 2 FSE, 3 repeat
	desc   []byte
	rleSym int
	norm   []int16
	log    uint
	tab    *cTable
}

func chooseFam(counts []uint32, maxSym int, maxLog uint, preNorm []int16, preLog uint, preTab *cTable, prev *famState) famChoice {
	best := famChoice{mode: 0, norm: preNorm, log: preLog, tab: preTab}
	bestCost := normCost(counts, preNorm, preLog)

	distinct, onlySym := 0, 0
	for s, c := range counts {
		if c > 0 {
			distinct++
			onlySym = s
		}
	}
	if distinct == 1 {
		if cost := 8.0; cost < bestCost { // the single description byte
			rleNorm := make([]int16, len(counts))
			rleNorm[onlySym] = 1
			best = famChoice{mode: 1, rleSym: onlySym, norm: rleNorm, log: 0, tab: rleCTable(onlySym)}
			bestCost = cost
		}
	}
	if prev.custom {
		if cost := normCost(counts, prev.norm, prev.log); cost < bestCost {
			best = famChoice{mode: 3, norm: prev.norm, log: prev.log, tab: prev.tab}
			bestCost = cost
		}
	}
	if distinct >= 2 {
		if norm, log := normalizeCounts(counts, maxSym, maxLog); norm != nil {
			desc := writeNCount(norm, log)
			// the estimate is approximate; a description must pay a
			// margin to displace predefined tables
			if cost := normCost(counts, norm, log) + 8*float64(len(desc)) + 24; cost < bestCost {
				best = famChoice{mode: 2, desc: desc, norm: norm, log: log, tab: buildCTable(norm, log)}
			}
		}
	}
	return best
}

// apply updates the family state after the block is written.
func (f *famState) apply(c famChoice) {
	switch c.mode {
	case 0:
		f.custom = false
	case 1, 2:
		f.custom = true
		f.norm, f.log, f.tab = c.norm, c.log, c.tab
	case 3:
		// the repeated table stays in force
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package zstd

import (
	"encoding/binary"
	"math"
)

// Probe bounds: matches longer than a couple of kilobytes price
// barely differently (the ML extra bits are logarithmic in effect),
// so the scan stops early — on perfectly periodic data an uncapped
// probe would compare to the end of every block at every position.
const (
	maxMatchProbe = 2048
	sufficientLen = 256
	repProbeBelow = 64
)

// The optimal parse: a shortest-path DP over the block.  Every
// position can end a literal step or a sequence step (a match at that
// position, coded either by its full offset or — when the distance is
// one of the frame's repeat offsets — by the cheap 1..3 rep value),
// and each cell keeps the cheapest prefix plus the repeat state that
// prefix implies.  Prices are estimated: literal bytes at their block
// entropy (what the Huffman literal coder then delivers), sequence
// codes at the predefined tables' log-shares plus extra bits.

// optCell is one DP state: the cheapest way to encode the prefix
// ending here, how it was reached, and the repeat history it implies.
type optCell struct {
	bits   float64
	prev   int32
	mlen   int32 // the step INTO this cell: 0 literal, else match length
	offVal int32 // the step's wire offset value (full-3, or 1..3 rep)
	litlen int32 // consecutive literal steps ending here
	rep0   int32
	rep1   int32
	rep2   int32
}

// priceModel holds the per-block literal entropy and the static
// sequence-code estimates.
type priceModel struct {
	lit [256]float64
}

// Predefined-table FSE bit estimates: log2(2^log / norm), with the -1
// "less than one" cells priced at the full accuracy (the reset rule).
var llEst, mlEst, ofEst [53]float64

func init() {
	for i, n := range llNorm {
		llEst[i] = estBits(int(n), 6)
	}
	for i, n := range mlNorm {
		mlEst[i] = estBits(int(n), 6)
	}
	for i, n := range ofNorm {
		ofEst[i] = estBits(int(n), 5)
	}
}

func estBits(norm, log int) float64 {
	if norm < 0 {
		return float64(log)
	}
	return float64(log) - math.Log2(float64(norm))
}

// newPriceModel builds the literal byte costs from the block's own
// statistics, clamped to the Huffman coder's range.
func newPriceModel(b []byte) *priceModel {
	p := &priceModel{}
	var count [256]int
	for _, c := range b {
		count[c]++
	}
	for s, n := range count {
		switch {
		case n == 0:
			p.lit[s] = 9.5
		case n == len(b):
			p.lit[s] = 0.05
		default:
			v := math.Log2(float64(len(b)) / float64(n))
			if v < 0.05 {
				v = 0.05
			}
			if v > 11 {
				v = 11
			}
			p.lit[s] = v
		}
	}
	return p
}

// seqBits prices one sequence: the LL/ML/OF FSE estimates plus the
// extra bits, given the wire offset value.
func seqBits(ll, ml int, offVal int32) float64 {
	bits := 0.0
	llc := llCodeFor(ll)
	bits += llEst[llc] + float64(llBits[llc])
	mlc := mlCodeFor(ml)
	bits += mlEst[mlc] + float64(mlBits[mlc])
	ofc := highbit32(uint32(offVal))
	bits += ofEst[ofc] + float64(ofc)
	return bits
}

// optimalSeqs turns src[from:to] into sequences by shortest path.
// The finder's hash chain is seeded position by position, so every
// candidate is history the decoder has already produced.
func (f *matchFinder) optimalSeqs(src []byte, from, to, window int) []seq {
	n := to - from
	pm := newPriceModel(src[from:to])

	cells := make([]optCell, n+1)
	for i := 1; i <= n; i++ {
		cells[i].bits = math.Inf(1)
	}
	repStart0, repStart1, repStart2 := f.rep0, f.rep1, f.rep2
	cells[0] = optCell{rep0: int32(repStart0), rep1: int32(repStart1), rep2: int32(repStart2)}

	// updateReps applies the wire offset's repeat-history update
	// (the encoder side of resolveOffset; reps only ever emit with
	// literals ahead, so the zero-literals shift never triggers).
	updateReps := func(rep [3]int32, offVal int32, dist int) [3]int32 {
		if offVal > 3 {
			return [3]int32{int32(dist), rep[0], rep[1]}
		}
		switch offVal {
		case 1:
			return [3]int32{rep[0], rep[1], rep[2]}
		case 2:
			return [3]int32{rep[1], rep[0], rep[2]}
		default:
			return [3]int32{rep[2], rep[0], rep[1]}
		}
	}

	relaxMatch := func(dst int, bits float64, prev int, ml int, offVal, dist int, rep [3]int32) {
		nb := cells[prev].bits + bits
		if nb < cells[dst].bits {
			cells[dst] = optCell{
				bits:   nb,
				prev:   int32(prev),
				mlen:   int32(ml),
				offVal: int32(offVal),
				rep0:   rep[0],
				rep1:   rep[1],
				rep2:   rep[2],
			}
		}
	}

	for i := 0; i < n; i++ {
		c := cells[i]
		abs := from + i

		// the literal step
		nl := c.bits + pm.lit[src[abs]]
		if nl < cells[i+1].bits {
			cells[i+1] = optCell{
				bits:   nl,
				prev:   int32(i),
				mlen:   0,
				litlen: c.litlen + 1,
				rep0:   c.rep0,
				rep1:   c.rep1,
				rep2:   c.rep2,
			}
		}

		if abs+4 > to {
			continue // too few bytes for a chain hash
		}
		f.insert(src, abs)
		cellRep := [3]int32{c.rep0, c.rep1, c.rep2}
		probeLim := to
		if abs+maxMatchProbe < to {
			probeLim = abs + maxMatchProbe
		}

		// hash-chain matches: the longest, and the shortest offset
		// (a nearer match can price cheaper than a longer far one)
		bestLen, bestOff := 0, 0
		cheapLen, cheapOff := 0, 0
		cand := int(f.head[f.hash4(src[abs:])])
		for d := 0; d < chainDepth && cand >= 0; d++ {
			dist := abs - cand
			if dist > window {
				break
			}
			if dist > 0 && cand+4 <= abs &&
				binary.LittleEndian.Uint32(src[cand:]) == binary.LittleEndian.Uint32(src[abs:]) {
				if l := matchLen(src, cand, abs, probeLim, 4); l > 0 {
					if l > bestLen {
						bestLen, bestOff = l, dist
					}
					if cheapOff == 0 || dist < cheapOff {
						cheapLen, cheapOff = l, dist
					}
				}
			}
			if bestLen >= sufficientLen {
				break // long enough; more candidates cannot pay
			}
			cand = int(f.prev[cand&chainMask])
			if cand >= abs {
				break // stale ring entry
			}
		}
		for _, cand := range [][2]int{{bestLen, bestOff}, {cheapLen, cheapOff}} {
			ml, dist := cand[0], cand[1]
			if ml < 4 {
				continue
			}
			relaxMatch(i+ml, seqBits(int(c.litlen), ml, int32(dist+3)),
				i, ml, dist+3, dist, updateReps(cellRep, int32(dist+3), dist))
		}

		// repeat-offset matches: the cheap 1..3 wire values, valid
		// only with at least one literal ahead (the zero-literals
		// shift would select a different offset).  Probed only when
		// the chain found nothing decent — a long chain match makes
		// the repeats redundant.
		if c.litlen >= 1 && bestLen < repProbeBelow {
			for idx, r := range cellRep {
				if r <= 0 || int(r) > window || abs-int(r) < 0 {
					continue
				}
				ml := matchLen(src, abs-int(r), abs, probeLim, 3)
				if ml < 3 {
					continue
				}
				relaxMatch(i+ml, seqBits(int(c.litlen), ml, int32(idx+1)),
					i, ml, idx+1, int(r), updateReps(cellRep, int32(idx+1), int(r)))
			}
		}
	}

	// backtrack the cheapest path into sequences
	type step struct {
		start, ml, offVal int
		dist              int
	}
	var steps []step
	for i := n; i > 0; {
		c := cells[i]
		if c.mlen == 0 {
			i--
			continue
		}
		prevCell := cells[c.prev]
		dist := int(c.offVal)
		if c.offVal <= 3 {
			// a rep value resolves against the prefix's repeat state
			switch c.offVal {
			case 1:
				dist = int(prevCell.rep0)
			case 2:
				dist = int(prevCell.rep1)
			default:
				dist = int(prevCell.rep2)
			}
		} else {
			dist -= 3 // the wire value is distance+3
		}
		steps = append(steps, step{start: int(c.prev), ml: int(c.mlen), offVal: int(c.offVal), dist: dist})
		i = int(c.prev)
	}
	// steps came out last-first
	var out []seq
	end := from // the prefix end so far
	for j := len(steps) - 1; j >= 0; j-- {
		st := steps[j]
		s := seq{litLen: st.start + from - end, offset: st.dist, matchLen: st.ml}
		if st.offVal <= 3 {
			s.offWire = st.offVal
		}
		out = append(out, s)
		end = st.start + from + st.ml
	}
	// Merge adjacent zero-literal sequences at the same distance:
	// the byte pattern is identical (a match at d then another at d
	// with no literals is one longer match at d), and it undoes the
	// probe cap's splitting of very long matches.  Merging changes
	// the repeat history, so every later rep-coded sequence is then
	// verified against the real state and rewritten to its full
	// offset when it no longer resolves.
	if len(out) > 1 {
		merged := out[:1]
		for _, s := range out[1:] {
			last := &merged[len(merged)-1]
			if s.offset > 0 && s.litLen == 0 && s.offset == last.offset {
				last.matchLen += s.matchLen
				continue
			}
			merged = append(merged, s)
		}
		out = merged
	}
	rep := [3]int{repStart0, repStart1, repStart2}
	for i := range out {
		s := &out[i]
		if s.offset == 0 {
			continue
		}
		if s.offWire != 0 {
			if rep[s.offWire-1] != s.offset {
				s.offWire = 0 // no longer a repeat; use the full form
			}
		}
		switch s.offWire {
		case 1:
			// rep0 stays rep0
		case 2:
			rep[1], rep[0] = rep[0], rep[1]
		case 3:
			rep[2], rep[1], rep[0] = rep[1], rep[0], rep[2]
		default:
			rep[2], rep[1], rep[0] = rep[1], rep[0], s.offset
		}
	}

	// the frame's repeat state rides forward
	f.rep0, f.rep1, f.rep2 = rep[0], rep[1], rep[2]
	if end < to {
		out = append(out, seq{litLen: to - end})
	}
	return out
}

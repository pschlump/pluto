/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import "encoding/binary"

// The compressor: a greedy match finder (4-byte single-slot hash
// table, backward extension, repeat-offset reuse) feeding sequences
// through the predefined FSE tables, with raw or RLE literals.  Each
// 128 KiB block is matched against the whole frame so far (up to the
// 8 MiB window); blocks that do not shrink are stored raw, the
// reference writer's rule.

// seq is one LZ77 command: litLen literals, then a match of matchLen
// bytes at offset.  offset 0 marks the run of trailing literals.
type seq struct {
	litLen   int
	offset   int
	matchLen int
}

// matchFinder finds matches over one frame: a hash chain (head plus
// prev links, the lz4 structure) walked a bounded number of
// candidates, so repetitive-but-not-identical data still finds long
// matches a single-slot table would miss.
type matchFinder struct {
	head []int32 // hash -> most recent position with that hash
	prev []int32 // position -> previous position with the same hash
	rep  int     // the frame's current repeat offset (0 before any match)
}

const chainMask = (1 << 18) - 1
const chainDepth = 8

func newMatchFinder() *matchFinder {
	f := &matchFinder{head: make([]int32, 1<<17), prev: make([]int32, chainMask+1)}
	for i := range f.head {
		f.head[i] = -1
	}
	return f
}

// insert links a position into its hash chain.
func (f *matchFinder) insert(src []byte, pos int) {
	h := f.hash4(src[pos:])
	f.prev[pos&chainMask] = f.head[h]
	f.head[h] = int32(pos)
}

func (f *matchFinder) hash4(b []byte) uint32 {
	v := binary.LittleEndian.Uint32(b)
	return (v * 2654435761) >> 15
}

// blockSeqs turns src[from:to] into sequences.  Candidates may reach
// anywhere in src[..from] within the window.  The finder's table is
// seeded with every position of the whole input up front, so later
// blocks see earlier ones; positions consumed inside the match re-seed
// sparsely as the scan advances.
func (f *matchFinder) blockSeqs(src []byte, from, to, window int) []seq {
	var out []seq
	litStart := from
	i := from
	for i+4 <= to {
		f.insert(src, i)

		best, bestLen := 0, 0
		isRep := false
		// repeat offset: only with literals ahead of it (a rep with
		// zero literals selects rep1, a different offset, in the
		// format), and min 3
		if f.rep > 0 && i > litStart && i-f.rep >= 0 && f.rep <= window {
			if l := matchLen(src, i-f.rep, i, to, 3); l >= 3 {
				best, bestLen, isRep = f.rep, l, true
			}
		}
		// hash chain: walk the candidates, longest wins; equal-length
		// matches prefer the closer (cheaper offset code)
		cand := int(f.head[f.hash4(src[i:])])
		for n := 0; n < chainDepth && cand >= 0; n++ {
			dist := i - cand
			if dist > window {
				break
			}
			if dist > 0 && dist != best && cand+4 <= i &&
				binary.LittleEndian.Uint32(src[cand:]) == binary.LittleEndian.Uint32(src[i:]) {
				if l := matchLen(src, cand, i, to, 4); l > bestLen {
					best, bestLen = dist, l
				}
			}
			cand = int(f.prev[cand&chainMask])
			if cand >= i {
				break // stale entry from a wrapped ring
			}
		}

		if bestLen < 4 {
			i++
			continue
		}
		// lazy matching: when the very next position starts a longer
		// match, this one is not worth taking — emit a literal
		if i+1+4 <= to && !isRep {
			probe := int(f.head[f.hash4(src[i+1:])])
			if probe == i {
				probe = int(f.prev[probe&chainMask])
			}
			if probe >= 0 && i+1-probe > 0 && i+1-probe <= window {
				if binary.LittleEndian.Uint32(src[probe:]) == binary.LittleEndian.Uint32(src[i+1:]) {
					if l := matchLen(src, probe, i+1, to, 4); l > bestLen {
						i++
						continue
					}
				}
			}
		}
		// backward extension into the pending literals: the source
		// moves down with the match, so the byte to compare at i-1
		// is at i-1-best; a rep match keeps at least one literal so
		// it stays a rep-0 code
		for i > litStart && best < window && i-1-best >= 0 && src[i-1] == src[i-1-best] &&
			(!isRep || i-1 > litStart) {
			i--
			bestLen++
		}
		out = append(out, seq{litLen: i - litStart, offset: best, matchLen: bestLen})
		f.rep = best
		i += bestLen
		litStart = i
		for p := i - bestLen; p+4 <= i; p++ {
			f.insert(src, p)
		}
	}
	if litStart < to {
		out = append(out, seq{litLen: to - litStart})
	}
	return out
}

// matchLen counts the common run at a and b (a < b, overlap allowed),
// bounded by to; 0 when shorter than min.
func matchLen(src []byte, a, b, to, min int) int {
	l := 0
	for b+l < to && src[a+l] == src[b+l] {
		l++
	}
	if l < min {
		return 0
	}
	return l
}

// llCodeFor maps a literals length to its LL code (binary search over
// the baselines).
func llCodeFor(ll int) int {
	if ll < 16 {
		return ll
	}
	lo, hi := 16, len(llBase)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if llBase[mid] <= uint32(ll) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// mlCodeFor maps a match length (>= 3) to its ML code.
func mlCodeFor(ml int) int {
	if ml < 35 {
		return ml - 3
	}
	lo, hi := 32, len(mlBase)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if mlBase[mid] <= uint32(ml) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// compressBlockBody builds one compressed block body for src[from:to]
// (matching against src[..from]), or returns nil when it would not
// beat the stored form.
func compressBlockBody(f *matchFinder, src []byte, from, to, window int) []byte {
	seqs := f.blockSeqs(src, from, to, window)
	var literals []byte
	pos := from
	for _, s := range seqs {
		literals = append(literals, src[pos:pos+s.litLen]...)
		if s.offset == 0 {
			break
		}
		pos += s.litLen + s.matchLen
	}
	litSec := literalsSection(literals)
	if h := huffmanLiterals(literals); h != nil && len(h) < len(litSec) {
		litSec = h
	}
	body := append(litSec, sequencesSection(seqs)...)
	if len(body) >= to-from {
		return nil
	}
	return body
}

// literalsSection encodes the literals block: RLE when one byte
// repeats, raw otherwise.
func literalsSection(lit []byte) []byte {
	n := len(lit)
	if n == 0 {
		return []byte{0x00} // raw, size 0
	}
	allSame := true
	for _, b := range lit[1:] {
		if b != lit[0] {
			allSame = false
			break
		}
	}
	typ := byte(0) // raw
	if allSame && n > 1 {
		typ = 1 // RLE: one stored byte regenerates n
		lit = lit[:1]
	}
	switch {
	case n <= 31:
		return append([]byte{typ | byte(n)<<3}, lit...)
	case n <= 4095:
		return append([]byte{typ | 1<<2 | byte(n&0xF)<<4, byte(n >> 4)}, lit...)
	default:
		b0 := typ | 3<<2 | byte(n&0xF)<<4
		return append([]byte{b0, byte(n >> 4), byte(n >> 12)}, lit...)
	}
}

// sequencesSection encodes the count, the all-predefined modes byte,
// and the interleaved backward bitstream.  The literals-only tail
// sequence (offset 0) contributes its LL code and extras but, as the
// first the decoder reads, only rides the init state.
func sequencesSection(seqs []seq) []byte {
	// the trailing-literals run is not a sequence; the decoder appends
	// the leftovers on its own, so it never enters the count
	if len(seqs) > 0 && seqs[len(seqs)-1].offset == 0 {
		seqs = seqs[:len(seqs)-1]
	}
	n := len(seqs)
	if n == 0 {
		return []byte{0x00}
	}
	var hdr []byte
	switch {
	case n < 128:
		hdr = []byte{byte(n)}
	case n < 0x7F00:
		hdr = []byte{byte(128 + n>>8), byte(n)}
	default:
		v := n - 0x7F00
		hdr = []byte{255, byte(v), byte(v >> 8)}
	}
	hdr = append(hdr, 0x00) // modes: LL, OF, ML predefined

	code := func(s seq) (llc, mlc, ofc int, llx, mlx, ofx uint32, llb, mlb, ofb uint8) {
		llc = llCodeFor(s.litLen)
		llx = uint32(s.litLen - int(llBase[llc]))
		llb = llBits[llc]
		if s.offset == 0 {
			return
		}
		ov := uint32(s.offset + 3)
		ofc = highbit32(ov)
		ofx = ov - 1<<uint(ofc)
		ofb = uint8(ofc)
		mlc = mlCodeFor(s.matchLen)
		mlx = uint32(s.matchLen - int(mlBase[mlc]))
		mlb = mlBits[mlc]
		return
	}

	w := &bitWriter{}
	llc, mlc, ofc, llx, mlx, ofx, llb, mlb, ofb := code(seqs[n-1])
	var llSt, mlSt, ofSt cState
	llSt.init2(llCTab, llc)
	mlSt.init2(mlCTab, mlc)
	ofSt.init2(ofCTab, ofc)
	w.add(llx, uint(llb))
	w.add(mlx, uint(mlb))
	w.add(ofx, uint(ofb))
	for i := n - 2; i >= 0; i-- {
		llc, mlc, ofc, llx, mlx, ofx, llb, mlb, ofb = code(seqs[i])
		ofSt.encode(w, ofc)
		mlSt.encode(w, mlc)
		llSt.encode(w, llc)
		w.add(llx, uint(llb))
		w.add(mlx, uint(mlb))
		w.add(ofx, uint(ofb))
	}
	mlSt.flush(w)
	ofSt.flush(w)
	llSt.flush(w)
	w.close()
	return append(hdr, w.out...)
}

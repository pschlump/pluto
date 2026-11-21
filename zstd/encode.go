/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zstd

import "encoding/binary"

// The compressor: an optimal parse (shortest-path DP, see
// optimal.go) over a bounded hash chain, repeat-offset value coding,
// Huffman literals, and custom-or-predefined FSE sequence tables.
// Each 128 KiB block is matched against the whole frame so far (up
// to the 8 MiB window); blocks that do not shrink are stored raw,
// the reference writer's rule.

// seq is one LZ77 command: litLen literals, then a match of matchLen
// bytes at offset.  offset 0 marks the run of trailing literals.
type seq struct {
	litLen   int
	offset   int // the match distance
	matchLen int
	offWire  int // explicit wire offset value (1..3 repeat codes); 0 derives offset+3
}

// matchFinder finds matches over one frame: a hash chain (head plus
// prev links, the lz4 structure) walked a bounded number of
// candidates, so repetitive-but-not-identical data still finds long
// matches a single-slot table would miss.
type matchFinder struct {
	head             []int32 // hash -> most recent position with that hash
	prev             []int32 // position -> previous position with the same hash
	rep0, rep1, rep2 int     // the frame's repeat offsets (seeded 1, 4, 8)
}

const chainMask = (1 << 18) - 1
const chainDepth = 8

func newMatchFinder() *matchFinder {
	f := &matchFinder{head: make([]int32, 1<<17), prev: make([]int32, chainMask+1), rep0: 1, rep1: 4, rep2: 8}
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
func compressBlockBody(f *matchFinder, src []byte, from, to, window int, sc *seqCodec) []byte {
	seqs := f.optimalSeqs(src, from, to, window)
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
	body := append(litSec, sc.sequencesSection(seqs)...)
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

// sequencesSection encodes the count, the per-family table modes and
// descriptions (predefined, RLE, fresh FSE, or repeat — chosen by an
// estimated bit cost against the block's actual statistics), and the
// interleaved backward bitstream.  The literals-only tail sequence
// (offset 0) contributes its LL code and extras but, as the first
// the decoder reads, only rides the init state.
func (sc *seqCodec) sequencesSection(seqs []seq) []byte {
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

	code := func(s seq) (llc, mlc, ofc int, llx, mlx, ofx uint32, llb, mlb, ofb uint8) {
		llc = llCodeFor(s.litLen)
		llx = uint32(s.litLen - int(llBase[llc]))
		llb = llBits[llc]
		if s.offset == 0 {
			return
		}
		ov := uint32(s.offset + 3)
		if s.offWire != 0 {
			ov = uint32(s.offWire)
		}
		ofc = highbit32(ov)
		ofx = ov - 1<<uint(ofc)
		ofb = uint8(ofc)
		mlc = mlCodeFor(s.matchLen)
		mlx = uint32(s.matchLen - int(mlBase[mlc]))
		mlb = mlBits[mlc]
		return
	}

	// the block's actual code statistics drive the table choice
	var llCounts [llMaxSym + 1]uint32
	var ofCounts [ofMaxSym + 1]uint32
	var mlCounts [mlMaxSym + 1]uint32
	for _, s := range seqs {
		llc, mlc, ofc, _, _, _, _, _, _ := code(s)
		llCounts[llc]++
		ofCounts[ofc]++
		mlCounts[mlc]++
	}
	llCh := chooseFam(llCounts[:], llMaxSym, 9, llNorm, 9, llCTab, &sc.ll)
	ofCh := chooseFam(ofCounts[:], ofMaxSym, 8, ofNorm, 8, ofCTab, &sc.of)
	mlCh := chooseFam(mlCounts[:], mlMaxSym, 9, mlNorm, 9, mlCTab, &sc.ml)
	sc.ll.apply(llCh)
	sc.of.apply(ofCh)
	sc.ml.apply(mlCh)

	modes := byte(llCh.mode<<6 | ofCh.mode<<4 | mlCh.mode<<2)
	hdr = append(hdr, modes)
	if llCh.mode == 1 {
		hdr = append(hdr, byte(llCh.rleSym))
	} else {
		hdr = append(hdr, llCh.desc...)
	}
	if ofCh.mode == 1 {
		hdr = append(hdr, byte(ofCh.rleSym))
	} else {
		hdr = append(hdr, ofCh.desc...)
	}
	if mlCh.mode == 1 {
		hdr = append(hdr, byte(mlCh.rleSym))
	} else {
		hdr = append(hdr, mlCh.desc...)
	}

	w := &bitWriter{}
	llc, mlc, ofc, llx, mlx, ofx, llb, mlb, ofb := code(seqs[n-1])
	var llSt, mlSt, ofSt cState
	llSt.init2(llCh.tab, llc)
	mlSt.init2(mlCh.tab, mlc)
	ofSt.init2(ofCh.tab, ofc)
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

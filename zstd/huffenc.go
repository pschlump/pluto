/*
Copyright (C) Philip Schlump, 2012-2026.

MIT Licensed.
*/

package zstd

import "container/heap"

// The Huffman literals encoder: frequencies -> limited code lengths
// -> weights (the RFC's convention, last present symbol's weight
// implied by the completion) -> a direct 4-bit tree description ->
// one backward Huffman stream.  buildHuff on the decode side inverts
// it; the pin that matters is the canonical assignment — symbols
// sorted by ascending weight (longest codes first), codes counting up
// through the maxBits-width space (RFC 8878 Table 25).

// huffNode is a Huffman tree node during length computation.
type huffNode struct {
	weight uint64
	symbol int // -1 internal
	a, b   *huffNode
}

type nodeHeap []*huffNode

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].weight < h[j].weight }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x interface{}) { *h = append(*h, x.(*huffNode)) }
func (h *nodeHeap) Pop() interface{} {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// huffLengths computes code lengths for the frequencies, limited to
// maxLen.  Over-long trees are fixed by frequency scaling — halve
// every frequency and rebuild until the tree fits (a uniform tree on
// ≤256 symbols is at most 8 deep, so the loop terminates) — which
// keeps every intermediate a genuine Huffman tree: the Kraft sum is
// exact by construction, the property the wire's implied-last-weight
// completion requires.
func huffLengths(freq []uint64, maxLen int) []uint8 {
	scaled := make([]uint64, len(freq))
	var syms []int
	for s, f := range freq {
		if f > 0 {
			syms = append(syms, s)
			scaled[s] = f
		}
	}
	if len(syms) < 2 {
		return nil
	}
	for {
		lengths := huffLengthsOnce(scaled, syms)
		fits := true
		for _, s := range syms {
			if lengths[s] > uint8(maxLen) {
				fits = false
				break
			}
		}
		if fits {
			return lengths
		}
		for _, s := range syms {
			scaled[s] = scaled[s]/2 + 1
		}
	}
}

func huffLengthsOnce(freq []uint64, syms []int) []uint8 {
	// heap.Push/heap.Pop (the package functions), never the raw
	// methods: a method Push appends without the sift-up, and the
	// pops then merge the wrong nodes — a valid but arbitrary tree
	// (or a disconnected one).
	h := &nodeHeap{}
	for _, s := range syms {
		heap.Push(h, &huffNode{weight: freq[s], symbol: s})
	}
	for h.Len() > 1 {
		a := heap.Pop(h).(*huffNode)
		b := heap.Pop(h).(*huffNode)
		// symbol -1 marks internal: walk stops at symbol >= 0, and
		// the zero value of the field is 0 — a real symbol
		heap.Push(h, &huffNode{weight: a.weight + b.weight, symbol: -1, a: a, b: b})
	}
	lengths := make([]uint8, len(freq))
	var walk func(n *huffNode, d int)
	walk = func(n *huffNode, d int) {
		if n.symbol >= 0 {
			if d == 0 {
				d = 1
			}
			lengths[n.symbol] = uint8(d)
			return
		}
		walk(n.a, d+1)
		walk(n.b, d+1)
	}
	walk((*h)[0], 0)
	return lengths
}

// huffEnc carries the canonical codes.
type huffEnc struct {
	maxBits uint8
	codes   []uint16
	bits    []uint8
}

// buildHuffEnc assigns canonical codes: symbols by ascending length
// (natural order within), code = val >> (maxBits-nb) counting up in
// the maxBits-width space — exactly what the decoder's LUT fill does.
func buildHuffEnc(lengths []uint8) *huffEnc {
	maxBits := uint8(0)
	present := 0
	for _, l := range lengths {
		if l > maxBits {
			maxBits = l
		}
		if l > 0 {
			present++
		}
	}
	if present < 2 {
		return nil
	}
	e := &huffEnc{maxBits: maxBits, codes: make([]uint16, len(lengths)), bits: lengths}
	// longest codes first, natural symbol order within — the decoder
	// distributes canonical codes in exactly this order
	var order []int
	for l := int(maxBits); l >= 1; l-- {
		for s, ls := range lengths {
			if int(ls) == l {
				order = append(order, s)
			}
		}
	}
	var val uint32
	for _, s := range order {
		nb := uint(lengths[s])
		e.codes[s] = uint16(val >> uint(int(maxBits)-int(nb)))
		val += 1 << uint(int(maxBits)-int(nb))
	}
	return e
}

// huffmanLiterals encodes the literals as one Huffman stream with a
// direct 4-bit tree description, or returns nil when that would not
// beat the raw form (too few literals, an alphabet that does not fit
// the direct form, or output ≥ input).
func huffmanLiterals(lit []byte) []byte {
	if len(lit) < 16 {
		return nil
	}
	var freq [256]uint64
	for _, b := range lit {
		freq[b]++
	}
	lengths := huffLengths(freq[:], maxHuffLog)
	if lengths == nil {
		return nil
	}
	last := -1
	for s, l := range lengths {
		if l > 0 {
			last = s
		}
	}
	if last <= 0 {
		return nil // single-symbol alphabet: RLE handles it
	}
	// weights: w = maxBits+1-nbBits; the direct form lists every
	// symbol index below the last present one, zeros included
	maxBits := uint8(0)
	for _, l := range lengths {
		if l > maxBits {
			maxBits = l
		}
	}
	tree := make([]byte, last)
	for s := 0; s < last; s++ {
		if lengths[s] > 0 {
			w := maxBits + 1 - lengths[s]
			if w > 15 {
				return nil // does not fit the 4-bit direct form
			}
			tree[s] = w
		}
	}
	// the weight list covers 0..last-1 (the last symbol's weight is
	// implied by the Kraft completion)
	weights := make([]byte, last)
	for s := 0; s < last; s++ {
		if lengths[s] > 0 {
			weights[s] = maxBits + 1 - lengths[s]
		}
	}
	var treeHdr []byte
	if last <= 128 {
		for i := 0; i < last; i += 2 {
			b := weights[i] << 4
			if i+1 < last {
				b |= weights[i+1]
			}
			treeHdr = append(treeHdr, b)
		}
		hdr := append([]byte{byte(127 + last)}, treeHdr...)
		treeHdr = hdr
	} else {
		var ok bool
		treeHdr, ok = fseWeights(weights)
		if !ok {
			return nil // the direct 4-bit form caps at 128 weights
		}
	}
	e := buildHuffEnc(lengths)
	if e == nil {
		return nil
	}

	encodeStream := func(b []byte) []byte {
		w := &bitWriter{}
		for i := len(b) - 1; i >= 0; i-- {
			s := b[i]
			w.add(uint32(e.codes[s]), uint(e.bits[s]))
		}
		w.close()
		return w.out
	}

	n := len(lit)
	// one stream while both sizes fit the header's fields; past the
	// 14-bit form the block splits into four streams with a 6-byte
	// jump table, the shape the format requires
	type form struct {
		hdrInt uint64
		hdrLen int
	}
	var f form
	var payload []byte
	single := encodeStream(lit)
	switch {
	case n <= 0x3FF && len(treeHdr)+len(single) <= 0x3FF:
		// the only single-stream form: 3-byte header, 10-bit sizes
		f = form{2 | 0<<2 | uint64(n)<<4 | uint64(len(treeHdr)+len(single))<<14, 3}
		payload = single
	default:
		seg := (n + 3) >> 2
		var jump []byte
		var streams []byte
		for i := 0; i < 4; i++ {
			hi := n
			if i < 3 {
				hi = seg * (i + 1)
			}
			st := encodeStream(lit[seg*i : hi])
			if i < 3 {
				jump = append(jump, byte(len(st)), byte(len(st)>>8))
			}
			streams = append(streams, st...)
		}
		comp := len(treeHdr) + 6 + len(streams)
		switch {
		case n <= 0x3FF && comp <= 0x3FF:
			f = form{2 | 1<<2 | uint64(n)<<4 | uint64(comp)<<14, 3}
		case n <= 0x3FFF && comp <= 0x3FFF:
			f = form{2 | 2<<2 | uint64(n)<<4 | uint64(comp)<<18, 4}
		default:
			f = form{2 | 3<<2 | uint64(n)<<4 | uint64(comp)<<22, 5}
		}
		payload = append(jump, streams...)
	}
	hdr := make([]byte, f.hdrLen)
	for i := 0; i < f.hdrLen; i++ {
		hdr[i] = byte(f.hdrInt >> uint(8*i))
	}

	body := append(append(append([]byte{}, hdr...), treeHdr...), payload...)
	if len(body) >= n {
		return nil
	}
	return body
}

// fseWeights writes the weight list in the FSE-coded form: a table
// description over weight VALUES, then a backward stream with two
// interleaved states.  The reader consumes even indices from the
// first state and odd from the second, strictly alternating, so the
// writer builds each chain's bit emissions backward (the natural FSE
// direction), re-derives the reader's alternating order, and emits
// the whole sequence reversed.  ok is false when the weights do not
// fit this form.
func fseWeights(weights []byte) ([]byte, bool) {
	k := len(weights)
	if k < 2 {
		return nil, false
	}
	var counts [16]uint32
	maxSym := 0
	for _, w := range weights {
		if int(w) > maxSym {
			maxSym = int(w)
		}
		counts[w]++
	}
	norm, log := normalizeCounts(counts[:maxSym+1], maxSym, 6)
	if norm == nil {
		return nil, false
	}
	desc := writeNCount(norm, log)
	if len(desc) > 120 {
		return nil, false // cannot fit the 1-byte size field
	}
	tab := buildCTable(norm, log)

	// one chain's emissions in WRITE order (backward): the chain's
	// symbols in reader order first, then encode backward
	chainBits := func(syms []int) []ebits {
		var st cState
		st.init2(tab, syms[len(syms)-1])
		var w2 []ebits
		for i := len(syms) - 2; i >= 0; i-- {
			tt := tab.tt[syms[i]]
			nb := (st.v + tt.deltaNbBits) >> 16
			w2 = append(w2, ebits{st.v, uint(nb)})
			st.v = uint32(tab.stateTable[(st.v>>nb)+uint32(tt.deltaFindState)])
		}
		return append(w2, ebits{st.v, tab.log})
	}

	var s1syms, s2syms []int
	for i := 0; i < k; i += 2 {
		s1syms = append(s1syms, int(weights[i]))
	}
	for i := 1; i < k; i += 2 {
		s2syms = append(s2syms, int(weights[i]))
	}
	rev := func(v []ebits) []ebits {
		out := make([]ebits, len(v))
		for i, x := range v {
			out[len(v)-1-i] = x
		}
		return out
	}
	r1 := rev(chainBits(s1syms)) // reader order for the even chain
	r2 := rev(chainBits(s2syms))

	// the reader's global order: emissions 0..k-1 alternate states
	g := make([]ebits, 0, k)
	for j := 0; j < k; j++ {
		if j%2 == 0 {
			g = append(g, r1[j/2])
		} else {
			g = append(g, r2[j/2])
		}
	}
	w := &bitWriter{}
	for i := len(g) - 1; i >= 0; i-- {
		w.add(g[i].v, g[i].n)
	}
	w.close()

	if len(desc)+len(w.out) > 127 {
		return nil, false // the size byte must stay under the direct form's range
	}
	out := append([]byte{byte(len(desc) + len(w.out))}, desc...)
	return append(out, w.out...), true
}

type ebits struct {
	v uint32
	n uint
}

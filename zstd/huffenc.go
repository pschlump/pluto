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
	h := &nodeHeap{}
	for _, s := range syms {
		h.Push(&huffNode{weight: freq[s], symbol: s})
	}
	for h.Len() > 1 {
		a := heap.Pop(h).(*huffNode)
		b := heap.Pop(h).(*huffNode)
		h.Push(&huffNode{weight: a.weight + b.weight, a: a, b: b})
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
	var order []int
	for l := 1; l <= int(maxBits); l++ {
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
	e := buildHuffEnc(lengths)
	if e == nil {
		return nil
	}

	w := &bitWriter{}
	for i := len(lit) - 1; i >= 0; i-- {
		s := lit[i]
		w.add(uint32(e.codes[s]), uint(e.bits[s]))
	}
	w.close()

	var treeBytes []byte
	for i := 0; i < len(tree); i += 2 {
		b := tree[i] << 4
		if i+1 < len(tree) {
			b |= tree[i+1]
		}
		treeBytes = append(treeBytes, b)
	}
	treeHdr := append([]byte{byte(127 + len(tree))}, treeBytes...)

	n := len(lit)
	comp := len(treeHdr) + len(w.out)
	// 2-2-10 header (single stream, sf 0) or 2-2-14 (sf 2), built as
	// a little-endian integer exactly the way the decoder reads it
	var hdrInt uint64
	var hdrLen int
	switch {
	case n <= 0x3FF && comp <= 0x3FF:
		hdrInt = 2 | 0<<2 | uint64(n)<<4 | uint64(comp)<<14
		hdrLen = 3
	default:
		hdrInt = 2 | 2<<2 | uint64(n)<<4 | uint64(comp)<<18
		hdrLen = 4
	}
	hdr := make([]byte, hdrLen)
	for i := 0; i < hdrLen; i++ {
		hdr[i] = byte(hdrInt >> uint(8*i))
	}

	body := append(append(append([]byte{}, hdr...), treeHdr...), w.out...)
	if len(body) >= n {
		return nil
	}
	return body
}

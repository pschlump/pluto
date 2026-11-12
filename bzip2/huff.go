/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2

import "github.com/pschlump/pluto/heap"

// Canonical Huffman machinery, shared by the encoder and the decoder.
// bzip2's prefix codes are canonical in the DEFLATE sense — symbols
// sorted by (length, symbol) get increasing code values — but they
// are emitted most-significant-bit first (bzip2's whole bitstream is
// MSB-first), so no bit reversal happens anywhere.
//
// Differences from pluto/deflate's machinery: code lengths run 1..20
// (not 1..15) and every symbol of the block's alphabet carries a code
// (the length tables cover the whole alphabet, so zero-frequency
// entries are given weight 1 instead of length 0 — the alphabet is
// always at least three symbols: RUNA, RUNB and EOB).

// buildLengths computes canonical code lengths for freqs, no length
// exceeding maxLen and no length below 1.  The tree is built with
// pluto/heap (ties break by node index so the result is
// deterministic); if the optimal tree exceeds maxLen — possible only
// for extremely skewed frequencies — every frequency is halved
// (rounding up) and the tree is rebuilt, which converges on a
// balanced tree.
func buildLengths(freqs []int, maxLen int) []uint8 {
	n := len(freqs)
	lengths := make([]uint8, n)
	f := make([]int64, n)
	for i, v := range freqs {
		f[i] = int64(v)
		if v == 0 {
			f[i] = 1 // unused symbols still need a code
		}
	}
	for {
		hp := heap.NewHeapFunc(func(a, b huffItem) int {
			if a.weight != b.weight {
				if a.weight < b.weight {
					return -1
				}
				return 1
			}
			if a.node != b.node {
				if a.node < b.node {
					return -1
				}
				return 1
			}
			return 0
		})
		for s, w := range f {
			hp.Push(huffItem{w, int32(s)})
		}

		// Internal nodes are appended after the n leaf slots; each
		// merge pops the two lightest items and pushes their parent.
		nodes := make([]huffNode, 0, n-1)
		for hp.Len() > 1 {
			a, _ := hp.Pop()
			b, _ := hp.Pop()
			nodes = append(nodes, huffNode{a.node, b.node})
			hp.Push(huffItem{a.weight + b.weight, int32(n + len(nodes) - 1)})
		}
		root, _ := hp.Pop()

		// Walk the tree, recording leaf depths.
		max := 0
		stack := []huffWalk{{root.node, 0}}
		for len(stack) > 0 {
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if x.node < int32(n) {
				lengths[x.node] = x.depth
				if int(x.depth) > max {
					max = int(x.depth)
				}
				continue
			}
			nd := nodes[x.node-int32(n)]
			stack = append(stack, huffWalk{nd.l, x.depth + 1}, huffWalk{nd.r, x.depth + 1})
		}
		if max <= maxLen {
			return lengths
		}
		for i, w := range f {
			f[i] = (w + 1) / 2
		}
	}
}

type huffItem struct {
	weight int64
	node   int32 // < n: the leaf (symbol); >= n: index into huffNode slice
}

type huffNode struct {
	l, r int32
}

type huffWalk struct {
	node  int32
	depth uint8
}

// canonicalCodes assigns canonical codes for the given lengths,
// MSB-first ready for the writer.  All lengths are >= 1 here.
func canonicalCodes(lengths []uint8) []uint32 {
	var blCount [21]int
	for _, l := range lengths {
		blCount[l]++
	}
	var nextCode [21]uint32
	c := uint32(0)
	for l := 1; l <= 20; l++ {
		c = (c + uint32(blCount[l-1])) << 1
		nextCode[l] = c
	}
	codes := make([]uint32, len(lengths))
	for s, l := range lengths {
		codes[s] = nextCode[l]
		nextCode[l]++
	}
	return codes
}

// decTree is the decoder side of a canonical code: the symbols sorted
// by (length, symbol value), plus the count of codes at each length —
// the same per-length walk zlib's puff uses, over lengths 1..20.
type decTree struct {
	counts [21]int
	syms   []uint16
}

// buildDecTree validates lengths and builds the decoding table.  ok
// is false for a length of 0 (every bzip2 symbol carries a code), a
// length over 20, or an over- or under-subscribed code.
func buildDecTree(lengths []uint8) (*decTree, bool) {
	t := &decTree{syms: make([]uint16, 0, len(lengths))}
	for _, l := range lengths {
		if l < 1 || l > 20 {
			return nil, false
		}
		t.counts[l]++
	}
	for l := 1; l <= 20; l++ {
		for s, ln := range lengths {
			if ln == uint8(l) {
				t.syms = append(t.syms, uint16(s))
			}
		}
	}
	left := 1
	for l := 1; l <= 20; l++ {
		left <<= 1
		left -= t.counts[l]
		if left < 0 {
			return nil, false // over-subscribed
		}
	}
	if left > 0 {
		return nil, false // incomplete
	}
	return t, true
}

// decodeSym reads one Huffman-coded symbol.  ok is false on end of
// input or on a bit pattern that matches no code.  The bit extraction
// is written out (the body of readBits for n=1) instead of calling
// readBit: this is the per-symbol hot path, and readBits must call
// the non-inlinable fill, which would keep it from inlining here —
// the same lesson as deflate's decTree.decode.
func (t *decTree) decodeSym(r *msbReader) (uint16, bool) {
	code, first, index := 0, 0, 0
	for l := 1; l <= 20; l++ {
		if r.n == 0 {
			if r.pos >= len(r.buf) {
				if !r.fill() {
					r.readFailed = true
					return 0, false
				}
			}
			r.acc = r.acc<<8 | uint64(r.buf[r.pos])
			r.pos++
			r.n = 8
		}
		b := (r.acc >> (r.n - 1)) & 1 // acc keeps stale bits above the unread window: mask
		r.n--
		code = code<<1 | int(b)
		count := t.counts[l]
		if code-first < count {
			return t.syms[index+code-first], true
		}
		index += count
		first = (first + count) << 1
	}
	return 0, false
}

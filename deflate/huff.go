/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate

import "github.com/pschlump/pluto/heap"

// Canonical Huffman machinery, shared by the encoder and the decoder
// (RFC 1951 §3.2.2): codes are assigned in order of increasing length
// and, within a length, in order of increasing symbol value.

// huffCode is one canonical Huffman code in *write order* — the
// canonical code bit-reversed, so that writeBits emits the code's most
// significant bit first as §3.1.1 requires.
type huffCode struct {
	bits uint16
	len  uint8
}

// buildLengths computes canonical Huffman code lengths for the given
// symbol frequencies, no length exceeding maxLen.  Symbols with zero
// frequency get length 0.  Exactly one used symbol gets length 1 (a
// single code cannot have length 0 — that would mean "no code").
//
// The tree is built with pluto/heap (intra-pluto composition; ties
// break by node index so the result is deterministic).  If the optimal
// tree exceeds maxLen — possible only for extremely skewed
// frequencies — every non-zero frequency is halved (rounding up) and
// the tree is rebuilt; that flattens the distribution and converges
// (in the limit of equal frequencies the tree is perfectly balanced).
//
// freqs is not modified.  Complexity is O(n log n) per pass with a
// small constant number of passes.
func buildLengths(freqs []int, maxLen int) []uint8 {
	n := len(freqs)
	lengths := make([]uint8, n)
	f := make([]int64, n)
	for i, v := range freqs {
		f[i] = int64(v)
	}
	for {
		used, last := 0, -1
		for s, w := range f {
			if w > 0 {
				used++
				last = s
			}
		}
		if used <= 1 {
			if used == 1 {
				lengths[last] = 1
			}
			return lengths
		}

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
			if w > 0 {
				hp.Push(huffItem{w, int32(s)})
			}
		}

		// Internal nodes are appended after the n leaf slots; each
		// merge pops the two lightest items and pushes their parent.
		nodes := make([]huffNode, 0, used-1)
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
			if w > 0 {
				f[i] = (w + 1) / 2
			}
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

// canonicalCodes assigns canonical codes for the given lengths and
// returns them bit-reversed, ready for the LSB-first writer.  Symbols
// with length 0 are left as the zero huffCode and must not be emitted.
func canonicalCodes(lengths []uint8) []huffCode {
	var blCount [17]int
	for _, l := range lengths {
		blCount[l]++
	}
	blCount[0] = 0
	var nextCode [17]uint16
	c := uint16(0)
	for l := 1; l <= 16; l++ {
		c = (c + uint16(blCount[l-1])) << 1
		nextCode[l] = c
	}
	codes := make([]huffCode, len(lengths))
	for s, l := range lengths {
		if l == 0 {
			continue
		}
		codes[s] = huffCode{reverseBits(nextCode[l], l), l}
		nextCode[l]++
	}
	return codes
}

// reverseBits returns the low n bits of v with their order reversed,
// so that writing the result LSB-first emits v's bit n-1 first.
func reverseBits(v uint16, n uint8) uint16 {
	var r uint16
	for i := uint8(0); i < n; i++ {
		r = r<<1 | (v>>i)&1
	}
	return r
}

// decTree is the decoder side of a canonical code: the symbols sorted
// by (length, symbol value), plus the count of codes at each length.
// Decoding walks code lengths from 1 up, exactly the reference
// algorithm of RFC 1951 §3.2.2 as expressed in zlib's puff.
type decTree struct {
	counts [16]int
	syms   []uint16
}

// buildFixedTree builds the decoding table for an RFC-defined fixed
// tree without validation — its lengths are constants of the format.
// The fixed distance tree is legitimately incomplete (§3.2.6: codes
// 30-31 are part of the tree but "will never actually occur in the
// compressed data"), which the stream-validation rule below would
// wrongly reject.
func buildFixedTree(lengths []uint8) *decTree {
	t := &decTree{syms: make([]uint16, 0, len(lengths))}
	for _, l := range lengths {
		if l > 0 {
			t.counts[l]++
		}
	}
	for l := 1; l <= 15; l++ {
		for s, ln := range lengths {
			if ln == uint8(l) {
				t.syms = append(t.syms, uint16(s))
			}
		}
	}
	return t
}

// buildDecTree validates the lengths and builds the decoding table.
// It returns ok=false for an over-subscribed code (more codes than the
// tree can hold).  An incomplete code — codes that do not fill the
// tree — is rejected as well, except in the one case zlib permits: a
// tree of a single 1-bit code (allowSingle), which the encoder writes
// for the distance alphabet when zero or one distances are used.
func buildDecTree(lengths []uint8, allowSingle bool) (*decTree, bool) {
	t := &decTree{syms: make([]uint16, 0, len(lengths))}
	single := true
	for _, l := range lengths {
		if l == 0 {
			continue
		}
		if l > 15 {
			return nil, false
		}
		t.counts[l]++
		if l != 1 {
			single = false
		}
	}
	// Symbols in canonical order: grouped by code length, and within a
	// length by symbol value — the order canonicalCodes assigns codes.
	for l := 1; l <= 15; l++ {
		for s, ln := range lengths {
			if ln == uint8(l) {
				t.syms = append(t.syms, uint16(s))
			}
		}
	}
	left := 1
	for l := 1; l <= 15; l++ {
		left <<= 1
		left -= t.counts[l]
		if left < 0 {
			return nil, false // over-subscribed
		}
	}
	if left > 0 && (!allowSingle || !single) {
		return nil, false // incomplete
	}
	return t, true
}

// decode reads one Huffman-coded symbol.  ok is false on end of input
// or on a bit pattern that matches no code (possible only in the
// incomplete-single-code case, or a corrupt stream).  It works in
// both bitReader modes — whole-buffer and streaming.  The bit
// extraction is written out (the body of readBit) instead of calling
// it: this is the per-symbol hot path, and a call from here to
// readBit — which must itself call the non-inlinable fill — would not
// be inlined.  decode is too large to inline into anyone, so the
// fill() call below is the one place it costs nothing.
func (t *decTree) decode(r *bitReader) (uint16, bool) {
	code, first, index := 0, 0, 0
	for l := 1; l <= 15; l++ {
		if r.n == 0 {
			if r.pos >= len(r.buf) {
				if !r.fill() {
					r.readFailed = true
					return 0, false
				}
			}
			r.acc = uint32(r.buf[r.pos])
			r.pos++
			r.n = 8
		}
		b := r.acc & 1
		r.acc >>= 1
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

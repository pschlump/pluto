/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package huffman implements Huffman compression and expansion of byte
// slices — Sedgwick & Wayne, Algorithms, 4th ed. §5.5 (Huffman.java):
// the optimal prefix-free code for a known symbol distribution.
//
// The alphabet is the 256 byte values.  Compress tabulates the input's
// symbol frequencies, builds the Huffman trie by pairwise merge (on
// pluto/heap — an intra-pluto composition), and emits a self-describing
// blob:
//
//	[trie, preorder] [symbol count, 64 bits] [codeword bits...]
//
// A trie node serializes as a 1 bit followed by the 8-bit symbol for a
// leaf, or a bare 0 bit for an internal node; a symbol's codeword is
// its path from the root (0 = left, 1 = right).  Bits pack MSB-first —
// the algs4 BinaryStdOut / BinaryStdIn convention shared with lzw and
// run_length (see bits.go).
//
//	compressed := huffman.Compress(data) // []byte, self-describing blob
//	back := huffman.Expand(compressed)   // == data, or nil if malformed
//
// The merge breaks frequency ties by insertion order — leaves in symbol
// order, merged subtrees in creation order — so the trie, the code and
// therefore the blob are deterministic: recompressing an expansion
// reproduces the blob byte for byte.
//
// The package NEVER panics.  Compress accepts any input; the empty (or
// nil) input yields the empty result.  Expand returns nil for malformed
// input — a blob truncated anywhere (in the trie, the count, or
// mid-codeword), whose trie exceeds the 511 nodes a full binary tree
// over 256 symbols can hold, or whose count cannot be honest (larger
// than any Go slice, or larger than the bits remaining under an
// internal root).  Trailing bytes after the last codeword bit are
// ignored (the lzw behavior).  One documented hazard: a single-symbol
// message costs zero bits per symbol — an honest 1 MiB of one byte
// compresses to 10 bytes — so a crafted single-leaf blob can claim a
// huge count and Expand will allocate accordingly (the standard
// decompression-bomb property of the format; algs4 has it too).
//
// Both functions are stateless — no package-level state — so they are
// safe for concurrent use, and there is no _ts twin.
package huffman

import (
	"cmp"

	"github.com/pschlump/pluto/heap"
)

// alphabet is the number of symbols: the input symbols are the bytes
// 0..255 (algs4's R = 256 for the 8-bit alphabet).
const alphabet = 256

// huffNode is one node of the Huffman trie.  A leaf (ch set, children
// nil) carries one byte symbol; an internal node carries the summed
// frequency of its subtree.
type huffNode struct {
	ch          byte
	freq        int64
	left, right *huffNode
}

// isLeaf reports whether n carries a symbol.  Every internal node of a
// Huffman trie has exactly two children, so testing one suffices.
func (n *huffNode) isLeaf() bool { return n.left == nil && n.right == nil }

// huffItem is what the merge heap holds: a subtree, the frequency it is
// ordered by, and a monotone sequence number that breaks ties so the
// trie — and therefore the compressed bytes — are deterministic.
type huffItem struct {
	freq int64
	seq  int
	node *huffNode
}

// Compress returns the Huffman-compressed form of data: the serialized
// trie, the 64-bit symbol count, and one codeword per input symbol, in
// order.  The empty (or nil) input yields the empty result; any other
// input yields a blob Expand inverts exactly.
//
// The frequencies are tabulated in one pass, the trie is built by
// pairwise merge on pluto/heap, and the codewords are the root-to-leaf
// paths of that trie.
// Complexity is O(n) for fixed alphabet size: the frequency pass and
// the codeword emission are O(n), and the trie and code tables are
// O(R log R) work over R ≤ 256 distinct symbols.
func Compress(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	// Tabulate the frequency counts (algs4 compress step 1).
	var freq [alphabet]int64
	for _, b := range data {
		freq[b]++
	}

	// Build the trie and the code it implies (steps 2–3).
	root := buildTrie(&freq)
	var codes [alphabet]string
	buildCode(&codes, root, "")

	// Write the trie, the symbol count, and the codewords (steps 4–5).
	var w bitWriter
	writeTrie(&w, root)
	w.writeBits(len(data), 64)
	for _, b := range data {
		w.writeCode(codes[b])
	}
	return w.bytes()
}

// buildTrie merges the frequency-ordered leaves pairwise until one tree
// remains: pop the two smallest, hang them under a new internal node
// (the first pop is the LEFT child, the algs4 convention), push the
// parent back.  A single present symbol never enters the loop, so the
// root is its leaf and its codeword is the empty string — Expand then
// reads zero bits per symbol (the algs4 behavior).
//
// Ties break by seq: the initial leaves in symbol order, merged
// subtrees in creation order.  Any tie-break yields an optimal code;
// this one merely makes the output deterministic.
// Complexity is O(R log R) over the R distinct symbols.
func buildTrie(freq *[alphabet]int64) *huffNode {
	h := heap.NewHeapFunc(func(a, b huffItem) int {
		if c := cmp.Compare(a.freq, b.freq); c != 0 {
			return c
		}
		return cmp.Compare(a.seq, b.seq)
	})
	seq := 0
	for c := range alphabet {
		if freq[c] > 0 {
			h.Push(huffItem{freq: freq[c], seq: seq, node: &huffNode{ch: byte(c), freq: freq[c]}})
			seq++
		}
	}
	for h.Len() > 1 {
		left, _ := h.Pop()
		right, _ := h.Pop()
		parent := &huffNode{freq: left.freq + right.freq, left: left.node, right: right.node}
		h.Push(huffItem{freq: parent.freq, seq: seq, node: parent})
		seq++
	}
	root, _ := h.Pop()
	return root.node
}

// buildCode fills codes with each symbol's codeword — its path from
// the root, 0 for left and 1 for right.  Codewords are strings, not
// integers: pathological skews (Fibonacci-weighted frequencies) grow
// codes far past 64 bits — unattainable through honest input sizes
// here, but the storage must not cap them (algs4 uses String too).
// Complexity is O(R·L) for codewords of length L ≤ R−1.
func buildCode(codes *[alphabet]string, n *huffNode, code string) {
	if n.isLeaf() {
		codes[n.ch] = code
		return
	}
	buildCode(codes, n.left, code+"0")
	buildCode(codes, n.right, code+"1")
}

// writeTrie serializes the trie preorder, one node at a time: a leaf
// is a 1 bit followed by its 8-bit symbol, an internal node a bare 0
// bit.  expand's readTrie rebuilds the exact shape from this walk.
// Complexity is O(R).
func writeTrie(w *bitWriter, n *huffNode) {
	if n.isLeaf() {
		w.writeBit(1)
		w.writeBits(int(n.ch), 8)
		return
	}
	w.writeBit(0)
	writeTrie(w, n.left)
	writeTrie(w, n.right)
}

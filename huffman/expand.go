/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman

// maxTrieNodes caps the trie readTrie will build.  A Huffman trie over
// the 256-symbol alphabet is a full binary tree with at most 256 leaves
// and 255 internal nodes, so 511 nodes bound it exactly — and they bound
// what a malformed blob can demand: the recursion depth (≤ 255 chain)
// and the allocation before the parse fails.
const maxTrieNodes = 2*alphabet - 1

// readTrie is the inverse of writeTrie: a 1 bit introduces a leaf whose
// symbol is the following 8 bits, a 0 bit an internal node whose two
// children follow.  used counts the nodes built so far against
// maxTrieNodes; ok is false on truncation or when the cap trips (a blob
// no honest Compress can produce).
// Complexity is O(nodes built), bounded by maxTrieNodes.
func readTrie(r *bitReader, used *int) (*huffNode, bool) {
	*used++
	if *used > maxTrieNodes {
		return nil, false
	}
	kind, ok := r.readBits(1)
	if !ok {
		return nil, false
	}
	if kind == 1 {
		ch, ok := r.readBits(8)
		if !ok {
			return nil, false
		}
		return &huffNode{ch: byte(ch)}, true
	}
	left, ok := readTrie(r, used)
	if !ok {
		return nil, false
	}
	right, ok := readTrie(r, used)
	if !ok {
		return nil, false
	}
	return &huffNode{left: left, right: right}, true
}

// Expand returns the bytes encoded by Compress, or nil if blob is
// malformed.  Malformed means: truncated anywhere — the trie, the
// count, or mid-codeword — a trie larger than 511 nodes, a count no Go
// slice could hold, or (under an internal root) a count larger than the
// bits remaining.  Trailing bytes after the last codeword bit are
// ignored (the lzw behavior).  The empty blob expands to nil.
//
// The trie is rebuilt first, then the 64-bit symbol count, then that
// many codewords are decoded by walking the trie from the root bit by
// bit (0 = left, 1 = right) until a leaf.  A single-leaf trie makes the
// walk zero bits long, so such a message costs nothing per symbol —
// legal, and the same property that lets 1 MiB of one byte compress to
// 10 bytes also lets a crafted count inflate the output (the standard
// decompression-bomb exposure of the format: an internal-rooted claim
// is bounded by the blob's own bits, a single-leaf claim only by the
// count).
// Complexity is O(n) over the decoded bytes.
func Expand(blob []byte) []byte {
	if len(blob) == 0 {
		return nil
	}

	r := bitReader{data: blob}
	used := 0
	root, ok := readTrie(&r, &used)
	if !ok {
		return nil
	}

	// The 64-bit count, read as two 32-bit halves so the value is
	// correct on 32-bit platforms (readBits accumulates into int).  A
	// count with its top bit set cannot be the length of any Go slice —
	// malformed outright, which also stops the accidental 0xFFFF… blob.
	hi, ok := r.readBits(32)
	if !ok {
		return nil
	}
	if hi >= 1<<31 {
		return nil
	}
	lo, ok := r.readBits(32)
	if !ok {
		return nil
	}
	n := int64(hi)<<32 | int64(lo)

	// An internal-rooted message spends at least one bit per symbol, so
	// a count larger than the bits left cannot be honest.  (A single
	// leaf spends zero bits per symbol — nothing bounds that case but
	// the count itself.)
	if !root.isLeaf() && n > int64(len(r.data)*8-r.pos) {
		return nil
	}

	capHint := n
	if capHint > int64(len(blob)) {
		capHint = int64(len(blob))
	}
	out := make([]byte, 0, capHint)
	for i := int64(0); i < n; i++ {
		x := root
		for !x.isLeaf() {
			bit, ok := r.readBits(1)
			if !ok {
				return nil // truncated mid-codeword
			}
			if bit == 0 {
				x = x.left
			} else {
				x = x.right
			}
		}
		out = append(out, x.ch)
	}
	return out
}

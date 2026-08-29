/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// On-disk B+ tree node encoding and the structural primitives that
// mutate nodes in place inside a cached block image.  Every function
// here operates on a raw block ([]byte of BlockSize); the caller (the
// Tree methods in tree.go) holds the store lock, pins the blocks it
// references, and marks blocks dirty after mutation.
//
// Node layouts (all integers little-endian, ks = keySize):
//
//	Leaf:     type(1) nkeys(4) nextLeaf(8) | n × (key ks, value 8)
//	Internal: type(1) nkeys(4) | keys region maxInt×ks | children region (maxInt+1)×8
//
// The internal-node children live at a fixed offset (past the full keys
// region, not past the n used keys) so inserting a key does not move
// the child pointers.
//
// Separator keys in internal nodes are lower bounds for the subtree to
// their right.  After deletes they may be STALE — a separator that
// named a since-deleted key stays in place and stays a valid lower
// bound; only merges and borrows rewrite separators.  This is the
// standard B+ simplification: routing stays correct and no separator is
// ever repaired on a plain delete.
//
// Occupancy: a full leaf holds maxLeaf entries, a full internal node
// maxInt keys.  Splits divide a full node into ceil(max/2) and
// floor(max/2).  Deletes repair a node when it falls below
// ceil(max/2) (the trigger), borrowing from a sibling that holds more
// than floor(max/2) entries — that threshold, not the trigger, is what
// guarantees a merge of trigger-1 + floor + 1 ≤ max always fits in one
// block.

package b_tree_disk_ts

import (
	"bytes"
	"encoding/binary"
)

const (
	nodeLeaf     byte = 1
	nodeInternal byte = 2
)

const (
	leafHeaderSize     = 13 // type(1) + nkeys(4) + nextLeaf(8)
	internalHeaderSize = 5  // type(1) + nkeys(4)
)

// maxLeafEntries returns how many (key, value) entries fit in a leaf.
func maxLeafEntries(blockSize, keySize int) int {
	return (blockSize - leafHeaderSize) / (keySize + 8)
}

// maxInternalKeys returns how many separator keys fit in an internal
// node (which then has one more child pointer).
func maxInternalKeys(blockSize, keySize int) int {
	return (blockSize - internalHeaderSize - 8) / (keySize + 8)
}

func isLeaf(b []byte) bool { return b[0] == nodeLeaf }

// nodeCount returns the entry count of either node kind.
func nodeCount(b []byte) int {
	if isLeaf(b) {
		return leafCount(b)
	}
	return internalCount(b)
}

// nodeFull reports whether the node is at capacity.
func nodeFull(b []byte, maxLeaf, maxInt int) bool {
	if isLeaf(b) {
		return leafCount(b) >= maxLeaf
	}
	return internalCount(b) >= maxInt
}

// -------------------------------------------------------------------------------------------------------
// Leaf nodes.

func leafEntryOff(i, ks int) int { return leafHeaderSize + i*(ks+8) }

func leafInit(b []byte) {
	b[0] = nodeLeaf
	leafSetCount(b, 0)
	leafSetNext(b, 0)
}

func leafCount(b []byte) int { return int(binary.LittleEndian.Uint32(b[1:5])) }

func leafSetCount(b []byte, n int) { binary.LittleEndian.PutUint32(b[1:5], uint32(n)) }

func leafNext(b []byte) uint64 { return binary.LittleEndian.Uint64(b[5:13]) }

func leafSetNext(b []byte, no uint64) { binary.LittleEndian.PutUint64(b[5:13], no) }

// leafKey returns the key bytes of entry i; the slice aliases the block.
func leafKey(b []byte, i, ks int) []byte {
	off := leafEntryOff(i, ks)
	return b[off : off+ks]
}

func leafValue(b []byte, i, ks int) uint64 {
	return binary.LittleEndian.Uint64(b[leafEntryOff(i, ks)+ks:])
}

// leafFind returns the index of the first entry not less than key, and
// whether that entry equals key.
// Complexity is O(log₂ entries).
func leafFind(b []byte, key []byte, ks int) (int, bool) {
	n := leafCount(b)
	lo, hi := 0, n
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if bytes.Compare(leafKey(b, mid, ks), key) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, lo < n && bytes.Equal(leafKey(b, lo, ks), key)
}

// leafInsertAt inserts (key, value) at position i, shifting later
// entries right.  The caller guarantees the leaf has room.
func leafInsertAt(b []byte, i int, key []byte, value uint64, ks int) {
	n := leafCount(b)
	copy(b[leafEntryOff(i+1, ks):leafEntryOff(n+1, ks)], b[leafEntryOff(i, ks):leafEntryOff(n, ks)])
	off := leafEntryOff(i, ks)
	copy(b[off:off+ks], key)
	binary.LittleEndian.PutUint64(b[off+ks:], value)
	leafSetCount(b, n+1)
}

// leafRemoveAt removes the entry at position i.
func leafRemoveAt(b []byte, i, ks int) {
	n := leafCount(b)
	copy(b[leafEntryOff(i, ks):leafEntryOff(n-1, ks)], b[leafEntryOff(i+1, ks):leafEntryOff(n, ks)])
	leafSetCount(b, n-1)
}

// leafSplit divides a FULL leaf: the receiver keeps the first
// ceil(maxLeaf/2) entries, nb (block nbNo, freshly allocated) takes the
// rest, and the nextLeaf chain is relinked receiver → nb → old next.
// It returns a copy of the separator — the first key of nb — which the
// parent must adopt; the key STAYS in nb (leaf separators are copies).
func leafSplit(b, nb []byte, nbNo uint64, ks, maxLeaf int) []byte {
	mid := (maxLeaf + 1) / 2
	rn := maxLeaf - mid
	leafInit(nb)
	copy(nb[leafEntryOff(0, ks):leafEntryOff(rn, ks)], b[leafEntryOff(mid, ks):leafEntryOff(maxLeaf, ks)])
	leafSetCount(nb, rn)
	leafSetCount(b, mid)
	leafSetNext(nb, leafNext(b))
	leafSetNext(b, nbNo)
	return append([]byte(nil), leafKey(nb, 0, ks)...)
}

// leafBorrowFromLeft moves the last entry of left to the front of child
// and refreshes the parent separator at pidx-1.
func leafBorrowFromLeft(parent []byte, pidx int, left, child []byte, ks int) {
	n := leafCount(child)
	copy(child[leafEntryOff(1, ks):leafEntryOff(n+1, ks)], child[leafEntryOff(0, ks):leafEntryOff(n, ks)])
	ln := leafCount(left)
	copy(child[leafEntryOff(0, ks):leafEntryOff(1, ks)], left[leafEntryOff(ln-1, ks):leafEntryOff(ln, ks)])
	leafSetCount(child, n+1)
	leafSetCount(left, ln-1)
	internalSetKey(parent, pidx-1, leafKey(child, 0, ks), ks)
}

// leafBorrowFromRight moves the first entry of right to the end of
// child and refreshes the parent separator at pidx.
func leafBorrowFromRight(parent []byte, pidx int, child, right []byte, ks int) {
	n := leafCount(child)
	copy(child[leafEntryOff(n, ks):leafEntryOff(n+1, ks)], right[leafEntryOff(0, ks):leafEntryOff(1, ks)])
	leafSetCount(child, n+1)
	rn := leafCount(right)
	copy(right[leafEntryOff(0, ks):leafEntryOff(rn-1, ks)], right[leafEntryOff(1, ks):leafEntryOff(rn, ks)])
	leafSetCount(right, rn-1)
	internalSetKey(parent, pidx, leafKey(right, 0, ks), ks)
}

// leafMergeInto appends every entry of src to dst (which then holds the
// merged leaf) and splices src out of the nextLeaf chain.  The caller
// frees src's block and removes the separator from the parent.
func leafMergeInto(dst, src []byte, ks int) {
	dn, sn := leafCount(dst), leafCount(src)
	copy(dst[leafEntryOff(dn, ks):leafEntryOff(dn+sn, ks)], src[leafEntryOff(0, ks):leafEntryOff(sn, ks)])
	leafSetCount(dst, dn+sn)
	leafSetNext(dst, leafNext(src))
}

// -------------------------------------------------------------------------------------------------------
// Internal nodes.

func internalKeyOff(i, ks int) int { return internalHeaderSize + i*ks }

func internalChildOff(i, ks, maxInt int) int { return internalHeaderSize + maxInt*ks + i*8 }

func internalInit(b []byte) {
	b[0] = nodeInternal
	internalSetCount(b, 0)
}

func internalCount(b []byte) int { return int(binary.LittleEndian.Uint32(b[1:5])) }

func internalSetCount(b []byte, n int) { binary.LittleEndian.PutUint32(b[1:5], uint32(n)) }

// internalKey returns separator i; the slice aliases the block.
func internalKey(b []byte, i, ks int) []byte {
	off := internalKeyOff(i, ks)
	return b[off : off+ks]
}

func internalSetKey(b []byte, i int, key []byte, ks int) {
	copy(b[internalKeyOff(i, ks):internalKeyOff(i+1, ks)], key)
}

func internalChild(b []byte, i, ks, maxInt int) uint64 {
	return binary.LittleEndian.Uint64(b[internalChildOff(i, ks, maxInt):])
}

func internalSetChild(b []byte, i int, no uint64, ks, maxInt int) {
	binary.LittleEndian.PutUint64(b[internalChildOff(i, ks, maxInt):], no)
}

// internalFind returns the child index to descend for key: the index of
// the first separator greater than key (nkeys if key is largest).
// Complexity is O(log₂ nkeys).
func internalFind(b []byte, key []byte, ks int) int {
	lo, hi := 0, internalCount(b)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if bytes.Compare(key, internalKey(b, mid, ks)) < 0 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// internalInsertAt inserts separator key at position i and child
// pointer childNo at position i+1.  The caller guarantees room.
func internalInsertAt(b []byte, i int, key []byte, childNo uint64, ks, maxInt int) {
	n := internalCount(b)
	copy(b[internalKeyOff(i+1, ks):internalKeyOff(n+1, ks)], b[internalKeyOff(i, ks):internalKeyOff(n, ks)])
	internalSetKey(b, i, key, ks)
	copy(b[internalChildOff(i+2, ks, maxInt):internalChildOff(n+2, ks, maxInt)],
		b[internalChildOff(i+1, ks, maxInt):internalChildOff(n+1, ks, maxInt)])
	internalSetChild(b, i+1, childNo, ks, maxInt)
	internalSetCount(b, n+1)
}

// internalRemoveAt removes separator i and child pointer i+1 (the pair
// a merge leaves behind).
func internalRemoveAt(b []byte, i, ks, maxInt int) {
	n := internalCount(b)
	copy(b[internalKeyOff(i, ks):internalKeyOff(n-1, ks)], b[internalKeyOff(i+1, ks):internalKeyOff(n, ks)])
	copy(b[internalChildOff(i+1, ks, maxInt):internalChildOff(n, ks, maxInt)],
		b[internalChildOff(i+2, ks, maxInt):internalChildOff(n+1, ks, maxInt)])
	internalSetCount(b, n-1)
}

// internalSplit divides a FULL internal node around its middle key:
// the receiver keeps the keys below it, nb takes the keys above it, and
// the middle key is PUSHED UP — it stays in neither child (unlike leaf
// separators, internal separators are moved, not copied).  Returns a
// copy of the pushed-up key.
func internalSplit(b, nb []byte, ks, maxInt int) []byte {
	mid := maxInt / 2
	sep := append([]byte(nil), internalKey(b, mid, ks)...)
	rn := maxInt - mid - 1
	internalInit(nb)
	copy(nb[internalKeyOff(0, ks):internalKeyOff(rn, ks)], b[internalKeyOff(mid+1, ks):internalKeyOff(maxInt, ks)])
	copy(nb[internalChildOff(0, ks, maxInt):internalChildOff(rn+1, ks, maxInt)],
		b[internalChildOff(mid+1, ks, maxInt):internalChildOff(maxInt+1, ks, maxInt)])
	internalSetCount(nb, rn)
	internalSetCount(b, mid)
	return sep
}

// internalBorrowFromLeft rotates one key through the parent: the parent
// separator moves down to the front of child, left's last separator
// becomes the parent separator, and left's last child moves to the
// front of child's children.
func internalBorrowFromLeft(parent []byte, pidx int, left, child []byte, ks, maxInt int) {
	cn := internalCount(child)
	copy(child[internalKeyOff(1, ks):internalKeyOff(cn+1, ks)], child[internalKeyOff(0, ks):internalKeyOff(cn, ks)])
	internalSetKey(child, 0, internalKey(parent, pidx-1, ks), ks)
	copy(child[internalChildOff(1, ks, maxInt):internalChildOff(cn+2, ks, maxInt)],
		child[internalChildOff(0, ks, maxInt):internalChildOff(cn+1, ks, maxInt)])
	ln := internalCount(left)
	internalSetChild(child, 0, internalChild(left, ln, ks, maxInt), ks, maxInt)
	internalSetKey(parent, pidx-1, internalKey(left, ln-1, ks), ks)
	internalSetCount(child, cn+1)
	internalSetCount(left, ln-1)
}

// internalBorrowFromRight rotates one key through the parent the other
// way: the parent separator moves down to the end of child, right's
// first separator becomes the parent separator, and right's first
// child moves to the end of child's children.
func internalBorrowFromRight(parent []byte, pidx int, child, right []byte, ks, maxInt int) {
	cn := internalCount(child)
	internalSetKey(child, cn, internalKey(parent, pidx, ks), ks)
	internalSetChild(child, cn+1, internalChild(right, 0, ks, maxInt), ks, maxInt)
	internalSetCount(child, cn+1)
	internalSetKey(parent, pidx, internalKey(right, 0, ks), ks)
	rn := internalCount(right)
	copy(right[internalKeyOff(0, ks):internalKeyOff(rn-1, ks)], right[internalKeyOff(1, ks):internalKeyOff(rn, ks)])
	copy(right[internalChildOff(0, ks, maxInt):internalChildOff(rn, ks, maxInt)],
		right[internalChildOff(1, ks, maxInt):internalChildOff(rn+1, ks, maxInt)])
	internalSetCount(right, rn-1)
}

// internalMergeInto merges src into dst through the separator sepKey:
// dst gains sepKey, then all of src's separators and children.  The
// caller frees src's block and removes the separator from the parent.
func internalMergeInto(dst, src, sepKey []byte, ks, maxInt int) {
	dn, sn := internalCount(dst), internalCount(src)
	internalSetKey(dst, dn, sepKey, ks)
	copy(dst[internalKeyOff(dn+1, ks):internalKeyOff(dn+1+sn, ks)], src[internalKeyOff(0, ks):internalKeyOff(sn, ks)])
	copy(dst[internalChildOff(dn+1, ks, maxInt):internalChildOff(dn+1+sn+1, ks, maxInt)],
		src[internalChildOff(0, ks, maxInt):internalChildOff(sn+1, ks, maxInt)])
	internalSetCount(dst, dn+1+sn)
}

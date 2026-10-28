/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// On-disk bucket and directory-block encoding, and the structural
// primitives that mutate buckets in place inside a cached block image.
// Every function here operates on a raw block ([]byte of BlockSize);
// the caller (the Index methods in index.go) holds the store lock and
// marks blocks dirty after mutation.
//
// Block layouts (all integers little-endian, ks = keySize):
//
//	Bucket:     type(1) count(4) depth(4) overflow(8) |
//	            count × (key ks, value 8)   — entries UNsorted
//	Directory:  type(1) reserved(7) | slots… (each a uint64 bucket
//	            block number)
//
// A bucket block is either a PRIMARY bucket (type blkBucket, pointed
// at by one or more directory slots, carrying the local depth) or an
// OVERFLOW block (type blkOverflow, reachable only through a primary's
// overflow chain, depth 0).  Entries within a bucket are in no order:
// a hash index never compares keys for ordering, only for equality
// (bytes.Equal), so inserts append and searches scan.  The overflow
// link exists only for the two degenerate split cases described in the
// package documentation (all-identical hashes, or the depth cap); a
// healthy store has no chains.
//
// The bucket header is 17 bytes; a bucket holds
// (BlockSize-17)/(ks+8) entries — for 8-byte keys, 254 per 4 KiB block
// (the B+ tree of b_tree_disk_ts holds 255; the hash bucket spends two
// more header bytes and saves the whole comparison descent).

package hash_index_disk_ts

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Block type bytes (offset 0 of every block).
const (
	blkBucket   byte = 1 // a primary bucket: directory slots point here
	blkOverflow byte = 2 // an overflow bucket: chained from a primary
	blkDir      byte = 3 // a directory block: the bucket-pointer array
)

const (
	bucketHeaderSize = 17 // type(1) + count(4) + depth(4) + overflow(8)
	dirHeaderSize    = 8  // type(1) + reserved(7); slots start at 8
)

// maxEntriesPerBucket returns how many (key, value) entries fit in a
// bucket block.
func maxEntriesPerBucket(blockSize, keySize int) int {
	return (blockSize - bucketHeaderSize) / (keySize + 8)
}

// slotsPerDirBlock returns how many bucket pointers fit in a directory
// block.
func slotsPerDirBlock(blockSize int) int {
	return (blockSize - dirHeaderSize) / 8
}

// dirBlockCount returns how many contiguous blocks a directory of the
// given depth occupies (at least one).
func (s *Store) dirBlockCount(depth uint32) uint64 {
	per := uint64(slotsPerDirBlock(s.blockSize))
	slots := uint64(1) << depth // depth ≤ maxDepth, no overflow
	n := (slots + per - 1) / per
	if n < 1 {
		n = 1
	}
	return n
}

// -------------------------------------------------------------------------------------------------------
// Bucket blocks.

func bucketEntryOff(i, ks int) int { return bucketHeaderSize + i*(ks+8) }

// bucketInit resets a block to an empty bucket of the given kind.
// blkBucket keeps the local depth; blkOverflow always stores 0.
func bucketInit(b []byte, typ byte, depth uint32) {
	b[0] = typ
	bucketSetCount(b, 0)
	bucketSetDepth(b, depth)
	bucketSetOverflow(b, 0)
}

func bucketCount(b []byte) int { return int(binary.LittleEndian.Uint32(b[1:5])) }

func bucketSetCount(b []byte, n int) { binary.LittleEndian.PutUint32(b[1:5], uint32(n)) }

func bucketDepth(b []byte) uint32 { return binary.LittleEndian.Uint32(b[5:9]) }

func bucketSetDepth(b []byte, d uint32) { binary.LittleEndian.PutUint32(b[5:9], d) }

func bucketOverflow(b []byte) uint64 { return binary.LittleEndian.Uint64(b[9:17]) }

func bucketSetOverflow(b []byte, no uint64) { binary.LittleEndian.PutUint64(b[9:17], no) }

// bucketKey returns the key bytes of entry i; the slice aliases the
// block.
func bucketKey(b []byte, i, ks int) []byte {
	off := bucketEntryOff(i, ks)
	return b[off : off+ks]
}

func bucketValue(b []byte, i, ks int) uint64 {
	return binary.LittleEndian.Uint64(b[bucketEntryOff(i, ks)+ks:])
}

func bucketSetValue(b []byte, i int, value uint64, ks int) {
	binary.LittleEndian.PutUint64(b[bucketEntryOff(i, ks)+ks:], value)
}

// bucketFind returns the index of the first entry equal to key.  The
// scan is linear — entries are unsorted, and equality of the encodings
// is the only comparison a hash index ever makes.  The average bucket
// holds maxEnt/2 entries (127 at 4 KiB blocks and 8-byte keys), so the
// per-entry cost is the whole cost of a Search: keys of 8 bytes (the
// common fixed-width encoding) compare as one integer, wider keys
// pre-filter on their first 8 bytes before the full bytes.Equal, and
// the rest fall back to bytes.Equal per entry.
// Complexity is O(entries in the block).
func bucketFind(b []byte, key []byte, ks int) (int, bool) {
	n := bucketCount(b)
	switch {
	case ks == 8:
		want := binary.LittleEndian.Uint64(key)
		for i := 0; i < n; i++ {
			if binary.LittleEndian.Uint64(b[bucketEntryOff(i, ks):]) == want {
				return i, true
			}
		}
	case ks > 8:
		want := binary.LittleEndian.Uint64(key)
		for i := 0; i < n; i++ {
			off := bucketEntryOff(i, ks)
			if binary.LittleEndian.Uint64(b[off:]) == want && bytes.Equal(b[off:off+ks], key) {
				return i, true
			}
		}
	default:
		for i := 0; i < n; i++ {
			if bytes.Equal(bucketKey(b, i, ks), key) {
				return i, true
			}
		}
	}
	return 0, false
}

// bucketAppend adds (key, value) at the end of the bucket.  The caller
// guarantees the bucket has room (count < maxEntriesPerBucket).
func bucketAppend(b []byte, key []byte, value uint64, ks int) {
	i := bucketCount(b)
	off := bucketEntryOff(i, ks)
	copy(b[off:off+ks], key)
	binary.LittleEndian.PutUint64(b[off+ks:], value)
	bucketSetCount(b, i+1)
}

// bucketRemoveAt removes the entry at position i by shifting the
// later entries left (order is irrelevant, but a shift keeps the
// removal position-independent).
func bucketRemoveAt(b []byte, i, ks int) {
	n := bucketCount(b)
	copy(b[bucketEntryOff(i, ks):bucketEntryOff(n-1, ks)], b[bucketEntryOff(i+1, ks):bucketEntryOff(n, ks)])
	bucketSetCount(b, n-1)
}

// checkBucketType validates a freshly loaded block as the expected
// bucket kind — a cheap guard so a trashed file surfaces as ErrCorrupt
// instead of nonsense results or a scribbled superblock.
func checkBucketType(b []byte, no uint64, want byte) error {
	if b[0] != want {
		return fmt.Errorf("%w: block %d has type %d, want %d", ErrCorrupt, no, b[0], want)
	}
	return nil
}

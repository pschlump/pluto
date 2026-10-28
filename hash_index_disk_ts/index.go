/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The generic index layer: Index[K] maps fixed-size encoded keys to
// uint64 values on top of a Store, with an extendible hash directory.
// There is no Compare function and no ordering anywhere in this file —
// a key is located by its hash (the frozen xxHash64 of the encoding)
// and identified by bytes.Equal of the encoding.  EncodeKey must be
// injective; that is the whole key contract.
//
// Insert walks the bucket chain to replace an existing entry or append
// to the first block with room.  When the chain is full it SPLITS the
// primary bucket: every entry re-partitions by one more hash bit, the
// local depth rises, and the directory slots that pointed at the
// bucket divide between the old and new blocks; a bucket that already
// fills its single directory slot doubles the directory first.  The
// retry loop terminates because each split strictly raises the local
// depth of the bucket the new key lands in — the only escape is the
// two degenerate cases (every entry carries the same 64-bit hash, or
// the depth cap is reached), which append to the overflow chain
// instead.
//
// Delete removes from wherever the entry lives and frees an emptied
// OVERFLOW block; primaries are never merged away and the directory
// never shrinks — the high-water-mark policy documented in the package
// comment (the b_tree_disk_ts stale-separator precedent).

package hash_index_disk_ts

import (
	"encoding/binary"
	"fmt"
	"iter"
)

// maxDepth caps the global (and local) directory depth: 2^26 slots is
// a 512 MiB directory, addressing ~2^26 buckets.  Reaching the cap
// requires an adversarial key set crafted against the frozen hash; a
// bucket that cannot split past the cap grows an overflow chain
// instead (still correct, slower).
const maxDepth uint32 = 26

// IndexConfig describes one named index in a store.  Re-opening an
// existing Name attaches to the existing index (KeySize must match).
type IndexConfig[K any] struct {
	Name      string                // ≤ 31 bytes
	KeySize   int                   // fixed encoded byte length of a key
	EncodeKey func(k K, buf []byte) // must write exactly KeySize bytes, injectively
	DecodeKey func(buf []byte) K    // must copy out of buf if K is a reference type
}

// Index is one named hash index inside a Store.  Create or attach with
// NewIndex; re-attach after reopening the store (an Index does not
// survive its Store being closed).  Index methods are safe for
// concurrent use — they all take the store lock; see the package
// documentation for why reads take the same lock as writes.
type Index[K any] struct {
	store   *Store
	name    string
	slot    int // registry slot in the superblock
	keySize int
	maxEnt  int // max entries in a bucket block

	encode func(k K, buf []byte)
	decode func(buf []byte) K

	// hash is the frozen xxHash64 of the encoded key bytes (see
	// hash.go) — a field, not a call, only so the degenerate-collision
	// tests can force 64-bit collisions; production code never touches
	// it.  Changing the default breaks on-disk compatibility.
	hash func(key []byte) uint64
}

// NewIndex creates a named index in the store, or attaches to the
// existing index of that name (KeySize must match, else
// ErrKeySizeMismatch).  A new index allocates one directory block (a
// single slot, depth 0) and one empty primary bucket.
//
// It panics on programmer errors: a nil store, nil EncodeKey or
// DecodeKey, KeySize ≤ 0, an empty Name, or a Name longer than 31
// bytes.  IO failures, a full registry (ErrTooManyIndexes) and a key
// too large for the block size (ErrKeyTooLarge) are returned as
// errors.
// Complexity is O(1).
func NewIndex[K any](s *Store, cfg IndexConfig[K]) (*Index[K], error) {
	if s == nil {
		panic("hash_index_disk_ts: NewIndex called with a nil store")
	}
	if cfg.EncodeKey == nil {
		panic("hash_index_disk_ts: NewIndex called with a nil EncodeKey function")
	}
	if cfg.DecodeKey == nil {
		panic("hash_index_disk_ts: NewIndex called with a nil DecodeKey function")
	}
	if cfg.KeySize <= 0 {
		panic("hash_index_disk_ts: NewIndex called with KeySize <= 0 (KeySize is the fixed encoded byte length of a key)")
	}
	if len(cfg.Name) == 0 {
		panic("hash_index_disk_ts: NewIndex called with an empty index name")
	}
	if len(cfg.Name) > maxIndexNameLen {
		panic(fmt.Sprintf("hash_index_disk_ts: NewIndex called with an index name of %d bytes, the maximum is %d", len(cfg.Name), maxIndexNameLen))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}

	ix := &Index[K]{
		store:   s,
		name:    cfg.Name,
		keySize: cfg.KeySize,
		maxEnt:  maxEntriesPerBucket(s.blockSize, cfg.KeySize),
		encode:  cfg.EncodeKey,
		decode:  cfg.DecodeKey,
		hash:    xxh64,
	}

	slot, found, err := s.findIndexLocked(cfg.Name)
	if err != nil {
		return nil, err
	}
	if found {
		_, _, ks, _, err := s.readEntryMeta(slot)
		if err != nil {
			return nil, err
		}
		if int(ks) != cfg.KeySize {
			return nil, fmt.Errorf("%w: index %q was created with key size %d", ErrKeySizeMismatch, cfg.Name, ks)
		}
		ix.slot = slot
		return ix, nil
	}

	if ix.maxEnt < 4 {
		return nil, fmt.Errorf("%w: key size %d in %d-byte blocks", ErrKeyTooLarge, cfg.KeySize, s.blockSize)
	}
	slot, err = s.freeSlotLocked()
	if err != nil {
		return nil, err
	}
	if slot < 0 {
		return nil, fmt.Errorf("%w: %d indexes in %d-byte blocks", ErrTooManyIndexes, s.registryCap(), s.blockSize)
	}
	dirNo, dir, err := s.allocBlock()
	if err != nil {
		return nil, err
	}
	dir.data[0] = blkDir
	bno, b, err := s.allocBlock()
	if err != nil {
		return nil, err
	}
	bucketInit(b.data, blkBucket, 0)
	binary.LittleEndian.PutUint64(dir.data[dirHeaderSize:], bno) // slot 0
	s.markDirty(dir)
	if err := s.writeEntryLocked(slot, cfg.Name, dirNo, 0, uint32(cfg.KeySize), 0); err != nil {
		return nil, err
	}
	ix.slot = slot
	return ix, nil
}

// -------------------------------------------------------------------------------------------------------
// Directory access.  The directory is a contiguous run of blocks at
// dirStart; slot i lives in block dirStart+i/per at byte offset
// dirHeaderSize+(i%per)*8.  Each helper fetches its block, mutates,
// and dirties it before returning — no block image is referenced
// across another cacheGet (the eviction pitfall; see cacheGet).

// dirGet returns the bucket block number in directory slot i.
// Caller holds s.mu.
func (t *Index[K]) dirGet(dirStart, slot uint64) (uint64, error) {
	s := t.store
	per := uint64(slotsPerDirBlock(s.blockSize))
	b, err := s.cacheGet(dirStart + slot/per)
	if err != nil {
		return 0, err
	}
	no := binary.LittleEndian.Uint64(b.data[dirHeaderSize+(slot%per)*8:])
	if no == 0 {
		return 0, fmt.Errorf("%w: directory slot %d is empty (block 0 is the superblock, never a bucket)", ErrCorrupt, slot)
	}
	return no, nil
}

// dirSet stores a bucket block number into directory slot i.
// Caller holds s.mu.
func (t *Index[K]) dirSet(dirStart, slot, no uint64) error {
	s := t.store
	per := uint64(slotsPerDirBlock(s.blockSize))
	b, err := s.cacheGet(dirStart + slot/per)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(b.data[dirHeaderSize+(slot%per)*8:], no)
	s.markDirty(b)
	return nil
}

// slotOf returns the directory slot a hash maps to at the given global
// depth: the TOP depth bits of the hash (depth 0 is the single slot
// 0).
func slotOf(h uint64, depth uint32) uint64 {
	if depth == 0 {
		return 0
	}
	return h >> (64 - depth)
}

// doubleDirectory doubles the directory from depth to depth+1,
// returning the (possibly relocated) start and the new depth.  Doubling
// repeats every pointer at slots 2i and 2i+1.  When the bigger
// directory fits in the same block count the rewrite happens in place,
// from the TOP down — slot i takes old slot i>>1, which is always a
// lower, not-yet-rewritten slot.  Otherwise the directory relocates to
// a fresh contiguous run at the end of the file (the free list cannot
// promise a run) and the old blocks are freed for reuse as buckets.
// Caller holds s.mu.
func (t *Index[K]) doubleDirectory(dirStart uint64, depth uint32) (uint64, uint32, error) {
	s := t.store
	newDepth := depth + 1
	oldCount := s.dirBlockCount(depth)
	newCount := s.dirBlockCount(newDepth)
	slots := uint64(1) << newDepth

	if newCount == oldCount {
		for i := int64(slots) - 1; i >= 1; i-- {
			v, err := t.dirGet(dirStart, uint64(i)>>1)
			if err != nil {
				return 0, 0, err
			}
			if err := t.dirSet(dirStart, uint64(i), v); err != nil {
				return 0, 0, err
			}
		}
		if err := s.setEntryDirLocked(t.slot, dirStart, newDepth); err != nil {
			return 0, 0, err
		}
		return dirStart, newDepth, nil
	}

	newStart, err := s.allocContiguous(newCount)
	if err != nil {
		return 0, 0, err
	}
	for i := uint64(0); i < slots; i++ {
		v, err := t.dirGet(dirStart, i>>1)
		if err != nil {
			return 0, 0, err
		}
		if err := t.dirSet(newStart, i, v); err != nil {
			return 0, 0, err
		}
	}
	for i := uint64(0); i < oldCount; i++ {
		if err := s.freeBlock(dirStart + i); err != nil {
			return 0, 0, err
		}
	}
	if err := s.setEntryDirLocked(t.slot, newStart, newDepth); err != nil {
		return 0, 0, err
	}
	return newStart, newDepth, nil
}

// -------------------------------------------------------------------------------------------------------
// Insert / Search / Delete.

// bucketEntry is one (key, value) pair copied out of a bucket block —
// the split path gathers entries as copies so no block image is
// referenced across the allocations that rebuild the buckets.
type bucketEntry struct {
	k []byte
	v uint64
}

// Insert adds key → value to the index.  It returns true when the key
// is new and false when an existing entry's value was replaced.  The
// change is in the cache when Insert returns; it is crash-durable
// after the next completed flush (see Sync and the package durability
// contract).  Insert on a closed store returns ErrClosed.
// Complexity is O(1) expected block accesses (one directory block, one
// bucket block); splits and directory doublings are amortized O(1).
func (t *Index[K]) Insert(key K, value uint64) (bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, ErrClosed
	}
	return t.insertNoLock(buf, value)
}

// insertNoLock is Insert without locking.  Caller holds s.mu.
func (t *Index[K]) insertNoLock(key []byte, value uint64) (bool, error) {
	s := t.store
	ks := t.keySize
	h := t.hash(key)

	for {
		dirStart, _, _, depth, err := s.readEntryMeta(t.slot)
		if err != nil {
			return false, err
		}
		slot := slotOf(h, depth)
		bno, err := t.dirGet(dirStart, slot)
		if err != nil {
			return false, err
		}

		// Walk the chain: replace an existing entry, or note the first
		// block with room.  No block is mutated during the walk.
		roomNo := uint64(0) // first chain block with room, 0 = none
		tailNo := bno
		cur, err := s.cacheGet(bno)
		if err != nil {
			return false, err
		}
		if err := checkBucketType(cur.data, bno, blkBucket); err != nil {
			return false, err
		}
		localDepth := bucketDepth(cur.data)
		for {
			if i, ok := bucketFind(cur.data, key, ks); ok {
				bucketSetValue(cur.data, i, value, ks)
				s.markDirty(cur)
				return false, nil
			}
			if bucketCount(cur.data) < t.maxEnt && roomNo == 0 {
				roomNo = cur.no
			}
			next := bucketOverflow(cur.data)
			if next == 0 {
				break
			}
			tailNo = next
			if cur, err = s.cacheGet(next); err != nil {
				return false, err
			}
			if err := checkBucketType(cur.data, next, blkOverflow); err != nil {
				return false, err
			}
		}

		if roomNo != 0 {
			// Re-fetch by number: the roomy block may have been evicted
			// while the walk loaded later chain blocks, and writing an
			// evicted image would be lost (see cacheGet).
			b, err := s.cacheGet(roomNo)
			if err != nil {
				return false, err
			}
			bucketAppend(b.data, key, value, ks)
			s.markDirty(b)
			if err := s.addEntryLengthLocked(t.slot, 1); err != nil {
				return false, err
			}
			return true, nil
		}

		// The chain is full and the key is absent: split.  Gather every
		// entry of the chain as copies first — the rebuild below
		// allocates blocks, and a cached block referenced across an
		// allocation can be evicted out from under the writer.
		var entries []bucketEntry
		var chain []uint64 // overflow blocks the split frees afterwards
		cur, err = s.cacheGet(bno)
		if err != nil {
			return false, err
		}
		for {
			n := bucketCount(cur.data)
			for i := 0; i < n; i++ {
				k := make([]byte, ks)
				copy(k, bucketKey(cur.data, i, ks))
				entries = append(entries, bucketEntry{k, bucketValue(cur.data, i, ks)})
			}
			next := bucketOverflow(cur.data)
			if next == 0 {
				break
			}
			chain = append(chain, next)
			if cur, err = s.cacheGet(next); err != nil {
				return false, err
			}
		}

		// A split separates entries by one hash bit; when every entry
		// (including the one being inserted) carries the same hash, no
		// bit ever separates them — and at the depth cap no bit may.  In
		// both cases the new entry goes onto the overflow chain instead.
		hashes := make([]uint64, len(entries))
		allSame := true
		for i, e := range entries {
			hashes[i] = t.hash(e.k)
			if hashes[i] != h {
				allSame = false
			}
		}
		if allSame || localDepth >= maxDepth {
			no, nb, err := s.allocBlock()
			if err != nil {
				return false, err
			}
			bucketInit(nb.data, blkOverflow, 0)
			bucketAppend(nb.data, key, value, ks) // nb is dirty from allocBlock
			tail, err := s.cacheGet(tailNo)
			if err != nil {
				return false, err
			}
			bucketSetOverflow(tail.data, no)
			s.markDirty(tail)
			if err := s.addEntryLengthLocked(t.slot, 1); err != nil {
				return false, err
			}
			return true, nil
		}

		// p — the bucket's pattern, its top localDepth hash bits — MUST
		// be computed from the pre-doubling slot and depth; a doubling
		// re-indexes every slot.
		p := slot >> (depth - localDepth)
		if localDepth == depth {
			if dirStart, depth, err = t.doubleDirectory(dirStart, depth); err != nil {
				return false, err
			}
		}
		nd := localDepth + 1
		span := uint64(1) << (depth - nd)
		base := p << (depth - localDepth)

		// Partition by hash bit localDepth (0-indexed from the top):
		// bit 0 stays, bit 1 moves to the new bucket.
		var stay, move []bucketEntry
		for i, e := range entries {
			if hashes[i]>>(63-localDepth)&1 == 1 {
				move = append(move, e)
			} else {
				stay = append(stay, e)
			}
		}

		newNo, newB, err := s.allocBlock()
		if err != nil {
			return false, err
		}
		bucketInit(newB.data, blkBucket, nd)
		if err := t.fillBucketChain(newB, move); err != nil {
			return false, err
		}

		pb, err := s.cacheGet(bno)
		if err != nil {
			return false, err
		}
		s.markDirty(pb) // before fillBucketChain's allocations
		bucketInit(pb.data, blkBucket, nd)
		if err := t.fillBucketChain(pb, stay); err != nil {
			return false, err
		}

		// The upper half of the bucket's slot range moves to the new
		// bucket; the lower half already points at the old one.
		for i := base + span; i < base+2*span; i++ {
			if err := t.dirSet(dirStart, i, newNo); err != nil {
				return false, err
			}
		}
		for _, no := range chain {
			if err := s.freeBlock(no); err != nil {
				return false, err
			}
		}
		// Retry against the post-split directory; the target bucket's
		// local depth rose, so the loop is bounded by maxDepth.
	}
}

// fillBucketChain fills primary (already bucketInit'ed) with entries,
// allocating overflow blocks for the excess.  Every block is dirty
// before another is allocated, so no in-progress image can be evicted.
// Caller holds s.mu.
func (t *Index[K]) fillBucketChain(primary *cacheBlock, entries []bucketEntry) error {
	s := t.store
	ks := t.keySize
	cur := primary
	for i := range entries {
		if bucketCount(cur.data) >= t.maxEnt {
			no, nb, err := s.allocBlock()
			if err != nil {
				return err
			}
			bucketInit(nb.data, blkOverflow, 0)
			bucketSetOverflow(cur.data, no)
			s.markDirty(cur)
			cur = nb
		}
		bucketAppend(cur.data, entries[i].k, entries[i].v, ks)
		s.markDirty(cur)
	}
	return nil
}

// Search returns the value stored for key and whether it was found.
// Search on a closed store returns ErrClosed.
// Complexity is O(1) expected block accesses: one directory block and
// one bucket block (plus the degenerate overflow chain, which does not
// exist in a healthy store).
func (t *Index[K]) Search(key K) (uint64, bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, false, ErrClosed
	}
	return t.searchNoLock(buf)
}

// searchNoLock is Search without locking.  Caller holds s.mu.
func (t *Index[K]) searchNoLock(key []byte) (uint64, bool, error) {
	s := t.store
	dirStart, _, _, depth, err := s.readEntryMeta(t.slot)
	if err != nil {
		return 0, false, err
	}
	bno, err := t.dirGet(dirStart, slotOf(t.hash(key), depth))
	if err != nil {
		return 0, false, err
	}
	cur, err := s.cacheGet(bno)
	if err != nil {
		return 0, false, err
	}
	if err := checkBucketType(cur.data, bno, blkBucket); err != nil {
		return 0, false, err
	}
	for {
		if i, ok := bucketFind(cur.data, key, t.keySize); ok {
			return bucketValue(cur.data, i, t.keySize), true, nil
		}
		next := bucketOverflow(cur.data)
		if next == 0 {
			return 0, false, nil
		}
		if cur, err = s.cacheGet(next); err != nil {
			return 0, false, err
		}
		if err := checkBucketType(cur.data, next, blkOverflow); err != nil {
			return 0, false, err
		}
	}
}

// Delete removes key from the index, returning true if it was present.
// Deleted-from buckets are never merged and the directory never
// shrinks — the high-water-mark policy; only an emptied OVERFLOW block
// is freed onto the store's free list.  Delete on a closed store
// returns ErrClosed.
// Complexity is O(1) expected block accesses.
func (t *Index[K]) Delete(key K) (bool, error) {
	buf := make([]byte, t.keySize)
	t.encode(key, buf)
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, ErrClosed
	}
	return t.deleteNoLock(buf)
}

// deleteNoLock is Delete without locking.  Caller holds s.mu.
func (t *Index[K]) deleteNoLock(key []byte) (bool, error) {
	s := t.store
	ks := t.keySize
	dirStart, _, _, depth, err := s.readEntryMeta(t.slot)
	if err != nil {
		return false, err
	}
	bno, err := t.dirGet(dirStart, slotOf(t.hash(key), depth))
	if err != nil {
		return false, err
	}
	prevNo := uint64(0)
	curNo := bno
	for {
		cur, err := s.cacheGet(curNo)
		if err != nil {
			return false, err
		}
		want := byte(blkOverflow)
		if prevNo == 0 {
			want = blkBucket
		}
		if err := checkBucketType(cur.data, curNo, want); err != nil {
			return false, err
		}
		if i, ok := bucketFind(cur.data, key, ks); ok {
			bucketRemoveAt(cur.data, i, ks)
			s.markDirty(cur)
			// An emptied overflow block is unlinked and freed; the
			// primary stays — the directory points at it.
			if prevNo != 0 && bucketCount(cur.data) == 0 {
				next := bucketOverflow(cur.data)
				prev, err := s.cacheGet(prevNo)
				if err != nil {
					return false, err
				}
				bucketSetOverflow(prev.data, next)
				s.markDirty(prev)
				if err := s.freeBlock(curNo); err != nil {
					return false, err
				}
			}
			if err := s.addEntryLengthLocked(t.slot, -1); err != nil {
				return false, err
			}
			return true, nil
		}
		next := bucketOverflow(cur.data)
		if next == 0 {
			return false, nil
		}
		prevNo, curNo = curNo, next
	}
}

// -------------------------------------------------------------------------------------------------------
// Length and iteration.

// Length returns the number of entries in the index, read from the
// registry entry under the store lock.  On a closed store it returns 0.
// Complexity is O(1).
func (t *Index[K]) Length() uint64 {
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0
	}
	_, n, _, _, err := s.readEntryMeta(t.slot)
	if err != nil {
		return 0
	}
	return n
}

// All returns an iterator over every (key, value) pair in the index.
// The order is BUCKET order — the physical order of the hash
// directory — which is deterministic for a given store but is NOT key
// order and carries no useful guarantee; when an order matters,
// collect and sort (MarshalJSON does).  The pairs are a SNAPSHOT: All
// collects them under one hold of the store lock (costing O(n) memory)
// and the returned iterator walks that slice with no lock held, so it
// is unaffected by — and does not block — concurrent inserts and
// deletes.  On a closed store (or an IO error) the iterator is empty.
// Complexity is O(n) to build the snapshot, O(1) per yield.
func (t *Index[K]) All() iter.Seq2[K, uint64] {
	pairs, err := t.collect()
	return pairSeq(pairs, err)
}

type kvPair[K any] struct {
	k K
	v uint64
}

// pairSeq turns a snapshot (or an error, which yields nothing) into an
// iterator.
func pairSeq[K any](pairs []kvPair[K], err error) iter.Seq2[K, uint64] {
	return func(yield func(K, uint64) bool) {
		if err != nil {
			return
		}
		for _, p := range pairs {
			if !yield(p.k, p.v) {
				return
			}
		}
	}
}

// collect snapshots the index's pairs under the store lock by walking
// every directory slot (deduplicating the buckets several slots share)
// and each bucket's overflow chain.
func (t *Index[K]) collect() ([]kvPair[K], error) {
	s := t.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	return t.collectNoLock()
}

// collectNoLock is collect without locking.  Caller holds s.mu.
func (t *Index[K]) collectNoLock() ([]kvPair[K], error) {
	s := t.store
	dirStart, _, _, depth, err := s.readEntryMeta(t.slot)
	if err != nil {
		return nil, err
	}
	slots := uint64(1) << depth
	seen := make(map[uint64]bool, slots)
	var out []kvPair[K]
	ks := t.keySize
	for i := uint64(0); i < slots; i++ {
		bno, err := t.dirGet(dirStart, i)
		if err != nil {
			return nil, err
		}
		if seen[bno] {
			continue
		}
		seen[bno] = true
		curNo := bno
		for {
			cur, err := s.cacheGet(curNo)
			if err != nil {
				return nil, err
			}
			n := bucketCount(cur.data)
			for j := 0; j < n; j++ {
				out = append(out, kvPair[K]{t.decode(bucketKey(cur.data, j, ks)), bucketValue(cur.data, j, ks)})
			}
			next := bucketOverflow(cur.data)
			if next == 0 {
				break
			}
			curNo = next
		}
	}
	return out, nil
}

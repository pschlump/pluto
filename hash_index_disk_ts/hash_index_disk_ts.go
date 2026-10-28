/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_index_disk_ts implements a disk-backed, thread-safe
// hash index: one data file of fixed-size blocks, an LRU write-back
// block cache, and a write-ahead journal for crash recovery.  It is
// the equality-lookup counterpart of b_tree_disk_ts — use it when
// lookups are exact-match only and must touch as few blocks as
// possible: a Search reads ONE directory block and ONE bucket block,
// expected, versus the O(log n) block descent of a B+ tree.  There is
// no ordered iteration and no range scan; when those are needed use
// b_tree_disk_ts instead (or both over the same record set — see the
// multi-index example).
//
// Values are ALWAYS uint64 — record offsets or IDs into the caller's
// own data set.  This is an index library, not a record store: the
// index maps fixed-size keys to 8-byte values and the caller resolves
// the values.
//
// # Equality, not ordering
//
// The index never orders keys: there is no Compare function.  A key is
// found by hashing the ENCODED key with a frozen internal xxHash64
// (seed 0 — the hyperloglog precedent; the hash is part of the on-disk
// format contract) and comparing candidate entries with bytes.Equal of
// their encodings.  The only caller contract is that EncodeKey is
// INJECTIVE: two distinct keys must encode to two distinct byte
// strings (a non-injective encoding silently collapses keys — distinct
// K values with the same encoding are the same key to the index).
//
// # On-disk layout
//
// The data file is a random-access array of BlockSize-byte blocks (4096
// by default; chosen at creation, must be ≥ 512 and a multiple of 512,
// and is verified on reopen).  All integers on disk are little-endian.
//
//	Block 0      — the superblock: magic, format version, block size,
//	               numBlocks, freeListHead, and a fixed-capacity
//	               registry of named indexes (each entry: a 32-byte
//	               name, dirStart, length, keySize, depth).  The
//	               registry holds (BlockSize-32)/56 indexes: 8 at
//	               512-byte blocks, 72 at 4 KiB blocks.
//	Free blocks  — linked from freeListHead; the first 8 bytes of a
//	               free block hold the next free block number, 0 = end.
//	Directory    — a contiguous run of dirStart blocks holding the
//	               bucket-pointer array of the extendible hash: slot i
//	               (8 bytes) is the block number of the primary bucket
//	               for every key whose hash starts with the bit pattern
//	               i.  2^depth slots, (BlockSize-8)/8 per block.
//	Bucket       — type byte, count, local depth, overflow link, then
//	               count UNsorted (key, value) entries, chained to
//	               overflow buckets when a bucket cannot split further.
//
// Block number 0 is the superblock, so 0 doubles as the nil sentinel
// for block pointers (free-list end, end of chain).  Superblock
// mutations go through the cache and are journaled like any block.
//
// # The extendible hash
//
// A directory of 2^depth slots; the slot for a hash h is its TOP depth
// bits (h >> (64-depth); depth 0 means the single slot 0).  Each
// primary bucket records its local depth d — every entry in the bucket
// shares the top d hash bits, and 2^(depth-d) directory slots point at
// it.  When a full bucket must take one more entry it splits: its
// entries re-partition by hash bit d, the local depth rises to d+1, and
// half the slots that pointed at it move to the fresh bucket.  When the
// bucket already spans a single slot (d == depth) the directory doubles
// first — each old pointer appears at slots 2i and 2i+1 — which is an
// O(directory) rewrite amortized O(1) per insert (the doubling is the
// only whole-structure rewrite in the package, and the depth is capped
// at maxDepth).
//
// Two degenerate cases fall back to overflow chains (rare, and
// structurally impossible without 64-bit hash collisions or an
// adversarial key set crafted against the frozen hash): a bucket whose
// entries all carry the SAME hash (no bit ever separates them) and a
// bucket at the depth cap.  Searches then walk the chain.
//
// Deletes never merge buckets and never shrink the directory — the
// directory is a high-water mark, exactly like the stale separators of
// the b_tree_disk_ts B+ tree: bucket blocks and directory slots stay
// allocated and are simply refilled by later inserts.  Only emptied
// OVERFLOW blocks are freed (onto the store free list).
//
// # Durability contract — READ THIS
//
// Insert and Delete return as soon as the change is in the in-memory
// cache.  The change becomes crash-durable only when a flush completes:
// the background flusher (every FlushInterval, and whenever more than
// half the cache is dirty), an explicit Sync, or Close.  Every flush
// appends ONE journal batch with the full images of all dirty blocks,
// fsyncs the journal, writes the data blocks, fsyncs the data file, and
// only then truncates the journal.
//
// Therefore: a crash NEVER corrupts the store and NEVER loses state
// that was flush-acknowledged (Sync returned, Close returned, or a
// background flush completed).  Operations after the last completed
// flush may be absent after recovery.  Recovery replays every complete,
// CRC-valid journal batch and ignores a torn or corrupt tail.
//
// # Locking contract
//
// One sync.Mutex guards the whole store.  Reads take the SAME lock as
// writes — like union_find_ts, where Find takes the write lock because
// path halving mutates, here Search mutates the LRU cache on every hit,
// so there is no read-only path.  All public Index and Store methods
// lock; none of them calls another locked method (unexported noLock
// helpers do the work).  The flusher releases the lock while it does
// file IO; per-block dirty epochs tell it which blocks were re-dirtied
// while a flush was in flight.
//
// CacheBlocks is a SOFT cap: a block referenced by an in-flight
// operation or waiting to be flushed is not evictable, so when every
// cached block is pinned the cache grows past the cap for a moment
// instead of stalling the operation on a lock-ordering hazard; the
// flusher brings it back under the cap.  Memory stays bounded by
// CacheBlocks×BlockSize plus the working set of one operation.
//
// # Panics
//
// Panics are reserved for programmer errors, caught at construction:
//
//	NewIndex with a nil EncodeKey or DecodeKey function.
//	NewIndex with KeySize ≤ 0, an empty Name, or a Name over 31 bytes.
//	NewIndex on a nil *Store.
//
// Everything else — IO failures, a corrupt file, a block-size mismatch,
// a too-large key — is returned as an error.  Using the store after
// Close returns ErrClosed (check with errors.Is).
package hash_index_disk_ts

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// File-format constants.  The store magic reads "pluto-hx".
const (
	storeMagic   uint64 = 0x706c75746f2d6878
	storeVersion uint32 = 1

	// DefaultBlockSize is the block size used when StoreConfig.BlockSize
	// is 0.
	DefaultBlockSize = 4096
	// DefaultCacheBlocks is the cache capacity used when
	// StoreConfig.CacheBlocks is 0.
	DefaultCacheBlocks = 1024
	// DefaultFlushInterval is the flush tick used when
	// StoreConfig.FlushInterval is 0.
	DefaultFlushInterval = 100 * time.Millisecond

	// maxIndexNameLen is the longest index name the registry accepts;
	// names are stored NUL-padded in a 32-byte field.
	maxIndexNameLen = 31
)

// Sentinel errors returned by this package; check them with errors.Is.
var (
	// ErrClosed is returned by every Store and Index operation on a closed
	// store.  (Length, which has no error return, yields 0; Indexes yields
	// nil; All yields nothing.)
	ErrClosed = errors.New("hash_index_disk_ts: store is closed")
	// ErrKeyTooLarge is returned by NewIndex when the key size leaves room
	// for fewer than 4 entries per bucket block; use bigger blocks or a
	// smaller key encoding.
	ErrKeyTooLarge = errors.New("hash_index_disk_ts: key size too large for the block size (a bucket must hold at least 4 entries)")
	// ErrKeySizeMismatch is returned by NewIndex when the named index
	// already exists and was created with a different KeySize.
	ErrKeySizeMismatch = errors.New("hash_index_disk_ts: index already exists with a different key size")
	// ErrTooManyIndexes is returned by NewIndex when the superblock
	// registry is full: (BlockSize-32)/56 indexes per store.
	ErrTooManyIndexes = errors.New("hash_index_disk_ts: index registry is full")
	// ErrCorrupt is returned when a block read from disk has a structure
	// this package cannot interpret (a bad block type, or a directory slot
	// of 0): the file is corrupt.
	ErrCorrupt = errors.New("hash_index_disk_ts: store file is corrupt")
)

// Superblock layout (block 0) and index-registry entry layout.
const (
	sbMagic     = 0  // uint64
	sbVersion   = 8  // uint32
	sbBlockSize = 12 // uint32
	sbNumBlocks = 16 // uint64
	sbFreeHead  = 24 // uint64
	sbHeader    = 32 // registry starts here

	regEntrySize = 56 // name[32] + dirStart uint64 + length uint64 + keySize uint32 + depth uint32
	regName      = 0
	regDirStart  = 32
	regLength    = 40
	regKeySize   = 48
	regDepth     = 52
)

// StoreConfig configures OpenStore.
type StoreConfig struct {
	Path          string        // data file; the journal is Path+".journal"
	BlockSize     int           // 0 => DefaultBlockSize; creation-only, verified on reopen
	CacheBlocks   int           // 0 => DefaultCacheBlocks
	FlushInterval time.Duration // 0 => DefaultFlushInterval
}

// Store is a disk-backed block file holding one or more named hash
// indexes, with an LRU write-back cache, a background flusher and a
// write-ahead journal.  Create or open one with OpenStore.  A Store is
// safe for concurrent use; see the package documentation for the
// durability and locking contracts.
type Store struct {
	mu   sync.Mutex // guards everything below except f/jf (see package doc)
	path string

	f  *os.File // data file
	jf *os.File // journal file (Path + ".journal")

	blockSize int
	numBlocks uint64 // total blocks in the data file, block 0 included
	freeHead  uint64 // head of the free list, 0 = empty

	cache      blockCache
	dirtyCount int // cached blocks with dirty == true

	epoch uint64 // flush epoch, bumped at the start of every flush
	seq   uint64 // journal batch sequence number

	closed        bool
	flushInterval time.Duration
	flusherStop   chan struct{}
	flusherSignal chan struct{} // capacity 1; sent when the cache is half dirty
	flusherWg     sync.WaitGroup

	flushMu sync.Mutex // serializes flushes (flusher goroutine, Sync, Close)
}

// OpenStore creates the data file if it does not exist, otherwise opens
// it and runs journal recovery: every complete, CRC-valid batch in
// Path+".journal" is replayed into the data file and the journal is
// truncated; a torn or corrupt tail is ignored.  BlockSize is fixed at
// creation — reopening with a different non-zero BlockSize is an error.
// Complexity is O(journal size) on open, O(1) thereafter.
func OpenStore(cfg StoreConfig) (*Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("hash_index_disk_ts: OpenStore requires a path")
	}
	blockSize := cfg.BlockSize
	if blockSize == 0 {
		blockSize = DefaultBlockSize
	}
	if blockSize < 512 || blockSize%512 != 0 {
		return nil, fmt.Errorf("hash_index_disk_ts: block size %d is invalid: must be at least 512 and a multiple of 512", blockSize)
	}
	cacheBlocks := cfg.CacheBlocks
	if cacheBlocks == 0 {
		cacheBlocks = DefaultCacheBlocks
	}
	if cacheBlocks < 0 {
		return nil, fmt.Errorf("hash_index_disk_ts: cache blocks %d is invalid: must be positive", cacheBlocks)
	}
	flushEvery := cfg.FlushInterval
	if flushEvery <= 0 {
		flushEvery = DefaultFlushInterval
	}

	f, err := os.OpenFile(cfg.Path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("hash_index_disk_ts: open %s: %w", cfg.Path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("hash_index_disk_ts: stat %s: %w", cfg.Path, err)
	}

	var numBlocks, freeHead uint64
	if st.Size() == 0 {
		// Fresh store: write the superblock.
		img := make([]byte, blockSize)
		binary.LittleEndian.PutUint64(img[sbMagic:], storeMagic)
		binary.LittleEndian.PutUint32(img[sbVersion:], storeVersion)
		binary.LittleEndian.PutUint32(img[sbBlockSize:], uint32(blockSize))
		binary.LittleEndian.PutUint64(img[sbNumBlocks:], 1)
		if _, err := f.WriteAt(img, 0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: write superblock: %w", err)
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: fsync superblock: %w", err)
		}
		numBlocks = 1
	} else {
		if st.Size() < sbHeader {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: %s is too short to be a store (%d bytes)", cfg.Path, st.Size())
		}
		hdr := make([]byte, sbHeader)
		if _, err := f.ReadAt(hdr, 0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: read superblock: %w", err)
		}
		if binary.LittleEndian.Uint64(hdr[sbMagic:]) != storeMagic {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: %s is not a hash_index_disk_ts store (bad magic)", cfg.Path)
		}
		if v := binary.LittleEndian.Uint32(hdr[sbVersion:]); v != storeVersion {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: %s has format version %d, this build understands %d", cfg.Path, v, storeVersion)
		}
		stored := int(binary.LittleEndian.Uint32(hdr[sbBlockSize:]))
		if cfg.BlockSize != 0 && cfg.BlockSize != stored {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: %s was created with block size %d, config requests %d", cfg.Path, stored, cfg.BlockSize)
		}
		if int64(stored) > st.Size() {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: %s is truncated: shorter than one %d-byte block", cfg.Path, stored)
		}
		blockSize = stored
		sb := make([]byte, blockSize)
		if _, err := f.ReadAt(sb, 0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("hash_index_disk_ts: read superblock: %w", err)
		}
		numBlocks = binary.LittleEndian.Uint64(sb[sbNumBlocks:])
		freeHead = binary.LittleEndian.Uint64(sb[sbFreeHead:])
	}

	jf, err := os.OpenFile(cfg.Path+".journal", os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("hash_index_disk_ts: open journal: %w", err)
	}
	seq, err := recoverJournal(jf, f, blockSize)
	if err != nil {
		_ = jf.Close()
		_ = f.Close()
		return nil, err
	}

	s := &Store{
		path:          cfg.Path,
		f:             f,
		jf:            jf,
		blockSize:     blockSize,
		numBlocks:     numBlocks,
		freeHead:      freeHead,
		cache:         newBlockCache(cacheBlocks),
		flushInterval: flushEvery,
		seq:           seq,
		flusherStop:   make(chan struct{}),
		flusherSignal: make(chan struct{}, 1),
	}
	s.flusherWg.Add(1)
	go s.flusher()
	return s, nil
}

// Sync forces a flush synchronously: every change returned from before
// Sync is crash-durable once Sync returns nil.  It returns ErrClosed on
// a closed store.
// Complexity is O(dirty blocks) file writes.
func (s *Store) Sync() error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ErrClosed
	}
	return s.flush()
}

// Close stops the background flusher, runs a final flush, and closes
// the data and journal files.  Every change returned from before Close
// is crash-durable once Close returns nil.  Using the store after Close
// returns ErrClosed; Close itself returns ErrClosed when called twice.
// Complexity is O(dirty blocks) file writes.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.mu.Unlock()

	close(s.flusherStop)
	s.flusherWg.Wait()

	s.flushMu.Lock()
	err := s.flushInner()
	s.flushMu.Unlock()

	if cerr := s.f.Close(); err == nil {
		err = cerr
	}
	if cerr := s.jf.Close(); err == nil {
		err = cerr
	}
	return err
}

// Indexes returns the sorted names of the indexes registered in the
// store, or nil on a closed store.
// Complexity is O(registry capacity).
func (s *Store) Indexes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	sb, err := s.cacheGet(0)
	if err != nil {
		return nil
	}
	var names []string
	for i := 0; i < s.registryCap(); i++ {
		off := sbHeader + i*regEntrySize
		if binary.LittleEndian.Uint64(sb.data[off+regDirStart:]) == 0 {
			continue
		}
		nb := sb.data[off+regName : off+regName+32]
		for z, c := range nb {
			if c == 0 {
				nb = nb[:z]
				break
			}
		}
		names = append(names, string(nb))
	}
	sort.Strings(names)
	return names
}

// simulateCrash stops the flusher and closes the raw file handles
// WITHOUT flushing, then marks the store unusable: exactly what the
// operating system would observe after a power loss.  It exists for the
// crash-recovery tests in thorough_test.go; calling it in production
// code throws away every change since the last completed flush.
func (s *Store) simulateCrash() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	close(s.flusherStop)
	s.flusherWg.Wait()

	// A crashed process checks nothing — including its close errors.
	_ = s.f.Close()
	_ = s.jf.Close()
}

// -------------------------------------------------------------------------------------------------------
// Superblock and registry helpers.  All assume s.mu is held; the
// superblock is block 0 of the cache, journaled like any other block.

// registryCap returns how many named indexes the superblock can hold.
func (s *Store) registryCap() int {
	return (s.blockSize - sbHeader) / regEntrySize
}

// syncSuperLocked writes the in-memory numBlocks/freeHead mirrors into
// the superblock image and marks it dirty.  Caller holds s.mu.
func (s *Store) syncSuperLocked() error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(sb.data[sbNumBlocks:], s.numBlocks)
	binary.LittleEndian.PutUint64(sb.data[sbFreeHead:], s.freeHead)
	s.markDirty(sb)
	return nil
}

// findIndexLocked returns the registry slot of the named index.
// Caller holds s.mu.
func (s *Store) findIndexLocked(name string) (slot int, found bool, err error) {
	sb, err := s.cacheGet(0)
	if err != nil {
		return 0, false, err
	}
	var enc [32]byte
	copy(enc[:], name)
	for i := 0; i < s.registryCap(); i++ {
		off := sbHeader + i*regEntrySize
		if binary.LittleEndian.Uint64(sb.data[off+regDirStart:]) == 0 {
			continue // empty slot: block 0 is never a directory start
		}
		if string(sb.data[off+regName:off+regName+32]) == string(enc[:]) {
			return i, true, nil
		}
	}
	return 0, false, nil
}

// freeSlotLocked returns an unused registry slot, or -1.
// Caller holds s.mu.
func (s *Store) freeSlotLocked() (int, error) {
	sb, err := s.cacheGet(0)
	if err != nil {
		return -1, err
	}
	for i := 0; i < s.registryCap(); i++ {
		off := sbHeader + i*regEntrySize
		if binary.LittleEndian.Uint64(sb.data[off+regDirStart:]) == 0 {
			return i, nil
		}
	}
	return -1, nil
}

// readEntryMeta reads a registry entry.  Caller holds s.mu.
func (s *Store) readEntryMeta(slot int) (dirStart, length uint64, keySize, depth uint32, err error) {
	sb, err := s.cacheGet(0)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	off := sbHeader + slot*regEntrySize
	return binary.LittleEndian.Uint64(sb.data[off+regDirStart:]),
		binary.LittleEndian.Uint64(sb.data[off+regLength:]),
		binary.LittleEndian.Uint32(sb.data[off+regKeySize:]),
		binary.LittleEndian.Uint32(sb.data[off+regDepth:]), nil
}

// writeEntryLocked writes a full registry entry and marks the
// superblock dirty.  Caller holds s.mu.
func (s *Store) writeEntryLocked(slot int, name string, dirStart, length uint64, keySize, depth uint32) error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	off := sbHeader + slot*regEntrySize
	for i := 0; i < 32; i++ {
		sb.data[off+regName+i] = 0
	}
	copy(sb.data[off+regName:], name)
	binary.LittleEndian.PutUint64(sb.data[off+regDirStart:], dirStart)
	binary.LittleEndian.PutUint64(sb.data[off+regLength:], length)
	binary.LittleEndian.PutUint32(sb.data[off+regKeySize:], keySize)
	binary.LittleEndian.PutUint32(sb.data[off+regDepth:], depth)
	s.markDirty(sb)
	return nil
}

// setEntryDirLocked updates the directory start and depth of a registry
// entry (a directory doubling or relocation).  Caller holds s.mu.
func (s *Store) setEntryDirLocked(slot int, dirStart uint64, depth uint32) error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	off := sbHeader + slot*regEntrySize
	binary.LittleEndian.PutUint64(sb.data[off+regDirStart:], dirStart)
	binary.LittleEndian.PutUint32(sb.data[off+regDepth:], depth)
	s.markDirty(sb)
	return nil
}

// addEntryLengthLocked adjusts the length of a registry entry by delta
// and marks the superblock dirty.  Caller holds s.mu.
func (s *Store) addEntryLengthLocked(slot int, delta int64) error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	off := sbHeader + slot*regEntrySize
	n := binary.LittleEndian.Uint64(sb.data[off+regLength:])
	if delta >= 0 {
		n += uint64(delta)
	} else {
		n -= uint64(-delta)
	}
	binary.LittleEndian.PutUint64(sb.data[off+regLength:], n)
	s.markDirty(sb)
	return nil
}

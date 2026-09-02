/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package b_tree_disk_ts implements a disk-backed, thread-safe B+ tree
// store: one data file of fixed-size blocks, an LRU write-back block
// cache, and a write-ahead journal for crash recovery.  It is the
// disk-resident counterpart of the in-memory b_tree package — use it
// when the index must survive process restarts or is larger than RAM.
//
// Values are ALWAYS uint64 — record offsets or IDs into the caller's own
// data set.  This is an index library, not a record store: the tree maps
// fixed-size keys to 8-byte values and the caller resolves the values.
//
// # On-disk layout
//
// The data file is a random-access array of BlockSize-byte blocks (4096
// by default; chosen at creation, must be ≥ 512 and a multiple of 512,
// and is verified on reopen).  All integers on disk are little-endian.
//
//	Block 0      — the superblock: magic, format version, block size,
//	               numBlocks, freeListHead, and a fixed-capacity
//	               registry of named trees (each entry: a 32-byte name,
//	               rootBlock, length, keySize).  The registry holds
//	               (BlockSize-32)/56 trees: 8 at 512-byte blocks, 72 at
//	               4 KiB blocks.
//	Free blocks  — linked from freeListHead; the first 8 bytes of a
//	               free block hold the next free block number, 0 = end.
//	Leaf blocks  — type byte, nkeys, nextLeaf link, then n sorted
//	               (key, value) entries.
//	Internal     — type byte, nkeys, then n sorted separator keys and
//	               n+1 child block numbers.
//
// Block number 0 is the superblock, so 0 doubles as the nil sentinel
// for block pointers (free-list end, empty nextLeaf).  Superblock
// mutations go through the cache and are journaled like any block.
//
// A store may hold many named trees (NewTree), so one file can carry
// several indexes over the same record set — e.g. by-ID and by-name.
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
// so there is no read-only path.  All public Tree and Store methods
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
//	NewTree with a nil EncodeKey, DecodeKey or Compare function.
//	NewTree with KeySize ≤ 0, an empty Name, or a Name over 31 bytes.
//	NewTree on a nil *Store.
//
// Everything else — IO failures, a corrupt file, a block-size mismatch,
// a too-large key — is returned as an error.  Using the store after
// Close returns ErrClosed (check with errors.Is).
package b_tree_disk_ts

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// File-format constants.  The store magic reads "pluto-bt".
const (
	storeMagic   uint64 = 0x706c75746f2d6274
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

	// maxTreeNameLen is the longest tree name the registry accepts; names
	// are stored NUL-padded in a 32-byte field.
	maxTreeNameLen = 31
)

// Sentinel errors returned by this package; check them with errors.Is.
var (
	// ErrClosed is returned by every Store and Tree operation on a closed
	// store.  (Length, which has no error return, yields 0; Trees yields
	// nil; All and Range yield nothing.)
	ErrClosed = errors.New("b_tree_disk_ts: store is closed")
	// ErrKeyTooLarge is returned by NewTree when the key size leaves room
	// for fewer than 4 entries per block; use bigger blocks or a smaller
	// key encoding.
	ErrKeyTooLarge = errors.New("b_tree_disk_ts: key size too large for the block size (a block must hold at least 4 entries)")
	// ErrKeySizeMismatch is returned by NewTree when the named tree
	// already exists and was created with a different KeySize.
	ErrKeySizeMismatch = errors.New("b_tree_disk_ts: tree already exists with a different key size")
	// ErrTooManyTrees is returned by NewTree when the superblock registry
	// is full: (BlockSize-32)/56 trees per store.
	ErrTooManyTrees = errors.New("b_tree_disk_ts: tree registry is full")
)

// Superblock layout (block 0) and tree-registry entry layout.
const (
	sbMagic     = 0  // uint64
	sbVersion   = 8  // uint32
	sbBlockSize = 12 // uint32
	sbNumBlocks = 16 // uint64
	sbFreeHead  = 24 // uint64
	sbHeader    = 32 // registry starts here

	regEntrySize = 56 // name[32] + root uint64 + length uint64 + keySize uint32 + pad uint32
	regName      = 0
	regRoot      = 32
	regLength    = 40
	regKeySize   = 48
)

// StoreConfig configures OpenStore.
type StoreConfig struct {
	Path          string        // data file; the journal is Path+".journal"
	BlockSize     int           // 0 => DefaultBlockSize; creation-only, verified on reopen
	CacheBlocks   int           // 0 => DefaultCacheBlocks
	FlushInterval time.Duration // 0 => DefaultFlushInterval
}

// Store is a disk-backed block file holding one or more named B+ trees,
// with an LRU write-back cache, a background flusher and a write-ahead
// journal.  Create or open one with OpenStore.  A Store is safe for
// concurrent use; see the package documentation for the durability and
// locking contracts.
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
		return nil, errors.New("b_tree_disk_ts: OpenStore requires a path")
	}
	blockSize := cfg.BlockSize
	if blockSize == 0 {
		blockSize = DefaultBlockSize
	}
	if blockSize < 512 || blockSize%512 != 0 {
		return nil, fmt.Errorf("b_tree_disk_ts: block size %d is invalid: must be at least 512 and a multiple of 512", blockSize)
	}
	cacheBlocks := cfg.CacheBlocks
	if cacheBlocks == 0 {
		cacheBlocks = DefaultCacheBlocks
	}
	if cacheBlocks < 0 {
		return nil, fmt.Errorf("b_tree_disk_ts: cache blocks %d is invalid: must be positive", cacheBlocks)
	}
	flushEvery := cfg.FlushInterval
	if flushEvery <= 0 {
		flushEvery = DefaultFlushInterval
	}

	f, err := os.OpenFile(cfg.Path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, fmt.Errorf("b_tree_disk_ts: open %s: %w", cfg.Path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("b_tree_disk_ts: stat %s: %w", cfg.Path, err)
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
			return nil, fmt.Errorf("b_tree_disk_ts: write superblock: %w", err)
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: fsync superblock: %w", err)
		}
		numBlocks = 1
	} else {
		if st.Size() < sbHeader {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: %s is too short to be a store (%d bytes)", cfg.Path, st.Size())
		}
		hdr := make([]byte, sbHeader)
		if _, err := f.ReadAt(hdr, 0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: read superblock: %w", err)
		}
		if binary.LittleEndian.Uint64(hdr[sbMagic:]) != storeMagic {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: %s is not a b_tree_disk_ts store (bad magic)", cfg.Path)
		}
		if v := binary.LittleEndian.Uint32(hdr[sbVersion:]); v != storeVersion {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: %s has format version %d, this build understands %d", cfg.Path, v, storeVersion)
		}
		stored := int(binary.LittleEndian.Uint32(hdr[sbBlockSize:]))
		if cfg.BlockSize != 0 && cfg.BlockSize != stored {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: %s was created with block size %d, config requests %d", cfg.Path, stored, cfg.BlockSize)
		}
		if int64(stored) > st.Size() {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: %s is truncated: shorter than one %d-byte block", cfg.Path, stored)
		}
		blockSize = stored
		sb := make([]byte, blockSize)
		if _, err := f.ReadAt(sb, 0); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("b_tree_disk_ts: read superblock: %w", err)
		}
		numBlocks = binary.LittleEndian.Uint64(sb[sbNumBlocks:])
		freeHead = binary.LittleEndian.Uint64(sb[sbFreeHead:])
	}

	jf, err := os.OpenFile(cfg.Path+".journal", os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("b_tree_disk_ts: open journal: %w", err)
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

// Trees returns the sorted names of the trees registered in the store,
// or nil on a closed store.
// Complexity is O(registry capacity).
func (s *Store) Trees() []string {
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
		if binary.LittleEndian.Uint64(sb.data[off+regRoot:]) == 0 {
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

// registryCap returns how many named trees the superblock can hold.
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

// findTreeLocked returns the registry slot of the named tree.
// Caller holds s.mu.
func (s *Store) findTreeLocked(name string) (slot int, found bool, err error) {
	sb, err := s.cacheGet(0)
	if err != nil {
		return 0, false, err
	}
	var enc [32]byte
	copy(enc[:], name)
	for i := 0; i < s.registryCap(); i++ {
		off := sbHeader + i*regEntrySize
		if binary.LittleEndian.Uint64(sb.data[off+regRoot:]) == 0 {
			continue // empty slot: block 0 is never a tree root
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
		if binary.LittleEndian.Uint64(sb.data[off+regRoot:]) == 0 {
			return i, nil
		}
	}
	return -1, nil
}

// readEntryLocked reads a registry entry.  Caller holds s.mu.
func (s *Store) readEntryLocked(slot int) (root, length uint64, keySize uint32, err error) {
	sb, err := s.cacheGet(0)
	if err != nil {
		return 0, 0, 0, err
	}
	off := sbHeader + slot*regEntrySize
	return binary.LittleEndian.Uint64(sb.data[off+regRoot:]),
		binary.LittleEndian.Uint64(sb.data[off+regLength:]),
		binary.LittleEndian.Uint32(sb.data[off+regKeySize:]), nil
}

// writeEntryLocked writes a full registry entry and marks the
// superblock dirty.  Caller holds s.mu.
func (s *Store) writeEntryLocked(slot int, name string, root, length uint64, keySize uint32) error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	off := sbHeader + slot*regEntrySize
	for i := 0; i < 32; i++ {
		sb.data[off+regName+i] = 0
	}
	copy(sb.data[off+regName:], name)
	binary.LittleEndian.PutUint64(sb.data[off+regRoot:], root)
	binary.LittleEndian.PutUint64(sb.data[off+regLength:], length)
	binary.LittleEndian.PutUint32(sb.data[off+regKeySize:], keySize)
	s.markDirty(sb)
	return nil
}

// setEntryRootLocked updates the root block of a registry entry (root
// split or root collapse).  Caller holds s.mu.
func (s *Store) setEntryRootLocked(slot int, root uint64) error {
	sb, err := s.cacheGet(0)
	if err != nil {
		return err
	}
	off := sbHeader + slot*regEntrySize
	binary.LittleEndian.PutUint64(sb.data[off+regRoot:], root)
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

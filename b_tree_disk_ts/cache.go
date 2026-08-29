/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The block cache, background flusher, write-ahead journal and journal
// recovery.
//
// Cache: an LRU over whole block images — built on pluto's lru package,
// with an eviction veto — with a per-block dirty flag, the flush epoch
// at which it was last dirtied, a flushing flag for blocks in an
// in-flight flush batch, and a pin count for blocks an in-flight
// operation still references.  Only clean, unflushing, unpinned blocks
// are evictable; when nothing is evictable the cache temporarily
// exceeds its capacity (lru's soft cap — see the package doc) rather
// than evicting a dirty block unsafely or deadlocking on a synchronous
// flush with the store lock held.
//
// Flusher: every FlushInterval, and whenever the dirty half of the
// cache is signalled, one flush runs.  A flush bumps the global epoch,
// snapshots every dirty block under the store lock, then — with the
// lock RELEASED — appends one journal batch, fsyncs the journal, writes
// the data blocks and fsyncs the data file.  Back under the lock it
// clears the dirty flag of every flushed block whose dirty epoch is
// still at or before the flush's epoch (a later epoch means the block
// was re-dirtied while the flush was in flight and must stay dirty),
// and finally truncates the journal: every batch in it is now durable
// in the data file.
//
// Journal: an append-only sequence of batches —
//
//	magic uint32 ("JRNL") | seq uint64 | count uint32 |
//	count × (blockNo uint64 + full block image) |
//	crc32 (Castagnoli) over seq..payload | commit uint32 ("COMM")
//
// The CRC is computed with pluto's crc package (crc.CastagnoliTable —
// the same polynomial hash/crc32 uses, so the journal format is
// unchanged).  Recovery scans batches in order, applies each valid
// committed batch's images to the data file, and stops at the first
// bad magic, bad CRC, bad commit mark or truncated tail (a torn
// write), then truncates the journal.

package b_tree_disk_ts

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pschlump/pluto/crc"
	"github.com/pschlump/pluto/lru"
)

const (
	journalMagic  uint32 = 0x4a524e4c // "JRNL"
	journalCommit uint32 = 0x434f4d4d // "COMM"
)

// cacheBlock is one cached block image.
type cacheBlock struct {
	no       uint64
	data     []byte
	dirty    bool
	epoch    uint64 // store epoch when the block was last dirtied
	flushing bool   // part of an in-flight flush batch
	pins     int    // >0 while an in-flight operation references the block
}

// blockCache is an LRU of block images; guarded by Store.mu.  The veto
// keeps dirty, flushing and pinned blocks from being evicted (lru's
// soft cap lets the cache grow past capacity when nothing is
// evictable).
type blockCache struct {
	capacity int
	lru      *lru.Lru[uint64, *cacheBlock]
}

func newBlockCache(capacity int) blockCache {
	return blockCache{
		capacity: capacity,
		lru: lru.NewLruFunc(capacity, func(_ uint64, b *cacheBlock) bool {
			return !b.dirty && !b.flushing && b.pins == 0
		}),
	}
}

// cacheGet returns the block's cache entry, loading it from disk if
// necessary.  Caller holds s.mu.  The returned entry is only valid
// while pinned or dirty — a clean unpinned block may be evicted by the
// next cacheGet.
func (s *Store) cacheGet(no uint64) (*cacheBlock, error) {
	if b, ok := s.cache.lru.Get(no); ok { // a hit promotes the block to most-recently-used
		return b, nil
	}
	if no >= s.numBlocks {
		return nil, fmt.Errorf("b_tree_disk_ts: block %d out of range (store has %d blocks): the file is corrupt", no, s.numBlocks)
	}
	data := make([]byte, s.blockSize)
	if _, err := s.f.ReadAt(data, int64(no)*int64(s.blockSize)); err != nil {
		return nil, fmt.Errorf("b_tree_disk_ts: read block %d: %w", no, err)
	}
	return s.cacheInsert(no, data), nil
}

// cacheInsert adds a block image to the cache; Put evicts down to
// capacity first, skipping vetoed (dirty, flushing, pinned) blocks.
// When nothing is evictable the cache grows past its capacity — the
// soft cap; the flusher shrinks the pinned set again.  Caller holds
// s.mu.
func (s *Store) cacheInsert(no uint64, data []byte) *cacheBlock {
	b := &cacheBlock{no: no, data: data}
	s.cache.lru.Put(no, b)
	return b
}

// markDirty flags a block for the next flush and nudges the flusher
// once more than half the cache is dirty.  Caller holds s.mu.
func (s *Store) markDirty(b *cacheBlock) {
	if !b.dirty {
		b.dirty = true
		s.dirtyCount++
	}
	b.epoch = s.epoch
	if s.dirtyCount*2 > s.cache.capacity {
		select {
		case s.flusherSignal <- struct{}{}:
		default:
		}
	}
}

// allocBlock returns a zeroed block: popped from the free list when one
// is available, otherwise appended to the file.  The caller initializes
// the block (leafInit / internalInit) and marks it dirty.
// Caller holds s.mu.
func (s *Store) allocBlock() (uint64, *cacheBlock, error) {
	var no uint64
	var b *cacheBlock
	if s.freeHead != 0 {
		no = s.freeHead
		var err error
		b, err = s.cacheGet(no)
		if err != nil {
			return 0, nil, err
		}
		s.freeHead = binary.LittleEndian.Uint64(b.data[0:8])
	} else {
		no = s.numBlocks
		s.numBlocks++
		b = s.cacheInsert(no, make([]byte, s.blockSize))
	}
	clear(b.data)
	s.markDirty(b) // the caller is about to rewrite the block; dirty it now
	// so syncSuperLocked cannot evict it from under the caller
	if err := s.syncSuperLocked(); err != nil {
		return 0, nil, err
	}
	return no, b, nil
}

// freeBlock pushes a block onto the free list.  The block stays in the
// cache (dirty) until evicted after a flush.  Caller holds s.mu.
func (s *Store) freeBlock(no uint64) error {
	b, err := s.cacheGet(no)
	if err != nil {
		return err
	}
	clear(b.data)
	binary.LittleEndian.PutUint64(b.data[0:8], s.freeHead)
	s.freeHead = no
	s.markDirty(b)
	return s.syncSuperLocked()
}

// -------------------------------------------------------------------------------------------------------
// Flusher.

// flusher is the background goroutine: it flushes on every tick of
// FlushInterval and whenever the cache signals it is half dirty.
func (s *Store) flusher() {
	defer s.flusherWg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.flusherStop:
			return
		case <-ticker.C:
			s.flush()
		case <-s.flusherSignal:
			s.flush()
		}
	}
}

// flush runs one flush cycle, serialized against Sync, Close and the
// flusher goroutine by flushMu.  It returns ErrClosed on a closed
// store.  IO errors leave every block dirty so the next flush retries.
func (s *Store) flush() error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ErrClosed
	}
	return s.flushInner()
}

// flushInner performs one flush.  It never holds s.mu during file IO,
// so tree operations proceed against the cache while a flush is in
// flight; the dirty epochs sort out which blocks changed meanwhile.
// Caller must hold flushMu (or have stopped the flusher, as Close has).
func (s *Store) flushInner() error {
	// Phase 1: under the store lock, bump the epoch and snapshot every
	// dirty block.
	s.mu.Lock()
	flushEpoch := s.epoch
	s.epoch++
	var blocks []*cacheBlock
	var images []blockImage
	for _, b := range s.cache.lru.All() {
		if b.dirty && !b.flushing {
			b.flushing = true
			cp := make([]byte, len(b.data))
			copy(cp, b.data)
			blocks = append(blocks, b)
			images = append(images, blockImage{no: b.no, data: cp})
		}
	}
	seq := s.seq
	s.seq++
	s.mu.Unlock()

	if len(images) == 0 {
		return nil
	}

	// Phase 2: journal, fsync, data blocks, fsync — no lock held.
	err := appendJournalBatch(s.jf, seq, images)
	if err == nil {
		err = s.jf.Sync()
	}
	if err == nil {
		for _, im := range images {
			if _, werr := s.f.WriteAt(im.data, int64(im.no)*int64(s.blockSize)); werr != nil {
				err = fmt.Errorf("b_tree_disk_ts: write block %d: %w", im.no, werr)
				break
			}
		}
	}
	if err == nil {
		err = s.f.Sync()
	}

	// Phase 3: bookkeeping, then checkpoint the journal — every batch in
	// it is durable in the data file now.
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range blocks {
		b.flushing = false
		if err == nil && b.dirty && b.epoch <= flushEpoch {
			b.dirty = false
			s.dirtyCount--
		}
	}
	if err == nil {
		s.jf.Truncate(0)
		s.jf.Seek(0, 0)
	}
	return err
}

// -------------------------------------------------------------------------------------------------------
// Journal writer and recovery.

// blockImage is one snapshotted block in a flush batch.
type blockImage struct {
	no   uint64
	data []byte
}

// appendJournalBatch appends one committed batch to the journal.
// The caller fsyncs.
func appendJournalBatch(jf *os.File, seq uint64, images []blockImage) error {
	var tmp [8]byte
	buf := make([]byte, 0, 16+len(images)*(8+len(images[0].data))+8)
	binary.LittleEndian.PutUint32(tmp[:4], journalMagic)
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint64(tmp[:8], seq)
	buf = append(buf, tmp[:8]...)
	binary.LittleEndian.PutUint32(tmp[:4], uint32(len(images)))
	buf = append(buf, tmp[:4]...)
	for _, im := range images {
		binary.LittleEndian.PutUint64(tmp[:8], im.no)
		buf = append(buf, tmp[:8]...)
		buf = append(buf, im.data...)
	}
	binary.LittleEndian.PutUint32(tmp[:4], crc.Checksum32(buf[4:], crc.CastagnoliTable))
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint32(tmp[:4], journalCommit)
	buf = append(buf, tmp[:4]...)
	if _, err := jf.Write(buf); err != nil {
		return fmt.Errorf("b_tree_disk_ts: journal append: %w", err)
	}
	return nil
}

// recoverJournal replays every valid committed batch into the data
// file, stops silently at the first corrupt or torn batch, fsyncs the
// data file if anything was applied, and truncates the journal.  It
// returns the sequence number of the last batch applied (the store
// continues numbering from there).
func recoverJournal(jf, f *os.File, blockSize int) (uint64, error) {
	data, err := io.ReadAll(jf)
	if err != nil {
		return 0, fmt.Errorf("b_tree_disk_ts: read journal: %w", err)
	}
	var lastSeq uint64
	off := 0
	entrySize := 8 + blockSize
	for off+16 <= len(data) {
		if binary.LittleEndian.Uint32(data[off:]) != journalMagic {
			break
		}
		seq := binary.LittleEndian.Uint64(data[off+4:])
		count := int(binary.LittleEndian.Uint32(data[off+12:]))
		if count > (len(data)-off-16-8)/entrySize {
			break // truncated payload: torn write
		}
		payloadEnd := off + 16 + count*entrySize
		crcWant := binary.LittleEndian.Uint32(data[payloadEnd:])
		if crc.Checksum32(data[off+4:payloadEnd], crc.CastagnoliTable) != crcWant {
			break // corrupt batch
		}
		if binary.LittleEndian.Uint32(data[payloadEnd+4:]) != journalCommit {
			break // uncommitted batch
		}
		p := off + 16
		for i := 0; i < count; i++ {
			no := binary.LittleEndian.Uint64(data[p:])
			if _, err := f.WriteAt(data[p+8:p+8+blockSize], int64(no)*int64(blockSize)); err != nil {
				return lastSeq, fmt.Errorf("b_tree_disk_ts: journal replay block %d: %w", no, err)
			}
			p += entrySize
		}
		lastSeq = seq
		off = payloadEnd + 8
	}
	if off > 0 {
		if err := f.Sync(); err != nil {
			return lastSeq, fmt.Errorf("b_tree_disk_ts: fsync after journal replay: %w", err)
		}
	}
	if len(data) > 0 {
		if err := jf.Truncate(0); err != nil {
			return lastSeq, fmt.Errorf("b_tree_disk_ts: truncate journal: %w", err)
		}
		if _, err := jf.Seek(0, 0); err != nil {
			return lastSeq, fmt.Errorf("b_tree_disk_ts: rewind journal: %w", err)
		}
	}
	return lastSeq, nil
}

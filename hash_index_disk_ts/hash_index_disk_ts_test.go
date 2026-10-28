package hash_index_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Unit tests: entries-per-block math, single insert/search/delete, the
// split/double mechanics of the extendible hash, free-list reuse,
// degenerate overflow chains (all-identical hashes forced through the
// test-only hash override), the non-injective-encoding collapse,
// multi-index stores, close/reopen persistence, Sync durability,
// ErrClosed behavior, the panic contract and the corruption guards.
// Tests run in this (internal) package so they can inspect the store,
// matching the house pattern of b_tree_disk_ts's test suite.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -------------------------------------------------------------------------------------------------------
// Test key encodings: uint64 keys as 8-byte big-endian.  There is no
// Compare — big-endian keeps the encoding injective, which is the only
// key contract.

var u64IndexConfig = IndexConfig[uint64]{
	Name:    "by-id",
	KeySize: 8,
	EncodeKey: func(k uint64, buf []byte) {
		binary.BigEndian.PutUint64(buf, k)
	},
	DecodeKey: func(buf []byte) uint64 {
		return binary.BigEndian.Uint64(buf)
	},
}

// openTempStore opens a store in t's temp dir and registers cleanup.
func openTempStore(t *testing.T, cfg StoreConfig) *Store {
	t.Helper()
	if cfg.Path == "" {
		cfg.Path = filepath.Join(t.TempDir(), "test.db")
	}
	s, err := OpenStore(cfg)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newU64Index(t *testing.T, s *Store) *Index[uint64] {
	t.Helper()
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	return ix
}

// expectPanic runs fx and fails unless it panics; the message must
// contain want.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected %s to panic, it did not", name)
			return
		}
		if !strings.Contains(fmt.Sprint(r), want) {
			t.Errorf("%s panicked with %q, want substring %q", name, r, want)
		}
	}()
	fx()
}

// indexDepth returns the global directory depth from the registry,
// under the lock.
func indexDepth(t *testing.T, ix *Index[uint64]) uint32 {
	t.Helper()
	s := ix.store
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, _, depth, err := s.readEntryMeta(ix.slot)
	if err != nil {
		t.Fatalf("readEntryMeta: %v", err)
	}
	return depth
}

// numBlocks returns the store's block count under the lock.
func numBlocks(s *Store) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numBlocks
}

func TestEntriesPerBlockMath(t *testing.T) {
	cases := []struct {
		blockSize, keySize int
		wantEntries        int
	}{
		{4096, 8, 254},
		{4096, 16, 169},
		{512, 8, 30},
		{512, 16, 20},
	}
	for _, c := range cases {
		if got := maxEntriesPerBucket(c.blockSize, c.keySize); got != c.wantEntries {
			t.Errorf("maxEntriesPerBucket(%d, %d) = %d, want %d", c.blockSize, c.keySize, got, c.wantEntries)
		}
	}
	// (BlockSize-8)/8 bucket pointers per directory block: 511 at 4 KiB,
	// 63 at 512.
	if got := slotsPerDirBlock(4096); got != 511 {
		t.Errorf("slotsPerDirBlock(4096) = %d, want 511", got)
	}
	if got := slotsPerDirBlock(512); got != 63 {
		t.Errorf("slotsPerDirBlock(512) = %d, want 63", got)
	}
	// Directory block counts: depth 0-5 fit in one 512-byte block (63
	// slots), depth 6 needs two — the relocation threshold.
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	for _, c := range []struct {
		depth uint32
		want  uint64
	}{
		{0, 1}, {5, 1}, {6, 2}, {12, 66},
	} {
		if got := s.dirBlockCount(c.depth); got != c.want {
			t.Errorf("dirBlockCount(%d) = %d, want %d", c.depth, got, c.want)
		}
	}
}

func TestErrKeyTooLarge(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	cfg := u64IndexConfig
	cfg.KeySize = 500 // (512-17)/(500+8) = 0 entries per bucket
	_, err := NewIndex[uint64](s, cfg)
	if !errors.Is(err, ErrKeyTooLarge) {
		t.Fatalf("NewIndex with a 500-byte key: got %v, want ErrKeyTooLarge", err)
	}
}

func TestInsertSearchDeleteSingle(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	ix := newU64Index(t, s)

	if ix.Length() != 0 {
		t.Fatalf("empty index has length %d", ix.Length())
	}
	if _, found, err := ix.Search(7); err != nil || found {
		t.Fatalf("Search on empty index = (%v, %v), want (false, nil)", found, err)
	}
	if ok, err := ix.Delete(7); err != nil || ok {
		t.Fatalf("Delete on empty index = (%v, %v), want (false, nil)", ok, err)
	}

	added, err := ix.Insert(42, 1000)
	if err != nil || !added {
		t.Fatalf("Insert = (%v, %v), want (true, nil)", added, err)
	}
	if ix.Length() != 1 {
		t.Fatalf("Length = %d, want 1", ix.Length())
	}
	v, found, err := ix.Search(42)
	if err != nil || !found || v != 1000 {
		t.Fatalf("Search = (%d, %v, %v), want (1000, true, nil)", v, found, err)
	}

	// Duplicate replaces the value and reports false.
	added, err = ix.Insert(42, 2000)
	if err != nil || added {
		t.Fatalf("duplicate Insert = (%v, %v), want (false, nil)", added, err)
	}
	if v, _, _ := ix.Search(42); v != 2000 {
		t.Fatalf("after replace Search = %d, want 2000", v)
	}
	if ix.Length() != 1 {
		t.Fatalf("after replace Length = %d, want 1", ix.Length())
	}

	ok, err := ix.Delete(42)
	if err != nil || !ok {
		t.Fatalf("Delete = (%v, %v), want (true, nil)", ok, err)
	}
	if _, found, _ := ix.Search(42); found {
		t.Fatal("Search finds deleted key")
	}
	if ix.Length() != 0 {
		t.Fatalf("after delete Length = %d, want 0", ix.Length())
	}
}

// TestSplitsAndDoubling drives a 512-byte-block store through enough
// inserts to split buckets, double the directory in place (depths that
// stay within one directory block) and double it past one block — the
// RELOCATION path where the directory moves to a fresh contiguous run
// at the end of the file.
func TestSplitsAndDoubling(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512, CacheBlocks: 64})
	ix := newU64Index(t, s)

	const n = 5000
	for i := 0; i < n; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)*7); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	depth := indexDepth(t, ix)
	// ~5000 keys over ~30-entry buckets needs ≥ 167 buckets ≥ 2^8
	// slots; depth 8 needs ceil(256/63) = 5 directory blocks, so the
	// relocation path ran.
	if depth < 8 {
		t.Fatalf("depth = %d after %d inserts, want ≥ 8 (the directory never doubled past one block)", depth, n)
	}
	if got := s.dirBlockCount(depth); got < 2 {
		t.Fatalf("dirBlockCount(%d) = %d, want ≥ 2 (relocation never exercised)", depth, got)
	}
	if ix.Length() != n {
		t.Fatalf("Length = %d, want %d", ix.Length(), n)
	}
	for i := 0; i < n; i++ {
		v, found, err := ix.Search(uint64(i))
		if err != nil || !found || v != uint64(i)*7 {
			t.Fatalf("Search(%d) = (%d, %v, %v)", i, v, found, err)
		}
	}
	checkInvariants(t, ix)

	// Deleting everything leaves the high-water directory (no merges,
	// no shrinking) but a fully usable index.
	for i := 0; i < n; i++ {
		if _, err := ix.Delete(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
	}
	if ix.Length() != 0 {
		t.Fatalf("Length after delete-all = %d, want 0", ix.Length())
	}
	if d := indexDepth(t, ix); d != depth {
		t.Fatalf("depth changed from %d to %d on deletes — the directory is a high-water mark", depth, d)
	}
	checkInvariants(t, ix)
}

// TestFreeListReuse verifies emptied bucket blocks recycle: after
// delete-all and a re-insert of the same size, the file does not grow
// (primaries stay allocated by the high-water policy and overflow
// blocks come from the free list).
func TestFreeListReuse(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512, CacheBlocks: 64})
	ix := newU64Index(t, s)

	const n = 800
	fill := func() {
		for i := 0; i < n; i++ {
			if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
				t.Fatalf("Insert %d: %v", i, err)
			}
		}
	}
	initial := numBlocks(s) // superblock + directory + first bucket
	fill()
	first := numBlocks(s)
	if first <= initial {
		t.Fatalf("fill allocated no blocks (%d → %d)", initial, first)
	}
	for i := 0; i < n; i++ {
		if _, err := ix.Delete(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
	}
	fill()
	second := numBlocks(s)
	if second > first {
		t.Fatalf("refill grew the file: %d blocks after the first fill, %d after the second", first, second)
	}
	checkInvariants(t, ix)
}

// TestDegenerateChains forces the all-identical-hashes case through
// the test-only hash override: buckets cannot split, so the overflow
// chain grows; searches walk it; deletes unlink and free emptied chain
// blocks.  The store is never reopened (its layout is only consistent
// under the overridden hash).
func TestDegenerateChains(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512, CacheBlocks: 64})
	ix := newU64Index(t, s)

	// Every key hashes the same: slot 0's bucket can never split.
	ix.store.mu.Lock()
	ix.hash = func([]byte) uint64 { return 0x12345 }
	ix.store.mu.Unlock()

	maxEnt := maxEntriesPerBucket(512, 8)
	const extra = 5
	n := 3*maxEnt + extra
	for i := 0; i < n; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if ix.Length() != uint64(n) {
		t.Fatalf("Length = %d, want %d", ix.Length(), n)
	}
	// superblock + directory + primary + 3 overflow blocks.
	if blocks := numBlocks(s); blocks != 6 {
		t.Fatalf("block count = %d, want 6 (superblock + directory + primary + 3 overflow blocks)", blocks)
	}
	for i := 0; i < n; i++ {
		v, found, err := ix.Search(uint64(i))
		if err != nil || !found || v != uint64(i) {
			t.Fatalf("Search(%d) = (%d, %v, %v) through the chain", i, v, found, err)
		}
	}
	checkInvariants(t, ix)

	// Empty the first overflow block (entries maxEnt..2*maxEnt-1 were
	// appended there): it must be unlinked and freed — the file does not
	// shrink (a freed block joins the in-file free list), so the check
	// is that the free list grew by exactly that block.
	s.mu.Lock()
	freeBefore := s.freeHead
	s.mu.Unlock()
	for i := maxEnt; i < 2*maxEnt; i++ {
		if _, err := ix.Delete(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
	}
	s.mu.Lock()
	freed := s.freeHead
	s.mu.Unlock()
	if freed == 0 || freed == freeBefore {
		t.Fatalf("emptying an overflow block freed nothing: free list head %d → %d", freeBefore, freed)
	}
	if blocks := numBlocks(s); blocks != 6 {
		t.Fatalf("block count = %d after the free, want 6 (a freed block stays in the file)", blocks)
	}
	for i := 0; i < n; i++ {
		_, found, err := ix.Search(uint64(i))
		want := i < maxEnt || i >= 2*maxEnt
		if err != nil || found != want {
			t.Fatalf("Search(%d) after unlink = (%v, %v), want (%v, nil)", i, found, err, want)
		}
	}
	checkInvariants(t, ix)
}

// TestNonInjectiveEncoding documents the collapse: two distinct K with
// the same encoding ARE the same key to the index.
func TestNonInjectiveEncoding(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	cfg := IndexConfig[uint64]{
		Name:    "sloppy",
		KeySize: 8,
		EncodeKey: func(k uint64, buf []byte) {
			binary.BigEndian.PutUint32(buf[4:], uint32(k)) // top 4 bytes left zero
		},
		DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
	}
	ix, err := NewIndex[uint64](s, cfg)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	if added, err := ix.Insert(1, 10); err != nil || !added {
		t.Fatalf("Insert(1) = (%v, %v)", added, err)
	}
	// 1<<32 + 1 encodes identically to 1: a replace, not a second entry.
	if added, err := ix.Insert(1<<32+1, 99); err != nil || added {
		t.Fatalf("Insert(colliding encoding) = (%v, %v), want (false, nil)", added, err)
	}
	if ix.Length() != 1 {
		t.Fatalf("Length = %d, want 1", ix.Length())
	}
}

func TestMultiIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.db")
	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	byID := newU64Index(t, s) // "by-id"
	byTwice := u64IndexConfig
	byTwice.Name = "by-twice"
	other, err := NewIndex[uint64](s, byTwice)
	if err != nil {
		t.Fatalf("NewIndex by-twice: %v", err)
	}

	for i := 0; i < 500; i++ {
		if _, err := byID.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("byID.Insert %d: %v", i, err)
		}
		if _, err := other.Insert(uint64(i), uint64(i)*2); err != nil {
			t.Fatalf("other.Insert %d: %v", i, err)
		}
	}
	names := s.Indexes()
	if fmt.Sprint(names) != "[by-id by-twice]" {
		t.Fatalf("Indexes() = %v, want [by-id by-twice]", names)
	}
	v, found, err := other.Search(42)
	if err != nil || !found || v != 84 {
		t.Fatalf("other.Search(42) = (%d, %v, %v)", v, found, err)
	}
	if v, _, _ := byID.Search(42); v != 42 {
		t.Fatalf("byID.Search(42) = %d, indexes share buckets", v)
	}
	checkInvariants(t, byID)
	checkInvariants(t, other)

	// Both survive a reopen as independent indexes.
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	id2 := newU64Index(t, s2)
	twice2, err := NewIndex[uint64](s2, byTwice)
	if err != nil {
		t.Fatalf("re-attach by-twice: %v", err)
	}
	if v, found, _ := id2.Search(7); !found || v != 7 {
		t.Fatalf("id2.Search(7) = (%d, %v)", v, found)
	}
	if v, found, _ := twice2.Search(7); !found || v != 14 {
		t.Fatalf("twice2.Search(7) = (%d, %v)", v, found)
	}
}

func TestCloseReopenPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	s, err := OpenStore(StoreConfig{Path: path, BlockSize: 512})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	const n = 2000
	for i := 0; i < n; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)*3); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	// Delete every third key so reopen sees both presence and absence.
	deleted := 0
	for i := 0; i < n; i += 3 {
		if _, err := ix.Delete(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
		deleted++
	}
	depth := indexDepth(t, ix)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := OpenStore(StoreConfig{Path: path, BlockSize: 512})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if ix2.Length() != uint64(n-deleted) {
		t.Fatalf("Length after reopen = %d, want %d", ix2.Length(), n-deleted)
	}
	if d := indexDepth(t, ix2); d != depth {
		t.Fatalf("depth after reopen = %d, want %d (the registry carries the depth)", d, depth)
	}
	for i := 0; i < n; i++ {
		v, found, err := ix2.Search(uint64(i))
		want := i%3 != 0
		if err != nil || found != want || (found && v != uint64(i)*3) {
			t.Fatalf("Search(%d) = (%d, %v, %v), want found=%v", i, v, found, err, want)
		}
	}
	checkInvariants(t, ix2)
}

func TestSyncDurability(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.db")

	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	const n = 3000
	for i := 0; i < n; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// A crash after Sync must lose nothing.
	s.simulateCrash()

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if ix2.Length() != n {
		t.Fatalf("Length after crash = %d, want %d (Sync must be durable)", ix2.Length(), n)
	}
	for i := 0; i < n; i++ {
		if _, found, _ := ix2.Search(uint64(i)); !found {
			t.Fatalf("key %d missing after Sync + crash", i)
		}
	}
}

func TestErrClosed(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	ix := newU64Index(t, s)
	if _, err := ix.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := ix.Insert(2, 2); !errors.Is(err, ErrClosed) {
		t.Errorf("Insert after Close = %v, want ErrClosed", err)
	}
	if _, _, err := ix.Search(1); !errors.Is(err, ErrClosed) {
		t.Errorf("Search after Close = %v, want ErrClosed", err)
	}
	if _, err := ix.Delete(1); !errors.Is(err, ErrClosed) {
		t.Errorf("Delete after Close = %v, want ErrClosed", err)
	}
	if err := s.Sync(); !errors.Is(err, ErrClosed) {
		t.Errorf("Sync after Close = %v, want ErrClosed", err)
	}
	if _, err := NewIndex[uint64](s, u64IndexConfig); !errors.Is(err, ErrClosed) {
		t.Errorf("NewIndex after Close = %v, want ErrClosed", err)
	}
	if err := s.Close(); !errors.Is(err, ErrClosed) {
		t.Errorf("second Close = %v, want ErrClosed", err)
	}
	if n := ix.Length(); n != 0 {
		t.Errorf("Length after Close = %d, want 0", n)
	}
	if names := s.Indexes(); names != nil {
		t.Errorf("Indexes after Close = %v, want nil", names)
	}
	for range ix.All() {
		t.Error("All after Close yields a pair")
	}
}

func TestNewIndexPanics(t *testing.T) {
	s := openTempStore(t, StoreConfig{})

	expectPanic(t, "NewIndex(nil store)", "nil store", func() {
		_, _ = NewIndex[uint64](nil, u64IndexConfig) //nolint:staticcheck // intentional nil
	})
	expectPanic(t, "NewIndex(nil EncodeKey)", "nil EncodeKey", func() {
		cfg := u64IndexConfig
		cfg.EncodeKey = nil
		_, _ = NewIndex[uint64](s, cfg)
	})
	expectPanic(t, "NewIndex(nil DecodeKey)", "nil DecodeKey", func() {
		cfg := u64IndexConfig
		cfg.DecodeKey = nil
		_, _ = NewIndex[uint64](s, cfg)
	})
	expectPanic(t, "NewIndex(KeySize 0)", "KeySize <= 0", func() {
		cfg := u64IndexConfig
		cfg.KeySize = 0
		_, _ = NewIndex[uint64](s, cfg)
	})
	expectPanic(t, "NewIndex(empty name)", "empty index name", func() {
		cfg := u64IndexConfig
		cfg.Name = ""
		_, _ = NewIndex[uint64](s, cfg)
	})
	expectPanic(t, "NewIndex(32-byte name)", "maximum is 31", func() {
		cfg := u64IndexConfig
		cfg.Name = strings.Repeat("x", 32)
		_, _ = NewIndex[uint64](s, cfg)
	})

	// A 31-byte name is legal.
	cfg := u64IndexConfig
	cfg.Name = strings.Repeat("y", 31)
	if _, err := NewIndex[uint64](s, cfg); err != nil {
		t.Fatalf("NewIndex with 31-byte name: %v", err)
	}
}

func TestKeySizeMismatch(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	newU64Index(t, s) // "by-id" with KeySize 8

	cfg := u64IndexConfig
	cfg.KeySize = 16
	_, err := NewIndex[uint64](s, cfg)
	if !errors.Is(err, ErrKeySizeMismatch) {
		t.Fatalf("re-attach with KeySize 16: got %v, want ErrKeySizeMismatch", err)
	}
}

func TestRegistryCapacity(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	// (512-32)/56 = 8 registry slots.
	for i := 0; i < 8; i++ {
		cfg := u64IndexConfig
		cfg.Name = fmt.Sprintf("index-%d", i)
		if _, err := NewIndex[uint64](s, cfg); err != nil {
			t.Fatalf("NewIndex index-%d: %v", i, err)
		}
	}
	cfg := u64IndexConfig
	cfg.Name = "one-too-many"
	if _, err := NewIndex[uint64](s, cfg); !errors.Is(err, ErrTooManyIndexes) {
		t.Fatalf("9th index: got %v, want ErrTooManyIndexes", err)
	}
	if names := s.Indexes(); len(names) != 8 {
		t.Fatalf("Indexes() has %d names, want 8", len(names))
	}
}

func TestBlockSizeMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bs.db")

	s, err := OpenStore(StoreConfig{Path: path, BlockSize: 1024})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := OpenStore(StoreConfig{Path: path, BlockSize: 4096}); err == nil {
		t.Fatal("reopen with wrong BlockSize: got nil error")
	}
	// BlockSize 0 means "whatever the file says".
	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen with BlockSize 0: %v", err)
	}
	if s2.blockSize != 1024 {
		t.Fatalf("reopened blockSize = %d, want 1024", s2.blockSize)
	}
	s2.Close()
}

func TestInvalidBlockSize(t *testing.T) {
	for _, bs := range []int{-1, 256, 513, 1000} {
		if _, err := OpenStore(StoreConfig{Path: filepath.Join(t.TempDir(), "x.db"), BlockSize: bs}); err == nil {
			t.Errorf("OpenStore with BlockSize %d: got nil error", bs)
		}
	}
}

func TestNotAStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.db")
	if err := os.WriteFile(path, []byte("this is not a pluto store, just some text padding........"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(StoreConfig{Path: path}); err == nil {
		t.Fatal("OpenStore on a garbage file: got nil error")
	}
}

func TestEmptyIndexIteration(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	ix := newU64Index(t, s)
	for range ix.All() {
		t.Error("All on an empty index yields a pair")
	}
	if b, err := ix.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("MarshalJSON on an empty index = (%s, %v), want ([], nil)", b, err)
	}
}

// TestCorruptDirectorySlot verifies the corruption guard: a directory
// slot of 0 (block 0 is the superblock, never a bucket) surfaces as
// ErrCorrupt instead of scribbling on the superblock.
func TestCorruptDirectorySlot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.db")
	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	if _, err := ix.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Block 1 is the index's first directory block (NewIndex allocates
	// it first); zero its slot 0 on disk.
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	zero := make([]byte, 8)
	if _, err := f.WriteAt(zero, 4096+8); err != nil { // block 1, slot 0
		t.Fatal(err)
	}
	f.Close()

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if _, _, err := ix2.Search(1); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Search through a zeroed directory slot = %v, want ErrCorrupt", err)
	}
}

func TestFlusherEventuallyPersists(t *testing.T) {
	// The background flusher must persist without an explicit Sync.  Poll
	// with a deadline instead of sleeping a fixed time.
	dir := t.TempDir()
	path := filepath.Join(dir, "flush.db")
	s, err := OpenStore(StoreConfig{Path: path, FlushInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	if _, err := ix.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		s.mu.Lock()
		dirty := s.dirtyCount
		s.mu.Unlock()
		if dirty == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("flusher did not clean the cache within 10s")
		}
		time.Sleep(2 * time.Millisecond)
	}
	s.simulateCrash()

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	ix2, err := NewIndex[uint64](s2, u64IndexConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if v, found, err := ix2.Search(1); err != nil || !found || v != 1 {
		t.Fatalf("Search after flusher-only persistence = (%d, %v, %v)", v, found, err)
	}
}

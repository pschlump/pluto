package b_tree_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Unit tests: keys-per-block math, single insert/search/delete, split
// propagation to the root, borrow vs merge, free-list reuse, multi-tree
// stores, close/reopen persistence, Sync durability, ErrClosed
// behavior and the panic contract.  Tests run in this (internal)
// package so they can inspect the store, matching the house pattern of
// b_tree's b_tree_test.go.

import (
	"bytes"
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
// Test key encodings: uint64 keys as 8-byte big-endian (byte order ==
// integer order), and fixed 16-byte string keys.  Both encoders agree
// with their Compare — the TreeConfig contract.

var u64TreeConfig = TreeConfig[uint64]{
	Name:    "by-id",
	KeySize: 8,
	EncodeKey: func(k uint64, buf []byte) {
		binary.BigEndian.PutUint64(buf, k)
	},
	DecodeKey: func(buf []byte) uint64 {
		return binary.BigEndian.Uint64(buf)
	},
	Compare: func(a, b uint64) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	},
}

type nameKey [16]byte

func toNameKey(s string) nameKey {
	var k nameKey
	copy(k[:], s)
	return k
}

var nameTreeConfig = TreeConfig[nameKey]{
	Name:    "by-name",
	KeySize: 16,
	EncodeKey: func(k nameKey, buf []byte) {
		copy(buf, k[:])
	},
	DecodeKey: func(buf []byte) nameKey {
		var k nameKey
		copy(k[:], buf)
		return k
	},
	Compare: func(a, b nameKey) int {
		return bytes.Compare(a[:], b[:])
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

func newU64Tree(t *testing.T, s *Store) *Tree[uint64] {
	t.Helper()
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	return tr
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

// treeDepth returns the number of levels of the tree (1 = root is a leaf).
func treeDepth(t *testing.T, tr *Tree[uint64]) int {
	t.Helper()
	s := tr.store
	s.mu.Lock()
	defer s.mu.Unlock()
	rootNo, _, _, err := s.readEntryLocked(tr.slot)
	if err != nil {
		t.Fatalf("readEntry: %v", err)
	}
	depth := 0
	no := rootNo
	for {
		b, err := s.cacheGet(no)
		if err != nil {
			t.Fatalf("cacheGet %d: %v", no, err)
		}
		depth++
		if isLeaf(b.data) {
			return depth
		}
		no = internalChild(b.data, 0, tr.keySize, tr.maxInt)
	}
}

// numBlocks returns the store's block count under the lock.
func numBlocks(s *Store) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.numBlocks
}

// -------------------------------------------------------------------------------------------------------

func TestKeysPerBlockMath(t *testing.T) {
	cases := []struct {
		blockSize, keySize int
		wantLeaf, wantInt  int
	}{
		{4096, 8, 255, 255},
		{4096, 16, 170, 170},
		{512, 8, 31, 31},
		{512, 16, 20, 20},
	}
	for _, c := range cases {
		if got := maxLeafEntries(c.blockSize, c.keySize); got != c.wantLeaf {
			t.Errorf("maxLeafEntries(%d, %d) = %d, want %d", c.blockSize, c.keySize, got, c.wantLeaf)
		}
		if got := maxInternalKeys(c.blockSize, c.keySize); got != c.wantInt {
			t.Errorf("maxInternalKeys(%d, %d) = %d, want %d", c.blockSize, c.keySize, got, c.wantInt)
		}
	}
}

func TestErrKeyTooLarge(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	cfg := u64TreeConfig
	cfg.KeySize = 500 // (512-13)/(500+8) = 0 entries per leaf
	_, err := NewTree[uint64](s, cfg)
	if !errors.Is(err, ErrKeyTooLarge) {
		t.Fatalf("NewTree with a 500-byte key: got %v, want ErrKeyTooLarge", err)
	}
}

func TestInsertSearchDeleteSingle(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)

	if tr.Length() != 0 {
		t.Fatalf("empty tree has length %d", tr.Length())
	}
	if _, found, err := tr.Search(7); err != nil || found {
		t.Fatalf("Search on empty tree = (%v, %v), want (false, nil)", found, err)
	}
	if ok, err := tr.Delete(7); err != nil || ok {
		t.Fatalf("Delete on empty tree = (%v, %v), want (false, nil)", ok, err)
	}

	added, err := tr.Insert(42, 1000)
	if err != nil || !added {
		t.Fatalf("Insert = (%v, %v), want (true, nil)", added, err)
	}
	if tr.Length() != 1 {
		t.Fatalf("Length = %d, want 1", tr.Length())
	}
	v, found, err := tr.Search(42)
	if err != nil || !found || v != 1000 {
		t.Fatalf("Search = (%d, %v, %v), want (1000, true, nil)", v, found, err)
	}

	// Duplicate replaces the value and reports false.
	added, err = tr.Insert(42, 2000)
	if err != nil || added {
		t.Fatalf("duplicate Insert = (%v, %v), want (false, nil)", added, err)
	}
	if v, _, _ := tr.Search(42); v != 2000 {
		t.Fatalf("after replace Search = %d, want 2000", v)
	}
	if tr.Length() != 1 {
		t.Fatalf("after replace Length = %d, want 1", tr.Length())
	}

	ok, err := tr.Delete(42)
	if err != nil || !ok {
		t.Fatalf("Delete = (%v, %v), want (true, nil)", ok, err)
	}
	if _, found, _ := tr.Search(42); found {
		t.Fatal("Search finds deleted key")
	}
	if tr.Length() != 0 {
		t.Fatalf("after delete Length = %d, want 0", tr.Length())
	}
}

func TestSplitPropagationToRoot(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512}) // 31 entries/leaf: shallow, so 2000 keys forces height 3
	tr := newU64Tree(t, s)

	const n = 2000
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i*10)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if tr.Length() != n {
		t.Fatalf("Length = %d, want %d", tr.Length(), n)
	}
	if d := treeDepth(t, tr); d < 3 {
		t.Fatalf("depth = %d after %d sequential inserts into 31-entry leaves, want ≥ 3", d, n)
	}
	checkInvariants(t, tr)
	for i := 0; i < n; i += 97 {
		v, found, err := tr.Search(uint64(i))
		if err != nil || !found || v != uint64(i*10) {
			t.Fatalf("Search(%d) = (%d, %v, %v)", i, v, found, err)
		}
	}
	// All walks ascending through the split leaves.
	prev := uint64(0)
	count := 0
	for k, v := range tr.All() {
		if count > 0 && k <= prev {
			t.Fatalf("All not ascending: %d after %d", k, prev)
		}
		if v != k*10 {
			t.Fatalf("All yields value %d for key %d, want %d", v, k, k*10)
		}
		prev = k
		count++
	}
	if count != n {
		t.Fatalf("All yielded %d pairs, want %d", count, n)
	}
}

func TestBorrowVsMerge(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	tr := newU64Tree(t, s)

	const n = 2000
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	model := make(map[uint64]bool, n)
	for i := 0; i < n; i++ {
		model[uint64(i)] = true
	}

	// Delete every other key: neighbors stay above floor, so underfull
	// leaves borrow through the parent.
	for i := 0; i < n; i += 2 {
		ok, err := tr.Delete(uint64(i))
		if err != nil || !ok {
			t.Fatalf("Delete(%d) = (%v, %v)", i, ok, err)
		}
		delete(model, uint64(i))
		if i%310 == 0 {
			checkInvariants(t, tr)
		}
	}
	// Delete contiguous runs: siblings empty out, forcing merges.
	for i := 1; i < n/2; i += 2 {
		ok, err := tr.Delete(uint64(i))
		if err != nil || !ok {
			t.Fatalf("Delete(%d) = (%v, %v)", i, ok, err)
		}
		delete(model, uint64(i))
		if i%313 == 1 {
			checkInvariants(t, tr)
		}
	}
	checkInvariants(t, tr)
	if tr.Length() != uint64(len(model)) {
		t.Fatalf("Length = %d, want %d", tr.Length(), len(model))
	}
	for k := range model {
		if _, found, _ := tr.Search(k); !found {
			t.Fatalf("Search(%d) missing after borrow/merge deletes", k)
		}
	}
}

func TestFreeListReuse(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	tr := newU64Tree(t, s)

	const n = 3000
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	grown := numBlocks(s)
	if grown < uint64(n/31) {
		t.Fatalf("numBlocks = %d after %d inserts, suspiciously small", grown, n)
	}

	for i := 0; i < n; i++ {
		if _, err := tr.Delete(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
	}
	if tr.Length() != 0 {
		t.Fatalf("Length = %d after deleting everything", tr.Length())
	}
	checkInvariants(t, tr)

	// Reinsert: the freed blocks must be reused — the file must not grow
	// past its earlier high-water mark.
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i+1)); err != nil {
			t.Fatalf("reinsert %d: %v", i, err)
		}
	}
	if got := numBlocks(s); got > grown {
		t.Fatalf("numBlocks grew from %d to %d on reinsert: free list not reused", grown, got)
	}
	checkInvariants(t, tr)
	for i := 0; i < n; i += 101 {
		v, found, _ := tr.Search(uint64(i))
		if !found || v != uint64(i+1) {
			t.Fatalf("Search(%d) = (%d, %v) after reinsert", i, v, found)
		}
	}
}

func TestMultiTree(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	byID := newU64Tree(t, s)
	byName, err := NewTree[nameKey](s, nameTreeConfig)
	if err != nil {
		t.Fatalf("NewTree by-name: %v", err)
	}

	// Two indexes over the same 10k records: id → record offset and
	// name → record offset.
	const n = 10000
	for i := 0; i < n; i++ {
		off := uint64(i) * 128 // pretend records of 128 bytes
		if _, err := byID.Insert(uint64(i), off); err != nil {
			t.Fatalf("byID.Insert %d: %v", i, err)
		}
		if _, err := byName.Insert(toNameKey(fmt.Sprintf("user-%d", i)), off); err != nil {
			t.Fatalf("byName.Insert %d: %v", i, err)
		}
	}
	if byID.Length() != n || byName.Length() != n {
		t.Fatalf("lengths = %d, %d, want %d, %d", byID.Length(), byName.Length(), n, n)
	}
	if names := s.Trees(); len(names) != 2 || names[0] != "by-id" || names[1] != "by-name" {
		t.Fatalf("Trees() = %v, want [by-id by-name]", names)
	}
	// Both indexes resolve a record to the same offset.
	off1, found1, _ := byID.Search(4242)
	off2, found2, _ := byName.Search(toNameKey("user-4242"))
	if !found1 || !found2 || off1 != off2 || off1 != 4242*128 {
		t.Fatalf("index lookup = (%d, %v), (%d, %v)", off1, found1, off2, found2)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and re-attach by name: both indexes persist.
	s2, err := OpenStore(StoreConfig{Path: s.path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	byID2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach by-id: %v", err)
	}
	byName2, err := NewTree[nameKey](s2, nameTreeConfig)
	if err != nil {
		t.Fatalf("re-attach by-name: %v", err)
	}
	if byID2.Length() != n || byName2.Length() != n {
		t.Fatalf("after reopen lengths = %d, %d", byID2.Length(), byName2.Length())
	}
	for i := 0; i < n; i += 733 {
		off, found, _ := byID2.Search(uint64(i))
		if !found || off != uint64(i)*128 {
			t.Fatalf("byID2.Search(%d) = (%d, %v)", i, off, found)
		}
		off, found, _ = byName2.Search(toNameKey(fmt.Sprintf("user-%d", i)))
		if !found || off != uint64(i)*128 {
			t.Fatalf("byName2.Search(user-%d) = (%d, %v)", i, off, found)
		}
	}
}

func TestCloseReopenPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	const n = 5000
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i*7%n), uint64(i)); err != nil { // scrambled order
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	tr2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if tr2.Length() != n {
		t.Fatalf("Length after reopen = %d, want %d", tr2.Length(), n)
	}
	// key i holds value i0 where i0*7 % n == i; since 7*2143 ≡ 1 (mod 5000),
	// i0 = i*2143 % n.
	for i := 0; i < n; i++ {
		v, found, err := tr2.Search(uint64(i))
		if err != nil || !found || v != uint64(i*2143%n) {
			t.Fatalf("Search(%d) = (%d, %v, %v), want (%d, true, nil)", i, v, found, err, i*2143%n)
		}
	}
}

func TestSyncDurability(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.db")

	s, err := OpenStore(StoreConfig{Path: path})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	const n = 3000
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
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
	tr2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if tr2.Length() != n {
		t.Fatalf("Length after crash = %d, want %d (Sync must be durable)", tr2.Length(), n)
	}
	for i := 0; i < n; i++ {
		if _, found, _ := tr2.Search(uint64(i)); !found {
			t.Fatalf("key %d missing after Sync + crash", i)
		}
	}
}

func TestErrClosed(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)
	if _, err := tr.Insert(1, 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := tr.Insert(2, 2); !errors.Is(err, ErrClosed) {
		t.Errorf("Insert after Close = %v, want ErrClosed", err)
	}
	if _, _, err := tr.Search(1); !errors.Is(err, ErrClosed) {
		t.Errorf("Search after Close = %v, want ErrClosed", err)
	}
	if _, err := tr.Delete(1); !errors.Is(err, ErrClosed) {
		t.Errorf("Delete after Close = %v, want ErrClosed", err)
	}
	if err := s.Sync(); !errors.Is(err, ErrClosed) {
		t.Errorf("Sync after Close = %v, want ErrClosed", err)
	}
	if _, err := NewTree[uint64](s, u64TreeConfig); !errors.Is(err, ErrClosed) {
		t.Errorf("NewTree after Close = %v, want ErrClosed", err)
	}
	if err := s.Close(); !errors.Is(err, ErrClosed) {
		t.Errorf("second Close = %v, want ErrClosed", err)
	}
	if n := tr.Length(); n != 0 {
		t.Errorf("Length after Close = %d, want 0", n)
	}
	if names := s.Trees(); names != nil {
		t.Errorf("Trees after Close = %v, want nil", names)
	}
	for range tr.All() {
		t.Error("All after Close yields a pair")
	}
	for range tr.Range(0) {
		t.Error("Range after Close yields a pair")
	}
}

func TestNewTreePanics(t *testing.T) {
	s := openTempStore(t, StoreConfig{})

	expectPanic(t, "NewTree(nil store)", "nil store", func() {
		_, _ = NewTree[uint64](nil, u64TreeConfig) //nolint:staticcheck // intentional nil
	})
	expectPanic(t, "NewTree(nil EncodeKey)", "nil EncodeKey", func() {
		cfg := u64TreeConfig
		cfg.EncodeKey = nil
		_, _ = NewTree[uint64](s, cfg)
	})
	expectPanic(t, "NewTree(nil DecodeKey)", "nil DecodeKey", func() {
		cfg := u64TreeConfig
		cfg.DecodeKey = nil
		_, _ = NewTree[uint64](s, cfg)
	})
	expectPanic(t, "NewTree(nil Compare)", "nil Compare", func() {
		cfg := u64TreeConfig
		cfg.Compare = nil
		_, _ = NewTree[uint64](s, cfg)
	})
	expectPanic(t, "NewTree(KeySize 0)", "KeySize <= 0", func() {
		cfg := u64TreeConfig
		cfg.KeySize = 0
		_, _ = NewTree[uint64](s, cfg)
	})
	expectPanic(t, "NewTree(empty name)", "empty tree name", func() {
		cfg := u64TreeConfig
		cfg.Name = ""
		_, _ = NewTree[uint64](s, cfg)
	})
	expectPanic(t, "NewTree(32-byte name)", "maximum is 31", func() {
		cfg := u64TreeConfig
		cfg.Name = strings.Repeat("x", 32)
		_, _ = NewTree[uint64](s, cfg)
	})

	// A 31-byte name is legal.
	cfg := u64TreeConfig
	cfg.Name = strings.Repeat("y", 31)
	if _, err := NewTree[uint64](s, cfg); err != nil {
		t.Fatalf("NewTree with 31-byte name: %v", err)
	}
}

func TestKeySizeMismatch(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	newU64Tree(t, s) // "by-id" with KeySize 8

	cfg := u64TreeConfig
	cfg.KeySize = 16
	_, err := NewTree[uint64](s, cfg)
	if !errors.Is(err, ErrKeySizeMismatch) {
		t.Fatalf("re-attach with KeySize 16: got %v, want ErrKeySizeMismatch", err)
	}
}

func TestRegistryCapacity(t *testing.T) {
	s := openTempStore(t, StoreConfig{BlockSize: 512})
	// (512-32)/56 = 8 registry slots.
	for i := 0; i < 8; i++ {
		cfg := u64TreeConfig
		cfg.Name = fmt.Sprintf("tree-%d", i)
		if _, err := NewTree[uint64](s, cfg); err != nil {
			t.Fatalf("NewTree tree-%d: %v", i, err)
		}
	}
	cfg := u64TreeConfig
	cfg.Name = "one-too-many"
	if _, err := NewTree[uint64](s, cfg); !errors.Is(err, ErrTooManyTrees) {
		t.Fatalf("9th tree: got %v, want ErrTooManyTrees", err)
	}
	if names := s.Trees(); len(names) != 8 {
		t.Fatalf("Trees() has %d names, want 8", len(names))
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
	for _, bs := range []int{100, 256, 511, 513, 1000} {
		_, err := OpenStore(StoreConfig{Path: filepath.Join(t.TempDir(), "x.db"), BlockSize: bs})
		if err == nil {
			t.Errorf("BlockSize %d: got nil error, want invalid block size", bs)
		}
	}
}

func TestEmptyTreeIteration(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)
	for range tr.All() {
		t.Error("All on an empty tree yields a pair")
	}
	for range tr.Range(100) {
		t.Error("Range on an empty tree yields a pair")
	}
}

func TestRangeStartKey(t *testing.T) {
	s := openTempStore(t, StoreConfig{})
	tr := newU64Tree(t, s)
	for i := 0; i < 1000; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}
	count := 0
	for k := range tr.Range(500) {
		if k != uint64(500+count) {
			t.Fatalf("Range(500) yields key %d at position %d", k, count)
		}
		count++
	}
	if count != 500 {
		t.Fatalf("Range(500) yielded %d pairs, want 500", count)
	}
	// lo above every key yields nothing; lo between keys starts at the next.
	for range tr.Range(1000) {
		t.Error("Range(1000) on keys 0..999 yields a pair")
	}
	first := true
	for k := range tr.Range(501) {
		if !first {
			break
		}
		first = false
		if k != 501 {
			t.Fatalf("Range(501) starts at %d, want 501", k)
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

func TestFlusherEventuallyPersists(t *testing.T) {
	// The background flusher must persist without an explicit Sync.  Poll
	// with a deadline instead of sleeping a fixed time.
	dir := t.TempDir()
	path := filepath.Join(dir, "flush.db")
	s, err := OpenStore(StoreConfig{Path: path, FlushInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	if _, err := tr.Insert(1, 1); err != nil {
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
	tr2, err := NewTree[uint64](s2, u64TreeConfig)
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	if _, found, _ := tr2.Search(1); !found {
		t.Fatal("key missing after background flush + crash")
	}
}

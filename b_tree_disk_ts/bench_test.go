package b_tree_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Benchmarks: insert, hot search (whole tree cached), cold search
// (cache far smaller than the tree) and delete.  Run with
//
//	go test -run='^$' -bench=. -benchmem ./b_tree_disk_ts/

import (
	"path/filepath"
	"testing"
)

func benchStore(b *testing.B, cacheBlocks int) (*Store, *Tree[uint64]) {
	b.Helper()
	s, err := OpenStore(StoreConfig{
		Path:        filepath.Join(b.TempDir(), "bench.db"),
		CacheBlocks: cacheBlocks,
	})
	if err != nil {
		b.Fatalf("OpenStore: %v", err)
	}
	tr, err := NewTree[uint64](s, u64TreeConfig)
	if err != nil {
		b.Fatalf("NewTree: %v", err)
	}
	return s, tr
}

func benchFill(b *testing.B, tr *Tree[uint64], n int) {
	b.Helper()
	for i := 0; i < n; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			b.Fatalf("Insert %d: %v", i, err)
		}
	}
}

func BenchmarkInsert(b *testing.B) {
	s, tr := benchStore(b, 1024)
	defer s.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tr.Insert(uint64(i), uint64(i)); err != nil {
			b.Fatalf("Insert %d: %v", i, err)
		}
	}
}

func BenchmarkSearchHot(b *testing.B) {
	s, tr := benchStore(b, 1024)
	defer s.Close()
	const n = 100000
	benchFill(b, tr, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := tr.Search(uint64(i % n)); err != nil || !found {
			b.Fatalf("Search(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

// BenchmarkSearchCold searches a 100k-key tree through a 64-block cache
// (~256 KiB against ~1.6 MB of leaf blocks), so most searches miss the
// cache and hit the file.
func BenchmarkSearchCold(b *testing.B) {
	s, tr := benchStore(b, 64)
	defer s.Close()
	const n = 100000
	benchFill(b, tr, n)
	if err := s.Sync(); err != nil {
		b.Fatalf("Sync: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := tr.Search(uint64(i % n)); err != nil || !found {
			b.Fatalf("Search(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	s, tr := benchStore(b, 1024)
	defer s.Close()
	benchFill(b, tr, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tr.Delete(uint64(i)); err != nil {
			b.Fatalf("Delete %d: %v", i, err)
		}
	}
}

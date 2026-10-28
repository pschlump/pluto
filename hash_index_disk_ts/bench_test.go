package hash_index_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Benchmarks: insert, hot search (whole index cached), cold search
// (cache far smaller than the index) and delete.  Run with
//
//	go test -run='^$' -bench=. -benchmem ./hash_index_disk_ts/

import (
	"path/filepath"
	"testing"
)

func benchStore(b *testing.B, cacheBlocks int) (*Store, *Index[uint64]) {
	b.Helper()
	s, err := OpenStore(StoreConfig{
		Path:        filepath.Join(b.TempDir(), "bench.db"),
		CacheBlocks: cacheBlocks,
	})
	if err != nil {
		b.Fatalf("OpenStore: %v", err)
	}
	ix, err := NewIndex[uint64](s, u64IndexConfig)
	if err != nil {
		b.Fatalf("NewIndex: %v", err)
	}
	return s, ix
}

func benchFill(b *testing.B, ix *Index[uint64], n int) {
	b.Helper()
	for i := 0; i < n; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			b.Fatalf("Insert %d: %v", i, err)
		}
	}
}

func BenchmarkInsert(b *testing.B) {
	s, ix := benchStore(b, 1024)
	defer s.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ix.Insert(uint64(i), uint64(i)); err != nil {
			b.Fatalf("Insert %d: %v", i, err)
		}
	}
}

func BenchmarkSearchHot(b *testing.B) {
	s, ix := benchStore(b, 1024)
	defer s.Close()
	const n = 100000
	benchFill(b, ix, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := ix.Search(uint64(i % n)); err != nil || !found {
			b.Fatalf("Search(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

// BenchmarkSearchCold searches a 100k-key index through a 64-block
// cache (~256 KiB against ~1.7 MB of bucket blocks), so most searches
// miss the cache and hit the file — but still touch only one directory
// block and one bucket block.
func BenchmarkSearchCold(b *testing.B) {
	s, ix := benchStore(b, 64)
	defer s.Close()
	const n = 100000
	benchFill(b, ix, n)
	if err := s.Sync(); err != nil {
		b.Fatalf("Sync: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := ix.Search(uint64(i % n)); err != nil || !found {
			b.Fatalf("Search(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	s, ix := benchStore(b, 1024)
	defer s.Close()
	benchFill(b, ix, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ix.Delete(uint64(i)); err != nil {
			b.Fatalf("Delete %d: %v", i, err)
		}
	}
}

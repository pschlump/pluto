package database_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Benchmarks: put (both indexes + the slot write), the two point-read
// paths (hash-index by ID, B+ tree by name), range scan and delete.
// Run with
//
//	go test -run='^$' -bench=. -benchmem ./database_disk_ts/

import (
	"path/filepath"
	"testing"
)

func benchDB(b *testing.B, cacheBlocks int) *DB[uint64, [16]byte] {
	b.Helper()
	eq, ord := testConfigs()
	db, err := OpenDB[uint64, [16]byte](DBConfig{
		Path:        filepath.Join(b.TempDir(), "bench.db"),
		CacheBlocks: cacheBlocks,
	}, eq, ord)
	if err != nil {
		b.Fatalf("OpenDB: %v", err)
	}
	return db
}

func benchFill(b *testing.B, db *DB[uint64, [16]byte], n int) {
	b.Helper()
	for i := 0; i < n; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), []byte{byte(i)}); err != nil {
			b.Fatalf("Put %d: %v", i, err)
		}
	}
}

func BenchmarkPut(b *testing.B) {
	db := benchDB(b, 1024)
	defer db.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), []byte{byte(i)}); err != nil {
			b.Fatalf("Put %d: %v", i, err)
		}
	}
}

// BenchmarkGetByID is the equality path: the hash index (O(1)
// expected) plus one slot read.
func BenchmarkGetByID(b *testing.B) {
	db := benchDB(b, 1024)
	defer db.Close()
	const n = 100000
	benchFill(b, db, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := db.GetByID(uint64(i % n)); err != nil || !found {
			b.Fatalf("GetByID(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

// BenchmarkGetByName is the ordered path: the B+ tree (O(log n)) plus
// one slot read.
func BenchmarkGetByName(b *testing.B) {
	db := benchDB(b, 1024)
	defer db.Close()
	const n = 100000
	benchFill(b, db, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, found, err := db.GetByName(nameOf(uint64(i % n))); err != nil || !found {
			b.Fatalf("GetByName(%d) = (%v, %v)", i%n, found, err)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	db := benchDB(b, 1024)
	defer db.Close()
	benchFill(b, db, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.DeleteByID(uint64(i)); err != nil {
			b.Fatalf("Delete %d: %v", i, err)
		}
	}
}

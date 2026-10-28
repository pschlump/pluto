/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package database_disk_ts_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pschlump/pluto/b_tree_disk_ts"
	"github.com/pschlump/pluto/database_disk_ts"
	"github.com/pschlump/pluto/hash_index_disk_ts"
)

// A small customer database: uint64 IDs (the hash-index side — O(1)
// expected equality) and fixed 16-byte names (the B+ tree side —
// ordered range scans).  Each record is a block of data; both indexes
// resolve it, and RangeByName walks names in order.
func Example() {
	dir, err := os.MkdirTemp("", "database_disk_ts-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := database_disk_ts.OpenDB[uint64, [16]byte](
		database_disk_ts.DBConfig{Path: filepath.Join(dir, "shop.db")},
		hash_index_disk_ts.IndexConfig[uint64]{
			Name:    "by-id",
			KeySize: 8,
			EncodeKey: func(k uint64, buf []byte) {
				binary.BigEndian.PutUint64(buf, k)
			},
			DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
		},
		b_tree_disk_ts.TreeConfig[[16]byte]{
			Name:      "by-name",
			KeySize:   16,
			EncodeKey: func(k [16]byte, buf []byte) { copy(buf, k[:]) },
			DecodeKey: func(buf []byte) (k [16]byte) { copy(k[:], buf); return },
			Compare:   func(a, b [16]byte) int { return bytes.Compare(a[:], b[:]) },
		})
	if err != nil {
		log.Fatal(err)
	}

	name := func(s string) (k [16]byte) { copy(k[:], s); return }
	customers := []string{"ada", "grace", "edsger"}
	for i, c := range customers {
		record := []byte(fmt.Sprintf("%s's record, #%d", c, i+1))
		if _, err := db.Put(uint64(i+1), name(c), record); err != nil {
			log.Fatal(err)
		}
	}

	// By name (B+ tree) and by ID (hash index) resolve the same record.
	record, found, err := db.GetByName(name("grace"))
	if err != nil || !found {
		log.Fatal(err)
	}
	fmt.Println(string(record))
	id := uint64(3)
	record, _, _ = db.GetByID(id)
	fmt.Println(string(record))

	// Ordered: everyone from "b" onward, ascending.
	for nm, rec := range db.RangeByName(name("b"), name("zzzz")) {
		fmt.Printf("%s: %s\n", bytes.TrimRight(nm[:], "\x00"), rec)
	}

	// Renaming is a Put: the old name entry goes, the ID stays.
	if _, err := db.Put(2, name("countess"), []byte("renamed")); err != nil {
		log.Fatal(err)
	}
	if _, found, _ := db.GetByName(name("grace")); found {
		log.Fatal("the old name still resolves")
	}
	rec2, _, _ := db.GetByID(2)
	fmt.Println(string(rec2))

	if err := db.Close(); err != nil { // final flush; everything is durable
		log.Fatal(err)
	}
	// Output:
	// grace's record, #2
	// edsger's record, #3
	// edsger: edsger's record, #3
	// grace: grace's record, #2
	// renamed
}

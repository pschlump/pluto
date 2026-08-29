/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package b_tree_disk_ts_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pschlump/pluto/b_tree_disk_ts"
)

// A store holds any number of named trees, so one file can carry
// several indexes over the same record set — here a by-ID index with
// uint64 keys and a by-name index with fixed 16-byte keys, both mapping
// to record offsets (values are always uint64).
func Example() {
	dir, err := os.MkdirTemp("", "b_tree_disk_ts-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := b_tree_disk_ts.OpenStore(b_tree_disk_ts.StoreConfig{
		Path: filepath.Join(dir, "shop.db"),
	})
	if err != nil {
		log.Fatal(err)
	}

	byID, err := b_tree_disk_ts.NewTree[uint64](store, b_tree_disk_ts.TreeConfig[uint64]{
		Name:    "by-id",
		KeySize: 8,
		EncodeKey: func(k uint64, buf []byte) {
			binary.BigEndian.PutUint64(buf, k) // big-endian: byte order == integer order
		},
		DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
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
	})
	if err != nil {
		log.Fatal(err)
	}

	byName, err := b_tree_disk_ts.NewTree[[16]byte](store, b_tree_disk_ts.TreeConfig[[16]byte]{
		Name:      "by-name",
		KeySize:   16,
		EncodeKey: func(k [16]byte, buf []byte) { copy(buf, k[:]) },
		DecodeKey: func(buf []byte) (k [16]byte) { copy(k[:], buf); return },
		Compare:   func(a, b [16]byte) int { return bytes.Compare(a[:], b[:]) },
	})
	if err != nil {
		log.Fatal(err)
	}

	nameKey := func(s string) (k [16]byte) { copy(k[:], s); return }
	names := []string{"ada", "grace", "edsger"}
	for i, name := range names {
		id := uint64(i + 1)
		offset := uint64(i) * 256 // pretend records of 256 bytes
		if _, err := byID.Insert(id, offset); err != nil {
			log.Fatal(err)
		}
		if _, err := byName.Insert(nameKey(name), offset); err != nil {
			log.Fatal(err)
		}
	}

	// Both indexes resolve a record to the same offset.
	off, found, err := byName.Search(nameKey("grace"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("grace at offset", off, "found:", found)

	// All iterates in ascending key order (a snapshot).
	for id, offset := range byID.All() {
		fmt.Println(id, names[offset/256])
	}

	if err := store.Close(); err != nil { // Close runs the final flush
		log.Fatal(err)
	}
	// Output:
	// grace at offset 256 found: true
	// 1 ada
	// 2 grace
	// 3 edsger
}

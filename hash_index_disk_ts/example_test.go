/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package hash_index_disk_ts_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/pschlump/pluto/hash_index_disk_ts"
)

// A store holds any number of named indexes, so one file can carry
// several indexes over the same record set — here a by-ID index with
// uint64 keys and a by-name index with fixed 16-byte keys, both mapping
// to record offsets (values are always uint64).  Note there is no
// Compare function anywhere: keys are found by equality of their
// encodings, never ordered.
func Example() {
	dir, err := os.MkdirTemp("", "hash_index_disk_ts-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := hash_index_disk_ts.OpenStore(hash_index_disk_ts.StoreConfig{
		Path: filepath.Join(dir, "shop.db"),
	})
	if err != nil {
		log.Fatal(err)
	}

	byID, err := hash_index_disk_ts.NewIndex[uint64](store, hash_index_disk_ts.IndexConfig[uint64]{
		Name:    "by-id",
		KeySize: 8,
		EncodeKey: func(k uint64, buf []byte) {
			binary.BigEndian.PutUint64(buf, k)
		},
		DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
	})
	if err != nil {
		log.Fatal(err)
	}

	byName, err := hash_index_disk_ts.NewIndex[[16]byte](store, hash_index_disk_ts.IndexConfig[[16]byte]{
		Name:      "by-name",
		KeySize:   16,
		EncodeKey: func(k [16]byte, buf []byte) { copy(buf, k[:]) },
		DecodeKey: func(buf []byte) (k [16]byte) { copy(k[:], buf); return },
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

	// Both indexes resolve a record to the same offset — one directory
	// block and one bucket block, expected.
	off, found, err := byName.Search(nameKey("grace"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("grace at offset", off, "found:", found)

	// All iterates in BUCKET order — the physical order of the hash
	// directory, not key order — so collect and sort when an order
	// matters (MarshalJSON does the same).
	ids := make([]uint64, 0, byID.Length())
	for id := range byID.All() {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		off, _, _ := byID.Search(id)
		fmt.Println(id, names[off/256])
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

// exampleJSONIndex opens a temporary store holding one uint64-keyed
// index for the JSON examples; the returned cleanup closes and removes
// it.
func exampleJSONIndex() (*hash_index_disk_ts.Index[uint64], func()) {
	dir, err := os.MkdirTemp("", "hash_index_disk_ts-json-example")
	if err != nil {
		log.Fatal(err)
	}
	store, err := hash_index_disk_ts.OpenStore(hash_index_disk_ts.StoreConfig{
		Path: filepath.Join(dir, "index.db"),
	})
	if err != nil {
		log.Fatal(err)
	}
	ix, err := hash_index_disk_ts.NewIndex[uint64](store, hash_index_disk_ts.IndexConfig[uint64]{
		Name:    "by-id",
		KeySize: 8,
		EncodeKey: func(k uint64, buf []byte) {
			binary.BigEndian.PutUint64(buf, k)
		},
		DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
	})
	if err != nil {
		log.Fatal(err)
	}
	cleanup := func() {
		if err := store.Close(); err != nil {
			log.Fatal(err)
		}
		os.RemoveAll(dir)
	}
	return ix, cleanup
}

// MarshalJSON encodes the index as a JSON array of {"key":…,"value":…}
// objects sorted by the encoded key bytes — a deterministic order the
// bucket order of All cannot supply.
func ExampleIndex_MarshalJSON() {
	ix, cleanup := exampleJSONIndex()
	defer cleanup()

	for _, kv := range [][2]uint64{{3, 300}, {1, 100}, {2, 200}} {
		if _, err := ix.Insert(kv[0], kv[1]); err != nil {
			log.Fatal(err)
		}
	}

	b, err := json.Marshal(ix)
	fmt.Println(string(b), err)
	// Output:
	// [{"key":1,"value":100},{"key":2,"value":200},{"key":3,"value":300}] <nil>
}

// UnmarshalJSON replaces the contents of the index from a JSON array of
// {"key":…,"value":…} objects; the printed pairs are sorted by key
// because All's bucket order carries no ordering guarantee.
func ExampleIndex_UnmarshalJSON() {
	ix, cleanup := exampleJSONIndex()
	defer cleanup()

	if err := json.Unmarshal([]byte(`[{"key":7,"value":70},{"key":4,"value":40}]`), ix); err != nil {
		fmt.Println("error:", err)
		return
	}
	keys := make([]uint64, 0, ix.Length())
	for k := range ix.All() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v, _, _ := ix.Search(k)
		fmt.Println(k, v)
	}
	// Output:
	// 4 40
	// 7 70
}

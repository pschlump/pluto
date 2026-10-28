package database_disk_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Tests for the record-file layer and the DB operations.  The
// randomized model test, invariant checker, reopen cycles and
// self-heal scenarios live in thorough_test.go.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pschlump/pluto/b_tree_disk_ts"
	"github.com/pschlump/pluto/hash_index_disk_ts"
)

// Test types: uint64 IDs (the hash-index side) and fixed 16-byte names
// (the B+ tree side) — the by-name note's shape.  The name derives
// deterministically from the id so the model test stays simple; the
// rename tests exercise independent names.
func nk(s string) [16]byte {
	var k [16]byte
	copy(k[:], s)
	return k
}

func testConfigs() (hash_index_disk_ts.IndexConfig[uint64], b_tree_disk_ts.TreeConfig[[16]byte]) {
	eq := hash_index_disk_ts.IndexConfig[uint64]{
		Name:    "by-id",
		KeySize: 8,
		EncodeKey: func(k uint64, buf []byte) {
			binary.BigEndian.PutUint64(buf, k)
		},
		DecodeKey: func(buf []byte) uint64 { return binary.BigEndian.Uint64(buf) },
	}
	ord := b_tree_disk_ts.TreeConfig[[16]byte]{
		Name:      "by-name",
		KeySize:   16,
		EncodeKey: func(k [16]byte, buf []byte) { copy(buf, k[:]) },
		DecodeKey: func(buf []byte) (k [16]byte) { copy(k[:], buf); return },
		Compare:   func(a, b [16]byte) int { return bytes.Compare(a[:], b[:]) },
	}
	return eq, ord
}

func openTempDB(t *testing.T, cfg DBConfig) *DB[uint64, [16]byte] {
	t.Helper()
	if cfg.Path == "" {
		cfg.Path = filepath.Join(t.TempDir(), "db")
	}
	eq, ord := testConfigs()
	db, err := OpenDB[uint64, [16]byte](cfg, eq, ord)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	return db
}

func nameOf(id uint64) [16]byte { return nk(fmt.Sprintf("n%07d", id)) }

func expectPanic(t *testing.T, what, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected a panic containing %q, got none", what, want)
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
			t.Fatalf("%s: panic %v does not contain %q", what, r, want)
		}
	}()
	f()
}

func TestPutGetBothIndexes(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	replaced, err := db.Put(7, nk("grace"), []byte("record of grace"))
	if err != nil || replaced {
		t.Fatalf("Put = (%v, %v), want (false, nil)", replaced, err)
	}
	if db.Len() != 1 {
		t.Fatalf("Len = %d, want 1", db.Len())
	}

	data, found, err := db.GetByID(7)
	if err != nil || !found || string(data) != "record of grace" {
		t.Fatalf("GetByID = (%q, %v, %v)", data, found, err)
	}
	data, found, err = db.GetByName(nk("grace"))
	if err != nil || !found || string(data) != "record of grace" {
		t.Fatalf("GetByName = (%q, %v, %v)", data, found, err)
	}

	// Misses on both paths.
	if _, found, _ := db.GetByID(8); found {
		t.Fatal("GetByID(8) found an absent record")
	}
	if _, found, _ := db.GetByName(nk("ada")); found {
		t.Fatal("GetByName(ada) found an absent record")
	}
}

func TestUpdateRewrite(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if _, err := db.Put(1, nk("ada"), []byte("v1")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	replaced, err := db.Put(1, nk("ada"), []byte("v2"))
	if err != nil || !replaced {
		t.Fatalf("rewrite Put = (%v, %v), want (true, nil)", replaced, err)
	}
	if db.Len() != 1 {
		t.Fatalf("Len = %d after rewrite, want 1", db.Len())
	}
	data, _, _ := db.GetByID(1)
	if string(data) != "v2" {
		t.Fatalf("GetByID after rewrite = %q, want v2", data)
	}
	data, _, _ = db.GetByName(nk("ada"))
	if string(data) != "v2" {
		t.Fatalf("GetByName after rewrite = %q, want v2", data)
	}
}

func TestRename(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if _, err := db.Put(1, nk("ada"), []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	replaced, err := db.Put(1, nk("countess"), []byte("data"))
	if err != nil || !replaced {
		t.Fatalf("rename Put = (%v, %v), want (true, nil)", replaced, err)
	}
	if db.Len() != 1 {
		t.Fatalf("Len = %d after rename, want 1 (rename is not an insert)", db.Len())
	}
	if _, found, _ := db.GetByName(nk("ada")); found {
		t.Fatal("the old name still resolves after the rename")
	}
	data, found, _ := db.GetByName(nk("countess"))
	if !found || string(data) != "data" {
		t.Fatalf("GetByName(countess) = (%q, %v)", data, found)
	}
	if data, found, _ := db.GetByID(1); !found || string(data) != "data" {
		t.Fatalf("GetByID after rename = (%q, %v)", data, found)
	}
	checkInvariants(t, db)
}

func TestNameConflict(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if _, err := db.Put(1, nk("ada"), []byte("one")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := db.Put(2, nk("ada"), []byte("two")); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("Put with a taken name = %v, want ErrNameConflict", err)
	}
	// A failed Put leaves nothing behind.
	if db.Len() != 1 {
		t.Fatalf("Len = %d after the rejected Put, want 1", db.Len())
	}
	if _, found, _ := db.GetByID(2); found {
		t.Fatal("the rejected record is visible by ID")
	}
	// Renaming record 2 ONTO ada's name is the same conflict.
	if _, err := db.Put(2, nk("grace"), []byte("two")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := db.Put(2, nk("ada"), []byte("two")); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("rename onto a taken name = %v, want ErrNameConflict", err)
	}
	// Deleting the holder frees the name.
	if ok, err := db.DeleteByID(1); err != nil || !ok {
		t.Fatalf("DeleteByID = (%v, %v)", ok, err)
	}
	if _, err := db.Put(2, nk("ada"), []byte("two")); err != nil {
		t.Fatalf("Put after freeing the name: %v", err)
	}
	checkInvariants(t, db)
}

func TestDeleteBothWays(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	const n = 100
	for i := 0; i < n; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), []byte{byte(i)}); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	// DeleteByID removes the name entry too.
	if ok, err := db.DeleteByID(10); err != nil || !ok {
		t.Fatalf("DeleteByID(10) = (%v, %v)", ok, err)
	}
	if _, found, _ := db.GetByName(nameOf(10)); found {
		t.Fatal("DeleteByID left the name entry behind")
	}
	// DeleteByName removes the id entry too.
	if ok, err := db.DeleteByName(nameOf(20)); err != nil || !ok {
		t.Fatalf("DeleteByName(20) = (%v, %v)", ok, err)
	}
	if _, found, _ := db.GetByID(20); found {
		t.Fatal("DeleteByName left the id entry behind")
	}
	if db.Len() != n-2 {
		t.Fatalf("Len = %d, want %d", db.Len(), n-2)
	}
	// Deleting again reports false, not an error.
	if ok, err := db.DeleteByID(10); err != nil || ok {
		t.Fatalf("second DeleteByID = (%v, %v), want (false, nil)", ok, err)
	}
	checkInvariants(t, db)
}

func TestSlotReuse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	db := openTempDB(t, DBConfig{Path: path})
	const n = 300
	for i := 0; i < n; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), nil); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}
	db.mu.Lock()
	high := db.numSlots
	db.mu.Unlock()
	for i := 0; i < n; i++ {
		if _, err := db.DeleteByID(uint64(i)); err != nil {
			t.Fatalf("Delete %d: %v", i, err)
		}
	}
	if db.Len() != 0 {
		t.Fatalf("Len = %d after delete-all", db.Len())
	}
	for i := 0; i < n; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), nil); err != nil {
			t.Fatalf("refill Put %d: %v", i, err)
		}
	}
	db.mu.Lock()
	after := db.numSlots
	db.mu.Unlock()
	if after != high {
		t.Fatalf("refill grew the record file: %d slots at the high-water mark, %d after the refill", high, after)
	}
	checkInvariants(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRecordTooLarge(t *testing.T) {
	db := openTempDB(t, DBConfig{RecordSize: 128})
	defer db.Close()

	// Slot: 13 header + 8 id + 16 name = 37; maxData = 128-37 = 91.
	if db.MaxData() != 91 {
		t.Fatalf("MaxData = %d, want 91", db.MaxData())
	}
	if _, err := db.Put(1, nk("ada"), make([]byte, 91)); err != nil {
		t.Fatalf("Put of exactly MaxData bytes: %v", err)
	}
	if _, err := db.Put(2, nk("grace"), make([]byte, 92)); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Put of MaxData+1 = %v, want ErrRecordTooLarge", err)
	}
}

func TestRangeByName(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	names := []string{"mike", "ada", "zoe", "grace", "edsger", "beck"}
	for i, name := range names {
		if _, err := db.Put(uint64(i), nk(name), []byte(name+"'s data")); err != nil {
			t.Fatalf("Put %s: %v", name, err)
		}
	}
	var got []string
	for name, data := range db.RangeByName(nk("c"), nk("mzzz")) {
		got = append(got, string(bytes.TrimRight(name[:], "\x00"))+"="+string(data))
	}
	// Ascending names: ada, beck, edsger, grace, mike, zoe.  [c, mzzz]
	// holds edsger, grace, mike (ada and beck sort below c, zoe above
	// mzzz).
	want := []string{"edsger=edsger's data", "grace=grace's data", "mike=mike's data"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("RangeByName(c, mzzz) = %v, want %v", got, want)
	}

	// A range nothing falls in is empty.
	count := 0
	for range db.RangeByName(nk("x"), nk("y")) {
		count++
	}
	if count != 0 {
		t.Fatalf("empty range yielded %d records", count)
	}

	// The full range is ascending name order.
	var order []string
	for name := range db.RangeByName(nk(""), nk("zzzzzzzzzzzzzzzz")) {
		order = append(order, string(name[:]))
	}
	sorted := append([]string(nil), order...)
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] > sorted[i] {
			t.Fatalf("RangeByName is not ascending: %v", order)
		}
	}
}

func TestAll(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	for i := 0; i < 50; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), []byte{byte(i)}); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}
	seen := make(map[uint64][]byte)
	for id, data := range db.All() {
		if _, dup := seen[id]; dup {
			t.Fatalf("All yielded id %d twice", id)
		}
		seen[id] = data
	}
	if len(seen) != 50 {
		t.Fatalf("All yielded %d records, want 50", len(seen))
	}
	for id, data := range seen {
		if len(data) != 1 || data[0] != byte(id) {
			t.Fatalf("All yielded %v for id %d", data, id)
		}
	}
}

func TestReopenPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")

	db := openTempDB(t, DBConfig{Path: path, RecordSize: 256})
	const n = 500
	for i := 0; i < n; i++ {
		if _, err := db.Put(uint64(i), nameOf(uint64(i)), []byte(fmt.Sprintf("data-%d", i))); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}
	if _, err := db.DeleteByID(42); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db2 := openTempDB(t, DBConfig{Path: path, RecordSize: 256})
	defer db2.Close()
	if db2.Len() != n-1 {
		t.Fatalf("Len after reopen = %d, want %d", db2.Len(), n-1)
	}
	for i := 0; i < n; i++ {
		data, found, err := db2.GetByID(uint64(i))
		want := i != 42
		if err != nil || found != want {
			t.Fatalf("GetByID(%d) = (%v, %v), want found=%v", i, found, err, want)
		}
		if want && string(data) != fmt.Sprintf("data-%d", i) {
			t.Fatalf("GetByID(%d) = %q", i, data)
		}
		if data, found, _ := db2.GetByName(nameOf(uint64(i))); found != want {
			t.Fatalf("GetByName(%d) found=%v, want %v", i, found, want)
		} else if want && string(data) != fmt.Sprintf("data-%d", i) {
			t.Fatalf("GetByName(%d) = %q", i, data)
		}
	}
	checkInvariants(t, db2)
}

func TestReopenMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	db := openTempDB(t, DBConfig{Path: path, RecordSize: 256})
	if _, err := db.Put(1, nk("ada"), []byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	eq, ord := testConfigs()

	// A different RecordSize is a layout mismatch.
	if _, err := OpenDB[uint64, [16]byte](DBConfig{Path: path, RecordSize: 512}, eq, ord); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("reopen with another RecordSize = %v, want ErrCorrupt", err)
	}

	// Different index names belong to another database.
	badEq := eq
	badEq.Name = "by-key"
	if _, err := OpenDB[uint64, [16]byte](DBConfig{Path: path, RecordSize: 256}, badEq, ord); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("reopen with another eq name = %v, want ErrCorrupt", err)
	}
	badOrd := ord
	badOrd.Name = "by-title"
	if _, err := OpenDB[uint64, [16]byte](DBConfig{Path: path, RecordSize: 256}, eq, badOrd); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("reopen with another ord name = %v, want ErrCorrupt", err)
	}

	// A record file that is not one is rejected: garbage magic.
	junk := filepath.Join(dir, "junk")
	if err := os.WriteFile(junk+".rec", bytes.Repeat([]byte{0xA5}, 700), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDB[uint64, [16]byte](DBConfig{Path: junk}, eq, ord); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("open over a junk record file = %v, want ErrCorrupt (bad magic)", err)
	}
}

func TestOpenDBErrors(t *testing.T) {
	eq, ord := testConfigs()

	if _, err := OpenDB[uint64, [16]byte](DBConfig{}, eq, ord); err == nil {
		t.Fatal("OpenDB with an empty path should fail")
	}

	// A record size too small for the two key encodings.
	if _, err := OpenDB[uint64, [16]byte](DBConfig{Path: filepath.Join(t.TempDir(), "db"), RecordSize: 30}, eq, ord); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("RecordSize 30 = %v, want ErrRecordTooLarge (needs 13+8+16+1)", err)
	}

	// The underlying packages' programmer-error panics propagate.
	expectPanic(t, "nil EncodeKey", "nil EncodeKey", func() {
		badEq := eq
		badEq.EncodeKey = nil
		_, _ = OpenDB[uint64, [16]byte](DBConfig{Path: filepath.Join(t.TempDir(), "db")}, badEq, ord)
	})
	expectPanic(t, "nil Compare", "nil Compare", func() {
		badOrd := ord
		badOrd.Compare = nil
		_, _ = OpenDB[uint64, [16]byte](DBConfig{Path: filepath.Join(t.TempDir(), "db")}, eq, badOrd)
	})
}

func TestSyncDurability(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	db := openTempDB(t, DBConfig{Path: path})
	if _, err := db.Put(1, nk("ada"), []byte("durable")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := db.Put(2, nk("grace"), []byte("durable too")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db2 := openTempDB(t, DBConfig{Path: path})
	defer db2.Close()
	for _, id := range []uint64{1, 2} {
		if _, found, _ := db2.GetByID(id); !found {
			t.Fatalf("id %d missing after Sync+Close", id)
		}
	}
}

func TestEmptyDB(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	defer db.Close()

	if db.Len() != 0 {
		t.Fatalf("Len = %d on an empty database", db.Len())
	}
	if _, found, _ := db.GetByID(1); found {
		t.Fatal("GetByID found something in an empty database")
	}
	count := 0
	for range db.All() {
		count++
	}
	for range db.RangeByName(nk("a"), nk("z")) {
		count++
	}
	if count != 0 {
		t.Fatalf("empty database iterated %d records", count)
	}
}

func TestErrClosed(t *testing.T) {
	db := openTempDB(t, DBConfig{})
	if _, err := db.Put(1, nk("ada"), nil); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := db.Close(); !errors.Is(err, ErrClosed) {
		t.Fatalf("second Close = %v, want ErrClosed", err)
	}
	if err := db.Sync(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Sync = %v, want ErrClosed", err)
	}
	if _, err := db.Put(2, nk("grace"), nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("Put = %v, want ErrClosed", err)
	}
	if _, _, err := db.GetByID(1); !errors.Is(err, ErrClosed) {
		t.Fatalf("GetByID = %v, want ErrClosed", err)
	}
	if _, _, err := db.GetByName(nk("ada")); !errors.Is(err, ErrClosed) {
		t.Fatalf("GetByName = %v, want ErrClosed", err)
	}
	if ok, err := db.DeleteByID(1); ok || !errors.Is(err, ErrClosed) {
		t.Fatalf("DeleteByID = (%v, %v), want (false, ErrClosed)", ok, err)
	}
	if ok, err := db.DeleteByName(nk("ada")); ok || !errors.Is(err, ErrClosed) {
		t.Fatalf("DeleteByName = (%v, %v), want (false, ErrClosed)", ok, err)
	}
	if db.Len() != 0 {
		t.Fatalf("Len = %d on a closed database, want 0", db.Len())
	}
	if db.MaxData() <= 0 {
		t.Fatal("MaxData is a layout constant; it stays valid after Close")
	}
	count := 0
	for range db.All() {
		count++
	}
	for range db.RangeByName(nk("a"), nk("z")) {
		count++
	}
	if count != 0 {
		t.Fatal("a closed database iterated records")
	}
}

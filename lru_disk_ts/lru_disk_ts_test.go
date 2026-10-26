package lru_disk_ts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// benc/bdec: a []byte codec (the identity), the common case.
func benc(b []byte) []byte { return b }

func bdec(b []byte) ([]byte, error) { return b, nil }

func newTest[V any](t *testing.T, capacity int, enc func(V) []byte, dec func([]byte) (V, error)) *LruDisk[V] {
	t.Helper()
	c, err := NewLruDisk[V](capacity, t.TempDir(), enc, dec)
	if err != nil {
		t.Fatalf("NewLruDisk: %v", err)
	}
	return c
}

// newTestMem is newTest with the memory tier enabled at memCapacity.
func newTestMem[V any](t *testing.T, capacity, memCapacity int, enc func(V) []byte, dec func([]byte) (V, error)) *LruDisk[V] {
	t.Helper()
	c, err := NewLruDisk[V](capacity, t.TempDir(), enc, dec, WithMemCapacity(memCapacity))
	if err != nil {
		t.Fatalf("NewLruDisk: %v", err)
	}
	return c
}

// countLdc counts this package's entry files in dir.
func countLdc(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, de := range entries {
		if !de.IsDir() && filepath.Ext(de.Name()) == entryFileSuffix {
			n++
		}
	}
	return n
}

func TestPutGetPeek(t *testing.T) {
	c := newTest[[]byte](t, 4, benc, bdec)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("empty cache has a hit")
	}
	if err := c.Put("k", []byte("value")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if v, ok := c.Get("k"); !ok || !bytes.Equal(v, []byte("value")) {
		t.Fatalf("Get = %q %v", v, ok)
	}
	// Peek does not change recency: peek k, then force an eviction — k
	// (oldest by real use) must be the one evicted.
	if v, ok := c.Peek("k"); !ok || !bytes.Equal(v, []byte("value")) {
		t.Fatalf("Peek = %q %v", v, ok)
	}
	if err := c.Put("k2", []byte("v2")); err != nil {
		t.Fatal(err)
	}
	c.Get("k") // k is now MRU; k2 is the LRU entry
	if key, popped, ok := c.PopOldest(); !ok || key != "k2" || string(popped) != "v2" {
		t.Fatalf("PopOldest = %q %q %v", key, popped, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d", c.Len())
	}
	if _, ok := c.Get("k2"); ok {
		t.Fatal("k2 should have been evicted as the LRU entry")
	}
}

func TestEvictionDeletesFiles(t *testing.T) {
	c := newTest[[]byte](t, 2, benc, bdec)
	for i := 0; i < 3; i++ {
		if err := c.Put(fmt.Sprintf("k%d", i), []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d", c.Len())
	}
	if n := countLdc(t, c.dir); n != 2 {
		t.Fatalf("%d files remain", n)
	}
	if _, ok := c.Get("k0"); ok {
		t.Fatal("k0 was evicted")
	}
}

func TestRestartReindex(t *testing.T) {
	dir := t.TempDir()
	c1, err := NewLruDisk[[]byte](3, dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := c1.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	// a "restart": a fresh cache over the same directory
	c2, err := NewLruDisk[[]byte](3, dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 3 {
		t.Fatalf("restart lost entries: Len = %d", c2.Len())
	}
	for i := 0; i < 3; i++ {
		if v, ok := c2.Get(fmt.Sprintf("k%d", i)); !ok || string(v) != fmt.Sprintf("v%d", i) {
			t.Fatalf("restart Get k%d = %q %v", i, v, ok)
		}
	}
}

func TestRestartCapacityEviction(t *testing.T) {
	dir := t.TempDir()
	c1, err := NewLruDisk[[]byte](2, dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := c1.Put(fmt.Sprintf("k%d", i), []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
		// reindex recency comes from file mtime — space the puts so the
		// order is real (ties are arbitrary by contract)
		time.Sleep(2 * time.Millisecond)
	}
	// k0 was evicted before the restart; reopening at capacity 1 must
	// evict the two older leftovers down to the newest (k2).
	c2, err := NewLruDisk[[]byte](1, dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 1 {
		t.Fatalf("Len = %d", c2.Len())
	}
	if _, ok := c2.Get("k2"); !ok {
		t.Fatal("newest entry lost in reindex")
	}
}

func TestDeleteAndClear(t *testing.T) {
	c := newTest[[]byte](t, 4, benc, bdec)
	if err := c.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	if !c.Delete("a") {
		t.Fatal("Delete of a present key")
	}
	if c.Delete("a") {
		t.Fatal("Delete of an absent key returned true")
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d", c.Len())
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 0 {
		t.Fatalf("Len after Clear = %d", c.Len())
	}
	if n := countLdc(t, c.dir); n != 0 {
		t.Fatalf("%d files survive Clear", n)
	}
}

// tieredClearClearsMem: Clear empties the memory tier too, so a later
// Get cannot serve a value whose entry is gone.
func TestTieredClearEmptiesMem(t *testing.T) {
	c := newTestMem[[]byte](t, 4, 4, benc, bdec)
	if err := c.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("a"); !ok || c.MemLen() != 1 {
		t.Fatalf("Get/MemLen = %v/%d", ok, c.MemLen())
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if c.MemLen() != 0 {
		t.Fatalf("MemLen after Clear = %d", c.MemLen())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("cleared entry still served")
	}
}

func TestTornFileSelfHeals(t *testing.T) {
	c := newTest[[]byte](t, 4, benc, bdec)
	if err := c.Put("k", []byte("good")); err != nil {
		t.Fatal(err)
	}
	// corrupt the file behind the cache's back
	path := fileFor(c.dir, "k")
	if err := os.WriteFile(path, []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	// force a re-read: a fresh cache over the directory reads lazily
	c2, err := NewLruDisk[[]byte](4, c.dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c2.Get("k"); ok {
		t.Fatal("torn entry read as a hit")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("torn file not removed")
	}
}

// tieredTornFile: self-heal runs on the disk-read path — a value still
// resident in the memory tier is served without touching the (torn)
// file; a disk-only entry with a torn file is a miss and is removed.
func TestTieredTornFile(t *testing.T) {
	c := newTestMem[[]byte](t, 4, 1, benc, bdec)
	if err := c.Put("hot", []byte("hot-value")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("cold", []byte("cold-value")); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("hot"); !ok {
		t.Fatal("Get hot")
	} // mem = {hot}; cold is disk-only
	if err := os.WriteFile(fileFor(c.dir, "hot"), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Peek("hot"); !ok || string(v) != "hot-value" {
		t.Fatalf("resident value not served over a torn file: %q %v", v, ok)
	}
	if err := os.WriteFile(fileFor(c.dir, "cold"), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("cold"); ok {
		t.Fatal("torn disk-only entry read as a hit")
	}
	if _, err := os.Stat(fileFor(c.dir, "cold")); !os.IsNotExist(err) {
		t.Fatal("torn file not removed")
	}
}

func TestForeignFilesIgnored(t *testing.T) {
	c := newTest[[]byte](t, 4, benc, bdec)
	if err := os.WriteFile(filepath.Join(c.dir, "foreign.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.dir, "wrong.ldc"), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	c2, err := NewLruDisk[[]byte](4, c.dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 0 {
		t.Fatalf("foreign files indexed: Len = %d", c2.Len())
	}
	if _, err := os.Stat(filepath.Join(c.dir, "foreign.txt")); err != nil {
		t.Fatal("foreign file touched")
	}
}

func TestOverwriteSameKey(t *testing.T) {
	dir := t.TempDir()
	c, err := NewLruDisk[[]byte](2, dir, benc, bdec)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put("k", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("k", []byte("second")); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d", c.Len())
	}
	if v, ok := c.Get("k"); !ok || string(v) != "second" {
		t.Fatalf("Get = %q", v)
	}
	c2, _ := NewLruDisk[[]byte](2, dir, benc, bdec)
	if v, ok := c2.Get("k"); !ok || string(v) != "second" {
		t.Fatalf("restart Get = %q (overwrite not durable)", v)
	}
}

// tieredOverwrite: an overwrite replaces both the file and the resident
// value; the restart reads the new payload.
func TestTieredOverwrite(t *testing.T) {
	dir := t.TempDir()
	c, err := NewLruDisk[[]byte](2, dir, benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put("k", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("k", []byte("second")); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 1 || c.MemLen() != 1 {
		t.Fatalf("Len/MemLen = %d/%d", c.Len(), c.MemLen())
	}
	if v, ok := c.Get("k"); !ok || string(v) != "second" {
		t.Fatalf("Get = %q (stale resident value)", v)
	}
	c2, _ := NewLruDisk[[]byte](2, dir, benc, bdec, WithMemCapacity(2))
	if v, ok := c2.Get("k"); !ok || string(v) != "second" {
		t.Fatalf("restart Get = %q (overwrite not durable)", v)
	}
}

// memoryTier: the fast path serves, promotes, and demotes without ever
// losing an entry — demotion keeps the file.
func TestMemoryTier(t *testing.T) {
	c := newTestMem[[]byte](t, 4, 2, benc, bdec)
	for _, k := range []string{"a", "b", "c", "d"} {
		if err := c.Put(k, []byte("v-"+k)); err != nil {
			t.Fatal(err)
		}
	}
	if c.Len() != 4 || c.MemLen() != 2 {
		t.Fatalf("Len/MemLen = %d/%d", c.Len(), c.MemLen())
	}
	// churn: every key stays retrievable, the tier never exceeds its
	// bound, and tier churn deletes no files
	keys := []string{"a", "b", "c", "d"}
	for i := 0; i < 100; i++ {
		k := keys[i%4]
		want := "v-" + k
		if v, ok := c.Get(k); !ok || string(v) != want {
			t.Fatalf("Get %s = %q %v (want %q)", k, v, ok, want)
		}
		if c.MemLen() > 2 {
			t.Fatalf("MemLen = %d over the tier bound", c.MemLen())
		}
		if c.Len() != 4 {
			t.Fatalf("Len = %d", c.Len())
		}
	}
	if n := countLdc(t, c.dir); n != 4 {
		t.Fatalf("%d files after tier churn", n)
	}
}

// memEvictionKeepsFiles: the point of the tier — a demoted value is a
// slower hit, not a miss.
func TestMemEvictionKeepsFiles(t *testing.T) {
	c := newTestMem[[]byte](t, 3, 1, benc, bdec)
	for i := 0; i < 3; i++ {
		if err := c.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if c.MemLen() != 1 {
		t.Fatalf("MemLen = %d", c.MemLen())
	}
	if _, ok := c.Get("k0"); !ok { // resident: k0
		t.Fatal("Get k0")
	}
	if _, ok := c.Get("k1"); !ok { // k1 resident now, k0 demoted
		t.Fatal("Get k1")
	}
	if c.MemLen() != 1 {
		t.Fatalf("MemLen = %d", c.MemLen())
	}
	// k0 was demoted but its file stayed: a hit, via the disk
	if v, ok := c.Get("k0"); !ok || string(v) != "v0" {
		t.Fatalf("demoted Get k0 = %q %v", v, ok)
	}
	if n := countLdc(t, c.dir); n != 3 {
		t.Fatalf("%d files after demotion (demotion must not delete)", n)
	}
	// and the demote/promote cycle again, the other way
	if v, ok := c.Get("k1"); !ok || string(v) != "v1" {
		t.Fatalf("demoted Get k1 = %q %v", v, ok)
	}
	if c.Len() != 3 || c.MemLen() != 1 {
		t.Fatalf("Len/MemLen = %d/%d", c.Len(), c.MemLen())
	}
}

// tieredPopOldestDecodesFromDisk: after a restart the entries are
// disk-only (the memory tier starts empty), so PopOldest must decode
// the file BEFORE removing it — the zero value with ok=true is only
// for an undecodable entry.
func TestTieredPopOldestDecodesFromDisk(t *testing.T) {
	dir := t.TempDir()
	c1, err := NewLruDisk[[]byte](2, dir, benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	if err := c1.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := c1.Put("b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	c2, err := NewLruDisk[[]byte](2, dir, benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	if c2.MemLen() != 0 {
		t.Fatalf("restart pre-warmed the tier: MemLen = %d", c2.MemLen())
	}
	if key, v, ok := c2.PopOldest(); !ok || key != "a" || string(v) != "1" {
		t.Fatalf("PopOldest = %q %q %v (decode-before-remove broken)", key, v, ok)
	}
	if key, v, ok := c2.PopOldest(); !ok || key != "b" || string(v) != "2" {
		t.Fatalf("PopOldest = %q %q %v", key, v, ok)
	}
	if _, _, ok := c2.PopOldest(); ok {
		t.Fatal("PopOldest on an empty cache")
	}
}

// tieredPopOldestResident: when the victim is memory-resident its value
// comes from the tier (no file read at all).
func TestTieredPopOldestResident(t *testing.T) {
	c := newTestMem[[]byte](t, 2, 2, benc, bdec)
	if err := c.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	// make both files unreadable: only the tier can supply the values
	if err := os.WriteFile(fileFor(c.dir, "a"), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileFor(c.dir, "b"), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if key, v, ok := c.PopOldest(); !ok || key != "a" || string(v) != "1" {
		t.Fatalf("PopOldest = %q %q %v", key, v, ok)
	}
	if c.MemLen() != 1 {
		t.Fatalf("MemLen = %d", c.MemLen())
	}
}

// tieredOverlap: mem capacity >= disk capacity — every disk eviction
// must also drop the victim from the (fully resident) memory tier.
func TestTieredOverlap(t *testing.T) {
	c := newTestMem[[]byte](t, 2, 2, benc, bdec)
	for i := 0; i < 3; i++ {
		if err := c.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if c.Len() != 2 || c.MemLen() != 2 {
		t.Fatalf("Len/MemLen = %d/%d", c.Len(), c.MemLen())
	}
	if _, ok := c.Get("k0"); ok {
		t.Fatal("k0 evicted from disk but still served — tier kept a ghost")
	}
	if c.MemLen() != 2 {
		t.Fatalf("MemLen = %d (ghost not dropped from the tier)", c.MemLen())
	}
	if v, ok := c.Get("k1"); !ok || string(v) != "v1" {
		t.Fatalf("Get k1 = %q %v", v, ok)
	}
	if n := countLdc(t, c.dir); n != 2 {
		t.Fatalf("%d files", n)
	}
}

// tieredRestart: the tier starts empty after a restart and refills
// lazily on first access.
func TestTieredRestart(t *testing.T) {
	dir := t.TempDir()
	c1, err := NewLruDisk[[]byte](3, dir, benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := c1.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	c2, err := NewLruDisk[[]byte](3, dir, benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 3 || c2.MemLen() != 0 {
		t.Fatalf("Len/MemLen after restart = %d/%d", c2.Len(), c2.MemLen())
	}
	for i := 0; i < 3; i++ {
		if v, ok := c2.Get(fmt.Sprintf("k%d", i)); !ok || string(v) != fmt.Sprintf("v%d", i) {
			t.Fatalf("restart Get k%d = %q %v", i, v, ok)
		}
		if c2.MemLen() > 2 {
			t.Fatalf("MemLen = %d over the tier bound", c2.MemLen())
		}
	}
	if c2.MemLen() != 2 {
		t.Fatalf("MemLen = %d (tier did not fill lazily)", c2.MemLen())
	}
}

// peekNeverPromotes: Peek serves either tier without file I/O when the
// value is resident, and without promoting when it is not.
func TestPeekTierSemantics(t *testing.T) {
	c := newTestMem[[]byte](t, 4, 2, benc, bdec)
	for _, k := range []string{"a", "b", "c", "d"} {
		if err := c.Put(k, []byte("v-"+k)); err != nil {
			t.Fatal(err)
		}
	}
	// resident peek: delete the file behind the cache's back — the
	// resident value must still be served (no file I/O on this path)
	if err := os.Remove(fileFor(c.dir, "d")); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Peek("d"); !ok || string(v) != "v-d" {
		t.Fatalf("resident Peek = %q %v", v, ok)
	}
	// disk-only peek: correct value, tier unchanged
	if v, ok := c.Peek("a"); !ok || string(v) != "v-a" {
		t.Fatalf("disk-only Peek = %q %v", v, ok)
	}
	if c.MemLen() != 2 {
		t.Fatalf("MemLen = %d (Peek must not promote)", c.MemLen())
	}
}

// memOptionDefaults: the tier is OFF by default; WithMemCapacity
// bounds below 1 leave it off, no panic.
func TestMemOptionDefaults(t *testing.T) {
	c := newTest[[]byte](t, 2, benc, bdec)
	if c.MemCapacity() != 0 || c.MemLen() != 0 {
		t.Fatalf("default tier is on: MemCapacity/MemLen = %d/%d", c.MemCapacity(), c.MemLen())
	}
	if err := c.Put("k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("k"); !ok {
		t.Fatal("Get without the tier")
	}
	if c.MemLen() != 0 {
		t.Fatalf("values retained without the tier: MemLen = %d", c.MemLen())
	}
	for _, n := range []int{0, -3} {
		c, err := NewLruDisk[[]byte](2, t.TempDir(), benc, bdec, WithMemCapacity(n))
		if err != nil {
			t.Fatal(err)
		}
		if c.MemCapacity() != 0 {
			t.Fatalf("WithMemCapacity(%d) enabled the tier", n)
		}
	}
	c, err := NewLruDisk[[]byte](2, t.TempDir(), benc, bdec, WithMemCapacity(2))
	if err != nil {
		t.Fatal(err)
	}
	if c.MemCapacity() != 2 {
		t.Fatalf("WithMemCapacity(2): MemCapacity = %d", c.MemCapacity())
	}
}

func TestNilAndZeroTolerance(t *testing.T) {
	var c *LruDisk[[]byte]
	if _, ok := c.Get("k"); ok {
		t.Fatal("nil cache hit")
	}
	if c.Len() != 0 || c.Capacity() != 0 || c.MemLen() != 0 || c.MemCapacity() != 0 {
		t.Fatal("nil cache has size")
	}
	c.Lock() // must not panic
	c.Unlock()
	if err := c.Put("k", nil); err == nil {
		t.Fatal("nil cache Put succeeded")
	}
	var zero LruDisk[[]byte]
	if _, ok := zero.Peek("k"); ok {
		t.Fatal("zero-value cache hit")
	}
	if zero.MemLen() != 0 || zero.MemCapacity() != 0 {
		t.Fatal("zero-value cache has a tier")
	}
	if zero.Delete("k") {
		t.Fatal("zero-value cache Delete")
	}
}

func TestConstructorErrors(t *testing.T) {
	if func() (ok bool) {
		defer func() { ok = recover() != nil }()
		_, _ = NewLruDisk[[]byte](0, t.TempDir(), benc, bdec)
		return
	}() != true {
		t.Fatal("capacity 0 did not panic")
	}
	if _, err := NewLruDisk[[]byte](2, filepath.Join(t.TempDir(), "f", "..", "x", "\x00bad"), benc, bdec); err == nil {
		t.Fatal("bad dir did not error")
	}
	if _, err := NewLruDisk[[]byte](2, t.TempDir(), nil, bdec); err == nil {
		t.Fatal("nil encode did not error")
	}
}

func TestCompoundLock(t *testing.T) {
	c := newTestMem[[]byte](t, 2, 1, benc, bdec)
	if err := c.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	c.Lock()
	if v, ok := c.NlGet("a"); !ok || string(v) != "1" {
		t.Fatalf("NlGet = %q %v", v, ok)
	}
	if c.NlLen() != 2 || c.NlCapacity() != 2 || c.NlMemCapacity() != 1 || c.NlMemLen() > 1 {
		t.Fatalf("NlLen/NlCapacity/NlMemLen/NlMemCapacity = %d/%d/%d/%d",
			c.NlLen(), c.NlCapacity(), c.NlMemLen(), c.NlMemCapacity())
	}
	if !c.NlDelete("a") {
		t.Fatal("NlDelete")
	}
	c.Unlock()
	if c.Len() != 1 {
		t.Fatalf("Len = %d", c.Len())
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := newTest[[]byte](t, 8, benc, bdec)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				k := fmt.Sprintf("k%d", i%16)
				if err := c.Put(k, []byte(fmt.Sprintf("v%d", i))); err != nil {
					t.Error(err)
					return
				}
				c.Get(k)
				c.Peek(k)
				if i%7 == 0 {
					c.Delete(k)
				}
			}
		}(g)
	}
	wg.Wait()
	if c.Len() > 8 {
		t.Fatalf("Len = %d over capacity", c.Len())
	}
}

// concurrentTiered: the two-tier invariants hold under contention —
// Len never over the disk bound, MemLen never over the tier bound.
func TestConcurrentTiered(t *testing.T) {
	c := newTestMem[[]byte](t, 8, 4, benc, bdec)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				k := fmt.Sprintf("k%d", i%16)
				if err := c.Put(k, []byte(fmt.Sprintf("v%d", i))); err != nil {
					t.Error(err)
					return
				}
				c.Get(k)
				c.Peek(k)
				if i%7 == 0 {
					c.Delete(k)
				}
			}
		}(g)
	}
	wg.Wait()
	if c.Len() > 8 {
		t.Fatalf("Len = %d over capacity", c.Len())
	}
	if c.MemLen() > 4 {
		t.Fatalf("MemLen = %d over the tier bound", c.MemLen())
	}
	if c.MemLen() > c.Len() {
		t.Fatalf("MemLen = %d over Len = %d (tier is not a subset)", c.MemLen(), c.Len())
	}
}

// overwriteAtCapacity: an overwrite replaces in place and never evicts
// (the lru.Put contract — the old code evicted the LRU entry to "make
// room" for a key that needed none).
func TestOverwriteAtCapacity(t *testing.T) {
	for _, memCap := range []int{0, 2} {
		c := newTestMem[[]byte](t, 2, memCap, benc, bdec)
		if err := c.Put("a", []byte("1")); err != nil {
			t.Fatal(err)
		}
		if err := c.Put("b", []byte("2")); err != nil {
			t.Fatal(err)
		}
		c.Get("a") // a is MRU; b is the LRU entry — but no room is needed
		if err := c.Put("a", []byte("3")); err != nil {
			t.Fatal(err)
		}
		if _, ok := c.Get("b"); !ok {
			t.Fatalf("mem=%d: the overwrite evicted the LRU entry", memCap)
		}
		if v, ok := c.Get("a"); !ok || string(v) != "3" {
			t.Fatalf("mem=%d: Get a = %q %v", memCap, v, ok)
		}
		if c.Len() != 2 {
			t.Fatalf("mem=%d: Len = %d", memCap, c.Len())
		}
		if n := countLdc(t, c.dir); n != 2 {
			t.Fatalf("mem=%d: %d files", memCap, n)
		}
	}
}

func TestStringCodec(t *testing.T) {
	// a non-bytes V: the codec carries it
	enc := func(s string) []byte { return []byte(s) }
	dec := func(b []byte) (string, error) { return string(b), nil }
	dir := t.TempDir()
	c, err := NewLruDisk[string](3, dir, enc, dec)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put("k", "hello"); err != nil {
		t.Fatal(err)
	}
	c2, _ := NewLruDisk[string](3, dir, enc, dec)
	if v, ok := c2.Get("k"); !ok || v != "hello" {
		t.Fatalf("restart Get = %q %v", v, ok)
	}
}

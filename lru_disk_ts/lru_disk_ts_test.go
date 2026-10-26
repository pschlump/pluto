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
	if _, popped, ok := c.PopOldest(); !ok || popped == nil {
		t.Fatalf("PopOldest ok=%v", ok)
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
	// the file for the evicted key must be gone: count *.ldc files
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("%d files remain", len(entries))
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
	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
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
	entries, _ := os.ReadDir(c.dir)
	if len(entries) != 0 {
		t.Fatalf("%d files survive Clear", len(entries))
	}
}

func TestTornFileSelfHeals(t *testing.T) {
	c := newTest[[]byte](t, 4, benc, bdec)
	c.Put("k", []byte("good"))
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
	c.Put("k", []byte("first"))
	c.Put("k", []byte("second"))
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

func TestNilAndZeroTolerance(t *testing.T) {
	var c *LruDisk[[]byte]
	if _, ok := c.Get("k"); ok {
		t.Fatal("nil cache hit")
	}
	if c.Len() != 0 || c.Capacity() != 0 {
		t.Fatal("nil cache has size")
	}
	c.Lock() // must not panic
	c.Unlock()
	if err := c.Put("k", nil); err == nil {
		t.Fatal("nil cache Put succeeded")
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
	c := newTest[[]byte](t, 2, benc, bdec)
	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	c.Lock()
	if v, ok := c.NlGet("a"); !ok || string(v) != "1" {
		t.Fatalf("NlGet = %q %v", v, ok)
	}
	if c.NlLen() != 2 || c.NlCapacity() != 2 {
		t.Fatalf("NlLen/NlCapacity = %d/%d", c.NlLen(), c.NlCapacity())
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
				c.Put(k, []byte(fmt.Sprintf("v%d", i)))
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

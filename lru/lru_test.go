/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// mruOrder returns the keys from most to least recently used.
func mruOrder[K comparable, V any](c *Lru[K, V]) []K {
	keys := []K{}
	for k := range c.All() {
		keys = append(keys, k)
	}
	return keys
}

// Get, Put and eviction ordering: a Get hit protects an entry, the
// least recently used entry is evicted first.
func TestGetPutEvict(t *testing.T) {
	c := NewLru[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"c", "b", "a"}) {
		t.Fatalf("order after puts = %v, want [c b a]", got)
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = (%v, %v), want (1, true)", v, ok)
	}
	// "a" is now most-recently-used, so "b" is the eviction victim.
	c.Put("d", 4)
	if _, ok := c.Get("b"); ok {
		t.Error("Get(b) after eviction found b, want not-found")
	}
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"d", "a", "c"}) {
		t.Fatalf("order after eviction = %v, want [d a c]", got)
	}
	if c.Len() != 3 || c.Capacity() != 3 {
		t.Errorf("Len/Capacity = %d/%d, want 3/3", c.Len(), c.Capacity())
	}
}

// Put on an existing key replaces the value and marks the entry
// most-recently-used — without evicting anyone.
func TestPutUpdateMarksMRU(t *testing.T) {
	c := NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 10) // update: "a" is MRU now, no eviction
	if c.Len() != 2 {
		t.Fatalf("Len after update = %d, want 2", c.Len())
	}
	if v, _ := c.Get("a"); v != 10 {
		t.Errorf("Get(a) = %v after update, want 10", v)
	}
	c.Put("c", 3) // evicts "b", the LRU
	if _, ok := c.Get("b"); ok {
		t.Error("Get(b) after eviction found b, want not-found")
	}
}

// Peek does not reorder; Get does — the Get-pinned entry survives the
// eviction that the Peek-ed entry falls to.
func TestPeekDoesNotReorder(t *testing.T) {
	c := NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2) // order: b, a
	if _, ok := c.Peek("a"); !ok {
		t.Fatal("Peek(a) not found")
	}
	c.Put("c", 3) // "a" is still LRU: evicted
	if _, ok := c.Get("a"); ok {
		t.Error("Peek reordered: a survived its own eviction")
	}

	// Same setup, but a Get pins "a" against the eviction.
	c = NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // order: a, b
	c.Put("c", 3) // evicts "b"
	if _, ok := c.Get("a"); !ok {
		t.Error("Get-pinned a was evicted, want it pinned")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("Get(b) found b; b was the LRU and should be evicted")
	}
}

// A vetoing callback protects entries: eviction skips vetoed entries
// and takes the next-older one.
func TestVetoSkipsProtected(t *testing.T) {
	protected := map[string]bool{"a": true}
	c := NewLruFunc(2, func(k string, _ int) bool { return !protected[k] })
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // "a" is LRU but vetoed; "b" goes instead
	if _, ok := c.Get("b"); ok {
		t.Error("Get(b) found b; the veto should have redirected the eviction to b")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Error("vetoed entry a was evicted, want it kept")
	}
}

// When every entry is vetoed the cache temporarily EXCEEDS its
// capacity — the soft cap; entries become evictable again when the veto
// lifts, on the next insert.
func TestAllVetoedSoftCap(t *testing.T) {
	vetoAll := true
	c := NewLruFunc(2, func(_ string, _ int) bool { return !vetoAll })
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // nothing evictable: soft cap
	if c.Len() != 3 {
		t.Fatalf("Len with all entries vetoed = %d, want 3 (soft cap)", c.Len())
	}
	vetoAll = false
	c.Put("d", 4) // evicts back down to capacity first: a and b go
	if c.Len() != 2 {
		t.Fatalf("Len after veto lifted = %d, want 2", c.Len())
	}
	if got := mruOrder(c); !reflect.DeepEqual(got, []string{"d", "c"}) {
		t.Fatalf("order after veto lifted = %v, want [d c]", got)
	}
}

func TestDelete(t *testing.T) {
	c := NewLru[int, string](3)
	c.Put(1, "one")
	c.Put(2, "two")
	if !c.Delete(1) {
		t.Error("Delete(1) = false, want true")
	}
	if c.Delete(1) {
		t.Error("second Delete(1) = true, want false")
	}
	if _, ok := c.Get(1); ok {
		t.Error("Get(1) after Delete found 1")
	}
	if c.Len() != 1 {
		t.Errorf("Len after Delete = %d, want 1", c.Len())
	}
}

func TestClear(t *testing.T) {
	c := NewLru[int, int](4)
	for i := 0; i < 4; i++ {
		c.Put(i, i)
	}
	c.Clear()
	if c.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", c.Len())
	}
	if c.Capacity() != 4 {
		t.Errorf("Capacity after Clear = %d, want 4 (kept)", c.Capacity())
	}
	c.Put(9, 9) // still usable, same capacity
	if v, ok := c.Get(9); !ok || v != 9 {
		t.Error("cache unusable after Clear")
	}
}

// Both iterator orders, early exit, and the single-variable range
// yielding the KEY.
func TestIterators(t *testing.T) {
	c := NewLru[int, string](4)
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")

	var fwd []int
	for k := range c.All() { // single variable: the KEY
		fwd = append(fwd, k)
	}
	if !reflect.DeepEqual(fwd, []int{3, 2, 1}) {
		t.Errorf("All keys = %v, want [3 2 1] (MRU first)", fwd)
	}
	var bwd []int
	for k := range c.Backward() {
		bwd = append(bwd, k)
	}
	if !reflect.DeepEqual(bwd, []int{1, 2, 3}) {
		t.Errorf("Backward keys = %v, want [1 2 3] (LRU first)", bwd)
	}
	var pairs []string
	for k, v := range c.All() {
		pairs = append(pairs, fmt.Sprintf("%d=%s", k, v))
		if len(pairs) == 2 {
			break // early exit must not hang or leak
		}
	}
	if !reflect.DeepEqual(pairs, []string{"3=three", "2=two"}) {
		t.Errorf("early-exit pairs = %v, want [3=three 2=two]", pairs)
	}
}

// A nil cache and a zero-value cache tolerate every read: Get/Peek
// report not-found, Delete false, Len/Capacity 0, Clear a no-op, and
// the iterators yield nothing.
func TestNilAndZeroValueTolerated(t *testing.T) {
	var nilCache *Lru[string, int]
	var zeroCache Lru[string, int]
	for _, c := range []*Lru[string, int]{nilCache, &zeroCache} {
		if v, ok := c.Get("x"); ok || v != 0 {
			t.Error("Get on nil/zero cache found something")
		}
		if _, ok := c.Peek("x"); ok {
			t.Error("Peek on nil/zero cache found something")
		}
		if c.Delete("x") {
			t.Error("Delete on nil/zero cache = true")
		}
		if c.Len() != 0 || c.Capacity() != 0 {
			t.Error("Len/Capacity on nil/zero cache != 0")
		}
		c.Clear() // must not panic
		for range c.All() {
			t.Error("All on nil/zero cache yielded")
		}
		for range c.Backward() {
			t.Error("Backward on nil/zero cache yielded")
		}
	}
}

// expectPanicMsg runs fx, requires it to panic, and requires the panic
// message to contain every fragment (the method name and the fix).
func expectPanicMsg(t *testing.T, what string, fx func(), fragments ...string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", what)
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("%s panicked with %v, not a message string", what, r)
			return
		}
		for _, frag := range fragments {
			if !strings.Contains(msg, frag) {
				t.Errorf("%s panic message %q should mention %q", what, msg, frag)
			}
		}
	}()
	fx()
}

// The full panic contract: bad capacity at construction, Put on a nil
// cache, Put on a zero-value cache.
func TestPanics(t *testing.T) {
	expectPanicMsg(t, "NewLru(0)", func() { NewLru[int, int](0) }, "NewLruFunc", "capacity")
	expectPanicMsg(t, "NewLruFunc(-1)", func() { NewLruFunc[int, int](-1, nil) }, "NewLruFunc", "capacity")

	var nilCache *Lru[string, int]
	expectPanicMsg(t, "Put on a nil cache", func() { nilCache.Put("k", 1) }, "Put", "NewLru")

	var zeroCache Lru[string, int]
	expectPanicMsg(t, "Put on a zero-value cache", func() { zeroCache.Put("k", 1) }, "Put", "NewLru")
}

func BenchmarkGetHit(b *testing.B) {
	c := NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1000)
	}
}

func BenchmarkPut(b *testing.B) {
	c := NewLru[int, int](1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Put(i%2000, i) // half updates, half inserts with eviction
	}
}

func BenchmarkGetMiss(b *testing.B) {
	c := NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(1000 + i%1000)
	}
}

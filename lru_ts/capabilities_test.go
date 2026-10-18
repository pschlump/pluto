/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts

import (
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/pluto/lru"
)

// The twin renders exactly what the plain package renders: keys only,
// most recently used first, after the same script.
func TestCapabilitiesStringParity(t *testing.T) {
	plain := lru.NewLru[string, int](8)
	twin := NewLru[string, int](8)
	for _, k := range []string{"a", "b", "c", "d"} {
		plain.Put(k, len(k))
		twin.Put(k, len(k))
	}
	plain.Get("b")
	twin.Get("b")
	plain.Delete("c")
	twin.Delete("c")
	want := plain.String()
	if got := twin.String(); got != want || got != "[b d a]" {
		t.Errorf("twin String = %q, plain String = %q, want both %q", got, want, "[b d a]")
	}
	if got := twin.StringFunc(func(key string) string { return "<" + key + ">" }); got != "[<b> <d> <a>]" {
		t.Errorf("twin StringFunc = %q, want %q", got, "[<b> <d> <a>]")
	}
}

// nil and empty caches render as "[]"; a nil format function too.
func TestCapabilitiesStringNilEmpty(t *testing.T) {
	var nilCache *Lru[string, int]
	if got := nilCache.String(); got != "[]" {
		t.Errorf("nil cache String = %q, want %q", got, "[]")
	}
	if got := nilCache.StringFunc(nil); got != "[]" {
		t.Errorf("nil cache StringFunc(nil) = %q, want %q", got, "[]")
	}
	if got := NewLru[string, int](4).String(); got != "[]" {
		t.Errorf("empty cache String = %q, want %q", got, "[]")
	}
}

// The StringFunc callback runs OUTSIDE the read lock: it may call back
// into the cache, including a mutating method — under the lock that
// would deadlock, so completing this test is the proof.
func TestCapabilitiesStringFuncCallbackUnlocked(t *testing.T) {
	c := NewLru[int, int](8)
	for i := range 4 {
		c.Put(i, i)
	}
	saw := 0
	got := c.StringFunc(func(key int) string {
		c.Put(key+100, key) // a write from inside the callback
		saw = c.Len()
		return fmt.Sprint(key)
	})
	if got != "[3 2 1 0]" {
		t.Errorf("StringFunc = %q, want %q", got, "[3 2 1 0]")
	}
	if saw != 8 {
		t.Errorf("Len seen from callback = %d, want 8", saw)
	}
}

// The race trio for the rendering: concurrent writers, a concurrent
// reader of String, a concurrent StringFunc — run under -race.
func TestCapabilitiesStringRace(t *testing.T) {
	c := NewLru[int, int](64)
	for i := range 32 {
		c.Put(i, i)
	}
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 200 {
				c.Put((g*200+i)%40, i)
				c.Get(i % 32)
				c.Delete((g + i) % 48)
			}
		}(g)
	}
	wg.Add(2)
	stringBytes, funcBytes := 0, 0
	go func() {
		defer wg.Done()
		for range 200 {
			stringBytes += len(c.String()) // always at least "[]" — the point is the race read
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			funcBytes += len(c.StringFunc(func(key int) string { return fmt.Sprint(key) }))
		}
	}()
	wg.Wait()
	if stringBytes < 400 || funcBytes < 400 { // 200 renders x at least "[]" (2 bytes) each
		t.Errorf("render bytes: String %d, StringFunc %d — want at least 400 each", stringBytes, funcBytes)
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import (
	"fmt"
	"testing"
)

// String renders the KEYS, most recently used first — the recency
// order is the rendering, no sorting anywhere.
func TestCapabilitiesString(t *testing.T) {
	c := NewLru[string, int](8)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	if got := c.String(); got != "[c b a]" {
		t.Errorf("String = %q, want %q", got, "[c b a]")
	}
	// A Get promotes: the rendering follows the recency order.
	c.Get("a")
	if got := c.String(); got != "[a c b]" {
		t.Errorf("String after Get(a) = %q, want %q", got, "[a c b]")
	}
	// A Put of a present key promotes too (and keeps the value slot).
	c.Put("b", 20)
	if got := c.String(); got != "[b a c]" {
		t.Errorf("String after Put(b) = %q, want %q", got, "[b a c]")
	}
	// Delete removes the key from the rendering.
	c.Delete("a")
	if got := c.String(); got != "[b c]" {
		t.Errorf("String after Delete(a) = %q, want %q", got, "[b c]")
	}
	// Eviction removes the least recently used key from the rendering.
	small := NewLru[string, int](2)
	small.Put("x", 1)
	small.Put("y", 2)
	small.Put("z", 3) // evicts x
	if got := small.String(); got != "[z y]" {
		t.Errorf("String after eviction = %q, want %q", got, "[z y]")
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
	empty := NewLru[string, int](4)
	if got := empty.String(); got != "[]" {
		t.Errorf("empty cache String = %q, want %q", got, "[]")
	}
	empty.Put("k", 9)
	if got := empty.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want %q", got, "[]")
	}
}

// StringFunc formats each key with the caller's function, in the same
// recency order; String is the fmt.Sprint specialization; the cache is
// a fmt.Stringer.
func TestCapabilitiesStringFunc(t *testing.T) {
	c := NewLru[int, string](8)
	c.Put(1, "one")
	c.Put(2, "two")
	c.Put(3, "three")
	got := c.StringFunc(func(key int) string { return fmt.Sprintf("k%d", key) })
	if got != "[k3 k2 k1]" {
		t.Errorf("StringFunc = %q, want %q", got, "[k3 k2 k1]")
	}
	if got := c.String(); got != "[3 2 1]" {
		t.Errorf("String = %q, want %q", got, "[3 2 1]")
	}
	if got := fmt.Sprintf("%v", c); got != "[3 2 1]" {
		t.Errorf("%%v = %q, want %q (fmt.Stringer)", got, "[3 2 1]")
	}
}

// The rendering is a function of the recency state only: replaying the
// same script produces the same string.
func TestCapabilitiesStringDeterminism(t *testing.T) {
	build := func() *Lru[string, int] {
		c := NewLru[string, int](8)
		for _, k := range []string{"m", "n", "o", "p"} {
			c.Put(k, len(k))
		}
		c.Get("n")
		c.Peek("m") // Peek must NOT promote
		return c
	}
	first, second := build().String(), build().String()
	if first != second || first != "[n p o m]" {
		t.Errorf("determinism: %q vs %q, want both %q", first, second, "[n p o m]")
	}
}

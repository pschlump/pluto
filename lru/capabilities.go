/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import (
	"fmt"
	"strings"
)

// This file holds the one set-convenience capability the
// set-capabilities plan grants the caches (§4.6): String, deferred to
// the plan's last phase because everything else about a cache is
// eviction-order-sensitive and murky.  The rendering follows the tries'
// keys-only rule — the keys ARE the cache's membership, the values are
// payload one call away (All/Peek) — and walks in All's order, most
// recently used first, so the printed sequence is the cache's defining
// artifact: its recency order.  Deterministic with no sorting — the
// order is data, not hash-seed luck.  Nothing here panics; a nil cache
// renders as "[]".

// String renders the cache as its keys joined with single spaces inside
// square brackets, most recently used first: "[k3 k1 k2]".  The values
// are not rendered — the keys are the membership; use All or Peek for
// the values.  A nil or empty cache renders as "[]".  For a custom
// per-key rendering use StringFunc.
// Complexity is O(n).
func (c *Lru[K, V]) String() string {
	return c.StringFunc(func(key K) string { return fmt.Sprint(key) })
}

// StringFunc is String with a caller supplied per-key format function;
// the keys are walked most recently used first (All's order) and the
// formatted strings joined inside square brackets, so the output is the
// recency order.  A nil format function or a nil cache renders as "[]".
// Complexity is O(n).
func (c *Lru[K, V]) StringFunc(format func(key K) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	first := true
	for key := range c.All() {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		sb.WriteString(format(key))
	}
	sb.WriteByte(']')
	return sb.String()
}

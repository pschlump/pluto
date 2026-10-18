/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts

import (
	"fmt"
	"strings"
)

// This file holds the one set-convenience capability the
// set-capabilities plan grants the caches (§4.6): String.  The
// rendering matches the plain package exactly — keys only, most
// recently used first (the snapshot All yields), values one call away
// (All/Peek) — with the twin's standard lock discipline: All snapshots
// under the read lock, so the formatting (and a StringFunc callback)
// runs on the copy, outside the lock.  Nothing here panics; a nil cache
// renders as "[]".

// String renders the cache as its keys joined with single spaces inside
// square brackets, most recently used first: "[k3 k1 k2]".  The values
// are not rendered — the keys are the membership; use All or Peek for
// the values.  A nil or empty cache renders as "[]".  For a custom
// per-key rendering use StringFunc.
// Complexity is O(n) time, O(n) space for the snapshot.
func (c *Lru[K, V]) String() string {
	return c.StringFunc(func(key K) string { return fmt.Sprint(key) })
}

// StringFunc is String with a caller supplied per-key format function;
// the keys are walked most recently used first (All's snapshot order)
// and the formatted strings joined inside square brackets.  The format
// callback runs OUTSIDE the lock, on the snapshot.  A nil format
// function or a nil cache renders as "[]".
// Complexity is O(n) time, O(n) space for the snapshot.
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

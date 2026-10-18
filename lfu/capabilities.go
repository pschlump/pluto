/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import (
	"fmt"

	"github.com/pschlump/pluto/g_lib"
)

// This file holds the one set-convenience capability the
// set-capabilities plan grants the caches (§4.6): String.  An Lfu has
// no natural iteration order — the backing hash_grow table hashes with
// a random per-table seed — so the rendering follows the hash-family
// rule instead of a walk order: the formatted key strings are SORTED,
// making the output deterministic run to run (g_lib.FormatSorted).  The
// keys only are rendered, the tries' keys-only rule: the keys are the
// table's membership, the frequency state is one call away (Counter,
// IdleMinutes).  Nothing here panics; a nil table renders as "[]".

// String renders the table as its keys formatted with fmt.Sprint, the
// formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  Sorting the formatted strings —
// not the keys — is what makes the output deterministic: the table's
// slot order depends on the per-table random hash seed.  The counters
// are not rendered — use Counter for them.  A nil or empty table
// renders as "[]".  For a custom per-key rendering use StringFunc.
// Complexity is O(n log n).
func (l *Lfu[K]) String() string {
	return l.StringFunc(func(key K) string { return fmt.Sprint(key) })
}

// StringFunc is String with a caller supplied per-key format function;
// the formatted strings are sorted ascending and joined inside square
// brackets, so the output is deterministic whatever the table's slot
// order.  A nil format function or a nil table renders as "[]".
// Complexity is O(n log n).
func (l *Lfu[K]) StringFunc(format func(key K) string) string {
	if l == nil || l.tab == nil || format == nil {
		return "[]"
	}
	return g_lib.FormatSorted(l.tab.Slice(), func(e lfuEntry[K]) string {
		return format(e.key)
	})
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts

import (
	"fmt"

	"github.com/pschlump/pluto/g_lib"
)

// This file holds the one set-convenience capability the
// set-capabilities plan grants the caches (§4.6): String, matching the
// plain package exactly: keys only, the formatted strings SORTED (the
// hash-family rule — no natural walk order), counters one call away
// (Counter).  The twin's standard lock discipline applies: the keys are
// snapshotted (copied) under the read lock and the formatting —
// including a StringFunc callback — runs on the copy, outside the lock.
// Sorting is also what makes plain/twin parity hold: the plain table
// walks its hash slots, the twin walks its first-insert key list, and
// both sort, so both render the identical string.  Nothing here
// panics; a nil table renders as "[]".

// String renders the table as its keys formatted with fmt.Sprint, the
// formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  The counters are not rendered —
// use Counter for them.  A nil or empty table renders as "[]".  For a
// custom per-key rendering use StringFunc.
// Complexity is O(n log n).
func (l *Lfu[K]) String() string {
	return l.StringFunc(func(key K) string { return fmt.Sprint(key) })
}

// StringFunc is String with a caller supplied per-key format function;
// the formatted strings are sorted ascending and joined inside square
// brackets, so the output is deterministic and identical to the plain
// package's.  The format callback runs OUTSIDE the lock, on the
// snapshot.  A nil format function or a nil table renders as "[]".
// Complexity is O(n log n).
func (l *Lfu[K]) StringFunc(format func(key K) string) string {
	if l == nil || format == nil {
		return "[]"
	}
	l.lock.RLock()
	keys := make([]K, len(l.keys))
	copy(keys, l.keys)
	l.lock.RUnlock()
	return g_lib.FormatSorted(keys, format)
}

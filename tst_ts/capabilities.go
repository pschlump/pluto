/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tst_ts

import "strings"

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for the string-keyed tries: the tries ARE pluto's string
// sets, so they get set ergonomics directly.  What is here: key/value
// materialization, rendering, deep copy, keys-only equality and the key
// batches.  Everything is additive and mirrors tst's capabilities.go
// method for method; none of the new methods panics — they are total
// functions over the nil trie and the zero value like every read.
// Ordering comes free: the in-order walk is ascending key order, so
// Keys, Values, String and StringFunc are deterministic with no
// sorting.  Batch insert is deliberately absent (a map-shaped batch is
// InsertMap territory — callers loop; the surface stays small), and
// Contains already exists, so it is not repeated here.  The empty key
// keeps its package-wide rejection: "" is never a key, so it is never
// contained and never deleted by these methods either.
//
// Lock discipline: the reads snapshot under the read lock (Keys and
// Values collect eagerly, like KeysWithPrefix and All), the
// compute-after-snapshot methods (String/StringFunc/Equal) snapshot
// under the read lock and run the caller format function without any
// lock held — a callback may safely call back into the trie — and the
// batches (ContainsSlice/ContainsAny under one read-lock hold,
// DeleteSlice under one write-lock hold) describe one consistent
// snapshot; a DeleteSlice is atomic: an interleaved reader sees none or
// all of it, never part.  Equal snapshots one side and probes the other
// key by key, so it never holds two locks — not even when two
// goroutines compare the same pair of tries in opposite directions.
// No new Nl methods: the new methods are self-locking, not compounds.

// Keys returns every key of the trie in ascending byte order — the
// order All yields.  The result is an eager snapshot taken under the
// read lock, so it is safe to call concurrently with any trie operation
// and never reflects later modifications.  A nil or empty trie yields
// nil.
// Complexity is O(n·L), one visit per node.
func (tt *Tst[T]) Keys() []string {
	return tt.KeysWithPrefix("") // the empty prefix is every key, snapshotted
}

// Values returns every value of the trie in ascending key order — the
// value side of All, aligned with Keys so Keys()[i] holds the key of
// Values()[i].  The result is an eager snapshot taken under the read
// lock.  A nil or empty trie yields nil.
// Complexity is O(n·L), one visit per node.
func (tt *Tst[T]) Values() []T {
	var values []T
	for _, v := range tt.All() {
		values = append(values, v)
	}
	return values
}

// String renders the trie as its keys joined with single spaces inside
// square brackets, ascending byte order: "[sea shells she]".  The
// values are not rendered — the trie is a string set here; use All or
// Values for them.  The keys are snapshotted under the read lock and
// the formatting runs without the lock.  A nil or empty trie renders as
// "[]".  For a custom per-key rendering use StringFunc.
// Complexity is O(total key bytes).
func (tt *Tst[T]) String() string {
	return tt.StringFunc(func(key string) string {
		return key
	})
}

// StringFunc is String with a caller supplied per-key format function;
// the keys are walked in ascending byte order and the formatted strings
// joined inside square brackets, so the output is ascending and
// deterministic.  The keys are snapshotted under the read lock and the
// format function runs without the lock, so it may safely call back
// into the trie.  A nil format function renders as "[]".
// Complexity is O(total key bytes).
func (tt *Tst[T]) StringFunc(format func(key string) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, key := range tt.Keys() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(key))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether tt and other hold the same KEYS: same length
// and every key of tt contained in other.
//
// The values are NOT compared — T is any, and comparing two arbitrary
// values is not expressible without boxing them into an interface.  Two
// tries with identical keys but different values are Equal; this is the
// map-shaped rule the plan sets for the tries.  Two empty tries are
// equal, a nil trie equals an empty trie, and Equal is symmetric.
//
// Each trie is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of tries in opposite directions.
//
// Note the empty-key asymmetry across packages: a tst rejects the empty
// string while a trie stores it at the root — an Equal between the two
// families is meaningful only for non-empty key sets (they are separate
// packages with separate Equals anyway).
// Complexity is O(n·(L + ln n)) on average.
func (tt *Tst[T]) Equal(other *Tst[T]) bool {
	if tt.Len() != other.Len() {
		return false
	}
	if tt.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, key := range tt.Keys() { // tt's lock, released before other is touched
		if !other.Contains(key) { // other's lock, one key at a time
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the trie: a fresh trie with a fresh copy
// of every node (bytes, the three links and values included), so no
// backing memory is shared with the receiver — mutating either trie
// afterwards never affects the other.  The read lock is held for the
// whole copy, so the clone describes one consistent snapshot.
// Complexity is O(n·L), one copy per node.
func (tt *Tst[T]) Clone() *Tst[T] {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var copyNode func(n *tstNode[T]) *tstNode[T]
	copyNode = func(n *tstNode[T]) *tstNode[T] {
		if n == nil {
			return nil
		}
		dup := &tstNode[T]{
			c:        n.c,
			value:    n.value,
			hasValue: n.hasValue,
		}
		dup.left = copyNode(n.left)
		dup.mid = copyNode(n.mid)
		dup.right = copyNode(n.right)
		return dup
	}
	dup := &Tst[T]{
		root:   copyNode(tt.root),
		length: tt.length,
	}
	return dup // a fresh local trie; its own zero-value lock is ready
}

// ContainsSlice reports whether every key of keys is in the trie (an
// empty slice is vacuously contained).  The whole batch runs under one
// hold of the read lock, so the answer describes one consistent
// snapshot.  For "at least one key present" use ContainsAny.
// Complexity is O(k·(L + ln n)) on average, k = len(keys).
func (tt *Tst[T]) ContainsSlice(keys []string) bool {
	if len(keys) == 0 {
		return true // an empty batch is vacuously contained
	}
	if tt == nil {
		return false // a nil trie contains nothing
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, key := range keys {
		if _, found := tt.NlSearch(key); !found {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one key of keys is in the trie.
// The whole batch runs under one hold of the read lock.  An empty slice
// contains nothing.
// Complexity is O(k·(L + ln n)) on average, k = len(keys).
func (tt *Tst[T]) ContainsAny(keys []string) bool {
	if tt == nil || len(keys) == 0 {
		return false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	for _, key := range keys {
		if _, found := tt.NlSearch(key); found {
			return true
		}
	}
	return false
}

// DeleteSlice deletes each key of keys and reports whether at least one
// was present (values are irrelevant to the match).  Duplicate keys in
// the slice are harmless — the second delete simply misses.  The whole
// batch runs under one hold of the write lock, so it is atomic: an
// interleaved reader sees none or all of it, never part.  Deleting is
// tolerated on a nil trie (returns false), like Delete.
// Complexity is O(k·(L + ln n)) on average, k = len(keys).
func (tt *Tst[T]) DeleteSlice(keys []string) bool {
	if tt == nil {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	changed := false
	for _, key := range keys {
		if tt.NlDelete(key) {
			changed = true
		}
	}
	return changed
}

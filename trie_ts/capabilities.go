/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package trie_ts

import "strings"

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for the string-keyed tries: the tries ARE pluto's string
// sets, so they get set ergonomics directly.  What is here: key/value
// materialization, rendering, deep copy, keys-only equality and the key
// batches.  Everything is additive and mirrors trie's capabilities.go
// method for method; none of the new methods panics — they are total
// functions over the nil trie and the zero value like every read.
// Ordering comes free: the byte-order walk is ascending key order, so
// Keys, Values, String and StringFunc are deterministic with no
// sorting.  Batch insert is deliberately absent (a map-shaped batch is
// InsertMap territory — callers loop; the surface stays small), and
// Contains already exists, so it is not repeated here.
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
// Complexity is O(nodes).
func (t *Trie[T]) Keys() []string {
	return t.KeysWithPrefix("") // the empty prefix is every key, snapshotted
}

// Values returns every value of the trie in ascending key order — the
// value side of All, aligned with Keys so Keys()[i] holds the key of
// Values()[i].  The result is an eager snapshot taken under the read
// lock.  A nil or empty trie yields nil.
// Complexity is O(nodes).
func (t *Trie[T]) Values() []T {
	var values []T
	for _, v := range t.All() {
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
func (t *Trie[T]) String() string {
	return t.StringFunc(func(key string) string {
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
func (t *Trie[T]) StringFunc(format func(key string) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, key := range t.Keys() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(key))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether t and other hold the same KEYS: same length and
// every key of t contained in other.
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
// Note the empty-key asymmetry across packages: a trie stores the empty
// string at its root, while tst rejects it — an Equal between the two
// families is meaningful only for non-empty key sets (they are separate
// packages with separate Equals anyway).
// Complexity is O(n·L), n = Len, L the average key length.
func (t *Trie[T]) Equal(other *Trie[T]) bool {
	if t.Len() != other.Len() {
		return false
	}
	if t.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, key := range t.Keys() { // t's lock, released before other is touched
		if !other.Contains(key) { // other's lock, one key at a time
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the trie: a fresh trie with a fresh copy
// of every node (values and the full children arrays included), so no
// backing memory is shared with the receiver — mutating either trie
// afterwards never affects the other.  The read lock is held for the
// whole copy, so the clone describes one consistent snapshot.  The walk
// copies node by node; it does not memcpy, so the 2 KiB R-way arrays of
// unused slots stay unallocated in the copy.
// Complexity is O(nodes).
func (t *Trie[T]) Clone() *Trie[T] {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	var copyNode func(n *trieNode[T]) *trieNode[T]
	copyNode = func(n *trieNode[T]) *trieNode[T] {
		if n == nil {
			return nil
		}
		dup := &trieNode[T]{
			value:    n.value,
			hasValue: n.hasValue,
		}
		for b := 0; b < radix; b++ {
			if n.children[b] != nil {
				dup.children[b] = copyNode(n.children[b])
			}
		}
		return dup
	}
	dup := &Trie[T]{
		root:   copyNode(t.root),
		length: t.length,
	}
	return dup // a fresh local trie; its own zero-value lock is ready
}

// ContainsSlice reports whether every key of keys is in the trie (an
// empty slice is vacuously contained).  The whole batch runs under one
// hold of the read lock, so the answer describes one consistent
// snapshot.  For "at least one key present" use ContainsAny.
// Complexity is O(k·L), k = len(keys).
func (t *Trie[T]) ContainsSlice(keys []string) bool {
	if len(keys) == 0 {
		return true // an empty batch is vacuously contained
	}
	if t == nil {
		return false // a nil trie contains nothing
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	for _, key := range keys {
		if x := t.findNode(key); x == nil || !x.hasValue {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one key of keys is in the trie.
// The whole batch runs under one hold of the read lock.  An empty slice
// contains nothing.
// Complexity is O(k·L), k = len(keys).
func (t *Trie[T]) ContainsAny(keys []string) bool {
	if t == nil || len(keys) == 0 {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	for _, key := range keys {
		if x := t.findNode(key); x != nil && x.hasValue {
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
// Complexity is O(k·L), k = len(keys).
func (t *Trie[T]) DeleteSlice(keys []string) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	changed := false
	for _, key := range keys {
		if t.NlDelete(key) {
			changed = true
		}
	}
	return changed
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package trie_ts implements a generic string-keyed symbol table on an
// R-way trie that is safe for concurrent use.  It is the thread-safe
// twin of github.com/pschlump/pluto/trie — the TrieST of Sedgwick &
// Wayne, Algorithms, 4th ed. §5.4 — with the identical API guarded by a
// sync.RWMutex, plus the Lock and Unlock pair and the Nl-prefixed
// (no-lock) methods for compound operations.
//
// The trie is byte-oriented (R = 256): keys are arbitrary strings,
// including UTF-8 text, and ordering falls out of the key bytes
// themselves — there is no comparison function to supply.  Values of
// any type T hang off the nodes; keys are not stored, they are implicit
// in the path from the root.  The empty string is a valid key, stored
// at the root (algs4 behavior).
//
// Basic operations (L is the length of the key in bytes):
//
//	Insert(key, value) — Add key with value; replaces an existing value.	O(L)
//	Search(key) — Return the value associated with key, (T, bool).			O(L)
//	Contains(key) — Report whether key is in the trie.						O(L)
//	Delete(key) — Remove key, pruning now-childless nodes.					O(L)
//	IsEmpty — Report whether the trie is empty.								O(1)
//	Length / Len — Return the number of keys.								O(1)
//	LongestPrefixOf(query) — Longest key that is a prefix of query.			O(L)
//	KeysWithPrefix(prefix) — All keys starting with prefix, ascending.		O(nodes visited)
//	KeysThatMatch(pattern) — All keys matching pattern ('.' = any byte).	O(nodes visited)
//	All — Iterate (key, value) pairs in ascending key order.				O(nodes)
//	Lock / Unlock + Nl* — compound multi-step operations.					O(1) to lock
//
// Concurrency model:
//
//	Reads (Search, Contains, IsEmpty, Length, Len, LongestPrefixOf) take
//	the read lock and release it before returning, so they run in
//	parallel with each other.
//	Writes (Insert, Delete) take the write lock.
//	KeysWithPrefix, KeysThatMatch and All take an eager snapshot under
//	the read lock: the matching keys (and, for All, the values) are
//	collected into a slice while the read lock is held, and the result
//	is returned or yielded after the lock is released.  They are safe to
//	use concurrently with any trie operation — including mutating the
//	trie from inside the All loop — and never observe later
//	modifications.
//
// The element type needs no constraints at all and the zero value of
// Trie is an empty trie ready to use (including Insert) — no
// constructor required.  Elements are stored and returned by value.
//
// A nil *Trie behaves as an empty trie for every operation except
// Insert — a nil trie cannot store a value, and that call panics with a
// message naming the method.  This is the package's only panic.  The
// nil guard runs before any lock acquisition.
//
// The memory trade-off of an R-way trie: every node carries a
// [256]*trieNode children array — 2 KiB of pointers per node, allocated
// even when a node has a single child.
//
// Run the tests with -race.
package trie_ts

import (
	"iter"
	"sync"
)

// radix is the alphabet size of the trie: one child slot per byte value.
const radix = 256

// trieNode is one node of the trie.  A node represents the key spelled
// by the path from the root; hasValue marks that key as present, with
// value its associated data.  children[c] is the subtrie for all keys
// that continue with byte c.
type trieNode[T any] struct {
	value    T
	hasValue bool
	children [radix]*trieNode[T]
}

// Trie is a string-keyed symbol table with values of type T, built on
// an R-way trie (R = 256, one child per byte), safe for concurrent use.
//
// The zero value of Trie is an empty trie ready to use.
type Trie[T any] struct {
	root   *trieNode[T]
	length int
	lock   sync.RWMutex
}

// Insert associates value with key and returns true if the key was
// added.  If key was already present its value is replaced and Insert
// returns false (the trees' duplicates-replace convention).  The empty
// string is a valid key; it is stored at the root.
//
// It panics on a nil *Trie — the package's only panic; the nil guard
// runs before the write lock is taken.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) Insert(key string, value T) bool {
	if t == nil {
		panic("trie_ts: Insert called on a nil Trie")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.NlInsert(key, value)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) NlInsert(key string, value T) bool {
	if t.root == nil {
		t.root = &trieNode[T]{}
	}
	x := t.root
	for i := 0; i < len(key); i++ {
		c := key[i]
		if x.children[c] == nil {
			x.children[c] = &trieNode[T]{}
		}
		x = x.children[c]
	}
	if x.hasValue {
		x.value = value
		return false
	}
	x.value = value
	x.hasValue = true
	t.length++
	return true
}

// findNode returns the node reached by following key from the root, or
// nil if the path dies out before the key ends.  The key is present
// only if the returned node has hasValue set.  It is lock-free; the
// caller must hold the lock.
func (t *Trie[T]) findNode(key string) *trieNode[T] {
	x := t.root
	for i := 0; i < len(key) && x != nil; i++ {
		x = x.children[key[i]]
	}
	return x
}

// Search returns the value associated with key and true, or the zero
// value and false if key is not in the trie.  A nil *Trie reports
// not-found.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) Search(key string) (T, bool) {
	if t == nil {
		var zero T
		return zero, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlSearch(key)
}

// NlSearch is Search without locking; call it only while holding Lock.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) NlSearch(key string) (T, bool) {
	if x := t.findNode(key); x != nil && x.hasValue {
		return x.value, true
	}
	var zero T
	return zero, false
}

// Contains reports whether key is in the trie.  A nil *Trie reports
// false.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) Contains(key string) bool {
	if t == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	x := t.findNode(key)
	return x != nil && x.hasValue
}

// Delete removes key from the trie and returns true; it returns false
// if key is not present.  Nodes that are left with no value and no
// children are pruned on the way back up (algs4's recursive delete), so
// a deleted key's path does not linger as dead weight.  The empty
// string is deleted from the root.  The write lock is held across the
// search and the unlink, so the delete is atomic.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) Delete(key string) bool {
	if t == nil {
		return false
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.NlDelete(key)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// Complexity is O(L), where L is the length of the key in bytes.
func (t *Trie[T]) NlDelete(key string) bool {
	if t.root == nil {
		return false
	}
	var deleted bool
	t.root, deleted = deleteNode(t.root, key, 0)
	if deleted {
		t.length--
	}
	return deleted
}

// deleteNode removes key[d:] from the subtrie rooted at x and returns
// the node that should replace x (nil when x is left childless and
// value-less, so the parent can prune it) and whether a key was
// deleted.  It is lock-free; the caller must hold the lock.
func deleteNode[T any](x *trieNode[T], key string, d int) (*trieNode[T], bool) {
	if x == nil {
		return nil, false
	}
	var deleted bool
	if d == len(key) {
		if !x.hasValue {
			return x, false
		}
		var zero T
		x.value = zero
		x.hasValue = false
		deleted = true
	} else {
		c := key[d]
		x.children[c], deleted = deleteNode(x.children[c], key, d+1)
	}
	if !deleted {
		return x, false
	}
	if x.hasValue {
		return x, true
	}
	for _, ch := range x.children {
		if ch != nil {
			return x, true
		}
	}
	return nil, true
}

// IsEmpty returns true if the trie contains no keys.  A nil *Trie
// reports true.
// Complexity is O(1).
func (t *Trie[T]) IsEmpty() bool {
	if t == nil {
		return true
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.nlIsEmpty()
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (t *Trie[T]) nlIsEmpty() bool {
	return t.length == 0
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (t *Trie[T]) NlIsEmpty() bool {
	return t.nlIsEmpty()
}

// Length returns the number of keys in the trie.  A nil *Trie reports 0.
// Complexity is O(1).
func (t *Trie[T]) Length() int {
	if t == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.length
}

// Len is an alias for Length.  (It takes the read lock itself rather
// than calling Length — a locked method never calls another locked
// method.)
// Complexity is O(1).
func (t *Trie[T]) Len() int {
	if t == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.length
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (t *Trie[T]) NlLen() int {
	return t.length
}

// Lock takes the trie's write lock for a compound sequence of
// Nl-prefixed operations (for example an atomic NlSearch followed by
// NlDelete).  Calling a locking public method while holding Lock
// deadlocks, so inside the critical section use only the Nl methods.
// Locking a nil trie is a no-op.
func (t *Trie[T]) Lock() {
	if t == nil {
		return
	}
	t.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil trie
// is a no-op.
func (t *Trie[T]) Unlock() {
	if t == nil {
		return
	}
	t.lock.Unlock()
}

// LongestPrefixOf returns the longest key of the trie that is a prefix
// of query, or "" if no key is.  A nil *Trie reports "".
// Complexity is O(L), where L is the length of query in bytes.
func (t *Trie[T]) LongestPrefixOf(query string) string {
	if t == nil {
		return ""
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.root == nil {
		return ""
	}
	x := t.root
	longest := -1
	if x.hasValue {
		longest = 0 // the empty-string key at the root
	}
	for i := 0; i < len(query); i++ {
		x = x.children[query[i]]
		if x == nil {
			break
		}
		if x.hasValue {
			longest = i + 1
		}
	}
	if longest < 0 {
		return ""
	}
	return query[:longest]
}

// KeysWithPrefix returns all keys that start with prefix, in ascending
// order, or nil if there are none.  KeysWithPrefix("") returns every
// key in the trie.  A nil *Trie returns nil.
//
// The result is an eager snapshot collected under the read lock: it is
// safe to call KeysWithPrefix concurrently with any trie operation, and
// the returned slice never reflects later modifications.
// Complexity is O(L + V), where L is the length of prefix and V is the
// number of nodes visited in the matching subtrie.
func (t *Trie[T]) KeysWithPrefix(prefix string) []string {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	x := t.findNode(prefix)
	if x == nil {
		return nil
	}
	var results []string
	collectKeys(x, []byte(prefix), &results)
	return results
}

// collectKeys appends every key of the subtrie rooted at x to results,
// in ascending order, with prefix being the path from the root to x.
// It is lock-free; the caller must hold the lock.
func collectKeys[T any](x *trieNode[T], prefix []byte, results *[]string) {
	if x.hasValue {
		*results = append(*results, string(prefix))
	}
	for b := 0; b < radix; b++ {
		if x.children[b] != nil {
			collectKeys(x.children[b], append(prefix, byte(b)), results)
		}
	}
}

// KeysThatMatch returns all keys that match pattern, in ascending
// order, or nil if there are none.  A '.' in the pattern matches any
// one byte (algs4 §5.4 wildcard matching); every other byte must match
// literally.  A nil *Trie returns nil.
//
// The result is an eager snapshot collected under the read lock: it is
// safe to call KeysThatMatch concurrently with any trie operation, and
// the returned slice never reflects later modifications.
// Complexity is proportional to the number of nodes visited, which is
// O(R^w · L) in the worst case for a pattern with w wildcards.
func (t *Trie[T]) KeysThatMatch(pattern string) []string {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.root == nil {
		return nil
	}
	var results []string
	collectMatch(t.root, nil, pattern, &results)
	return results
}

// collectMatch appends every key of the subtrie rooted at x that
// matches pattern[len(prefix):] to results, in ascending order, with
// prefix being the path from the root to x.  It is lock-free; the
// caller must hold the lock.
func collectMatch[T any](x *trieNode[T], prefix []byte, pattern string, results *[]string) {
	d := len(prefix)
	if d == len(pattern) {
		if x.hasValue {
			*results = append(*results, string(prefix))
		}
		return
	}
	c := pattern[d]
	if c == '.' {
		for b := 0; b < radix; b++ {
			if x.children[b] != nil {
				collectMatch(x.children[b], append(prefix, byte(b)), pattern, results)
			}
		}
	} else if x.children[c] != nil {
		collectMatch(x.children[c], append(prefix, c), pattern, results)
	}
}

// All returns a range-over-func iterator (iter.Seq2) that visits every
// (key, value) pair of the trie in ascending key order:
//
//	for key, value := range tr.All() { ... }
//
// A single-variable range yields the key, not the value (use the
// two-variable form above).  A nil *Trie iterates as an empty one.
//
// The iterator operates on a snapshot of the (key, value) pairs
// collected under the read lock when All is called, so it is safe to
// call other trie methods — including writes — from the loop body, and
// it never observes modifications made after the call.  (The plain
// package's All walks the live trie; this is the inverted, thread-safe
// contract.)
// Complexity is O(nodes) for a full traversal.
func (t *Trie[T]) All() iter.Seq2[string, T] {
	if t == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	type pair struct {
		key   string
		value T
	}
	var snap []pair
	t.lock.RLock()
	var walk func(x *trieNode[T], prefix []byte)
	walk = func(x *trieNode[T], prefix []byte) {
		if x == nil {
			return
		}
		if x.hasValue {
			snap = append(snap, pair{key: string(prefix), value: x.value})
		}
		for b := 0; b < radix; b++ {
			if x.children[b] != nil {
				walk(x.children[b], append(prefix, byte(b)))
			}
		}
	}
	walk(t.root, nil)
	t.lock.RUnlock()
	return func(yield func(string, T) bool) {
		for _, p := range snap {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

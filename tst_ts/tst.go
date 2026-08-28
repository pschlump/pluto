/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tst_ts implements a thread-safe ternary search trie (TST) —
// the string-keyed symbol table of Sedgwick & Wayne, Algorithms, 4th ed.
// §5.5.  It is the thread-safe twin of github.com/pschlump/pluto/tst —
// the same API, guarded by a sync.RWMutex — with the addition of the
// Lock and Unlock pair and the Nl-prefixed (no-lock) methods for
// compound operations.
//
// Keys are byte strings (arbitrary UTF-8 is fine — ordering is by key
// bytes, so no comparison function is ever needed).  Each node stores a
// single byte plus three links: left and right for the alternative
// bytes at the current position, mid for the next byte of the key.  A
// TST uses one small node per character instead of a 256-way array per
// character, trading a logarithmic-in-alphabet-size factor on each step
// for drastically less memory on sparse key sets.
//
// Basic operations (L is the key length, n the number of keys):
//
//	Insert — add or replace the value of a key.				O(L + ln n) average
//	Search — the value of a key, (value, ok).				O(L + ln n) average
//	Contains — is the key present.							O(L + ln n) average
//	Delete — remove a key, pruning dead branches.			O(L + ln n) average
//	IsEmpty / Length / Len — size queries.					O(1)
//	LongestPrefixOf — longest key that is a prefix of query. O(L + ln n) average
//	KeysWithPrefix — all keys with a prefix, ascending.		O(matches)
//	KeysThatMatch — all keys matching a '.' wildcard pattern. O(matches)
//	All — iterate (key, value) pairs in ascending key order.  O(n·L)
//	Lock / Unlock + Nl* — compound multi-step operations.		O(1) to lock
//
// The empty key is REJECTED: Insert("") returns false and changes
// nothing, Search("")/Contains("") report not-found, Delete("") returns
// false.  (algs4 throws an exception on the empty key; pluto reports.)
//
// The zero value is fully usable, including Insert — there are no
// constructors.  The package has exactly one panic:
//
//	Insert on a nil *Tst — a nil structure cannot store a value.
//
// A nil *Tst and the zero value behave as an empty trie for every other
// operation: searches report not-found, Delete returns false, the key
// queries return nil, and All visits nothing.
//
// Concurrency model:
//
//	Reads (Search, Contains, IsEmpty, Length, Len, LongestPrefixOf)
//	take the read lock and release it before returning, so they run
//	in parallel with each other.
//	Writes (Insert, Delete) take the write lock.
//	All, KeysWithPrefix and KeysThatMatch collect an eager snapshot
//	of the (key, value) pairs / key strings under the read lock when
//	they are called, then release it — so they are safe to use
//	concurrently with any trie operation, including mutating the trie
//	from inside the All loop, and never observe later modifications.
//
// Lock and Unlock expose the real write lock for compound multi-step
// operations (an atomic NlSearch followed by NlDelete, say): hold Lock
// and use only the Nl-prefixed methods inside the critical section —
// calling a locking public method while holding Lock deadlocks.
//
// Run the tests with -race.
package tst_ts

import (
	"iter"
	"sync"
)

// tstNode is one character of one key.  left and right hold the
// alternative bytes at this key position (a binary search tree on the
// byte c), mid continues the current key one byte deeper.  value is
// meaningful only when hasValue is true; hasValue marks that some key
// ENDS at this node.
type tstNode[T any] struct {
	c        byte
	left     *tstNode[T]
	mid      *tstNode[T]
	right    *tstNode[T]
	value    T
	hasValue bool
}

// Tst is a thread-safe ternary search trie mapping string keys to
// values of type T.  The zero value is an empty trie, ready to use.
type Tst[T any] struct {
	root   *tstNode[T]
	length int // number of keys in the trie
	lock   sync.RWMutex
}

// Insert associates value with key and returns true if the key was
// added, false if an existing value was replaced.
//
// The empty key is rejected: Insert("") returns false and changes
// nothing.
//
// It panics on a nil *Tst: a nil structure cannot store a value (the
// package's only panic).  The panic fires BEFORE any lock acquisition.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) Insert(key string, value T) bool {
	if tt == nil {
		panic("tst_ts: Insert called on a nil Tst")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlInsert(key, value)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) NlInsert(key string, value T) bool {
	if key == "" {
		return false
	}
	added := false
	tt.root = insert(tt.root, key, value, 0, &added)
	if added {
		tt.length++
	}
	return added
}

// insert is the recursive core of Insert: it descends the left/right
// BST on byte key[d] and the mid link on a match, creating nodes as
// needed, and stores the value at the node where the last byte lands.
func insert[T any](x *tstNode[T], key string, value T, d int, added *bool) *tstNode[T] {
	c := key[d]
	if x == nil {
		x = &tstNode[T]{c: c}
	}
	switch {
	case c < x.c:
		x.left = insert(x.left, key, value, d, added)
	case c > x.c:
		x.right = insert(x.right, key, value, d, added)
	case d < len(key)-1:
		x.mid = insert(x.mid, key, value, d+1, added)
	default:
		if !x.hasValue {
			*added = true
		}
		x.value = value
		x.hasValue = true
	}
	return x
}

// get returns the node where key ends, or nil if key is not in the
// trie.  The caller checks hasValue.
func get[T any](x *tstNode[T], key string, d int) *tstNode[T] {
	if x == nil {
		return nil
	}
	c := key[d]
	switch {
	case c < x.c:
		return get(x.left, key, d)
	case c > x.c:
		return get(x.right, key, d)
	case d < len(key)-1:
		return get(x.mid, key, d+1)
	default:
		return x
	}
}

// Search returns the value associated with key and true, or the zero
// value and false if key is not in the trie.  The empty key is never
// found.  A nil or zero-value trie reports not-found.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) Search(key string) (T, bool) {
	if tt == nil || key == "" {
		var zero T
		return zero, false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlSearch(key)
}

// NlSearch is Search without locking; call it only while holding Lock.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) NlSearch(key string) (T, bool) {
	if key == "" {
		var zero T
		return zero, false
	}
	x := get(tt.root, key, 0)
	if x == nil || !x.hasValue {
		var zero T
		return zero, false
	}
	return x.value, true
}

// Contains reports whether key is in the trie.  The empty key is never
// contained.  A nil or zero-value trie reports false.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) Contains(key string) bool {
	if tt == nil || key == "" {
		return false
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	_, found := tt.NlSearch(key)
	return found
}

// Delete removes key from the trie and returns true, or returns false
// — changing nothing — if key is absent.  Besides clearing the value,
// Delete prunes the childless value-less nodes the key leaves behind,
// so removing every key restores the trie to its zero shape.  The write
// lock is held across the search and the unlink, so a Delete-then-
// Search race cannot resurrect a key.  A nil or zero-value trie reports
// false.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) Delete(key string) bool {
	if tt == nil || key == "" {
		return false
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDelete(key)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// Complexity is O(L + ln n) on average, where L is the key length.
func (tt *Tst[T]) NlDelete(key string) bool {
	if key == "" {
		return false
	}
	var deleted bool
	tt.root, deleted = deleteNode(tt.root, key, 0)
	if deleted {
		tt.length--
	}
	return deleted
}

// deleteNode is the recursive core of Delete.  On the way back up it
// unlinks every node that carries no value and has no children left —
// a dead end created by removing the key.  A childless value-less node
// with left or right siblings in use stays: its subtree still routes
// other keys.
func deleteNode[T any](x *tstNode[T], key string, d int) (*tstNode[T], bool) {
	if x == nil {
		return nil, false
	}
	c := key[d]
	var deleted bool
	switch {
	case c < x.c:
		x.left, deleted = deleteNode(x.left, key, d)
	case c > x.c:
		x.right, deleted = deleteNode(x.right, key, d)
	case d < len(key)-1:
		x.mid, deleted = deleteNode(x.mid, key, d+1)
	default:
		if !x.hasValue {
			return x, false
		}
		var zero T
		x.value = zero
		x.hasValue = false
		deleted = true
	}
	if !x.hasValue && x.left == nil && x.mid == nil && x.right == nil {
		return nil, deleted // prune the dead end
	}
	return x, deleted
}

// IsEmpty reports whether the trie has no keys.  A nil or zero-value
// trie is empty.
// Complexity is O(1).
func (tt *Tst[T]) IsEmpty() bool {
	if tt == nil {
		return true
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlIsEmpty()
}

// nlIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (tt *Tst[T]) nlIsEmpty() bool {
	return tt.length == 0
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (tt *Tst[T]) NlIsEmpty() bool {
	return tt.nlIsEmpty()
}

// Length returns the number of keys in the trie.  A nil or zero-value
// trie reports 0.
// Complexity is O(1).
func (tt *Tst[T]) Length() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Len returns the number of keys in the trie — an alias for Length (it
// does not call Length: a locked public method never calls another).
// Complexity is O(1).
func (tt *Tst[T]) Len() int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (tt *Tst[T]) NlLen() int {
	return tt.length
}

// Lock takes the trie's write lock for a compound sequence of
// Nl-prefixed operations (for example an atomic NlSearch followed by
// NlDelete).  Calling a locking public method while holding Lock
// deadlocks, so inside the critical section use only the Nl methods.
// Locking a nil trie is a no-op.
func (tt *Tst[T]) Lock() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil trie
// is a no-op.
func (tt *Tst[T]) Unlock() {
	if tt == nil {
		return
	}
	tt.lock.Unlock()
}

// LongestPrefixOf returns the longest key of the trie that is a prefix
// of query, or "" if there is no such key.  A nil or zero-value trie
// returns "".
// Complexity is O(L + ln n) on average, where L is the query length.
func (tt *Tst[T]) LongestPrefixOf(query string) string {
	if tt == nil || query == "" {
		return ""
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	x := tt.root
	length := 0
	d := 0
	for x != nil && d < len(query) {
		c := query[d]
		switch {
		case c < x.c:
			x = x.left
		case c > x.c:
			x = x.right
		default:
			d++
			if x.hasValue {
				length = d
			}
			x = x.mid
		}
	}
	return query[:length]
}

// KeysWithPrefix returns every key of the trie that begins with prefix,
// in ascending key order, or nil if there are none.  An empty prefix
// matches every key.  A nil or zero-value trie returns nil.
//
// The keys are collected into a snapshot under the read lock when
// KeysWithPrefix is called, then the lock is released before the slice
// is returned — the result is unaffected by later modifications.
// Complexity is O(L + m) on average, where L is the prefix length and m
// is the total length of the matched keys.
func (tt *Tst[T]) KeysWithPrefix(prefix string) []string {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var keys []string
	if prefix == "" {
		collect(tt.root, "", &keys)
		return keys
	}
	x := get(tt.root, prefix, 0)
	if x == nil {
		return nil
	}
	if x.hasValue {
		keys = append(keys, prefix)
	}
	collect(x.mid, prefix, &keys)
	return keys
}

// collect appends, in ascending key order, every key of the subtree at
// x (a depth-len(prefix) node).  prefix is the key bytes accumulated so
// far, NOT including x's own byte.
func collect[T any](x *tstNode[T], prefix string, keys *[]string) {
	if x == nil {
		return
	}
	collect(x.left, prefix, keys)
	key := prefix + string([]byte{x.c}) // byte, not rune: keys are byte strings
	if x.hasValue {
		*keys = append(*keys, key)
	}
	collect(x.mid, key, keys)
	collect(x.right, prefix, keys)
}

// KeysThatMatch returns every key of the trie that matches pattern —
// where '.' matches any single byte — in ascending key order, or nil if
// there are none.  A nil or zero-value trie returns nil.
//
// The keys are collected into a snapshot under the read lock when
// KeysThatMatch is called, then the lock is released before the slice
// is returned — the result is unaffected by later modifications.
// Complexity is O(m) on average, where m is the total length of the
// matched keys; a pattern of all '.' visits every node of the trie.
func (tt *Tst[T]) KeysThatMatch(pattern string) []string {
	if tt == nil {
		return nil
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	var keys []string
	collectMatch(tt.root, "", pattern, &keys)
	return keys
}

// collectMatch is the recursive core of KeysThatMatch: at depth
// d = len(prefix) it follows left when pattern[d] sorts before x.c,
// right when after, and mid on a match — with '.' taking all three
// branches in ascending order.
func collectMatch[T any](x *tstNode[T], prefix, pattern string, keys *[]string) {
	if x == nil {
		return
	}
	d := len(prefix)
	if d == len(pattern) {
		return
	}
	c := pattern[d]
	if c == '.' || c < x.c {
		collectMatch(x.left, prefix, pattern, keys)
	}
	if c == '.' || c == x.c {
		key := prefix + string([]byte{x.c})
		if d == len(pattern)-1 && x.hasValue {
			*keys = append(*keys, key)
		}
		collectMatch(x.mid, key, pattern, keys)
	}
	if c == '.' || c > x.c {
		collectMatch(x.right, prefix, pattern, keys)
	}
}

// All returns a range-over-func iterator that visits every (key, value)
// pair of the trie in ascending key order:
//
//	for key, value := range trie.All() { ... }
//
// A single-variable range yields the KEY (a string), not the value.
// A nil or empty trie visits nothing.
//
// The iterator operates on an eager snapshot of the (key, value) pairs
// collected under the read lock when All is called (the plain tst
// package's All walks the live trie — here the contracts differ), so it
// is safe to call other trie methods — including Insert and Delete —
// from the loop body, and later modifications are never observed.
// Complexity is O(n·L), one visit per node.
func (tt *Tst[T]) All() iter.Seq2[string, T] {
	if tt == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	type pair struct {
		key   string
		value T
	}
	var snap []pair
	tt.lock.RLock()
	var walk func(x *tstNode[T], prefix string)
	walk = func(x *tstNode[T], prefix string) {
		if x == nil {
			return
		}
		walk(x.left, prefix)
		key := prefix + string([]byte{x.c})
		if x.hasValue {
			snap = append(snap, pair{key: key, value: x.value})
		}
		walk(x.mid, key)
		walk(x.right, prefix)
	}
	walk(tt.root, "")
	tt.lock.RUnlock()
	return func(yield func(string, T) bool) {
		for _, p := range snap {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package patricia_trie_ts implements a thread-safe generic
// string-keyed symbol table on a Patricia trie — the compact bitwise
// radix trie of Sedgwick's radix-searching chapter, in the equivalent
// crit-bit formulation: keys live only at the leaves, and every
// internal node stores nothing but a critical bit index and two
// children.  It is the thread-safe twin of
// github.com/pschlump/pluto/patricia_trie — the same API, guarded by a
// sync.RWMutex — with the addition of the Lock and Unlock pair and the
// Nl-prefixed (no-lock) methods for compound operations.
//
// Keys are arbitrary strings — UTF-8 text, embedded NUL bytes, the
// empty string — and ordering falls out of the key bytes themselves,
// so there is no comparison function to supply: the package is
// constraint-free, has no constructors, and the zero value of
// PatriciaTrie[T] is an empty trie ready to use (including Insert).
// Path compression means one internal node per branching point and no
// others: a trie of n keys has exactly n leaves and n-1 internal
// nodes, and every operation is O(w) in the key length in bits,
// independent of the number of keys.  To keep arbitrary byte strings
// prefix-free while preserving byte order, keys are viewed through an
// order-preserving encoding: each byte contributes a 1 bit followed by
// its 8 bits (most significant first), and the end of the key
// contributes a single 0 bit.
//
// Basic operations (w is the key length in bits, O(w) = O(L) for an
// L-byte key; n is the number of keys; k keys share the glob's literal
// prefix, m is the pattern/key length):
//
//	Insert(key, value) — Add key with value; replaces an existing value.	O(w)
//	Search(key) — Return the value associated with key, (T, bool).			O(w)
//	Contains(key) — Report whether key is in the trie.						O(w)
//	Delete(key) — Remove key, collapsing its branch.						O(w)
//	IsEmpty — Report whether the trie is empty.								O(1)
//	Length / Len — Return the number of keys.								O(1)
//	KeysWithPrefix(prefix) — All keys starting with prefix, ascending.		O(w + matches)
//	LongestPrefixOf(key) — Longest stored key that is a prefix of key.		O(w)
//	KeysThatMatch(pattern) — Iterate keys matching a Redis glob, ascending.	O(w + k·m)
//	All — Iterate (key, value) pairs in ascending key order.				O(n)
//	Backward — Iterate (key, value) pairs in descending key order.			O(n)
//	Lock / Unlock + Nl* — compound multi-step operations.					O(1) to lock
//
// The element type needs no constraints at all.  Elements are stored
// and returned by value.
//
// The zero value is fully usable, including Insert — there are no
// constructors.  The package has exactly one panic:
//
//	Insert on a nil *PatriciaTrie — a nil structure cannot store a value.
//
// A nil *PatriciaTrie and the zero value behave as an empty trie for
// every other operation: searches report not-found, Delete returns
// false, the key queries return nil/empty, and the iterators visit
// nothing.
//
// Concurrency model:
//
//	Reads (Search, Contains, IsEmpty, Length, Len, LongestPrefixOf)
//	take the read lock and release it before returning, so they run
//	in parallel with each other.
//	Writes (Insert, Delete, Truncate) take the write lock.
//	All, Backward and KeysThatMatch collect an eager snapshot of the
//	(key, value) pairs under the read lock when they are called, then
//	release it — so they are safe to use concurrently with any trie
//	operation, including mutating the trie from inside the loop, and
//	never observe later modifications (the plain patricia_trie
//	package's iterators walk the live trie — here the contracts
//	differ).  KeysWithPrefix returns an eager slice and is a snapshot
//	by construction.  Dump holds the read lock for the whole traversal,
//	so the callback must not touch the trie.
//
// Lock and Unlock expose the real write lock for compound multi-step
// operations (an atomic NlSearch followed by NlDelete, say): hold Lock
// and use only the Nl-prefixed methods inside the critical section —
// calling a locking public method while holding Lock deadlocks.
//
// Run the tests with -race.
package patricia_trie_ts

import (
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
)

// symbolBits is the number of encoded bits per key byte: one marker bit
// (1 for a real byte, 0 for the end-of-key terminator) plus the byte's
// own 8 bits.
const symbolBits = 9

// patriciaNode is a leaf or an internal branching node of the trie.
// A leaf (bit == -1) carries one key and its value.  An internal node
// (bit >= 0) carries no key: it routes on the encoded bit at position
// bit, with child[0] the 0-side and child[1] the 1-side.  Path
// compression guarantees both children of an internal node are non-nil.
type patriciaNode[T any] struct {
	bit   int // critical bit index; -1 marks a leaf
	key   string
	value T
	child [2]*patriciaNode[T]
}

// pair is one (key, value) snapshot element.
type pair[T any] struct {
	key   string
	value T
}

// PatriciaTrie is a thread-safe string-keyed symbol table with values
// of type T, built on a Patricia trie (compact bitwise radix trie).
// The zero value is an empty trie, ready to use.
type PatriciaTrie[T any] struct {
	root   *patriciaNode[T]
	length int
	lock   sync.RWMutex
}

// bitDir returns the branching direction of key at encoded bit
// position i: 0 or 1.  The key is viewed through the order-preserving
// prefix-free encoding described in the package comment: byte s of the
// key occupies positions 9s..9s+8 — a marker 1 bit at 9s, then the
// byte's 8 bits most-significant-first — and the end of the key reads
// as 0 everywhere.  Positions at or past the end return 0, so the empty
// key branches 0 at every position.
func bitDir(key string, i int) int {
	s, o := i/symbolBits, i%symbolBits
	if s >= len(key) {
		return 0
	}
	if o == 0 {
		return 1
	}
	return int(key[s]>>(8-o)) & 1
}

// findLeaf returns the leaf reached by following key's branching bits
// from the root, or nil if the trie is empty.  The key is present only
// if the returned leaf's key equals it — path compression skips bits,
// so the full key comparison at the leaf is what decides membership.
// The caller must hold at least the read lock.
func (pt *PatriciaTrie[T]) findLeaf(key string) *patriciaNode[T] {
	x := pt.root
	for x != nil && x.bit >= 0 {
		x = x.child[bitDir(key, x.bit)]
	}
	return x
}

// Insert associates value with key and returns true if the key was
// added.  If key was already present its value is replaced and Insert
// returns false (the trees' duplicates-replace convention).  The empty
// string is an ordinary valid key.
//
// Insertion is the classic Patricia split: the search for key ends at
// some leaf; the new branch node is placed at the first encoded bit
// where key and that leaf's key differ, above every node that tests a
// later bit.
//
// It panics on a nil *PatriciaTrie: a nil structure cannot store a
// value (the package's only panic).  The panic fires BEFORE any lock
// acquisition.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) Insert(key string, value T) bool {
	if pt == nil {
		panic("patricia_trie_ts: Insert called on a nil PatriciaTrie")
	}
	pt.lock.Lock()
	defer pt.lock.Unlock()
	return pt.NlInsert(key, value)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) NlInsert(key string, value T) bool {
	if pt.root == nil {
		pt.root = &patriciaNode[T]{bit: -1, key: key, value: value}
		pt.length = 1
		return true
	}
	leaf := pt.findLeaf(key)
	if leaf.key == key {
		leaf.value = value
		return false
	}
	// The critical bit: the first encoded bit where the keys differ.
	// Distinct keys always differ somewhere under the encoding.
	crit := 0
	for bitDir(key, crit) == bitDir(leaf.key, crit) {
		crit++
	}
	// Descend again to the insertion point: the first slot whose node
	// tests a bit at or past crit.  (No node on the path tests crit
	// itself: such a node would route key and leaf.key the same way,
	// contradicting the choice of crit.)
	pp := &pt.root
	for (*pp).bit >= 0 && (*pp).bit < crit {
		pp = &(*pp).child[bitDir(key, (*pp).bit)]
	}
	d := bitDir(key, crit)
	in := &patriciaNode[T]{bit: crit}
	in.child[d] = &patriciaNode[T]{bit: -1, key: key, value: value}
	in.child[1-d] = *pp
	*pp = in
	pt.length++
	return true
}

// Search returns the value associated with key and true, or the zero
// value and false if key is not in the trie.  A nil or zero-value trie
// reports not-found.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) Search(key string) (T, bool) {
	if pt == nil {
		var zero T
		return zero, false
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	return pt.NlSearch(key)
}

// NlSearch is Search without locking; call it only while holding Lock.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) NlSearch(key string) (T, bool) {
	if x := pt.findLeaf(key); x != nil && x.key == key {
		return x.value, true
	}
	var zero T
	return zero, false
}

// Contains reports whether key is in the trie.  A nil or zero-value
// trie reports false.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) Contains(key string) bool {
	if pt == nil {
		return false
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	_, found := pt.NlSearch(key)
	return found
}

// Delete removes key from the trie and returns true; it returns false
// if key is not present.  The key's leaf is unlinked and its parent
// branch node is replaced by the sibling subtree, so the trie never
// keeps a one-child internal node: deleting every key restores the
// zero shape (root == nil).  The write lock is held across the search
// and the unlink, so a Delete-then-Search race cannot resurrect a key.
// A nil or zero-value trie reports false.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) Delete(key string) bool {
	if pt == nil {
		return false
	}
	pt.lock.Lock()
	defer pt.lock.Unlock()
	return pt.NlDelete(key)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// Complexity is O(w), where w is the key length in bits.
func (pt *PatriciaTrie[T]) NlDelete(key string) bool {
	if pt.root == nil {
		return false
	}
	// Descend to the leaf, tracking its parent p and the slot pSlot
	// (above p) that points to p.
	var p *patriciaNode[T]
	var pSlot **patriciaNode[T]
	slot := &pt.root
	x := pt.root
	for x.bit >= 0 {
		p = x
		pSlot = slot
		slot = &x.child[bitDir(key, x.bit)]
		x = *slot
	}
	if x.key != key {
		return false
	}
	if p == nil {
		// The trie held exactly this one key.
		pt.root = nil
	} else {
		// Replace the parent branch by the sibling subtree: the branch
		// existed only to separate the deleted key from its sibling.
		d := bitDir(key, p.bit)
		*pSlot = p.child[1-d]
	}
	pt.length--
	return true
}

// IsEmpty returns true if the trie contains no keys.  A nil or
// zero-value trie is empty.
// Complexity is O(1).
func (pt *PatriciaTrie[T]) IsEmpty() bool {
	if pt == nil {
		return true
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	return pt.length == 0
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (pt *PatriciaTrie[T]) NlIsEmpty() bool {
	return pt.length == 0
}

// Length returns the number of keys in the trie.  A nil or zero-value
// trie reports 0.
// Complexity is O(1).
func (pt *PatriciaTrie[T]) Length() int {
	if pt == nil {
		return 0
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	return pt.length
}

// Len returns the number of keys in the trie — an alias for Length (it
// does not call Length: a locked public method never calls another).
// Complexity is O(1).
func (pt *PatriciaTrie[T]) Len() int {
	if pt == nil {
		return 0
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	return pt.length
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (pt *PatriciaTrie[T]) NlLen() int {
	return pt.length
}

// Truncate removes all keys from the trie, restoring the zero shape.
// A nil *PatriciaTrie is left alone.
// Complexity is O(1).
func (pt *PatriciaTrie[T]) Truncate() {
	if pt == nil {
		return
	}
	pt.lock.Lock()
	defer pt.lock.Unlock()
	pt.root = nil
	pt.length = 0
}

// Lock takes the trie's write lock for a compound sequence of
// Nl-prefixed operations (for example an atomic NlSearch followed by
// NlDelete).  Calling a locking public method while holding Lock
// deadlocks, so inside the critical section use only the Nl methods.
// Locking a nil trie is a no-op.
func (pt *PatriciaTrie[T]) Lock() {
	if pt == nil {
		return
	}
	pt.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil trie
// is a no-op.
func (pt *PatriciaTrie[T]) Unlock() {
	if pt == nil {
		return
	}
	pt.lock.Unlock()
}

// KeysWithPrefix returns all keys that start with prefix, in ascending
// order, or nil if there are none.  KeysWithPrefix("") returns every
// key in the trie.  A nil or zero-value trie returns nil.
//
// The keys are collected under the read lock when KeysWithPrefix is
// called, then the lock is released before the slice is returned — the
// result is unaffected by later modifications (a snapshot, as in the
// other _ts packages).
//
// The search descends by prefix's bits only as far as prefix reaches;
// because path compression skips untested bits, the leaves collected
// below that point are filtered with a literal prefix check.
// Complexity is O(w + m), where w is the prefix length in bits and m is
// the total length of the matched keys.
func (pt *PatriciaTrie[T]) KeysWithPrefix(prefix string) []string {
	if pt == nil {
		return nil
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	x := pt.root
	for x != nil && x.bit >= 0 && x.bit < symbolBits*len(prefix) {
		x = x.child[bitDir(prefix, x.bit)]
	}
	var results []string
	var collect func(x *patriciaNode[T])
	collect = func(x *patriciaNode[T]) {
		if x == nil {
			return
		}
		if x.bit < 0 {
			if strings.HasPrefix(x.key, prefix) {
				results = append(results, x.key)
			}
			return
		}
		collect(x.child[0])
		collect(x.child[1])
	}
	collect(x)
	return results
}

// leftmostLeaf returns the leaf holding the smallest key of the subtree
// rooted at x — in-order is ascending key order, so the leftmost leaf
// is the minimum.  The caller must hold at least the read lock.
func leftmostLeaf[T any](x *patriciaNode[T]) *patriciaNode[T] {
	for x != nil && x.bit >= 0 {
		x = x.child[0]
	}
	return x
}

// LongestPrefixOf returns the longest key of the trie that is a prefix
// of key, together with its value and true; if no stored key is a
// prefix of key it returns ("", zero, false).  A nil or zero-value trie
// reports not-found.
//
// The descent follows key's branching bits.  Every stored prefix of key
// hangs off that descent: at a branch where key's bit is 1, the 0-side
// subtree can hold at most one prefix of key — the key that ends at
// that exact bit position, which is that subtree's smallest key (its
// leftmost leaf) — and where key's bit is 0 the 1-side holds none (a
// prefix of key agrees with key on every bit it has).  The leaf the
// descent ends on is tested last; deeper branches carry longer
// prefixes, so the last candidate found wins.
//
// Complexity is O(w) for the descent plus at most one leftmost descent
// per level — a worst case of O(w + h²) where h is the trie height —
// O(w) in practice when few stored keys are prefixes of key.
func (pt *PatriciaTrie[T]) LongestPrefixOf(key string) (string, T, bool) {
	if pt == nil {
		var zero T
		return "", zero, false
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	var bestKey string
	var bestVal T
	found := false
	consider := func(leaf *patriciaNode[T]) {
		if leaf != nil && strings.HasPrefix(key, leaf.key) {
			bestKey, bestVal, found = leaf.key, leaf.value, true
		}
	}
	// Encoded keys longer than key cannot be prefixes of it, so stop at
	// the first branch below key's last encoded bit (position 9·len).
	x := pt.root
	for x != nil && x.bit >= 0 && x.bit <= symbolBits*len(key) {
		d := bitDir(key, x.bit)
		if d == 1 {
			consider(leftmostLeaf(x.child[0]))
		}
		x = x.child[d]
	}
	if x != nil && x.bit < 0 {
		consider(x) // the leaf the descent ends on — key itself when stored
	}
	return bestKey, bestVal, found
}

// KeysThatMatch returns a range-over-func iterator (iter.Seq2) that
// visits every (key, value) pair whose key matches pattern, in ascending
// key order:
//
//	for key, value := range pt.KeysThatMatch("user:*") { ... }
//
// A single-variable range yields the key, not the value (use the
// two-variable form above).  A nil *PatriciaTrie iterates as an empty
// one; breaking out of the loop stops the walk early.
//
// The pattern is a Redis-style glob — the semantics of Redis's
// stringmatchlen (util.c), the matcher behind the KEYS command:
//
//   - any sequence of bytes, including empty
//     ?        any single byte
//     [abc]    any one of the listed bytes
//     [a-z]    any byte in the range (bounds swap if reversed)
//     [^abc]   any byte NOT listed / NOT in range
//     \x       a literal x — the backslash escapes any special byte
//
// Edge behavior mirrors the C exactly: a trailing lone backslash
// matches a literal backslash; an unterminated '[' runs as a class to
// the end of the pattern (a bare trailing '[' is an empty class that
// matches nothing, an unterminated '[^' matches any byte); ']' as the
// first class byte closes immediately; '*' does not match the empty
// string (the C loop never runs on it).  The match is on raw bytes —
// keys are binary-safe, any byte value legal.
//
// The iterator operates on an eager snapshot of the matching pairs
// collected under the read lock when KeysThatMatch is called (the plain
// patricia_trie package's iterator walks the live trie — here the
// contracts differ), so it is safe to mutate the trie from inside the
// loop, and later modifications are never observed.
//
// Cost: the pattern's literal prefix (its bytes before the first
// wildcard, escapes resolved) prunes the descent — only the subtree
// holding keys with that prefix is walked, then each candidate is
// matched in full.  Complexity is O(w + k·m) where w is the literal
// prefix length in bits, k is the number of keys sharing it, and m is
// the pattern/key length — O(n·m) in the worst case for a pattern that
// starts with a wildcard (a full descent).
func (pt *PatriciaTrie[T]) KeysThatMatch(pattern string) iter.Seq2[string, T] {
	if pt == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	prefix := globLiteralPrefix(pattern)
	pt.lock.RLock()
	var snap []pair[T]
	x := pt.root
	for x != nil && x.bit >= 0 && x.bit < symbolBits*len(prefix) {
		x = x.child[bitDir(prefix, x.bit)]
	}
	var walk func(x *patriciaNode[T])
	walk = func(x *patriciaNode[T]) {
		if x == nil {
			return
		}
		if x.bit < 0 {
			if strings.HasPrefix(x.key, prefix) && globMatch(pattern, x.key) {
				snap = append(snap, pair[T]{key: x.key, value: x.value})
			}
			return
		}
		walk(x.child[0])
		walk(x.child[1])
	}
	walk(x)
	pt.lock.RUnlock()
	return func(yield func(string, T) bool) {
		for _, p := range snap {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

// snapshotPairs collects every (key, value) pair of the trie in
// ascending key order — the shared collector behind All and Backward.
// The caller must hold at least the read lock.
func (pt *PatriciaTrie[T]) snapshotPairs() []pair[T] {
	var snap []pair[T]
	var walk func(x *patriciaNode[T])
	walk = func(x *patriciaNode[T]) {
		if x == nil {
			return
		}
		if x.bit < 0 {
			snap = append(snap, pair[T]{key: x.key, value: x.value})
			return
		}
		walk(x.child[0])
		walk(x.child[1])
	}
	walk(pt.root)
	return snap
}

// All returns a range-over-func iterator that visits every (key, value)
// pair of the trie in ascending key order:
//
//	for key, value := range pt.All() { ... }
//
// A single-variable range yields the KEY (a string), not the value.
// A nil or empty trie visits nothing.
//
// The iterator operates on an eager snapshot of the (key, value) pairs
// collected under the read lock when All is called (the plain
// patricia_trie package's All walks the live trie — here the contracts
// differ), so it is safe to call other trie methods — including Insert
// and Delete — from the loop body, and later modifications are never
// observed.
// Complexity is O(n).
func (pt *PatriciaTrie[T]) All() iter.Seq2[string, T] {
	if pt == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	pt.lock.RLock()
	snap := pt.snapshotPairs()
	pt.lock.RUnlock()
	return func(yield func(string, T) bool) {
		for _, p := range snap {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that visits every (key,
// value) pair of the trie in descending key order — the mirror image of
// All, over the same call-time snapshot.  A nil or empty trie visits
// nothing.
//
// The iterator operates on an eager snapshot (see All); mutating the
// trie from inside the loop is safe.
// Complexity is O(n).
func (pt *PatriciaTrie[T]) Backward() iter.Seq2[string, T] {
	if pt == nil {
		return func(func(string, T) bool) {} // a nil trie iterates as an empty one
	}
	pt.lock.RLock()
	snap := pt.snapshotPairs()
	pt.lock.RUnlock()
	return func(yield func(string, T) bool) {
		for i := len(snap) - 1; i >= 0; i-- {
			if !yield(snap[i].key, snap[i].value) {
				return
			}
		}
	}
}

// Dump writes an indented pre-order listing of the trie's structure to
// fo — internal nodes as "[bit N]", leaves as "key" => value.  It is a
// debugging aid; use All, Backward, KeysWithPrefix or KeysThatMatch to
// process the data.  A nil or empty trie dumps a single "(empty)" line.
//
// The read lock is held for the whole traversal, so nothing else may
// touch the trie until it returns.
// Complexity is O(n).
func (pt *PatriciaTrie[T]) Dump(fo io.Writer) {
	if pt == nil {
		_, _ = fmt.Fprintf(fo, "PatriciaTrie (empty)\n")
		return
	}
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	if pt.root == nil {
		_, _ = fmt.Fprintf(fo, "PatriciaTrie (empty)\n")
		return
	}
	_, _ = fmt.Fprintf(fo, "PatriciaTrie length=%d\n", pt.length)
	var walk func(x *patriciaNode[T], depth int)
	walk = func(x *patriciaNode[T], depth int) {
		indent := strings.Repeat(" ", 4*depth)
		if x.bit < 0 {
			_, _ = fmt.Fprintf(fo, "%s%q => %v\n", indent, x.key, x.value)
			return
		}
		_, _ = fmt.Fprintf(fo, "%s[bit %d]\n", indent, x.bit)
		walk(x.child[0], depth+1)
		walk(x.child[1], depth+1)
	}
	walk(pt.root, 0)
}

// globMatch reports whether s matches the glob pattern in full — the
// behavior of Redis's stringmatchlen (note/redis/src/util.c), ported
// byte for byte, case-sensitive.  See KeysThatMatch for the syntax.
func globMatch(pattern, s string) bool {
	skipLonger := false
	return globMatchImpl(pattern, 0, s, 0, &skipLonger, 0)
}

// globMatchImpl is the recursive core of globMatch — a direct port of
// Redis's stringmatchlen_impl.  pi/si are cursors into pat/s.  The
// skipLonger flag is the C function's early-termination optimization:
// once a '*'s remainder is known to match nowhere in the remaining
// string, enclosing '*' loops stop trying longer prefixes (it never
// changes the outcome).  nesting bounds recursion from pathological
// many-'*' patterns, as in the C.
func globMatchImpl(pat string, pi int, s string, si int, skipLonger *bool, nesting int) bool {
	// Protection against abusive patterns.
	if nesting > 1000 {
		return false
	}
	pEnd, sEnd := len(pat), len(s)
	for pi < pEnd && si < sEnd {
		switch pat[pi] {
		case '*':
			for pi+1 < pEnd && pat[pi+1] == '*' {
				pi++
			}
			if pi == pEnd-1 {
				return true // a lone '*' swallows the rest of the string
			}
			for si < sEnd {
				if globMatchImpl(pat, pi+1, s, si, skipLonger, nesting+1) {
					return true
				}
				if *skipLonger {
					return false
				}
				si++
			}
			// The pattern after '*' matches no suffix of the remaining
			// string; any earlier '*' would need it to match a shorter
			// one, so the enclosing searches can stop too.
			*skipLonger = true
			return false
		case '?':
			si++
		case '[':
			pi++
			negated := pi < pEnd && pat[pi] == '^'
			if negated {
				pi++
			}
			match := false
			for {
				if pi+1 < pEnd && pat[pi] == '\\' {
					pi++
					if pat[pi] == s[si] {
						match = true
					}
				} else if pi == pEnd {
					pi-- // unterminated class: it ran to the end of the pattern
					break
				} else if pat[pi] == ']' {
					break
				} else if pi+2 < pEnd && pat[pi+1] == '-' {
					start, end := pat[pi], pat[pi+2]
					if start > end {
						start, end = end, start
					}
					pi += 2
					if c := s[si]; c >= start && c <= end {
						match = true
					}
				} else if pat[pi] == s[si] {
					match = true
				}
				pi++
			}
			if negated {
				match = !match
			}
			if !match {
				return false
			}
			si++
		case '\\':
			if pi+1 < pEnd {
				pi++ // match the escaped byte that follows
			}
			// A trailing lone backslash matches a literal backslash.
			fallthrough
		default:
			if pat[pi] != s[si] {
				return false
			}
			si++
		}
		pi++
		if si == sEnd {
			// The string is exhausted; only trailing '*'s may remain.
			for pi < pEnd && pat[pi] == '*' {
				pi++
			}
			break
		}
	}
	return pi == pEnd && si == sEnd
}

// globLiteralPrefix returns the longest prefix of pattern that every
// matching string must start with: the bytes before the first wildcard,
// with backslash escapes resolved to their literal bytes.  It is the
// pruning key for KeysThatMatch — "user:\*" yields "user:*", "h?i"
// yields "h", "*x" yields "".
func globLiteralPrefix(pattern string) string {
	var sb strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[':
			return sb.String()
		case '\\':
			if i+1 < len(pattern) {
				i++
				sb.WriteByte(pattern[i])
			} else {
				sb.WriteByte('\\') // a trailing lone backslash is a literal one
			}
		default:
			sb.WriteByte(pattern[i])
		}
	}
	return sb.String()
}

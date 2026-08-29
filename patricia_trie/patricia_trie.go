/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package patricia_trie implements a generic string-keyed symbol table
// on a Patricia trie — the compact bitwise radix trie of Sedgwick's
// radix-searching chapter (PATRICIA: Practical Algorithm To Retrieve
// Information Coded In Alphanumeric).  The equivalent "crit-bit tree"
// formulation is used: keys live only at the leaves, and every internal
// node stores nothing but a critical bit index and two children — the
// same data structure Sedgwick describes, with the upward links
// replaced by explicit leaf nodes.
//
// Keys are arbitrary strings (UTF-8 text, embedded NUL bytes, the empty
// string — all fine): ordering falls out of the key bytes themselves,
// so there is no comparison function to supply.  Path compression means
// the trie stores one internal node per branching point of the key set
// and no others — there are no one-child internal nodes, so a trie of n
// keys has exactly n leaves and n-1 internal nodes.  Search examines
// each bit of the key at most once, independent of the number of keys.
//
// To make arbitrary byte strings prefix-free while preserving byte
// order, keys are viewed through an order-preserving encoding: each
// byte contributes a 1 bit followed by its 8 bits (most significant
// first), and the end of the key contributes a single 0 bit.  Two
// distinct keys therefore always differ at some encoded bit (so "" and
// "\x00" never collide), and ascending encoded-bit order is exactly
// ascending byte order with shorter prefixes first.
//
// Basic operations (w is the key length in bits, O(w) = O(L) for an
// L-byte key; n is the number of keys):
//
//	Insert(key, value) — Add key with value; replaces an existing value.	O(w)
//	Search(key) — Return the value associated with key, (T, bool).			O(w)
//	Contains(key) — Report whether key is in the trie.						O(w)
//	Delete(key) — Remove key, collapsing its branch.						O(w)
//	IsEmpty — Report whether the trie is empty.								O(1)
//	Length / Len — Return the number of keys.								O(1)
//	KeysWithPrefix(prefix) — All keys starting with prefix, ascending.		O(w + matches)
//	All — Iterate (key, value) pairs in ascending key order.				O(n)
//	Backward — Iterate (key, value) pairs in descending key order.			O(n)
//
// The element type needs no constraints at all and the zero value of
// PatriciaTrie is an empty trie ready to use (including Insert) — no
// constructor required.  Elements are stored and returned by value.
//
// A nil *PatriciaTrie behaves as an empty trie for every operation
// except Insert — a nil trie cannot store a value, and that call panics
// with a message naming the method.  This is the package's only panic.
//
// The structure is not safe for concurrent use.
package patricia_trie

import (
	"fmt"
	"io"
	"strings"
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

// PatriciaTrie is a string-keyed symbol table with values of type T,
// built on a Patricia trie (compact bitwise radix trie).
//
// The zero value of PatriciaTrie is an empty trie ready to use.
type PatriciaTrie[T any] struct {
	root   *patriciaNode[T]
	length int
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
func (t *PatriciaTrie[T]) findLeaf(key string) *patriciaNode[T] {
	if t == nil {
		return nil
	}
	x := t.root
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
// It panics on a nil *PatriciaTrie — the package's only panic.
// Complexity is O(w), where w is the key length in bits.
func (t *PatriciaTrie[T]) Insert(key string, value T) bool {
	if t == nil {
		panic("patricia_trie: Insert called on a nil PatriciaTrie")
	}
	if t.root == nil {
		t.root = &patriciaNode[T]{bit: -1, key: key, value: value}
		t.length = 1
		return true
	}
	leaf := t.findLeaf(key)
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
	pp := &t.root
	for (*pp).bit >= 0 && (*pp).bit < crit {
		pp = &(*pp).child[bitDir(key, (*pp).bit)]
	}
	d := bitDir(key, crit)
	in := &patriciaNode[T]{bit: crit}
	in.child[d] = &patriciaNode[T]{bit: -1, key: key, value: value}
	in.child[1-d] = *pp
	*pp = in
	t.length++
	return true
}

// Search returns the value associated with key and true, or the zero
// value and false if key is not in the trie.  A nil *PatriciaTrie
// reports not-found.
// Complexity is O(w), where w is the key length in bits.
func (t *PatriciaTrie[T]) Search(key string) (T, bool) {
	if x := t.findLeaf(key); x != nil && x.key == key {
		return x.value, true
	}
	var zero T
	return zero, false
}

// Contains reports whether key is in the trie.  A nil *PatriciaTrie
// reports false.
// Complexity is O(w), where w is the key length in bits.
func (t *PatriciaTrie[T]) Contains(key string) bool {
	x := t.findLeaf(key)
	return x != nil && x.key == key
}

// Delete removes key from the trie and returns true; it returns false
// if key is not present.  The key's leaf is unlinked and its parent
// branch node is replaced by the sibling subtree, so the trie never
// keeps a one-child internal node: deleting every key restores the
// zero shape (root == nil).  A nil *PatriciaTrie reports false.
// Complexity is O(w), where w is the key length in bits.
func (t *PatriciaTrie[T]) Delete(key string) bool {
	if t == nil || t.root == nil {
		return false
	}
	// Descend to the leaf, tracking its parent p and the slot pSlot
	// (above p) that points to p.
	var p *patriciaNode[T]
	var pSlot **patriciaNode[T]
	slot := &t.root
	x := t.root
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
		t.root = nil
	} else {
		// Replace the parent branch by the sibling subtree: the branch
		// existed only to separate the deleted key from its sibling.
		d := bitDir(key, p.bit)
		*pSlot = p.child[1-d]
	}
	t.length--
	return true
}

// IsEmpty returns true if the trie contains no keys.  A nil
// *PatriciaTrie reports true.
// Complexity is O(1).
func (t *PatriciaTrie[T]) IsEmpty() bool {
	return t == nil || t.length == 0
}

// Length returns the number of keys in the trie.  A nil *PatriciaTrie
// reports 0.
// Complexity is O(1).
func (t *PatriciaTrie[T]) Length() int {
	if t == nil {
		return 0
	}
	return t.length
}

// Len is an alias for Length.
// Complexity is O(1).
func (t *PatriciaTrie[T]) Len() int {
	return t.Length()
}

// Truncate removes all keys from the trie, restoring the zero shape.
// A nil *PatriciaTrie is left alone.
// Complexity is O(1).
func (t *PatriciaTrie[T]) Truncate() {
	if t == nil {
		return
	}
	t.root = nil
	t.length = 0
}

// KeysWithPrefix returns all keys that start with prefix, in ascending
// order, or nil if there are none.  KeysWithPrefix("") returns every
// key in the trie.  A nil *PatriciaTrie returns nil.
//
// The search descends by prefix's bits only as far as prefix reaches;
// because path compression skips untested bits, the leaves collected
// below that point are filtered with a literal prefix check.
// Complexity is O(w + m), where w is the prefix length in bits and m is
// the total length of the matched keys.
func (t *PatriciaTrie[T]) KeysWithPrefix(prefix string) []string {
	if t == nil || t.root == nil {
		return nil
	}
	x := t.root
	for x.bit >= 0 && x.bit < symbolBits*len(prefix) {
		x = x.child[bitDir(prefix, x.bit)]
	}
	var results []string
	collectPrefixed(x, prefix, &results)
	return results
}

// collectPrefixed appends every key of the subtree rooted at x that
// starts with prefix to results, in ascending order.
func collectPrefixed[T any](x *patriciaNode[T], prefix string, results *[]string) {
	if x.bit < 0 {
		if strings.HasPrefix(x.key, prefix) {
			*results = append(*results, x.key)
		}
		return
	}
	collectPrefixed(x.child[0], prefix, results)
	collectPrefixed(x.child[1], prefix, results)
}

// Dump writes an indented pre-order listing of the trie's structure to
// fo — internal nodes as "[bit N]", leaves as "key" => value.  It is a
// debugging aid; use All, Backward or KeysWithPrefix to process the
// data.  A nil or empty trie dumps a single "(empty)" line.
// Complexity is O(n).
func (t *PatriciaTrie[T]) Dump(fo io.Writer) {
	if t == nil || t.root == nil {
		_, _ = fmt.Fprintf(fo, "PatriciaTrie (empty)\n")
		return
	}
	_, _ = fmt.Fprintf(fo, "PatriciaTrie length=%d\n", t.length)
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
	walk(t.root, 0)
}

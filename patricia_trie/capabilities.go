/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package patricia_trie

import "strings"

// This file holds the set-convenience capabilities (the hashicorp/go-set
// translation) for the string-keyed tries: the tries ARE pluto's string
// sets, so they get set ergonomics directly.  What is here: key/value
// materialization, rendering, deep copy, keys-only equality and the key
// batches.  Everything is additive; none of the new methods panics —
// they are total functions over the nil trie and the zero value like
// every read.  Ordering comes free: the in-order walk is ascending key
// order (ascending encoded-bit order, which is ascending byte order
// with shorter prefixes first), so Keys, Values, String and StringFunc
// are deterministic with no sorting.  Batch insert is deliberately
// absent (a map-shaped batch is InsertMap territory — callers loop; the
// surface stays small), and Contains already exists, so it is not
// repeated here.

// Keys returns every key of the trie in ascending byte order — the
// order All yields.  The slice is a fresh copy: mutating it does not
// affect the trie.  A nil or empty trie yields nil.
// Complexity is O(n), one visit per node.
func (t *PatriciaTrie[T]) Keys() []string {
	return t.KeysWithPrefix("") // the empty prefix is every key
}

// Values returns every value of the trie in ascending key order — the
// value side of All, aligned with Keys so Keys()[i] holds the key of
// Values()[i].  A nil or empty trie yields nil.
// Complexity is O(n), one visit per node.
func (t *PatriciaTrie[T]) Values() []T {
	var values []T
	for _, v := range t.All() {
		values = append(values, v)
	}
	return values
}

// String renders the trie as its keys joined with single spaces inside
// square brackets, ascending byte order: "[sea shells she]".  The
// values are not rendered — the trie is a string set here; use All or
// Values for them.  A nil or empty trie renders as "[]".  For a custom
// per-key rendering use StringFunc.
// Complexity is O(total key bytes).
func (t *PatriciaTrie[T]) String() string {
	return t.StringFunc(func(key string) string {
		return key
	})
}

// StringFunc is String with a caller supplied per-key format function;
// the keys are walked in ascending byte order and the formatted strings
// joined inside square brackets, so the output is ascending and
// deterministic.  A nil format function renders as "[]".
// Complexity is O(total key bytes).
func (t *PatriciaTrie[T]) StringFunc(format func(key string) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, key := range t.Keys() {
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
// Note the empty-key asymmetry across packages: a PatriciaTrie and a
// trie both store the empty string, but a tst rejects it — an Equal
// between the families is meaningful only for non-empty key sets (they
// are separate packages with separate Equals anyway).
// Complexity is O(n·w), w the average key length in bits.
func (t *PatriciaTrie[T]) Equal(other *PatriciaTrie[T]) bool {
	if t.Len() != other.Len() {
		return false
	}
	if t.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, key := range t.Keys() {
		if !other.Contains(key) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the trie: a fresh trie with a fresh copy
// of every node (critical bits, keys, values and both children
// included), so no backing memory is shared with the receiver —
// mutating either trie afterwards never affects the other.
// Complexity is O(n), one copy per node.
func (t *PatriciaTrie[T]) Clone() *PatriciaTrie[T] {
	if t == nil {
		return nil
	}
	var copyNode func(n *patriciaNode[T]) *patriciaNode[T]
	copyNode = func(n *patriciaNode[T]) *patriciaNode[T] {
		if n == nil {
			return nil
		}
		dup := &patriciaNode[T]{
			bit:   n.bit,
			key:   n.key,
			value: n.value,
		}
		dup.child[0] = copyNode(n.child[0])
		dup.child[1] = copyNode(n.child[1])
		return dup
	}
	return &PatriciaTrie[T]{
		root:   copyNode(t.root),
		length: t.length,
	}
}

// ContainsSlice reports whether every key of keys is in the trie (an
// empty slice is vacuously contained).  For "at least one key present"
// use ContainsAny.
// Complexity is O(k·w), k = len(keys), w the key length in bits.
func (t *PatriciaTrie[T]) ContainsSlice(keys []string) bool {
	if len(keys) == 0 {
		return true // an empty batch is vacuously contained
	}
	if t == nil {
		return false // a nil trie contains nothing
	}
	for _, key := range keys {
		if !t.Contains(key) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one key of keys is in the trie.
// An empty slice contains nothing.
// Complexity is O(k·w), k = len(keys), w the key length in bits.
func (t *PatriciaTrie[T]) ContainsAny(keys []string) bool {
	if t == nil || len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if t.Contains(key) {
			return true
		}
	}
	return false
}

// DeleteSlice deletes each key of keys and reports whether at least one
// was present (values are irrelevant to the match).  Duplicate keys in
// the slice are harmless — the second delete simply misses.  Deleting
// is tolerated on a nil trie (returns false), like Delete.
// Complexity is O(k·w), k = len(keys), w the key length in bits.
func (t *PatriciaTrie[T]) DeleteSlice(keys []string) bool {
	changed := false
	for _, key := range keys {
		if t.Delete(key) {
			changed = true
		}
	}
	return changed
}

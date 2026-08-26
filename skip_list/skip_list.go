/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

/*
Package skip_list implements an ordered collection built on a skip list.
A skip list is a probabilistic data structure with multiple levels of
linked lists; each higher level acts as an "express lane" that skips over
roughly half of the nodes below it.  The operations provided are:

  - Insert   — add an item; an item that compares equal to an existing one replaces it.	O(log₂ n) expected
  - Search   — find an item; reports not-found if not present.						O(log₂ n) expected
  - Delete   — remove an item; returns false if not present.							O(log₂ n) expected
  - IsEmpty  — true if the list has no nodes.											O(1)
  - Len / Length — number of nodes in the list.											O(1)
  - FindMin / FindMax — smallest / largest item.										O(1) / O(log₂ n) expected
  - DeleteAtHead / DeleteAtTail — remove smallest / largest item.						O(1) / O(log₂ n) expected
  - Truncate — remove all nodes.														O(1)
  - Dump     — write a per-level picture of the list for debugging.					O(n)
  - All / Backward — range-over-func iterators (see iter.go).							O(n)

Skip lists are probabilistic: the expected cost of Insert/Search/Delete is
O(log₂ n), but the worst case is O(n).  Unlike an unbalanced binary search
tree, performance does not degrade when items are inserted in sorted
order.  The tower heights are drawn from the global math/rand/v2 source;
the observable ordering semantics are deterministic regardless of the
heights drawn.

It is a rework of github.com/pschlump/pluto/skip_list in which the
comparable.Comparable interface constraint has been replaced with plain Go
type parameters.  Elements are stored and returned by value and ordering
is a direct function call, so element data is never boxed into an
interface and never unboxed with a type assertion.  Lists of naturally
ordered key types (all integers, floats and strings) are created with
NewSkipList, which orders elements with the built-in < and > operators of
the key type; lists of any other type — including structs ordered by a
single field — are created with NewSkipListFunc, which takes a caller
supplied comparison function; the element type does not have to implement
any interface.

A nil *SkipList and the zero value both behave as an empty list for every
operation except Insert: Search finds nothing, Delete, DeleteAtHead and
DeleteAtTail return false, FindMin and FindMax report not-found, Len is 0,
and the iterators visit nothing.

The package panics in exactly three situations, all programmer errors
that cannot be handled where they occur:

	NewSkipListFunc(nil)             — nil comparison function, caught at construction.
	Insert on a nil list             — a nil list cannot store an element.
	Insert on a zero-value list      — no comparison function; the message names the constructors.

SkipList is not safe for concurrent use.
*/
package skip_list

import (
	"cmp"
	"fmt"
	"io"
	"math/rand/v2"
)

// maxLevel is the maximum number of forward pointers a node can have.  With
// p = 0.5 this comfortably covers lists of up to 2**32 items.
const maxLevel = 32

// levelProbability is the probability that a node is promoted to the next
// higher level during insertion.
const levelProbability = 0.5

// SkipListNode is a single node of the list.  It holds the item data and a
// slice of forward pointers, one per level the node participates in.  Level 0
// links every node in ascending order.
type SkipListNode[T any] struct {
	data    T
	forward []*SkipListNode[T]
}

// SkipList is a generic skip list.  Use NewSkipList for naturally ordered
// key types (numbers, strings) or NewSkipListFunc for a caller supplied
// comparison function.  The zero value is an empty list.
type SkipList[T any] struct {
	head   *SkipListNode[T] // Sentinel node; its data is the zero value.
	level  int              // Highest level currently in use (0 when empty).
	length int              // Number of nodes in the list.

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewSkipList and
// is handy for building custom comparison functions.
func Compare[T cmp.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// NewSkipList creates a new SkipList for any naturally ordered key type
// (all integers, floats and strings — cmp.Ordered).  Ordering uses the
// built-in < and > operators of T; no interface and no boxing is involved.
// Complexity is O(1).
func NewSkipList[T cmp.Ordered]() *SkipList[T] {
	return &SkipList[T]{cmp: Compare[T]}
}

// NewSkipListFunc creates a new SkipList that orders elements with the
// caller supplied comparison function fx.  fx must return a negative value
// if a sorts before b, 0 if the two are duplicates and a positive value if
// a sorts after b, and must order elements consistently.  This lets any
// type — for example a struct ordered by one of its fields — be stored
// without implementing any interface.
// Complexity is O(1).
func NewSkipListFunc[T any](fx func(a, b T) int) *SkipList[T] {
	if fx == nil {
		panic("skip_list: NewSkipListFunc called with a nil comparison function")
	}
	return &SkipList[T]{cmp: fx}
}

// ensureHead lazily allocates the sentinel head node.
func (tt *SkipList[T]) ensureHead() {
	if tt.head == nil {
		tt.head = &SkipListNode[T]{forward: make([]*SkipListNode[T], maxLevel)}
	}
}

// randomLevel returns a random level in [1, maxLevel] for a new node, with
// each promotion to the next level having probability levelProbability.
func randomLevel() int {
	lvl := 1
	for lvl < maxLevel && rand.Float64() < levelProbability {
		lvl++
	}
	return lvl
}

// findPath returns, for each level, the last node that compares strictly
// less than item.  It is shared by Insert, Search and Delete.  The caller is
// responsible for having called ensureHead.
func (tt *SkipList[T]) findPath(item T) (update []*SkipListNode[T]) {
	update = make([]*SkipListNode[T], maxLevel)
	for i := range update {
		update[i] = tt.head
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, item) < 0 {
			cur = cur.forward[i]
		}
		update[i] = cur
	}
	return
}

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (tt *SkipList[T]) IsEmpty() bool {
	return tt == nil || tt.length == 0
}

// Len returns the number of elements in the list.
// Complexity is O(1).
func (tt *SkipList[T]) Len() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Length is an alias for Len; it returns the number of elements in the
// list.
// Complexity is O(1).
func (tt *SkipList[T]) Length() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// Truncate removes all data from the list.  The comparison function is
// kept, so the list remains usable and can simply be refilled.
// Complexity is O(1).
func (tt *SkipList[T]) Truncate() {
	if tt == nil {
		return
	}
	tt.head = nil
	tt.level = 0
	tt.length = 0
}

// Insert will add a new item to the list.  If it is a duplicate of an
// existing item the new item will replace the existing one in place and
// false is returned; true is returned when a new node was added.
// Insert panics on a nil list or on a zero-value list (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty list.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Insert(item T) (added bool) {
	if tt == nil {
		panic("skip_list: Insert called on a nil list")
	}
	if tt.cmp == nil {
		panic("skip_list: Insert called on a list with no comparison function (create the list with NewSkipList or NewSkipListFunc)")
	}
	tt.ensureHead()
	update := tt.findPath(item)

	// If the next node at level 0 is equal, replace its data.
	if next := update[0].forward[0]; next != nil && tt.cmp(next.data, item) == 0 {
		next.data = item
		return false
	}

	lvl := randomLevel()
	if lvl > tt.level {
		// The head node is the predecessor on all new levels.
		for i := tt.level; i < lvl; i++ {
			update[i] = tt.head
		}
		tt.level = lvl
	}

	node := &SkipListNode[T]{data: item, forward: make([]*SkipListNode[T], lvl)}
	for i := range lvl {
		node.forward[i] = update[i].forward[i]
		update[i].forward[i] = node
	}
	tt.length++
	return true
}

// Search will return the item in the list that compares equal to `item`,
// or false if it is not present.  `item` only needs the fields that the
// list's comparison function reads: a probe with just the key fields set
// finds the element with the full data.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Search(item T) (rv T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, item) < 0 {
			cur = cur.forward[i]
		}
	}
	cur = cur.forward[0]
	if cur != nil && tt.cmp(cur.data, item) == 0 {
		return cur.data, true
	}
	return
}

// Delete will remove the item in the list that compares equal to `item`.
// It returns false if the item is not present.  As with Search, `item`
// only needs the fields that the comparison function reads.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Delete(item T) (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	update := tt.findPath(item)
	node := update[0].forward[0]
	if node == nil || tt.cmp(node.data, item) != 0 {
		return false
	}
	for i := 0; i < tt.level; i++ {
		if update[i].forward[i] == node {
			update[i].forward[i] = node.forward[i]
		}
	}
	// Drop levels that are no longer in use.
	for tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		tt.level--
	}
	tt.length--
	return true
}

// FindMin returns the smallest item in the list, or false if it is empty.
// Complexity is O(1).
func (tt *SkipList[T]) FindMin() (rv T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}
	return tt.head.forward[0].data, true
}

// FindMax returns the largest item in the list, or false if it is empty.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) FindMax() (rv T, found bool) {
	if tt == nil || tt.IsEmpty() {
		return
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil {
			cur = cur.forward[i]
		}
	}
	return cur.data, true
}

// DeleteAtHead removes the smallest item in the list.  It returns false if
// the list is empty.
// Complexity is O(1) amortized (proportional to the removed node's level).
func (tt *SkipList[T]) DeleteAtHead() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	node := tt.head.forward[0]
	copy(tt.head.forward, node.forward)
	// Drop levels that are no longer in use.
	for tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		tt.level--
	}
	tt.length--
	return true
}

// DeleteAtTail removes the largest item in the list.  It returns false if
// the list is empty.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) DeleteAtTail() (found bool) {
	if tt == nil || tt.IsEmpty() {
		return false
	}
	x, _ := tt.FindMax()
	return tt.Delete(x)
}

// Dump writes a per-level picture of the list to `w` for debugging.  The
// bottom level (L0) lists every node; each higher level lists only the
// nodes promoted to that level.
// Complexity is O(n).
func (tt *SkipList[T]) Dump(w io.Writer) {
	if tt == nil || tt.IsEmpty() {
		_, _ = fmt.Fprintf(w, "SkipList (empty)\n")
		return
	}
	_, _ = fmt.Fprintf(w, "SkipList length=%d level=%d\n", tt.length, tt.level)
	for i := tt.level - 1; i >= 0; i-- {
		_, _ = fmt.Fprintf(w, "L%d: ", i)
		for cur := tt.head.forward[i]; cur != nil; cur = cur.forward[i] {
			_, _ = fmt.Fprintf(w, "%v ", cur.data)
		}
		_, _ = fmt.Fprintf(w, "\n")
	}
}

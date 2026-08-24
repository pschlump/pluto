package skip_list_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

/*
Package skip_list_dll implements an ordered collection built on a
doubly-linked skip list.  It is a variant of github.com/pschlump/pluto/skip_list
in which every node also carries a back pointer on level 0, so the bottom
level is a doubly-linked list.  That makes descending iteration (Backward) an
O(1)-space walk from the tail instead of an O(n)-space snapshot.  The
operations provided are:

  - Insert   — add an item; an item that Compare-equal to an existing one replaces it.	O(log₂ n) expected
  - Search   — find an item; returns nil if not present.								O(log₂ n) expected
  - Delete   — remove an item; returns false if not present.							O(log₂ n) expected
  - IsEmpty  — true if the list has no nodes.											O(1)
  - Length   — number of nodes in the list.												O(1)
  - FindMin / FindMax — smallest / largest item.										O(1) / O(log₂ n) expected
  - DeleteAtHead / DeleteAtTail — remove smallest / largest item.						O(1) / O(log₂ n) expected
  - Truncate — remove all nodes.														O(1)
  - Dump     — write a per-level picture of the list for debugging.					O(n)
  - All / Backward — Go 1.23+ range-over-func iterators (see iter.go).				O(n), O(1) extra space

Skip lists are probabilistic: the expected cost of Insert/Search/Delete is
O(log₂ n), but the worst case is O(n).  Unlike an unbalanced binary search
tree, performance does not degrade when items are inserted in sorted order.
*/

import (
	"fmt"
	"io"
	"math/rand/v2"

	"github.com/pschlump/pluto/comparable"
)

// maxLevel is the maximum number of forward pointers a node can have.  With
// p = 0.5 this comfortably covers lists of up to 2**32 items.
const maxLevel = 32

// levelProbability is the probability that a node is promoted to the next
// higher level during insertion.
const levelProbability = 0.5

// SkipListNode is a single node of the list.  It holds the item data, a
// slice of forward pointers (one per level the node participates in), and a
// back pointer on level 0.  Level 0 links every node in ascending order in
// both directions.
type SkipListNode[T comparable.Comparable] struct {
	data    *T
	forward []*SkipListNode[T]
	prev    *SkipListNode[T] // Previous node on level 0; head for the first node.
}

// SkipList is a generic doubly-linked skip list of items that implement
// comparable.Comparable.  The zero value is ready to use.
type SkipList[T comparable.Comparable] struct {
	head   *SkipListNode[T] // Sentinel node; its data is nil.
	level  int              // Highest level currently in use (0 when empty).
	length int              // Number of nodes in the list.
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
		for cur.forward[i] != nil && (*cur.forward[i].data).Compare(item) < 0 {
			cur = cur.forward[i]
		}
		update[i] = cur
	}
	return
}

// lastNode returns the node holding the largest item, or nil if the list is
// empty.
func (tt *SkipList[T]) lastNode() *SkipListNode[T] {
	if tt.IsEmpty() {
		return nil
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil {
			cur = cur.forward[i]
		}
	}
	return cur
}

// unlink splices `node` out of every level and fixes up the level-0 back
// pointer of its successor.  `update` is the predecessor array from
// findPath.  The caller adjusts length and drops unused levels.
func (tt *SkipList[T]) unlink(node *SkipListNode[T], update []*SkipListNode[T]) {
	for i := 0; i < tt.level; i++ {
		if update[i].forward[i] == node {
			update[i].forward[i] = node.forward[i]
		}
	}
	if node.forward[0] != nil {
		node.forward[0].prev = update[0]
	}
}

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (tt SkipList[T]) IsEmpty() bool {
	return tt.length == 0
}

// Length will return the number of elements in the list.
// Complexity is O(1).
func (tt SkipList[T]) Length() int {
	return tt.length
}

// Truncate removes all data from the list.
// Complexity is O(1).
func (tt *SkipList[T]) Truncate() {
	tt.head = nil
	tt.level = 0
	tt.length = 0
}

// Insert will add a new item to the list.  If it is a duplicate of an
// existing item the new item will replace the existing one.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Insert(item T) {
	if tt == nil {
		panic("skip list should not be nil")
	}
	tt.ensureHead()
	update := tt.findPath(item)

	// If the next node at level 0 is equal, replace its data.
	if next := update[0].forward[0]; next != nil && (*next.data).Compare(item) == 0 {
		next.data = &item
		return
	}

	lvl := randomLevel()
	if lvl > tt.level {
		// The head node is the predecessor on all new levels.
		for i := tt.level; i < lvl; i++ {
			update[i] = tt.head
		}
		tt.level = lvl
	}

	node := &SkipListNode[T]{data: &item, forward: make([]*SkipListNode[T], lvl)}
	for i := 0; i < lvl; i++ {
		node.forward[i] = update[i].forward[i]
		update[i].forward[i] = node
	}
	// Splice into the level-0 back chain.
	node.prev = update[0]
	if node.forward[0] != nil {
		node.forward[0].prev = node
	}
	tt.length++
}

// Search will return the item in the list that Compare-equal to `item`, or
// nil if it is not present.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Search(item T) (rv *T) {
	if tt.IsEmpty() {
		return nil
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && (*cur.forward[i].data).Compare(item) < 0 {
			cur = cur.forward[i]
		}
	}
	cur = cur.forward[0]
	if cur != nil && (*cur.data).Compare(item) == 0 {
		return cur.data
	}
	return nil
}

// Delete will remove the item in the list that Compare-equal to `item`.  It
// returns false if the item is not present.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Delete(item T) (found bool) {
	if tt == nil {
		panic("skip list should not be nil")
	}
	if tt.IsEmpty() {
		return false
	}
	update := tt.findPath(item)
	node := update[0].forward[0]
	if node == nil || (*node.data).Compare(item) != 0 {
		return false
	}
	tt.unlink(node, update)
	// Drop levels that are no longer in use.
	for tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		tt.level--
	}
	tt.length--
	return true
}

// FindMin returns the smallest item in the list, or nil if it is empty.
// Complexity is O(1).
func (tt *SkipList[T]) FindMin() (rv *T) {
	if tt.IsEmpty() {
		return nil
	}
	return tt.head.forward[0].data
}

// FindMax returns the largest item in the list, or nil if it is empty.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) FindMax() (rv *T) {
	if last := tt.lastNode(); last != nil {
		return last.data
	}
	return nil
}

// DeleteAtHead removes the smallest item in the list.  It returns false if
// the list is empty.
// Complexity is O(1) amortized (proportional to the removed node's level).
func (tt *SkipList[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("skip list should not be nil")
	}
	if tt.IsEmpty() {
		return false
	}
	node := tt.head.forward[0]
	for i := 0; i < len(node.forward); i++ {
		tt.head.forward[i] = node.forward[i]
	}
	if node.forward[0] != nil {
		node.forward[0].prev = tt.head
	}
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
	if tt == nil {
		panic("skip list should not be nil")
	}
	if tt.IsEmpty() {
		return false
	}
	return tt.Delete(*tt.lastNode().data)
}

// Dump writes a per-level picture of the list to `w` for debugging.  The
// bottom level (L0) lists every node; each higher level lists only the nodes
// promoted to that level.
// Complexity is O(n).
func (tt *SkipList[T]) Dump(w io.Writer) {
	if tt.IsEmpty() {
		fmt.Fprintf(w, "SkipList (empty)\n")
		return
	}
	fmt.Fprintf(w, "SkipList length=%d level=%d\n", tt.length, tt.level)
	for i := tt.level - 1; i >= 0; i-- {
		fmt.Fprintf(w, "L%d: ", i)
		for cur := tt.head.forward[i]; cur != nil; cur = cur.forward[i] {
			fmt.Fprintf(w, "%v ", *cur.data)
		}
		fmt.Fprintf(w, "\n")
	}
}

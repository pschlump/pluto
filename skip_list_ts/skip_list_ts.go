// Package skip_list_ts implements a generic skip list that is safe for
// concurrent use.  All operations are guarded by an internal sync.RWMutex.
//
// A skip list is a probabilistic ordered data structure with multiple levels
// of linked lists; each higher level acts as an "express lane" that skips
// over roughly half of the nodes below it.  Basic operations:
//
//	Insert - add a new element to the list (duplicates replace the old data).	O(log2(n)) expected
//	Delete - remove an element from the list.					O(log2(n)) expected
//	Search - return the stored element equal to the probe, or nil.			O(log2(n)) expected
//	FindMin - return the smallest element in the list.				O(1)
//	FindMax - return the largest element in the list.				O(log2(n)) expected
//	DeleteAtHead - remove the smallest element.					O(1)
//	DeleteAtTail - remove the largest element.					O(log2(n)) expected
//	IsEmpty - report whether the list is empty.					O(1)
//	Length - return the number of elements in the list.				O(1)
//	Truncate - remove all elements from the list.					O(1)
//	All/Backward - Go 1.23 range-over-func iterators over a snapshot of the list.	O(n)
//
// This is the thread-safe version of github.com/pschlump/pluto/skip_list;
// both packages expose the identical API.  The iterators (All/Backward)
// operate on a consistent snapshot taken under the read lock, so the loop
// body never runs while the lock is held.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package skip_list_ts

import (
	"fmt"
	"io"
	"math/rand/v2"
	"sync"

	"github.com/pschlump/pluto/comparable"
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
type SkipListNode[T comparable.Comparable] struct {
	data    *T
	forward []*SkipListNode[T]
}

// SkipList is a generic skip list of items that implement
// comparable.Comparable.  It is safe for concurrent use.  The zero value is
// ready to use.
type SkipList[T comparable.Comparable] struct {
	head   *SkipListNode[T] // Sentinel node; its data is nil.
	level  int              // Highest level currently in use (0 when empty).
	length int              // Number of nodes in the list.
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// ensureHead lazily allocates the sentinel head node.  The caller must hold
// the write lock.
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
// less than item.  It is shared by Insert, Search and Delete.  The caller
// must hold at least the read lock and have called ensureHead.
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

// isEmpty is the lock-free body of IsEmpty; the caller must hold at least
// the read lock.
func (tt *SkipList[T]) isEmpty() bool {
	return tt.length == 0
}

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (tt *SkipList[T]) IsEmpty() bool {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.isEmpty()
}

// Length will return the number of elements in the list.
// Complexity is O(1).
func (tt *SkipList[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.length
}

// Truncate removes all data from the list.
// Complexity is O(1).
func (tt *SkipList[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
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
	tt.length++
}

// Search will return a copy of the item in the list that Compare-equal to
// `item`, or nil if it is not present.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Search(item T) (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
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
		cp := *cur.data
		return &cp
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	if tt.isEmpty() {
		return false
	}
	return tt.deleteLocked(item)
}

// FindMin returns a copy of the smallest item in the list, or nil if it is
// empty.
// Complexity is O(1).
func (tt *SkipList[T]) FindMin() (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
		return nil
	}
	cp := *tt.head.forward[0].data
	return &cp
}

// FindMax returns a copy of the largest item in the list, or nil if it is
// empty.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) FindMax() (rv *T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
		return nil
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil {
			cur = cur.forward[i]
		}
	}
	cp := *cur.data
	return &cp
}

// DeleteAtHead removes the smallest item in the list.  It returns false if
// the list is empty.
// Complexity is O(1) amortized (proportional to the removed node's level).
func (tt *SkipList[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("skip list should not be nil")
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	if tt.isEmpty() {
		return false
	}
	node := tt.head.forward[0]
	for i := 0; i < len(node.forward); i++ {
		tt.head.forward[i] = node.forward[i]
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
	tt.lock.Lock()
	defer tt.lock.Unlock()
	if tt.isEmpty() {
		return false
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil {
			cur = cur.forward[i]
		}
	}
	return tt.deleteLocked(*cur.data)
}

// deleteLocked is the lock-free body of Delete; the caller must hold the
// write lock.
func (tt *SkipList[T]) deleteLocked(item T) (found bool) {
	update := tt.findPath(item)
	node := update[0].forward[0]
	if node == nil || (*node.data).Compare(item) != 0 {
		return false
	}
	for i := 0; i < tt.level; i++ {
		if update[i].forward[i] == node {
			update[i].forward[i] = node.forward[i]
		}
	}
	for tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		tt.level--
	}
	tt.length--
	return true
}

// toSlice returns a snapshot of the list data in ascending sequence.
// Complexity is O(n).
func (tt *SkipList[T]) toSlice() (items []T) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
		return nil
	}
	for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
		items = append(items, *cur.data)
	}
	return
}

// Dump writes a per-level picture of the list to `w` for debugging.  The
// bottom level (L0) lists every node; each higher level lists only the nodes
// promoted to that level.
// Complexity is O(n).
func (tt *SkipList[T]) Dump(w io.Writer) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() {
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

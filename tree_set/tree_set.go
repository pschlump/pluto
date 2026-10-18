/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tree_set implements TreeSet, a sorted set of unique elements —
// the algs4 SET analog with Go generics and no interfaces.  It is a thin
// facade over github.com/pschlump/pluto/avl_tree (the
// priority_queue-over-heap composition precedent): every operation
// delegates to a wrapped AvlTree, so Insert, Delete and Search are
// O(log₂ n) worst case regardless of insertion order, and the whole
// surface is value-typed — elements are stored and returned by value,
// never boxed into an interface.
//
// Sets of naturally ordered element types (all integers, floats and
// strings) are created with NewTreeSet, which orders elements with the
// built-in < and > operators of the element type.  Sets of any other
// type — including structs ordered by a single field — are created with
// NewTreeSetFunc, which takes a caller supplied comparison function;
// the element type does not have to implement any interface.
//
// Basic operations on a TreeSet:
//
//	Insert — add an element; a duplicate replaces the existing one.	O(log₂ n)
//	Delete — remove an element.										O(log₂ n)
//	Search — return the stored element equal to the probe.			O(log₂ n)
//	Contains — the found flag of Search, spelled as a predicate.	O(log₂ n)
//	FindMin / FindMax — smallest / largest element.					O(log₂ n)
//	Floor / Ceil — largest ≤ key / smallest ≥ key.					O(log₂ n)
//	DeleteAtHead / DeleteAtTail — remove the smallest / largest.		O(log₂ n)
//	Index — the pos-th element, ascending.							O(n)
//	Range — iterate the elements with lo ≤ x ≤ hi, ascending.		O(n)
//	Len / Length / IsEmpty / Truncate — size and reset.				O(1) / O(1)
//
//	Batch operations (set-capabilities plan §3):
//
//	InsertSlice / InsertIter — insert each; true if any was new.	O(k·log₂ n)
//	DeleteSlice / DeleteFunc — delete each / by predicate.			O(k·log₂ n)
//	ContainsSlice / ContainsAny — all / at least one present.		O(k·log₂ n)
//
//	Conveniences:
//
//	Slice — fresh []T of the elements, ascending.					O(n)
//	String / StringFunc — render the elements: "[a b c]".			O(n)
//	Equal — same element set as another set (membership).			O(n log₂ n)
//	Clone — deep copy of the set, its own nodes.					O(n)
//
//	Set algebra (avl's destination-form contract, verbatim):
//
//	Union / Intersect / Minus / SymmetricDifference — ts = yy ∪/∩/−/△ zz.
//	                                                                    O((n+m)·log₂(n+m))
//
//	Relations (receiver-first):
//
//	IsSubsetOf / IsProperSubsetOf / IsSupersetOf /					O(n·log₂ m)
//	IsProperSupersetOf / IsDisjointWith.
//
//	Iteration:
//
//	All / Backward — range-over-func iterators, ascending / descending.	O(n)
//	WalkInOrder / WalkPreOrder / WalkPostOrder — callback traversals.	O(n)
//
//	JSON: a TreeSet marshals as a JSON array of its elements in
//	ascending order (the canonical form two equal sets encode
//	identically); Unmarshal replaces the contents and keeps the
//	comparison function.
//
// A nil *TreeSet and the zero value both behave as an empty set for
// every operation that has a sane answer: Search finds nothing, Delete
// and DeleteAtHead/DeleteAtTail return false, FindMin, FindMax, Floor,
// Ceil and Index report not-found, Len is 0, the walks and iterators
// visit nothing, and the set algebra leaves the receiver empty.
//
// The package panics in exactly these situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewTreeSetFunc(nil) / FromSliceFunc(nil, …) — nil comparison
//	function, caught at construction.
//	Insert — on a nil set or a zero-value set (no comparison function);
//	the message names the constructors.  The batch inserters
//	InsertSlice/InsertIter and UnmarshalJSON raise the same contract
//	when, and only when, they would actually store an element — an empty
//	batch is tolerated everywhere.
//
// TreeSet is not safe for concurrent use; a mutex-guarded twin,
// tree_set_ts, has the same interface.
package tree_set

import (
	"cmp"
	"fmt"
	"iter"
	"strings"

	"github.com/pschlump/pluto/avl_tree"
)

// TreeSet is a sorted set of unique elements, ordered by its comparison
// function.  Use NewTreeSet for naturally ordered element types (numbers,
// strings) or NewTreeSetFunc for a caller supplied comparison function.
// The zero value is an empty set for reads; Insert on it panics — create
// sets with the constructors.
type TreeSet[T any] struct {
	inner *avl_tree.AvlTree[T]

	// cmp is the same comparison function the wrapped tree was built
	// with, kept as the facade's own copy (the priority_queue precedent:
	// the wrapped structure does not export its comparator).  Range
	// compares bounds with it, and the set algebra syncs it when a
	// zero-value set adopts an operand's ordering.
	cmp func(a, b T) int
}

// ApplyFunction is the callback type of the Walk* functions, an alias of
// avl_tree's so a callback written for either package works with both.
type ApplyFunction[T any] = avl_tree.ApplyFunction[T]

// -------------------------------------------------------------------------------------------------------

// NewTreeSet creates a new, empty set for any naturally ordered element
// type (all integers, floats and strings — cmp.Ordered).  Ordering uses
// the built-in < and > operators of T; no interface and no boxing is
// involved.
// Complexity is O(1).
func NewTreeSet[T cmp.Ordered]() *TreeSet[T] {
	return newTreeSet[T](avl_tree.Compare[T])
}

// NewTreeSetFunc creates a new, empty set that orders elements with the
// caller supplied comparison function fx — for example a struct ordered
// by one of its fields.  fx must return a negative value if a sorts
// before b, 0 if the two are duplicates and a positive value if a sorts
// after b, and must order elements consistently.  It panics if fx is
// nil.
// Complexity is O(1).
func NewTreeSetFunc[T any](fx func(a, b T) int) *TreeSet[T] {
	if fx == nil {
		panic("tree_set: NewTreeSetFunc called with a nil comparison function")
	}
	return newTreeSet[T](fx)
}

// FromSlice creates a new set holding the elements of items, in the
// natural ordering of T.  Duplicates in items collapse to one element,
// exactly as inserting them in order would.
// Complexity is O(k·log₂ k), k = len(items).
func FromSlice[T cmp.Ordered](items []T) *TreeSet[T] {
	set := NewTreeSet[T]()
	set.InsertSlice(items)
	return set
}

// FromSliceFunc is FromSlice with a caller supplied comparison function
// (the NewTreeSetFunc form).  It panics if fx is nil.
// Complexity is O(k·log₂ k), k = len(items).
func FromSliceFunc[T any](fx func(a, b T) int, items []T) *TreeSet[T] {
	if fx == nil {
		panic("tree_set: FromSliceFunc called with a nil comparison function")
	}
	set := NewTreeSetFunc[T](fx)
	set.InsertSlice(items)
	return set
}

func newTreeSet[T any](cmp func(a, b T) int) *TreeSet[T] {
	return &TreeSet[T]{inner: avl_tree.NewAvlTreeFunc[T](cmp), cmp: cmp}
}

// innerOf returns the wrapped tree, or nil for a nil set or a zero-value
// set.  Every avl_tree operation is nil-tolerant on a nil tree, so the
// delegation helpers below can call straight through it.
func (ts *TreeSet[T]) innerOf() *avl_tree.AvlTree[T] {
	if ts == nil {
		return nil
	}
	return ts.inner
}

// insertContractCheck applies the insert panic contract for the
// operations that store elements.  `caller` names the public method in
// the message.
func (ts *TreeSet[T]) insertContractCheck(caller string) {
	if ts == nil || ts.inner == nil {
		panic(fmt.Sprintf("tree_set: %s called on a nil or zero-value set (create the set with NewTreeSet or NewTreeSetFunc)", caller))
	}
}

// -------------------------------------------------------------------------------------------------------
// Core operations — thin delegation.

// Insert adds item to the set.  If an equal element is already present
// the new item replaces it in place and false is returned; true is
// returned when a new element was added.
// Insert panics on a nil set or on a zero-value set (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty set.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Insert(item T) bool {
	ts.insertContractCheck("Insert")
	return ts.inner.Insert(item)
}

// Delete removes the element equal to find and reports whether one was
// present.  As with Search, `find` only needs the fields that the
// comparison function reads.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Delete(find T) bool {
	return ts.innerOf().Delete(find)
}

// Search returns the stored element equal to find, or false when the
// set does not hold one.  `find` only needs the fields that the
// comparison function reads: a probe with just the key fields set finds
// the element with the full data.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Search(find T) (item T, found bool) {
	return ts.innerOf().Search(find)
}

// Contains reports whether the set holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Contains(find T) bool {
	return ts.innerOf().Contains(find)
}

// Len returns the number of elements in the set.
// Complexity is O(1).
func (ts *TreeSet[T]) Len() int {
	return ts.innerOf().Len()
}

// Length is an alias for Len; it returns the number of elements in the
// set.
// Complexity is O(1).
func (ts *TreeSet[T]) Length() int {
	return ts.innerOf().Len()
}

// IsEmpty returns true if the set has no elements.
// Complexity is O(1).
func (ts *TreeSet[T]) IsEmpty() bool {
	return ts.innerOf().IsEmpty()
}

// Truncate removes all elements.  The comparison function is kept, so
// the set remains usable and can simply be refilled.  A nil set or a
// zero-value set is left as it is.
// Complexity is O(1).
func (ts *TreeSet[T]) Truncate() {
	if ts == nil || ts.inner == nil {
		return // nothing to clear
	}
	ts.inner.Truncate()
}

// FindMin returns the smallest element of the set, or false if the set
// is empty.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) FindMin() (item T, found bool) {
	return ts.innerOf().FindMin()
}

// FindMax returns the largest element of the set, or false if the set
// is empty.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) FindMax() (item T, found bool) {
	return ts.innerOf().FindMax()
}

// Floor returns the largest element that compares <= key, or found=false
// when key is less than every element.  As with Search, `key` only needs
// the fields that the comparison function reads.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Floor(key T) (rv T, found bool) {
	return ts.innerOf().Floor(key)
}

// Ceil returns the smallest element that compares >= key, or found=false
// when key is greater than every element.  As with Search, `key` only
// needs the fields that the comparison function reads.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) Ceil(key T) (rv T, found bool) {
	return ts.innerOf().Ceil(key)
}

// DeleteAtHead removes the smallest element of the set, returning false
// if the set is empty.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) DeleteAtHead() bool {
	return ts.innerOf().DeleteAtHead()
}

// DeleteAtTail removes the largest element of the set, returning false
// if the set is empty.
// Complexity is O(log₂ n).
func (ts *TreeSet[T]) DeleteAtTail() bool {
	return ts.innerOf().DeleteAtTail()
}

// Index returns the `pos`-th element of the set in ascending order
// (0-based), or false if pos is out of range.  It never panics.
// Complexity is O(n).
func (ts *TreeSet[T]) Index(pos int) (item T, found bool) {
	return ts.innerOf().Index(pos)
}

// -------------------------------------------------------------------------------------------------------
// Conveniences — the set-capabilities menu.

// Slice returns a fresh slice holding every element of the set in
// ascending order — the order All yields, deterministic with no sorting
// anywhere.  The slice is a copy: mutating it does not affect the set.
// A nil set yields nil; an empty set yields an empty (non-nil) slice.
// Complexity is O(n).
func (ts *TreeSet[T]) Slice() []T {
	if ts == nil {
		return nil
	}
	if ts.inner == nil {
		return []T{}
	}
	return ts.inner.Slice()
}

// String renders the set as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in ascending order:
// "[a b c]".  The in-order walk is already deterministic, so no sorting
// is involved.  A nil or empty set renders as "[]".  For a custom
// per-element rendering use StringFunc.
// Complexity is O(n).
func (ts *TreeSet[T]) String() string {
	return ts.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in ascending order and the
// formatted strings joined inside square brackets, so the output is
// deterministic.  A nil format function renders as "[]".
// Complexity is O(n).
func (ts *TreeSet[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range ts.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether ts and other hold the same set of elements:
// same length and every element of ts found in other.  Membership is
// compared, never shape.  Two empty sets are equal, a nil set equals an
// empty set, and Equal is symmetric on any pair that satisfies its
// precondition.
//
// Precondition: both sets must use consistent comparison functions (a
// nil set trivially does).  Functions are not comparable, so this cannot
// be checked — two sets built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n log₂ n).
func (ts *TreeSet[T]) Equal(other *TreeSet[T]) bool {
	if ts.Len() != other.Len() {
		return false
	}
	if ts.Len() == 0 {
		return true // both empty (either may be nil)
	}
	return ts.inner.Equal(other.innerOf())
}

// Clone returns a deep copy of the set: a fresh set with the same
// comparison function and a fresh copy of every node, preserving the
// receiver's shape.  No backing memory is shared, so mutating either set
// afterwards never affects the other.  A nil set clones to nil; a
// zero-value set clones to a fresh zero value.
// Complexity is O(n).
func (ts *TreeSet[T]) Clone() *TreeSet[T] {
	if ts == nil {
		return nil
	}
	if ts.inner == nil {
		return &TreeSet[T]{} // a zero-value set clones to a fresh zero value
	}
	return &TreeSet[T]{inner: ts.inner.Clone(), cmp: ts.cmp}
}

// -------------------------------------------------------------------------------------------------------
// Batch operations.

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (Insert replaces an equal element and
// reports false, exactly as a caller loop does).
//
// It follows the insert contract: a batch that would store an element
// into a nil set or a zero-value set panics with the standard
// insert-family message; an empty batch stores nothing and is tolerated
// everywhere.
// Complexity is O(k·log₂ n), k = len(items).
func (ts *TreeSet[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	ts.insertContractCheck("InsertSlice")
	return ts.inner.InsertSlice(items)
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between.  The iterator is drained to a slice before the
// first insert, so feeding a set its own All() is safe.  It reports
// whether at least one element was new and follows the same insert
// contract as InsertSlice.  A nil iterator inserts nothing and returns
// false.
// Complexity is O(k·log₂ n), k = number of yielded elements.
func (ts *TreeSet[T]) InsertIter(seq iter.Seq[T]) bool {
	if seq == nil {
		return false
	}
	var items []T
	for item := range seq {
		items = append(items, item)
	}
	if len(items) == 0 {
		return false
	}
	ts.insertContractCheck("InsertIter")
	return ts.inner.InsertSlice(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  Deleting is tolerated on a nil set (returns false),
// like Delete.
// Complexity is O(k·log₂ n), k = len(items).
func (ts *TreeSet[T]) DeleteSlice(items []T) bool {
	return ts.innerOf().DeleteSlice(items)
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The matching elements are collected
// first (an in-order snapshot) and deleted afterwards — the set is never
// modified while it is being walked.  A nil set, an empty set or a nil
// fx deletes nothing and returns false.
// Complexity is O(n log₂ n).
func (ts *TreeSet[T]) DeleteFunc(fx func(T) bool) bool {
	return ts.innerOf().DeleteFunc(fx)
}

// ContainsSlice reports whether every element of items is present in the
// set (an empty items slice is vacuously contained).  For "at least one
// present" use ContainsAny.
// Complexity is O(k·log₂ n), k = len(items).
func (ts *TreeSet[T]) ContainsSlice(items []T) bool {
	return ts.innerOf().ContainsSlice(items)
}

// ContainsAny reports whether at least one element of items is present
// in the set.  An empty items slice contains nothing.
// Complexity is O(k·log₂ n), k = len(items).
func (ts *TreeSet[T]) ContainsAny(items []T) bool {
	return ts.innerOf().ContainsAny(items)
}

// -------------------------------------------------------------------------------------------------------
// Set algebra — avl's destination-form contract, verbatim.

// adoptCmp gives a zero-value set a comparison function, adopting yy's
// (then zz's) facade-level copy.  It reports whether one is available.
func (ts *TreeSet[T]) adoptCmp(yy, zz *TreeSet[T]) bool {
	if ts.cmp == nil && yy != nil {
		ts.cmp = yy.cmp
	}
	if ts.cmp == nil && zz != nil {
		ts.cmp = zz.cmp
	}
	return ts.cmp != nil
}

// Union is a set union, ts = yy union zz.  If an element is in both yy
// and zz the one from zz is kept.  Operands are snapshotted first, so
// either may alias ts.  The result is ordered by ts's comparison
// function; a zero-value ts adopts yy's (then zz's).
//
// Precondition: the sets involved must use consistent comparison
// functions — two sets built by the same constructor over the same
// element type always satisfy it.
// Complexity is O((n+m)·log₂(n+m)).
func (ts *TreeSet[T]) Union(yy, zz *TreeSet[T]) {
	if ts == nil {
		return
	}
	ts.adoptCmp(yy, zz) // keep the facade's own copy in sync on adoption
	if ts.cmp == nil {
		ts.inner = nil // no ordering anywhere: the empty result is the zero value
		return
	}
	if ts.inner == nil {
		ts.inner = avl_tree.NewAvlTreeFunc[T](ts.cmp)
	}
	ts.inner.Union(yy.innerOf(), zz.innerOf()) // snapshots operands itself: alias-safe
}

// Intersect is a set intersection, ts = yy intersect zz.  Operands are
// snapshotted first, so either may alias ts.  The result is ordered by
// ts's comparison function; a zero-value ts adopts yy's (then zz's).
//
// Precondition: as for Union — consistent comparison functions.
// Complexity is O((n+m)·log₂(n+m)).
func (ts *TreeSet[T]) Intersect(yy, zz *TreeSet[T]) {
	if ts == nil {
		return
	}
	ts.adoptCmp(yy, zz)
	if ts.cmp == nil {
		ts.inner = nil
		return
	}
	if ts.inner == nil {
		ts.inner = avl_tree.NewAvlTreeFunc[T](ts.cmp)
	}
	ts.inner.Intersect(yy.innerOf(), zz.innerOf())
}

// Minus is a set minus, ts = yy - zz.  Operands are snapshotted first,
// so either may alias ts.  The result is ordered by ts's comparison
// function; a zero-value ts adopts yy's (then zz's).
//
// Precondition: as for Union — consistent comparison functions.
// Complexity is O((n+m)·log₂(n+m)).
func (ts *TreeSet[T]) Minus(yy, zz *TreeSet[T]) {
	if ts == nil {
		return
	}
	ts.adoptCmp(yy, zz)
	if ts.cmp == nil {
		ts.inner = nil
		return
	}
	if ts.inner == nil {
		ts.inner = avl_tree.NewAvlTreeFunc[T](ts.cmp)
	}
	ts.inner.Minus(yy.innerOf(), zz.innerOf())
}

// SymmetricDifference is a set symmetric difference, ts = yy △ zz = the
// elements in exactly one of yy and zz.  Operands are snapshotted
// first, so either may alias ts.  The result is ordered by ts's
// comparison function; a zero-value ts adopts yy's (then zz's).
//
// Precondition: as for Union — consistent comparison functions.
// Complexity is O((n+m)·log₂(n+m)).
func (ts *TreeSet[T]) SymmetricDifference(yy, zz *TreeSet[T]) {
	if ts == nil {
		return
	}
	ts.adoptCmp(yy, zz)
	if ts.cmp == nil {
		ts.inner = nil
		return
	}
	if ts.inner == nil {
		ts.inner = avl_tree.NewAvlTreeFunc[T](ts.cmp)
	}
	ts.inner.SymmetricDifference(yy.innerOf(), zz.innerOf())
}

// -------------------------------------------------------------------------------------------------------
// Relations — receiver-first, so they read correctly.

// IsSubsetOf reports whether every element of the receiver is also in
// other (the receiver ⊆ other).  The empty set is a subset of every
// set, including another empty set.
//
// Precondition: both sets must use consistent comparison functions (a
// nil set trivially does).
// Complexity is O(n·log₂ m), n = receiver's size, m = other's size.
func (ts *TreeSet[T]) IsSubsetOf(other *TreeSet[T]) bool {
	if ts.Len() > other.Len() {
		return false // the receiver holds more: it cannot be a subset
	}
	for _, v := range ts.Slice() {
		if !other.Contains(v) {
			return false
		}
	}
	return true
}

// IsProperSubsetOf reports whether the receiver ⊆ other and the two are
// not equal (the receiver's size is strictly less).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(n·log₂ m).
func (ts *TreeSet[T]) IsProperSubsetOf(other *TreeSet[T]) bool {
	return ts.Len() < other.Len() && ts.IsSubsetOf(other)
}

// IsSupersetOf reports whether every element of other is also in the
// receiver (the receiver ⊇ other).  It is other.IsSubsetOf(ts).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(m·log₂ n).
func (ts *TreeSet[T]) IsSupersetOf(other *TreeSet[T]) bool {
	return other.IsSubsetOf(ts)
}

// IsProperSupersetOf reports whether the receiver ⊃ other strictly (the
// receiver's size is strictly greater).  It is other.IsProperSubsetOf(ts).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(m·log₂ n).
func (ts *TreeSet[T]) IsProperSupersetOf(other *TreeSet[T]) bool {
	return other.IsProperSubsetOf(ts)
}

// IsDisjointWith reports whether the receiver and other hold no common
// element.  Two empty sets ARE disjoint (vacuously — they share
// nothing).
//
// Precondition: as for IsSubsetOf — consistent comparison functions.
// Complexity is O(n·log₂ m).
func (ts *TreeSet[T]) IsDisjointWith(other *TreeSet[T]) bool {
	for _, v := range ts.Slice() {
		if other.Contains(v) {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Iteration.

// All returns a range-over-func iterator that visits every element of
// the set in ascending order:
//
//	for item := range set.All() { ... }
//
// The set must not be modified while the iterator is being consumed —
// it walks the live nodes.  A nil set iterates as an empty one.
// Complexity is O(n).
func (ts *TreeSet[T]) All() iter.Seq[T] {
	return ts.innerOf().All()
}

// Backward returns a range-over-func iterator that visits every element
// of the set in descending order.  As with All, the set must not be
// modified while the iterator is being consumed.  A nil set iterates as
// an empty one.
// Complexity is O(n).
func (ts *TreeSet[T]) Backward() iter.Seq[T] {
	return ts.innerOf().Backward()
}

// Range returns an iterator over the elements x with lo <= x <= hi, in
// ascending sequence.  A range with lo > hi iterates as empty.  Like
// All, the iterator walks the live nodes: the set must not be modified
// while it is being consumed, and it supports early exit via break.
//
// The elements carry no index (a set has no ranks in iteration), so this
// is an iter.Seq[T], not the iter.Seq2[int, T] of skip_list.Range; and
// the walk starts at the smallest element, stopping once past hi —
// O(n) worst case, not skip_list's O(log₂ n + m) positioned descent.
// That is the documented price of the avl facade reuse.
// Complexity is O(n).
func (ts *TreeSet[T]) Range(lo, hi T) iter.Seq[T] {
	if ts == nil || ts.cmp == nil {
		return func(func(T) bool) {} // a nil or zero-value set iterates as an empty one
	}
	compare := ts.cmp
	if compare(lo, hi) > 0 {
		return func(func(T) bool) {}
	}
	return func(yield func(T) bool) {
		for v := range ts.inner.All() { // ascending
			if compare(v, hi) > 0 {
				return // past hi: nothing later can be in range
			}
			if compare(v, lo) >= 0 {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// WalkInOrder visits every element in ascending order.  Returning false
// from fx stops the walk.
// Complexity is O(n).
func (ts *TreeSet[T]) WalkInOrder(fx ApplyFunction[T]) {
	ts.innerOf().WalkInOrder(fx)
}

// WalkPreOrder visits every element in pre-order (node, left, right)
// order.  Returning false from fx stops the walk.
// Complexity is O(n).
func (ts *TreeSet[T]) WalkPreOrder(fx ApplyFunction[T]) {
	ts.innerOf().WalkPreOrder(fx)
}

// WalkPostOrder visits every element in post-order (left, right, node)
// order.  Returning false from fx stops the walk.
// Complexity is O(n).
func (ts *TreeSet[T]) WalkPostOrder(fx ApplyFunction[T]) {
	ts.innerOf().WalkPostOrder(fx)
}

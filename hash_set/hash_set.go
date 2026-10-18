/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hash_set implements HashSet[T] — the algs4 HashSET analog — a
// mathematical set of unique elements with the full set surface: core
// membership, batch operations, the set algebra (union, intersection,
// minus, symmetric difference), the subset/disjoint relations, and the
// set-convenience capabilities (materialization, rendering, deep copy,
// equality).  It is a facade over the general-purpose hash engine
// hash_grow.HashTab (auto-growing open addressing), composed the way
// priority_queue composes heap — the set is the table plus the surface,
// not a re-implementation.
//
// The element type implements no interface.  Sets of types that can be
// compared with == (the builtin comparable constraint) are created with
// NewHashSet, which hashes with the stdlib hash/maphash under a per-set
// random seed — equal values always hash equal, and no method has to be
// implemented on T.  Sets of any other type — or with field-based
// equality — are created with NewHashSetFunc, which takes a caller
// supplied equality function and hash function; the two must agree:
// whenever eq(a, b) is true, hash(a) and hash(b) must be equal.
// Elements are stored and returned by value (T, not *T).
//
// Operations:
//
//	Insert / Delete / Search / Contains — core membership.				O(1) average
//	Len / IsEmpty / Truncate — size and reset.							O(1) / O(1) / O(n)
//	InsertSlice / InsertIter / DeleteSlice / DeleteFunc — batches.		O(k) average
//	ContainsSlice / ContainsAny — batch membership: all / at least one.	O(k) average
//	Slice — fresh []T of the elements, UNSPECIFIED order.				O(n)
//	String / StringFunc — deterministic rendering (sorted strings).		O(n log n)
//	Equal — same element set as another set (membership, never order).	O(n) average
//	Clone — deep copy, the same eq/hash pair, fresh backing.			O(n)
//	Union / Intersect / Minus / SymmetricDifference — the algebra.		O(n+m) expected
//	IsSubsetOf / IsProperSubsetOf / IsSupersetOf /
//	IsProperSupersetOf / IsDisjointWith — the relations.				O(n) average
//	All — Range-over-func iterator over the elements.					O(n)
//	Walk — Call a callback for each element.							O(n)
//	MarshalJSON / UnmarshalJSON — array-of-elements JSON.				O(n)
//
// The algebra follows the avl_tree set-operation contract verbatim:
// destination form (the receiver is truncated and rebuilt as the result),
// the operands are snapshotted first so either may alias the receiver
// (s.Union(s, s) is legal), and a zero-value receiver adopts an
// operand's equality/hash pair (yy's first, then zz's).  The relations
// are receiver-first (IsSubsetOf reads correctly) and the subset
// predicates are the usual ones: proper means subset and strictly
// shorter; two empty sets are disjoint (vacuously) and the empty set is
// a subset of everything.  Equal, the relations and the algebra all
// require both sets to use consistent equality/hash functions — the same
// documented, unchecked precondition the avl set operations rely on.
//
// A nil *HashSet and the zero value both behave as an empty set for
// every read: searches report not-found, deletes return false, the
// iterators visit nothing, Slice is nil and String is "[]".  The package
// panics in exactly the same situations hash_grow does, each message
// naming the fix:
//
//	NewHashSetFunc / FromSliceFunc with a nil equality or hash function — caught at construction.
//	Insert (and the storing batches, when they would actually store an element) on a nil set — a nil set cannot store an element.
//	Insert (and the storing batches) on a zero-value set — no equality/hash functions; the message names the constructors.
//
// This package is not suitable for concurrent usage; a mutex-guarded
// thread-safe twin, hash_set_ts, has the same interface plus Lock/Unlock
// and the Nl-prefixed single-element methods.
package hash_set

import (
	"fmt"
	"hash/maphash"
	"iter"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/hash_grow"
)

// defaultCapacity and defaultSaturation are the initial table shape every
// constructor uses; a saturation of 0 selects hash_grow's default of 0.5.
// The table doubles on its own from there, so no options machinery is
// exposed.
const (
	defaultCapacity   = 16
	defaultSaturation = 0.0
)

// HashSet is a set of T values with no duplicates, backed by the
// auto-growing open-addressed hash table hash_grow.HashTab.  Use
// NewHashSet for element types that support ==, or NewHashSetFunc for a
// caller supplied equality and hash function.  The zero value is an
// empty read-only set.
type HashSet[T any] struct {
	tab *hash_grow.HashTab[T]

	// eq reports whether two elements are considered the same, and hash
	// returns a hash for an element — the pair the underlying table also
	// stores, kept here so Clone and the algebra's operand adoption work
	// at the set level.  Both are set by the constructors (and by an
	// algebra adoption) and are the only things that know how to compare
	// and hash T.
	eq   func(a, b T) bool
	hash func(a T) uint64
}

// -------------------------------------------------------------------------------------------------------

// NewHashSet creates a new, empty set for any element type comparable
// with == (all scalars, strings, pointers, arrays and structs of
// comparable fields).  Elements are compared with the == operator and
// hashed with the stdlib hash/maphash using a per-set random seed — no
// method has to be implemented on T, and no element is ever boxed into
// an interface.
// Complexity is O(1) for the set; O(capacity) for the bucket allocation.
func NewHashSet[T comparable]() *HashSet[T] {
	var seed = maphash.MakeSeed()
	return newHashSet(
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
	)
}

// NewHashSetFunc creates a new, empty set that identifies elements with
// the caller supplied equality function and hashes them with the caller
// supplied hash function.  The two functions must agree: whenever
// eq(a, b) is true, hash(a) and hash(b) must be equal, otherwise Search
// and Delete can miss elements.  It panics if eq or hash is nil.
// Complexity is O(1) for the set; O(capacity) for the bucket allocation.
func NewHashSetFunc[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashSet[T] {
	if eq == nil {
		panic("hash_set: NewHashSetFunc called with a nil equality function")
	}
	if hash == nil {
		panic("hash_set: NewHashSetFunc called with a nil hash function")
	}
	return newHashSet(eq, hash)
}

// FromSlice creates a new set initialized with the elements of items —
// the hashicorp From analog.  Equal elements collapse the way repeated
// Insert calls do; a nil or empty slice yields an empty set.
// Complexity is O(k) average, k = len(items).
func FromSlice[T comparable](items []T) *HashSet[T] {
	ss := NewHashSet[T]()
	ss.InsertSlice(items)
	return ss
}

// FromSliceFunc is FromSlice for a caller supplied equality and hash
// function (which must agree — see NewHashSetFunc).  It panics if eq or
// hash is nil.
// Complexity is O(k) average, k = len(items).
func FromSliceFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, items []T) *HashSet[T] {
	if eq == nil {
		panic("hash_set: FromSliceFunc called with a nil equality function")
	}
	if hash == nil {
		panic("hash_set: FromSliceFunc called with a nil hash function")
	}
	ss := newHashSet(eq, hash)
	ss.InsertSlice(items)
	return ss
}

// newHashSet is the shared constructor body; the public constructors
// have already validated the pair.
func newHashSet[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashSet[T] {
	return &HashSet[T]{
		tab:  hash_grow.NewHashTabFunc(eq, hash, defaultCapacity, defaultSaturation),
		eq:   eq,
		hash: hash,
	}
}

// insertContractCheck applies the insert panic contract for the storing
// operations.  `caller` names the public method in the messages.
func (ss *HashSet[T]) insertContractCheck(caller string) {
	if ss == nil {
		panic(fmt.Sprintf("hash_set: %s called on a nil set (a nil set cannot store an element)", caller))
	}
	if ss.eq == nil || ss.hash == nil {
		panic(fmt.Sprintf("hash_set: %s called on a set with no equality/hash functions (create the set with NewHashSet or NewHashSetFunc)", caller))
	}
}

// Insert adds item to the set.  If an equal element is already present
// the new item replaces it and false is returned; true is returned when
// a new element was added.
// Insert panics on a nil set or on a zero-value set (no equality/hash
// functions); these are the only panics on non-constructor,
// non-batch calls in the package — every other operation treats both as
// an empty set.
// Complexity is O(1) average, O(n) worst case.
func (ss *HashSet[T]) Insert(item T) bool {
	ss.insertContractCheck("Insert")
	return ss.tab.Insert(item)
}

// Delete removes the element equal to find, if present, and reports
// whether it was found.  Deleting from a nil or zero-value set returns
// false.
// Complexity is O(1) average, O(n) worst case.
func (ss *HashSet[T]) Delete(find T) bool {
	if ss == nil || ss.tab == nil {
		return false
	}
	return ss.tab.Delete(find)
}

// Search returns the stored element equal to find, or the zero value of
// T and false when it is not present.  The element is returned by value;
// it does not alias the set's internals.
// Complexity is O(1) average, O(n) worst case.
func (ss *HashSet[T]) Search(find T) (rv T, found bool) {
	if ss == nil || ss.tab == nil {
		return
	}
	return ss.tab.Search(find)
}

// Contains reports whether the set holds an element equal to find.  It
// is the found flag of Search, spelled as a membership predicate.
// Complexity is O(1) average, O(n) worst case.
func (ss *HashSet[T]) Contains(find T) bool {
	_, found := ss.Search(find)
	return found
}

// Len returns the number of elements in the set.
// Complexity is O(1).
func (ss *HashSet[T]) Len() int {
	if ss == nil || ss.tab == nil {
		return 0
	}
	return ss.tab.Len()
}

// IsEmpty returns true if the set has no elements.
// Complexity is O(1).
func (ss *HashSet[T]) IsEmpty() bool {
	return ss.Len() == 0
}

// Truncate removes all elements from the set.  The table's capacity and
// the equality/hash functions are kept, so the set remains usable and
// can simply be refilled.
// Complexity is O(n).
func (ss *HashSet[T]) Truncate() {
	if ss == nil || ss.tab == nil {
		return
	}
	ss.tab.Truncate()
}

// Eq returns the set's stored equality function — the pair member the
// constructors install, Clone keeps and the algebra adopts.  It returns
// nil for a nil or zero-value set.  The thread-safe twin reads it (and
// Hash) to apply the insert contract and to adopt at the wrapper level.
// Complexity is O(1).
func (ss *HashSet[T]) Eq() func(a, b T) bool {
	if ss == nil {
		return nil
	}
	return ss.eq
}

// Hash returns the set's stored hash function; see Eq for the contract.
// Complexity is O(1).
func (ss *HashSet[T]) Hash() func(a T) uint64 {
	if ss == nil {
		return nil
	}
	return ss.hash
}

// -------------------------------------------------------------------------------------------------------
// Batch operations.

// InsertSlice inserts each element of items in order and reports whether
// at least one element was new (an equal present element is replaced and
// reports false, exactly as a caller loop does).  An empty batch stores
// nothing and is tolerated everywhere.
//
// It follows the insert contract: a batch that would store an element
// into a nil set or a zero-value set panics with the standard
// insert-family message.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (ss *HashSet[T]) InsertSlice(items []T) bool {
	if len(items) == 0 {
		return false // nothing to store: the insert contract never fires
	}
	ss.insertContractCheck("InsertSlice")
	return ss.tab.InsertSlice(items)
}

// InsertIter is InsertSlice over a Go 1.23 range-over-func iterator:
// every pluto container's All() or Values() feeds it directly, with no
// interface between — the interface-free InsertSet analog.  The iterator
// is drained before the first insert, so feeding a set its own All() is
// safe.  It reports whether at least one element was new and follows the
// same insert contract as InsertSlice.  A nil iterator inserts nothing
// and returns false.
// Complexity is O(k) average, k = number of yielded elements.
func (ss *HashSet[T]) InsertIter(seq iter.Seq[T]) bool {
	if seq == nil {
		return false
	}
	items := make([]T, 0)
	for item := range seq {
		items = append(items, item)
	}
	if len(items) == 0 {
		return false
	}
	ss.insertContractCheck("InsertIter")
	return ss.tab.InsertSlice(items)
}

// DeleteSlice deletes each element of items and reports whether at least
// one was present.  Deleting from a nil or zero-value set returns false.
// Complexity is O(k) average, O(n) worst case per element, k = len(items).
func (ss *HashSet[T]) DeleteSlice(items []T) bool {
	if ss == nil || ss.tab == nil {
		return false
	}
	return ss.tab.DeleteSlice(items)
}

// DeleteFunc deletes every element for which fx returns true and reports
// whether anything was deleted.  The matching elements are collected
// first and deleted afterwards (the underlying table's collect-then-
// delete — mandatory with its backward-shift deletion, not just
// tidiness).  A nil set, an empty set or a nil fx deletes nothing and
// returns false.
// Complexity is O(n) average.
func (ss *HashSet[T]) DeleteFunc(fx func(T) bool) bool {
	if ss == nil || ss.tab == nil {
		return false
	}
	return ss.tab.DeleteFunc(fx)
}

// ContainsSlice reports whether every element of items is present in the
// set (an empty items slice is vacuously contained).  For "at least one
// present" use ContainsAny.
// Complexity is O(k) average, k = len(items).
func (ss *HashSet[T]) ContainsSlice(items []T) bool {
	for _, item := range items {
		if !ss.Contains(item) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether at least one element of items is present
// in the set.  An empty items slice contains nothing.
// Complexity is O(k) average, k = len(items).
func (ss *HashSet[T]) ContainsAny(items []T) bool {
	for _, item := range items {
		if ss.Contains(item) {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------------------------------------------
// Set-convenience capabilities.

// Slice returns a fresh slice holding every element of the set.  The
// order is the underlying table's slot order, which depends on the
// per-set hash seed and the table's growth history, and is therefore
// UNSPECIFIED — sort the result when a fixed order is needed.  The slice
// is a copy: mutating it does not affect the set.  A nil set yields nil;
// an empty set yields an empty slice.
// Complexity is O(n).
func (ss *HashSet[T]) Slice() []T {
	if ss == nil || ss.tab == nil {
		return nil
	}
	return ss.tab.Slice()
}

// String renders the set as its elements formatted with fmt.Sprint, the
// formatted strings sorted ascending and joined with single spaces
// inside square brackets: "[a b c]".  Sorting the formatted strings —
// not the elements — is what makes the output deterministic: slot order
// depends on the per-set random hash seed and differs from run to run,
// but two sets holding the same elements always render the same string.
// A nil or empty set renders as "[]".  For a custom per-element
// rendering use StringFunc.
// Complexity is O(n log n).
func (ss *HashSet[T]) String() string {
	return ss.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are formatted, the strings sorted ascending and
// joined inside square brackets, so the output is deterministic whatever
// order the slots happen to be in.  A nil format function renders as
// "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n log n).
func (ss *HashSet[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	return g_lib.FormatSorted(ss.Slice(), format)
}

// Equal reports whether ss and other hold the same set of elements: same
// length and every element of ss found in other by Search.  Membership
// is compared, never iteration order.  Two empty sets are equal, a nil
// set equals an empty set, and Equal is symmetric on any pair that
// satisfies its precondition.
//
// Precondition: both sets must use consistent equality and hash
// functions (a nil set trivially does).  Functions are not comparable,
// so this cannot be checked — two sets built by the same constructor
// over the same element type always satisfy it.
// Complexity is O(n) average.
func (ss *HashSet[T]) Equal(other *HashSet[T]) bool {
	if ss.Len() != other.Len() {
		return false
	}
	if ss.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for _, v := range ss.Slice() {
		if !other.Contains(v) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the set: a fresh set with the same
// equality/hash functions and fresh backing storage — nothing is shared
// with the receiver, so mutating either set afterwards never affects the
// other.  A nil set clones to nil; a zero-value set clones to a fresh
// zero-value set.
// Complexity is O(n).
func (ss *HashSet[T]) Clone() *HashSet[T] {
	if ss == nil {
		return nil
	}
	dup := &HashSet[T]{eq: ss.eq, hash: ss.hash}
	if ss.tab != nil {
		dup.tab = ss.tab.Clone()
	}
	return dup
}

// -------------------------------------------------------------------------------------------------------
// The set algebra — the avl_tree set-operation contract, verbatim in
// spirit: destination form, snapshotted operands, zero-value adoption.

// algebraCapacity sizes a fresh table for n expected elements so the
// load factor starts under the saturation threshold and the rebuild does
// no immediate rehash (the table still grows on its own if a later
// insert pushes past the threshold).
func algebraCapacity(n int) int {
	if c := 2*n + 9; c > 5 {
		return c
	}
	return 5
}

// adopt picks the equality/hash pair an algebra rebuild uses: the
// receiver's own pair when it has one, otherwise yy's, otherwise zz's.
// A nil return means no operand had a usable pair either.
func (ss *HashSet[T]) adopt(yy, zz *HashSet[T]) (eq func(a, b T) bool, hash func(a T) uint64) {
	if ss.eq != nil && ss.hash != nil {
		return ss.eq, ss.hash
	}
	if yy != nil && yy.eq != nil && yy.hash != nil {
		return yy.eq, yy.hash
	}
	if zz != nil && zz.eq != nil && zz.hash != nil {
		return zz.eq, zz.hash
	}
	return nil, nil
}

// resetForAlgebra performs the destination-form step every algebra
// method starts with: snapshot-adopt the pair, drop the old contents.
// It reports whether a usable pair was found; with none, the receiver
// stays an empty zero-value set.
func (ss *HashSet[T]) resetForAlgebra(yy, zz *HashSet[T]) bool {
	eq, hash := ss.adopt(yy, zz)
	ss.tab = nil // the old contents go: the result is rebuilt from scratch
	ss.eq, ss.hash = eq, hash
	return eq != nil && hash != nil
}

// Union is a set union, ss = yy ∪ zz: the receiver is truncated and
// rebuilt with every element of yy and zz.  Operands are snapshotted
// first, so either may alias ss (s.Union(s, s) is legal).  A zero-value
// ss adopts yy's equality/hash pair (then zz's); a nil ss is a no-op,
// and nil operands contribute nothing.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (ss *HashSet[T]) Union(yy, zz *HashSet[T]) {
	if ss == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias ss
	if !ss.resetForAlgebra(yy, zz) {
		return
	}
	ss.tab = hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(a)+len(b)), defaultSaturation)
	for _, v := range a {
		ss.tab.Insert(v)
	}
	for _, v := range b {
		ss.tab.Insert(v)
	}
}

// Intersect is a set intersection, ss = yy ∩ zz: the receiver is
// truncated and rebuilt with the elements present in both yy and zz.
// Operands are snapshotted first, so either may alias ss.  A zero-value
// ss adopts yy's equality/hash pair (then zz's); a nil ss is a no-op,
// and nil operands contribute nothing (the result is empty).
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (ss *HashSet[T]) Intersect(yy, zz *HashSet[T]) {
	if ss == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias ss
	if !ss.resetForAlgebra(yy, zz) {
		return
	}
	ss.tab = hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(a)), defaultSaturation)
	inZZ := hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(b)), defaultSaturation)
	for _, v := range b {
		inZZ.Insert(v)
	}
	for _, v := range a {
		if inZZ.Contains(v) {
			ss.tab.Insert(v)
		}
	}
}

// Minus is a set minus, ss = yy − zz: the receiver is truncated and
// rebuilt with the elements of yy that are not in zz.  Operands are
// snapshotted first, so either may alias ss (s.Minus(s, s) empties s).
// A zero-value ss adopts yy's equality/hash pair (then zz's); a nil ss
// is a no-op, and nil operands contribute nothing.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (ss *HashSet[T]) Minus(yy, zz *HashSet[T]) {
	if ss == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias ss
	if !ss.resetForAlgebra(yy, zz) {
		return
	}
	ss.tab = hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(a)), defaultSaturation)
	inZZ := hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(b)), defaultSaturation)
	for _, v := range b {
		inZZ.Insert(v)
	}
	for _, v := range a {
		if !inZZ.Contains(v) {
			ss.tab.Insert(v)
		}
	}
}

// SymmetricDifference is a set symmetric difference, ss = yy △ zz — the
// elements in exactly one of yy and zz.  Operands are snapshotted first,
// so either may alias ss.  A zero-value ss adopts yy's equality/hash
// pair (then zz's); a nil ss is a no-op, and nil operands contribute
// nothing.  Together with Union, Intersect and Minus this completes the
// set algebra.
//
// Precondition: the sets must use consistent equality/hash functions.
// Complexity is O(n+m) expected, n = |yy|, m = |zz|.
func (ss *HashSet[T]) SymmetricDifference(yy, zz *HashSet[T]) {
	if ss == nil {
		return
	}
	a, b := yy.Slice(), zz.Slice() // snapshots first: operands may alias ss
	if !ss.resetForAlgebra(yy, zz) {
		return
	}
	ss.tab = hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(a)+len(b)), defaultSaturation)
	inYY := hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(a)), defaultSaturation)
	inZZ := hash_grow.NewHashTabFunc(ss.eq, ss.hash, algebraCapacity(len(b)), defaultSaturation)
	for _, v := range a {
		inYY.Insert(v)
	}
	for _, v := range b {
		inZZ.Insert(v)
	}
	for _, v := range a {
		if !inZZ.Contains(v) {
			ss.tab.Insert(v)
		}
	}
	for _, v := range b {
		if !inYY.Contains(v) {
			ss.tab.Insert(v)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// The subset/disjoint relations.

// IsSubsetOf reports whether every element of ss is present in other.
// The empty set is a subset of everything (including a nil set); a nil
// ss is the empty set.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |ss|.
func (ss *HashSet[T]) IsSubsetOf(other *HashSet[T]) bool {
	if ss.Len() > other.Len() {
		return false // a longer set cannot be a subset
	}
	for _, v := range ss.Slice() {
		if !other.Contains(v) {
			return false
		}
	}
	return true
}

// IsProperSubsetOf reports whether ss is a subset of other AND strictly
// shorter — subset with at least one element of other left over.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |ss|.
func (ss *HashSet[T]) IsProperSubsetOf(other *HashSet[T]) bool {
	return ss.Len() < other.Len() && ss.IsSubsetOf(other)
}

// IsSupersetOf reports whether every element of other is present in ss —
// other.IsSubsetOf(ss).
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(m) expected, m = |other|.
func (ss *HashSet[T]) IsSupersetOf(other *HashSet[T]) bool {
	return other.IsSubsetOf(ss)
}

// IsProperSupersetOf reports whether ss is a superset of other AND
// strictly longer.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(m) expected, m = |other|.
func (ss *HashSet[T]) IsProperSupersetOf(other *HashSet[T]) bool {
	return other.IsProperSubsetOf(ss)
}

// IsDisjointWith reports whether ss and other share no element.  Two
// empty sets are disjoint (there is no shared element), as is any pair
// where either side is empty or nil.
//
// Precondition: both sets must use consistent equality/hash functions.
// Complexity is O(n) expected, n = |ss|.
func (ss *HashSet[T]) IsDisjointWith(other *HashSet[T]) bool {
	for _, v := range ss.Slice() {
		if other.Contains(v) {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Iteration.

// ApplyFunction is the callback type for Walk, an alias of the
// underlying table's (the bucket position and the element stored
// there; returning false stops the walk).
type ApplyFunction[T any] = hash_grow.ApplyFunction[T]

// All returns an iterator (Go 1.23 range-over-func) over the elements of
// the set.  Typical use:
//
//	for item := range ss.All() { ... }
//
// The order is the table's slot order, which depends on the per-set hash
// seed and varies from process to process — never assert a fixed order.
// Complexity is O(n).
func (ss *HashSet[T]) All() iter.Seq[T] {
	if ss == nil || ss.tab == nil {
		return func(func(T) bool) {}
	}
	return ss.tab.Values()
}

// Walk calls fx for each element of the set, in slot order, until all
// elements have been visited or fx returns false.  It returns true if
// the walk ran to completion.  The order varies from process to process
// (per-set hash seed).
// Complexity is O(n).
func (ss *HashSet[T]) Walk(fx ApplyFunction[T]) (b bool) {
	if ss == nil || ss.tab == nil {
		return true
	}
	return ss.tab.Walk(fx)
}

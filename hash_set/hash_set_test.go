/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// expectPanic runs fx and fails the test unless it panics with a message
// containing want.
func expectPanic(t *testing.T, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected a panic containing %q, got none", want)
			return
		}
		got, ok := r.(string)
		if !ok {
			t.Errorf("expected a string panic containing %q, got %#v", want, r)
			return
		}
		if !strings.Contains(got, want) {
			t.Errorf("expected a panic containing %q, got %q", want, got)
		}
	}()
	fx()
}

// asSet drains a set into the reference model: a map[T]bool.
func asSet[T comparable](t *testing.T, ss *HashSet[T]) map[T]bool {
	t.Helper()
	m := make(map[T]bool)
	for _, v := range ss.Slice() {
		if m[v] {
			t.Fatalf("duplicate element %v in set Slice", v)
		}
		m[v] = true
	}
	if len(m) != ss.Len() {
		t.Fatalf("Len = %d, Slice holds %d elements", ss.Len(), len(m))
	}
	return m
}

// sameContent verifies that ss holds exactly the keys of want.
func sameContent[T comparable](t *testing.T, ss *HashSet[T], want map[T]bool) {
	t.Helper()
	got := asSet(t, ss)
	if len(got) != len(want) {
		t.Fatalf("set holds %d elements, want %d", len(got), len(want))
	}
	for v := range want {
		if !got[v] {
			t.Fatalf("element %v missing from set", v)
		}
	}
}

// The model algebra — the reference answers for the property tests.
func modelUnion[T comparable](a, b map[T]bool) map[T]bool {
	m := make(map[T]bool, len(a)+len(b))
	for v := range a {
		m[v] = true
	}
	for v := range b {
		m[v] = true
	}
	return m
}

func modelIntersect[T comparable](a, b map[T]bool) map[T]bool {
	m := make(map[T]bool)
	for v := range a {
		if b[v] {
			m[v] = true
		}
	}
	return m
}

func modelMinus[T comparable](a, b map[T]bool) map[T]bool {
	m := make(map[T]bool)
	for v := range a {
		if !b[v] {
			m[v] = true
		}
	}
	return m
}

func modelSymDiff[T comparable](a, b map[T]bool) map[T]bool {
	m := make(map[T]bool)
	for v := range a {
		if !b[v] {
			m[v] = true
		}
	}
	for v := range b {
		if !a[v] {
			m[v] = true
		}
	}
	return m
}

func TestHashSetBasic(t *testing.T) {
	ss := NewHashSet[int]()
	if !ss.IsEmpty() || ss.Len() != 0 {
		t.Errorf("new set not empty: IsEmpty=%v Len=%d", ss.IsEmpty(), ss.Len())
	}
	if ss.Contains(1) {
		t.Errorf("new set contains 1")
	}
	if _, found := ss.Search(1); found {
		t.Errorf("Search on empty set found")
	}

	if !ss.Insert(1) || !ss.Insert(2) || !ss.Insert(3) {
		t.Errorf("first inserts must report true")
	}
	if ss.Insert(2) {
		t.Errorf("duplicate insert must report false")
	}
	if ss.Len() != 3 {
		t.Errorf("Len = %d, want 3", ss.Len())
	}
	for _, v := range []int{1, 2, 3} {
		if !ss.Contains(v) {
			t.Errorf("missing %d", v)
		}
		if _, found := ss.Search(v); !found {
			t.Errorf("Search missed %d", v)
		}
	}

	if !ss.Delete(2) {
		t.Errorf("Delete of present element must report true")
	}
	if ss.Delete(2) {
		t.Errorf("Delete of absent element must report false")
	}
	if ss.Len() != 2 || ss.Contains(2) {
		t.Errorf("delete left the set wrong: Len=%d Contains2=%v", ss.Len(), ss.Contains(2))
	}

	ss.Truncate()
	if ss.Len() != 0 || !ss.IsEmpty() {
		t.Errorf("Truncate left the set non-empty")
	}
	// The set stays usable after Truncate (capacity and functions kept).
	if !ss.Insert(9) || !ss.Contains(9) {
		t.Errorf("set unusable after Truncate")
	}
}

// ciNode is the house-shaped test type: S is what a case-insensitive
// equality compares, N is satellite data it ignores.  The hash agrees
// with the equality (it lowercases first) — the pair is used by the
// Func, FromSlice and JSON tests.
type ciNode struct {
	S string
	N int
}

func ciEq(a, b ciNode) bool { return strings.EqualFold(a.S, b.S) }

func ciHash(a ciNode) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range strings.ToLower(a.S) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

func TestHashSetFuncReplacesValue(t *testing.T) {
	ss := NewHashSetFunc(func(a, b ciNode) bool { return a.S == b.S },
		func(a ciNode) uint64 { return uint64(len(a.S)) + 1 }) // consistent with eq
	ss.Insert(ciNode{"a", 1})
	if ss.Insert(ciNode{"a", 2}) {
		t.Errorf("duplicate insert must report false")
	}
	got, found := ss.Search(ciNode{"a", 0})
	if !found || got.N != 2 {
		t.Errorf("Search = (%+v, %v), want N=2 — the new value must replace the old", got, found)
	}
	if ss.Len() != 1 {
		t.Errorf("Len = %d, want 1", ss.Len())
	}
}

func TestHashSetCaseInsensitive(t *testing.T) {
	ss := NewHashSetFunc(ciEq, ciHash)
	if !ss.Insert(ciNode{"Alpha", 1}) {
		t.Errorf("first insert must report true")
	}
	if ss.Insert(ciNode{"ALPHA", 2}) {
		t.Errorf("case-insensitive duplicate insert must report false")
	}
	got, found := ss.Search(ciNode{"alpha", 0})
	if !found || got.N != 2 {
		t.Errorf("Search = (%+v, %v), want N=2", got, found)
	}
	if !ss.Delete(ciNode{"aLpHa", 0}) {
		t.Errorf("Delete must match case-insensitively")
	}
	if ss.Len() != 0 {
		t.Errorf("Len = %d, want 0", ss.Len())
	}
}

func TestHashSetFromSlice(t *testing.T) {
	ss := FromSlice([]int{1, 2, 2, 3, 1})
	if ss.Len() != 3 {
		t.Errorf("Len = %d, want 3 (duplicates collapse)", ss.Len())
	}
	sameContent(t, ss, map[int]bool{1: true, 2: true, 3: true})

	empty := FromSlice[int](nil)
	if empty.Len() != 0 || empty.tab == nil {
		t.Errorf("FromSlice(nil) must yield a usable empty set")
	}
	if !empty.Insert(1) {
		t.Errorf("FromSlice(nil) set must be insertable")
	}

	ssf := FromSliceFunc(ciEq, ciHash, []ciNode{{"a", 1}, {"A", 2}})
	if ssf.Len() != 1 {
		t.Errorf("Len = %d, want 1 (case-insensitive eq collapses a/A)", ssf.Len())
	}
}

func TestHashSetBatches(t *testing.T) {
	ss := NewHashSet[int]()

	if ss.InsertSlice(nil) {
		t.Errorf("empty InsertSlice must report false")
	}
	if !ss.InsertSlice([]int{5, 6, 5}) {
		t.Errorf("InsertSlice with new elements must report true")
	}
	if ss.InsertSlice([]int{5}) {
		t.Errorf("InsertSlice of only-present elements must report false")
	}
	sameContent(t, ss, map[int]bool{5: true, 6: true})

	if !ss.InsertIter(slices.Values([]int{7, 8})) {
		t.Errorf("InsertIter with new elements must report true")
	}
	if ss.InsertIter(nil) {
		t.Errorf("nil InsertIter must report false")
	}
	if ss.InsertIter(func(yield func(int) bool) {}) {
		t.Errorf("empty InsertIter must report false")
	}
	// Feeding a set its own All() is safe (drained before the inserts).
	if ss.InsertIter(ss.All()) {
		t.Errorf("InsertIter of the receiver's own elements must report false")
	}

	if !ss.ContainsSlice([]int{5, 6, 7, 8}) {
		t.Errorf("ContainsSlice of all-present must be true")
	}
	if ss.ContainsSlice([]int{5, 99}) {
		t.Errorf("ContainsSlice with an absent element must be false")
	}
	if !ss.ContainsSlice(nil) {
		t.Errorf("empty ContainsSlice is vacuously true")
	}
	if !ss.ContainsAny([]int{99, 5}) {
		t.Errorf("ContainsAny with one present must be true")
	}
	if ss.ContainsAny([]int{99, 98}) {
		t.Errorf("ContainsAny with none present must be false")
	}
	if ss.ContainsAny(nil) {
		t.Errorf("empty ContainsAny must be false")
	}

	if !ss.DeleteSlice([]int{6, 8, 100}) {
		t.Errorf("DeleteSlice with a hit must report true")
	}
	if ss.DeleteSlice([]int{100}) {
		t.Errorf("DeleteSlice with no hit must report false")
	}
	sameContent(t, ss, map[int]bool{5: true, 7: true})
}

func TestHashSetDeleteFuncHalf(t *testing.T) {
	ss := NewHashSet[int]()
	for i := 0; i < 1000; i++ {
		ss.Insert(i)
	}
	if !ss.DeleteFunc(func(v int) bool { return v%2 == 0 }) {
		t.Errorf("DeleteFunc with matches must report true")
	}
	if ss.Len() != 500 {
		t.Fatalf("Len = %d, want 500", ss.Len())
	}
	// Full re-verification: every odd element still present, every even
	// one gone — the collect-then-delete must not have lost anything.
	for i := 0; i < 1000; i++ {
		if ss.Contains(i) != (i%2 == 1) {
			t.Fatalf("Contains(%d) = %v, want %v", i, ss.Contains(i), i%2 == 1)
		}
	}
	if ss.DeleteFunc(nil) {
		t.Errorf("nil DeleteFunc must report false")
	}
	if ss.DeleteFunc(func(int) bool { return false }) {
		t.Errorf("DeleteFunc with no matches must report false")
	}
}

func TestHashSetIteration(t *testing.T) {
	ss := FromSlice([]int{3, 1, 4, 1, 5})

	count := 0
	for v := range ss.All() {
		if !ss.Contains(v) {
			t.Errorf("All yielded %d, not a member", v)
		}
		count++
	}
	if count != 4 {
		t.Errorf("All visited %d elements, want 4", count)
	}

	seen := make(map[int]bool)
	complete := ss.Walk(func(_ int, v int) bool {
		if seen[v] {
			t.Errorf("Walk visited %d twice", v)
		}
		seen[v] = true
		return true
	})
	if !complete || len(seen) != 4 {
		t.Errorf("Walk complete=%v visited %d, want complete over 4", complete, len(seen))
	}

	// Early stop: false stops the walk and reports not-complete.
	stopped := ss.Walk(func(_ int, _ int) bool { return false })
	if stopped {
		t.Errorf("Walk stopped by callback must report false")
	}
}

func TestHashSetSliceSemantics(t *testing.T) {
	var nilSet *HashSet[int]
	if nilSet.Slice() != nil {
		t.Errorf("Slice on nil set must be nil")
	}

	empty := NewHashSet[int]()
	if got := empty.Slice(); got == nil || len(got) != 0 {
		t.Errorf("Slice on empty set = %#v, want empty non-nil", got)
	}

	ss := FromSlice([]int{1, 2, 3})
	got := ss.Slice()
	got[0] = 99 // a copy: mutating it must not touch the set
	if ss.Contains(99) {
		t.Errorf("Slice aliases the set's storage")
	}
	sameContent(t, ss, map[int]bool{1: true, 2: true, 3: true})
}

func TestHashSetStringDeterminism(t *testing.T) {
	// Two separately constructed sets hash under two different random
	// maphash seeds, so slot order differs — but String sorts the
	// formatted strings, so the renderings must agree.
	items := []int{10, 2, 33, 4, 5555, 6, 7, 8, 9, 1}
	a := FromSlice(items)
	b := FromSlice(slices.Clone(items))
	if gotA, gotB := a.String(), b.String(); gotA != gotB {
		t.Errorf("String not deterministic across sets: %q vs %q", gotA, gotB)
	}
	if got := a.String(); got != "[1 10 2 33 4 5555 6 7 8 9]" {
		t.Errorf("String = %q, want the formatted strings sorted", got)
	}

	var nilSet *HashSet[int]
	if got := nilSet.String(); got != "[]" {
		t.Errorf("String on nil set = %q, want []", got)
	}
	if got := NewHashSet[int]().String(); got != "[]" {
		t.Errorf("String on empty set = %q, want []", got)
	}
	if got := a.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestHashSetEqual(t *testing.T) {
	base := []int{1, 2, 3, 4, 5, 6, 7}
	a := FromSlice(base)
	b := FromSlice(base) // separately seeded: different slot order, same content
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("sets over the same elements must be equal, both directions")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflexive")
	}

	// Same insertion content in a different order still equals.
	c := NewHashSet[int]()
	for i := len(base) - 1; i >= 0; i-- {
		c.Insert(base[i])
	}
	if !a.Equal(c) {
		t.Errorf("insertion order must not matter")
	}

	d := FromSlice([]int{1, 2, 3})
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("different lengths must not be equal")
	}
	e := FromSlice([]int{1, 2, 3, 4, 5, 6, 8})
	if a.Equal(e) || e.Equal(a) {
		t.Errorf("same length different content must not be equal")
	}

	var nilSet *HashSet[int]
	empty := NewHashSet[int]()
	if !nilSet.Equal(nilSet) || !nilSet.Equal(empty) || !empty.Equal(nilSet) {
		t.Errorf("nil equals empty and itself")
	}
}

func TestHashSetClone(t *testing.T) {
	a := FromSlice([]int{1, 2, 3})
	b := a.Clone()
	if !a.Equal(b) {
		t.Errorf("clone must hold the same elements")
	}

	// Deep copy, direction 1: mutating the original leaves the clone.
	a.Insert(4)
	a.Delete(1)
	sameContent(t, b, map[int]bool{1: true, 2: true, 3: true})

	// Deep copy, direction 2: mutating the clone leaves the original.
	sameContent(t, a, map[int]bool{2: true, 3: true, 4: true})
	b.Insert(99)
	sameContent(t, a, map[int]bool{2: true, 3: true, 4: true})

	// Clone keeps the functions: a Func-built clone still identifies
	// case-insensitively.
	f := FromSliceFunc(ciEq, ciHash, []ciNode{{"x", 1}})
	g := f.Clone()
	if g.Insert(ciNode{"X", 2}) {
		t.Errorf("clone lost the equality function")
	}

	var nilSet *HashSet[int]
	if nilSet.Clone() != nil {
		t.Errorf("nil clones to nil")
	}
	z := (&HashSet[int]{}).Clone() // zero value clones to a fresh zero value
	if z == nil || z.Len() != 0 || z.Eq() != nil || z.Hash() != nil {
		t.Errorf("zero-value clone shape wrong: %+v", z)
	}
}

func TestHashSetEqHashAccessors(t *testing.T) {
	ss := NewHashSet[int]()
	if ss.Eq() == nil || ss.Hash() == nil {
		t.Errorf("constructor-built set must expose its functions")
	}
	var zero HashSet[int]
	if zero.Eq() != nil || zero.Hash() != nil {
		t.Errorf("zero-value set must expose nil functions")
	}
	var nilSet *HashSet[int]
	if nilSet.Eq() != nil || nilSet.Hash() != nil {
		t.Errorf("nil set must expose nil functions")
	}
}

// algebraTriples builds overlapping random operand pairs and exercises
// every algebra method against the map-computed reference.
func TestHashSetAlgebraProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for round := 0; round < 20; round++ {
		var xs, ys []int
		for i := 0; i < 60; i++ {
			xs = append(xs, rng.Intn(50))
			ys = append(ys, rng.Intn(50))
		}
		a, b := FromSlice(xs), FromSlice(ys)
		ma, mb := asSet(t, a), asSet(t, b)

		u, v := NewHashSet[int](), NewHashSet[int]()
		u.Union(a, b)
		v.Union(b, a)
		sameContent(t, u, modelUnion(ma, mb))
		if !u.Equal(v) {
			t.Errorf("round %d: union not commutative", round)
		}

		i1, i2 := NewHashSet[int](), NewHashSet[int]()
		i1.Intersect(a, b)
		i2.Intersect(b, a)
		sameContent(t, i1, modelIntersect(ma, mb))
		if !i1.Equal(i2) {
			t.Errorf("round %d: intersection not commutative", round)
		}

		m := NewHashSet[int]()
		m.Minus(a, b)
		sameContent(t, m, modelMinus(ma, mb))

		d1, d2 := NewHashSet[int](), NewHashSet[int]()
		d1.SymmetricDifference(a, b)
		d2.SymmetricDifference(b, a)
		sameContent(t, d1, modelSymDiff(ma, mb))
		if !d1.Equal(d2) {
			t.Errorf("round %d: symmetric difference not commutative", round)
		}
		// Law: s △ t = (s ∪ t) − (s ∩ t).
		law := NewHashSet[int]()
		law.Minus(u, i1)
		if !d1.Equal(law) {
			t.Errorf("round %d: s△t != (s∪t)−(s∩t)", round)
		}
		// Idempotence through aliases: x ∪ x = x, x − x = ∅, x △ x = ∅,
		// x ∩ x = x.
		au, am, ad, ai := a.Clone(), a.Clone(), a.Clone(), a.Clone()
		au.Union(au, au)
		am.Minus(am, am)
		ad.SymmetricDifference(ad, ad)
		ai.Intersect(ai, ai)
		if !au.Equal(a) || !ai.Equal(a) {
			t.Errorf("round %d: x∪x or x∩x changed the set", round)
		}
		if am.Len() != 0 || ad.Len() != 0 {
			t.Errorf("round %d: x−x or x△x not empty", round)
		}
	}
}

func TestHashSetAlgebraAliases(t *testing.T) {
	a := FromSlice([]int{1, 2, 3, 4})
	b := FromSlice([]int{3, 4, 5, 6})

	// The destination may be one of the operands.
	s := a.Clone()
	s.Union(s, s)
	if !s.Equal(a) {
		t.Errorf("s.Union(s, s) changed the set")
	}
	s = a.Clone()
	s.Union(s, b)
	sameContent(t, s, modelUnion(asSet(t, a), asSet(t, b)))

	s = a.Clone()
	s.Intersect(s, b)
	sameContent(t, s, modelIntersect(asSet(t, a), asSet(t, b)))

	s = a.Clone()
	s.Minus(s, s)
	if s.Len() != 0 {
		t.Errorf("s.Minus(s, s) must empty the set")
	}
	s = a.Clone()
	s.Minus(b, s) // zz aliases ss: the snapshot must still see b's side
	sameContent(t, s, modelMinus(asSet(t, b), asSet(t, a)))

	s = a.Clone()
	s.SymmetricDifference(s, b)
	sameContent(t, s, modelSymDiff(asSet(t, a), asSet(t, b)))

	// Nil operands contribute nothing.
	s = a.Clone()
	s.Union(nil, nil)
	if s.Len() != 0 {
		t.Errorf("Union(nil, nil) must leave an empty result")
	}
	s = a.Clone()
	s.Union(nil, b)
	sameContent(t, s, asSet(t, b))
}

func TestHashSetAlgebraAdoption(t *testing.T) {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{3, 4, 5})

	// A zero-value receiver adopts an operand's pair and produces the
	// result.
	var r HashSet[int]
	r.Union(a, b)
	sameContent(t, &r, modelUnion(asSet(t, a), asSet(t, b)))
	if r.Eq() == nil || r.Hash() == nil {
		t.Errorf("zero-value receiver must adopt an operand's functions")
	}

	// yy is zero-value: zz's pair is adopted.
	var r2 HashSet[int]
	var zeroOp HashSet[int]
	r2.Minus(&zeroOp, b)
	if r2.Len() != 0 || r2.Eq() == nil {
		t.Errorf("adoption must fall through to zz")
	}

	// All operands zero-value: the receiver stays an empty zero value.
	var r3 HashSet[int]
	r3.Intersect(&zeroOp, &HashSet[int]{})
	if r3.Len() != 0 || r3.Eq() != nil || r3.tab != nil {
		t.Errorf("nothing to adopt: receiver must stay zero-valued")
	}

	// A nil receiver is a no-op, like avl_tree.
	var np *HashSet[int]
	np.Union(a, b)
	np.Intersect(a, b)
	np.Minus(a, b)
	np.SymmetricDifference(a, b)
	if np != nil {
		t.Errorf("nil receiver must stay nil")
	}

	// An already-built receiver keeps its OWN pair (yy's is not forced
	// onto it) — observable only as a non-nil pair plus correct content.
	keep := NewHashSet[int]()
	keep.Minus(a, b)
	sameContent(t, keep, modelMinus(asSet(t, a), asSet(t, b)))
}

func TestHashSetRelations(t *testing.T) {
	a := FromSlice([]int{1, 2, 3, 4})
	b := FromSlice([]int{1, 2, 3, 4})
	sub := FromSlice([]int{2, 4})
	other := FromSlice([]int{9, 10})
	var nilSet *HashSet[int]
	empty := NewHashSet[int]()

	if !a.IsSubsetOf(a) {
		t.Errorf("s is a subset of itself")
	}
	if a.IsProperSubsetOf(a) || a.IsProperSubsetOf(b) {
		t.Errorf("equal sets: no proper subset")
	}
	if !sub.IsSubsetOf(a) || !sub.IsProperSubsetOf(a) {
		t.Errorf("sub must be a (proper) subset of a")
	}
	if a.IsSubsetOf(sub) {
		t.Errorf("longer set cannot be a subset")
	}
	if !empty.IsSubsetOf(a) || !nilSet.IsSubsetOf(a) {
		t.Errorf("empty is a subset of everything")
	}
	if !a.IsSupersetOf(sub) || !a.IsProperSupersetOf(sub) {
		t.Errorf("a must be a (proper) superset of sub")
	}
	if !a.IsSupersetOf(a) || a.IsProperSupersetOf(a) {
		t.Errorf("superset of itself, but not a proper one")
	}
	if nilSet.IsSupersetOf(sub) {
		t.Errorf("nil cannot be a superset of a non-empty set")
	}
	if !nilSet.IsSupersetOf(nilSet) {
		t.Errorf("nil is a superset of nil (both empty)")
	}
	if !a.IsDisjointWith(other) || !other.IsDisjointWith(a) {
		t.Errorf("no shared element: disjoint")
	}
	if a.IsDisjointWith(sub) {
		t.Errorf("a and sub share elements: not disjoint")
	}
	if !empty.IsDisjointWith(empty) || !nilSet.IsDisjointWith(empty) {
		t.Errorf("two empty sets ARE disjoint")
	}

	// The subset laws the property battery pins: s ⊆ s∪t, s∩t ⊆ s.
	u, i := NewHashSet[int](), NewHashSet[int]()
	u.Union(a, other)
	i.Intersect(a, other)
	if !a.IsSubsetOf(u) || !other.IsSubsetOf(u) {
		t.Errorf("s ⊆ s∪t")
	}
	if !i.IsSubsetOf(a) {
		t.Errorf("s∩t ⊆ s")
	}
}

func TestHashSetJSON(t *testing.T) {
	a := FromSlice([]int{3, 1, 2})
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out []int
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode marshaled form: %v", err)
	}
	slices.Sort(out)
	if !slices.Equal(out, []int{1, 2, 3}) {
		t.Errorf("marshaled array = %v, want the elements", out)
	}

	// Round trip into a set built with custom functions: the functions
	// are KEPT, so the restored set still identifies case-insensitively.
	f := FromSliceFunc(ciEq, ciHash, []ciNode{{"abc", 1}, {"xyz", 2}})
	fData, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal func set: %v", err)
	}
	g := NewHashSetFunc(ciEq, ciHash)
	if err := g.UnmarshalJSON(fData); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if g.Len() != 2 {
		t.Fatalf("Len = %d, want 2", g.Len())
	}
	if g.Insert(ciNode{"ABC", 9}) {
		t.Errorf("unmarshal must keep the equality function (case-insensitive)")
	}
	if g.Len() != 2 {
		t.Errorf("Len = %d, want 2 — case-insensitive match must not add", g.Len())
	}
	// The false-returning insert still REPLACED the stored value (the
	// documented Insert contract), and "aBc" still finds it — both only
	// work because the unmarshaled set kept its eq/hash pair.
	got, found := g.Search(ciNode{"aBc", 0})
	if !found || got.N != 9 {
		t.Errorf("Search = (%+v, %v), want N=9", got, found)
	}

	// Unmarshal REPLACES contents.
	h := FromSlice([]int{7, 8})
	if err := h.UnmarshalJSON(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	sameContent(t, h, map[int]bool{1: true, 2: true, 3: true})

	// Malformed input leaves the set untouched.
	if err := h.UnmarshalJSON([]byte(`{"not":"an array"}`)); err == nil {
		t.Errorf("non-array JSON must error")
	}
	sameContent(t, h, map[int]bool{1: true, 2: true, 3: true})

	// nil and empty decode fine and empty the set.
	if err := h.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("null: %v", err)
	}
	if err := h.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("[]: %v", err)
	}
	if h.Len() != 0 {
		t.Errorf("empty array must empty the set")
	}

	var nilSet *HashSet[int]
	// The stdlib renders a nil pointer as "null" before MarshalJSON runs;
	// the method called directly on a nil set produces [].
	nilData, err := json.Marshal(nilSet)
	if err != nil || string(nilData) != "null" {
		t.Errorf("json.Marshal(nil pointer) = %s, %v — want null (stdlib rule)", nilData, err)
	}
	nilDirect, err := nilSet.MarshalJSON()
	if err != nil || string(nilDirect) != "[]" {
		t.Errorf("nilSet.MarshalJSON() = %s, %v — want []", nilDirect, err)
	}
	if err := nilSet.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("nil + []: %v", err)
	}
}

// randomizedModel drives the core and batch surface against a
// map[T]bool reference model for one fixed seed.
func randomizedModel(t *testing.T, seed int64) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	ss := NewHashSet[string]()
	model := make(map[string]bool)
	universe := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		universe = append(universe, fmt.Sprintf("k%03d", i))
	}

	for step := 0; step < 800; step++ {
		op := rng.Intn(10)
		k := universe[rng.Intn(len(universe))]
		switch {
		case op < 4: // insert
			if got, want := ss.Insert(k), !model[k]; got != want {
				t.Fatalf("seed %d step %d: Insert(%q) = %v, want %v", seed, step, k, got, want)
			}
			model[k] = true
		case op < 6: // delete
			if got, want := ss.Delete(k), model[k]; got != want {
				t.Fatalf("seed %d step %d: Delete(%q) = %v, want %v", seed, step, k, got, want)
			}
			delete(model, k)
		case op < 7: // search
			got, found := ss.Search(k)
			if found != model[k] || (found && got != k) {
				t.Fatalf("seed %d step %d: Search(%q) = (%q, %v), model %v", seed, step, k, got, found, model[k])
			}
		case op < 8: // contains
			if ss.Contains(k) != model[k] {
				t.Fatalf("seed %d step %d: Contains(%q) mismatch", seed, step, k)
			}
		case op < 9: // batch insert
			items := []string{k, universe[rng.Intn(len(universe))]}
			changed := false
			for _, it := range items {
				if !model[it] {
					changed = true
				}
				model[it] = true
			}
			if ss.InsertSlice(items) != changed {
				t.Fatalf("seed %d step %d: InsertSlice mismatch", seed, step)
			}
		default: // batch delete
			items := []string{k, universe[rng.Intn(len(universe))]}
			changed := false
			for _, it := range items {
				if model[it] {
					changed = true
				}
				delete(model, it)
			}
			if ss.DeleteSlice(items) != changed {
				t.Fatalf("seed %d step %d: DeleteSlice mismatch", seed, step)
			}
		}
		if ss.Len() != len(model) {
			t.Fatalf("seed %d step %d: Len = %d, model %d", seed, step, ss.Len(), len(model))
		}
	}
	sameContent(t, ss, model)
}

// TestHashSetRandomizedModel runs the model test under BOTH fixed seeds
// (the binary_tree fixed-seed-42 convention, plus 7 — two seeds, two
// different operation interleavings).
func TestHashSetRandomizedModel(t *testing.T) {
	randomizedModel(t, 42)
	randomizedModel(t, 7)
}

func TestHashSetNilAndZeroValuePanics(t *testing.T) {
	var nilSet *HashSet[int]
	var zero HashSet[int]

	// Constructor panics, message granularity per function.
	expectPanic(t, "hash_set: NewHashSetFunc called with a nil equality function",
		func() { NewHashSetFunc[int](nil, func(a int) uint64 { return 0 }) })
	expectPanic(t, "hash_set: NewHashSetFunc called with a nil hash function",
		func() { NewHashSetFunc[int](func(a, b int) bool { return false }, nil) })
	expectPanic(t, "hash_set: FromSliceFunc called with a nil equality function",
		func() { FromSliceFunc[int](nil, func(a int) uint64 { return 0 }, []int{1}) })
	expectPanic(t, "hash_set: FromSliceFunc called with a nil hash function",
		func() { FromSliceFunc[int](func(a, b int) bool { return false }, nil, []int{1}) })

	// Insert contract, nil set.
	expectPanic(t, "hash_set: Insert called on a nil set",
		func() { nilSet.Insert(1) })
	expectPanic(t, "hash_set: InsertSlice called on a nil set",
		func() { nilSet.InsertSlice([]int{1}) })
	expectPanic(t, "hash_set: InsertIter called on a nil set",
		func() { nilSet.InsertIter(slices.Values([]int{1})) })

	// Insert contract, zero-value set.
	expectPanic(t, "hash_set: Insert called on a set with no equality/hash functions",
		func() { zero.Insert(1) })
	expectPanic(t, "hash_set: InsertSlice called on a set with no equality/hash functions",
		func() { zero.InsertSlice([]int{1}) })

	// Empty storing batches never fire the contract.
	if zero.InsertSlice(nil) || nilSet.InsertSlice(nil) {
		t.Errorf("empty InsertSlice must report false")
	}
	if zero.InsertIter(nil) || nilSet.InsertIter(nil) {
		t.Errorf("nil InsertIter must report false")
	}

	// Every read is a total function on nil and zero value.
	for _, ss := range []*HashSet[int]{nilSet, &zero} {
		if ss.Len() != 0 || !ss.IsEmpty() {
			t.Errorf("nil/zero Len/IsEmpty wrong")
		}
		if _, found := ss.Search(1); found {
			t.Errorf("nil/zero Search found")
		}
		if ss.Contains(1) || ss.Delete(1) || ss.DeleteSlice([]int{1}) {
			t.Errorf("nil/zero membership/delete wrong")
		}
		if ss.DeleteFunc(func(int) bool { return true }) {
			t.Errorf("nil/zero DeleteFunc must report false")
		}
		if ss.ContainsSlice([]int{1}) || !ss.ContainsSlice(nil) {
			t.Errorf("nil/zero ContainsSlice wrong: [1] absent (false), nil vacuously true")
		}
		if ss.ContainsAny([]int{1}) {
			t.Errorf("nil/zero ContainsAny must be false")
		}
		if ss.Slice() != nil && len(ss.Slice()) != 0 {
			t.Errorf("nil/zero Slice wrong")
		}
		if got := ss.String(); got != "[]" {
			t.Errorf("nil/zero String = %q", got)
		}
		if ss.Clone() != nil && ss.Len() != 0 {
			t.Errorf("nil/zero Clone wrong")
		}
		n := 0
		for range ss.All() {
			n++
		}
		if n != 0 {
			t.Errorf("nil/zero All visited %d", n)
		}
		if !ss.Walk(func(_ int, _ int) bool { return true }) {
			t.Errorf("nil/zero Walk must report complete")
		}
		if !ss.Equal(NewHashSet[int]()) {
			t.Errorf("nil/zero must equal empty")
		}
		ss.Truncate() // no-op, must not panic
	}

	// JSON insert contract: storing elements into nil/zero panics;
	// empty arrays are tolerated everywhere.
	expectPanic(t, "hash_set: UnmarshalJSON called on a nil set",
		func() { _ = nilSet.UnmarshalJSON([]byte(`[1]`)) })
	expectPanic(t, "hash_set: UnmarshalJSON called on a set with no equality/hash functions",
		func() { _ = zero.UnmarshalJSON([]byte(`[1]`)) })
	if err := zero.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Errorf("zero + []: %v", err)
	}
	if err := zero.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("zero + null: %v", err)
	}
	if zero.Len() != 0 {
		t.Errorf("zero-value set changed by empty unmarshal")
	}
}

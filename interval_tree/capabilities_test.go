package interval_tree

// Tests for the set-convenience capabilities (capabilities.go) in their
// map-shaped, reduced form: materialization, deterministic rendering,
// deep copy, exact-match membership batches and predicate bulk delete —
// and the deliberate absence of batch inserters.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestCapabilitiesContains(t *testing.T) {
	tree := NewIntervalTree[int, string]()
	tree.Insert(iv(1, 5), "a")
	if !tree.Contains(iv(1, 5)) {
		t.Errorf("Contains should find a stored interval")
	}
	// Exact-match, NOT overlap: overlapping-but-different intervals are
	// absent (that query family is AnyOverlap/Overlaps).
	if tree.Contains(iv(2, 3)) || tree.Contains(iv(1, 6)) || tree.Contains(iv(0, 5)) {
		t.Errorf("Contains is exact-match; overlapping variants must not be found")
	}
	var nilTree *IntervalTree[int, string]
	if nilTree.Contains(iv(1, 5)) {
		t.Errorf("Contains on a nil tree should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	tree := NewIntervalTree[int, int]()
	tree.Insert(iv(20, 25), 2)
	tree.Insert(iv(10, 12), 1)
	tree.Insert(iv(10, 11), 0) // same Lo: Hi breaks the tie, still 3 entries
	tree.Insert(iv(20, 25), 9) // replace: still 3 entries

	got := tree.Slice()
	if len(got) != 3 {
		t.Fatalf("Slice length = %d, want 3", len(got))
	}
	// Ascending (Lo, Hi) without any sorting.
	want := []Interval[int]{iv(10, 11), iv(10, 12), iv(20, 25)}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Slice[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// The slice is a copy: mutating it must not affect the tree.
	got[0] = iv(900, 901)
	if tree.Contains(iv(900, 901)) {
		t.Errorf("mutating the Slice result changed the tree")
	}

	if empty := NewIntervalTree[int, int]().Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty tree should be empty, got %d", len(empty))
	}
	var nilTree *IntervalTree[int, int]
	if s := nilTree.Slice(); s != nil {
		t.Errorf("Slice of a nil tree should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed trees holding the same intervals must
	// render identically — the in-order walk is shape-independent.
	a := NewIntervalTree[int, int]()
	a.Insert(iv(1, 5), 1)
	a.Insert(iv(2, 7), 2)
	b := NewIntervalTree[int, int]()
	b.Insert(iv(2, 7), 2)
	b.Insert(iv(1, 5), 1)
	if a.String() != b.String() {
		t.Errorf("same intervals must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[1..5 2..7]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewIntervalTree[int, int]().String(); got != "[]" {
		t.Errorf("String of an empty tree = %q, want []", got)
	}
	var nilTree *IntervalTree[int, int]
	if got := nilTree.String(); got != "[]" {
		t.Errorf("String of a nil tree = %q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	tree := NewIntervalTree[int, int]()
	tree.Insert(iv(1, 5), 1)
	tree.Insert(iv(2, 7), 2)
	got := tree.StringFunc(func(iv Interval[int]) string {
		return fmt.Sprintf("[%d,%d]", iv.Hi, iv.Lo)
	})
	if want := "[[5,1] [7,2]]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := tree.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesClone(t *testing.T) {
	tree := NewIntervalTree[int, string]()
	for i := 50; i > 0; i -= 7 {
		tree.Insert(iv(i, i+3), fmt.Sprintf("v%d", i))
	}
	checkInvariants(t, tree)

	dup := tree.Clone()
	if !dup.Equal(tree) || !tree.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	checkInvariants(t, dup) // colors, maxHi, parents all copied intact
	if dup.Len() != tree.Len() {
		t.Errorf("clone len=%d, want %d", dup.Len(), tree.Len())
	}
	// Values are carried too.
	if v, found := dup.Search(iv(50, 53)); !found || v != "v50" {
		t.Errorf("clone should carry the value, got %q found=%v", v, found)
	}

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(iv(3, 4), "new")
	dup.Delete(iv(50, 53))
	if tree.Len() != 8 || !tree.Contains(iv(50, 53)) || tree.Contains(iv(3, 4)) {
		t.Errorf("mutating the clone changed the receiver: len=%d", tree.Len())
	}

	// And the other direction.
	tree.Insert(iv(2, 2), "other")
	if dup.Contains(iv(2, 2)) {
		t.Errorf("mutating the receiver changed the clone")
	}

	var nilTree *IntervalTree[int, string]
	if c := nilTree.Clone(); c != nil {
		t.Errorf("Clone of a nil tree should be nil, got %v", c)
	}
	var zero IntervalTree[int, string]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value tree should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same intervals inserted in different orders: equal.
	a := NewIntervalTree[int, int]()
	for _, e := range []entryInt{{iv(1, 2), 1}, {iv(3, 4), 2}, {iv(5, 6), 3}} {
		a.Insert(e.iv, e.val)
	}
	b := NewIntervalTree[int, int]()
	for _, e := range []entryInt{{iv(5, 6), 3}, {iv(3, 4), 2}, {iv(1, 2), 1}} {
		b.Insert(e.iv, e.val)
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("trees with the same intervals must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// KEYS ONLY: identical intervals with different values are still
	// Equal — comparing V would need boxing (documented).
	c := NewIntervalTree[int, int]()
	for _, e := range []entryInt{{iv(1, 2), 100}, {iv(3, 4), 200}, {iv(5, 6), 300}} {
		c.Insert(e.iv, e.val)
	}
	if !a.Equal(c) || !c.Equal(a) {
		t.Errorf("Equal compares interval keys only; different values must still be Equal")
	}

	// Different lengths.
	d := NewIntervalTree[int, int]()
	d.Insert(iv(1, 2), 1)
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different intervals.
	e := NewIntervalTree[int, int]()
	for _, x := range []entryInt{{iv(1, 2), 1}, {iv(3, 4), 2}, {iv(99, 99), 3}} {
		e.Insert(x.iv, x.val)
	}
	if a.Equal(e) || e.Equal(a) {
		t.Errorf("same length but different intervals must not be Equal")
	}

	// Empty trees, nil trees.
	empty := NewIntervalTree[int, int]()
	if !empty.Equal(NewIntervalTree[int, int]()) {
		t.Errorf("two empty trees must be Equal")
	}
	var nilTree *IntervalTree[int, int]
	if !empty.Equal(nilTree) || !nilTree.Equal(empty) {
		t.Errorf("an empty tree and a nil tree must be Equal")
	}
	if !nilTree.Equal(nilTree) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	tree := NewIntervalTree[int, int]()
	tree.Insert(iv(1, 2), 1)
	tree.Insert(iv(3, 4), 2)
	tree.Insert(iv(5, 6), 3)
	if !tree.ContainsSlice([]Interval[int]{iv(1, 2), iv(3, 4), iv(5, 6)}) {
		t.Errorf("ContainsSlice should be true when all present")
	}
	if !tree.ContainsSlice([]Interval[int]{iv(5, 6), iv(1, 2)}) {
		t.Errorf("ContainsSlice should be true for a subset")
	}
	if tree.ContainsSlice([]Interval[int]{iv(1, 2), iv(9, 9)}) {
		t.Errorf("ContainsSlice should be false when one is missing")
	}
	if !tree.ContainsSlice([]Interval[int]{}) {
		t.Errorf("ContainsSlice of an empty slice is vacuously true")
	}
	if !tree.ContainsAny([]Interval[int]{iv(9, 9), iv(3, 4)}) {
		t.Errorf("ContainsAny should be true when one is present")
	}
	if tree.ContainsAny([]Interval[int]{iv(9, 9), iv(8, 8)}) {
		t.Errorf("ContainsAny should be false when none present")
	}
	if tree.ContainsAny([]Interval[int]{}) {
		t.Errorf("ContainsAny of an empty slice should be false")
	}
	var nilTree *IntervalTree[int, int]
	if nilTree.ContainsSlice([]Interval[int]{iv(1, 1)}) || !nilTree.ContainsSlice([]Interval[int]{}) {
		t.Errorf("ContainsSlice on a nil tree: non-empty false, empty true")
	}
	if nilTree.ContainsAny([]Interval[int]{iv(1, 1)}) {
		t.Errorf("ContainsAny on a nil tree should be false")
	}
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	tree := NewIntervalTree[int, int]()
	for _, x := range []entryInt{{iv(1, 2), 1}, {iv(3, 4), 2}, {iv(5, 6), 3}, {iv(7, 8), 4}, {iv(9, 10), 5}} {
		tree.Insert(x.iv, x.val)
	}
	if changed := tree.DeleteSlice([]Interval[int]{iv(3, 4), iv(99, 99), iv(7, 8)}); !changed || tree.Len() != 3 {
		t.Errorf("DeleteSlice: changed=%v len=%d, want true 3", changed, tree.Len())
	}
	if changed := tree.DeleteSlice([]Interval[int]{iv(99, 99), iv(98, 98)}); changed {
		t.Errorf("DeleteSlice of nothing present should report false")
	}
	if changed := tree.DeleteSlice(nil); changed {
		t.Errorf("DeleteSlice(nil) should report false")
	}
	checkInvariants(t, tree)

	var nilTree *IntervalTree[int, int]
	if nilTree.DeleteSlice([]Interval[int]{iv(1, 1)}) {
		t.Errorf("DeleteSlice on a nil tree should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose red-black or maxHi damage: fill, delete roughly half
// by a predicate over BOTH the interval and the value, then re-verify
// every membership and the structural invariants.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	tree := NewIntervalTree[int, int]()
	model := map[Interval[int]]int{}
	for i := 0; i < 1000; i++ {
		ivl := iv(i, i+7)
		tree.Insert(ivl, i%10)
		model[ivl] = i % 10
	}
	if !tree.DeleteFunc(func(ivl Interval[int], val int) bool {
		return ivl.Lo%3 == 0 || val == 7
	}) {
		t.Fatalf("DeleteFunc should report true")
	}
	for ivl, val := range model {
		want := ivl.Lo%3 != 0 && val != 7
		if tree.Contains(ivl) != want {
			t.Fatalf("after mass DeleteFunc, Contains(%v)=%v, want %v", ivl, tree.Contains(ivl), want)
		}
	}
	if tree.Len() != 599 { // 1000 - |Lo div by 3 or val 7| = 1000 - (334+100-33)
		t.Errorf("after mass DeleteFunc len=%d, want 599", tree.Len())
	}
	checkInvariants(t, tree)

	// A predicate with access only to the value sees the values.
	if !tree.DeleteFunc(func(Interval[int], int) bool { return true }) {
		t.Errorf("delete-everything predicate should report true")
	}
	if tree.Len() != 0 {
		t.Errorf("tree should be empty, len=%d", tree.Len())
	}
	checkInvariants(t, tree)

	if changed := tree.DeleteFunc(func(Interval[int], int) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := tree.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTree *IntervalTree[int, string]
	var zero IntervalTree[int, string]
	for name, tr := range map[string]*IntervalTree[int, string]{"nil tree": nilTree, "zero value": &zero} {
		if tr.Contains(iv(1, 2)) {
			t.Errorf("%s: Contains should be false", name)
		}
		if tr.String() != "[]" {
			t.Errorf("%s: String should be []", name)
		}
		if tr.Len() != 0 || !tr.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if changed := tr.DeleteSlice([]Interval[int]{iv(1, 2)}); changed {
			t.Errorf("%s: DeleteSlice should report false", name)
		}
		if changed := tr.DeleteFunc(func(Interval[int], string) bool { return true }); changed {
			t.Errorf("%s: DeleteFunc should report false", name)
		}
		if !tr.Equal(nilTree) {
			t.Errorf("%s: should Equal a nil tree", name)
		}
		if s := tr.Slice(); s != nil {
			t.Errorf("%s: Slice should be nil", name)
		}
	}
}

// TestCapabilitiesRandomizedModel cross-checks the batch and predicate
// operations against a map reference model with a fixed seed.
func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	tree := NewIntervalTree[int, int]()
	model := map[Interval[int]]int{}

	ivKey := func() Interval[int] { return iv(rng.IntN(200), rng.IntN(200)+200) }

	sortModel := func() []Interval[int] {
		keys := make([]Interval[int], 0, len(model))
		for k := range model {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b Interval[int]) int {
			if a.Lo != b.Lo {
				return a.Lo - b.Lo
			}
			return a.Hi - b.Hi
		})
		return keys
	}

	for step := 0; step < 800; step++ {
		switch rng.IntN(5) {
		case 0, 1:
			k := ivKey()
			model[k] = step
			tree.Insert(k, step)
		case 2:
			k := ivKey()
			delete(model, k)
			tree.Delete(k)
		case 3:
			// Predicate delete on a low-endpoint digit class.
			d := rng.IntN(10)
			for k := range model {
				if k.Lo%10 == d {
					delete(model, k)
				}
			}
			tree.DeleteFunc(func(ivl Interval[int], _ int) bool { return ivl.Lo%10 == d })
		case 4:
			// Batch delete of 1..3 keys.
			items := make([]Interval[int], rng.IntN(4))
			for i := range items {
				items[i] = ivKey()
			}
			for _, k := range items {
				delete(model, k)
			}
			tree.DeleteSlice(items)
		}

		if tree.Len() != len(model) {
			t.Fatalf("step %d: len=%d, model=%d", step, tree.Len(), len(model))
		}

		if step%100 == 99 {
			checkInvariants(t, tree)
			for k := range model {
				if !tree.Contains(k) {
					t.Fatalf("step %d: Contains(%v) should be true", step, k)
				}
			}
			if !slices.Equal(tree.Slice(), sortModel()) {
				t.Fatalf("step %d: Slice is not the sorted model (it must be ascending)", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh tree built from the model.
	want := sortModel()
	fresh := NewIntervalTree[int, int]()
	for i, k := range want {
		fresh.Insert(k, i)
	}
	if !tree.Equal(fresh) || !fresh.Equal(tree) {
		t.Errorf("final tree must Equal a fresh tree built from the model")
	}
	if tree.String() != fresh.String() {
		t.Errorf("same intervals must render the same String: %q vs %q", tree.String(), fresh.String())
	}
	if !tree.ContainsSlice(want) {
		t.Errorf("final tree must contain every model interval")
	}
	if tree.ContainsSlice(append(want, iv(99999, 99999))) {
		t.Errorf("ContainsSlice with a missing interval must be false")
	}
	if !tree.ContainsAny(append(want, iv(99999, 99999))) {
		t.Errorf("ContainsAny with one present interval must be true")
	}
}

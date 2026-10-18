package avl_tree_ts

// Tests for the set-convenience capabilities (capabilities.go) and the
// phase-2 avl riders (SymmetricDifference, Floor, Ceil): materialization,
// deterministic rendering, deep copy, membership batches, predicate bulk
// delete, the nil/zero-value read contract, and the set algebra.  The
// same file (package name aside) runs against the plain avl_tree — the
// twins must behave identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

func TestCapabilitiesContains(t *testing.T) {
	tree := NewAvlTree[string]()
	tree.Insert("alpha")
	if !tree.Contains("alpha") {
		t.Errorf("Contains should find a stored element")
	}
	if tree.Contains("beta") {
		t.Errorf("Contains should not find a missing element")
	}
	var nilTree *AvlTree[string]
	if nilTree.Contains("alpha") {
		t.Errorf("Contains on a nil tree should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	tree := newTestTree()
	tree.Insert(mkKey(20))
	tree.Insert(mkKey(10))
	tree.Insert(mkKey(30))
	tree.Insert(TestTreeNode{S: "00000010", N: 7}) // replace: still 3 elements

	got := tree.Slice()
	if len(got) != 3 {
		t.Fatalf("Slice length = %d, want 3", len(got))
	}
	// Slice is in-order: ascending without any sorting.
	if got[0].S != "00000010" || got[1].S != "00000020" || got[2].S != "00000030" {
		t.Errorf("Slice order = [%s %s %s], want ascending", got[0].S, got[1].S, got[2].S)
	}
	// Replacement keeps the last value.
	if got[0].N != 7 {
		t.Errorf("Slice should carry the replaced value, got N=%d, want 7", got[0].N)
	}

	// The slice is a copy: mutating it must not affect the tree.
	got[0] = TestTreeNode{S: "zzzzzzzz"}
	if _, found := tree.Search(TestTreeNode{S: "zzzzzzzz"}); found {
		t.Errorf("mutating the Slice result changed the tree")
	}

	if empty := newTestTree().Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty tree should be empty, got %d", len(empty))
	}
	var nilTree *AvlTree[TestTreeNode]
	if s := nilTree.Slice(); s != nil {
		t.Errorf("Slice of a nil tree should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed trees holding the same elements (inserted
	// in different orders, so different shapes) must render identically.
	a := NewAvlTree[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		a.Insert(s)
	}
	b := NewAvlTree[string]()
	for _, s := range []string{"fig", "pear", "apple"} {
		b.Insert(s)
	}
	if a.String() != b.String() {
		t.Errorf("same elements must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[apple fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewAvlTree[string]().String(); got != "[]" {
		t.Errorf("String of an empty tree = %q, want []", got)
	}
	var nilTree *AvlTree[string]
	if got := nilTree.String(); got != "[]" {
		t.Errorf("String of a nil tree = %q, want []", got)
	}

	numbers := NewAvlTree[int]()
	numbers.Insert(9)
	numbers.Insert(2)
	if got, want := numbers.String(), "[2 9]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	tree := NewAvlTree[int]()
	tree.Insert(5)
	tree.Insert(8)
	got := tree.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) })
	if want := "[#5 #8]"; got != want {
		t.Errorf("StringFunc = %q, want %q", got, want)
	}
	if got := tree.StringFunc(nil); got != "[]" {
		t.Errorf("StringFunc(nil) = %q, want []", got)
	}
}

func TestCapabilitiesClone(t *testing.T) {
	tree := newTestTree()
	for i := 50; i > 0; i -= 7 {
		tree.Insert(mkKey(i))
	}

	dup := tree.Clone()
	if !dup.Equal(tree) || !tree.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	validateAVLNode(t, dup.root) // heights carried over correctly

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(mkKey(3))
	dup.Delete(mkKey(50))
	if tree.Len() != 8 || !tree.Contains(mkKey(50)) || tree.Contains(mkKey(3)) {
		t.Errorf("mutating the clone changed the receiver: len=%d", tree.Len())
	}

	// And the other direction.
	tree.Insert(mkKey(2))
	if dup.Contains(mkKey(2)) {
		t.Errorf("mutating the receiver changed the clone")
	}

	// The comparison function carried over: the clone still matches by S
	// only, and a duplicate insert replaces the satellite data.
	dup.Insert(TestTreeNode{S: "00000043", N: 99}) // 43 was inserted (50-7)
	if v, found := dup.Search(TestTreeNode{S: "00000043"}); !found || v.N != 99 {
		t.Errorf("clone should keep the comparison function (replace by S), got %v %v", v, found)
	}

	var nilTree *AvlTree[TestTreeNode]
	if c := nilTree.Clone(); c != nil {
		t.Errorf("Clone of a nil tree should be nil, got %v", c)
	}
	var zero AvlTree[TestTreeNode]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value tree should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same elements inserted in different orders (different shapes): equal.
	a := newTestTree()
	for _, k := range []int{1, 2, 3, 4} {
		a.Insert(mkKey(k))
	}
	b := newTestTree()
	for _, k := range []int{4, 3, 2, 1} {
		b.Insert(mkKey(k))
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("trees with the same elements must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths.
	c := newTestTree()
	c.Insert(mkKey(1))
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different elements.
	d := newTestTree()
	for _, k := range []int{1, 2, 3, 99} {
		d.Insert(mkKey(k))
	}
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("same length but different elements must not be Equal")
	}

	// Empty trees, nil trees.
	empty := newTestTree()
	if !empty.Equal(newTestTree()) {
		t.Errorf("two empty trees must be Equal")
	}
	var nilTree *AvlTree[TestTreeNode]
	if !empty.Equal(nilTree) || !nilTree.Equal(empty) {
		t.Errorf("an empty tree and a nil tree must be Equal")
	}
	if !nilTree.Equal(nilTree) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	tree := NewAvlTree[int]()
	for _, v := range []int{1, 2, 3} {
		tree.Insert(v)
	}
	if !tree.ContainsSlice([]int{1, 2, 3}) {
		t.Errorf("ContainsSlice should be true when all present")
	}
	if !tree.ContainsSlice([]int{3, 1}) {
		t.Errorf("ContainsSlice should be true for a subset")
	}
	if tree.ContainsSlice([]int{1, 9}) {
		t.Errorf("ContainsSlice should be false when one is missing")
	}
	if !tree.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice of an empty slice is vacuously true")
	}
	if !tree.ContainsAny([]int{9, 2}) {
		t.Errorf("ContainsAny should be true when one is present")
	}
	if tree.ContainsAny([]int{9, 8}) {
		t.Errorf("ContainsAny should be false when none present")
	}
	if tree.ContainsAny([]int{}) {
		t.Errorf("ContainsAny of an empty slice should be false")
	}
	var nilTree *AvlTree[int]
	if nilTree.ContainsSlice([]int{1}) || !nilTree.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice on a nil tree: non-empty false, empty true")
	}
	if nilTree.ContainsAny([]int{1}) {
		t.Errorf("ContainsAny on a nil tree should be false")
	}
}

func TestCapabilitiesInsertBatches(t *testing.T) {
	tree := NewAvlTree[int]()
	if changed := tree.InsertSlice([]int{1, 2, 3}); !changed {
		t.Errorf("InsertSlice of new elements should report true")
	}
	if changed := tree.InsertSlice([]int{2, 3}); changed {
		t.Errorf("InsertSlice of only existing elements should report false")
	}
	if changed := tree.InsertSlice([]int{4, 4}); !changed || tree.Len() != 4 {
		t.Errorf("InsertSlice with duplicates: changed=%v len=%d, want true 4", changed, tree.Len())
	}
	if changed := tree.InsertSlice(nil); changed {
		t.Errorf("InsertSlice of nothing should report false")
	}

	// InsertIter fed from another tree's All — the interface-free batch
	// bridge.
	other := NewAvlTree[int]()
	other.Insert(10)
	other.Insert(20)
	if changed := tree.InsertIter(other.All()); !changed || !tree.Contains(20) {
		t.Errorf("InsertIter from another tree: changed=%v", changed)
	}
	if changed := tree.InsertIter(other.All()); changed {
		t.Errorf("InsertIter of only existing elements should report false")
	}
	if changed := tree.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) should report false")
	}

	// Self-feeding is safe: the iterator is drained before inserting.
	self := NewAvlTree[int]()
	self.InsertSlice([]int{1, 2, 3})
	if changed := self.InsertIter(self.All()); changed || self.Len() != 3 {
		t.Errorf("self-feed InsertIter: changed=%v len=%d, want false 3", changed, self.Len())
	}

	// Ascending insert into an AVL tree stays balanced.
	tree2 := NewAvlTree[int]()
	tree2.InsertSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	validateIntAVL(t, tree2)
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	tree := NewAvlTree[int]()
	tree.InsertSlice([]int{1, 2, 3, 4, 5})
	if changed := tree.DeleteSlice([]int{2, 9, 4}); !changed || tree.Len() != 3 {
		t.Errorf("DeleteSlice: changed=%v len=%d, want true 3", changed, tree.Len())
	}
	if changed := tree.DeleteSlice([]int{9, 8}); changed {
		t.Errorf("DeleteSlice of nothing present should report false")
	}
	if changed := tree.DeleteSlice(nil); changed {
		t.Errorf("DeleteSlice(nil) should report false")
	}

	// DeleteFunc removes exactly the matches and leaves the rest.
	tree2 := NewAvlTree[int]()
	for i := 0; i < 200; i++ {
		tree2.Insert(i)
	}
	if changed := tree2.DeleteFunc(func(v int) bool { return v%2 == 0 }); !changed {
		t.Errorf("DeleteFunc that matches half should report true")
	}
	if tree2.Len() != 100 {
		t.Fatalf("after DeleteFunc len=%d, want 100", tree2.Len())
	}
	for i := 0; i < 200; i++ {
		_, found := tree2.Search(i)
		if i%2 == 0 && found {
			t.Errorf("even element %d should be deleted", i)
		}
		if i%2 == 1 && !found {
			t.Errorf("odd element %d should remain", i)
		}
	}
	validateIntAVL(t, tree2)

	if changed := tree2.DeleteFunc(func(int) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := tree2.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}

	var nilTree *AvlTree[int]
	if nilTree.DeleteSlice([]int{1}) || nilTree.DeleteFunc(func(int) bool { return true }) {
		t.Errorf("deletes on a nil tree should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose rebalance damage: fill, delete roughly half by
// predicate, then re-verify every membership and the AVL invariants.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	tree := NewAvlTree[int]()
	model := map[int]bool{}
	for i := 0; i < 1000; i++ {
		tree.Insert(i)
		model[i] = true
	}
	if !tree.DeleteFunc(func(v int) bool { return v%3 == 0 || v%5 == 0 }) {
		t.Fatalf("DeleteFunc should report true")
	}
	for i := range model {
		want := i%3 != 0 && i%5 != 0
		if tree.Contains(i) != want {
			t.Fatalf("after mass DeleteFunc, Contains(%d)=%v, want %v", i, tree.Contains(i), want)
		}
	}
	if tree.Len() != 533 { // 1000 - |div by 3 or 5| = 1000 - (333+200-66)
		t.Errorf("after mass DeleteFunc len=%d, want 533", tree.Len())
	}
	validateIntAVL(t, tree)
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTree *AvlTree[string]
	var zero AvlTree[string]
	for name, tr := range map[string]*AvlTree[string]{"nil tree": nilTree, "zero value": &zero} {
		if tr.Contains("x") {
			t.Errorf("%s: Contains should be false", name)
		}
		if tr.String() != "[]" {
			t.Errorf("%s: String should be []", name)
		}
		if tr.Len() != 0 || !tr.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if changed := tr.DeleteSlice([]string{"x"}); changed {
			t.Errorf("%s: DeleteSlice should report false", name)
		}
		if changed := tr.DeleteFunc(func(string) bool { return true }); changed {
			t.Errorf("%s: DeleteFunc should report false", name)
		}
		if !tr.Equal(nilTree) {
			t.Errorf("%s: should Equal a nil tree", name)
		}
		if s := tr.Slice(); name == "nil tree" && s != nil {
			t.Errorf("nil tree: Slice should be nil")
		} else if name == "zero value" && len(s) != 0 {
			t.Errorf("zero value: Slice should be empty")
		}
		if _, found := tr.Floor("x"); found {
			t.Errorf("%s: Floor should report not-found", name)
		}
		if _, found := tr.Ceil("x"); found {
			t.Errorf("%s: Ceil should report not-found", name)
		}
	}

	// The insert contract fires only when an element would be stored.
	if changed := nilTree.InsertSlice([]string{}); changed {
		t.Errorf("InsertSlice of nothing on a nil tree should report false")
	}
	if changed := nilTree.InsertIter(nil); changed {
		t.Errorf("InsertIter(nil) on a nil tree should report false")
	}
	expectPanicMessage(t, "InsertSlice on nil", "InsertSlice called on a nil tree", func() {
		nilTree.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on nil", "InsertIter called on a nil tree", func() {
		nilTree.InsertIter(func(yield func(string) bool) { yield("x") })
	})
	expectPanicMessage(t, "InsertSlice on zero value", "no comparison function", func() {
		zero.InsertSlice([]string{"x"})
	})
	expectPanicMessage(t, "InsertIter on zero value", "no comparison function", func() {
		zero.InsertIter(func(yield func(string) bool) { yield("x") })
	})
}

// TestCapabilitiesRandomizedModel cross-checks the batch and predicate
// operations against a map reference model with a fixed seed.
func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	tree := NewAvlTree[int]()
	model := map[int]bool{}

	key := func() int { return rng.Intn(200) }

	for step := 0; step < 800; step++ {
		switch rng.Intn(6) {
		case 0, 1:
			k := key()
			model[k] = true
			tree.Insert(k)
		case 2:
			k := key()
			delete(model, k)
			tree.Delete(k)
		case 3:
			// Batch insert of 1..3 keys.
			items := make([]int, rng.Intn(4))
			for i := range items {
				items[i] = key()
			}
			for _, k := range items {
				model[k] = true
			}
			tree.InsertSlice(items)
		case 4:
			// Predicate delete on a digit class.
			d := rng.Intn(10)
			for k := range model {
				if k%10 == d {
					delete(model, k)
				}
			}
			tree.DeleteFunc(func(v int) bool { return v%10 == d })
		case 5:
			// Batch delete of 1..3 keys.
			items := make([]int, rng.Intn(4))
			for i := range items {
				items[i] = key()
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
			for k, want := range model {
				if tree.Contains(k) != want {
					t.Fatalf("step %d: Contains(%d)=%v, want %v", step, k, tree.Contains(k), want)
				}
			}
			snap := tree.Slice()
			sortedModel := capabilitiesModelKeys(model)
			sort.Ints(sortedModel)
			if !g_lib.EqualSlice(snap, sortedModel) {
				t.Fatalf("step %d: Slice is not the sorted model (it must be ascending)", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh tree built from the model.
	want := capabilitiesModelKeys(model)
	sort.Ints(want)
	fresh := NewAvlTree[int]()
	fresh.InsertSlice(want)
	if !tree.Equal(fresh) || !fresh.Equal(tree) {
		t.Errorf("final tree must Equal a fresh tree built from the model")
	}
	if tree.String() != fresh.String() {
		t.Errorf("same elements must render the same String: %q vs %q", tree.String(), fresh.String())
	}
	if !tree.ContainsSlice(want) {
		t.Errorf("final tree must contain every model key")
	}
	missing := append(want, 1000000)
	if tree.ContainsSlice(missing) {
		t.Errorf("ContainsSlice with a missing key must be false")
	}
	if !tree.ContainsAny(missing) {
		t.Errorf("ContainsAny with one present key must be true")
	}
}

// ---------------------------------------------------------------------------
// avl riders: SymmetricDifference and Floor/Ceil.

func TestCapabilitiesSymmetricDifference(t *testing.T) {
	build := func(keys ...int) *AvlTree[TestTreeNode] {
		tr := newTestTree()
		for _, k := range keys {
			tr.Insert(mkKey(k))
		}
		return tr
	}
	keysOf := func(tr *AvlTree[TestTreeNode]) (rv []string) {
		for item := range tr.All() {
			rv = append(rv, item.S)
		}
		return
	}
	eq := func(what string, got, expect []string) {
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
			t.Errorf("%s: expected %v got %v", what, expect, got)
		}
	}

	t1 := build(1, 2, 3, 4)
	t2 := build(3, 4, 5, 6)

	// Fixed vector: elements in exactly one operand, ascending.
	dst := newTestTree()
	dst.SymmetricDifference(t1, t2)
	eq("SymmetricDifference", keysOf(dst), keysOf(build(1, 2, 5, 6)))
	validateAVLNode(t, dst.root)

	// With itself: empty (every element is in both).
	t1.SymmetricDifference(t1, t1)
	if !t1.IsEmpty() {
		t.Errorf("s △ s must be empty, got %v", keysOf(t1))
	}

	// With an empty operand: a copy.
	t3 := build(7, 8)
	t3.SymmetricDifference(t3, newTestTree())
	eq("△ with empty", keysOf(t3), keysOf(build(7, 8)))

	// Aliased destination must not corrupt the sources mid-operation.
	t4 := build(1, 2)
	t5 := build(2, 3)
	t4.SymmetricDifference(t4, t5)
	eq("alias dst=yy", keysOf(t4), keysOf(build(1, 3)))

	// Zero-value destination adopts an operand's comparison.
	var zero AvlTree[TestTreeNode]
	zero.SymmetricDifference(t5, nil)
	eq("adoption", keysOf(&zero), keysOf(build(2, 3)))
	if zero.cmp == nil {
		t.Errorf("zero-value destination should adopt an operand's comparison function")
	}

	// Nil destination is a no-op.
	var nilTree *AvlTree[TestTreeNode]
	nilTree.SymmetricDifference(t5, t5) // must not panic
}

// TestCapabilitiesSymmetricDifferenceModel is a fixed-seed property test
// of the algebra: the map-computed reference result, commutativity, the
// Minus(Union, Intersect) equivalence, and alias safety.
func TestCapabilitiesSymmetricDifferenceModel(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	gen := func() (*AvlTree[int], map[int]bool) {
		tr := NewAvlTree[int]()
		model := map[int]bool{}
		for i := 0; i < 200; i++ {
			k := rng.Intn(100)
			tr.Insert(k)
			model[k] = true
		}
		return tr, model
	}
	sortedKeys := func(m map[int]bool) []int {
		keys := capabilitiesModelKeys(m)
		sort.Ints(keys)
		return keys
	}

	for round := 0; round < 10; round++ {
		aa, aModel := gen()
		bb, bModel := gen()

		// Reference: elements in exactly one model.
		wantModel := map[int]bool{}
		for k := range aModel {
			if !bModel[k] {
				wantModel[k] = true
			}
		}
		for k := range bModel {
			if !aModel[k] {
				wantModel[k] = true
			}
		}

		got := NewAvlTree[int]()
		got.SymmetricDifference(aa, bb)
		if s := got.Slice(); !g_lib.EqualSlice(s, sortedKeys(wantModel)) {
			t.Fatalf("round %d: SymmetricDifference = %v, want %v", round, s, sortedKeys(wantModel))
		}
		validateIntAVL(t, got)

		// Commutativity.
		got2 := NewAvlTree[int]()
		got2.SymmetricDifference(bb, aa)
		if !got.Equal(got2) {
			t.Fatalf("round %d: SymmetricDifference must commute", round)
		}

		// Equivalence with (aa ∪ bb) − (aa ∩ bb).
		union, inter, minus := NewAvlTree[int](), NewAvlTree[int](), NewAvlTree[int]()
		union.Union(aa, bb)
		inter.Intersect(aa, bb)
		minus.Minus(union, inter)
		if !got.Equal(minus) {
			t.Fatalf("round %d: ▽ disagrees with (union - intersection)", round)
		}

		// Alias safety: destination aliasing an operand.
		alias := NewAvlTree[int]()
		alias.Union(aa, nil) // alias = copy of aa
		alias.SymmetricDifference(alias, bb)
		if !alias.Equal(got) {
			t.Fatalf("round %d: aliased destination must give the same result", round)
		}
	}
}

func TestCapabilitiesFloorCeil(t *testing.T) {
	tree := NewAvlTree[int]()
	for _, v := range []int{10, 20, 30, 40} {
		tree.Insert(v)
	}

	// Exact hits return the element itself.
	if v, found := tree.Floor(20); !found || v != 20 {
		t.Errorf("Floor(20) = %v %v, want 20 true", v, found)
	}
	if v, found := tree.Ceil(20); !found || v != 20 {
		t.Errorf("Ceil(20) = %v %v, want 20 true", v, found)
	}

	// Between elements: Floor rounds down, Ceil rounds up.
	if v, found := tree.Floor(25); !found || v != 20 {
		t.Errorf("Floor(25) = %v %v, want 20 true", v, found)
	}
	if v, found := tree.Ceil(25); !found || v != 30 {
		t.Errorf("Ceil(25) = %v %v, want 30 true", v, found)
	}

	// Off the ends.
	if _, found := tree.Floor(5); found {
		t.Errorf("Floor(5) below every element should be not-found")
	}
	if _, found := tree.Ceil(45); found {
		t.Errorf("Ceil(45) above every element should be not-found")
	}
	if v, found := tree.Floor(100); !found || v != 40 {
		t.Errorf("Floor(100) = %v %v, want 40 true", v, found)
	}
	if v, found := tree.Ceil(0); !found || v != 10 {
		t.Errorf("Ceil(0) = %v %v, want 10 true", v, found)
	}

	// Empty, nil and zero-value trees report not-found.
	if _, found := NewAvlTree[int]().Floor(1); found {
		t.Errorf("Floor on an empty tree should be not-found")
	}
	if _, found := NewAvlTree[int]().Ceil(1); found {
		t.Errorf("Ceil on an empty tree should be not-found")
	}
	var nilTree *AvlTree[int]
	if _, found := nilTree.Floor(1); found {
		t.Errorf("Floor on a nil tree should be not-found")
	}
	if _, found := nilTree.Ceil(1); found {
		t.Errorf("Ceil on a nil tree should be not-found")
	}
	var zero AvlTree[int]
	if _, found := zero.Floor(1); found {
		t.Errorf("Floor on a zero-value tree should be not-found")
	}
	if _, found := zero.Ceil(1); found {
		t.Errorf("Ceil on a zero-value tree should be not-found")
	}

	// Consistency with skip_list's documented semantics: Ceil(x) is the
	// first element >= x, Floor(x) the last element <= x — cross-checked
	// against a linear scan over Slice.
	full := NewAvlTree[int]()
	for i := 0; i < 100; i += 3 { // 0, 3, 6, ...
		full.Insert(i)
	}
	slice := full.Slice()
	for probe := -2; probe < 104; probe++ {
		wantFloor, floorOK := -1, false
		for _, v := range slice {
			if v <= probe {
				wantFloor, floorOK = v, true
			}
		}
		if v, found := full.Floor(probe); found != floorOK || (found && v != wantFloor) {
			t.Fatalf("Floor(%d) = %v %v, want %v %v", probe, v, found, wantFloor, floorOK)
		}

		wantCeil, ceilOK := -1, false
		for _, v := range slice {
			if v >= probe {
				wantCeil, ceilOK = v, true
				break
			}
		}
		if v, found := full.Ceil(probe); found != ceilOK || (found && v != wantCeil) {
			t.Fatalf("Ceil(%d) = %v %v, want %v %v", probe, v, found, wantCeil, ceilOK)
		}
	}
}

// validateIntAVL is validateAVLNode for int trees (test helper).
func validateIntAVL(t *testing.T, tt *AvlTree[int]) {
	t.Helper()
	var check func(n *AvlTreeElement[int]) int
	check = func(n *AvlTreeElement[int]) int {
		if n == nil {
			return 0
		}
		lh := check(n.left)
		rh := check(n.right)
		if n.left != nil && n.left.data >= n.data {
			t.Fatalf("BST order violated: left child %d >= node %d", n.left.data, n.data)
		}
		if n.right != nil && n.right.data <= n.data {
			t.Fatalf("BST order violated: right child %d <= node %d", n.right.data, n.data)
		}
		if d := lh - rh; d < -1 || d > 1 {
			t.Fatalf("AVL balance violated at node %d: left=%d right=%d", n.data, lh, rh)
		}
		if want := max(lh, rh) + 1; n.height != want {
			t.Fatalf("cached height wrong at node %d: got %d want %d", n.data, n.height, want)
		}
		return max(lh, rh) + 1
	}
	check(tt.root)
}

// capabilitiesModelKeys returns the keys of m in unspecified order (test
// helper).
func capabilitiesModelKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// twin-only: concurrency.

// TestCapabilitiesConcurrent runs the new methods from several goroutines
// at once — `go test -race` is the real assertion here (no data race and
// no deadlock), the final-state check just verifies the writes all
// landed.
func TestCapabilitiesConcurrent(t *testing.T) {
	tree := NewAvlTree[int]()
	tree.InsertSlice([]int{-1, -2}) // a little initial content

	const workers = 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				k := base + i*workers
				switch i % 9 {
				case 0, 1, 2:
					tree.Insert(k)
				case 3:
					tree.InsertSlice([]int{k, k})
				case 4:
					tree.InsertIter(func(yield func(int) bool) {
						yield(k)
						yield(k + 1)
					})
				case 5:
					tree.Delete(k)
					tree.DeleteSlice([]int{k + 1})
				case 6:
					_ = tree.Contains(k)
					_ = tree.ContainsSlice([]int{k, k + 1})
					_ = tree.ContainsAny([]int{k, k + 1})
				case 7:
					_ = tree.Len()
					_ = tree.IsEmpty()
					_ = tree.String()
					_ = tree.Slice()
					_ = tree.Clone().Len()
					_ = tree.DeleteFunc(func(v int) bool { return v == k+2 })
				case 8:
					_, _ = tree.Floor(k) // the phase-2 riders race too
					_, _ = tree.Ceil(k)
				}
			}
		}(w * 100000)
	}
	wg.Wait()

	if tree.Len() == 0 {
		t.Errorf("expected residual content after the concurrent mix")
	}
	for v := range tree.All() { // snapshot iterator: safe after the wait
		_ = v
	}
}

// TestCapabilitiesBatchAtomicity verifies that a batch insert holds the
// write lock for the whole batch: a concurrent reader observes either
// none or all of the batch, never a partial application.
func TestCapabilitiesBatchAtomicity(t *testing.T) {
	tree := NewAvlTree[int]()
	const batch = 5000
	items := make([]int, batch)
	for i := range items {
		items[i] = i
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tree.InsertSlice(items)
	}()

	observed := map[int]int{}
	for running := true; running; {
		n := tree.Len() // read lock; blocks while the batch holds the write lock
		observed[n]++
		select {
		case <-done:
			running = false
		default:
		}
	}
	if observed[batch] == 0 {
		t.Errorf("the final size %d was never observed (observations: %v)", batch, observed)
	}
	for n := range observed {
		if n != 0 && n != batch {
			t.Errorf("observed partial batch state len=%d (observations: %v)", n, observed)
		}
	}
}

// TestCapabilitiesDeleteFuncCallback verifies that the DeleteFunc
// predicate runs without the lock held: it may call back into the tree.
func TestCapabilitiesDeleteFuncCallback(t *testing.T) {
	tree := NewAvlTree[int]()
	for i := 0; i < 50; i++ {
		tree.Insert(i)
	}
	changed := tree.DeleteFunc(func(v int) bool {
		return v%2 == 0 && tree.Contains(v+1000) == false && tree.Len() >= 0 // call-backs under the predicate
	})
	if !changed || tree.Len() != 25 {
		t.Errorf("DeleteFunc with a call-back predicate: changed=%v len=%d, want true 25", changed, tree.Len())
	}
}

// TestCapabilitiesEqualNoDeadlock compares two trees from two goroutines
// in opposite directions at once — Equal must never hold both locks.
func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := NewAvlTree[int]()
	b := NewAvlTree[int]()
	for i := 0; i < 500; i++ {
		a.Insert(i)
		b.Insert(i)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
}

// TestCapabilitiesSymmetricDifferenceConcurrent rebuilds a destination
// while readers hammer it and the operands — the snapshots must make the
// whole rebuild safe.  `go test -race` is the assertion.
func TestCapabilitiesSymmetricDifferenceConcurrent(t *testing.T) {
	a := NewAvlTree[int]()
	b := NewAvlTree[int]()
	for i := 0; i < 300; i += 2 {
		a.Insert(i)
	}
	for i := 1; i < 300; i += 4 {
		b.Insert(i)
	}

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				dst := NewAvlTree[int]()
				dst.SymmetricDifference(a, b)
				_ = dst.Len()
				_ = dst.String()
			}
		}()
	}
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = a.Contains(i)
				_ = a.Slice()
				_, _ = a.Floor(i)
				_, _ = b.Ceil(i)
			}
		}()
	}
	wg.Wait()

	// The operands were only read: both unchanged.
	if a.Len() != 150 || b.Len() != 75 {
		t.Errorf("operands changed: a=%d b=%d, want 150 75", a.Len(), b.Len())
	}
	dst := NewAvlTree[int]()
	dst.SymmetricDifference(a, b)
	// a = evens, b = every 4th odd starting 1; sym diff = evens + odds not ≡1 (mod 4).
	if dst.Len() != 150+75 {
		t.Errorf("symmetric difference len=%d, want 225 (the operand sets are disjoint)", dst.Len())
	}
}

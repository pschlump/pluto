package rb_tree

// Tests for the set-convenience capabilities (capabilities.go):
// materialization, deterministic rendering, deep copy, membership
// batches, predicate bulk delete, the nil/zero-value read contract, and
// the red-black invariants across every new mutation path.  The same
// file (package name aside) runs against the rb_tree_ts twin — the
// twins must behave identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// capKey builds a TestRbTreeNode keyed by its zero-padded index, so the
// S-ordering and the integer ordering agree.
func capKey(i int) TestRbTreeNode {
	return TestRbTreeNode{S: fmt.Sprintf("%08d", i)}
}

func TestCapabilitiesContains(t *testing.T) {
	tree := NewRbTree[string]()
	tree.Insert("alpha")
	if !tree.Contains("alpha") {
		t.Errorf("Contains should find a stored element")
	}
	if tree.Contains("beta") {
		t.Errorf("Contains should not find a missing element")
	}
	var nilTree *RbTree[string]
	if nilTree.Contains("alpha") {
		t.Errorf("Contains on a nil tree should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	tree := newTestTree()
	tree.Insert(capKey(20))
	tree.Insert(capKey(10))
	tree.Insert(capKey(30))
	tree.Insert(TestRbTreeNode{S: "00000010", N: 7}) // replace: still 3 elements

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
	got[0] = TestRbTreeNode{S: "zzzzzzzz"}
	if _, found := tree.Search(TestRbTreeNode{S: "zzzzzzzz"}); found {
		t.Errorf("mutating the Slice result changed the tree")
	}

	if empty := newTestTree().Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty tree should be empty, got %d", len(empty))
	}
	var nilTree *RbTree[TestRbTreeNode]
	if s := nilTree.Slice(); s != nil {
		t.Errorf("Slice of a nil tree should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed trees holding the same elements (inserted
	// in different orders, so different shapes) must render identically.
	a := NewRbTree[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		a.Insert(s)
	}
	b := NewRbTree[string]()
	for _, s := range []string{"fig", "pear", "apple"} {
		b.Insert(s)
	}
	if a.String() != b.String() {
		t.Errorf("same elements must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[apple fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewRbTree[string]().String(); got != "[]" {
		t.Errorf("String of an empty tree = %q, want []", got)
	}
	var nilTree *RbTree[string]
	if got := nilTree.String(); got != "[]" {
		t.Errorf("String of a nil tree = %q, want []", got)
	}

	numbers := NewRbTree[int]()
	numbers.Insert(9)
	numbers.Insert(2)
	if got, want := numbers.String(), "[2 9]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	tree := NewRbTree[int]()
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
		tree.Insert(capKey(i))
	}
	checkInvariants(t, tree)

	dup := tree.Clone()
	if !dup.Equal(tree) || !tree.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	checkInvariants(t, dup) // colors and parent links carried over correctly

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(capKey(3))
	dup.Delete(capKey(50))
	if tree.Len() != 8 || !tree.Contains(capKey(50)) || tree.Contains(capKey(3)) {
		t.Errorf("mutating the clone changed the receiver: len=%d", tree.Len())
	}

	// And the other direction.
	tree.Insert(capKey(2))
	if dup.Contains(capKey(2)) {
		t.Errorf("mutating the receiver changed the clone")
	}

	// The comparison function carried over: the clone still matches by S
	// only, and a duplicate insert replaces the satellite data.
	dup.Insert(TestRbTreeNode{S: "00000043", N: 99}) // 43 was inserted (50-7)
	if v, found := dup.Search(TestRbTreeNode{S: "00000043"}); !found || v.N != 99 {
		t.Errorf("clone should keep the comparison function (replace by S), got %v %v", v, found)
	}

	var nilTree *RbTree[TestRbTreeNode]
	if c := nilTree.Clone(); c != nil {
		t.Errorf("Clone of a nil tree should be nil, got %v", c)
	}
	var zero RbTree[TestRbTreeNode]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value tree should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same elements inserted in different orders (different shapes): equal.
	a := newTestTree()
	for _, k := range []int{1, 2, 3, 4} {
		a.Insert(capKey(k))
	}
	b := newTestTree()
	for _, k := range []int{4, 3, 2, 1} {
		b.Insert(capKey(k))
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("trees with the same elements must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths.
	c := newTestTree()
	c.Insert(capKey(1))
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different elements.
	d := newTestTree()
	for _, k := range []int{1, 2, 3, 99} {
		d.Insert(capKey(k))
	}
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("same length but different elements must not be Equal")
	}

	// Empty trees, nil trees.
	empty := newTestTree()
	if !empty.Equal(newTestTree()) {
		t.Errorf("two empty trees must be Equal")
	}
	var nilTree *RbTree[TestRbTreeNode]
	if !empty.Equal(nilTree) || !nilTree.Equal(empty) {
		t.Errorf("an empty tree and a nil tree must be Equal")
	}
	if !nilTree.Equal(nilTree) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	tree := NewRbTree[int]()
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
	var nilTree *RbTree[int]
	if nilTree.ContainsSlice([]int{1}) || !nilTree.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice on a nil tree: non-empty false, empty true")
	}
	if nilTree.ContainsAny([]int{1}) {
		t.Errorf("ContainsAny on a nil tree should be false")
	}
}

func TestCapabilitiesInsertBatches(t *testing.T) {
	tree := NewRbTree[int]()
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
	other := NewRbTree[int]()
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
	self := NewRbTree[int]()
	self.InsertSlice([]int{1, 2, 3})
	if changed := self.InsertIter(self.All()); changed || self.Len() != 3 {
		t.Errorf("self-feed InsertIter: changed=%v len=%d, want false 3", changed, self.Len())
	}

	// Ascending insert keeps the red-black properties intact.
	tree2 := newTestTree()
	for i := 1; i <= 10; i++ {
		tree2.Insert(capKey(i))
	}
	if tree2.Len() != 10 {
		t.Fatalf("ascending insert len=%d, want 10", tree2.Len())
	}
	checkInvariants(t, tree2)

	tree3 := newTestTree()
	tree3.InsertSlice([]TestRbTreeNode{capKey(1), capKey(2), capKey(3), capKey(4), capKey(5),
		capKey(6), capKey(7), capKey(8), capKey(9), capKey(10)})
	if tree3.Len() != 10 || !tree3.Equal(tree2) {
		t.Fatalf("ascending batch len=%d, want 10 equal to the loop-built tree", tree3.Len())
	}
	checkInvariants(t, tree3)
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	tree := NewRbTree[int]()
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
	// (The predicate reads the last digit of the zero-padded key, which
	// has the same parity as the index it encodes.)
	tree2 := newTestTree()
	for i := 0; i < 200; i++ {
		tree2.Insert(capKey(i))
	}
	if changed := tree2.DeleteFunc(func(v TestRbTreeNode) bool {
		return len(v.S) > 0 && v.S[len(v.S)-1]%2 == 0
	}); !changed {
		t.Errorf("DeleteFunc that matches half should report true")
	}
	if tree2.Len() != 100 {
		t.Fatalf("after DeleteFunc len=%d, want 100", tree2.Len())
	}
	for i := 0; i < 200; i++ {
		_, found := tree2.Search(capKey(i))
		if i%2 == 0 && found {
			t.Errorf("even element %d should be deleted", i)
		}
		if i%2 == 1 && !found {
			t.Errorf("odd element %d should remain", i)
		}
	}
	checkInvariants(t, tree2)

	if changed := tree2.DeleteFunc(func(TestRbTreeNode) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := tree2.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}

	var nilTree *RbTree[int]
	if nilTree.DeleteSlice([]int{1}) || nilTree.DeleteFunc(func(int) bool { return true }) {
		t.Errorf("deletes on a nil tree should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose fixup damage: fill, delete roughly half by predicate,
// then re-verify every membership and the red-black invariants.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	tree := newTestTree()
	model := map[int]bool{}
	for i := 0; i < 1000; i++ {
		tree.Insert(capKey(i))
		model[i] = true
	}
	if !tree.DeleteFunc(func(v TestRbTreeNode) bool {
		n, _ := strconv.Atoi(v.S)
		return n%3 == 0 || n%5 == 0
	}) {
		t.Fatalf("DeleteFunc should report true")
	}
	for i := range model {
		want := i%3 != 0 && i%5 != 0
		if tree.Contains(capKey(i)) != want {
			t.Fatalf("after mass DeleteFunc, Contains(%d)=%v, want %v", i, tree.Contains(capKey(i)), want)
		}
	}
	if tree.Len() != 533 { // 1000 - |div by 3 or 5| = 1000 - (333+200-66)
		t.Errorf("after mass DeleteFunc len=%d, want 533", tree.Len())
	}
	checkInvariants(t, tree)
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTree *RbTree[string]
	var zero RbTree[string]
	for name, tr := range map[string]*RbTree[string]{"nil tree": nilTree, "zero value": &zero} {
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
	rng := rand.New(rand.NewPCG(1, 42))
	tree := newTestTree()
	model := map[int]bool{}

	key := func() int { return rng.IntN(200) }

	for step := 0; step < 800; step++ {
		switch rng.IntN(6) {
		case 0, 1:
			k := key()
			model[k] = true
			tree.Insert(capKey(k))
		case 2:
			k := key()
			delete(model, k)
			tree.Delete(capKey(k))
		case 3:
			// Batch insert of 1..3 keys.
			items := make([]TestRbTreeNode, rng.IntN(4))
			for i := range items {
				k := key()
				model[k] = true
				items[i] = capKey(k)
			}
			tree.InsertSlice(items)
		case 4:
			// Predicate delete on a digit class.
			d := rng.IntN(10)
			for k := range model {
				if k%10 == d {
					delete(model, k)
				}
			}
			tree.DeleteFunc(func(v TestRbTreeNode) bool {
				return len(v.S) > 0 && v.S[len(v.S)-1]-'0' == byte(d)
			})
		case 5:
			// Batch delete of 1..3 keys.
			n := rng.IntN(4)
			items := make([]TestRbTreeNode, 0, n)
			for i := 0; i < n; i++ {
				k := key()
				delete(model, k)
				items = append(items, capKey(k))
			}
			tree.DeleteSlice(items)
		}

		if tree.Len() != len(model) {
			t.Fatalf("step %d: len=%d, model=%d", step, tree.Len(), len(model))
		}

		if step%100 == 99 {
			checkInvariants(t, tree)
			for k, want := range model {
				if tree.Contains(capKey(k)) != want {
					t.Fatalf("step %d: Contains(%d)=%v, want %v", step, k, tree.Contains(capKey(k)), want)
				}
			}
			snap := tree.Slice()
			sortedModel := capabilitiesModelKeys(model)
			sort.Ints(sortedModel)
			want := make([]TestRbTreeNode, len(sortedModel))
			for i, k := range sortedModel {
				want[i] = capKey(k)
			}
			if !g_lib.EqualSlice(snap, want) {
				t.Fatalf("step %d: Slice is not the sorted model (it must be ascending)", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh tree built from the model.
	want := capabilitiesModelKeys(model)
	sort.Ints(want)
	fresh := newTestTree()
	for _, k := range want {
		fresh.Insert(capKey(k))
	}
	if !tree.Equal(fresh) || !fresh.Equal(tree) {
		t.Errorf("final tree must Equal a fresh tree built from the model")
	}
	if tree.String() != fresh.String() {
		t.Errorf("same elements must render the same String: %q vs %q", tree.String(), fresh.String())
	}
	probes := make([]TestRbTreeNode, len(want))
	for i, k := range want {
		probes[i] = capKey(k)
	}
	if !tree.ContainsSlice(probes) {
		t.Errorf("final tree must contain every model key")
	}
	if tree.ContainsSlice(append(probes, TestRbTreeNode{S: "zzzzzzzz"})) {
		t.Errorf("ContainsSlice with a missing key must be false")
	}
	if !tree.ContainsAny(append(probes, TestRbTreeNode{S: "zzzzzzzz"})) {
		t.Errorf("ContainsAny with one present key must be true")
	}
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

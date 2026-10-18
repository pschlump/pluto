package binary_tree_ts

// Tests for the set-convenience capabilities (capabilities.go):
// materialization, deterministic rendering, deep copy, membership
// batches, predicate bulk delete, and the nil/zero-value read contract.
// The same file (package name aside) runs against the binary_tree_ts
// twin — the twins must behave identically.

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
	tree := NewBinaryTree[string]()
	tree.Insert("alpha")
	if !tree.Contains("alpha") {
		t.Errorf("Contains should find a stored element")
	}
	if tree.Contains("beta") {
		t.Errorf("Contains should not find a missing element")
	}
	var nilTree *BinaryTree[string]
	if nilTree.Contains("alpha") {
		t.Errorf("Contains on a nil tree should be false")
	}
}

func TestCapabilitiesSlice(t *testing.T) {
	tree := newTestTree()
	tree.Insert(TestTreeNode{S: "m", N: 1})
	tree.Insert(TestTreeNode{S: "a", N: 2})
	tree.Insert(TestTreeNode{S: "z", N: 3})
	tree.Insert(TestTreeNode{S: "a", N: 4}) // replace: still 3 elements

	got := tree.Slice()
	if len(got) != 3 {
		t.Fatalf("Slice length = %d, want 3", len(got))
	}
	// Slice is in-order: ascending without any sorting.
	if got[0].S != "a" || got[1].S != "m" || got[2].S != "z" {
		t.Errorf("Slice order = [%s %s %s], want [a m z]", got[0].S, got[1].S, got[2].S)
	}
	// Replacement keeps the last value.
	if got[0].N != 4 {
		t.Errorf("Slice should carry the replaced value, got N=%d, want 4", got[0].N)
	}

	// The slice is a copy: mutating it must not affect the tree.
	got[0] = TestTreeNode{S: "zzz"}
	if _, found := tree.Search(TestTreeNode{S: "zzz"}); found {
		t.Errorf("mutating the Slice result changed the tree")
	}

	if empty := newTestTree().Slice(); len(empty) != 0 {
		t.Errorf("Slice of an empty tree should be empty, got %d", len(empty))
	}
	var nilTree *BinaryTree[TestTreeNode]
	if s := nilTree.Slice(); s != nil {
		t.Errorf("Slice of a nil tree should be nil, got %v", s)
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	// Two separately constructed trees holding the same elements (inserted
	// in different orders, so different shapes) must render identically.
	a := NewBinaryTree[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		a.Insert(s)
	}
	b := NewBinaryTree[string]()
	for _, s := range []string{"fig", "pear", "apple"} {
		b.Insert(s)
	}
	if a.String() != b.String() {
		t.Errorf("same elements must render identically: %q vs %q", a.String(), b.String())
	}
	if got, want := a.String(), "[apple fig pear]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	if got := NewBinaryTree[string]().String(); got != "[]" {
		t.Errorf("String of an empty tree = %q, want []", got)
	}
	var nilTree *BinaryTree[string]
	if got := nilTree.String(); got != "[]" {
		t.Errorf("String of a nil tree = %q, want []", got)
	}

	numbers := NewBinaryTree[int]()
	numbers.Insert(9)
	numbers.Insert(2)
	if got, want := numbers.String(), "[2 9]"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	tree := NewBinaryTree[int]()
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
	for i, s := range []string{"m", "a", "z", "k"} {
		tree.Insert(TestTreeNode{S: s, N: i})
	}

	dup := tree.Clone()
	if !dup.Equal(tree) || !tree.Equal(dup) {
		t.Errorf("clone must be equal to the receiver")
	}
	checkInvariants(t, dup)

	// Deep copy: mutate the clone, the receiver must not move.
	dup.Insert(TestTreeNode{S: "d"})
	dup.Delete(TestTreeNode{S: "a"})
	if tree.Len() != 4 || !tree.Contains(TestTreeNode{S: "a"}) || tree.Contains(TestTreeNode{S: "d"}) {
		t.Errorf("mutating the clone changed the receiver: len=%d", tree.Len())
	}

	// And the other direction.
	tree.Insert(TestTreeNode{S: "e"})
	if dup.Len() != 4 || dup.Contains(TestTreeNode{S: "e"}) {
		t.Errorf("mutating the receiver changed the clone: len=%d", dup.Len())
	}

	// The comparison function carried over: the clone still matches by S
	// only, and a duplicate insert replaces the satellite data.
	dup.Insert(TestTreeNode{S: "k", N: 99})
	if v, found := dup.Search(TestTreeNode{S: "k"}); !found || v.N != 99 {
		t.Errorf("clone should keep the comparison function (replace by S), got %v %v", v, found)
	}

	var nilTree *BinaryTree[TestTreeNode]
	if c := nilTree.Clone(); c != nil {
		t.Errorf("Clone of a nil tree should be nil, got %v", c)
	}
	var zero BinaryTree[TestTreeNode]
	if c := zero.Clone(); c.Len() != 0 {
		t.Errorf("Clone of a zero-value tree should be empty")
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	// Same elements inserted in different orders (different shapes): equal.
	a := newTestTree()
	for _, s := range []string{"a", "b", "c", "d"} {
		a.Insert(TestTreeNode{S: s})
	}
	b := newTestTree()
	for _, s := range []string{"d", "c", "b", "a"} {
		b.Insert(TestTreeNode{S: s})
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Errorf("trees with the same elements must be Equal (both directions)")
	}
	if !a.Equal(a) {
		t.Errorf("Equal must be reflective")
	}

	// Different lengths.
	c := newTestTree()
	c.Insert(TestTreeNode{S: "a"})
	if a.Equal(c) || c.Equal(a) {
		t.Errorf("different lengths must not be Equal")
	}

	// Same length, different elements.
	d := newTestTree()
	for _, s := range []string{"a", "b", "c", "x"} {
		d.Insert(TestTreeNode{S: s})
	}
	if a.Equal(d) || d.Equal(a) {
		t.Errorf("same length but different elements must not be Equal")
	}

	// Empty trees, nil trees.
	empty := newTestTree()
	if !empty.Equal(newTestTree()) {
		t.Errorf("two empty trees must be Equal")
	}
	var nilTree *BinaryTree[TestTreeNode]
	if !empty.Equal(nilTree) || !nilTree.Equal(empty) {
		t.Errorf("an empty tree and a nil tree must be Equal")
	}
	if !nilTree.Equal(nilTree) {
		t.Errorf("nil must Equal nil")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	tree := NewBinaryTree[int]()
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
	var nilTree *BinaryTree[int]
	if nilTree.ContainsSlice([]int{1}) || !nilTree.ContainsSlice([]int{}) {
		t.Errorf("ContainsSlice on a nil tree: non-empty false, empty true")
	}
	if nilTree.ContainsAny([]int{1}) {
		t.Errorf("ContainsAny on a nil tree should be false")
	}
}

func TestCapabilitiesInsertBatches(t *testing.T) {
	tree := NewBinaryTree[int]()
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
	other := NewBinaryTree[int]()
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
	self := NewBinaryTree[int]()
	self.InsertSlice([]int{1, 2, 3})
	if changed := self.InsertIter(self.All()); changed || self.Len() != 3 {
		t.Errorf("self-feed InsertIter: changed=%v len=%d, want false 3", changed, self.Len())
	}
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	tree := NewBinaryTree[int]()
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
	tree2 := newTestTree()
	for i := 0; i < 200; i++ {
		tree2.Insert(TestTreeNode{S: fmt.Sprintf("k%03d", i), N: i})
	}
	if changed := tree2.DeleteFunc(func(v TestTreeNode) bool { return v.N%2 == 0 }); !changed {
		t.Errorf("DeleteFunc that matches half should report true")
	}
	if tree2.Len() != 100 {
		t.Fatalf("after DeleteFunc len=%d, want 100", tree2.Len())
	}
	for i := 0; i < 200; i++ {
		_, found := tree2.Search(TestTreeNode{S: fmt.Sprintf("k%03d", i)})
		if i%2 == 0 && found {
			t.Errorf("even element %d should be deleted", i)
		}
		if i%2 == 1 && !found {
			t.Errorf("odd element %d should remain", i)
		}
	}
	checkInvariants(t, tree2)

	if changed := tree2.DeleteFunc(func(v TestTreeNode) bool { return false }); changed {
		t.Errorf("DeleteFunc matching nothing should report false")
	}
	if changed := tree2.DeleteFunc(nil); changed {
		t.Errorf("DeleteFunc(nil) should report false")
	}

	var nilTree *BinaryTree[int]
	if nilTree.DeleteSlice([]int{1}) || nilTree.DeleteFunc(func(int) bool { return true }) {
		t.Errorf("deletes on a nil tree should report false")
	}
}

// TestCapabilitiesDeleteFuncMassDelete exercises bulk deletion hard
// enough to expose splice damage: fill, delete roughly half by predicate,
// then re-verify every membership and the BST invariants.
func TestCapabilitiesDeleteFuncMassDelete(t *testing.T) {
	tree := NewBinaryTree[int]()
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
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTree *BinaryTree[string]
	var zero BinaryTree[string]
	for name, tr := range map[string]*BinaryTree[string]{"nil tree": nilTree, "zero value": &zero} {
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
	rng := rand.New(rand.NewSource(42))
	tree := NewBinaryTree[int]()
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
			sortedSnap := append([]int(nil), snap...)
			sort.Ints(sortedSnap)
			sortedModel := capabilitiesModelKeys(model)
			sort.Ints(sortedModel)
			if !g_lib.EqualSlice(snap, sortedModel) {
				t.Fatalf("step %d: Slice is not the sorted model (it must be ascending)", step)
			}
			if !g_lib.EqualSlice(snap, sortedSnap) {
				t.Fatalf("step %d: Slice must already be ascending", step)
			}
		}
	}

	// Final full verification, plus Equal and deterministic String
	// against a fresh tree built from the model.
	want := make([]int, 0, len(model))
	for k := range model {
		want = append(want, k)
	}
	sort.Ints(want)
	fresh := NewBinaryTree[int]()
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
// Twin-only tests: locking discipline of the new methods.

// TestCapabilitiesConcurrent runs the new methods from several goroutines
// at once — `go test -race` is the real assertion here (no data race and
// no deadlock), the final-state check just verifies the writes all
// landed.
func TestCapabilitiesConcurrent(t *testing.T) {
	tree := NewBinaryTree[int]()
	tree.InsertSlice([]int{-1, -2}) // a little initial content

	const workers = 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				k := base + i*workers
				switch i % 8 {
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
	tree := NewBinaryTree[int]()
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
	tree := NewBinaryTree[int]()
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
	a := NewBinaryTree[int]()
	b := NewBinaryTree[int]()
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

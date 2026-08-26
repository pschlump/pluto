package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Additional tests: AVL invariant validation, delete-with-successor
// regression, iterators (old-style and range-over-func), empty-tree
// behavior, set operations, and benchmarks.

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// validateAVLNode recursively checks the BST ordering, the AVL balance
// invariant and the cached height of every node.  It returns the computed
// height of the subtree rooted at n.
func validateAVLNode(t *testing.T, n *AvlTreeElement[TestTreeNode]) int {
	t.Helper()
	if n == nil {
		return 0
	}
	lh := validateAVLNode(t, n.left)
	rh := validateAVLNode(t, n.right)
	if n.left != nil && cmpTestTreeNode(n.left.data, n.data) >= 0 {
		t.Fatalf("BST order violated: left child %v >= node %v", n.left.data, n.data)
	}
	if n.right != nil && cmpTestTreeNode(n.right.data, n.data) <= 0 {
		t.Fatalf("BST order violated: right child %v <= node %v", n.right.data, n.data)
	}
	if d := lh - rh; d < -1 || d > 1 {
		t.Fatalf("AVL balance violated at node %v: left=%d right=%d", n.data, lh, rh)
	}
	if want := max(lh, rh) + 1; n.height != want {
		t.Fatalf("cached height wrong at node %v: got %d want %d", n.data, n.height, want)
	}
	return max(lh, rh) + 1
}

// TestTreeDeleteSuccessor is a regression test for deleting a node with
// two children where the node used to patch the hole has a subtree of its
// own.  The old implementation dropped that subtree, silently losing data.
func TestTreeDeleteSuccessor(t *testing.T) {
	tree := newTestTree()
	for _, v := range []int{50, 30, 70, 90, 80} {
		tree.Insert(mkKey(v))
	}
	if !tree.Delete(mkKey(50)) {
		t.Fatalf("Expected to delete 50, not found")
	}
	if got := tree.Length(); got != 4 {
		t.Fatalf("Expected length 4 after delete, got %d", got)
	}
	for _, v := range []int{30, 70, 80, 90} {
		if _, found := tree.Search(mkKey(v)); !found {
			t.Errorf("Expected %d to still be in the tree after deleting 50", v)
		}
	}
	validateAVLNode(t, tree.root)
}

// TestTreeRandomOps inserts and deletes a large number of keys in random
// order, validating the AVL invariants and cross-checking against a map.
func TestTreeRandomOps(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const n = 2000

	tree := newTestTree()
	ref := make(map[int]bool, n)

	for _, v := range rng.Perm(n) {
		tree.Insert(mkKey(v))
		ref[v] = true
	}
	if got := tree.Length(); got != n {
		t.Fatalf("Expected length %d, got %d", n, got)
	}
	validateAVLNode(t, tree.root)

	// In-order iteration must produce sorted output.
	prev := -1
	count := 0
	for item := range tree.All() {
		var v int
		if _, err := fmt.Sscanf(item.S, "%d", &v); err != nil {
			t.Fatalf("bad key %q: %v", item.S, err)
		}
		if v <= prev {
			t.Fatalf("All() not in sorted order: %d after %d", v, prev)
		}
		prev = v
		count++
	}
	if count != n {
		t.Fatalf("All() visited %d items, expected %d", count, n)
	}

	// Delete half of the keys in random order.
	half := rng.Perm(n)[:n/2]
	for _, v := range half {
		if !tree.Delete(mkKey(v)) {
			t.Fatalf("Expected to find %d to delete", v)
		}
		delete(ref, v)
	}
	validateAVLNode(t, tree.root)
	if got := tree.Length(); got != len(ref) {
		t.Fatalf("Expected length %d, got %d", len(ref), got)
	}

	// Everything in ref must be found, everything else must not.
	for i := range n {
		_, found := tree.Search(mkKey(i))
		if ref[i] && !found {
			t.Errorf("Expected to find %d", i)
		}
		if !ref[i] && found {
			t.Errorf("Expected not to find %d", i)
		}
	}
	// Deleting an already-deleted key must report false.
	if tree.Delete(mkKey(half[0])) {
		t.Errorf("Expected Delete of missing key to return false")
	}
}

func TestTreeIterators(t *testing.T) {
	tree := newTestTree()
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	expect := []string{mkKey(0).S, mkKey(2).S, mkKey(3).S, mkKey(5).S, mkKey(9).S}

	// Old-style Front/Value/Next/Done iterator.
	var got []string
	for it := tree.Front(); !it.Done(); it.Next() {
		v, _ := it.Value()
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
		t.Errorf("Front/Next iteration: expected %v got %v", expect, got)
	}
	if v, found := tree.Front().Value(); !found || v.S != expect[0] {
		t.Errorf("Front().Value(): expected %s got %v", expect[0], v)
	}

	// Range-over-func forward.
	got = got[:0]
	for item := range tree.All() {
		got = append(got, item.S)
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
		t.Errorf("All() iteration: expected %v got %v", expect, got)
	}

	// Range-over-func backward.
	var rev []string
	for item := range tree.Backward() {
		rev = append(rev, item.S)
	}
	for i := range expect {
		if rev[i] != expect[len(expect)-1-i] {
			t.Fatalf("Backward() iteration: expected reverse of %v got %v", expect, rev)
		}
	}

	// Early exit from a range-over-func loop must stop iteration.
	count := 0
	for range tree.All() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("Early break: expected 2 iterations, got %d", count)
	}

	// Early exit from a backward range-over-func loop must stop iteration.
	count = 0
	for range tree.Backward() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("Backward early break: expected 2 iterations, got %d", count)
	}
}

func TestTreeEmptyOps(t *testing.T) {
	tree := newTestTree()
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree")
	}
	if tree.Length() != 0 {
		t.Errorf("Expected length 0")
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected depth 0 on empty tree")
	}
	if _, found := tree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on empty tree")
	}
	if _, found := tree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on empty tree")
	}
	if _, found := tree.Search(mkKey(1)); found {
		t.Errorf("Expected not-found from Search on empty tree")
	}
	if tree.Delete(mkKey(1)) {
		t.Errorf("Expected false from Delete on empty tree")
	}
	if tree.DeleteAtHead() || tree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on empty tree")
	}
	if _, found := tree.Index(0); found {
		t.Errorf("Expected not-found from Index on empty tree")
	}
	if !tree.Front().Done() {
		t.Errorf("Expected Front() to be Done on empty tree")
	}
	for range tree.All() {
		t.Errorf("Expected no iterations from All() on empty tree")
	}
	for range tree.Backward() {
		t.Errorf("Expected no iterations from Backward() on empty tree")
	}
	// Reverse and Truncate on an empty tree must be no-ops.
	tree.Reverse()
	tree.Truncate()
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree after no-op Reverse/Truncate")
	}
}

func TestTreeSetOps(t *testing.T) {
	build := func(keys ...int) *AvlTree[TestTreeNode] {
		tr := newTestTree()
		for _, k := range keys {
			tr.Insert(mkKey(k))
		}
		return tr
	}
	keys := func(tr *AvlTree[TestTreeNode]) (rv []string) {
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

	t1 := build(1, 2)
	t2 := build(3, 4)

	// Copy
	dst := newTestTree()
	dst.Insert(mkKey(99))
	dst.Copy(t2)
	eq("Copy", keys(dst), keys(t2))

	// Copy onto itself must be a no-op.
	dst.Copy(dst)
	eq("Copy self", keys(dst), keys(t2))

	// Union
	t3 := build(2, 5)
	dst.Union(t1, t3)
	eq("Union", keys(dst), keys(build(1, 2, 5)))

	// Union with aliased destination must not corrupt the sources
	// mid-operation.
	dst.Union(t1, dst)
	eq("Union self", keys(dst), keys(build(1, 2, 5)))

	// Minus
	dst.Minus(t1, t3)
	eq("Minus", keys(dst), keys(build(1)))

	// Intersect
	dst.Intersect(t1, t3)
	eq("Intersect", keys(dst), keys(build(2)))
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

func BenchmarkInsert(b *testing.B) {
	tree := newTestTree()
	keys := make([]TestTreeNode, b.N)
	for i := range keys {
		keys[i] = mkKey(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(keys[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	const size = 4096
	tree := newTestTree()
	keys := make([]TestTreeNode, size)
	for i := range keys {
		keys[i] = mkKey(i)
		tree.Insert(keys[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(keys[i%size])
	}
}

func BenchmarkDelete(b *testing.B) {
	const size = 4096
	tree := newTestTree()
	keys := make([]TestTreeNode, size)
	for i := range keys {
		keys[i] = mkKey(i)
	}
	fill := func() {
		for _, k := range keys {
			tree.Insert(k)
		}
	}
	fill()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if tree.IsEmpty() {
			fill()
		}
		tree.Delete(keys[i%size])
	}
}

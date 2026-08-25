package avl_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Thorough tests: GetData, Dump, nil-tree panics, iterator edge cases,
// duplicate-replace semantics, walk early termination, and a fixed-seed
// randomized property test cross-checked against a sorted-slice model.

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

func TestTreeGetData(t *testing.T) {
	x := TestTreeNode{S: "hello"}
	e := NewAvlTreeElement(&x)
	if got := e.GetData(); got == nil || got.S != "hello" {
		t.Errorf("GetData: expected hello, got %v", got)
	}
}

func TestTreeDump(t *testing.T) {
	var tree AvlTree[TestTreeNode]

	// Dump on an empty tree must not panic and must produce no output.
	var sb strings.Builder
	tree.Dump(&sb)
	if sb.Len() != 0 {
		t.Errorf("Dump on empty tree: expected no output, got %q", sb.String())
	}

	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	sb.Reset()
	tree.Dump(&sb)
	out := sb.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("Dump: expected 5 lines, got %d: %q", len(lines), out)
	}
	// Dump is an in-order traversal, so the keys must appear in sorted order.
	expect := []string{mkKey(0).S, mkKey(2).S, mkKey(3).S, mkKey(5).S, mkKey(9).S}
	for i, key := range expect {
		if !strings.Contains(lines[i], key) {
			t.Errorf("Dump line %d: expected to contain %q, got %q", i, key, lines[i])
		}
	}

	// Dump must not panic when the writer fails.
	tree.Dump(failingWriter{})
}

// failingWriter always returns an error from Write, to exercise the error
// branch in Dump.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

// mustPanic runs f and fails the test unless f panics.
func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	f()
}

func TestTreeNilPanics(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	key := mkKey(1)

	mustPanic(t, "Insert on nil tree", func() { nilTree.Insert(key) })
	mustPanic(t, "Search on nil tree", func() { nilTree.Search(key) })
	mustPanic(t, "Delete on nil tree", func() { nilTree.Delete(key) })
	mustPanic(t, "FindMin on nil tree", func() { nilTree.FindMin() })
	mustPanic(t, "FindMax on nil tree", func() { nilTree.FindMax() })
	mustPanic(t, "DeleteAtHead on nil tree", func() { nilTree.DeleteAtHead() })
	mustPanic(t, "DeleteAtTail on nil tree", func() { nilTree.DeleteAtTail() })
	mustPanic(t, "Reverse on nil tree", func() { nilTree.Reverse() })
	mustPanic(t, "Index on nil tree", func() { nilTree.Index(0) })
	mustPanic(t, "Depth on nil tree", func() { nilTree.Depth() })

	var live AvlTree[TestTreeNode]
	mustPanic(t, "Copy on nil tree", func() { nilTree.Copy(&live) })
	mustPanic(t, "Union on nil tree", func() { nilTree.Union(&live, &live) })
	mustPanic(t, "Minus on nil tree", func() { nilTree.Minus(&live, &live) })
	mustPanic(t, "Intersect on nil tree", func() { nilTree.Intersect(&live, &live) })
}

// TestTreeIteratorEdge covers the edge branches of the old-style iterator:
// Value/Next after the iterator is exhausted.
func TestTreeIteratorEdge(t *testing.T) {
	var tree AvlTree[TestTreeNode]

	// Empty tree: Front is immediately Done, Value is nil, Next is a no-op.
	it := tree.Front()
	if !it.Done() {
		t.Errorf("Expected Front() to be Done on empty tree")
	}
	if it.Value() != nil {
		t.Errorf("Expected nil Value() on empty tree iterator")
	}
	it.Next() // must not panic
	if !it.Done() {
		t.Errorf("Expected Done() after Next() on empty tree iterator")
	}

	// Non-empty tree: walk to the end, then Value is nil and Next stays done.
	tree.Insert(mkKey(1))
	tree.Insert(mkKey(2))
	it = tree.Front()
	steps := 0
	for !it.Done() {
		it.Next()
		steps++
	}
	if steps != 2 {
		t.Errorf("Expected 2 steps to exhaust iterator, got %d", steps)
	}
	if it.Value() != nil {
		t.Errorf("Expected nil Value() after iterator exhausted")
	}
	it.Next() // Next on exhausted iterator must be a no-op, not a panic
	if !it.Done() {
		t.Errorf("Expected Done() to hold after extra Next()")
	}

	// Next must descend the left spine of the right subtree: with the
	// balanced tree of keys 1..7, node 4's right child 6 has left child 5,
	// so after 4 the iterator pushes 6 and lands on 5.
	var tree2 AvlTree[TestTreeNode]
	for v := 1; v <= 7; v++ {
		tree2.Insert(mkKey(v))
	}
	var order []string
	for it := tree2.Front(); !it.Done(); it.Next() {
		order = append(order, it.Value().S)
	}
	var expect []string
	for v := 1; v <= 7; v++ {
		expect = append(expect, mkKey(v).S)
	}
	if fmt.Sprintf("%v", order) != fmt.Sprintf("%v", expect) {
		t.Errorf("Iterator descent through right subtree: expected %v, got %v", expect, order)
	}

	// calcAvlBalance on a nil node is defined as 0.
	if got := tree.calcAvlBalance(nil); got != 0 {
		t.Errorf("calcAvlBalance(nil): expected 0, got %d", got)
	}
	if got := tree.Height(nil); got != 0 {
		t.Errorf("Height(nil): expected 0, got %d", got)
	}
}

// TestTreeDuplicateReplace verifies that inserting a duplicate replaces the
// stored item (keeps the newest pointer) and does not change the length.
func TestTreeDuplicateReplace(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length 5, got %d", got)
	}

	// Insert a duplicate of "2" and check the stored pointer is the new one.
	repl := mkKey(2)
	tree.Insert(repl)
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length to stay 5 after duplicate insert, got %d", got)
	}
	got := tree.Search(mkKey(2))
	if got != repl {
		t.Errorf("Expected duplicate insert to replace stored item pointer")
	}

	// Replace the root too, then check the tree is still structurally sound.
	repl = mkKey(5)
	tree.Insert(repl)
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length to stay 5 after duplicate root insert, got %d", got)
	}
	if tree.Search(mkKey(5)) != repl {
		t.Errorf("Expected duplicate root insert to replace stored item pointer")
	}
	validateAVLNode(t, tree.root)
}

// TestTreeSingleElement covers operations on a tree holding exactly one item.
func TestTreeSingleElement(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	tree.Insert(mkKey(7))

	if tree.IsEmpty() {
		t.Errorf("Expected non-empty tree")
	}
	if got := tree.Length(); got != 1 {
		t.Errorf("Expected length 1, got %d", got)
	}
	if got := tree.Depth(); got != 1 {
		t.Errorf("Expected depth 1, got %d", got)
	}
	if got := tree.FindMin(); got == nil || got.S != mkKey(7).S {
		t.Errorf("FindMin on single-element tree: got %v", got)
	}
	if got := tree.FindMax(); got == nil || got.S != mkKey(7).S {
		t.Errorf("FindMax on single-element tree: got %v", got)
	}
	if got := tree.Index(0); got == nil || got.S != mkKey(7).S {
		t.Errorf("Index(0) on single-element tree: got %v", got)
	}
	if tree.Index(1) != nil || tree.Index(-1) != nil {
		t.Errorf("Expected nil for out-of-range Index on single-element tree")
	}
	// DeleteAtTail on a single element must drain the tree.
	if !tree.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to succeed on single-element tree")
	}
	if !tree.IsEmpty() || tree.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtTail of last element")
	}
	// Re-fill and drain via DeleteAtHead.
	tree.Insert(mkKey(7))
	if !tree.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to succeed on single-element tree")
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree after DeleteAtHead of last element")
	}
}

// TestTreeWalkEarlyStop verifies that returning false from the walk callback
// terminates WalkInOrder, WalkPreOrder and WalkPostOrder, and that the walks
// on an empty tree never invoke the callback.
func TestTreeWalkEarlyStop(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}

	// Early stop after 2 nodes for each order.
	for name, walk := range map[string]func(ApplyFunction[TestTreeNode], interface{}){
		"WalkInOrder":   tree.WalkInOrder,
		"WalkPreOrder":  tree.WalkPreOrder,
		"WalkPostOrder": tree.WalkPostOrder,
	} {
		count := 0
		walk(func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
			count++
			if userData == nil {
				t.Errorf("%s: userData was not passed through", name)
			}
			return count < 2
		}, "sentinel")
		if count != 2 {
			t.Errorf("%s early stop: expected 2 callbacks, got %d", name, count)
		}
	}

	// Empty tree: no callbacks at all.
	var empty AvlTree[TestTreeNode]
	called := false
	fx := func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
		called = true
		return true
	}
	empty.WalkInOrder(fx, nil)
	empty.WalkPreOrder(fx, nil)
	empty.WalkPostOrder(fx, nil)
	if called {
		t.Errorf("Expected no callbacks when walking an empty tree")
	}
}

// TestTreeSetOpsEdge covers set operations with empty operands.
func TestTreeSetOpsEdge(t *testing.T) {
	var empty1, empty2, dst AvlTree[TestTreeNode]

	dst.Insert(mkKey(1))

	dst.Copy(&empty1)
	if !dst.IsEmpty() {
		t.Errorf("Copy of empty tree: expected empty destination")
	}

	dst.Union(&empty1, &empty2)
	if !dst.IsEmpty() {
		t.Errorf("Union of two empty trees: expected empty result")
	}

	dst.Insert(mkKey(1))
	dst.Insert(mkKey(2))
	dst.Minus(&dst, &dst)
	if !dst.IsEmpty() {
		t.Errorf("Minus of tree with itself: expected empty result")
	}

	dst.Insert(mkKey(1))
	var other AvlTree[TestTreeNode]
	dst.Intersect(&dst, &other)
	if !dst.IsEmpty() {
		t.Errorf("Intersect with empty tree: expected empty result")
	}
}

// TestTreeRandomModel is a fixed-seed randomized property test.  It performs
// hundreds of mixed operations (Insert, Delete, Search, FindMin, FindMax,
// Index, DeleteAtHead, DeleteAtTail, iteration) and cross-checks every result
// against a plain sorted-slice reference model.
func TestTreeRandomModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(12345, 67890))
	const keySpace = 100 // small key space to force duplicate inserts

	var tree AvlTree[TestTreeNode]
	var model []int // sorted, unique

	contains := func(v int) bool {
		i := sort.SearchInts(model, v)
		return i < len(model) && model[i] == v
	}
	modelInsert := func(v int) {
		if !contains(v) {
			i := sort.SearchInts(model, v)
			model = append(model, 0)
			copy(model[i+1:], model[i:])
			model[i] = v
		}
	}
	modelDelete := func(v int) bool {
		i := sort.SearchInts(model, v)
		if i < len(model) && model[i] == v {
			model = append(model[:i], model[i+1:]...)
			return true
		}
		return false
	}
	checkAll := func(step int) {
		t.Helper()
		if got := tree.Length(); got != len(model) {
			t.Fatalf("step %d: length mismatch: tree=%d model=%d", step, got, len(model))
		}
		// Full forward iteration must match the sorted model exactly.
		i := 0
		for item := range tree.All() {
			var v int
			if _, err := fmt.Sscanf(item.S, "%d", &v); err != nil {
				t.Fatalf("step %d: bad key %q: %v", step, item.S, err)
			}
			if i >= len(model) || v != model[i] {
				t.Fatalf("step %d: All()[%d]=%d, model has %v", step, i, v, model)
			}
			i++
		}
		if i != len(model) {
			t.Fatalf("step %d: All() visited %d items, model has %d", step, i, len(model))
		}
		// Backward iteration must be the reverse.
		i = len(model) - 1
		for item := range tree.Backward() {
			var v int
			if _, err := fmt.Sscanf(item.S, "%d", &v); err != nil {
				t.Fatalf("step %d: bad key %q: %v", step, item.S, err)
			}
			if i < 0 || v != model[i] {
				t.Fatalf("step %d: Backward()[%d]=%d, model has %v", step, i, v, model)
			}
			i--
		}
		validateAVLNode(t, tree.root)
	}

	for step := 0; step < 1000; step++ {
		v := rng.IntN(keySpace)
		switch rng.IntN(9) {
		case 0, 1, 2: // Insert (may be a duplicate -> replace)
			tree.Insert(mkKey(v))
			modelInsert(v)
		case 3, 4: // Delete
			if got := tree.Delete(mkKey(v)); got != modelDelete(v) {
				t.Fatalf("step %d: Delete(%d) returned %v, model says %v", step, v, got, contains(v))
			}
		case 5: // Search
			got := tree.Search(mkKey(v))
			if want := contains(v); (got != nil) != want {
				t.Fatalf("step %d: Search(%d) found=%v, model says %v", step, v, got != nil, want)
			}
		case 6: // FindMin / FindMax
			mn, mx := tree.FindMin(), tree.FindMax()
			if len(model) == 0 {
				if mn != nil || mx != nil {
					t.Fatalf("step %d: FindMin/FindMax on empty tree returned %v/%v", step, mn, mx)
				}
			} else {
				var mnv, mxv int
				fmt.Sscanf(mn.S, "%d", &mnv)
				fmt.Sscanf(mx.S, "%d", &mxv)
				if mnv != model[0] || mxv != model[len(model)-1] {
					t.Fatalf("step %d: FindMin=%d FindMax=%d, model %v", step, mnv, mxv, model)
				}
			}
		case 7: // DeleteAtHead / DeleteAtTail
			if rng.IntN(2) == 0 {
				got := tree.DeleteAtHead()
				if len(model) == 0 {
					if got {
						t.Fatalf("step %d: DeleteAtHead on empty tree returned true", step)
					}
				} else if got {
					modelDelete(model[0])
				} else {
					t.Fatalf("step %d: DeleteAtHead failed on non-empty tree", step)
				}
			} else {
				got := tree.DeleteAtTail()
				if len(model) == 0 {
					if got {
						t.Fatalf("step %d: DeleteAtTail on empty tree returned true", step)
					}
				} else if got {
					modelDelete(model[len(model)-1])
				} else {
					t.Fatalf("step %d: DeleteAtTail failed on non-empty tree", step)
				}
			}
		case 8: // Index
			if len(model) == 0 {
				if tree.Index(0) != nil {
					t.Fatalf("step %d: Index(0) on empty tree returned non-nil", step)
				}
			} else {
				pos := rng.IntN(len(model))
				got := tree.Index(pos)
				if got == nil {
					t.Fatalf("step %d: Index(%d) returned nil, model %v", step, pos, model)
				}
				var gv int
				fmt.Sscanf(got.S, "%d", &gv)
				if gv != model[pos] {
					t.Fatalf("step %d: Index(%d)=%d, model says %d", step, pos, gv, model[pos])
				}
				if tree.Index(len(model)) != nil || tree.Index(-1) != nil {
					t.Fatalf("step %d: out-of-range Index returned non-nil", step)
				}
			}
		}
		if step%50 == 0 {
			checkAll(step)
		}
	}
	checkAll(1000)

	// Depth of an AVL tree with n nodes must be at most ~1.44 log2(n+2).
	// With n <= 100 that bound is well under 12; check a generous ceiling.
	if len(model) > 0 {
		if d := tree.Depth(); d > 12 {
			t.Fatalf("Depth %d exceeds AVL bound for %d nodes", d, len(model))
		}
	}
}

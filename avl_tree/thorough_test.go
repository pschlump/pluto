package avl_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: GetData, Dump, nil-tree panics, iterator edge cases,
// duplicate-replace semantics, walk early termination, set-operation
// edges, and a fixed-seed randomized property test cross-checked against a
// sorted-slice model.

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

func TestTreeGetData(t *testing.T) {
	e := NewAvlTreeElement(TestTreeNode{S: "hello"})
	if got := e.GetData(); got.S != "hello" {
		t.Errorf("GetData: expected hello, got %v", got)
	}
}

func TestTreeDump(t *testing.T) {
	tree := newTestTree()

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

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// TestTreeNilPanics verifies the documented panic when Insert is called on
// a nil tree — the one operation with no sane answer.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	key := mkKey(1)

	expectPanic(t, "Insert", func() { nilTree.Insert(key) })

	// Verify the panic message names the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert to panic on nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilTree.Insert(key)
	}()
}

// TestTreeNilTolerated verifies that every operation other than Insert
// treats a nil tree as an empty tree instead of panicking.
func TestTreeNilTolerated(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	key := mkKey(1)

	if !nilTree.IsEmpty() {
		t.Errorf("Expected nil tree to be empty.")
	}
	if nilTree.Len() != 0 || nilTree.Length() != 0 {
		t.Errorf("Expected nil tree to have length 0.")
	}
	if nilTree.Depth() != 0 {
		t.Errorf("Expected depth 0 on nil tree.")
	}
	if _, found := nilTree.Search(key); found {
		t.Errorf("Expected not-found from Search on nil tree.")
	}
	if nilTree.Delete(key) {
		t.Errorf("Expected false from Delete on nil tree.")
	}
	if _, found := nilTree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on nil tree.")
	}
	if _, found := nilTree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on nil tree.")
	}
	if nilTree.DeleteAtHead() || nilTree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on nil tree.")
	}
	if _, found := nilTree.Index(0); found {
		t.Errorf("Expected not-found from Index on nil tree.")
	}
	nilTree.Reverse()  // no-op, must not panic
	nilTree.Truncate() // no-op, must not panic

	called := false
	noop := func(pos, depth int, data TestTreeNode) bool {
		called = true
		return true
	}
	nilTree.WalkInOrder(noop)
	nilTree.WalkPreOrder(noop)
	nilTree.WalkPostOrder(noop)
	if called {
		t.Errorf("Expected no walk visits on nil tree.")
	}

	if it := nilTree.Front(); !it.Done() {
		t.Errorf("Expected iterator on nil tree to be Done immediately.")
	}
	for range nilTree.All() {
		t.Errorf("Expected no values from All on nil tree.")
	}
	for range nilTree.Backward() {
		t.Errorf("Expected no values from Backward on nil tree.")
	}

	var sb strings.Builder
	nilTree.Dump(&sb)
	if sb.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil tree.")
	}

	// Set operations tolerate nil in every position: a nil destination is a
	// no-op and a nil operand acts as the empty set.
	live := newTestTree()
	live.Insert(mkKey(1))
	nilTree.Copy(live) // no-op on nil destination
	nilTree.Union(live, live)
	nilTree.Minus(live, live)
	nilTree.Intersect(live, live)

	dst := newTestTree()
	dst.Insert(mkKey(1))
	dst.Copy(nilTree)
	if !dst.IsEmpty() {
		t.Errorf("Expected Copy of a nil tree to empty the destination.")
	}
	dst.Insert(mkKey(1))
	dst.Union(live, nilTree) // a nil operand is the empty set: the result is live's contents
	if dst.Length() != live.Length() {
		t.Errorf("Expected Union with nil operand to treat it as empty, got length %d", dst.Length())
	}
}

// TestTreeIteratorEdge covers the edge branches of the old-style iterator:
// Value/Next after the iterator is exhausted.
func TestTreeIteratorEdge(t *testing.T) {
	tree := newTestTree()

	// Empty tree: Front is immediately Done, Value is not-found, Next is a
	// no-op.
	it := tree.Front()
	if !it.Done() {
		t.Errorf("Expected Front() to be Done on empty tree")
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value() on empty tree iterator")
	}
	it.Next() // must not panic
	if !it.Done() {
		t.Errorf("Expected Done() after Next() on empty tree iterator")
	}

	// Non-empty tree: walk to the end, then Value is not-found and Next
	// stays done.
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
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value() after iterator exhausted")
	}
	it.Next() // Next on exhausted iterator must be a no-op, not a panic
	if !it.Done() {
		t.Errorf("Expected Done() to hold after extra Next()")
	}

	// Next must descend the left spine of the right subtree: with the
	// balanced tree of keys 1..7, node 4's right child 6 has left child 5,
	// so after 4 the iterator pushes 6 and lands on 5.
	tree2 := newTestTree()
	for v := 1; v <= 7; v++ {
		tree2.Insert(mkKey(v))
	}
	var order []string
	for it := tree2.Front(); !it.Done(); it.Next() {
		v, _ := it.Value()
		order = append(order, v.S)
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

// TestTreeDuplicateReplace verifies that inserting a duplicate replaces
// the stored value and does not change the length.
func TestTreeDuplicateReplace(t *testing.T) {
	tree := newTestTree()
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length 5, got %d", got)
	}

	// Insert a duplicate of "2" and check the stored satellite data is the
	// new one.
	if tree.Insert(TestTreeNode{S: mkKey(2).S, N: 7}) {
		t.Fatalf("Expected duplicate insert to return false")
	}
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length to stay 5 after duplicate insert, got %d", got)
	}
	got, found := tree.Search(mkKey(2))
	if !found || got.N != 7 {
		t.Errorf("Expected duplicate insert to replace stored value, got %+v found=%v", got, found)
	}

	// Replace the root too, then check the tree is still structurally sound.
	if tree.Insert(TestTreeNode{S: mkKey(5).S, N: 7}) {
		t.Fatalf("Expected duplicate root insert to return false")
	}
	if got := tree.Length(); got != 5 {
		t.Fatalf("Expected length to stay 5 after duplicate root insert, got %d", got)
	}
	if got, found := tree.Search(mkKey(5)); !found || got.N != 7 {
		t.Errorf("Expected duplicate root insert to replace stored value, got %+v", got)
	}
	validateAVLNode(t, tree.root)
}

// TestTreeSingleElement covers operations on a tree holding exactly one
// item.
func TestTreeSingleElement(t *testing.T) {
	tree := newTestTree()
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
	if got, found := tree.FindMin(); !found || got.S != mkKey(7).S {
		t.Errorf("FindMin on single-element tree: got %v", got)
	}
	if got, found := tree.FindMax(); !found || got.S != mkKey(7).S {
		t.Errorf("FindMax on single-element tree: got %v", got)
	}
	if got, found := tree.Index(0); !found || got.S != mkKey(7).S {
		t.Errorf("Index(0) on single-element tree: got %v", got)
	}
	if _, found := tree.Index(1); found {
		t.Errorf("Expected not-found for out-of-range Index on single-element tree")
	}
	if _, found := tree.Index(-1); found {
		t.Errorf("Expected not-found for negative Index on single-element tree")
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

// TestTreeWalkEarlyStop verifies that returning false from the walk
// callback terminates WalkInOrder, WalkPreOrder and WalkPostOrder, and
// that the walks on an empty tree never invoke the callback.
func TestTreeWalkEarlyStop(t *testing.T) {
	tree := newTestTree()
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}

	// Early stop after 2 nodes for each order.
	for name, walk := range map[string]func(ApplyFunction[TestTreeNode]){
		"WalkInOrder":   tree.WalkInOrder,
		"WalkPreOrder":  tree.WalkPreOrder,
		"WalkPostOrder": tree.WalkPostOrder,
	} {
		count := 0
		walk(func(pos, depth int, data TestTreeNode) bool {
			count++
			return count < 2
		})
		if count != 2 {
			t.Errorf("%s early stop: expected 2 callbacks, got %d", name, count)
		}
	}

	// Empty tree: no callbacks at all.
	empty := newTestTree()
	called := false
	fx := func(pos, depth int, data TestTreeNode) bool {
		called = true
		return true
	}
	empty.WalkInOrder(fx)
	empty.WalkPreOrder(fx)
	empty.WalkPostOrder(fx)
	if called {
		t.Errorf("Expected no callbacks when walking an empty tree")
	}
}

// TestTreeSetOpsEdge covers set operations with empty and nil operands,
// and the zero-value destination adopting a comparison function.
func TestTreeSetOpsEdge(t *testing.T) {
	empty1, empty2 := newTestTree(), newTestTree()
	dst := newTestTree()

	dst.Insert(mkKey(1))

	dst.Copy(empty1)
	if !dst.IsEmpty() {
		t.Errorf("Copy of empty tree: expected empty destination")
	}

	dst.Union(empty1, empty2)
	if !dst.IsEmpty() {
		t.Errorf("Union of two empty trees: expected empty result")
	}

	dst.Insert(mkKey(1))
	dst.Insert(mkKey(2))
	dst.Minus(dst, dst)
	if !dst.IsEmpty() {
		t.Errorf("Minus of tree with itself: expected empty result")
	}

	dst.Insert(mkKey(1))
	other := newTestTree()
	dst.Intersect(dst, other)
	if !dst.IsEmpty() {
		t.Errorf("Intersect with empty tree: expected empty result")
	}

	// A zero-value destination adopts the source's comparison function.
	src := newTestTree()
	for _, v := range []int{3, 1, 2} {
		src.Insert(mkKey(v))
	}
	var zero AvlTree[TestTreeNode]
	zero.Copy(src)
	if zero.Length() != 3 {
		t.Fatalf("Expected Copy onto zero-value tree to adopt the comparator and copy 3 items, got %d", zero.Length())
	}
	if got, found := zero.FindMin(); !found || got.S != mkKey(1).S {
		t.Errorf("Expected min 1 on adopted-comparator tree, got %+v", got)
	}
	if !zero.Insert(mkKey(0)) {
		t.Errorf("Expected Insert to work on a tree that adopted its comparator")
	}
}

// TestTreeRandomModel is a fixed-seed randomized property test.  It
// performs hundreds of mixed operations (Insert, Delete, Search, FindMin,
// FindMax, Index, DeleteAtHead, DeleteAtTail, iteration) and cross-checks
// every result against a plain sorted-slice reference model.
func TestTreeRandomModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(12345, 67890))
	const keySpace = 100 // small key space to force duplicate inserts

	tree := NewAvlTreeFunc(cmpTestTreeNode)
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

	for step := range 1000 {
		v := rng.IntN(keySpace)
		switch rng.IntN(9) {
		case 0, 1, 2: // Insert (may be a duplicate -> replace)
			added := tree.Insert(mkKey(v))
			if added == contains(v) {
				t.Fatalf("step %d: Insert(%d)=%v, model said present=%v", step, v, added, contains(v))
			}
			modelInsert(v)
		case 3, 4: // Delete
			if got := tree.Delete(mkKey(v)); got != modelDelete(v) {
				t.Fatalf("step %d: Delete(%d) returned %v, model said %v", step, v, got, contains(v))
			}
		case 5: // Search
			_, found := tree.Search(mkKey(v))
			if want := contains(v); found != want {
				t.Fatalf("step %d: Search(%d) found=%v, model says %v", step, v, found, want)
			}
		case 6: // FindMin / FindMax
			mn, mnOK := tree.FindMin()
			mx, mxOK := tree.FindMax()
			if len(model) == 0 {
				if mnOK || mxOK {
					t.Fatalf("step %d: FindMin/FindMax on empty tree returned %v/%v", step, mn, mx)
				}
			} else {
				var mnv, mxv int
				if _, err := fmt.Sscanf(mn.S, "%d", &mnv); err != nil {
					t.Fatalf("step %d: bad min key %q: %v", step, mn.S, err)
				}
				if _, err := fmt.Sscanf(mx.S, "%d", &mxv); err != nil {
					t.Fatalf("step %d: bad max key %q: %v", step, mx.S, err)
				}
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
				if _, found := tree.Index(0); found {
					t.Fatalf("step %d: Index(0) on empty tree returned found", step)
				}
			} else {
				pos := rng.IntN(len(model))
				got, found := tree.Index(pos)
				if !found {
					t.Fatalf("step %d: Index(%d) returned not-found, model %v", step, pos, model)
				}
				var gv int
				if _, err := fmt.Sscanf(got.S, "%d", &gv); err != nil {
					t.Fatalf("step %d: bad key %q: %v", step, got.S, err)
				}
				if gv != model[pos] {
					t.Fatalf("step %d: Index(%d)=%d, model says %d", step, pos, gv, model[pos])
				}
				if _, found := tree.Index(len(model)); found {
					t.Fatalf("step %d: out-of-range Index returned found", step)
				}
				if _, found := tree.Index(-1); found {
					t.Fatalf("step %d: negative Index returned found", step)
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

package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: GetData, Dump, nil-tree panics, iterator edge cases,
// duplicate-replace semantics, walk early termination, set-operation
// edges, and a fixed-seed randomized property test cross-checked against a
// sorted-slice model.

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"sync"
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

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestTreeIterateSnapshot verifies that the Front/All/Backward iterators
// operate on a snapshot taken when they are called: later modifications —
// even truncating the whole tree — are not observed, and mutating the tree
// from inside the loop is safe.
func TestTreeIterateSnapshot(t *testing.T) {
	tree := newTestTree()
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}

	all := tree.All()
	backward := tree.Backward()
	it := tree.Front()

	tree.Truncate() // the iterators above must not observe this

	expect := []string{mkKey(0).S, mkKey(2).S, mkKey(3).S, mkKey(5).S, mkKey(9).S}

	var got []string
	for v := range all {
		got = append(got, v.S)
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, got)
	}

	var gotB []string
	for v := range backward {
		gotB = append(gotB, v.S)
	}
	for i := range expect {
		if gotB[i] != expect[len(expect)-1-i] {
			t.Fatalf("Backward after Truncate error, expected reverse of %v got %v", expect, gotB)
		}
	}

	var gotF []string
	for ; !it.Done(); it.Next() {
		v, _ := it.Value()
		gotF = append(gotF, v.S)
	}
	if !reflect.DeepEqual(gotF, expect) {
		t.Errorf("Front iterator after Truncate error, expected %v got %v", expect, gotF)
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	tree = newTestTree()
	for _, v := range []int{5, 2, 9} {
		tree.Insert(mkKey(v))
	}
	visited := 0
	for v := range tree.All() {
		visited++
		tree.Delete(v)
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while deleting during iteration, got %d", visited)
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree after deleting every visited element.")
	}
}

// TestTreeConcurrent runs writers (each owning a disjoint key range)
// against a reader that iterates snapshots and queries metadata in a
// tight loop.  It is primarily a test for the race detector (`make race`);
// it also verifies that every operation reports success and that the tree
// ends up empty, balanced and consistent.
func TestTreeConcurrent(t *testing.T) {
	tree := NewAvlTreeFunc(cmpTestTreeNode)

	const workers = 8
	const perWorker = 200

	stop := make(chan struct{})
	var writers sync.WaitGroup

	// Reader: iterate snapshots and query metadata while the writers work.
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			for range tree.All() {
			}
			for range tree.Backward() {
			}
			_ = tree.Len()
			_ = tree.Depth()
			_ = tree.IsEmpty()
			_, _ = tree.FindMin()
			_, _ = tree.FindMax()
		}
	}()

	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				k := TestTreeNode{S: fmt.Sprintf("%02d-%04d", w, i)}
				if !tree.Insert(k) {
					t.Errorf("worker %d: Insert(%s) returned false", w, k.S)
					return
				}
			}
			for i := range perWorker {
				k := TestTreeNode{S: fmt.Sprintf("%02d-%04d", w, i)}
				if _, found := tree.Search(k); !found {
					t.Errorf("worker %d: Search(%s) not found", w, k.S)
				}
				if !tree.Delete(k) {
					t.Errorf("worker %d: Delete(%s) returned false", w, k.S)
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)

	if !tree.IsEmpty() || tree.Length() != 0 {
		t.Errorf("Expected empty tree after concurrent insert/delete, got length %d", tree.Length())
	}
}

// TestTreeConcurrentSetOps runs many goroutines building set-operation
// results from two shared, immutable source trees.  The sources are read
// concurrently through their snapshots; they must not change.
func TestTreeConcurrentSetOps(t *testing.T) {
	a, b := newTestTree(), newTestTree()
	for i := range 50 {
		a.Insert(mkKey(i))
	}
	for i := 50; i < 100; i++ {
		b.Insert(mkKey(i))
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			dst := newTestTree()
			for range 20 {
				dst.Union(a, b)
				if dst.Length() != 100 {
					t.Errorf("Union: expected 100 elements, got %d", dst.Length())
					return
				}
				dst.Minus(a, b)
				if dst.Length() != 50 {
					t.Errorf("Minus: expected 50 elements, got %d", dst.Length())
					return
				}
				dst.Intersect(a, b)
				if dst.Length() != 0 {
					t.Errorf("Intersect: expected 0 elements, got %d", dst.Length())
					return
				}
				dst.Copy(b)
				if dst.Length() != 50 {
					t.Errorf("Copy: expected 50 elements, got %d", dst.Length())
					return
				}
			}
		})
	}
	wg.Wait()

	if a.Length() != 50 || b.Length() != 50 {
		t.Errorf("Sources changed by concurrent set operations: a=%d b=%d", a.Length(), b.Length())
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the tree.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output in in-order (sorted) sequence, whatever the
	// insertion order was.
	tree := NewAvlTree[int]()
	for _, v := range []int{3, 1, 2} {
		tree.Insert(v)
	}
	b, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal(tree): %v", err)
	}
	if string(b) != "[1,2,3]" {
		t.Errorf("Expected [1,2,3], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := newTestTree()
	for _, s := range []string{"b", "a"} {
		items.Insert(TestTreeNode{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a","N":0},{"S":"b","N":0}]` {
		t.Errorf(`Expected [{"S":"a","N":0},{"S":"b","N":0}], got (%s, %v)`, b, err)
	}

	// An empty tree encodes as [].
	if b, err := json.Marshal(NewAvlTree[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero AvlTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *AvlTree never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTree *AvlTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewAvlTree[upperString]()
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewAvlTreeFunc(func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a tree of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// The decoded elements land in the tree in sorted sequence, whatever
	// the order of the array.
	tree := NewAvlTree[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for v := range tree.All() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("Expected [1 2 3], got %v", got)
	}
	if min, found := tree.FindMin(); !found || min != 1 {
		t.Errorf("Expected min 1, got (%v, %v)", min, found)
	}

	// A round trip rebuilds a structurally sound tree and keeps the
	// comparison function (Search works on the rebuilt tree).
	items := newTestTree()
	for _, v := range []int{3, 1, 2} {
		items.Insert(mkKey(v))
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTree()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	validateAVLNode(t, again.root)
	var keys []string
	for v := range again.All() {
		keys = append(keys, v.S)
	}
	if got, want := fmt.Sprint(keys), fmt.Sprint([]string{mkKey(1).S, mkKey(2).S, mkKey(3).S}); got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, found := again.Search(mkKey(2)); !found {
		t.Errorf("Expected Search to work after unmarshal.")
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte("[7]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := tree.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the tree.
	full := newTestTree()
	full.Insert(TestTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the tree.")
	}
	full.Insert(TestTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the tree.")
	}

	// Element-level unmarshalers are honored.
	custom := NewAvlTree[upperString]()
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for v := range custom.All() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the tree untouched.
	keep := newTestTree()
	keep.Insert(TestTreeNode{S: "keep"})
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := keep.Length(); got != 1 {
			t.Errorf("Tree changed after the error on %s: length %d", badData, got)
		}
		if v, found := keep.FindMin(); !found || v.S != "keep" {
			t.Errorf("Tree changed after the error on %s: min %v", badData, v)
		}
	}
	validateAVLNode(t, keep.root)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero AvlTree[TestTreeNode]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value tree to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewAvlTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilTree *AvlTree[TestTreeNode]
	if err := nilTree.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil tree to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil tree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilTree.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()
}

// TestJSONStructField marshals and unmarshals an AvlTree nested in a
// struct through the encoding/json package.  The tree must be created
// with NewAvlTree/NewAvlTreeFunc before unmarshaling: for a nil *AvlTree
// field the json package allocates a zero-value tree itself (no
// comparison function), so non-empty data panics with the insert-family
// message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string           `json:"title"`
		Tags  *AvlTree[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewAvlTree[string]()}
	d.Tags.Insert("go")
	d.Tags.Insert("ds")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created tree field.
	var out Doc
	out.Tags = NewAvlTree[string]()
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for v := range out.Tags.All() {
		tags = append(tags, v)
	}
	if fmt.Sprint(tags) != "[ds go]" {
		t.Errorf("Expected [ds go], got %v", tags)
	}

	// A nil tree field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created tree and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewAvlTree[string]()}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the tree.")
	}

	// Non-empty data into a nil *AvlTree field: the json package allocates
	// a zero-value tree, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated tree field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewAvlTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a sorted-slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 500

	tree := NewAvlTree[int]()
	model := []int{} // sorted, unique; non-nil so an emptied model marshals as [] like the tree

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
	modelDelete := func(v int) {
		i := sort.SearchInts(model, v)
		if i < len(model) && model[i] == v {
			model = append(model[:i], model[i+1:]...)
		}
	}

	for step := range ops {
		v := rng.IntN(100)
		switch rng.IntN(2) {
		case 0:
			tree.Insert(v)
			modelInsert(v)
		case 1:
			tree.Delete(v)
			modelDelete(v)
		}

		// Marshal must equal the model marshaled as a plain slice (the
		// tree marshals in in-order, which is the sorted order).
		got, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(model)
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh tree must reproduce the model.
		fresh := NewAvlTree[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		var vals []int
		for v := range fresh.All() {
			vals = append(vals, v)
		}
		if fmt.Sprint(vals) != fmt.Sprint(model) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, model)
		}
	}
}

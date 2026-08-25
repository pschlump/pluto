package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Thorough tests: documented nil-tree panics, node accessors, Dump output,
// duplicate replacement semantics, early-stop walks, iterator edge cases,
// Index bounds, DeleteAtHead/DeleteAtTail drain order, a fixed-seed
// randomized model test, and a mixed concurrent-use test for -race.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"testing"
)

// expectPanic runs fx and reports an error unless it panics.
func expectPanic(t *testing.T, what string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected a panic on a nil tree, got none", what)
		}
	}()
	fx()
}

// TestTreeNilTreePanics verifies that every operation documented to panic on
// a nil tree actually does so.
func TestTreeNilTreePanics(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	key := mkKey(1)

	expectPanic(t, "Insert", func() { nilTree.Insert(key) })
	expectPanic(t, "Search", func() { nilTree.Search(key) })
	expectPanic(t, "Delete", func() { nilTree.Delete(key) })
	expectPanic(t, "FindMin", func() { nilTree.FindMin() })
	expectPanic(t, "FindMax", func() { nilTree.FindMax() })
	expectPanic(t, "DeleteAtHead", func() { nilTree.DeleteAtHead() })
	expectPanic(t, "DeleteAtTail", func() { nilTree.DeleteAtTail() })
	expectPanic(t, "Reverse", func() { nilTree.Reverse() })
	expectPanic(t, "Index", func() { nilTree.Index(0) })
	expectPanic(t, "Depth", func() { nilTree.Depth() })
	expectPanic(t, "WalkInOrder", func() {
		nilTree.WalkInOrder(func(_, _ int, _ *TestTreeNode, _ interface{}) bool { return true }, nil)
	})
	expectPanic(t, "WalkPreOrder", func() {
		nilTree.WalkPreOrder(func(_, _ int, _ *TestTreeNode, _ interface{}) bool { return true }, nil)
	})
	expectPanic(t, "WalkPostOrder", func() {
		nilTree.WalkPostOrder(func(_, _ int, _ *TestTreeNode, _ interface{}) bool { return true }, nil)
	})
	expectPanic(t, "Copy", func() { nilTree.Copy(&AvlTree[TestTreeNode]{}) })
	expectPanic(t, "Union", func() { nilTree.Union(&AvlTree[TestTreeNode]{}, &AvlTree[TestTreeNode]{}) })
	expectPanic(t, "Minus", func() { nilTree.Minus(&AvlTree[TestTreeNode]{}, &AvlTree[TestTreeNode]{}) })
	expectPanic(t, "Intersect", func() { nilTree.Intersect(&AvlTree[TestTreeNode]{}, &AvlTree[TestTreeNode]{}) })
}

// TestTreeNodeAccessors covers NewAvlTreeElement, GetData, Height and the
// nil-node branches of Height and calcAvlBalance.
func TestTreeNodeAccessors(t *testing.T) {
	key := mkKey(7)
	node := NewAvlTreeElement[TestTreeNode](key)
	if node.GetData() != key {
		t.Errorf("GetData: expected the original pointer back")
	}
	if node.height != 1 {
		t.Errorf("NewAvlTreeElement: expected height 1, got %d", node.height)
	}

	var tree AvlTree[TestTreeNode]
	if got := tree.Height(node); got != 1 {
		t.Errorf("Height(node): expected 1, got %d", got)
	}
	if got := tree.Height(nil); got != 0 {
		t.Errorf("Height(nil): expected 0, got %d", got)
	}
	if got := tree.calcAvlBalance(nil); got != 0 {
		t.Errorf("calcAvlBalance(nil): expected 0, got %d", got)
	}
	if got := tree.calcAvlBalance(node); got != 0 {
		t.Errorf("calcAvlBalance(leaf): expected 0, got %d", got)
	}
}

// errWriter always fails, to exercise the write-error branch of Dump.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("forced write error") }

// TestTreeDumpOutput verifies Dump prints every key and stops on write error.
func TestTreeDumpOutput(t *testing.T) {
	var tree AvlTree[TestTreeNode]

	// Dump of an empty tree produces no output but must not panic.
	var buf bytes.Buffer
	tree.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Dump of empty tree: expected no output, got %q", buf.String())
	}

	keys := []int{5, 2, 9, 0, 3, 7}
	for _, v := range keys {
		tree.Insert(mkKey(v))
	}
	buf.Reset()
	tree.Dump(&buf)
	out := buf.String()
	for _, v := range keys {
		if !strings.Contains(out, mkKey(v).S) {
			t.Errorf("Dump: output missing key %s\n%s", mkKey(v).S, out)
		}
	}
	if lines := strings.Count(out, "\n"); lines != len(keys) {
		t.Errorf("Dump: expected %d lines, got %d", len(keys), lines)
	}

	// A failing writer must not panic; the dump just stops.
	tree.Dump(errWriter{})
}

// TestTreeDuplicateReplaces verifies that inserting a duplicate key replaces
// the stored item rather than adding a second node.
func TestTreeDuplicateReplaces(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	before := tree.Length()

	p1 := &TestTreeNode{S: "zz-first"}
	tree.Insert(p1)
	if tree.Search(&TestTreeNode{S: "zz-first"}) != p1 {
		t.Fatalf("Expected the inserted pointer to be stored")
	}

	// Insert a different pointer with an equal key: it must replace p1.
	p2 := &TestTreeNode{S: "zz-first"}
	tree.Insert(p2)
	got := tree.Search(&TestTreeNode{S: "zz-first"})
	if got != p2 {
		t.Errorf("Duplicate insert: expected the new item to replace the old one")
	}
	if tree.Length() != before+1 {
		t.Errorf("Duplicate insert: expected length %d, got %d", before+1, tree.Length())
	}
	validateAVLNode(t, tree.root)

	// Duplicates of the root and of an interior node must also keep length.
	tree.Insert(mkKey(5))
	tree.Insert(mkKey(0))
	tree.Insert(mkKey(9))
	if tree.Length() != before+1 {
		t.Errorf("Duplicate insert of existing keys: expected length %d, got %d", before+1, tree.Length())
	}
	validateAVLNode(t, tree.root)
}

// TestTreeWalkEarlyStop verifies that returning false from the walk callback
// stops each of the three walks immediately.
func TestTreeWalkEarlyStop(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	for _, v := range []int{5, 2, 9, 0, 3, 7, 1} {
		tree.Insert(mkKey(v))
	}

	stopAfter := func(name string, walk func(ApplyFunction[TestTreeNode], interface{})) {
		calls := 0
		walk(func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
			calls++
			if pos != calls-1 {
				t.Errorf("%s: pos = %d on call %d, expected %d", name, pos, calls, calls-1)
			}
			return calls < 2 // stop after the 2nd call
		}, nil)
		if calls != 2 {
			t.Errorf("%s: expected walk to stop after 2 calls, got %d", name, calls)
		}
	}
	stopAfter("WalkInOrder", tree.WalkInOrder)
	stopAfter("WalkPreOrder", tree.WalkPreOrder)
	stopAfter("WalkPostOrder", tree.WalkPostOrder)

	// userData must be passed through to the callback untouched.
	token := &struct{ n int }{n: 42}
	tree.WalkInOrder(func(_, _ int, _ *TestTreeNode, userData interface{}) bool {
		if userData != token {
			t.Errorf("WalkInOrder: userData not passed through")
		}
		return false
	}, token)
}

// TestTreeIteratorEdges covers the edge branches of the old-style iterator
// and the range-over-func iterators, including snapshot isolation.
func TestTreeIteratorEdges(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	for _, v := range []int{5, 2, 9} {
		tree.Insert(mkKey(v))
	}

	// Value returns nil once the iterator is done, and Next past the end is
	// a no-op.
	it := tree.Front()
	for !it.Done() {
		it.Next()
	}
	if it.Value() != nil {
		t.Errorf("Value after Done: expected nil, got %v", it.Value())
	}
	it.Next()
	it.Next()
	if !it.Done() {
		t.Errorf("Done: expected iterator to stay done after extra Next calls")
	}

	// Backward early break stops iteration.
	count := 0
	for range tree.Backward() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Backward early break: expected 1 iteration, got %d", count)
	}

	// Front takes a snapshot: mutations after Front must not affect the
	// iteration in progress.
	it = tree.Front()
	tree.Insert(mkKey(0))
	tree.Delete(mkKey(9))
	var got []string
	for ; !it.Done(); it.Next() {
		got = append(got, it.Value().S)
	}
	expect := []string{mkKey(2).S, mkKey(5).S, mkKey(9).S}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
		t.Errorf("Front snapshot: expected %v got %v", expect, got)
	}
}

// TestTreeIndexBounds covers out-of-range positions and every in-range one.
func TestTreeIndexBounds(t *testing.T) {
	var tree AvlTree[TestTreeNode]
	keys := []int{50, 20, 80, 10, 30, 60, 90}
	for _, v := range keys {
		tree.Insert(mkKey(v))
	}
	sort.Ints(keys)

	if tree.Index(-1) != nil {
		t.Errorf("Index(-1): expected nil")
	}
	if tree.Index(len(keys)) != nil {
		t.Errorf("Index(length): expected nil")
	}
	if tree.Index(1000) != nil {
		t.Errorf("Index(1000): expected nil")
	}
	for i, v := range keys {
		item := tree.Index(i)
		if item == nil || item.S != mkKey(v).S {
			t.Errorf("Index(%d): expected %s got %v", i, mkKey(v).S, item)
		}
	}
}

// TestTreeDeleteAtHeadTailDrain drains a tree from both ends and checks the
// removal order, the found flags and the final empty state.
func TestTreeDeleteAtHeadTailDrain(t *testing.T) {
	rng := rand.New(rand.NewPCG(1234, 99))
	const n = 200

	var tree AvlTree[TestTreeNode]
	for _, v := range rng.Perm(n) {
		tree.Insert(mkKey(v))
	}

	// Drain from the head: keys must come out in ascending order.
	prev := -1
	for i := 0; i < n; i++ {
		mn := tree.FindMin()
		if mn == nil {
			t.Fatalf("FindMin returned nil with %d elements left", n-i)
		}
		var v int
		if _, err := fmt.Sscanf(mn.S, "%d", &v); err != nil {
			t.Fatalf("bad key %q: %v", mn.S, err)
		}
		if v <= prev {
			t.Fatalf("DeleteAtHead order: %d after %d, not ascending", v, prev)
		}
		prev = v
		if !tree.DeleteAtHead() {
			t.Fatalf("DeleteAtHead returned false with %d elements left", n-i)
		}
	}
	if !tree.IsEmpty() || tree.DeleteAtHead() {
		t.Errorf("Expected empty tree and false from DeleteAtHead after drain")
	}

	// Drain from the tail: keys must come out in descending order.
	for _, v := range rng.Perm(n) {
		tree.Insert(mkKey(v))
	}
	prev = n
	for i := 0; i < n; i++ {
		mx := tree.FindMax()
		if mx == nil {
			t.Fatalf("FindMax returned nil with %d elements left", n-i)
		}
		var v int
		if _, err := fmt.Sscanf(mx.S, "%d", &v); err != nil {
			t.Fatalf("bad key %q: %v", mx.S, err)
		}
		if v >= prev {
			t.Fatalf("DeleteAtTail order: %d after %d, not descending", v, prev)
		}
		prev = v
		if !tree.DeleteAtTail() {
			t.Fatalf("DeleteAtTail returned false with %d elements left", n-i)
		}
	}
	if !tree.IsEmpty() || tree.DeleteAtTail() {
		t.Errorf("Expected empty tree and false from DeleteAtTail after drain")
	}
	validateAVLNode(t, tree.root)
}

// TestTreeRandomMixedModel is a fixed-seed property test that performs
// hundreds of mixed operations and cross-checks the tree against a simple
// sorted-slice reference model after every step.
func TestTreeRandomMixedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(2024, 5))
	const ops = 800
	const keySpace = 120 // small enough to force frequent duplicates

	var tree AvlTree[TestTreeNode]
	model := make(map[int]bool)

	sortedModel := func() (rv []int) {
		for k := range model {
			rv = append(rv, k)
		}
		sort.Ints(rv)
		return
	}

	checkModel := func(op string) {
		t.Helper()
		if got := tree.Length(); got != len(model) {
			t.Fatalf("after %s: length = %d, model has %d", op, got, len(model))
		}
		if got := tree.IsEmpty(); got != (len(model) == 0) {
			t.Fatalf("after %s: IsEmpty = %v, model empty = %v", op, got, len(model) == 0)
		}
		sorted := sortedModel()
		// Full in-order iteration must match the sorted model exactly.
		i := 0
		for item := range tree.All() {
			if i >= len(sorted) {
				t.Fatalf("after %s: All() yielded more than %d items", op, len(sorted))
			}
			if item.S != mkKey(sorted[i]).S {
				t.Fatalf("after %s: All()[%d] = %s, model has %s", op, i, item.S, mkKey(sorted[i]).S)
			}
			i++
		}
		if i != len(sorted) {
			t.Fatalf("after %s: All() yielded %d items, model has %d", op, i, len(sorted))
		}
		// Index must agree with the sorted model at every position.
		for j, v := range sorted {
			if item := tree.Index(j); item == nil || item.S != mkKey(v).S {
				t.Fatalf("after %s: Index(%d) = %v, model has %s", op, j, item, mkKey(v).S)
			}
		}
		// Min/max and depth bound.
		if len(sorted) == 0 {
			if tree.FindMin() != nil || tree.FindMax() != nil {
				t.Fatalf("after %s: FindMin/FindMax non-nil on empty tree", op)
			}
		} else {
			if mn := tree.FindMin(); mn == nil || mn.S != mkKey(sorted[0]).S {
				t.Fatalf("after %s: FindMin = %v, model has %s", op, mn, mkKey(sorted[0]).S)
			}
			if mx := tree.FindMax(); mx == nil || mx.S != mkKey(sorted[len(sorted)-1]).S {
				t.Fatalf("after %s: FindMax = %v, model has %s", op, mx, mkKey(sorted[len(sorted)-1]).S)
			}
		}
		// AVL guarantee: depth is O(log n); 2*log2(n+2) is a safe bound.
		if d, n := tree.Depth(), len(sorted); n > 0 && d > 2*log2ceil(n+2) {
			t.Fatalf("after %s: depth %d too large for %d nodes", op, d, n)
		}
		validateAVLNode(t, tree.root)
	}

	for step := 0; step < ops; step++ {
		k := rng.IntN(keySpace)
		switch rng.IntN(8) {
		case 0, 1, 2: // insert (possibly a duplicate -> replace)
			tree.Insert(mkKey(k))
			model[k] = true
		case 3, 4: // delete
			got := tree.Delete(mkKey(k))
			if got != model[k] {
				t.Fatalf("step %d: Delete(%d) = %v, model says %v", step, k, got, model[k])
			}
			delete(model, k)
		case 5: // search
			got := tree.Search(mkKey(k))
			if (got != nil) != model[k] {
				t.Fatalf("step %d: Search(%d) found=%v, model says %v", step, k, got != nil, model[k])
			}
		case 6: // delete min
			sorted := sortedModel()
			got := tree.DeleteAtHead()
			if got != (len(sorted) > 0) {
				t.Fatalf("step %d: DeleteAtHead = %v, model empty = %v", step, got, len(sorted) == 0)
			}
			if got {
				delete(model, sorted[0])
			}
		case 7: // delete max
			sorted := sortedModel()
			got := tree.DeleteAtTail()
			if got != (len(sorted) > 0) {
				t.Fatalf("step %d: DeleteAtTail = %v, model empty = %v", step, got, len(sorted) == 0)
			}
			if got {
				delete(model, sorted[len(sorted)-1])
			}
		}
		if step%50 == 0 {
			checkModel(fmt.Sprintf("step %d", step))
		}
	}
	checkModel("final")
}

func log2ceil(n int) int {
	r := 0
	for (1 << r) < n {
		r++
	}
	return r
}

// TestTreeGoroutineMixed runs concurrent inserters, deleters, readers and
// iterators against one shared tree.  It is meant to be run with the race
// detector (make race).  Correctness is checked loosely — the point is to
// detect data races, deadlocks and panics, not exact interleavings.
func TestTreeGoroutineMixed(t *testing.T) {
	tree := NewAvlTree[TestTreeNode]()

	// Seed the tree so readers/deleters have something to work on.
	for i := 0; i < 100; i++ {
		tree.Insert(mkKey(i))
	}

	var wg sync.WaitGroup

	// Writers: Insert (with duplicate keys across goroutines).
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				tree.Insert(mkKey(base + i%173))
			}
		}(g * 1000)
	}

	// Deleters: Delete and DeleteAtHead/DeleteAtTail (false on missing is fine).
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				tree.Delete(mkKey(base + i%211))
				tree.DeleteAtHead()
				tree.DeleteAtTail()
			}
		}(g * 500)
	}

	// Readers: Search / FindMin / FindMax / Length / Depth / Index / Dump.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				tree.Search(mkKey(i % 150))
				tree.FindMin()
				tree.FindMax()
				tree.Length()
				tree.IsEmpty()
				tree.Depth()
				tree.Index(i % 10)
				tree.Dump(io.Discard)
			}
		}()
	}

	// Iterators: snapshot iteration over All, Backward and Front while other
	// goroutines mutate the tree.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				for range tree.All() {
					break
				}
				for range tree.Backward() {
					break
				}
				for it := tree.Front(); !it.Done(); it.Next() {
				}
			}
		}()
	}

	// Walks run under the read lock and must not call back into the tree.
	wg.Add(1)
	go func() {
		defer wg.Done()
		fx := func(_, _ int, _ *TestTreeNode, _ interface{}) bool { return true }
		for i := 0; i < 50; i++ {
			tree.WalkInOrder(fx, nil)
			tree.WalkPreOrder(fx, nil)
			tree.WalkPostOrder(fx, nil)
		}
	}()

	wg.Wait()

	// Whatever survived must still be a valid AVL tree that agrees with
	// Length.
	n := tree.Length()
	count := 0
	for range tree.All() {
		count++
	}
	if count != n {
		t.Errorf("After concurrent run: Length = %d but All() visited %d", n, count)
	}
	validateAVLNode(t, tree.root)
}

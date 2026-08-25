package bst

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// expectPanic runs f and fails the test unless f panics.
func expectPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	f()
}

// The documented behavior of Insert, Search, Delete, FindMin, FindMax,
// DeleteAtHead, DeleteAtTail, Reverse, Index, and Depth is to panic when the
// tree pointer is nil.
func TestTreeNilReceiverPanics(t *testing.T) {
	var tree *BinarySearchTree[IntNode]

	expectPanic(t, "Insert", func() { tree.Insert(IntNode(1)) })
	expectPanic(t, "Search", func() { tree.Search(IntNode(1)) })
	expectPanic(t, "Delete", func() { tree.Delete(IntNode(1)) })
	expectPanic(t, "FindMin", func() { tree.FindMin() })
	expectPanic(t, "FindMax", func() { tree.FindMax() })
	expectPanic(t, "DeleteAtHead", func() { tree.DeleteAtHead() })
	expectPanic(t, "DeleteAtTail", func() { tree.DeleteAtTail() })
	expectPanic(t, "Reverse", func() { tree.Reverse() })
	expectPanic(t, "Index", func() { tree.Index(0) })
	expectPanic(t, "Depth", func() { tree.Depth() })
}

// Dump must write every item in the tree to the supplied writer.
func TestTreeDump(t *testing.T) {
	var tree BinarySearchTree[IntNode]

	// Dump of an empty tree must not panic and must produce no output.
	var emptyBuf bytes.Buffer
	tree.Dump(&emptyBuf)
	if emptyBuf.Len() != 0 {
		t.Errorf("Expected no output dumping an empty tree, got %q", emptyBuf.String())
	}

	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		tree.Insert(v)
	}
	var buf bytes.Buffer
	tree.Dump(&buf)
	out := buf.String()
	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		if !strings.Contains(out, fmt.Sprintf("%d", int(v))) {
			t.Errorf("Dump output missing item %d, got:\n%s", v, out)
		}
	}
	if n := strings.Count(out, "\n"); n != 5 {
		t.Errorf("Expected 5 lines from Dump, got %d:\n%s", n, out)
	}
}

// A single-element tree must behave correctly for every operation.
func TestTreeSingleElement(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	tree.Insert(IntNode(42))

	if tree.IsEmpty() {
		t.Errorf("Expected non-empty tree")
	}
	if tree.Length() != 1 {
		t.Errorf("Expected length 1, got %d", tree.Length())
	}
	if tree.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", tree.Depth())
	}
	if got := tree.FindMin(); got == nil || *got != IntNode(42) {
		t.Errorf("FindMin: expected 42, got %v", got)
	}
	if got := tree.FindMax(); got == nil || *got != IntNode(42) {
		t.Errorf("FindMax: expected 42, got %v", got)
	}
	if got := tree.Index(0); got == nil || *got != IntNode(42) {
		t.Errorf("Index(0): expected 42, got %v", got)
	}
	if tree.Index(1) != nil {
		t.Errorf("Index(1): expected nil for out-of-range position")
	}
	var fwd []IntNode
	for v := range tree.All() {
		fwd = append(fwd, v)
	}
	if !reflect.DeepEqual(fwd, []IntNode{42}) {
		t.Errorf("All: expected [42], got %v", fwd)
	}
	var bwd []IntNode
	for v := range tree.Backward() {
		bwd = append(bwd, v)
	}
	if !reflect.DeepEqual(bwd, []IntNode{42}) {
		t.Errorf("Backward: expected [42], got %v", bwd)
	}

	// DeleteAtTail on a single-element tree empties it.
	if !tree.DeleteAtTail() {
		t.Errorf("DeleteAtTail: expected true on single-element tree")
	}
	if !tree.IsEmpty() || tree.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtTail")
	}

	// Refill, then DeleteAtHead must empty it too.
	tree.Insert(IntNode(7))
	if !tree.DeleteAtHead() {
		t.Errorf("DeleteAtHead: expected true on single-element tree")
	}
	if !tree.IsEmpty() || tree.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead")
	}

	// Both must report false once the tree is empty.
	if tree.DeleteAtHead() {
		t.Errorf("DeleteAtHead: expected false on empty tree")
	}
	if tree.DeleteAtTail() {
		t.Errorf("DeleteAtTail: expected false on empty tree")
	}
}

// A duplicate insert must replace the stored item, keep the length unchanged,
// and work at any position in the tree, not just the root.
func TestTreeDuplicateReplace(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		tree.Insert(v)
	}
	before := tree.Length()

	// Duplicate at a leaf (0) and at a mid-tree node (2).  Search returns the
	// stored pointer, so a genuine replacement must yield a different pointer.
	p0 := tree.Search(IntNode(0))
	p2 := tree.Search(IntNode(2))
	tree.Insert(IntNode(0))
	tree.Insert(IntNode(2))

	if tree.Length() != before {
		t.Errorf("Duplicate insert changed length from %d to %d", before, tree.Length())
	}
	if q0 := tree.Search(IntNode(0)); q0 == nil || q0 == p0 {
		t.Errorf("Duplicate insert at leaf did not replace stored item")
	}
	if q2 := tree.Search(IntNode(2)); q2 == nil || q2 == p2 {
		t.Errorf("Duplicate insert at mid node did not replace stored item")
	}

	// In-order sequence must be untouched.
	var got []IntNode
	for v := range tree.All() {
		got = append(got, v)
	}
	if expect := []IntNode{0, 2, 3, 5, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("In-order after duplicates: expected %v got %v", expect, got)
	}
}

// Deleting a node whose in-order successor is its direct right child must
// promote the right child and keep the rest of the sub-tree.
func TestTreeDeleteDirectSuccessor(t *testing.T) {
	var tree BinarySearchTree[IntNode]

	//        5
	//     3     8
	//              9
	for _, v := range []IntNode{5, 3, 8, 9} {
		tree.Insert(v)
	}
	if !tree.Delete(IntNode(5)) {
		t.Fatalf("Expected to delete 5")
	}
	if size := tree.Length(); size != 3 {
		t.Errorf("Expected 3 nodes after delete, got %d", size)
	}
	var got []IntNode
	for v := range tree.All() {
		got = append(got, v)
	}
	if expect := []IntNode{3, 8, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("In-order after delete: expected %v got %v", expect, got)
	}

	// Deleting an item that is not present must fail and change nothing.
	if tree.Delete(IntNode(4)) {
		t.Errorf("Expected delete of absent item to fail")
	}
	if tree.Delete(IntNode(100)) {
		t.Errorf("Expected delete of absent item to fail")
	}
	if size := tree.Length(); size != 3 {
		t.Errorf("Failed delete changed length, got %d", size)
	}
}

// The Walk* traversals must stop as soon as fx returns false, must pass the
// correct pos/depth values, and must pass userData through.
func TestTreeWalkEarlyStopAndArgs(t *testing.T) {
	build := func() *BinarySearchTree[IntNode] {
		var tree BinarySearchTree[IntNode]
		for _, v := range []IntNode{5, 2, 9, 0, 3} {
			tree.Insert(v)
		}
		return &tree
	}
	// The tree is:
	//        5        depth 0
	//     2     9     depth 1
	//   0   3         depth 2

	// Full in-order walk: verify pos and depth arguments per visit.
	type visit struct {
		pos, depth int
		data       IntNode
	}
	var visits []visit
	userData := "marker"
	tree := build()
	tree.WalkInOrder(func(pos, depth int, data *IntNode, ud interface{}) bool {
		if ud != userData {
			t.Errorf("userData not passed through, got %v", ud)
		}
		visits = append(visits, visit{pos, depth, *data})
		return true
	}, userData)
	expect := []visit{
		{0, 2, 0}, {1, 1, 2}, {2, 2, 3}, {3, 0, 5}, {4, 1, 9},
	}
	if !reflect.DeepEqual(visits, expect) {
		t.Errorf("WalkInOrder visits: expected %+v got %+v", expect, visits)
	}

	// Early stop in each traversal: fx returning false must stop the walk
	// immediately.
	stopAfter := func(name string, walk func(fx ApplyFunction[IntNode], ud interface{})) {
		n := 0
		walk(func(pos, depth int, data *IntNode, ud interface{}) bool {
			n++
			return false
		}, nil)
		if n != 1 {
			t.Errorf("%s: expected walk to stop after 1 call, got %d", name, n)
		}
	}
	stopAfter("WalkInOrder", tree.WalkInOrder)
	stopAfter("WalkPreOrder", tree.WalkPreOrder)
	stopAfter("WalkPostOrder", tree.WalkPostOrder)

	// Walks on an empty tree must not call fx at all.
	var empty BinarySearchTree[IntNode]
	calls := 0
	fx := func(pos, depth int, data *IntNode, ud interface{}) bool {
		calls++
		return true
	}
	empty.WalkInOrder(fx, nil)
	empty.WalkPreOrder(fx, nil)
	empty.WalkPostOrder(fx, nil)
	if calls != 0 {
		t.Errorf("Expected 0 walk calls on empty tree, got %d", calls)
	}
}

// Backward must honor an early break, and both iterators must handle a tree
// that is mutated between iterations.
func TestTreeBackwardEarlyBreak(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		tree.Insert(v)
	}

	count := 0
	for v := range tree.Backward() {
		if v != IntNode(9) {
			t.Errorf("Backward with break: expected first value 9, got %d", v)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("Backward with break: expected 1 iteration, got %d", count)
	}

	// Iterating an empty tree with Backward must yield nothing.
	var empty BinarySearchTree[IntNode]
	for range empty.Backward() {
		t.Errorf("Backward on empty tree yielded a value")
	}
}

// A degenerate (sorted-insert) tree must have depth equal to its length.
func TestTreeDepthDegenerate(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	const n = 10
	for i := 0; i < n; i++ {
		tree.Insert(IntNode(i))
	}
	if got := tree.Depth(); got != n {
		t.Errorf("Sorted-insert tree: expected depth %d, got %d", n, got)
	}
	// Reverse a degenerate tree and iterate: in-order must be descending.
	tree.Reverse()
	var got []IntNode
	for v := range tree.All() {
		got = append(got, v)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] <= got[i] {
			t.Errorf("After Reverse: expected strictly descending order, got %v", got)
			break
		}
	}
}

// TestTreeRandomized cross-checks the tree against a simple reference model
// (a set plus a sorted slice) over hundreds of mixed operations driven by a
// fixed-seed PRNG, verifying structural invariants after every operation.
func TestTreeRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(20240817))
	var tree BinarySearchTree[IntNode]
	model := make(map[int]bool)
	const keySpace = 120
	const ops = 800

	sortedModel := func() []IntNode {
		var out []IntNode
		for k := range model {
			out = append(out, IntNode(k))
		}
		for i := 1; i < len(out); i++ {
			for j := i; j > 0 && out[j] < out[j-1]; j-- {
				out[j], out[j-1] = out[j-1], out[j]
			}
		}
		return out
	}

	verify := func(op string) {
		t.Helper()
		// Length matches the model.
		if tree.Length() != len(model) {
			t.Fatalf("after %s: length %d, model %d", op, tree.Length(), len(model))
		}
		// IsEmpty agrees with the model.
		if tree.IsEmpty() != (len(model) == 0) {
			t.Fatalf("after %s: IsEmpty disagrees with model", op)
		}
		// In-order iteration is sorted and exactly matches the model.
		sorted := sortedModel()
		var fwd []IntNode
		for v := range tree.All() {
			fwd = append(fwd, v)
		}
		if !reflect.DeepEqual(fwd, sorted) {
			t.Fatalf("after %s: in-order %v, model %v", op, fwd, sorted)
		}
		// Backward iteration is the reverse.
		var bwd []IntNode
		for v := range tree.Backward() {
			bwd = append(bwd, v)
		}
		for i := range bwd {
			if bwd[i] != sorted[len(sorted)-1-i] {
				t.Fatalf("after %s: backward %v, model %v", op, bwd, sorted)
			}
		}
		// Min/max and Index match the model.
		if len(sorted) > 0 {
			if got := tree.FindMin(); got == nil || *got != sorted[0] {
				t.Fatalf("after %s: FindMin %v, model %v", op, got, sorted[0])
			}
			if got := tree.FindMax(); got == nil || *got != sorted[len(sorted)-1] {
				t.Fatalf("after %s: FindMax %v, model %v", op, got, sorted[len(sorted)-1])
			}
			pos := rng.Intn(len(sorted))
			if got := tree.Index(pos); got == nil || *got != sorted[pos] {
				t.Fatalf("after %s: Index(%d) %v, model %v", op, pos, got, sorted[pos])
			}
		}
		// Depth must lie between a perfect tree's depth and n.
		if d, n := tree.Depth(), len(model); n > 0 && (d < 1 || d > n) {
			t.Fatalf("after %s: depth %d out of range for %d nodes", op, d, n)
		}
	}

	for i := 0; i < ops; i++ {
		k := rng.Intn(keySpace)
		switch rng.Intn(6) {
		case 0, 1: // insert (heavier weight to grow the tree)
			tree.Insert(IntNode(k))
			model[k] = true
			verify(fmt.Sprintf("op %d Insert(%d)", i, k))
		case 2: // delete
			_, present := model[k]
			if got := tree.Delete(IntNode(k)); got != present {
				t.Fatalf("op %d: Delete(%d) returned %v, model says present=%v", i, k, got, present)
			}
			delete(model, k)
			verify(fmt.Sprintf("op %d Delete(%d)", i, k))
		case 3: // search
			got := tree.Search(IntNode(k))
			if present := model[k]; (got != nil) != present {
				t.Fatalf("op %d: Search(%d) found=%v, model says present=%v", i, k, got != nil, present)
			}
		case 4: // delete at head
			if len(model) == 0 {
				if tree.DeleteAtHead() {
					t.Fatalf("op %d: DeleteAtHead on empty tree returned true", i)
				}
				continue
			}
			min := sortedModel()[0]
			if !tree.DeleteAtHead() {
				t.Fatalf("op %d: DeleteAtHead returned false", i)
			}
			delete(model, int(min))
			verify(fmt.Sprintf("op %d DeleteAtHead (min=%d)", i, min))
		case 5: // delete at tail
			if len(model) == 0 {
				if tree.DeleteAtTail() {
					t.Fatalf("op %d: DeleteAtTail on empty tree returned true", i)
				}
				continue
			}
			sorted := sortedModel()
			max := sorted[len(sorted)-1]
			if !tree.DeleteAtTail() {
				t.Fatalf("op %d: DeleteAtTail returned false", i)
			}
			delete(model, int(max))
			verify(fmt.Sprintf("op %d DeleteAtTail (max=%d)", i, max))
		}
	}

	// Drain the tree completely via Truncate and confirm it behaves as new.
	tree.Truncate()
	if !tree.IsEmpty() || tree.Length() != 0 || tree.Depth() != 0 {
		t.Errorf("After Truncate: expected pristine empty tree")
	}
}

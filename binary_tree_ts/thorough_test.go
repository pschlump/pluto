package binary_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------------------------

// inOrder returns the in-order (ascending) values of the tree.
func inOrder(tt *BinaryTree[TestTreeNode]) []string {
	var got []string
	for v := range tt.All() {
		got = append(got, v.S)
	}
	return got
}

// expectPanics runs fn and fails the test unless it panics with a message
// containing wantSub.
func expectPanics(t *testing.T, name string, wantSub string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected panic, got none", name)
			return
		}
		msg := fmt.Sprintf("%v", r)
		if !strings.Contains(msg, wantSub) {
			t.Errorf("%s: expected panic containing %q, got %q", name, wantSub, msg)
		}
	}()
	fn()
}

// buildTree inserts the given keys into a fresh zero-value tree.
func buildTree(keys ...string) *BinaryTree[TestTreeNode] {
	var tree BinaryTree[TestTreeNode]
	for _, k := range keys {
		tree.Insert(&TestTreeNode{S: k})
	}
	return &tree
}

// -------------------------------------------------------------------------------------------------------
// Nil-tree panics (documented behavior)
// -------------------------------------------------------------------------------------------------------

// TestNilTreePanics verifies the documented panics when operations are
// called on a nil tree.
func TestNilTreePanics(t *testing.T) {
	var nilTree *BinaryTree[TestTreeNode]
	key := &TestTreeNode{S: "05"}
	cmp := func(a, b *TestTreeNode) int { return a.Compare(*b) }

	expectPanics(t, "Insert", "Insert called on a nil tree", func() { nilTree.Insert(key) })
	expectPanics(t, "Search", "Search called on a nil tree", func() { nilTree.Search(key) })
	expectPanics(t, "Delete", "Delete called on a nil tree", func() { nilTree.Delete(key) })
	expectPanics(t, "DeleteMatch", "DeleteMatch called on a nil tree", func() { nilTree.DeleteMatch(key, cmp) })
	expectPanics(t, "FindMin", "FindMin called on a nil tree", func() { nilTree.FindMin() })
	expectPanics(t, "FindMax", "FindMax called on a nil tree", func() { nilTree.FindMax() })
	expectPanics(t, "DeleteAtHead", "DeleteAtHead called on a nil tree", func() { nilTree.DeleteAtHead() })
	expectPanics(t, "DeleteAtTail", "DeleteAtTail called on a nil tree", func() { nilTree.DeleteAtTail() })
	expectPanics(t, "Reverse", "Reverse called on a nil tree", func() { nilTree.Reverse() })
	expectPanics(t, "Index", "Index called on a nil tree", func() { nilTree.Index(0) })
	expectPanics(t, "Depth", "Depth called on a nil tree", func() { nilTree.Depth() })
	expectPanics(t, "WalkFunc", "WalkFunc called on a nil tree", func() { nilTree.WalkFunc(func(a *TestTreeNode) {}) })
}

// -------------------------------------------------------------------------------------------------------
// Dump
// -------------------------------------------------------------------------------------------------------

func TestTreeDump(t *testing.T) {
	tree := buildTree("05", "02", "09", "00", "03")

	var buf bytes.Buffer
	tree.Dump(&buf)
	out := buf.String()

	for _, k := range []string{"05", "02", "09", "00", "03"} {
		if !strings.Contains(out, k) {
			t.Errorf("Dump output missing key %q", k)
		}
	}
	// One line per node.
	if n := strings.Count(out, "\n"); n != 5 {
		t.Errorf("Expected Dump to print 5 lines, got %d", n)
	}

	// Dump of an empty tree prints nothing and does not panic.
	var empty BinaryTree[TestTreeNode]
	buf.Reset()
	empty.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected empty Dump output, got %q", buf.String())
	}
}

// failAfterWriter accepts n writes and then returns an error.
type failAfterWriter struct {
	n   int
	err error
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, w.err
	}
	w.n--
	return len(p), nil
}

// TestTreeDumpWriteError verifies that a failing writer stops the dump
// early without panicking, at every possible failure point (in the left
// subtree, at the node itself, and in the right subtree).
func TestTreeDumpWriteError(t *testing.T) {
	tree := buildTree("05", "02", "09", "00")

	for failAfter := 0; failAfter <= 4; failAfter++ {
		w := &failAfterWriter{n: failAfter, err: errors.New("boom")}
		tree.Dump(w) // must not panic
	}
}

// -------------------------------------------------------------------------------------------------------
// Single-element and duplicate edge cases
// -------------------------------------------------------------------------------------------------------

func TestTreeSingleElement(t *testing.T) {
	var tree BinaryTree[TestTreeNode]

	if !tree.Insert(&TestTreeNode{S: "05"}) {
		t.Fatalf("Expected first Insert to report a new node.")
	}
	if tree.Len() != 1 || tree.Length() != 1 {
		t.Errorf("Expected length 1, got %d/%d", tree.Len(), tree.Length())
	}
	if x := tree.FindMin(); x == nil || x.S != "05" {
		t.Errorf("Expected min 05, got %+v", x)
	}
	if x := tree.FindMax(); x == nil || x.S != "05" {
		t.Errorf("Expected max 05, got %+v", x)
	}
	if x := tree.Index(0); x == nil || x.S != "05" {
		t.Errorf("Expected Index(0) 05, got %+v", x)
	}
	if d := tree.Depth(); d != 1 {
		t.Errorf("Expected depth 1, got %d", d)
	}
	if got := inOrder(&tree); !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05], got %v", got)
	}

	// DeleteAtHead on a single element empties the tree.
	if !tree.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to succeed on single-element tree.")
	}
	if !tree.IsEmpty() || tree.Len() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead, len=%d", tree.Len())
	}

	// DeleteAtTail on a single element empties the tree.
	tree.Insert(&TestTreeNode{S: "05"})
	if !tree.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to succeed on single-element tree.")
	}
	if !tree.IsEmpty() || tree.Len() != 0 {
		t.Errorf("Expected empty tree after DeleteAtTail, len=%d", tree.Len())
	}

	// Reverse on a single-element tree is a no-op.
	tree.Insert(&TestTreeNode{S: "05"})
	tree.Reverse()
	if got := inOrder(&tree); !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05] after Reverse, got %v", got)
	}
}

func TestTreeDuplicateReplace(t *testing.T) {
	var tree BinaryTree[TestTreeNode]

	if !tree.Insert(&TestTreeNode{S: "05"}) {
		t.Fatalf("Expected first Insert to be new.")
	}
	tree.Insert(&TestTreeNode{S: "03"})
	tree.Insert(&TestTreeNode{S: "08"})

	// Replacing the root (which has two children) must keep its subtrees.
	repl := &TestTreeNode{S: "05"}
	if tree.Insert(repl) {
		t.Errorf("Expected duplicate Insert to report replacement (false).")
	}
	if tree.Len() != 3 {
		t.Errorf("Expected length 3 after duplicate insert, got %d", tree.Len())
	}
	// The stored node must be the new value.
	if got := tree.Search(&TestTreeNode{S: "05"}); got != repl {
		t.Errorf("Expected Search to return the replacement node.")
	}
	// Both children must still be reachable.
	if tree.Search(&TestTreeNode{S: "03"}) == nil || tree.Search(&TestTreeNode{S: "08"}) == nil {
		t.Errorf("Children lost after duplicate root insert.")
	}
	if got := inOrder(&tree); !reflect.DeepEqual(got, []string{"03", "05", "08"}) {
		t.Errorf("Expected in-order [03 05 08], got %v", got)
	}

	// Replacing a leaf.
	if tree.Insert(&TestTreeNode{S: "03"}) {
		t.Errorf("Expected duplicate leaf Insert to report replacement (false).")
	}
	if tree.Len() != 3 {
		t.Errorf("Expected length 3 after duplicate leaf insert, got %d", tree.Len())
	}
}

// -------------------------------------------------------------------------------------------------------
// Iterators over a snapshot
// -------------------------------------------------------------------------------------------------------

// TestTreeIteratorSnapshot verifies that All/Backward/Front operate on a
// snapshot: mutating the tree during iteration does not change what the
// iterator yields, and does not deadlock.
func TestTreeIteratorSnapshot(t *testing.T) {
	tree := buildTree("05", "02", "09", "00", "03")

	var got []string
	for v := range tree.All() {
		got = append(got, v.S)
		// Mutate the tree while iterating: must be safe and invisible.
		tree.Delete(&TestTreeNode{S: "00"})
		tree.Insert(&TestTreeNode{S: "77"})
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Snapshot iteration error, expected %v got %v", expect, got)
	}

	// The mutations did happen to the tree itself.
	if tree.Search(&TestTreeNode{S: "77"}) == nil {
		t.Errorf("Expected 77 to be in the tree after mutation.")
	}
	if tree.Search(&TestTreeNode{S: "00"}) != nil {
		t.Errorf("Expected 00 to be gone from the tree after mutation.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Walk depth / userData plumbing
// -------------------------------------------------------------------------------------------------------

func TestTreeWalkDepthAndUserData(t *testing.T) {
	tree := buildTree("05", "02", "09", "00", "03")

	// In-order depth of each node in this tree:
	//        05 (0)
	//   02 (1)    09 (1)
	// 00 (2) 03 (2)
	type visit struct {
		s     string
		depth int
	}
	var got []visit
	userData := "tag"
	var seenUserData []interface{}
	tree.WalkInOrder(func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
		got = append(got, visit{s: data.S, depth: depth})
		seenUserData = append(seenUserData, ud)
		return true
	}, userData)

	expect := []visit{
		{s: "00", depth: 2},
		{s: "02", depth: 1},
		{s: "03", depth: 2},
		{s: "05", depth: 0},
		{s: "09", depth: 1},
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("WalkInOrder depth error, expected %+v got %+v", expect, got)
	}
	for i, ud := range seenUserData {
		if ud != userData {
			t.Errorf("userData not passed through at pos %d: got %v", i, ud)
		}
	}

	// Pre-order and post-order early stop.
	var pre []string
	tree.WalkPreOrder(func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
		pre = append(pre, data.S)
		return false
	}, nil)
	if !reflect.DeepEqual(pre, []string{"05"}) {
		t.Errorf("WalkPreOrder early stop error, got %v", pre)
	}

	var post []string
	tree.WalkPostOrder(func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
		post = append(post, data.S)
		return false
	}, nil)
	if !reflect.DeepEqual(post, []string{"00"}) {
		t.Errorf("WalkPostOrder early stop error, got %v", post)
	}

	// Walks on an empty tree never invoke the callback.
	var empty BinaryTree[TestTreeNode]
	called := false
	noop := func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
		called = true
		return true
	}
	empty.WalkInOrder(noop, nil)
	empty.WalkPreOrder(noop, nil)
	empty.WalkPostOrder(noop, nil)
	if called {
		t.Errorf("Walk on empty tree invoked the callback.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Delete configurations not covered elsewhere
// -------------------------------------------------------------------------------------------------------

func TestTreeDeleteTwoChildrenDeepSuccessor(t *testing.T) {
	// Delete a node whose in-order successor is deep in the right subtree:
	//        10
	//    05      20
	//          15
	//        12    18
	tree := buildTree("10", "05", "20", "15", "12", "18")

	if !tree.Delete(&TestTreeNode{S: "10"}) {
		t.Fatalf("Expected Delete to find the root.")
	}
	if tree.Len() != 5 {
		t.Fatalf("Expected length 5 after delete, got %d", tree.Len())
	}
	expect := []string{"05", "12", "15", "18", "20"}
	if got := inOrder(tree); !reflect.DeepEqual(got, expect) {
		t.Errorf("In-order after delete error, expected %v got %v", expect, got)
	}
	for _, k := range expect {
		if tree.Search(&TestTreeNode{S: k}) == nil {
			t.Errorf("Expected to find %s after delete.", k)
		}
	}

	// Delete a node with only a left child that itself has children.
	tree2 := buildTree("10", "05", "03", "07")
	if !tree2.Delete(&TestTreeNode{S: "05"}) {
		t.Fatalf("Expected Delete to find 05.")
	}
	if got := inOrder(tree2); !reflect.DeepEqual(got, []string{"03", "07", "10"}) {
		t.Errorf("In-order error, expected [03 07 10] got %v", got)
	}

	// Delete a node with only a right child that itself has children.
	tree3 := buildTree("10", "20", "15", "25")
	if !tree3.Delete(&TestTreeNode{S: "20"}) {
		t.Fatalf("Expected Delete to find 20.")
	}
	if got := inOrder(tree3); !reflect.DeepEqual(got, []string{"10", "15", "25"}) {
		t.Errorf("In-order error, expected [10 15 25] got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Truncate then reuse
// -------------------------------------------------------------------------------------------------------

// TestTreeReverseEmpty verifies that Reverse on an empty tree is a no-op.
func TestTreeReverseEmpty(t *testing.T) {
	var tree BinaryTree[TestTreeNode]
	tree.Reverse() // must be a no-op, not a panic
	if !tree.IsEmpty() || tree.Len() != 0 {
		t.Errorf("Expected tree to still be empty after Reverse.")
	}
}

func TestTreeTruncateReuse(t *testing.T) {
	tree := buildTree("05", "02", "09")
	tree.Truncate()
	if !tree.IsEmpty() || tree.Len() != 0 || tree.Depth() != 0 {
		t.Errorf("Expected empty tree after Truncate.")
	}
	if tree.FindMin() != nil || tree.FindMax() != nil {
		t.Errorf("Expected nil min/max after Truncate.")
	}
	// The tree must be fully usable again.
	tree.Insert(&TestTreeNode{S: "42"})
	if got := inOrder(tree); !reflect.DeepEqual(got, []string{"42"}) {
		t.Errorf("Expected [42] after re-insert, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Degenerate (sorted input) shapes
// -------------------------------------------------------------------------------------------------------

func TestTreeDegenerateSortedInput(t *testing.T) {
	// Ascending inserts produce a right-only chain.
	var asc BinaryTree[TestTreeNode]
	for _, k := range []string{"01", "02", "03", "04", "05"} {
		asc.Insert(&TestTreeNode{S: k})
	}
	if d := asc.Depth(); d != 5 {
		t.Errorf("Expected depth 5 for ascending chain, got %d", d)
	}
	if got := inOrder(&asc); !reflect.DeepEqual(got, []string{"01", "02", "03", "04", "05"}) {
		t.Errorf("In-order error, got %v", got)
	}
	// Delete from the middle of a right-only chain.
	if !asc.Delete(&TestTreeNode{S: "03"}) {
		t.Errorf("Expected to delete 03.")
	}
	if got := inOrder(&asc); !reflect.DeepEqual(got, []string{"01", "02", "04", "05"}) {
		t.Errorf("In-order after delete error, got %v", got)
	}

	// Descending inserts produce a left-only chain.
	var desc BinaryTree[TestTreeNode]
	for _, k := range []string{"05", "04", "03", "02", "01"} {
		desc.Insert(&TestTreeNode{S: k})
	}
	if d := desc.Depth(); d != 5 {
		t.Errorf("Expected depth 5 for descending chain, got %d", d)
	}
	if !desc.Delete(&TestTreeNode{S: "03"}) {
		t.Errorf("Expected to delete 03.")
	}
	if got := inOrder(&desc); !reflect.DeepEqual(got, []string{"01", "02", "04", "05"}) {
		t.Errorf("In-order after delete error, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test (fixed seed) against a sorted-slice model
// -------------------------------------------------------------------------------------------------------

func TestTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	var tree BinaryTree[TestTreeNode]
	var model []string // sorted set of keys currently expected in the tree

	keyOf := func(i int) string { return fmt.Sprintf("%04d", i) }
	contains := func(k string) bool {
		i := sort.SearchStrings(model, k)
		return i < len(model) && model[i] == k
	}
	addToModel := func(k string) {
		i := sort.SearchStrings(model, k)
		if i < len(model) && model[i] == k {
			return
		}
		model = append(model, "")
		copy(model[i+1:], model[i:])
		model[i] = k
	}
	removeFromModel := func(k string) {
		i := sort.SearchStrings(model, k)
		if i < len(model) && model[i] == k {
			model = append(model[:i], model[i+1:]...)
		}
	}

	// verify cross-checks the tree against the model.
	verify := func(op string) {
		t.Helper()
		if tree.Len() != len(model) {
			t.Fatalf("after %s: Len()=%d, model has %d", op, tree.Len(), len(model))
		}
		if tree.IsEmpty() != (len(model) == 0) {
			t.Fatalf("after %s: IsEmpty()=%v, model len=%d", op, tree.IsEmpty(), len(model))
		}
		// Full in-order traversal must equal the sorted model.
		if got := inOrder(&tree); !equalStrings(got, model) {
			t.Fatalf("after %s: in-order=%v, model=%v", op, got, model)
		}
		// Backward iteration must be the reverse of the model.
		var back []string
		for v := range tree.Backward() {
			back = append(back, v.S)
		}
		if len(back) != len(model) {
			t.Fatalf("after %s: Backward yielded %d items, model has %d", op, len(back), len(model))
		}
		for i := range back {
			if back[i] != model[len(model)-1-i] {
				t.Fatalf("after %s: Backward mismatch at %d: %v vs model %v", op, i, back, model)
			}
		}
		if len(model) == 0 {
			if tree.FindMin() != nil || tree.FindMax() != nil {
				t.Fatalf("after %s: min/max non-nil on empty tree", op)
			}
			return
		}
		if x := tree.FindMin(); x == nil || x.S != model[0] {
			t.Fatalf("after %s: FindMin=%+v, model min=%s", op, x, model[0])
		}
		if x := tree.FindMax(); x == nil || x.S != model[len(model)-1] {
			t.Fatalf("after %s: FindMax=%+v, model max=%s", op, x, model[len(model)-1])
		}
		// Index(i) must match model[i]; out-of-range must be nil.
		for i, k := range model {
			if x := tree.Index(i); x == nil || x.S != k {
				t.Fatalf("after %s: Index(%d)=%+v, expected %s", op, i, x, k)
			}
		}
		if x := tree.Index(len(model)); x != nil {
			t.Fatalf("after %s: Index(%d)=%+v, expected nil", op, len(model), x)
		}
		// Depth must be consistent: a non-empty tree has depth >= 1.
		if d := tree.Depth(); d < 1 {
			t.Fatalf("after %s: Depth()=%d for non-empty tree", op, d)
		}
		// Every model key must be searchable; a random absent key must not be.
		for _, k := range model {
			if tree.Search(&TestTreeNode{S: k}) == nil {
				t.Fatalf("after %s: Search(%s) returned nil", op, k)
			}
		}
	}

	const ops = 600
	for i := 0; i < ops; i++ {
		k := keyOf(rng.Intn(120)) // small key space forces duplicates
		switch rng.Intn(8) {
		case 0, 1, 2: // Insert
			isNew := tree.Insert(&TestTreeNode{S: k})
			if isNew == containsBefore(model, k) {
				t.Fatalf("op %d: Insert(%s) returned %v, model said present=%v", i, k, isNew, contains(k))
			}
			addToModel(k)
		case 3, 4: // Delete
			wasPresent := contains(k)
			if found := tree.Delete(&TestTreeNode{S: k}); found != wasPresent {
				t.Fatalf("op %d: Delete(%s) returned %v, model said present=%v", i, k, found, wasPresent)
			}
			removeFromModel(k)
		case 5: // DeleteAtHead
			found := tree.DeleteAtHead()
			if (len(model) > 0) != found {
				t.Fatalf("op %d: DeleteAtHead returned %v, model len=%d", i, found, len(model))
			}
			if len(model) > 0 {
				removeFromModel(model[0])
			}
		case 6: // DeleteAtTail
			found := tree.DeleteAtTail()
			if (len(model) > 0) != found {
				t.Fatalf("op %d: DeleteAtTail returned %v, model len=%d", i, found, len(model))
			}
			if len(model) > 0 {
				removeFromModel(model[len(model)-1])
			}
		case 7: // Reverse twice is the identity; check in between.
			tree.Reverse()
			var rev []string
			tree.WalkInOrder(func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
				rev = append(rev, data.S)
				return true
			}, nil)
			for j := range rev {
				if rev[j] != model[len(model)-1-j] {
					t.Fatalf("op %d: after Reverse, in-order[%d]=%s, expected %s", i, j, rev[j], model[len(model)-1-j])
				}
			}
			tree.Reverse()
		}
		verify(fmt.Sprintf("op %d", i))
	}

	// Drain the tree with DeleteAtHead; each removal must take the current
	// model minimum.
	for len(model) > 0 {
		if !tree.DeleteAtHead() {
			t.Fatalf("DeleteAtHead returned false with %d model keys left", len(model))
		}
		removeFromModel(model[0])
		verify("drain")
	}
	if tree.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to fail on drained tree.")
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected tree to be empty after draining.")
	}
	verify("drained")
}

// containsBefore reports whether the sorted slice model already holds k.
// (Kept separate so the Insert check reads naturally at the call site.)
func containsBefore(model []string, k string) bool {
	i := sort.SearchStrings(model, k)
	return i < len(model) && model[i] == k
}

// equalStrings compares two string slices, treating nil and empty as equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------------------------------------
// Concurrency: writers, readers, iterators
// -------------------------------------------------------------------------------------------------------

// TestTreeConcurrentMixed hammers the tree with concurrent writers, readers,
// and iterators.  Run with -race to be meaningful.
func TestTreeConcurrentMixed(t *testing.T) {
	tree := NewBinaryTree[TestTreeNode]()

	// Pre-populate so readers/iterators see data from the start.
	for i := 0; i < 200; i++ {
		tree.Insert(&TestTreeNode{S: fmt.Sprintf("pre:%04d", i)})
	}

	var wg sync.WaitGroup

	// Writers: insert and delete their own disjoint key ranges.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				v := &TestTreeNode{S: fmt.Sprintf("w%d:%04d", w, i)}
				tree.Insert(v)
				if i%2 == 0 {
					tree.Delete(v)
				}
			}
		}(w)
	}

	// Head/tail deleters: shrink whatever is in the tree.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if w%2 == 0 {
					tree.DeleteAtHead()
				} else {
					tree.DeleteAtTail()
				}
			}
		}(w)
	}

	// Readers: search, min/max, length, depth, index.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				tree.Search(&TestTreeNode{S: fmt.Sprintf("pre:%04d", i%200)})
				tree.FindMin()
				tree.FindMax()
				tree.Len()
				tree.Length()
				tree.IsEmpty()
				tree.Depth()
				tree.Index(i % 50)
			}
		}()
	}

	// Iterators: range over snapshots, breaking early and in full.
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				for range tree.All() {
					break
				}
				if r%2 == 0 {
					n := 0
					for range tree.Backward() {
						n++
					}
					_ = n
				} else {
					for it := tree.Front(); !it.Done(); it.Next() {
						_ = it.Value()
					}
				}
				tree.WalkInOrder(func(pos, depth int, data *TestTreeNode, ud interface{}) bool {
					return true
				}, nil)
			}
		}(r)
	}

	wg.Wait()

	// Structural invariant after the storm: in-order traversal is sorted and
	// its length matches Len().
	prev := ""
	first := true
	count := 0
	for v := range tree.All() {
		if !first && prev >= v.S {
			t.Fatalf("Tree not in sorted order after concurrent run: %q then %q", prev, v.S)
		}
		first = false
		prev = v.S
		count++
	}
	if count != tree.Len() {
		t.Fatalf("Len()=%d but in-order walk visited %d nodes", tree.Len(), count)
	}
}

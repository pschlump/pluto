package binary_tree

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
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------------------------

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic on a nil tree, it did not.", name)
		}
	}()
	fx()
}

// inOrder returns the keys of the tree in in-order (ascending) order.
// It always returns a non-nil slice so empty-tree comparisons work with
// reflect.DeepEqual.
func inOrder(tt *BinaryTree[TestTreeNode]) []string {
	got := []string{}
	for v := range tt.All() {
		got = append(got, v.S)
	}
	return got
}

// checkInvariants verifies the BST ordering invariant, that Length matches
// the number of nodes, and that every in-order key is found by Search.
func checkInvariants(t *testing.T, tt *BinaryTree[TestTreeNode]) {
	t.Helper()
	got := inOrder(tt)
	if !sort.StringsAreSorted(got) {
		t.Errorf("BST invariant violated: in-order walk is not sorted: %v", got)
	}
	if len(got) != tt.Length() {
		t.Errorf("Length mismatch: Length()=%d but in-order walk has %d nodes", tt.Length(), len(got))
	}
	seen := make(map[string]bool, len(got))
	for _, s := range got {
		if seen[s] {
			t.Errorf("BST invariant violated: duplicate key %q in tree", s)
		}
		seen[s] = true
		if tt.Search(&TestTreeNode{S: s}) == nil {
			t.Errorf("Search failed to find in-order key %q", s)
		}
	}
	if len(got) == 0 && !tt.IsEmpty() {
		t.Errorf("IsEmpty() is false but the tree has no nodes")
	}
	if len(got) > 0 && tt.IsEmpty() {
		t.Errorf("IsEmpty() is true but the tree has %d nodes", len(got))
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructor and nil-tree panics
// -------------------------------------------------------------------------------------------------------

func TestTreeNewBinaryTree(t *testing.T) {
	tree := NewBinaryTree[TestTreeNode]()
	if tree == nil {
		t.Fatalf("NewBinaryTree returned nil.")
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected new tree to be empty.")
	}
	if tree.Len() != 0 || tree.Length() != 0 {
		t.Errorf("Expected new tree to have length 0, got %d/%d", tree.Len(), tree.Length())
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected new tree to have depth 0.")
	}
	// The constructed tree must be usable.
	if !tree.Insert(&TestTreeNode{S: "05"}) {
		t.Errorf("Expected Insert on constructed tree to return true.")
	}
	if tree.Length() != 1 {
		t.Errorf("Expected length 1 after insert, got %d", tree.Length())
	}
}

// TestTreeNilPanics verifies the documented panics when methods are called
// on a nil tree receiver.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *BinaryTree[TestTreeNode]
	key := &TestTreeNode{S: "05"}

	expectPanic(t, "Insert", func() { nilTree.Insert(key) })
	expectPanic(t, "Search", func() { nilTree.Search(key) })
	expectPanic(t, "Delete", func() { nilTree.Delete(key) })
	expectPanic(t, "DeleteMatch", func() {
		nilTree.DeleteMatch(key, func(a, b *TestTreeNode) int { return a.Compare(*b) })
	})
	expectPanic(t, "FindMin", func() { nilTree.FindMin() })
	expectPanic(t, "FindMax", func() { nilTree.FindMax() })
	expectPanic(t, "DeleteAtHead", func() { nilTree.DeleteAtHead() })
	expectPanic(t, "DeleteAtTail", func() { nilTree.DeleteAtTail() })
	expectPanic(t, "Reverse", func() { nilTree.Reverse() })
	expectPanic(t, "Index", func() { nilTree.Index(0) })
	expectPanic(t, "Depth", func() { nilTree.Depth() })
	expectPanic(t, "WalkFunc", func() { nilTree.WalkFunc(func(a *TestTreeNode) {}) })

	// Verify the panic message names the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Search to panic on nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Search") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilTree.Search(key)
	}()
}

// -------------------------------------------------------------------------------------------------------
// Dump
// -------------------------------------------------------------------------------------------------------

// failingWriter always returns an error, to exercise the error path in Dump.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestTreeDump(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	// Dump of an empty tree produces no output.
	var buf bytes.Buffer
	Tree1.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty tree, got %q", buf.String())
	}

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})

	buf.Reset()
	Tree1.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines from Dump, got %d:\n%s", len(lines), out)
	}
	// Dump is an in-order traversal, so keys appear in ascending order.
	// Values are printed with %v, so a TestTreeNode shows up as "{00}".
	var keys []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			t.Fatalf("Unexpected empty line in Dump output: %q", out)
		}
		keys = append(keys, strings.Trim(fields[0], "{}"))
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(keys, expect) {
		t.Errorf("Dump order error, expected %s got %s", expect, keys)
	}
	// Deeper nodes are indented more than the root.
	rootIndent := strings.Index(lines[3], "05")
	leafIndent := strings.Index(lines[0], "00")
	if leafIndent <= rootIndent {
		t.Errorf("Expected deeper nodes to be indented more in Dump output:\n%s", out)
	}

	// A failing writer must not panic; the traversal just stops early.
	Tree1.Dump(failingWriter{})
}

// failAfterWriter allows `n` successful writes, then fails.  This exercises
// the error propagation through the right-subtree recursion in Dump: a node
// whose own write succeeds but whose right child's write fails must stop the
// whole traversal without panicking.
type failAfterWriter struct {
	n       int
	written int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.written >= w.n {
		return 0, errors.New("write failed")
	}
	w.written++
	return len(p), nil
}

func TestTreeDumpPartialWriteFailure(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	//        {05}
	//    {02}      {09}
	//  {00} {03}
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(&TestTreeNode{S: s})
	}

	// Writes happen in-order: 00, 02, 03, 05, 09.  Failing on the 3rd write
	// (03) makes the right-subtree recursion of node 02 return false, which
	// node 02 must propagate upward.
	w := &failAfterWriter{n: 2}
	Tree1.Dump(w) // must not panic
	if w.written != 2 {
		t.Errorf("Expected Dump to stop after the 2nd successful write, got %d writes", w.written)
	}

	// Failing on the 1st write stops immediately (leftmost node's own write).
	w = &failAfterWriter{n: 0}
	Tree1.Dump(w)

	// Every failure point must be handled without panic.
	for n := 0; n <= 5; n++ {
		Tree1.Dump(&failAfterWriter{n: n})
	}
}

// -------------------------------------------------------------------------------------------------------
// Insert / duplicate replacement
// -------------------------------------------------------------------------------------------------------

// TestTreeInsertDuplicateReplaces verifies that inserting a duplicate key
// replaces the stored item (the new pointer is what Search returns) while
// preserving the node's children and the tree length.
func TestTreeInsertDuplicateReplaces(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	Tree1.Insert(&TestTreeNode{S: "08"})

	first := Tree1.Search(&TestTreeNode{S: "05"})
	if first == nil {
		t.Fatalf("Expected to find 05.")
	}

	// Insert duplicates at the root, a left child, and a right child.
	for _, s := range []string{"05", "03", "08"} {
		if Tree1.Insert(&TestTreeNode{S: s}) {
			t.Errorf("Expected duplicate insert of %s to return false.", s)
		}
	}
	if Tree1.Length() != 3 {
		t.Errorf("Expected length 3 after duplicate inserts, got %d", Tree1.Length())
	}
	second := Tree1.Search(&TestTreeNode{S: "05"})
	if second == first {
		t.Errorf("Expected duplicate insert to replace the stored item, but the same pointer is stored.")
	}
	checkInvariants(t, &Tree1)
	if expect := []string{"03", "05", "08"}; !reflect.DeepEqual(inOrder(&Tree1), expect) {
		t.Errorf("In-order error after duplicate inserts, expected %s got %v", expect, inOrder(&Tree1))
	}
}

// -------------------------------------------------------------------------------------------------------
// Delete cases
// -------------------------------------------------------------------------------------------------------

// TestTreeDeleteChildShapes exercises Delete for every child shape: leaf,
// left-child-only, right-child-only, and two children with a successor that
// itself has a right subtree.
func TestTreeDeleteChildShapes(t *testing.T) {
	build := func() *BinaryTree[TestTreeNode] {
		//        {20}
		//    {10}      {30}
		//  {05} {15} {25} {40}
		//           {28}
		tree := NewBinaryTree[TestTreeNode]()
		for _, s := range []string{"20", "10", "30", "05", "15", "25", "40", "28"} {
			tree.Insert(&TestTreeNode{S: s})
		}
		return tree
	}
	sorted := []string{"05", "10", "15", "20", "25", "28", "30", "40"}

	remove := func(s string) []string {
		var out []string
		for _, v := range sorted {
			if v != s {
				out = append(out, v)
			}
		}
		return out
	}

	// Leaf delete.
	tree := build()
	if !tree.Delete(&TestTreeNode{S: "05"}) {
		t.Errorf("leaf: expected Delete(05) to return true")
	}
	if got := inOrder(tree); !reflect.DeepEqual(got, remove("05")) {
		t.Errorf("leaf: expected %v got %v", remove("05"), got)
	}
	checkInvariants(t, tree)

	// Node with only a left child: delete 25 after removing 28 makes 25 a leaf;
	// instead build a tree where a node has only a left child:
	//        {20}
	//    {10}
	//  {05}
	t2 := NewBinaryTree[TestTreeNode]()
	for _, s := range []string{"20", "10", "05"} {
		t2.Insert(&TestTreeNode{S: s})
	}
	if !t2.Delete(&TestTreeNode{S: "10"}) { // 10 has only left child 05.
		t.Errorf("leftChildOnly: expected Delete(10) to return true")
	}
	if got := inOrder(t2); !reflect.DeepEqual(got, []string{"05", "20"}) {
		t.Errorf("leftChildOnly: expected [05 20] got %v", got)
	}
	checkInvariants(t, t2)

	// Node with only a right child.
	t3 := NewBinaryTree[TestTreeNode]()
	for _, s := range []string{"20", "10", "15"} {
		t3.Insert(&TestTreeNode{S: s})
	}
	if !t3.Delete(&TestTreeNode{S: "10"}) { // 10 has only right child 15.
		t.Errorf("rightChildOnly: expected Delete(10) to return true")
	}
	if got := inOrder(t3); !reflect.DeepEqual(got, []string{"15", "20"}) {
		t.Errorf("rightChildOnly: expected [15 20] got %v", got)
	}
	checkInvariants(t, t3)

	// Two children where the in-order successor has its own right subtree.
	t4 := build()
	if !t4.Delete(&TestTreeNode{S: "20"}) {
		t.Errorf("twoChildren: expected Delete(20) to return true")
	}
	if got := inOrder(t4); !reflect.DeepEqual(got, remove("20")) {
		t.Errorf("twoChildren: expected %v got %v", remove("20"), got)
	}
	checkInvariants(t, t4)
	if x := t4.FindMin(); x == nil || x.S != "05" {
		t.Errorf("twoChildren: expected min 05, got %+v", x)
	}

	// Two children deep in the tree.
	t5 := build()
	if !t5.Delete(&TestTreeNode{S: "30"}) {
		t.Errorf("twoChildrenMid: expected Delete(30) to return true")
	}
	if got := inOrder(t5); !reflect.DeepEqual(got, remove("30")) {
		t.Errorf("twoChildrenMid: expected %v got %v", remove("30"), got)
	}
	checkInvariants(t, t5)

	// Deleting a key that is not in the tree returns false and changes nothing.
	t6 := build()
	if t6.Delete(&TestTreeNode{S: "12"}) {
		t.Errorf("expected Delete of missing key to return false")
	}
	if got := inOrder(t6); !reflect.DeepEqual(got, sorted) {
		t.Errorf("missing-key delete changed the tree: expected %v got %v", sorted, got)
	}
	checkInvariants(t, t6)

	// Delete every node, one at a time, in-order.
	t7 := build()
	for i, s := range sorted {
		if !t7.Delete(&TestTreeNode{S: s}) {
			t.Fatalf("expected Delete(%s) to return true", s)
		}
		if got := inOrder(t7); !reflect.DeepEqual(got, sorted[i+1:]) {
			t.Errorf("after deleting %s: expected %v got %v", s, sorted[i+1:], got)
		}
		checkInvariants(t, t7)
	}
	if !t7.IsEmpty() {
		t.Errorf("expected empty tree after deleting all nodes")
	}
}

// -------------------------------------------------------------------------------------------------------
// Single-element and degenerate shapes
// -------------------------------------------------------------------------------------------------------

func TestTreeSingleElement(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree.")
	}
	if Tree1.Length() != 1 {
		t.Errorf("Expected length 1, got %d", Tree1.Length())
	}
	if Tree1.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", Tree1.Depth())
	}
	if x := Tree1.FindMin(); x == nil || x.S != "05" {
		t.Errorf("Expected min 05, got %+v", x)
	}
	if x := Tree1.FindMax(); x == nil || x.S != "05" {
		t.Errorf("Expected max 05, got %+v", x)
	}
	if x := Tree1.Index(0); x == nil || x.S != "05" {
		t.Errorf("Expected Index(0) = 05, got %+v", x)
	}
	if got := inOrder(&Tree1); !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05], got %v", got)
	}
	var bk []string
	for v := range Tree1.Backward() {
		bk = append(bk, v.S)
	}
	if !reflect.DeepEqual(bk, []string{"05"}) {
		t.Errorf("Expected backward [05], got %v", bk)
	}

	// Reverse on a single node is a no-op.
	Tree1.Reverse()
	if got := inOrder(&Tree1); !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05] after Reverse, got %v", got)
	}

	// DeleteAtHead then re-insert; DeleteAtTail on the fresh node.
	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true.")
	}
	if !Tree1.IsEmpty() || Tree1.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead.")
	}
	Tree1.Insert(&TestTreeNode{S: "07"})
	if !Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true.")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after DeleteAtTail.")
	}
}

// TestTreeDegenerate builds a right-leaning (sorted input) and a left-leaning
// (reverse-sorted input) chain and verifies depth and ordering.
func TestTreeDegenerate(t *testing.T) {
	var right BinaryTree[TestTreeNode]
	for i := 0; i < 10; i++ {
		right.Insert(&TestTreeNode{S: fmt.Sprintf("%02d", i)})
	}
	if d := right.Depth(); d != 10 {
		t.Errorf("Expected depth 10 for sorted input, got %d", d)
	}
	checkInvariants(t, &right)

	var left BinaryTree[TestTreeNode]
	for i := 9; i >= 0; i-- {
		left.Insert(&TestTreeNode{S: fmt.Sprintf("%02d", i)})
	}
	if d := left.Depth(); d != 10 {
		t.Errorf("Expected depth 10 for reverse-sorted input, got %d", d)
	}
	checkInvariants(t, &left)

	// Both degenerate trees must still iterate correctly.
	var expect []string
	for i := 0; i < 10; i++ {
		expect = append(expect, fmt.Sprintf("%02d", i))
	}
	if got := inOrder(&right); !reflect.DeepEqual(got, expect) {
		t.Errorf("Right-chain in-order error, expected %v got %v", expect, got)
	}
	if got := inOrder(&left); !reflect.DeepEqual(got, expect) {
		t.Errorf("Left-chain in-order error, expected %v got %v", expect, got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Reverse, Index, DeleteAtHead/Tail edges
// -------------------------------------------------------------------------------------------------------

func TestTreeReverseEmpty(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	Tree1.Reverse() // must not panic
	if !Tree1.IsEmpty() {
		t.Errorf("Expected tree to remain empty after Reverse.")
	}
}

func TestTreeIndexAllPositions(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	keys := []string{"05", "02", "09", "00", "03", "07", "08", "01"}
	for _, s := range keys {
		Tree1.Insert(&TestTreeNode{S: s})
	}
	sortedKeys := append([]string(nil), keys...)
	sort.Strings(sortedKeys)

	for i, s := range sortedKeys {
		x := Tree1.Index(i)
		if x == nil {
			t.Errorf("Index(%d) returned nil, expected %s", i, s)
		} else if x.S != s {
			t.Errorf("Index(%d) = %s, expected %s", i, x.S, s)
		}
	}
	if x := Tree1.Index(len(sortedKeys)); x != nil {
		t.Errorf("Expected nil for Index(len), got %+v", x)
	}
}

func TestTreeDeleteAtHeadTailDrain(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	keys := []string{"05", "02", "09", "00", "03", "07", "08", "01"}
	for _, s := range keys {
		Tree1.Insert(&TestTreeNode{S: s})
	}

	// Drain from the head: min must increase each time.
	sortedKeys := append([]string(nil), keys...)
	sort.Strings(sortedKeys)
	for i, s := range sortedKeys {
		if x := Tree1.FindMin(); x == nil || x.S != s {
			t.Fatalf("Drain head: expected min %s, got %+v", s, x)
		}
		if !Tree1.DeleteAtHead() {
			t.Fatalf("Drain head: DeleteAtHead returned false at step %d", i)
		}
		checkInvariants(t, &Tree1)
	}
	if Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on drained tree to return false.")
	}

	// Refill and drain from the tail.
	for _, s := range keys {
		Tree1.Insert(&TestTreeNode{S: s})
	}
	for i := len(sortedKeys) - 1; i >= 0; i-- {
		if x := Tree1.FindMax(); x == nil || x.S != sortedKeys[i] {
			t.Fatalf("Drain tail: expected max %s, got %+v", sortedKeys[i], x)
		}
		if !Tree1.DeleteAtTail() {
			t.Fatalf("Drain tail: DeleteAtTail returned false at step %d", i)
		}
		checkInvariants(t, &Tree1)
	}
	if Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on drained tree to return false.")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after draining.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Walks: depth parameter, userData, early stop for pre/post order
// -------------------------------------------------------------------------------------------------------

func TestTreeWalkDepthAndUserData(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	//        {05}
	//    {02}      {09}
	//  {00} {03}
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(&TestTreeNode{S: s})
	}
	expectedDepth := map[string]int{"05": 0, "02": 1, "09": 1, "00": 2, "03": 2}

	userData := "marker"
	fx := func(p, depth int, data *TestTreeNode, ud interface{}) bool {
		if ud != userData {
			t.Errorf("Expected userData to be passed through, got %v", ud)
		}
		if d, ok := expectedDepth[data.S]; !ok {
			t.Errorf("Unexpected key %s in walk", data.S)
		} else if d != depth {
			t.Errorf("Wrong depth for %s: expected %d got %d", data.S, d, depth)
		}
		return true
	}
	Tree1.WalkInOrder(fx, userData)
	Tree1.WalkPreOrder(fx, userData)
	Tree1.WalkPostOrder(fx, userData)

	// Walks on an empty tree must not call the callback at all.
	var empty BinaryTree[TestTreeNode]
	called := false
	noop := func(p, depth int, data *TestTreeNode, ud interface{}) bool {
		called = true
		return true
	}
	empty.WalkInOrder(noop, nil)
	empty.WalkPreOrder(noop, nil)
	empty.WalkPostOrder(noop, nil)
	if called {
		t.Errorf("Walk on empty tree called the callback.")
	}
}

func TestTreeWalkEarlyStopPrePost(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(&TestTreeNode{S: s})
	}

	// Pre-order early stop: only the root is visited.
	var pre []string
	Tree1.WalkPreOrder(func(p, depth int, data *TestTreeNode, ud interface{}) bool {
		pre = append(pre, data.S)
		return false
	}, nil)
	if expect := []string{"05"}; !reflect.DeepEqual(pre, expect) {
		t.Errorf("PreOrder early stop error, expected %s got %s", expect, pre)
	}

	// Post-order early stop: only the first post-order element is visited.
	var post []string
	Tree1.WalkPostOrder(func(p, depth int, data *TestTreeNode, ud interface{}) bool {
		post = append(post, data.S)
		return false
	}, nil)
	if expect := []string{"00"}; !reflect.DeepEqual(post, expect) {
		t.Errorf("PostOrder early stop error, expected %s got %s", expect, post)
	}

	// Stop after 3 elements in each order.
	stopAfter := func(order string, walk func(ApplyFunction[TestTreeNode], interface{}), expect []string) {
		var got []string
		walk(func(p, depth int, data *TestTreeNode, ud interface{}) bool {
			got = append(got, data.S)
			return len(got) < 3
		}, nil)
		if !reflect.DeepEqual(got, expect) {
			t.Errorf("%s stop-after-3 error, expected %s got %s", order, expect, got)
		}
	}
	stopAfter("InOrder", Tree1.WalkInOrder, []string{"00", "02", "03"})
	stopAfter("PreOrder", Tree1.WalkPreOrder, []string{"05", "02", "00"})
	stopAfter("PostOrder", Tree1.WalkPostOrder, []string{"00", "03", "02"})
}

// -------------------------------------------------------------------------------------------------------
// Truncate reuse
// -------------------------------------------------------------------------------------------------------

func TestTreeTruncateReuse(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(&TestTreeNode{S: s})
	}
	Tree1.Truncate()
	if !Tree1.IsEmpty() || Tree1.Length() != 0 || Tree1.Depth() != 0 {
		t.Errorf("Expected fully empty tree after Truncate.")
	}
	// Tree must be fully reusable after Truncate.
	for _, s := range []string{"50", "20", "90"} {
		Tree1.Insert(&TestTreeNode{S: s})
	}
	checkInvariants(t, &Tree1)
	if got := inOrder(&Tree1); !reflect.DeepEqual(got, []string{"20", "50", "90"}) {
		t.Errorf("Expected [20 50 90] after re-fill, got %v", got)
	}
	// Truncating an already-empty tree is fine.
	var empty BinaryTree[TestTreeNode]
	empty.Truncate()
	if !empty.IsEmpty() {
		t.Errorf("Expected empty tree after Truncate on empty tree.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

func TestTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	var tree BinaryTree[TestTreeNode]
	model := make(map[string]bool) // set of keys the tree should contain

	keyOf := func(i int) string { return fmt.Sprintf("%03d", i) }
	sortedModel := func() []string {
		keys := []string{}
		for k := range model {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	verify := func(step int) {
		keys := sortedModel()
		if tree.Length() != len(model) {
			t.Fatalf("step %d: Length()=%d, model has %d keys", step, tree.Length(), len(model))
		}
		if got := inOrder(&tree); !reflect.DeepEqual(got, keys) {
			t.Fatalf("step %d: in-order mismatch, expected %v got %v", step, keys, got)
		}
		// Backward must be the exact reverse of All.
		var bk []string
		for v := range tree.Backward() {
			bk = append(bk, v.S)
		}
		for i := range bk {
			if bk[i] != keys[len(keys)-1-i] {
				t.Fatalf("step %d: Backward mismatch at %d: %v vs %v", step, i, bk, keys)
			}
		}
		// FindMin/FindMax must match the model extremes.
		if len(keys) == 0 {
			if tree.FindMin() != nil || tree.FindMax() != nil {
				t.Fatalf("step %d: expected nil min/max on empty tree", step)
			}
		} else {
			if x := tree.FindMin(); x == nil || x.S != keys[0] {
				t.Fatalf("step %d: FindMin=%+v, expected %s", step, x, keys[0])
			}
			if x := tree.FindMax(); x == nil || x.S != keys[len(keys)-1] {
				t.Fatalf("step %d: FindMax=%+v, expected %s", step, x, keys[len(keys)-1])
			}
			// Index must agree with the sorted model at every position.
			for i, k := range keys {
				if x := tree.Index(i); x == nil || x.S != k {
					t.Fatalf("step %d: Index(%d)=%+v, expected %s", step, i, x, k)
				}
			}
		}
		checkInvariants(t, &tree)
	}

	const maxKey = 60 // small key space so duplicates and deletes are common
	for step := 0; step < 800; step++ {
		k := keyOf(rng.Intn(maxKey))
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert (duplicates replace in both tree and model)
			added := tree.Insert(&TestTreeNode{S: k})
			if added == model[k] {
				t.Fatalf("step %d: Insert(%s)=%v, model said present=%v", step, k, added, model[k])
			}
			model[k] = true
		case 4, 5, 6: // Delete
			got := tree.Delete(&TestTreeNode{S: k})
			if got != model[k] {
				t.Fatalf("step %d: Delete(%s)=%v, model said present=%v", step, k, got, model[k])
			}
			delete(model, k)
		case 7: // Search
			found := tree.Search(&TestTreeNode{S: k})
			if (found != nil) != model[k] {
				t.Fatalf("step %d: Search(%s) found=%v, model said present=%v", step, k, found != nil, model[k])
			}
		case 8: // DeleteAtHead
			got := tree.DeleteAtHead()
			if len(model) == 0 {
				if got {
					t.Fatalf("step %d: DeleteAtHead on empty tree returned true", step)
				}
			} else {
				if !got {
					t.Fatalf("step %d: DeleteAtHead returned false on non-empty tree", step)
				}
				delete(model, sortedModel()[0])
			}
		case 9: // DeleteAtTail
			got := tree.DeleteAtTail()
			if len(model) == 0 {
				if got {
					t.Fatalf("step %d: DeleteAtTail on empty tree returned true", step)
				}
			} else {
				if !got {
					t.Fatalf("step %d: DeleteAtTail returned false on non-empty tree", step)
				}
				keys := sortedModel()
				delete(model, keys[len(keys)-1])
			}
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)

	// Depth sanity: for <= 60 distinct random keys the depth of a random BST
	// is well under the worst case; just verify the reported depth is at
	// least the information-theoretic minimum for its size.
	if n := tree.Length(); n > 0 {
		d := tree.Depth()
		if d < 1 || d > n {
			t.Errorf("Depth %d out of range for %d nodes", d, n)
		}
	}
}

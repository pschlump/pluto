package binary_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
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
		if _, found := tt.Search(TestTreeNode{S: s}); !found {
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
	tree := newTestTree()
	if tree == nil {
		t.Fatalf("NewBinaryTreeFunc returned nil.")
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
	if !tree.Insert(TestTreeNode{S: "05"}) {
		t.Errorf("Expected Insert on constructed tree to return true.")
	}
	if tree.Length() != 1 {
		t.Errorf("Expected length 1 after insert, got %d", tree.Length())
	}

	// Same checks for the natural-ordering constructor.
	ordered := NewBinaryTree[string]()
	if !ordered.IsEmpty() || ordered.Len() != 0 || ordered.Depth() != 0 {
		t.Errorf("Expected NewBinaryTree to start empty.")
	}
	if !ordered.Insert("05") {
		t.Errorf("Expected Insert on NewBinaryTree tree to return true.")
	}
}

// TestTreeNilPanics verifies the documented panic when Insert is called on
// a nil tree — the one operation with no sane answer, since a nil tree
// cannot store an element.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *BinaryTree[TestTreeNode]
	key := TestTreeNode{S: "05"}

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
	var nilTree *BinaryTree[TestTreeNode]
	key := TestTreeNode{S: "05"}

	if !nilTree.IsEmpty() {
		t.Errorf("Expected nil tree to be empty.")
	}
	if nilTree.Len() != 0 || nilTree.Length() != 0 {
		t.Errorf("Expected nil tree to have length 0.")
	}
	if _, found := nilTree.Search(key); found {
		t.Errorf("Expected not-found from Search on nil tree.")
	}
	if nilTree.Delete(key) {
		t.Errorf("Expected false from Delete on nil tree.")
	}
	if nilTree.DeleteMatch(key, cmpTestTreeNode) {
		t.Errorf("Expected false from DeleteMatch on nil tree.")
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
	if nilTree.Depth() != 0 {
		t.Errorf("Expected depth 0 on nil tree.")
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
	nilTree.WalkFunc(func(a TestTreeNode) { called = true })
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

	var buf bytes.Buffer
	nilTree.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil tree, got %q", buf.String())
	}
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
	Tree1 := newTestTree()

	// Dump of an empty tree produces no output.
	var buf bytes.Buffer
	Tree1.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty tree, got %q", buf.String())
	}

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	buf.Reset()
	Tree1.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines from Dump, got %d:\n%s", len(lines), out)
	}
	// Dump is an in-order traversal, so keys appear in ascending order.
	// Values are printed with %v, so a TestTreeNode shows up as "{00 0}".
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

// -------------------------------------------------------------------------------------------------------
// Insert / duplicate replacement
// -------------------------------------------------------------------------------------------------------

// TestTreeInsertDuplicateReplaces verifies that inserting a duplicate key
// replaces the stored value (the new satellite data is what Search
// returns) while preserving the node's children and the tree length.
func TestTreeInsertDuplicateReplaces(t *testing.T) {
	tree := newTestTree()

	tree.Insert(TestTreeNode{S: "05", N: 1})
	tree.Insert(TestTreeNode{S: "03", N: 1})
	tree.Insert(TestTreeNode{S: "08", N: 1})

	first, found := tree.Search(TestTreeNode{S: "05"})
	if !found {
		t.Fatalf("Expected to find 05.")
	}
	if first.N != 1 {
		t.Fatalf("Expected the original N of 1, got %d.", first.N)
	}

	// Insert duplicates at the root, a left child, and a right child.
	for _, s := range []string{"05", "03", "08"} {
		if tree.Insert(TestTreeNode{S: s, N: 7}) {
			t.Errorf("Expected duplicate insert of %s to return false.", s)
		}
	}
	if tree.Length() != 3 {
		t.Errorf("Expected length 3 after duplicate inserts, got %d", tree.Length())
	}
	second, _ := tree.Search(TestTreeNode{S: "05"})
	if second.N != 7 {
		t.Errorf("Expected duplicate insert to replace the stored value, got N=%d want 7.", second.N)
	}
	checkInvariants(t, tree)
	if expect := []string{"03", "05", "08"}; !reflect.DeepEqual(inOrder(tree), expect) {
		t.Errorf("In-order error after duplicate inserts, expected %s got %s", expect, inOrder(tree))
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
		tree := newTestTree()
		for _, s := range []string{"20", "10", "30", "05", "15", "25", "40", "28"} {
			tree.Insert(TestTreeNode{S: s})
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
	if !tree.Delete(TestTreeNode{S: "05"}) {
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
	t2 := newTestTree()
	for _, s := range []string{"20", "10", "05"} {
		t2.Insert(TestTreeNode{S: s})
	}
	if !t2.Delete(TestTreeNode{S: "10"}) { // 10 has only left child 05.
		t.Errorf("leftChildOnly: expected Delete(10) to return true")
	}
	if got := inOrder(t2); !reflect.DeepEqual(got, []string{"05", "20"}) {
		t.Errorf("leftChildOnly: expected [05 20] got %v", got)
	}
	checkInvariants(t, t2)

	// Node with only a right child.
	t3 := newTestTree()
	for _, s := range []string{"20", "10", "15"} {
		t3.Insert(TestTreeNode{S: s})
	}
	if !t3.Delete(TestTreeNode{S: "10"}) { // 10 has only right child 15.
		t.Errorf("rightChildOnly: expected Delete(10) to return true")
	}
	if got := inOrder(t3); !reflect.DeepEqual(got, []string{"15", "20"}) {
		t.Errorf("rightChildOnly: expected [15 20] got %v", got)
	}
	checkInvariants(t, t3)

	// Two children where the in-order successor has its own right subtree.
	t4 := build()
	if !t4.Delete(TestTreeNode{S: "20"}) {
		t.Errorf("twoChildren: expected Delete(20) to return true")
	}
	if got := inOrder(t4); !reflect.DeepEqual(got, remove("20")) {
		t.Errorf("twoChildren: expected %v got %v", remove("20"), got)
	}
	checkInvariants(t, t4)
	if x, found := t4.FindMin(); !found || x.S != "05" {
		t.Errorf("twoChildren: expected min 05, got %+v", x)
	}

	// Two children deep in the tree.
	t5 := build()
	if !t5.Delete(TestTreeNode{S: "30"}) {
		t.Errorf("twoChildrenMid: expected Delete(30) to return true")
	}
	if got := inOrder(t5); !reflect.DeepEqual(got, remove("30")) {
		t.Errorf("twoChildrenMid: expected %v got %v", remove("30"), got)
	}
	checkInvariants(t, t5)

	// Deleting a key that is not in the tree returns false and changes nothing.
	t6 := build()
	if t6.Delete(TestTreeNode{S: "12"}) {
		t.Errorf("expected Delete of missing key to return false")
	}
	if got := inOrder(t6); !reflect.DeepEqual(got, sorted) {
		t.Errorf("missing-key delete changed the tree: expected %v got %v", sorted, got)
	}
	checkInvariants(t, t6)

	// Delete every node, one at a time, in-order.
	t7 := build()
	for i, s := range sorted {
		if !t7.Delete(TestTreeNode{S: s}) {
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
	Tree1 := newTestTree()
	Tree1.Insert(TestTreeNode{S: "05"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree.")
	}
	if Tree1.Length() != 1 {
		t.Errorf("Expected length 1, got %d", Tree1.Length())
	}
	if Tree1.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", Tree1.Depth())
	}
	if x, found := Tree1.FindMin(); !found || x.S != "05" {
		t.Errorf("Expected min 05, got %+v", x)
	}
	if x, found := Tree1.FindMax(); !found || x.S != "05" {
		t.Errorf("Expected max 05, got %+v", x)
	}
	if x, found := Tree1.Index(0); !found || x.S != "05" {
		t.Errorf("Expected Index(0) = 05, got %+v", x)
	}
	if got := inOrder(Tree1); !reflect.DeepEqual(got, []string{"05"}) {
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
	if got := inOrder(Tree1); !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05] after Reverse, got %v", got)
	}

	// DeleteAtHead then re-insert; DeleteAtTail on the fresh node.
	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true.")
	}
	if !Tree1.IsEmpty() || Tree1.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead.")
	}
	Tree1.Insert(TestTreeNode{S: "07"})
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
	right := newTestTree()
	for i := range 10 {
		right.Insert(TestTreeNode{S: fmt.Sprintf("%02d", i)})
	}
	if d := right.Depth(); d != 10 {
		t.Errorf("Expected depth 10 for sorted input, got %d", d)
	}
	checkInvariants(t, right)

	left := newTestTree()
	for i := 9; i >= 0; i-- {
		left.Insert(TestTreeNode{S: fmt.Sprintf("%02d", i)})
	}
	if d := left.Depth(); d != 10 {
		t.Errorf("Expected depth 10 for reverse-sorted input, got %d", d)
	}
	checkInvariants(t, left)

	// Both degenerate trees must still iterate correctly.
	var expect []string
	for i := range 10 {
		expect = append(expect, fmt.Sprintf("%02d", i))
	}
	if got := inOrder(right); !reflect.DeepEqual(got, expect) {
		t.Errorf("Right-chain in-order error, expected %v got %v", expect, got)
	}
	if got := inOrder(left); !reflect.DeepEqual(got, expect) {
		t.Errorf("Left-chain in-order error, expected %v got %v", expect, got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Reverse, Index, DeleteAtHead/Tail edges
// -------------------------------------------------------------------------------------------------------

func TestTreeReverseEmpty(t *testing.T) {
	Tree1 := newTestTree()
	Tree1.Reverse() // must not panic
	if !Tree1.IsEmpty() {
		t.Errorf("Expected tree to remain empty after Reverse.")
	}
}

func TestTreeIndexAllPositions(t *testing.T) {
	Tree1 := newTestTree()
	keys := []string{"05", "02", "09", "00", "03", "07", "08", "01"}
	for _, s := range keys {
		Tree1.Insert(TestTreeNode{S: s})
	}
	sortedKeys := append([]string(nil), keys...)
	sort.Strings(sortedKeys)

	for i, s := range sortedKeys {
		x, found := Tree1.Index(i)
		if !found {
			t.Errorf("Index(%d) returned not-found, expected %s", i, s)
		} else if x.S != s {
			t.Errorf("Index(%d) = %s, expected %s", i, x.S, s)
		}
	}
	if _, found := Tree1.Index(len(sortedKeys)); found {
		t.Errorf("Expected not-found for Index(len)")
	}
}

func TestTreeDeleteAtHeadTailDrain(t *testing.T) {
	Tree1 := newTestTree()
	keys := []string{"05", "02", "09", "00", "03", "07", "08", "01"}
	for _, s := range keys {
		Tree1.Insert(TestTreeNode{S: s})
	}

	// Drain from the head: min must increase each time.
	sortedKeys := append([]string(nil), keys...)
	sort.Strings(sortedKeys)
	for i, s := range sortedKeys {
		if x, found := Tree1.FindMin(); !found || x.S != s {
			t.Fatalf("Drain head: expected min %s, got %+v", s, x)
		}
		if !Tree1.DeleteAtHead() {
			t.Fatalf("Drain head: DeleteAtHead returned false at step %d", i)
		}
		checkInvariants(t, Tree1)
	}
	if Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on drained tree to return false.")
	}

	// Refill and drain from the tail.
	for _, s := range keys {
		Tree1.Insert(TestTreeNode{S: s})
	}
	for i, sortedKey := range slices.Backward(sortedKeys) {
		if x, found := Tree1.FindMax(); !found || x.S != sortedKey {
			t.Fatalf("Drain tail: expected max %s, got %+v", sortedKey, x)
		}
		if !Tree1.DeleteAtTail() {
			t.Fatalf("Drain tail: DeleteAtTail returned false at step %d", i)
		}
		checkInvariants(t, Tree1)
	}
	if Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on drained tree to return false.")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after draining.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Walks: depth parameter, early stop for pre/post order
// -------------------------------------------------------------------------------------------------------

func TestTreeWalkDepth(t *testing.T) {
	Tree1 := newTestTree()
	//        {05}
	//    {02}      {09}
	//  {00} {03}
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	expectedDepth := map[string]int{"05": 0, "02": 1, "09": 1, "00": 2, "03": 2}

	// Caller state (the visit counter) is captured in the closure — the
	// type-safe alternative to an interface{} userData parameter.
	visits := 0
	fx := func(pos, depth int, data TestTreeNode) bool {
		visits++
		if d, ok := expectedDepth[data.S]; !ok {
			t.Errorf("Unexpected key %s in walk", data.S)
		} else if d != depth {
			t.Errorf("Wrong depth for %s: expected %d got %d", data.S, d, depth)
		}
		return true
	}
	Tree1.WalkInOrder(fx)
	Tree1.WalkPreOrder(fx)
	Tree1.WalkPostOrder(fx)
	if visits != 15 {
		t.Errorf("Expected 15 total visits over 3 walks, got %d", visits)
	}

	// Walks on an empty tree must not call the callback at all.
	empty := newTestTree()
	called := false
	noop := func(pos, depth int, data TestTreeNode) bool {
		called = true
		return true
	}
	empty.WalkInOrder(noop)
	empty.WalkPreOrder(noop)
	empty.WalkPostOrder(noop)
	if called {
		t.Errorf("Walk on empty tree called the callback.")
	}
}

func TestTreeWalkEarlyStopPrePost(t *testing.T) {
	Tree1 := newTestTree()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestTreeNode{S: s})
	}

	// Pre-order early stop: only the root is visited.
	var pre []string
	Tree1.WalkPreOrder(func(pos, depth int, data TestTreeNode) bool {
		pre = append(pre, data.S)
		return false
	})
	if expect := []string{"05"}; !reflect.DeepEqual(pre, expect) {
		t.Errorf("PreOrder early stop error, expected %s got %s", expect, pre)
	}

	// Post-order early stop: only the first post-order element is visited.
	var post []string
	Tree1.WalkPostOrder(func(pos, depth int, data TestTreeNode) bool {
		post = append(post, data.S)
		return false
	})
	if expect := []string{"00"}; !reflect.DeepEqual(post, expect) {
		t.Errorf("PostOrder early stop error, expected %s got %s", expect, post)
	}

	// Stop after 3 elements in each order.
	stopAfter := func(order string, walk func(ApplyFunction[TestTreeNode]), expect []string) {
		var got []string
		walk(func(pos, depth int, data TestTreeNode) bool {
			got = append(got, data.S)
			return len(got) < 3
		})
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
	Tree1 := newTestTree()
	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	Tree1.Truncate()
	if !Tree1.IsEmpty() || Tree1.Length() != 0 || Tree1.Depth() != 0 {
		t.Errorf("Expected fully empty tree after Truncate.")
	}
	// Tree must be fully reusable after Truncate.
	for _, s := range []string{"50", "20", "90"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	checkInvariants(t, Tree1)
	if got := inOrder(Tree1); !reflect.DeepEqual(got, []string{"20", "50", "90"}) {
		t.Errorf("Expected [20 50 90] after re-fill, got %v", got)
	}
	// Truncating an already-empty tree is fine.
	empty := newTestTree()
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

	tree := NewBinaryTreeFunc(cmpTestTreeNode)
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
		if got := inOrder(tree); !reflect.DeepEqual(got, keys) {
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
			if _, found := tree.FindMin(); found {
				t.Fatalf("step %d: expected no min on empty tree", step)
			}
			if _, found := tree.FindMax(); found {
				t.Fatalf("step %d: expected no max on empty tree", step)
			}
		} else {
			if x, found := tree.FindMin(); !found || x.S != keys[0] {
				t.Fatalf("step %d: FindMin=%+v, expected %s", step, x, keys[0])
			}
			if x, found := tree.FindMax(); !found || x.S != keys[len(keys)-1] {
				t.Fatalf("step %d: FindMax=%+v, expected %s", step, x, keys[len(keys)-1])
			}
			// Index must agree with the sorted model at every position.
			for i, k := range keys {
				if x, found := tree.Index(i); !found || x.S != k {
					t.Fatalf("step %d: Index(%d)=%+v, expected %s", step, i, x, k)
				}
			}
		}
		checkInvariants(t, tree)
	}

	const maxKey = 60 // small key space so duplicates and deletes are common
	for step := range 800 {
		k := keyOf(rng.Intn(maxKey))
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert (duplicates replace in both tree and model)
			added := tree.Insert(TestTreeNode{S: k})
			if added == model[k] {
				t.Fatalf("step %d: Insert(%s)=%v, model said present=%v", step, k, added, model[k])
			}
			model[k] = true
		case 4, 5, 6: // Delete
			got := tree.Delete(TestTreeNode{S: k})
			if got != model[k] {
				t.Fatalf("step %d: Delete(%s)=%v, model said present=%v", step, k, got, model[k])
			}
			delete(model, k)
		case 7: // Search
			_, found := tree.Search(TestTreeNode{S: k})
			if found != model[k] {
				t.Fatalf("step %d: Search(%s) found=%v, model said present=%v", step, k, found, model[k])
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

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestTreeIterateSnapshot verifies that the Front/All/Backward iterators
// operate on a snapshot taken when they are called: later modifications —
// even truncating the whole tree — are not observed, and mutating the tree
// from inside the loop is safe.
func TestTreeIterateSnapshot(t *testing.T) {
	tree := newTestTree()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		tree.Insert(TestTreeNode{S: s})
	}

	all := tree.All()
	backward := tree.Backward()
	it := tree.Front()

	tree.Truncate() // the iterators above must not observe this

	var got []string
	for v := range all {
		got = append(got, v.S)
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All after Truncate error, expected %s got %s", expect, got)
	}

	var gotB []string
	for v := range backward {
		gotB = append(gotB, v.S)
	}
	if expect := []string{"09", "05", "03", "02", "00"}; !reflect.DeepEqual(gotB, expect) {
		t.Errorf("Backward after Truncate error, expected %s got %s", expect, gotB)
	}

	var gotF []string
	for ; !it.Done(); it.Next() {
		v, _ := it.Value()
		gotF = append(gotF, v.S)
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(gotF, expect) {
		t.Errorf("Front iterator after Truncate error, expected %s got %s", expect, gotF)
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	tree = newTestTree()
	for _, s := range []string{"05", "02", "09"} {
		tree.Insert(TestTreeNode{S: s})
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
// ends up empty and consistent.
func TestTreeConcurrent(t *testing.T) {
	tree := NewBinaryTreeFunc(cmpTestTreeNode)

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
	checkInvariants(t, tree)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperJSONString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the tree.
type upperJSONString string

func (u upperJSONString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperJSONString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperJSONString(s)
	return nil
}

func TestTreeMarshalJSON(t *testing.T) {
	// Exact array output in in-order (ascending) order, whatever the
	// insertion order was.
	tree := NewBinaryTree[int]()
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
	if b, err := json.Marshal(NewBinaryTree[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero BinaryTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *BinaryTree never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilTree *BinaryTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewBinaryTree[upperJSONString]()
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewBinaryTreeFunc(func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a tree of channels.")
	}
}

func TestTreeUnmarshalJSON(t *testing.T) {
	// Decoded elements are inserted in array order and iterate in
	// ascending order.
	tree := NewBinaryTree[int]()
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

	// A round trip rebuilds a sound tree and keeps the comparison
	// function (Search works on the rebuilt tree).
	items := newTestTree()
	for _, s := range []string{"a", "b", "c"} {
		items.Insert(TestTreeNode{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTree()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	if got, want := fmt.Sprint(inOrder(again)), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, found := again.Search(TestTreeNode{S: "b"}); !found {
		t.Errorf("Expected Search to work after unmarshal.")
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte("[7]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := tree.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// Duplicate elements in the data collapse, exactly as repeated Insert
	// calls would.
	dup := newTestTree()
	if err := json.Unmarshal([]byte(`[{"S":"a","N":1},{"S":"a","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if dup.Len() != 1 {
		t.Errorf("Expected duplicates to collapse to 1 element, got %d", dup.Len())
	}
	if x, found := dup.Search(TestTreeNode{S: "a"}); !found || x.N != 2 {
		t.Errorf("Expected the later duplicate to win (N=2), got %+v found=%v", x, found)
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
	checkInvariants(t, full)

	// Element-level unmarshalers are honored.
	custom := NewBinaryTree[upperJSONString]()
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
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(inOrder(keep)), "[keep]"; got != want {
			t.Errorf("Tree changed after the error on %s: %s", badData, got)
		}
	}
	checkInvariants(t, keep)
}

// TestTreeUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestTreeUnmarshalJSONPanics(t *testing.T) {
	var zero BinaryTree[TestTreeNode]
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewBinaryTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilTree *BinaryTree[TestTreeNode]
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

// TestTreeJSONStructField marshals and unmarshals a BinaryTree nested in a
// struct through the encoding/json package.  The tree must be created with
// NewBinaryTree/NewBinaryTreeFunc before unmarshaling: for a nil
// *BinaryTree field the json package allocates a zero-value tree itself
// (no comparison function), so non-empty data panics with the
// insert-family message.
func TestTreeJSONStructField(t *testing.T) {
	type Doc struct {
		Title string              `json:"title"`
		Tags  *BinaryTree[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewBinaryTree[string]()}
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
	out.Tags = NewBinaryTree[string]()
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
	clearDoc := Doc{Title: "x", Tags: NewBinaryTree[string]()}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the tree.")
	}

	// Non-empty data into a nil *BinaryTree field: the json package
	// allocates a zero-value tree, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated tree field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBinaryTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestTreeJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a sorted-slice reference model at fixed seed.
func TestTreeJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run
	const ops = 500

	tree := NewBinaryTree[int]()
	model := make(map[int]bool) // set of values the tree should contain

	sortedModel := func() []int {
		vals := []int{} // non-nil, so an emptied model marshals as [] like the tree
		for v := range model {
			vals = append(vals, v)
		}
		sort.Ints(vals)
		return vals
	}

	for step := range ops {
		v := rng.Intn(100)
		switch rng.Intn(3) {
		case 0, 1: // Insert (duplicates replace in both tree and model)
			tree.Insert(v)
			model[v] = true
		case 2: // Delete
			tree.Delete(v)
			delete(model, v)
		}

		// Marshal must equal the sorted model marshaled as a plain slice.
		want, _ := json.Marshal(sortedModel())
		got, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh tree must reproduce the model.
		fresh := NewBinaryTree[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		var vals []int
		for v := range fresh.All() {
			vals = append(vals, v)
		}
		if fmt.Sprint(vals) != fmt.Sprint(sortedModel()) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, sortedModel())
		}
	}
}

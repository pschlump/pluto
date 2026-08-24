package rb_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// TestRbTreeNode is an Interface Matching data type for the nodes that
// supports the Comparable interface.  This means that it has a Compare
// function.
type TestRbTreeNode struct {
	S string
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestRbTreeNode)(nil)

// Compare implements the Compare function to satisfy the interface
// requirements.
func (aa TestRbTreeNode) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestRbTreeNode); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestRbTreeNode); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

// checkInvariants verifies the red-black properties of the tree:
//
//  1. The root is black.
//  2. No red node has a red child.
//  3. Every root-to-nil-leaf path has the same number of black nodes.
//  4. The tree is a valid BST (in-order traversal is sorted) of the
//     expected size.
//  5. Every child's parent pointer points back at its parent.
func checkInvariants[T comparable.Comparable](t *testing.T, tt *RbTree[T]) {
	t.Helper()

	if tt.root != nil && tt.root.red {
		t.Errorf("root is red")
	}
	if tt.root != nil && tt.root.parent != nil {
		t.Errorf("root has a parent")
	}
	if tt.root == nil && tt.length != 0 {
		t.Errorf("empty root with length %d", tt.length)
	}

	// BST order, size and parent-pointer integrity.
	n := 0
	prev := ""
	var checkParents func(cur, parent *RbTreeNode[T])
	checkParents = func(cur, parent *RbTreeNode[T]) {
		if cur == nil {
			return
		}
		if cur.parent != parent {
			t.Errorf("node %v has wrong parent pointer", *cur.data)
		}
		checkParents(cur.left, cur)
		checkParents(cur.right, cur)
	}
	checkParents(tt.root, nil)
	for v := range tt.All() {
		s := fmt.Sprintf("%v", v)
		if n > 0 && s <= prev {
			t.Errorf("in-order traversal not sorted: %q after %q", s, prev)
		}
		prev = s
		n++
	}
	if n != tt.length {
		t.Errorf("in-order traversal visited %d nodes, length is %d", n, tt.length)
	}

	// Black height and red-red property.
	var blackHeight func(cur *RbTreeNode[T]) int
	blackHeight = func(cur *RbTreeNode[T]) int {
		if cur == nil {
			return 1 // nil leaves are black.
		}
		if cur.red && (isRed(cur.left) || isRed(cur.right)) {
			t.Errorf("red node %v has a red child", *cur.data)
		}
		lb := blackHeight(cur.left)
		rb := blackHeight(cur.right)
		if lb != rb {
			t.Errorf("black height mismatch at node %v: left=%d right=%d", *cur.data, lb, rb)
		}
		if cur.red {
			return lb
		}
		return lb + 1
	}
	blackHeight(tt.root)
}

func TestTreeInsertSearch(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	Tree1.Insert(TestRbTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if Tree1.Length() != 1 {
		t.Errorf("Expected length of 1, got %d", Tree1.Length())
	}

	ptr := Tree1.Search(TestRbTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

	ptr = Tree1.Search(TestRbTreeNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

	Tree1.Insert(TestRbTreeNode{S: "11"})
	Tree1.Insert(TestRbTreeNode{S: "13"})
	Tree1.Insert(TestRbTreeNode{S: "10"})
	for _, s := range []string{"10", "11", "13"} {
		if Tree1.Search(TestRbTreeNode{S: s}) == nil {
			t.Errorf("Expected to find node %s in tree, returned nil instead", s)
		}
	}
	if ptr := Tree1.Search(TestRbTreeNode{S: "14"}); ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}
	if Tree1.Length() != 4 {
		t.Errorf("Expected length of 4, got %d", Tree1.Length())
	}
	checkInvariants(t, &Tree1)
}

func TestTreeInsertDuplicateReplaces(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	Tree1.Insert(TestRbTreeNode{S: "12"})
	Tree1.Insert(TestRbTreeNode{S: "12"})

	if Tree1.Length() != 1 {
		t.Errorf("Expected duplicate insert to replace, length should be 1, got %d", Tree1.Length())
	}
	if ptr := Tree1.Search(TestRbTreeNode{S: "12"}); ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	checkInvariants(t, &Tree1)
}

func TestTreeDelete(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if Tree1.Delete(TestRbTreeNode{S: "12"}) {
		t.Errorf("Expected delete on empty tree to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03", "07"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if Tree1.Delete(TestRbTreeNode{S: "12"}) {
		t.Errorf("Expected delete of absent item to return false")
	}
	if !Tree1.Delete(TestRbTreeNode{S: "00"}) {
		t.Errorf("Expected delete of present item to return true")
	}
	if Tree1.Search(TestRbTreeNode{S: "00"}) != nil {
		t.Errorf("Expected node to be gone after delete")
	}
	if Tree1.Length() != 5 {
		t.Errorf("Expected length of 5, got %d", Tree1.Length())
	}
	if Tree1.Delete(TestRbTreeNode{S: "00"}) {
		t.Errorf("Expected second delete of same item to return false")
	}
	checkInvariants(t, &Tree1)
}

func TestTreeTruncate(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	Tree1.Truncate()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after truncate")
	}
	if Tree1.Length() != 0 {
		t.Errorf("Expected length of 0 after truncate, got %d", Tree1.Length())
	}
	if Tree1.Search(TestRbTreeNode{S: "05"}) != nil {
		t.Errorf("Expected search after truncate to return nil")
	}

	// Tree should still be usable after a truncate.
	Tree1.Insert(TestRbTreeNode{S: "07"})
	if Tree1.Length() != 1 {
		t.Errorf("Expected length of 1 after insert into truncated tree, got %d", Tree1.Length())
	}
	checkInvariants(t, &Tree1)
}

func TestTreeFindMinMax(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if Tree1.FindMin() != nil || Tree1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax on empty tree to return nil")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if mn := Tree1.FindMin(); mn == nil || mn.S != "00" {
		t.Errorf("Expected min of 00, got %+v", mn)
	}
	if mx := Tree1.FindMax(); mx == nil || mx.S != "09" {
		t.Errorf("Expected max of 09, got %+v", mx)
	}
}

func TestTreeDeleteAtHeadTail(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if Tree1.DeleteAtHead() || Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on empty tree to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true")
	}
	if mn := Tree1.FindMin(); mn == nil || mn.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", mn)
	}
	if !Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true")
	}
	if mx := Tree1.FindMax(); mx == nil || mx.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", mx)
	}
	if Tree1.Length() != 3 {
		t.Errorf("Expected length of 3, got %d", Tree1.Length())
	}

	// Drain the tree from both ends.
	if !Tree1.DeleteAtHead() || !Tree1.DeleteAtTail() || !Tree1.DeleteAtHead() {
		t.Errorf("Expected draining deletes to return true")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after draining")
	}
	if Tree1.DeleteAtHead() || Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on drained tree to return false")
	}
	checkInvariants(t, &Tree1)
}

func TestTreeIterators(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	var fwd []string
	for v := range Tree1.All() {
		fwd = append(fwd, v.S)
	}
	want := []string{"00", "02", "03", "05", "09"}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Errorf("All: expected %v, got %v", want, fwd)
	}

	var bwd []string
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	want = []string{"09", "05", "03", "02", "00"}
	if fmt.Sprintf("%v", bwd) != fmt.Sprintf("%v", want) {
		t.Errorf("Backward: expected %v, got %v", want, bwd)
	}

	// Early exit via break, both directions.
	n := 0
	for range Tree1.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break in All to yield exactly 1 item, got %d", n)
	}
	n = 0
	for range Tree1.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break in Backward to yield exactly 1 item, got %d", n)
	}

	// Iterating an empty tree yields nothing.
	var Empty RbTree[TestRbTreeNode]
	for range Empty.All() {
		t.Errorf("Expected no items from All on empty tree")
	}
	for range Empty.Backward() {
		t.Errorf("Expected no items from Backward on empty tree")
	}
}

// TestTreeSortedInsert verifies that inserting in sorted order — the worst
// case for an unbalanced tree — stays balanced and correct.
func TestTreeSortedInsert(t *testing.T) {
	const N = 1000

	var Tree1 RbTree[TestRbTreeNode]
	for i := 0; i < N; i++ {
		Tree1.Insert(TestRbTreeNode{S: fmt.Sprintf("%06d", i)})
	}
	if Tree1.Length() != N {
		t.Fatalf("Expected length of %d, got %d", N, Tree1.Length())
	}
	// A red-black tree with N nodes has depth at most 2*log2(N+1).
	if d := Tree1.Depth(); d > 25 {
		t.Errorf("Expected depth <= 25 after sorted insert of %d nodes, got %d", N, d)
	}
	checkInvariants(t, &Tree1)
}

// TestTreeRandomized inserts a large number of items in random order,
// checking the red-black invariants along the way, then deletes them all in
// a different random order.
func TestTreeRandomized(t *testing.T) {
	const N = 10000

	rng := rand.New(rand.NewPCG(42, 42))
	perm := rng.Perm(N)

	var Tree1 RbTree[TestRbTreeNode]
	for i, n := range perm {
		Tree1.Insert(TestRbTreeNode{S: fmt.Sprintf("%06d", n)})
		if i%1000 == 0 {
			checkInvariants(t, &Tree1)
		}
	}
	if Tree1.Length() != N {
		t.Fatalf("Expected length of %d, got %d", N, Tree1.Length())
	}
	checkInvariants(t, &Tree1)

	// The tree must iterate in sorted order in both directions.
	ref := make([]string, 0, N)
	for v := range Tree1.All() {
		ref = append(ref, v.S)
	}
	if !sort.StringsAreSorted(ref) {
		t.Fatalf("All: items are not in ascending order")
	}
	if len(ref) != N {
		t.Fatalf("All: expected %d items, got %d", N, len(ref))
	}
	if ref[0] != "000000" || ref[N-1] != fmt.Sprintf("%06d", N-1) {
		t.Errorf("All: unexpected endpoints %s ... %s", ref[0], ref[N-1])
	}
	var bwd []string
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	if len(bwd) != N {
		t.Fatalf("Backward: expected %d items, got %d", N, len(bwd))
	}
	for i := range ref {
		if bwd[N-1-i] != ref[i] {
			t.Fatalf("Backward does not mirror All at position %d", i)
		}
	}

	// Spot-check searches, then delete everything in a different random order.
	for i := 0; i < N; i += 997 {
		if Tree1.Search(TestRbTreeNode{S: fmt.Sprintf("%06d", i)}) == nil {
			t.Errorf("Expected to find %06d in tree", i)
		}
	}

	del := rand.New(rand.NewPCG(7, 7)).Perm(N)
	for i, n := range del {
		key := TestRbTreeNode{S: fmt.Sprintf("%06d", n)}
		if !Tree1.Delete(key) {
			t.Fatalf("Delete %d of %s failed", i, key.S)
		}
		if Tree1.Search(key) != nil {
			t.Fatalf("Found %s after delete", key.S)
		}
		if i%1000 == 0 {
			checkInvariants(t, &Tree1)
		}
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting everything, length=%d", Tree1.Length())
	}
	checkInvariants(t, &Tree1)
}

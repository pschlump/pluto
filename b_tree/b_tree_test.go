package b_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"strings"
	"testing"
)

// TestBTreeNode is the test element type.  It has no Compare method and
// implements no interface.  Ordering is supplied to the tree as a plain
// function (cmpTestBTreeNode below).
type TestBTreeNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

// cmpTestBTreeNode orders TestBTreeNode by its S field.
func cmpTestBTreeNode(a, b TestBTreeNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestTree builds a BTree of the given order over TestBTreeNode,
// ordered by S.
func newTestTree(order int) *BTree[TestBTreeNode] {
	return NewBTreeFunc(order, cmpTestBTreeNode)
}

// mkKey makes a key with the given ordinal, zero-padded so that the
// lexicographic string order matches the numeric order.
func mkKey(i int) TestBTreeNode {
	return TestBTreeNode{S: fmt.Sprintf("%08d", i)}
}

func TestTreeInsertSearch(t *testing.T) {

	Tree1 := newTestTree(4)

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after declaration, failed to get one.")
	}

	if !Tree1.Insert(TestBTreeNode{S: "12"}) {
		t.Errorf("Expected Insert of a new node to return true.")
	}

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}

	if item, found := Tree1.Search(TestBTreeNode{S: "12"}); !found {
		t.Errorf("Expected to find node in tree, search returned not-found")
	} else if item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %s", item.S)
	}

	if _, found := Tree1.Search(TestBTreeNode{S: "11"}); found {
		t.Errorf("Expected *NOT* to find node in tree, search found it")
	}

	Tree1.Insert(TestBTreeNode{S: "11"})
	Tree1.Insert(TestBTreeNode{S: "13"})
	Tree1.Insert(TestBTreeNode{S: "10"})
	if _, found := Tree1.Search(TestBTreeNode{S: "10"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestBTreeNode{S: "13"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestBTreeNode{S: "11"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestBTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}

}

func TestTreeInsertWithDupsSearch(t *testing.T) {

	Tree8 := newTestTree(4)

	Tree8.Insert(TestBTreeNode{S: "12"})

	if item, found := Tree8.Search(TestBTreeNode{S: "12"}); !found || item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %v found=%v", item, found)
	}

	Tree8.Insert(TestBTreeNode{S: "11"})
	Tree8.Insert(TestBTreeNode{S: "13"})
	Tree8.Insert(TestBTreeNode{S: "10"})
	// Duplicates replace: the tree must stay consistent.
	if Tree8.Insert(TestBTreeNode{S: "12"}) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if Tree8.Insert(TestBTreeNode{S: "12"}) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if got := Tree8.Length(); got != 4 {
		t.Errorf("Expected length 4 after duplicate inserts, got %d", got)
	}
	for _, s := range []string{"10", "11", "12", "13"} {
		if _, found := Tree8.Search(TestBTreeNode{S: s}); !found {
			t.Errorf("Expected to find node %s in tree, did not.", s)
		}
	}
	if _, found := Tree8.Search(TestBTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}

}

// TestTreeDuplicateReplace verifies that inserting a duplicate replaces
// the stored value and does not change the length, for duplicates found
// both in a leaf and in an internal node.
func TestTreeDuplicateReplace(t *testing.T) {
	tree := newTestTree(3)
	for i := range 50 {
		tree.Insert(mkKey(i))
	}
	if got := tree.Length(); got != 50 {
		t.Fatalf("Expected length 50, got %d", got)
	}

	// Insert duplicates with fresh satellite data; the search probe must
	// return the new value.
	for _, v := range []int{0, 25, 49} {
		if tree.Insert(TestBTreeNode{S: mkKey(v).S, N: 7}) {
			t.Fatalf("Expected duplicate insert of %d to return false", v)
		}
	}
	if got := tree.Length(); got != 50 {
		t.Fatalf("Expected length to stay 50 after duplicate inserts, got %d", got)
	}
	for _, v := range []int{0, 25, 49} {
		if got, found := tree.Search(mkKey(v)); !found || got.N != 7 {
			t.Errorf("Expected duplicate insert to replace stored value for %d, got %+v found=%v", v, got, found)
		}
	}
	checkBTreeInvariants(t, tree)
}

func TestTreeTruncate(t *testing.T) {

	Tree1 := newTestTree(4)

	Tree1.Insert(TestBTreeNode{S: "05"})
	Tree1.Insert(TestBTreeNode{S: "02"})
	Tree1.Insert(TestBTreeNode{S: "09"})
	Tree1.Insert(TestBTreeNode{S: "00"})
	Tree1.Insert(TestBTreeNode{S: "03"})
	Tree1.Truncate()
	if Tree1.Length() != 0 {
		t.Errorf("Expected empty tree")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected IsEmpty after Truncate")
	}

	// The tree remains usable: the comparison function was kept.
	Tree1.Insert(TestBTreeNode{S: "42"})
	if got := Tree1.Length(); got != 1 {
		t.Errorf("Expected length 1 after re-insert, got %d", got)
	}

}

// test deleting node from tree.  This is a set of tests on .Delete() that
// works through the basic configurations of trees.
func TestTreeDelete(t *testing.T) {

	Tree1 := newTestTree(4)

	// Delete from Empty tree
	found := Tree1.Delete(TestBTreeNode{S: "05"})
	if found == true {
		t.Errorf("Found node in empty tree.")
	}

	// Root-Test: Delete from tree with a single root node.
	Tree1.Insert(TestBTreeNode{S: "05"})
	found = Tree1.Delete(TestBTreeNode{S: "05"})
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 0 {
		t.Errorf("Expected to empty tree got, %d", size)
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting the only element.")
	}

	// Delete one of several keys sitting in a single (leaf) root.
	Tree1.Insert(TestBTreeNode{S: "05"})
	Tree1.Insert(TestBTreeNode{S: "03"})
	found = Tree1.Delete(TestBTreeNode{S: "05"})
	if !found {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
	}
	if _, found := Tree1.Search(TestBTreeNode{S: "03"}); !found {
		t.Errorf("Expected 03 to still be in the tree.")
	}

	// Delete a key that is not present.
	if Tree1.Delete(TestBTreeNode{S: "99"}) {
		t.Errorf("Expected false from Delete of a missing key.")
	}
}

func TestTreeMinMax(t *testing.T) {
	Tree1 := newTestTree(4)

	Tree1.Insert(TestBTreeNode{S: "05"})
	Tree1.Insert(TestBTreeNode{S: "02"})
	Tree1.Insert(TestBTreeNode{S: "09"})
	Tree1.Insert(TestBTreeNode{S: "00"})
	Tree1.Insert(TestBTreeNode{S: "03"})

	if x, found := Tree1.FindMax(); !found || x.S != "09" {
		t.Errorf("Unexpected Max, got %+v found=%v", x, found)
	}

	if x, found := Tree1.FindMin(); !found || x.S != "00" {
		t.Errorf("Unexpected Min, got %+v found=%v", x, found)
	}
}

func TestTreeDepth(t *testing.T) {
	Tree1 := newTestTree(4)

	if got := Tree1.Depth(); got != 0 {
		t.Errorf("Expected depth 0 on empty tree, got %d", got)
	}

	Tree1.Insert(TestBTreeNode{S: "05"})
	if got := Tree1.Depth(); got != 1 {
		t.Errorf("Expected depth 1 on single-element tree, got %d", got)
	}

	// A B-tree of order 4 holds 1000 keys in at most
	// ceil(log2(1001)) levels; sorted input stays just as shallow.
	for i := range 1000 {
		Tree1.Insert(mkKey(i + 1))
	}
	if d := Tree1.Depth(); d > 10 {
		t.Errorf("Expected shallow depth for 1000 sorted inserts, got %d", d)
	}
	checkBTreeInvariants(t, Tree1)
}

func TestTreeDeleteAtTail(t *testing.T) {
	Tree1 := newTestTree(4)

	Tree1.Insert(TestBTreeNode{S: "05"})
	Tree1.Insert(TestBTreeNode{S: "02"})
	Tree1.Insert(TestBTreeNode{S: "09"})
	Tree1.Insert(TestBTreeNode{S: "00"})
	Tree1.Insert(TestBTreeNode{S: "03"})

	found := Tree1.DeleteAtTail()
	if !found {
		t.Errorf("Expected DeleteAtTail to find a node, did not.")
	}
	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
	if x, found := Tree1.FindMax(); !found || x.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", x)
	}
}

func TestTreeDeleteAtHead(t *testing.T) {
	Tree1 := newTestTree(4)

	Tree1.Insert(TestBTreeNode{S: "05"})
	Tree1.Insert(TestBTreeNode{S: "02"})
	Tree1.Insert(TestBTreeNode{S: "09"})
	Tree1.Insert(TestBTreeNode{S: "00"})
	Tree1.Insert(TestBTreeNode{S: "03"})

	found := Tree1.DeleteAtHead()
	if !found {
		t.Errorf("Expected DeleteAtHead to find a node, did not.")
	}
	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
	if x, found := Tree1.FindMin(); !found || x.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", x)
	}
}

// TestTreeSingleElement covers operations on a tree holding exactly one
// item.
func TestTreeSingleElement(t *testing.T) {
	tree := newTestTree(4)
	tree.Insert(mkKey(7))

	if tree.IsEmpty() {
		t.Errorf("Expected non-empty tree")
	}
	if got := tree.Length(); got != 1 {
		t.Errorf("Expected length 1, got %d", got)
	}
	if got, found := tree.FindMin(); !found || got.S != mkKey(7).S {
		t.Errorf("FindMin on single-element tree: got %v", got)
	}
	if got, found := tree.FindMax(); !found || got.S != mkKey(7).S {
		t.Errorf("FindMax on single-element tree: got %v", got)
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

// TestTreeIterators covers the range-over-func iterators: ascending and
// descending order, the index that a single-variable range yields, and
// early termination.
func TestTreeIterators(t *testing.T) {
	tree := newTestTree(4)
	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	expect := []string{mkKey(0).S, mkKey(2).S, mkKey(3).S, mkKey(5).S, mkKey(9).S}

	// Range-over-func forward: index and value.
	var got []string
	for i, item := range tree.All() {
		if i != len(got) {
			t.Errorf("All(): expected index %d, got %d", len(got), i)
		}
		got = append(got, item.S)
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", expect) {
		t.Errorf("All() iteration: expected %v got %v", expect, got)
	}

	// Range-over-func backward.
	var rev []string
	for i, item := range tree.Backward() {
		if i != len(rev) {
			t.Errorf("Backward(): expected index %d, got %d", len(rev), i)
		}
		rev = append(rev, item.S)
	}
	for i := range expect {
		if rev[i] != expect[len(expect)-1-i] {
			t.Fatalf("Backward() iteration: expected reverse of %v got %v", expect, rev)
		}
	}

	// A single-variable range yields the INDEX.
	var indices []int
	for i := range tree.All() {
		indices = append(indices, i)
	}
	if fmt.Sprintf("%v", indices) != fmt.Sprintf("%v", []int{0, 1, 2, 3, 4}) {
		t.Errorf("All() single-variable range: expected indices [0 1 2 3 4], got %v", indices)
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
}

func TestTreeEmptyOps(t *testing.T) {
	tree := newTestTree(4)
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
	for range tree.All() {
		t.Errorf("Expected no iterations from All() on empty tree")
	}
	for range tree.Backward() {
		t.Errorf("Expected no iterations from Backward() on empty tree")
	}
	// Truncate on an empty tree must be a no-op.
	tree.Truncate()
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree after no-op Truncate")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewBTree.
func TestCompare(t *testing.T) {
	if c := Compare(1, 2); c != -1 {
		t.Errorf("Compare(1,2) = %d, expected -1", c)
	}
	if c := Compare(2, 1); c != 1 {
		t.Errorf("Compare(2,1) = %d, expected +1", c)
	}
	if c := Compare(1, 1); c != 0 {
		t.Errorf("Compare(1,1) = %d, expected 0", c)
	}
	if c := Compare("abc", "abd"); c != -1 {
		t.Errorf("Compare(abc,abd) = %d, expected -1", c)
	}
	if c := Compare("b", "a"); c != 1 {
		t.Errorf("Compare(b,a) = %d, expected +1", c)
	}
	if c := Compare("a", "a"); c != 0 {
		t.Errorf("Compare(a,a) = %d, expected 0", c)
	}
	if c := Compare(2.5, 2.25); c != 1 {
		t.Errorf("Compare(2.5,2.25) = %d, expected +1", c)
	}
}

// TestNewBTreeOrdered verifies the constructor for naturally ordered key
// types and the balance guarantee: sorted input still produces a shallow
// tree.
func TestNewBTreeOrdered(t *testing.T) {
	ints := NewBTree[int](4)
	for i := range 100 {
		if !ints.Insert(i) {
			t.Errorf("Expected Insert of %d to return true.", i)
		}
	}
	// 100 keys inserted in sorted order: an unbalanced tree would be a
	// chain of depth 100; an order-4 B-tree must be far shallower.
	if d := ints.Depth(); d > 7 {
		t.Errorf("Expected balanced depth <= 7 for 100 sorted inserts, got %d", d)
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 99 {
		t.Errorf("Expected max 99, got %d found=%v", x, found)
	}

	strs := NewBTree[string](3)
	for _, s := range []string{"pear", "apple", "fig"} {
		strs.Insert(s)
	}
	if x, found := strs.FindMin(); !found || x != "apple" {
		t.Errorf("Expected min apple, got %q found=%v", x, found)
	}
	if x, found := strs.FindMax(); !found || x != "pear" {
		t.Errorf("Expected max pear, got %q found=%v", x, found)
	}
}

// TestNewBTreeFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order
// of the struct.
func TestNewBTreeFunc(t *testing.T) {
	tree := NewBTreeFunc(5, func(a, b TestBTreeNode) int {
		return a.N - b.N
	})
	for _, n := range []TestBTreeNode{{S: "five", N: 5}, {S: "two", N: 2}, {S: "nine", N: 9}} {
		tree.Insert(n)
	}
	if x, found := tree.FindMin(); !found || x.S != "two" {
		t.Errorf("Expected min two, got %+v found=%v", x, found)
	}
	var got []string
	for _, v := range tree.All() {
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", []string{"two", "five", "nine"}) {
		t.Errorf("NewBTreeFunc order error, expected %v got %v", []string{"two", "five", "nine"}, got)
	}
}

// TestNewBTreeFuncNil verifies that a nil comparison function is rejected
// at construction time, not on first use.
func TestNewBTreeFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewBTreeFunc(4, nil) to panic.")
		}
	}()
	NewBTreeFunc[TestBTreeNode](4, nil)
}

// TestNewBTreeBadOrder verifies that an order below 3 is rejected at
// construction time by both constructors, with a message naming the fix.
func TestNewBTreeBadOrder(t *testing.T) {
	for _, order := range []int{-1, 0, 1, 2} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewBTree(%d) to panic.", order)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "order") {
					t.Errorf("NewBTree(%d): unexpected panic message: %v", order, r)
				}
			}()
			NewBTree[int](order)
		}()
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected NewBTreeFunc(%d, ...) to panic.", order)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "order") {
					t.Errorf("NewBTreeFunc(%d, ...): unexpected panic message: %v", order, r)
				}
			}()
			NewBTreeFunc(order, cmpTestBTreeNode)
		}()
	}
}

// TestZeroValueTree verifies that the zero value of BTree behaves as an
// empty tree for every non-ordering operation and that Insert fails
// loudly because no comparison function has been set.
func TestZeroValueTree(t *testing.T) {
	var tree BTree[TestBTreeNode]

	if !tree.IsEmpty() {
		t.Errorf("Expected zero value tree to be empty.")
	}
	if tree.Len() != 0 || tree.Length() != 0 {
		t.Errorf("Expected zero value tree to have length 0.")
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected zero value tree to have depth 0.")
	}
	tree.Truncate() // must not panic
	if _, found := tree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on zero value tree.")
	}
	if _, found := tree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on zero value tree.")
	}
	if _, found := tree.Search(mkKey(1)); found {
		t.Errorf("Expected not-found from Search on zero value tree.")
	}
	if tree.Delete(mkKey(1)) {
		t.Errorf("Expected false from Delete on zero value tree.")
	}
	if tree.DeleteAtHead() || tree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on zero value tree.")
	}
	for range tree.All() {
		t.Errorf("Expected no values from All on zero value tree.")
	}
	for range tree.Backward() {
		t.Errorf("Expected no values from Backward on zero value tree.")
	}

	// Insert without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero value tree to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		tree.Insert(mkKey(1))
	}()
}

// TestTreeNilPanics verifies the documented panic when Insert is called
// on a nil tree — the one operation with no sane answer.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *BTree[TestBTreeNode]
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
	var nilTree *BTree[TestBTreeNode]
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
	nilTree.Truncate() // no-op, must not panic

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
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

func BenchmarkInsert(b *testing.B) {
	tree := newTestTree(8)
	keys := make([]TestBTreeNode, b.N)
	for i := range keys {
		keys[i] = mkKey(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(keys[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	const size = 1000
	tree := newTestTree(8)
	keys := make([]TestBTreeNode, size)
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
	const size = 1000
	tree := newTestTree(8)
	keys := make([]TestBTreeNode, size)
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

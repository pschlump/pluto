package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestTreeNode is the test element type.  It implements no interface: no
// Compare method, no interface assertion, no type assertions inside a
// comparison.  Ordering is supplied to the tree as a plain function
// (cmpTestTreeNode below).
type TestTreeNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

func NewTestTree() TestTreeNode {
	return TestTreeNode{}
}

// cmpTestTreeNode orders TestTreeNode by its S field.
func cmpTestTreeNode(a, b TestTreeNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestTree builds a tree of TestTreeNode ordered by S.
func newTestTree() *BinaryTree[TestTreeNode] {
	return NewBinaryTreeFunc(cmpTestTreeNode)
}

func TestTreeInsertSearch(t *testing.T) {

	// Verify we can create a node.
	ANode := NewTestTree()
	_ = ANode

	Tree1 := newTestTree()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after declaration, failed to get one.")
	}

	v1 := Tree1.Insert(TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if v1 == false {
		t.Errorf("Expected to insert new node, got back false for new.")
	}

	v1 = Tree1.Insert(TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if v1 == true {
		t.Errorf("Expected to insert duplicate node, got back false for new.")
	}
	if Tree1.Len() != 1 {
		t.Errorf("Expected 1 node in tree, got %d", Tree1.Len())
	}

	if item, found := Tree1.Search(TestTreeNode{S: "12"}); !found {
		t.Errorf("Expected to find node in tree, search returned not-found")
	} else if item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %s", item.S)
	}

	if _, found := Tree1.Search(TestTreeNode{S: "11"}); found {
		t.Errorf("Expected *NOT* to find node in tree, search found it")
	}

	Tree1.Insert(TestTreeNode{S: "11"})
	Tree1.Insert(TestTreeNode{S: "13"})
	Tree1.Insert(TestTreeNode{S: "10"})
	if _, found := Tree1.Search(TestTreeNode{S: "10"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "13"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "11"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}

}

// Test tree truncate, verify tree empty after build.
func TestTreeTruncate(t *testing.T) {

	Tree1 := newTestTree()

	// Build this tree:
	//			{00}
	//		{02}
	//			{03}
	//	{05}
	//		{09}
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	Tree1.Truncate()
	if Tree1.Length() != 0 {
		t.Errorf("Expected empty tree")
	}

}

// test deleting node from tree.  This is a set of tests on .Delete() that tries
// works through all possible configurations of trees.
func TestTreeDelete(t *testing.T) {

	Tree1 := newTestTree()

	// Build this tree (eventually):
	//			{00}
	//		{02}
	//			{03}
	//	{05}
	//		{09}

	// -------------------------------------------------------------------------------
	// Delete from Empty tree
	found := Tree1.Delete(TestTreeNode{S: "05"}) // Delete called on empty tree.
	if found == true {
		t.Errorf("Found node in empty tree.")
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a single root node.
	Tree1.Insert(TestTreeNode{S: "05"})
	found = Tree1.Delete(TestTreeNode{S: "05"}) // Delete leaf (Only Node in tree)
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 0 {
		t.Errorf("Expected to empty tree got, %d", size)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a left sub-tree
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a right sub-tree
	Tree1.Truncate() // This tests tree.Truncate() also.
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	found = Tree1.Delete(TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete root node with 2 sub trees.
	Tree1.Truncate()
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"}) // Delete Tree with left and right children.
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size)
	}
	// Should have a tree that looks like *(left is highter up)*
	//		{03}
	//	{08}

	// -------------------------------------------------------------------------------
	// Mid-Leaf Test:

	// -------------------------------------------------------------------------------
	// Original Delete test.

	Tree1.Truncate()
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	found = Tree1.Delete(TestTreeNode{S: "03"}) // Delete leaf
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 4 {
		t.Errorf("Expected to tree contain 4 nodes got, %d", size)
	}

	found = Tree1.Delete(TestTreeNode{S: "02"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 3 {
		t.Errorf("Expected to tree contain 3 nodes got, %d", size)
	}

	found = Tree1.Delete(TestTreeNode{S: "00"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size)
	}

	found = Tree1.Delete(TestTreeNode{S: "09"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 nodes got, %d", size)
	}
}

// TestTreeDeleteKeepsSubtree is a regression test: deleting a node with two
// children must not silently drop the left subtree of the replacement node.
func TestTreeDeleteKeepsSubtree(t *testing.T) {

	Tree1 := newTestTree()

	// Build this tree:
	//	{05}
	//		{09}
	//	    {07}
	//	      {08}
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "07"})
	Tree1.Insert(TestTreeNode{S: "08"})

	found := Tree1.Delete(TestTreeNode{S: "05"})
	if !found {
		t.Fatalf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 3 {
		t.Fatalf("Expected tree to contain 3 nodes got, %d", size)
	}
	for _, s := range []string{"07", "08", "09"} {
		if _, found := Tree1.Search(TestTreeNode{S: s}); !found {
			t.Errorf("Expected to find node %s in tree after delete, did not.", s)
		}
	}
	if x, found := Tree1.FindMin(); !found || x.S != "07" {
		t.Errorf("Expected min of 07 after delete, got %+v", x)
	}

	// Also verify the whole in-order traversal is intact.
	var got []string
	Tree1.WalkInOrder(func(pos, depth int, data TestTreeNode) bool {
		got = append(got, data.S)
		return true
	})
	if expect := []string{"07", "08", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder error, expected %s got %s", expect, got)
	}
}

func TestTreeDeleteMatch(t *testing.T) {

	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})

	// A caller supplied comparison function, independent of the tree's own.
	cmp := func(a, b TestTreeNode) int {
		return strings.Compare(a.S, b.S)
	}

	found := Tree1.DeleteMatch(TestTreeNode{S: "02"}, cmp)
	if !found {
		t.Errorf("Expected DeleteMatch to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected tree to contain 2 nodes got, %d", size)
	}
	if _, found := Tree1.Search(TestTreeNode{S: "02"}); found {
		t.Errorf("Expected node 02 to be gone after DeleteMatch.")
	}

	found = Tree1.DeleteMatch(TestTreeNode{S: "77"}, cmp)
	if found {
		t.Errorf("Expected DeleteMatch to not find a node, but it did.")
	}
}

func TestTreeSetGetData(t *testing.T) {
	el := &BinaryTreeElement[TestTreeNode]{}
	el.SetData(TestTreeNode{S: "42"})
	if d := el.GetData(); d.S != "42" {
		t.Errorf("Expected GetData to return 42, got %+v", d)
	}
}

func TestTreeMinMax(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	if x, found := Tree1.FindMax(); !found || x.S != "09" {
		t.Errorf("Unexpected Max, got %+v found=%v", x, found)
	}

	if x, found := Tree1.FindMin(); !found || x.S != "00" {
		t.Errorf("Unexpected Min, got %+v found=%v", x, found)
	}
}

func TestTreeDepth(t *testing.T) {
	Tree1 := newTestTree()

	if n := Tree1.Depth(); n != 0 {
		t.Errorf("Unexpected Depth for empty tree, got %d expected 0", n)
	}

	Tree1.Insert(TestTreeNode{S: "05"})
	if n := Tree1.Depth(); n != 1 {
		t.Errorf("Unexpected Depth for root-only tree, got %d expected 1", n)
	}

	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	n := Tree1.Depth()
	if n != 3 {
		t.Errorf("Unexpected Depth, got %d expected 3", n)
	}
}

func TestTreeIndex(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	for pos, expect := range []string{"00", "02", "03", "05", "09"} {
		x, found := Tree1.Index(pos)
		if !found {
			t.Errorf("Error, not found returned for %d index", pos)
		} else if x.S != expect {
			t.Errorf("Error, expected ->%s<- got ->%s<-", expect, x.S)
		}
	}

	if _, found := Tree1.Index(-1); found {
		t.Errorf("Error, expected not-found for -1 index")
	}
	if _, found := Tree1.Index(5); found {
		t.Errorf("Error, expected not-found for out of range index")
	}
}

func TestTreeReverse(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	Tree1.Reverse()

	if size := Tree1.Length(); size != 5 {
		t.Errorf("Error")
	}

	// After a Reverse the in-order walk is the reverse of the original.
	var got []string
	Tree1.WalkInOrder(func(pos, depth int, data TestTreeNode) bool {
		got = append(got, data.S)
		return true
	})
	if expect := []string{"09", "05", "03", "02", "00"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder after Reverse error, expected %s got %s", expect, got)
	}

	// Reversing twice restores the original order.
	Tree1.Reverse()
	got = got[:0]
	Tree1.WalkInOrder(func(pos, depth int, data TestTreeNode) bool {
		got = append(got, data.S)
		return true
	})
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder after double Reverse error, expected %s got %s", expect, got)
	}
}

func TestTreeDeleteAtTail(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

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
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

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

// TestTreeEmptyOps verifies that operations on an empty tree behave sanely.
func TestTreeEmptyOps(t *testing.T) {
	Tree1 := newTestTree()

	if _, found := Tree1.Search(TestTreeNode{S: "05"}); found {
		t.Errorf("Expected not-found from Search on empty tree.")
	}
	if _, found := Tree1.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on empty tree.")
	}
	if _, found := Tree1.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on empty tree.")
	}
	if Tree1.DeleteAtHead() {
		t.Errorf("Expected false from DeleteAtHead on empty tree.")
	}
	if Tree1.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtTail on empty tree.")
	}
	if _, found := Tree1.Index(0); found {
		t.Errorf("Expected not-found from Index on empty tree.")
	}
	if Tree1.Delete(TestTreeNode{S: "05"}) {
		t.Errorf("Expected false from Delete on empty tree.")
	}
	if n := 0; Tree1.Len() != n || Tree1.Length() != n {
		t.Errorf("Expected 0 length on empty tree.")
	}
}

func TestTreeWalkInOrder(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	var pos []int
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		pos = append(pos, p)
		return true
	}
	Tree1.WalkInOrder(fx)

	//	Output: [00 02 03 05 09]
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("InOrder error, expected %s got %s", expect, x)
	}
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("InOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkPreOrder(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	var pos []int
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		pos = append(pos, p)
		return true
	}
	Tree1.WalkPreOrder(fx)

	// PreOrder Output: [05 02 00 03 09]
	expect := []string{"05", "02", "00", "03", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PreOrder error, expected %s got %s", expect, x)
	}
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("PreOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkPostOrder(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	var pos []int
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		pos = append(pos, p)
		return true
	}
	Tree1.WalkPostOrder(fx)

	// PostOrder Output: [00 03 02 09 05]
	expect := []string{"00", "03", "02", "09", "05"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PostOrder error, expected %s got %s", expect, x)
	}
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("PostOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkEarlyStop(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})

	// Returning false from the callback must stop the walk.
	var x []string
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		return false
	}
	Tree1.WalkInOrder(fx)
	if expect := []string{"02"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("InOrder early stop error, expected %s got %s", expect, x)
	}
}

// TestTreeWalkClosureCapture demonstrates closures replacing an
// interface{} userData parameter: the callback captures caller state
// (count, sumLen) directly and it keeps its static type.
func TestTreeWalkClosureCapture(t *testing.T) {
	Tree1 := newTestTree()

	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(TestTreeNode{S: s})
	}

	count := 0
	sumLen := 0
	Tree1.WalkInOrder(func(pos, depth int, data TestTreeNode) bool {
		count++
		sumLen += len(data.S)
		return true
	})
	if count != 3 {
		t.Errorf("Expected the callback to be called 3 times, got %d", count)
	}
	if sumLen != 6 {
		t.Errorf("Expected the callback to see 6 characters total, got %d", sumLen)
	}
}

func TestTreeWalkFunc(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	Tree1.WalkFunc(func(a TestTreeNode) {
		x = append(x, a.S)
	})

	// WalkFunc is pre-order: [05 02 00 03 09]
	expect := []string{"05", "02", "00", "03", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("WalkFunc error, expected %s got %s", expect, x)
	}

	// WalkFunc on an empty tree must not call Fx.
	empty := newTestTree()
	called := false
	empty.WalkFunc(func(a TestTreeNode) {
		called = true
	})
	if called {
		t.Errorf("WalkFunc on empty tree called the callback.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewBinaryTree.
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

// TestNewBinaryTreeOrdered verifies the constructor for naturally ordered
// key types: integers, floats and strings ordered by their own operators,
// with no comparison function supplied.
func TestNewBinaryTreeOrdered(t *testing.T) {
	ints := NewBinaryTree[int]()
	for _, v := range []int{5, 2, 9, 0, 3} {
		if !ints.Insert(v) {
			t.Errorf("Expected Insert of %d to return true.", v)
		}
	}
	var gotInts []int
	for v := range ints.All() {
		gotInts = append(gotInts, v)
	}
	if expect := []int{0, 2, 3, 5, 9}; !reflect.DeepEqual(gotInts, expect) {
		t.Errorf("NewBinaryTree[int] order error, expected %v got %v", expect, gotInts)
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 9 {
		t.Errorf("Expected max 9, got %d found=%v", x, found)
	}

	strs := NewBinaryTree[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		strs.Insert(s)
	}
	if x, found := strs.FindMin(); !found || x != "apple" {
		t.Errorf("Expected min apple, got %q found=%v", x, found)
	}
	if x, found := strs.FindMax(); !found || x != "pear" {
		t.Errorf("Expected max pear, got %q found=%v", x, found)
	}

	floats := NewBinaryTree[float64]()
	for _, f := range []float64{2.5, 1.5, 3.5} {
		floats.Insert(f)
	}
	if x, found := floats.FindMin(); !found || x != 1.5 {
		t.Errorf("Expected min 1.5, got %v found=%v", x, found)
	}

	// Duplicates replace: 5 replaces 5, length stays.
	if ints.Insert(5) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if ints.Length() != 5 {
		t.Errorf("Expected length 5 after duplicate, got %d", ints.Length())
	}
}

// TestNewBinaryTreeFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order of
// the struct.
func TestNewBinaryTreeFunc(t *testing.T) {
	// Order TestTreeNode by N instead of S.
	tree := NewBinaryTreeFunc(func(a, b TestTreeNode) int {
		return a.N - b.N
	})
	for _, n := range []TestTreeNode{{S: "five", N: 5}, {S: "two", N: 2}, {S: "nine", N: 9}} {
		tree.Insert(n)
	}
	if x, found := tree.FindMin(); !found || x.S != "two" {
		t.Errorf("Expected min two, got %+v found=%v", x, found)
	}
	var got []string
	for v := range tree.All() {
		got = append(got, v.S)
	}
	if expect := []string{"two", "five", "nine"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("NewBinaryTreeFunc order error, expected %v got %v", expect, got)
	}
}

// TestNewBinaryTreeFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewBinaryTreeFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewBinaryTreeFunc(nil) to panic.")
		}
	}()
	NewBinaryTreeFunc[TestTreeNode](nil)
}

// TestZeroValueTree verifies that the zero value of BinaryTree behaves as
// an empty tree for every non-ordering operation and that Insert fails
// loudly because no comparison function has been set.
func TestZeroValueTree(t *testing.T) {
	var tree BinaryTree[TestTreeNode]

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
	if _, found := tree.Index(0); found {
		t.Errorf("Expected not-found from Index on zero value tree.")
	}
	if _, found := tree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on zero value tree.")
	}
	if _, found := tree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on zero value tree.")
	}
	if _, found := tree.Search(TestTreeNode{S: "05"}); found {
		t.Errorf("Expected not-found from Search on zero value tree.")
	}
	if tree.Delete(TestTreeNode{S: "05"}) {
		t.Errorf("Expected false from Delete on zero value tree.")
	}
	if tree.DeleteAtHead() || tree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on zero value tree.")
	}
	called := false
	tree.WalkInOrder(func(pos, depth int, data TestTreeNode) bool {
		called = true
		return true
	})
	if called {
		t.Errorf("Expected no walk visits on zero value tree.")
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBinaryTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		tree.Insert(TestTreeNode{S: "05"})
	}()
}

// -------------------------------------------------------------------------------------------------------
// Iterators
// -------------------------------------------------------------------------------------------------------

func TestTreeOldStyleIter(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	for it := Tree1.Front(); !it.Done(); it.Next() {
		v, found := it.Value()
		if !found {
			t.Fatalf("Value not found while Done() is false.")
		}
		x = append(x, v.S)
	}
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("Iterator error, expected %s got %s", expect, x)
	}

	// Value after Done must report not-found.
	it := Tree1.Front()
	for !it.Done() {
		it.Next()
	}
	if _, found := it.Value(); found {
		t.Errorf("Expected no Value after Done.")
	}

	// Empty tree: iterator is Done immediately.
	empty := newTestTree()
	if it := empty.Front(); !it.Done() {
		t.Errorf("Expected iterator on empty tree to be Done immediately.")
	}
}

func TestTreeAll(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	for v := range Tree1.All() {
		x = append(x, v.S)
	}
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("All error, expected %s got %s", expect, x)
	}

	// Early break must stop the iteration.
	x = x[:0]
	for v := range Tree1.All() {
		x = append(x, v.S)
		if v.S == "02" {
			break
		}
	}
	if expect := []string{"00", "02"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("All early break error, expected %s got %s", expect, x)
	}

	// Empty tree yields nothing.
	empty := newTestTree()
	for range empty.All() {
		t.Errorf("All on empty tree yielded a value.")
	}
}

func TestTreeBackward(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	for v := range Tree1.Backward() {
		x = append(x, v.S)
	}
	expect := []string{"09", "05", "03", "02", "00"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("Backward error, expected %s got %s", expect, x)
	}

	// Early break must stop the iteration.
	x = x[:0]
	for v := range Tree1.Backward() {
		x = append(x, v.S)
		if v.S == "05" {
			break
		}
	}
	if expect := []string{"09", "05"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("Backward early break error, expected %s got %s", expect, x)
	}

	// Empty tree yields nothing.
	empty := newTestTree()
	for range empty.Backward() {
		t.Errorf("Backward on empty tree yielded a value.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkTreeSize = 1000

func BenchmarkInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tree := newTestTree()
		for j := range benchmarkTreeSize {
			tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	tree := newTestTree()
	for j := range benchmarkTreeSize {
		tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	find := TestTreeNode{S: fmt.Sprintf("%06d", benchmarkTreeSize/2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(find)
	}
}

func BenchmarkDelete(b *testing.B) {
	tree := newTestTree()
	for j := range benchmarkTreeSize {
		tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range benchmarkTreeSize {
			tree.Delete(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
		for j := range benchmarkTreeSize {
			tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

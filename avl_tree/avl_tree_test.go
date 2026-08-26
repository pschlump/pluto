package avl_tree

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

// TestTreeNode is the test element type.  Note what is missing compared
// to the pluto version of this test file: no Compare method, no interface
// assertion, no type assertions inside a comparison.  Ordering is supplied
// to the tree as a plain function (cmpTestTreeNode below).
type TestTreeNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

// cmpTestTreeNode orders TestTreeNode by its S field.
func cmpTestTreeNode(a, b TestTreeNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestTree builds an AvlTree of TestTreeNode ordered by S.
func newTestTree() *AvlTree[TestTreeNode] {
	return NewAvlTreeFunc(cmpTestTreeNode)
}

// mkKey makes a key with the given ordinal, zero-padded so that the
// lexicographic string order matches the numeric order.
func mkKey(i int) TestTreeNode {
	return TestTreeNode{S: fmt.Sprintf("%08d", i)}
}

func TestTreeInsertSearch(t *testing.T) {

	Tree1 := newTestTree()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after declaration, failed to get one.")
	}

	if !Tree1.Insert(TestTreeNode{S: "12"}) {
		t.Errorf("Expected Insert of a new node to return true.")
	}

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
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

func TestTreeInsertWithDupsSearch(t *testing.T) {

	Tree8 := newTestTree()

	Tree8.Insert(TestTreeNode{S: "12"})

	if item, found := Tree8.Search(TestTreeNode{S: "12"}); !found || item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %v found=%v", item, found)
	}

	Tree8.Insert(TestTreeNode{S: "11"})
	Tree8.Insert(TestTreeNode{S: "13"})
	Tree8.Insert(TestTreeNode{S: "10"})
	// Duplicates replace: the tree must stay consistent.
	if Tree8.Insert(TestTreeNode{S: "12"}) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if Tree8.Insert(TestTreeNode{S: "12"}) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if got := Tree8.Length(); got != 4 {
		t.Errorf("Expected length 4 after duplicate inserts, got %d", got)
	}
	for _, s := range []string{"10", "11", "12", "13"} {
		if _, found := Tree8.Search(TestTreeNode{S: s}); !found {
			t.Errorf("Expected to find node %s in tree, did not.", s)
		}
	}
	if _, found := Tree8.Search(TestTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}

}

// TEST TODO: func (tt *AvlTree[T]) Truncate()  {
func TestTreeTruncate(t *testing.T) {

	Tree1 := newTestTree()

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

// test deleting node from tree.  This is a set of tests on .Delete() that
// tries works through all possible configurations of trees.
func TestTreeDelete(t *testing.T) {

	Tree1 := newTestTree()

	// Delete from Empty tree
	found := Tree1.Delete(TestTreeNode{S: "05"})
	if found == true {
		t.Errorf("Found node in empty tree.")
	}

	// Root-Test: Delete from tree with a single root node.
	Tree1.Insert(TestTreeNode{S: "05"})
	found = Tree1.Delete(TestTreeNode{S: "05"})
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 0 {
		t.Errorf("Expected to empty tree got, %d", size)
	}

	// Root-Test: Delete from tree with a root node and a left sub-tree
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"})
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
	}

	// Root-Test: Delete from tree with a root node and a right sub-tree
	Tree1.Truncate()
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	found = Tree1.Delete(TestTreeNode{S: "05"})
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
	}

	// Root-Test: Delete root node with 2 sub trees.
	Tree1.Truncate()
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"})
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size)
	}

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

	Tree1.Insert(TestTreeNode{S: "05"})
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
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPreOrder(fx)

	// PreOrder Output: [05 02 00 03 09]
	expect := []string{"05", "02", "00", "03", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PreOrder error, expected %s got %s", expect, x)
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
	fx := func(p, depth int, data TestTreeNode) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPostOrder(fx)

	// PostOrder Output: [00 03 02 09 05]
	expect := []string{"00", "03", "02", "09", "05"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PostOrder error, expected %s got %s", expect, x)
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewAvlTree.
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

// TestNewAvlTreeOrdered verifies the constructor for naturally ordered key
// types and the AVL balance guarantee: sorted input still produces a
// shallow tree.
func TestNewAvlTreeOrdered(t *testing.T) {
	ints := NewAvlTree[int]()
	for i := range 100 {
		if !ints.Insert(i) {
			t.Errorf("Expected Insert of %d to return true.", i)
		}
	}
	// 100 nodes inserted in sorted order: an unbalanced tree would be a
	// chain of depth 100; the AVL tree must be at most ~1.44*log2(102).
	if d := ints.Depth(); d > 11 {
		t.Errorf("Expected balanced depth <= 11 for 100 sorted inserts, got %d", d)
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 99 {
		t.Errorf("Expected max 99, got %d found=%v", x, found)
	}

	strs := NewAvlTree[string]()
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

// TestNewAvlTreeFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order of
// the struct.
func TestNewAvlTreeFunc(t *testing.T) {
	tree := NewAvlTreeFunc(func(a, b TestTreeNode) int {
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
		t.Errorf("NewAvlTreeFunc order error, expected %v got %v", expect, got)
	}
}

// TestNewAvlTreeFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewAvlTreeFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewAvlTreeFunc(nil) to panic.")
		}
	}()
	NewAvlTreeFunc[TestTreeNode](nil)
}

// TestZeroValueTree verifies that the zero value of AvlTree behaves as an
// empty tree for every non-ordering operation and that Insert fails loudly
// because no comparison function has been set.
func TestZeroValueTree(t *testing.T) {
	var tree AvlTree[TestTreeNode]

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
	if _, found := tree.Search(mkKey(1)); found {
		t.Errorf("Expected not-found from Search on zero value tree.")
	}
	if tree.Delete(mkKey(1)) {
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
	if it := tree.Front(); !it.Done() {
		t.Errorf("Expected Front on zero value tree to be Done immediately.")
	}

	// Insert without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero value tree to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewAvlTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		tree.Insert(mkKey(1))
	}()
}

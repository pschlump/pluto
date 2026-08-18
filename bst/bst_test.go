package bst

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
)

// TestTreeNode is an Inteface Matcing data type for the Nodes that supports the Comparable
// interface.  This means that it has a Compare fucntion.

type TestTreeNode struct {
	S string
}

func NewTestTree() *TestTreeNode {
	return &TestTreeNode{}
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestTreeNode)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa TestTreeNode) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestTreeNode); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestTreeNode); ok {
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

func TestTreeInsertSearch(t *testing.T) {

	// Verify we can create a node.
	ANode := NewTestTree()
	_ = ANode

	var Tree1 BinarySearchTree[TestTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	Tree1.Insert(TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}

	if db2 {
		fmt.Printf("Test -- search for found item, at:%s\n", dbgo.LF())
	}
	ptr := Tree1.Search(TestTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

	if db2 {
		fmt.Printf("Test -- search for not found item\n")
	}
	ptr = Tree1.Search(TestTreeNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

	Tree1.Insert(TestTreeNode{S: "11"})
	Tree1.Insert(TestTreeNode{S: "13"})
	Tree1.Insert(TestTreeNode{S: "10"})
	ptr = Tree1.Search(TestTreeNode{S: "10"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(TestTreeNode{S: "13"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(TestTreeNode{S: "11"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(TestTreeNode{S: "14"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

}

// TEST TODO: func (tt *Binarytree[T]) Truncate()  {
func TestTreeTruncate(t *testing.T) {

	var Tree1 BinarySearchTree[TestTreeNode]

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
	if db4 {
		fmt.Printf("before Truncate at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
	Tree1.Truncate()
	if Tree1.Length() != 0 {
		t.Errorf("Expected empty tree")
		if db4 {
			fmt.Printf("Error: After Truncate at:%s tree=\n", dbgo.LF())
			Tree1.Dump(os.Stdout)
		}
	}

}

// test deleting node from tree.  This is a set of tests on .Delete() that tries
// works through all possible configurations of trees.
func TestTreeDelete(t *testing.T) {

	var Tree1 BinarySearchTree[TestTreeNode]

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
		fmt.Printf("Shoudl be empty but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
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
		fmt.Printf("Shoudl be single node, but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a right sub-tree
	Tree1.Truncate() // This tests tree.Trundate() also.
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	found = Tree1.Delete(TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
	if !found {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
		fmt.Printf("Shoudl be single node, but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
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
		fmt.Printf("Shoudl be empty but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
	// Should have a tree that looks like *(left is highter up)*
	//		{03}
	//	{08}
	if db6 {
		fmt.Printf("%sAfter delete with 2 nodes remaining: at:%s tree=%s\n", MiscLib.ColorYellow, dbgo.LF(), MiscLib.ColorReset)
		Tree1.Dump(os.Stdout)
	}

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
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
	if db5 {
		fmt.Printf("\nOrignal Tree at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "03"}) // Delete leaf
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 4 {
		t.Errorf("Expected to tree contain 4 nodes got, %d", size)
	}

	if db5 {
		fmt.Printf("\nAfter 2nd Delete\nSo Far So Good AT:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "02"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 3 {
		t.Errorf("Expected to tree contain 3 nodes got, %d", size)
	}
	if db5 {
		fmt.Printf("\nAfter 2nd Delete\nSo Far So Good AT:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "00"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size)
	}
	if db5 {
		fmt.Printf("\nAfter 3rd Delete\nSo Far So Good AT:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "09"}) // Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 nodes got, %d", size)
	}
	if db5 {
		fmt.Printf("\nAfter 4rd Delete\nEnd at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
}

func TestTreeMinMax(t *testing.T) {
	// func (tt *BinarySearchTree[T]) FindMax() ( item *T ) {
	// func (tt *BinarySearchTree[T]) FindMin() ( item *T ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	x := Tree1.FindMax()
	if (*x).S != "09" {
		t.Errorf("Unexpecd Max")
	}

	x = Tree1.FindMin()
	if (*x).S != "00" {
		t.Errorf("Unexpecd Min")
	}
}

func TestTreeDepth(t *testing.T) {
	// func (tt *BinarySearchTree[T]) Depth() ( d int ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	n := Tree1.Depth()
	if n != 3 {
		t.Errorf("Unexpecd Depth, got %d expected 3", n)
	}
}

func TestTreeIndex(t *testing.T) {
	// func (tt *BinarySearchTree[T]) Index(pos int) ( item *T ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	// fmt.Printf ( "\nBefore ------------------------------\n" )
	// Tree1.Dump(os.Stdout)

	x := Tree1.Index(0)
	if x == nil {
		t.Errorf("Error, nil returend for 0 index")
	} else if x.S != "00" {
		t.Errorf("Error, Not Fond expected ->00<- got ->%s<-", x.S)
	}

	x = Tree1.Index(1)
	if x == nil {
		t.Errorf("Error, nil returend for 1 index")
	} else if x.S != "02" {
		t.Errorf("Error, Not Fond expected ->02<- got ->%s<-", x.S)
	}

	x = Tree1.Index(4)
	if x == nil {
		t.Errorf("Error, nil returend for 1 index")
	} else if x.S != "09" {
		t.Errorf("Error, Not Fond expected ->09<- got ->%s<-", x.S)
	}
}

func TestTreeRevese(t *testing.T) {
	// func (tt *BinarySearchTree[T]) Reverse() {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	Tree1.Reverse()

	if false {
		fmt.Printf("\nAfter ------------------------------\n")
		Tree1.Dump(os.Stdout)
	}

	if size := Tree1.Length(); size != 5 {
		t.Errorf("Error")
	}
}

func TestTreeDeleteAtTail(t *testing.T) {
	// func (tt *BinarySearchTree[T]) DeleteAtTail(find T) ( found bool ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found := Tree1.DeleteAtTail()

	if false {
		fmt.Printf("\nAfter ------------------------------ %v\n", found)
		Tree1.Dump(os.Stdout)
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
}

func TestTreeDeleteAtHead(t *testing.T) {
	// func (tt *BinarySearchTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found := Tree1.DeleteAtHead()

	if false {
		fmt.Printf("\nAfter ------------------------------ %v\n", found)
		Tree1.Dump(os.Stdout)
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
}

/*
func (tt *BinarySearchTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {
func (tt *BinarySearchTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {
*/

func TestTreeWalkInOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *BinarySearchTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	fx := func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	if db7 {
		fmt.Printf("Output: %s\n", x)
	}

	//	Output: [00 02 03 05 09]
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("InOrder error, expcted %s got %s", expect, x)
	}

}

func TestTreeWalkPreOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *BinarySearchTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	fx := func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPreOrder(fx, nil)

	if db7 {
		fmt.Printf("PreOrder Output: %s\n", x)
	}

	//PreOrder Output: [05 02 00 03 09]
	expect := []string{"05", "02", "00", "03", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PreOrder error, expcted %s got %s", expect, x)
	}
}

func TestTreeWalkPostOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *BinarySearchTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinarySearchTree[TestTreeNode]

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	fx := func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPostOrder(fx, nil)

	if db7 {
		fmt.Printf("PostOrder Output: %s\n", x)
	}

	// PostOrder Output: [00 03 02 09 05]
	expect := []string{"00", "03", "02", "09", "05"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("PostOrder error, expcted %s got %s", expect, x)
	}
}

const db2 = false
const db3 = false
const db4 = false
const db5 = false
const db6 = false
const db7 = false

// IntNode is a Comparable int used for focused tests and benchmarks.
type IntNode int

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = IntNode(0)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa IntNode) Compare(x comparable.Comparable) int {
	if bb, ok := x.(IntNode); ok {
		if aa < bb {
			return -1
		} else if aa > bb {
			return 1
		}
		return 0
	}
	panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
}

// Inserting a duplicate key must replace the stored item without losing the
// existing children or changing the length.
func TestTreeInsertDuplicate(t *testing.T) {
	var tree BinarySearchTree[IntNode]

	tree.Insert(IntNode(5))
	tree.Insert(IntNode(2))
	tree.Insert(IntNode(9))
	tree.Insert(IntNode(5)) // duplicate of the root

	if size := tree.Length(); size != 3 {
		t.Errorf("Expected tree to contain 3 nodes after duplicate insert, got %d", size)
	}
	if tree.Search(IntNode(2)) == nil {
		t.Errorf("Left child lost after duplicate insert at root")
	}
	if tree.Search(IntNode(9)) == nil {
		t.Errorf("Right child lost after duplicate insert at root")
	}
	if got := *tree.Search(IntNode(5)); got != IntNode(5) {
		t.Errorf("Expected to find 5 at root, got %d", got)
	}
}

// Deleting a node with two children must promote the in-order successor
// without losing the successor's sub-tree.
func TestTreeDeleteTwoChildrenKeepsSubtree(t *testing.T) {
	var tree BinarySearchTree[IntNode]

	//            5
	//        2       9
	//              7   11
	//                10  12
	for _, v := range []IntNode{5, 2, 9, 7, 11, 10, 12} {
		tree.Insert(v)
	}

	// Deleting 9 promotes its in-order successor 10; 11 and 12 must survive.
	tree.Delete(IntNode(9))
	if size := tree.Length(); size != 6 {
		t.Errorf("Expected tree to contain 6 nodes after delete, got %d", size)
	}
	for _, v := range []IntNode{5, 2, 7, 10, 11, 12} {
		if tree.Search(v) == nil {
			t.Errorf("Node %d lost after deleting 9", v)
		}
	}
	if tree.Search(IntNode(9)) != nil {
		t.Errorf("Deleted node 9 still present in tree")
	}

	// In-order sequence must still be sorted.
	var got []IntNode
	for v := range tree.All() {
		got = append(got, v)
	}
	expect := []IntNode{2, 5, 7, 10, 11, 12}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("In-order after delete, expected %v got %v", expect, got)
	}
}

// Operations on an empty tree must behave sanely.
func TestTreeEmptyOps(t *testing.T) {
	var tree BinarySearchTree[IntNode]

	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree")
	}
	if tree.Length() != 0 {
		t.Errorf("Expected length 0, got %d", tree.Length())
	}
	if tree.Search(IntNode(1)) != nil {
		t.Errorf("Expected nil search on empty tree")
	}
	if tree.Delete(IntNode(1)) {
		t.Errorf("Expected delete to fail on empty tree")
	}
	if tree.FindMin() != nil || tree.FindMax() != nil {
		t.Errorf("Expected nil min/max on empty tree")
	}
	if tree.DeleteAtHead() || tree.DeleteAtTail() {
		t.Errorf("Expected DeleteAt* to fail on empty tree")
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected depth 0 on empty tree, got %d", tree.Depth())
	}
	if tree.Index(0) != nil {
		t.Errorf("Expected nil index on empty tree")
	}
	tree.Reverse() // must not panic
	n := 0
	for range tree.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected empty iteration on empty tree")
	}
}

// Index must reject out-of-range positions.
func TestTreeIndexOutOfRange(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	for _, v := range []IntNode{5, 2, 9} {
		tree.Insert(v)
	}
	if tree.Index(-1) != nil {
		t.Errorf("Expected nil for negative index")
	}
	if tree.Index(3) != nil {
		t.Errorf("Expected nil for index >= length")
	}
	if got := tree.Index(2); got == nil || *got != IntNode(9) {
		t.Errorf("Expected 9 at index 2")
	}
}

// All must iterate ascending; Backward must iterate descending; both must
// honor early break.
func TestTreeIterators(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		tree.Insert(v)
	}

	var fwd []IntNode
	for v := range tree.All() {
		fwd = append(fwd, v)
	}
	if expect := []IntNode{0, 2, 3, 5, 9}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All: expected %v got %v", expect, fwd)
	}

	var bwd []IntNode
	for v := range tree.Backward() {
		bwd = append(bwd, v)
	}
	if expect := []IntNode{9, 5, 3, 2, 0}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward: expected %v got %v", expect, bwd)
	}

	// Early break: only the first item should be seen.
	count := 0
	for v := range tree.All() {
		if v != IntNode(0) {
			t.Errorf("All with break: expected first value 0, got %d", v)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("All with break: expected 1 iteration, got %d", count)
	}
}

// Reverse must mirror the tree so that in-order iteration is descending.
func TestTreeReverseOrder(t *testing.T) {
	var tree BinarySearchTree[IntNode]
	for _, v := range []IntNode{5, 2, 9, 0, 3} {
		tree.Insert(v)
	}
	tree.Reverse()

	var got []IntNode
	for v := range tree.All() {
		got = append(got, v)
	}
	if expect := []IntNode{9, 5, 3, 2, 0}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Reverse: expected %v got %v", expect, got)
	}
	if size := tree.Length(); size != 5 {
		t.Errorf("Expected length 5 after Reverse, got %d", size)
	}
}

// lcgKey produces a deterministic pseudo-random key stream so benchmarks
// build reasonably bushy (non-degenerate) trees.
func lcgKey(seed *uint32, mod int) IntNode {
	*seed = *seed*48271 + 1
	return IntNode(int(*seed) % mod)
}

func BenchmarkInsert(b *testing.B) {
	var seed uint32 = 12345
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var tree BinarySearchTree[IntNode]
		for j := 0; j < 1000; j++ {
			tree.Insert(lcgKey(&seed, 100000))
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	var tree BinarySearchTree[IntNode]
	var seed uint32 = 12345
	const n = 10000
	for j := 0; j < n; j++ {
		tree.Insert(lcgKey(&seed, 10*n))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(lcgKey(&seed, 10*n))
	}
}

func BenchmarkDelete(b *testing.B) {
	var seed uint32 = 12345
	keys := make([]IntNode, 0, 1000)
	var tree BinarySearchTree[IntNode]
	for j := 0; j < 1000; j++ {
		k := lcgKey(&seed, 100000)
		keys = append(keys, k)
		tree.Insert(k)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var t2 BinarySearchTree[IntNode]
		for _, k := range keys {
			t2.Insert(k)
		}
		b.StartTimer()
		for _, k := range keys {
			t2.Delete(k)
		}
	}
}

func BenchmarkIterateAll(b *testing.B) {
	var tree BinarySearchTree[IntNode]
	var seed uint32 = 12345
	for j := 0; j < 10000; j++ {
		tree.Insert(lcgKey(&seed, 100000))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0
		for v := range tree.All() {
			sum += int(v)
		}
		_ = sum
	}
}

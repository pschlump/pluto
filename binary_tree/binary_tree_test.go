package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

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

	var Tree1 BinaryTree[TestTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	v1 := Tree1.Insert(&TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if v1 == false {
		t.Errorf("Expected to insert new node, got back false for new.")
	}

	v1 = Tree1.Insert(&TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if v1 == true {
		t.Errorf("Expected to insert duplicate node, got back false for new.")
	}
	if Tree1.Len() != 1 {
		t.Errorf("Expected 1 node in tree, got %d", Tree1.Len())
	}

	// 	return

	if db2 {
		fmt.Printf("Test -- search for found item, at:%s\n", dbgo.LF())
	}
	ptr := Tree1.Search(&TestTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

	if db2 {
		fmt.Printf("Test -- search for not found item\n")
	}
	ptr = Tree1.Search(&TestTreeNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

	Tree1.Insert(&TestTreeNode{S: "11"})
	Tree1.Insert(&TestTreeNode{S: "13"})
	Tree1.Insert(&TestTreeNode{S: "10"})
	ptr = Tree1.Search(&TestTreeNode{S: "10"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(&TestTreeNode{S: "13"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(&TestTreeNode{S: "11"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree1.Search(&TestTreeNode{S: "14"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

}

// Test tree truncate, very tree empty after build.
func TestTreeTruncate(t *testing.T) {

	var Tree1 BinaryTree[TestTreeNode]

	// Build this tree:
	//			{00}
	//		{02}
	//			{03}
	//	{05}
	//		{09}
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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

	var Tree1 BinaryTree[TestTreeNode]

	// Build this tree (eventually):
	//			{00}
	//		{02}
	//			{03}
	//	{05}
	//		{09}

	// -------------------------------------------------------------------------------
	// Delete from Empty tree
	found := Tree1.Delete(&TestTreeNode{S: "05"}) // Delete called on empty tree.
	if found == true {
		t.Errorf("Found node in empty tree.")
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a single root node.
	Tree1.Insert(&TestTreeNode{S: "05"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete leaf (Only Node in tree)
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
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
		fmt.Printf("Shoudl be single node, but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a right sub-tree
	Tree1.Truncate() // This tests tree.Trundate() also.
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "08"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size)
		fmt.Printf("Shoudl be single node, but is: at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete root node with 2 sub trees.
	Tree1.Truncate()
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "08"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete Tree with left and right children.
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
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}
	if db5 {
		fmt.Printf("\nOrignal Tree at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(&TestTreeNode{S: "03"}) // Delete leaf
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

	found = Tree1.Delete(&TestTreeNode{S: "02"}) // Delete mid node
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

	found = Tree1.Delete(&TestTreeNode{S: "00"}) // Delete mid node
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

	found = Tree1.Delete(&TestTreeNode{S: "09"}) // Delete mid node
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
	// func (tt *BinaryTree[T]) FindMax() ( item *T ) {
	// func (tt *BinaryTree[T]) FindMin() ( item *T ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
	// func (tt *BinaryTree[T]) Depth() ( d int ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	n := Tree1.Depth()
	if n == 3 {
		t.Errorf("Unexpecd Depth, got %d expected 3", n)
	}
}

func TestTreeIndex(t *testing.T) {
	// func (tt *BinaryTree[T]) Index(pos int) ( item *T ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
	// func (tt *BinaryTree[T]) Reverse() {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
	// func (tt *BinaryTree[T]) DeleteAtTail(find T) ( found bool ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
	// func (tt *BinaryTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
func (tt *BinaryTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {
func (tt *BinaryTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {
*/

func TestTreeWalkInOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *BinaryTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	if db8 {
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
	// func (tt *BinaryTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPreOrder(fx, nil)

	if db8 {
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
	// func (tt *BinaryTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	if db3 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
	}

	var x []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return true
	}
	Tree1.WalkPostOrder(fx, nil)

	if db8 {
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
const db8 = false

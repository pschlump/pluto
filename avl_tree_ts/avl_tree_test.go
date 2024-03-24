package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// TODO - add a avlValidate (tree) that recrusivly checks heights and verifys all are 1,0,-1

import (
	"fmt"
	"os"
	"testing"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/g_lib"
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

	var Tree1 AvlTree[TestTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	Tree1.Insert(&TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}

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

func TestTreeInsertWithDupsSearch(t *testing.T) {

	var Tree8 AvlTree[TestTreeNode]

	if !Tree8.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	Tree8.Insert(&TestTreeNode{S: "12"})

	if Tree8.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}

	if db2 {
		fmt.Printf("Test -- search for found item, at:%s\n", dbgo.LF())
	}
	ptr := Tree8.Search(&TestTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

	if db2 {
		fmt.Printf("Test -- search for not found item\n")
	}
	ptr = Tree8.Search(&TestTreeNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}

	Tree8.Insert(&TestTreeNode{S: "11"})
	Tree8.Insert(&TestTreeNode{S: "13"})
	Tree8.Insert(&TestTreeNode{S: "10"})
	// ------------------------------------------- new -------------------------------------------
	if db7 {
		fmt.Printf("Before adding dups\n")
		Tree8.Dump(os.Stdout)
	}
	Tree8.Insert(&TestTreeNode{S: "12"})
	Tree8.Insert(&TestTreeNode{S: "12"})
	if db7 {
		fmt.Printf("After adding dups\n")
		Tree8.Dump(os.Stdout)
	}
	// ------------------------------------------- end -------------------------------------------
	ptr = Tree8.Search(&TestTreeNode{S: "10"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree8.Search(&TestTreeNode{S: "13"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree8.Search(&TestTreeNode{S: "11"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}
	ptr = Tree8.Search(&TestTreeNode{S: "14"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead", *ptr)
	}
	ptr = Tree8.Search(&TestTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

}

// TEST TODO: func (tt *Binarytree[T]) Truncate()  {
func TestTreeTruncate(t *testing.T) {

	var Tree1 AvlTree[TestTreeNode]

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

	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) FindMax() ( item *T ) {
	// func (tt *AvlTree[T]) FindMin() ( item *T ) {
	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) Depth() ( d int ) {
	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) Index(pos int) ( item *T ) {
	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) Reverse() {
	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) DeleteAtTail(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]

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
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]

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
func (tt *AvlTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {
func (tt *AvlTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {
*/

func TestTreeWalkInOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]

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

	if db1 {
		fmt.Printf("Output: %s\n", x)
	}

	// TODO -- automate correct answer.
}

func TestTreeWalkPreOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]

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

	if db1 {
		fmt.Printf("PreOrder Output: %s\n", x)
	}

	// TODO -- automate correct answer.
}

func TestTreeWalkPostOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]

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

	if db1 {
		fmt.Printf("PostOrder Output: %s\n", x)
	}

	// TODO -- automate correct answer.
}

func TestTreeCopy(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	var Tree2 AvlTree[TestTreeNode]
	Tree2.Insert(&TestTreeNode{S: "nn"})
	Tree2.Insert(&TestTreeNode{S: "vv"})

	if db10 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
		Tree2.Dump(os.Stdout)
	}

	Tree1.Copy(&Tree2)

	var got []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		// fmt.Printf("%d %d: %s\n", pos, depth, data.S)
		got = append(got, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	if db1 {
		fmt.Printf("PostOrder Output: %s\n", got)
	}

	expect := []string{"nn", "vv"}
	if !g_lib.EqualSlice(expect, got) {
		t.Errorf("Expected %s got %s - slices differ\n", expect, got)
	}
}

func TestTreeUnion(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	var Tree2 AvlTree[TestTreeNode]
	Tree2.Insert(&TestTreeNode{S: "nn"})
	Tree2.Insert(&TestTreeNode{S: "vv"})
	Tree2.Insert(&TestTreeNode{S: "bb"})
	var Tree3 AvlTree[TestTreeNode]
	Tree3.Insert(&TestTreeNode{S: "aa"})
	Tree3.Insert(&TestTreeNode{S: "bb"})
	Tree3.Insert(&TestTreeNode{S: "nn"})

	if db10 {
		fmt.Printf("at:%s tree=\n", dbgo.LF())
		Tree1.Dump(os.Stdout)
		Tree2.Dump(os.Stdout)
		Tree3.Dump(os.Stdout)
	}

	Tree1.Union(&Tree2, &Tree3)

	var got []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		if db11 {
			fmt.Printf("%d %d: %s\n", pos, depth, data.S)
		}
		got = append(got, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	if db1 {
		fmt.Printf("PostOrder Output: %s\n", got)
	}

	expect := []string{"aa", "bb", "nn", "vv"}
	if !g_lib.EqualSlice(expect, got) {
		t.Errorf("Expected %s got %s - slices differ\n", expect, got)
	}
}

func TestTreeMinus(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	var Tree2 AvlTree[TestTreeNode]
	Tree2.Insert(&TestTreeNode{S: "nn"})
	Tree2.Insert(&TestTreeNode{S: "vvv"})
	Tree2.Insert(&TestTreeNode{S: "bbbb"})
	var Tree3 AvlTree[TestTreeNode]
	Tree3.Insert(&TestTreeNode{S: "a"})
	Tree3.Insert(&TestTreeNode{S: "bbbb"})
	Tree3.Insert(&TestTreeNode{S: "nnnnn"})

	if db10 {
		fmt.Printf("Minus Test: at:%s tree=\n", dbgo.LF())
		fmt.Printf("Tree 1\n")
		Tree1.Dump(os.Stdout)
		fmt.Printf("Tree 2\n")
		Tree2.Dump(os.Stdout)
		fmt.Printf("Tree 3\n")
		Tree3.Dump(os.Stdout)
	}

	Tree1.Minus(&Tree2, &Tree3)

	var got []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		if db13 {
			fmt.Printf("%d %d: %s\n", pos, depth, data.S)
		}
		got = append(got, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	expect := []string{"nn", "vvv"}
	if !g_lib.EqualSlice(expect, got) {
		t.Errorf("Expected %s got %s - slices differ\n", expect, got)
	}
}

func TestTreeIntersect(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
	// func (tt *AvlTree[T]) DeleteAtHead(find T) ( found bool ) {
	var Tree1 AvlTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	var Tree2 AvlTree[TestTreeNode]
	Tree2.Insert(&TestTreeNode{S: "nn"})
	Tree2.Insert(&TestTreeNode{S: "vv"})
	Tree2.Insert(&TestTreeNode{S: "bb"})
	var Tree3 AvlTree[TestTreeNode]
	Tree3.Insert(&TestTreeNode{S: "aa"})
	Tree3.Insert(&TestTreeNode{S: "bb"})
	Tree3.Insert(&TestTreeNode{S: "nn"})

	if db11 {
		fmt.Printf("Intersect Test: at:%s tree=\n", dbgo.LF())
		fmt.Printf("Tree 1\n")
		Tree1.Dump(os.Stdout)
		fmt.Printf("Tree 2\n")
		Tree2.Dump(os.Stdout)
		fmt.Printf("Tree 3\n")
		Tree3.Dump(os.Stdout)
	}

	Tree1.Intersect(&Tree2, &Tree3)

	var got []string
	var fx ApplyFunction[TestTreeNode]
	fx = func(pos, depth int, data *TestTreeNode, y interface{}) bool {
		if db11 {
			fmt.Printf("%d %d: %s\n", pos, depth, data.S)
		}
		got = append(got, data.S)
		return true
	}
	Tree1.WalkInOrder(fx, nil)

	expect := []string{"bb", "nn"}
	if !g_lib.EqualSlice(expect, got) {
		t.Errorf("Expected %s got %s - slices differ\n", expect, got)
	}
}

const db1 = false
const db2 = false
const db3 = false
const db4 = false
const db5 = false
const db6 = false
const db7 = false
const db10 = false
const db11 = false
const db13 = false

package binary_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"os"
	"reflect"
	"sync"
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
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "08"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete Tree with 1 side node.
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
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "08"})
	Tree1.Insert(&TestTreeNode{S: "03"})
	found = Tree1.Delete(&TestTreeNode{S: "05"}) // Delete Tree with left and right children.
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

// TestTreeDeleteKeepsSubtree is a regression test: deleting a node with two
// children must not silently drop the left subtree of the replacement node.
func TestTreeDeleteKeepsSubtree(t *testing.T) {

	var Tree1 BinaryTree[TestTreeNode]

	// Build this tree:
	//	{05}
	//		{09}
	//	    {07}
	//	      {08}
	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "07"})
	Tree1.Insert(&TestTreeNode{S: "08"})

	found := Tree1.Delete(&TestTreeNode{S: "05"})
	if !found {
		t.Fatalf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 3 {
		t.Fatalf("Expected tree to contain 3 nodes got, %d", size)
	}
	for _, s := range []string{"07", "08", "09"} {
		if Tree1.Search(&TestTreeNode{S: s}) == nil {
			t.Errorf("Expected to find node %s in tree after delete, did not.", s)
		}
	}
	if x := Tree1.FindMin(); x == nil || x.S != "07" {
		t.Errorf("Expected min of 07 after delete, got %+v", x)
	}

	// Also verify the whole in-order traversal is intact.
	var got []string
	Tree1.WalkInOrder(func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
		got = append(got, data.S)
		return true
	}, nil)
	if expect := []string{"07", "08", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder error, expected %s got %s", expect, got)
	}
}

func TestTreeDeleteMatch(t *testing.T) {

	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})

	cmp := func(a, b *TestTreeNode) int {
		return a.Compare(*b)
	}

	found := Tree1.DeleteMatch(&TestTreeNode{S: "02"}, cmp)
	if !found {
		t.Errorf("Expected DeleteMatch to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected tree to contain 2 nodes got, %d", size)
	}
	if Tree1.Search(&TestTreeNode{S: "02"}) != nil {
		t.Errorf("Expected node 02 to be gone after DeleteMatch.")
	}

	found = Tree1.DeleteMatch(&TestTreeNode{S: "77"}, cmp)
	if found {
		t.Errorf("Expected DeleteMatch to not find a node, but it did.")
	}
}

func TestTreeSetGetData(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]
	Tree1.Insert(&TestTreeNode{S: "05"})

	el := &BinaryTreeElement[TestTreeNode]{}
	el.SetData(&TestTreeNode{S: "42"})
	if d := el.GetData(); d == nil || d.S != "42" {
		t.Errorf("Expected GetData to return 42, got %+v", d)
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

	if n := Tree1.Depth(); n != 0 {
		t.Errorf("Unexpecd Depth for empty tree, got %d expected 0", n)
	}

	Tree1.Insert(&TestTreeNode{S: "05"})
	if n := Tree1.Depth(); n != 1 {
		t.Errorf("Unexpecd Depth for root-only tree, got %d expected 1", n)
	}

	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})
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
		t.Errorf("Error, nil returend for 4 index")
	} else if x.S != "09" {
		t.Errorf("Error, Not Fond expected ->09<- got ->%s<-", x.S)
	}

	if x = Tree1.Index(-1); x != nil {
		t.Errorf("Error, expected nil for -1 index, got %+v", x)
	}
	if x = Tree1.Index(5); x != nil {
		t.Errorf("Error, expected nil for out of range index, got %+v", x)
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

	if size := Tree1.Length(); size != 5 {
		t.Errorf("Error")
	}

	// After a Reverse the in-order walk is the reverse of the original.
	var got []string
	Tree1.WalkInOrder(func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
		got = append(got, data.S)
		return true
	}, nil)
	if expect := []string{"09", "05", "03", "02", "00"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder after Reverse error, expected %s got %s", expect, got)
	}

	// Reversing twice restores the original order.
	Tree1.Reverse()
	got = got[:0]
	Tree1.WalkInOrder(func(pos, depth int, data *TestTreeNode, userData interface{}) bool {
		got = append(got, data.S)
		return true
	}, nil)
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("InOrder after double Reverse error, expected %s got %s", expect, got)
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
	if !found {
		t.Errorf("Expected DeleteAtTail to find a node, did not.")
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
	if x := Tree1.FindMax(); x == nil || x.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", x)
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
	if !found {
		t.Errorf("Expected DeleteAtHead to find a node, did not.")
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Error")
	}
	if x := Tree1.FindMin(); x == nil || x.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", x)
	}
}

// TestTreeEmptyOps verifies that operations on an empty tree behave sanely.
func TestTreeEmptyOps(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	if Tree1.Search(&TestTreeNode{S: "05"}) != nil {
		t.Errorf("Expected nil from Search on empty tree.")
	}
	if Tree1.FindMin() != nil {
		t.Errorf("Expected nil from FindMin on empty tree.")
	}
	if Tree1.FindMax() != nil {
		t.Errorf("Expected nil from FindMax on empty tree.")
	}
	if Tree1.DeleteAtHead() {
		t.Errorf("Expected false from DeleteAtHead on empty tree.")
	}
	if Tree1.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtTail on empty tree.")
	}
	if Tree1.Index(0) != nil {
		t.Errorf("Expected nil from Index on empty tree.")
	}
	if Tree1.Delete(&TestTreeNode{S: "05"}) {
		t.Errorf("Expected false from Delete on empty tree.")
	}
	if n := 0; Tree1.Len() != n || Tree1.Length() != n {
		t.Errorf("Expected 0 length on empty tree.")
	}
}

func TestTreeWalkInOrder(t *testing.T) {
	// type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool
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
	var pos []int
	fx := func(p, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		pos = append(pos, p)
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
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("InOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkPreOrder(t *testing.T) {
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
	var pos []int
	fx := func(p, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		pos = append(pos, p)
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
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("PreOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkPostOrder(t *testing.T) {
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
	var pos []int
	fx := func(p, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		pos = append(pos, p)
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
	if expectPos := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(pos, expectPos) {
		t.Errorf("PostOrder pos error, expected %v got %v", expectPos, pos)
	}
}

func TestTreeWalkEarlyStop(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})

	// Returning false from the callback must stop the walk.
	var x []string
	fx := func(p, depth int, data *TestTreeNode, y interface{}) bool {
		x = append(x, data.S)
		return false
	}
	Tree1.WalkInOrder(fx, nil)
	if expect := []string{"02"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("InOrder early stop error, expected %s got %s", expect, x)
	}
}

func TestTreeWalkFunc(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})

	var x []string
	Tree1.WalkFunc(func(a *TestTreeNode) {
		x = append(x, a.S)
	})

	// WalkFunc is pre-order: [05 02 00 03 09]
	expect := []string{"05", "02", "00", "03", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("WalkFunc error, expected %s got %s", expect, x)
	}

	// WalkFunc on an empty tree must not call Fx.
	var empty BinaryTree[TestTreeNode]
	called := false
	empty.WalkFunc(func(a *TestTreeNode) {
		called = true
	})
	if called {
		t.Errorf("WalkFunc on empty tree called the callback.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Iterators
// -------------------------------------------------------------------------------------------------------

func TestTreeOldStyleIter(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})

	var x []string
	for it := Tree1.Front(); !it.Done(); it.Next() {
		x = append(x, it.Value().S)
	}
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("Iterator error, expected %s got %s", expect, x)
	}

	// Value after Done must be nil.
	it := Tree1.Front()
	for !it.Done() {
		it.Next()
	}
	if it.Value() != nil {
		t.Errorf("Expected nil Value after Done.")
	}

	// Empty tree: iterator is Done immediately.
	var empty BinaryTree[TestTreeNode]
	if it := empty.Front(); !it.Done() {
		t.Errorf("Expected iterator on empty tree to be Done immediately.")
	}
}

func TestTreeAll(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})

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
	var empty BinaryTree[TestTreeNode]
	for range empty.All() {
		t.Errorf("All on empty tree yielded a value.")
	}
}

func TestTreeBackward(t *testing.T) {
	var Tree1 BinaryTree[TestTreeNode]

	Tree1.Insert(&TestTreeNode{S: "05"})
	Tree1.Insert(&TestTreeNode{S: "02"})
	Tree1.Insert(&TestTreeNode{S: "09"})
	Tree1.Insert(&TestTreeNode{S: "00"})
	Tree1.Insert(&TestTreeNode{S: "03"})

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
	var empty BinaryTree[TestTreeNode]
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
		var tree BinaryTree[TestTreeNode]
		for j := 0; j < benchmarkTreeSize; j++ {
			tree.Insert(&TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	var tree BinaryTree[TestTreeNode]
	for j := 0; j < benchmarkTreeSize; j++ {
		tree.Insert(&TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	find := TestTreeNode{S: fmt.Sprintf("%06d", benchmarkTreeSize/2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(&find)
	}
}

func BenchmarkDelete(b *testing.B) {
	var tree BinaryTree[TestTreeNode]
	for j := 0; j < benchmarkTreeSize; j++ {
		tree.Insert(&TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < benchmarkTreeSize; j++ {
			tree.Delete(&TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
		for j := 0; j < benchmarkTreeSize; j++ {
			tree.Insert(&TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

const db2 = false

const db3 = false
const db4 = false
const db5 = false
const db6 = false
const db8 = false

// TestTreeConcurrent exercises the tree from multiple goroutines; it is
// intended to be run with the race detector enabled.
func TestTreeConcurrent(t *testing.T) {
	tree := NewBinaryTree[TestTreeNode]()

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				v := &TestTreeNode{S: fmt.Sprintf("%d:%04d", w, i)}
				tree.Insert(v)
				tree.Search(v)
				if i%3 == 0 {
					tree.Delete(v)
				}
				tree.Len()
				for range tree.All() {
					break
				}
			}
		}(w)
	}
	wg.Wait()
}

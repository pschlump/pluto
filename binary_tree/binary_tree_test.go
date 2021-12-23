package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

 	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
!	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
 	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
 	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
 	Index - return the Nth item	in the list - in a format usable with Delete.					O(n) n/2
 	InsertBeforeHead — Inserts a new element before the current first ement of list.  			O(1)
*	IsEmpty — Returns true if the linked list is empty											O(1)
*	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
 	Peek - Look at data at head of list.														O(1)
 	Pop	- Remove and return from the head of the list.											O(1)
 	Push - Insert at the head of the list.														O(1)
*	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n) n/2
!	Truncate - Delete all the nodes in list. 													O(1)
*	Walk - Iterate from head to tail of list. 													O(n)
*	WalkInorder - Iterate from head to tail of list. 													O(n)
*	WalkPreorder - Iterate from head to tail of list. 													O(n)
*	WalkPostorder - Iterate from head to tail of list. 													O(n)
*	WalkDepthFirst - Iterate from head to tail of list. 													O(n)

With the basic stack operations it also can be used as a stack:
*	Push — Inserts an element at the top														O(1)
*	Pop - will remove the top element from the stack.  An error is returned if the stack is		O(1)
		empty.
*	IsEmpty — Returns true if the stack is empty												O(1)
*	Peek — Returns the top element without removing from the stack								O(1)

With the use of Enque can be used as a Queue.  This is a synonym for AppendAtTail.				O(1)

* 	PeekTail - Peek returns the last element of the DLL (like a Queue) or an error 				O(1)
		indicating that the queue is empty.			
* 	PopTail - Remvoe the element at the end of the DLL.											O(1)
*	Enque - add to the tail so that DLL can be used as a Queue.									O(1)

This version of the DLL is not suitable for concurrnet usage but ../DLLTs has mutex 
locks so that it is thread safe.  It has the exact same interface.
*/

import (
	"fmt"
	"os"
	"testing"

	"github.com/pschlump/godebug"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/MiscLib"
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
		panic ( fmt.Sprintf("Passed invalid type %T to a Compare function.",x) )
	}
	return 0
}

func TestTreeInsertSearch(t *testing.T) {

	return

	// Verify we can create a node.
	ANode := NewTestTree()
	_ = ANode 

	var Tree1 BinaryTree[TestTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after decleration, failed to get one.")
	}

	Tree1.Insert(TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}

	if db2 {
		fmt.Printf ( "Test -- search for found item, at:%s\n", godebug.LF() );
	}
	ptr := Tree1.Search(TestTreeNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in tree, returned nil instead")
	}

	if db2 {
		fmt.Printf ( "Test -- search for not found item\n" );
	}
	ptr = Tree1.Search(TestTreeNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead",*ptr)
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
		t.Errorf("Expected *NOT* to find node in tree, returned value [%+v] instead",*ptr)
	}

}

// TEST TODO: func (tt *Binarytree[T]) Truncate()  {
func TestTreeTruncate(t *testing.T) {

	var Tree1 BinaryTree[TestTreeNode]

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
		fmt.Printf ( "before Truncate at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	Tree1.Truncate()
	if Tree1.Length() != 0 {
		t.Errorf("Expected empty tree")
		if db4 {
			fmt.Printf ( "Error: After Truncate at:%s tree=\n", godebug.LF())
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
	found := Tree1.Delete(TestTreeNode{S: "05"})	// Delete called on empty tree.
	if found == true {
		t.Errorf("Found node in empty tree." )
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a single root node.
	Tree1.Insert(TestTreeNode{S: "05"})
	found = Tree1.Delete(TestTreeNode{S: "05"})	// Delete leaf (Only Node in tree)
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	if size := Tree1.Length(); size != 0 {
		t.Errorf("Expected to empty tree got, %d", size )
		fmt.Printf ( "Shoudl be empty but is: at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a left sub-tree
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"})	// Delete Tree with 1 side node.
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size )
		fmt.Printf ( "Shoudl be single node, but is: at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete from tree with a root node and a right sub-tree
	Tree1.Truncate()		// This tests tree.Trundate() also.
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	found = Tree1.Delete(TestTreeNode{S: "05"})	// Delete Tree with 1 side node.
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 node got, %d", size )
		fmt.Printf ( "Shoudl be single node, but is: at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}

	// -------------------------------------------------------------------------------
	// Root-Test: Delete root node with 2 sub trees.
	Tree1.Truncate()
	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "08"})
	Tree1.Insert(TestTreeNode{S: "03"})
	found = Tree1.Delete(TestTreeNode{S: "05"})	// Delete Tree with left and right children.
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size )
		fmt.Printf ( "Shoudl be empty but is: at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	// Should have a tree that looks like *(left is highter up)*
	//		{03} 
	//	{08} 
	if db6 {
		fmt.Printf ( "%sAfter delete with 2 nodes remaining: at:%s tree=%s\n", MiscLib.ColorYellow, godebug.LF(), MiscLib.ColorReset)
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
		fmt.Printf ( "at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	if db5 {
		fmt.Printf ( "\nOrignal Tree at:%s tree=\n", godebug.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "03"})	// Delete leaf
	if db3 {
		fmt.Printf ( "at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	if size := Tree1.Length(); size != 4 {
		t.Errorf("Expected to tree contain 4 nodes got, %d", size )
	}

	if db5 {
		fmt.Printf ( "\nAfter 2nd Delete\nSo Far So Good AT:%s tree=\n", godebug.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "02"})	// Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	if size := Tree1.Length(); size != 3 {
		t.Errorf("Expected to tree contain 3 nodes got, %d", size )
	}
	if db5 {
		fmt.Printf ( "\nAfter 2nd Delete\nSo Far So Good AT:%s tree=\n", godebug.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "00"})	// Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	if size := Tree1.Length(); size != 2 {
		t.Errorf("Expected to tree contain 2 nodes got, %d", size )
	}
	if db5 {
		fmt.Printf ( "\nAfter 3rd Delete\nSo Far So Good AT:%s tree=\n", godebug.LF())
		Tree1.Dump(os.Stdout)
	}

	found = Tree1.Delete(TestTreeNode{S: "09"})	// Delete mid node
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	if size := Tree1.Length(); size != 1 {
		t.Errorf("Expected to tree contain 1 nodes got, %d", size )
	}
	if db5 {
		fmt.Printf ( "\nAfter 4rd Delete\nEnd at:%s tree=\n", godebug.LF())
		Tree1.Dump(os.Stdout)
	}
}

const db2 = false
const db3 = false
const db4 = false
const db5 = false
const db6 = false


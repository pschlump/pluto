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

func TestTreeDelete(t *testing.T) {

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
	if db3 {
		fmt.Printf ( "at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}

	/*
	TODO ----------- TODO ----------- TODO ----------- TODO ----------- TODO ----------- TODO ----------- 
	found := Tree1.Remove(TestTreeNode{S: "00"})	// Delete leaf
	if db3 {
		fmt.Printf ( "at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}
	*/

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

const db2 = false
const db3 = true
const db4 = false


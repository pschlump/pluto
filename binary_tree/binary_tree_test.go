package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
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
	found := Tree1.Remove(TestTreeNode{S: "00"})	// Delete leaf
	if db3 {
		fmt.Printf ( "at:%s tree=\n", godebug.LF())
	   	Tree1.Dump(os.Stdout)
	}
	if found == false {
		t.Errorf("Expected to find find a node to delete, did not." )
	}

}

const db2 = false
const db3 = true


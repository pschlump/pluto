package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*
	-- Add "Depth" -> int to get deepest part of tree
	-- Add "WalkInOrder, WalkPreOrder, WalkPostOrder"
*/

import (
	"fmt"
	"os"
	"strings"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/godebug"
	// "github.com/pschlump/MiscLib"
)

type BinaryTreeNode[T comparable.Comparable] struct {
	data 		*T
	left, right *BinaryTreeNode[T]
}

// BinaryTree is a generic binary tree
type BinaryTree[T comparable.Comparable] struct {
	root 	*BinaryTreeNode[T]
	length 	int
}

// IsEmpty will return true if the binary-tree is empty
func (tt BinaryTree[T]) IsEmpty() bool {
	if db1 {
		fmt.Printf ( "at:%s\n", godebug.LF())
	}
	return tt.root == nil 
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
func (tt *BinaryTree[T]) Insert(item T) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	node := &BinaryTreeNode[T]{ data : &item }
	node.left = nil
	node.right = nil
	if (*tt).IsEmpty() {
		tt.root = node
		tt.length = 1
		return
	}

	// Simple is recursive, can be replce with an iterative tree traversal.
	var insert func ( root **BinaryTreeNode[T] )
	insert = func ( root **BinaryTreeNode[T] ) {
		if *root == nil {
			*root = node
			tt.length++
		// } else if c := (*(node.data)).Compare( (*root).data ); c == 0 {
		} else if c := item.Compare( *((*root).data) ); c == 0 {
			(*root) = node
		} else if c < 0 {
			insert ( &( (*root).left ) )
		} else {
			insert ( &( (*root).right ) )
		}
	}

	insert ( &( (*tt).root ) )
}

// Length returns the number of elements in the list.
func (tt *BinaryTree[T]) Length() int {
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
func (tt *BinaryTree[T]) Search(find T) ( item *T ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return nil
	}

	// Iterative search through tree (can be used above)
	cur := tt.root
	for tt != nil {
		// fmt.Printf ( "at:%s\n", godebug.LF())
		c := find.Compare(*cur.data)
		if c == 0 {
			// fmt.Printf ( "FOUND! at:%s\n", godebug.LF())
			item = cur.data 
			return
		}
		// fmt.Printf ( "at:%s\n", godebug.LF())
		if c < 0 && cur.left != nil {
			cur = (*cur).left 
		} else if c > 0 && cur.right != nil {
			cur = (*cur).right 
		} else {
			// fmt.Printf ( "at:%s\n", godebug.LF())
			break
		}
	}
	// fmt.Printf ( "NOT Found --- at:%s\n", godebug.LF())
	return nil
}

// Dump will print out the tree to the file `fo`.
func (tt *BinaryTree[T]) Dump(fo *os.File) {
	var inorderTraversal func ( tt *BinaryTreeNode[T], n int, fo *os.File )
	inorderTraversal = func ( cur *BinaryTreeNode[T], n int, fo *os.File ) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal ( (*cur).left, n+1, fo);
		}
		fmt.Printf ( "%s%v (left=%p/%p, right=%p/%p) self=%p\n", strings.Repeat(" ",4*n), *((*cur).data), (*cur).left, &((*cur).left), (*cur).right, &((*cur).right), cur )
		if (*cur).right != nil {
			inorderTraversal ( (*cur).right, n+1, fo);
		}
	}
	inorderTraversal ( tt.root, 0, fo)
}


/*
func (tt *BinaryTree[T]) Remove(find T) ( found bool ) {

	// This is a little bit tricky.  To delare a local pointer to a function
	// that can recursivly call itslef you have to first declare the pointer
	// then initialize the pointer.   If you try to do this in one step 
	// it will error.  The variable is not declared until the end of the
	// funtion that initialized it - so it can't be used inside itself.
	var internalRemove func ( parent **BinaryTree[T], find T ) bool 
	internalRemove = func ( parent **BinaryTree[T], find T ) bool {
		// this := **parent
		fmt.Printf ( "interalRemove Top at:%s node=%+v\n", godebug.LF(), *((**parent).data))
		if cc := find.Compare ( *((**parent).data) ) ; cc == 0 {
			fmt.Printf ( "%sFound%s ! at:%s, *parent=%p/%p (**parent).left=%p\n", MiscLib.ColorGreen, MiscLib.ColorReset, godebug.LF(), *parent, parent, (**parent).left)
			*parent = (**parent).left
			return true
		} else if cc < 0 && (**parent).left != nil {
			fmt.Printf ( "at:%s\n", godebug.LF())
			return internalRemove ( &(**parent).left, find ) 
		} else if cc > 0 && (**parent).right != nil {
			fmt.Printf ( "at:%s\n", godebug.LF())
			return internalRemove ( &(**parent).right, find )
		} 
		fmt.Printf ( "at:%s\n", godebug.LF())
		return false
	}

	findLeftMost := func ( parent **BinaryTree[T], find T ) ( found bool, it *BinaryTree[T], pAtIt **BinaryTree[T] ) {
		fmt.Printf ( "at:%s\n", godebug.LF())
		this := **parent
		for this.right != nil {
			fmt.Printf ( "at:%s\n", godebug.LF())
			parent = &(this.right)
			this = **parent
		}
		fmt.Printf ( "at:%s\n", godebug.LF())
		it = (*parent)
		pAtIt = parent
		return
	}

	if c := find.Compare(*tt.data); c == 0 {
		fmt.Printf ( "at:%s\n", godebug.LF())
		// Found at "top" node.
		if (*tt).right != nil {
			// I think I need to go find the "left most" child in the right sub-tree
			fmt.Printf ( "at:%s\n", godebug.LF())
			found, it, pAtIt := findLeftMost ( &tt.right, find ) 
			if found {
				fmt.Printf ( "at:%s\n", godebug.LF())
				(*tt).data = it.data
				(*pAtIt) = it.right	// if most right node has a left node then promot it.
			} else {
				panic ( "Malformed Tree" )
			}
		} else if (*tt).left != nil {
			fmt.Printf ( "at:%s\n", godebug.LF())
			(*tt).data = ((*tt).left.data)
			(*tt).left = ((*tt).left.left)
		} else {
			fmt.Printf ( "at:%s\n", godebug.LF())
			(*tt).data = nil
		}
		return true
	} else if c < 0 && tt.left != nil {
		fmt.Printf ( "at:%s\n", godebug.LF())
		return internalRemove ( &tt.left, find )
	} else if c > 0 && tt.left != nil {
		fmt.Printf ( "at:%s\n", godebug.LF())
		return internalRemove ( &tt.right, find )
	} else {
		fmt.Printf ( "at:%s\n", godebug.LF())
		return false
	}
	fmt.Printf ( "at:%s\n", godebug.LF())

	return
	/ *
at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:158
interalRemove Top at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:105 node={S:02}
at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:110
interalRemove Top at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:105 node={S:00}
at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:107
at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree.go LineNo:116
at:File: /Users/philip/go/src/github.com/pschlump/pluto/binary_tree/binary_tree_test.go LineNo:129 tree=
        {00}
    {02}
        {03}
{05}
    {09}
	* /
}
*/

const db1 = true

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

// Truncate removes all data from the tree.
func (tt *BinaryTree[T]) Truncate()  {
	(*tt).root = nil 
	(*tt).length = 0
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
		c := find.Compare(*cur.data)
		if c == 0 {
			item = cur.data 
			return
		}
		if c < 0 && cur.left != nil {
			cur = (*cur).left 
		} else if c > 0 && cur.right != nil {
			cur = (*cur).right 
		} else {
			break
		}
	}
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
		fmt.Printf ( "%s%v%s (left=%p/%p, right=%p/%p) self=%p\n", strings.Repeat(" ",4*n), *((*cur).data), strings.Repeat(" ",20-(4*n)),  (*cur).left, &((*cur).left), (*cur).right, &((*cur).right), cur )
		if (*cur).right != nil {
			inorderTraversal ( (*cur).right, n+1, fo);
		}
	}
	inorderTraversal ( tt.root, 0, fo)
}


func (tt *BinaryTree[T]) Delete(find T) ( found bool ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return false
	}

	findLeftMostInRightSubtree := func ( parent **BinaryTreeNode[T] ) ( found bool, pAtIt **BinaryTreeNode[T] ) {
		// fmt.Printf ( "%sFindLeftMost/At Top: at:%s%s\n", MiscLib.ColorCyan, godebug.LF(), MiscLib.ColorReset)
		this := **parent
		if *parent == nil {
			// fmt.Printf ( "%sFindLeftMost/no tree: at:%s%s\n", MiscLib.ColorCyan, godebug.LF(), MiscLib.ColorReset)
			return
		}
		for this.right != nil {
			// fmt.Printf ( "%sAdvance 1 step. at:%s%s\n", MiscLib.ColorCyan, godebug.LF(), MiscLib.ColorReset)
			parent = &(this.right)
			this = **parent
		}
		// fmt.Printf ( "%sat bottom at:%s%s\n", MiscLib.ColorCyan, godebug.LF(), MiscLib.ColorReset)
		found = true
		pAtIt = parent
		return
	}

	// Iterative search through tree (can be used above)
	cur := &tt.root	// ptr to ptr to tree
	for tt != nil {
		// fmt.Printf ( "at:%s\n", godebug.LF())
		c := find.Compare(*(*cur).data)
		if c == 0 {
			// fmt.Printf ( "FOUND! now remove it! at:%s\n", godebug.LF())
			(*tt).length --
			if (*cur).left == nil && (*cur).right == nil {
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur) = nil // just delete the node, it has no children.
			} else if (*cur).left != nil && (*cur).right == nil {
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur) = (*cur).left // Has only left children, promote them.
			} else if (*cur).left == nil && (*cur).right != nil {
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur) = (*cur).right // Has only right children, promote them.
			} else { // has both children.
				// fmt.Printf ( "at:%s\n", godebug.LF())
				// Has only right children, promote them.
				found, pAtIt := findLeftMostInRightSubtree ( &((*cur).right) )	// Find lft mos of right sub-tree
				if !found {
					// fmt.Printf ( "%sAbout to Panic: Failed to have a subtree. AT:%s%s\n", MiscLib.ColorRed, godebug.LF(), MiscLib.ColorReset)
					panic ( "Can't have a missing sub-tree." )
				}
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur).data = (*pAtIt).data	// promote node's data.
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*pAtIt) = (*pAtIt).right // Left most can have a right sub-tree - but it is left most so it can't have a more left tree.
				// fmt.Printf ( "at:%s\n", godebug.LF())
			}
			return true
		}
		// fmt.Printf ( "at:%s\n", godebug.LF())
		if c < 0 && (*cur).left != nil {
			// fmt.Printf ( "Go Left at:%s\n", godebug.LF())
			cur = &((*cur).left)
		} else if c > 0 && (*cur).right != nil {
			// fmt.Printf ( "Go Right at:%s\n", godebug.LF())
			cur = &((*cur).right)
		} else {
			// fmt.Printf ( "not found - in loop - at:%s\n", godebug.LF())
			break
		}
	}
	// fmt.Printf ( "NOT Found --- at:%s\n", godebug.LF())
	return false
}

/*
        {00}
    {02}
        {03}
{05}
    {09}
*/

const db1 = false // print in IsEmpty

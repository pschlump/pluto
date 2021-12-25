package binary_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a Binary Tree.

* 	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(log|2(n))
* 	Index - return the Nth item	in the list - in a format usable with Delete.					O(n) 
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
* 	Reverse - Reverse all the nodes in list. 													O(n)
* 	Search — Returns the given element from a linked list.  Search is from head to tail.		O(log|2(n))
* 	Truncate - Delete all the nodes in list. 													O(1)
*	FindMin
*	FindMax
*	Depth -> int to get deepest part of tree

* 	DeleteAtHead — Deletes the first element of the linked list.  								O(log|2(n))
		Delete ( FindMin ( ) )
* 	DeleteAtTail — Deletes the last element of the linked list. 								O(log|2(n))
		Delete ( FindMax ( ) )

*	WalkInOrder
+	WalkPreOrder
+	WalkPostOrder

*/

import (
	"fmt"
	"os"
	"strings"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/godebug"
	"github.com/pschlump/pluto/g_lib"
	// "github.com/pschlump/MiscLib"
)

type BinaryTreeElement[T comparable.Comparable] struct {
	data 		*T
	left, right *BinaryTreeElement[T]
}

// BinaryTree is a generic binary tree
type BinaryTree[T comparable.Comparable] struct {
	root 	*BinaryTreeElement[T]
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
	node := &BinaryTreeElement[T]{ data : &item }
	node.left = nil
	node.right = nil
	if (*tt).IsEmpty() {
		tt.root = node
		tt.length = 1
		return
	}

	// Simple is recursive, can be replce with an iterative tree traversal.
	var insert func ( root **BinaryTreeElement[T] )
	insert = func ( root **BinaryTreeElement[T] ) {
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
	var inorderTraversal func ( cur *BinaryTreeElement[T], n int, fo *os.File )
	inorderTraversal = func ( cur *BinaryTreeElement[T], n int, fo *os.File ) {
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

	findLeftMostInRightSubtree := func ( parent **BinaryTreeElement[T] ) ( found bool, pAtIt **BinaryTreeElement[T] ) {
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

func (tt *BinaryTree[T]) FindMin() ( item *T ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return nil
	}

	// Iterative search through tree (can be used above)
	cur := tt.root
	if (*cur).left == nil {
		return (*cur).data
	}
	for cur.left != nil {
		cur = (*cur).left 
	}
	return (*cur).data
}

func (tt *BinaryTree[T]) FindMax() ( item *T ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return nil
	}

	// Iterative search through tree (can be used above)
	cur := tt.root
	if (*cur).right == nil {
		return (*cur).data
	}
	for cur.right != nil {
		cur = (*cur).right 
	}
	return (*cur).data
}

func (tt *BinaryTree[T]) DeleteAtHead() ( found bool ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return false
	}

	x := tt.FindMin()
	tt.Delete ( *x )
	return true
}

func (tt *BinaryTree[T]) DeleteAtTail() ( found bool ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return false
	}

	x := tt.FindMax()
	tt.Delete ( *x )
	return true
}

func (tt *BinaryTree[T]) Reverse() {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return 
	}

	var postTraversal func ( cur *BinaryTreeElement[T] )
	postTraversal = func ( cur *BinaryTreeElement[T] ) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			postTraversal ( (*cur).left )
		}
		if (*cur).right != nil {
			postTraversal ( (*cur).right )
		}
		(*cur).left, (*cur).right = (*cur).right, (*cur).left
	}
	postTraversal ( tt.root )
}

func (tt *BinaryTree[T]) Index(pos int) ( item *T ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return nil
	}

	if pos < 0 || pos >= tt.length {
		return nil
	}

	var n = 0
	var done = false
	var inorderTraversal func ( cur *BinaryTreeElement[T] )
	inorderTraversal = func ( cur *BinaryTreeElement[T] ) {
		if cur == nil {
			return
		}
		if ! done {
			if (*cur).left != nil {
				inorderTraversal ( (*cur).left )
			}
		}
		// fmt.Printf ( "InOrder - Before Set, Top n=%d, pos=%d,    value=%+v     at:%s\n", n, pos, item, godebug.LF() )
		if n == pos {
			item = (*cur).data
			// fmt.Printf ( "*********** Set \n")
			done = true
		}
		n++
		if ! done {
			if (*cur).right != nil {
				inorderTraversal ( (*cur).right )
			}
		}
	}
	inorderTraversal ( tt.root )
	return
}

func (tt *BinaryTree[T]) Depth() ( d int ) {
	if tt == nil {
		panic ( "tree sholud not be a nil" )
	}
	if (*tt).IsEmpty() {
		return 0
	}

	d = 0
	var inorderTraversal func ( cur *BinaryTreeElement[T] )
	inorderTraversal = func ( cur *BinaryTreeElement[T] ) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal ( (*cur).left )
			d = g_lib.Max[int]( d, d+1 )
		}
		if (*cur).right != nil {
			inorderTraversal ( (*cur).right )
			d = g_lib.Max[int]( d, d+1 )
		}
	}
	if tt.root != nil {
		inorderTraversal ( tt.root )
	} 
	return
}

type ApplyFunction[T comparable.Comparable] func ( pos, depth int, data *T, userData interface{} ) bool

func (tt *BinaryTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var inorderTraversal func ( cur *BinaryTreeElement[T], n int )
	inorderTraversal = func ( cur *BinaryTreeElement[T], n int ) {
		if cur == nil {
			return
		}
		if b { 
			if (*cur).left != nil {
				inorderTraversal ( (*cur).left, n+1 )
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx ( p, n, (*cur).data, userData )
		p++
		// ----------------------------------------------------------------------
		if b {
			if (*cur).right != nil {
				inorderTraversal ( (*cur).right, n+1 )
			}
		}
	}
	inorderTraversal ( tt.root, 0 )
}

func (tt *BinaryTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var preOrderTraversal func ( cur *BinaryTreeElement[T], n int )
	preOrderTraversal = func ( cur *BinaryTreeElement[T], n int ) {
		if cur == nil {
			return
		}
		// ----------------------------------------------------------------------
		b = b && fx ( p, n, (*cur).data, userData )
		// ----------------------------------------------------------------------
		if b { 
			if (*cur).left != nil {
				preOrderTraversal ( (*cur).left, n+1 )
			}
		}
		p++
		if b {
			if (*cur).right != nil {
				preOrderTraversal ( (*cur).right, n+1 )
			}
		}
	}
	preOrderTraversal ( tt.root, 0 )
}

func (tt *BinaryTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {

	p := 0
	b := true
	var postOrderTraversal func ( cur *BinaryTreeElement[T], n int )
	postOrderTraversal = func ( cur *BinaryTreeElement[T], n int ) {
		if cur == nil {
			return
		}
		if b { 
			if (*cur).left != nil {
				postOrderTraversal ( (*cur).left, n+1 )
			}
		}
		p++
		if b {
			if (*cur).right != nil {
				postOrderTraversal ( (*cur).right, n+1 )
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx ( p, n, (*cur).data, userData )
		// ----------------------------------------------------------------------
	}
	postOrderTraversal ( tt.root, 0 )
}


const db1 = false // print in IsEmpty

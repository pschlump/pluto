package binary_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a Binary Tree.

* 	Insert - create a new element in tree.														O(log|2(n))
* 	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(log|2(n))
* 	Index - return the Nth item	in the list - in a format usable with Delete.					O(n)
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Len — Returns number of elements in the list.  0 length is an empty list.				O(1)
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
	"io"
	"strings"
	"sync"

	"github.com/pschlump/godebug"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/g_lib"
	// "github.com/pschlump/MiscLib"
)

type BinaryTreeElement[T comparable.Comparable] struct {
	data        *T
	left, right *BinaryTreeElement[T]
}

// BinaryTree is a generic binary tree
type BinaryTree[T comparable.Comparable] struct {
	root   *BinaryTreeElement[T]
	length int
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// Create a new BinaryTree and return it.
// Complexity is O(1).
func NewBinaryTree[T comparable.Comparable]() *BinaryTree[T] {
	return &BinaryTree[T]{
		root:   nil,
		length: 0,
	}
}

// Complexity is O(1).
func (ee *BinaryTreeElement[T]) GetData() *T {
	return ee.data
}

// Complexity is O(1).
func (ee *BinaryTreeElement[T]) SetData(x *T) {
	ee.data = x
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the binary-tree is empty
func (tt *BinaryTree[T]) IsEmpty() bool {
	if db1 {
		fmt.Printf("at:%s\n", godebug.LF())
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.root == nil
}

func (tt *BinaryTree[T]) nlIsEmpty() bool {
	return tt.root == nil
}

// Truncate removes all data from the tree.
func (tt *BinaryTree[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	(*tt).root = nil
	(*tt).length = 0
}

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
func (tt *BinaryTree[T]) Insert(item *T) (vv bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	node := &BinaryTreeElement[T]{data: item}
	node.left = nil
	node.right = nil
	if (*tt).nlIsEmpty() {
		tt.root = node
		tt.length = 1
		return true
	}

	// Simple is recursive, can be replce with an iterative tree traversal.
	var insert func(root **BinaryTreeElement[T]) bool
	insert = func(root **BinaryTreeElement[T]) bool {
		if *root == nil {
			*root = node
			tt.length++
			// dbgo.Printf("%(green)True at %(LF): %+v\n", *root)
			return true
		} else if c := (*item).Compare(*((*root).data)); c == 0 {
			node.left = (*root).left
			node.right = (*root).right
			(*root) = node
			return false
		} else if c < 0 {
			return insert(&((*root).left))
		} else {
			return insert(&((*root).right))
		}
	}

	vv = insert(&((*tt).root))
	// fmt.Printf("for %+v returining %v\n", item, vv)
	return
}

// Length returns the number of elements in the list.
func (tt *BinaryTree[T]) Len() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return (*tt).length
}
func (tt *BinaryTree[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
func (tt *BinaryTree[T]) Search(find *T) (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if (*tt).nlIsEmpty() {
		return nil
	}

	// fmt.Printf("at:%s\n", godebug.LF())

	// Iterative search through tree (can be used above)
	cur := tt.root
	for tt != nil {
		// fmt.Printf(" at:%s ->%s<-\n", godebug.LF(), *cur.data)
		c := (*find).Compare(*cur.data)
		if c == 0 {
			// fmt.Printf("  %sfound%s at:%s\n", MiscLib.ColorGreen, MiscLib.ColorReset, godebug.LF())
			item = cur.data
			return
		}
		if c < 0 && cur.left != nil {
			// fmt.Printf("  left at:%s\n", godebug.LF())
			cur = (*cur).left
		} else if c > 0 && cur.right != nil {
			// fmt.Printf("  right at:%s\n", godebug.LF())
			cur = (*cur).right
		} else {
			// fmt.Printf("  ( not found / break loop ) at:%s\n", godebug.LF())
			break
		}
	}
	// fmt.Printf("all done at:%s\n", godebug.LF())
	return nil
}

// Dump will print out the tree to the file `fo`.
func (tt *BinaryTree[T]) Dump(fo io.Writer) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	k := tt.nlDepth() * 4
	var inorderTraversal func(cur *BinaryTreeElement[T], n int)
	inorderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal((*cur).left, n+1)
		}
		fmt.Fprintf(fo, "%s%v%s (left=%p/%p, right=%p/%p) self=%p\n", strings.Repeat(" ", 4*n), *((*cur).data), strings.Repeat(" ", k-(4*n)), (*cur).left, &((*cur).left), (*cur).right, &((*cur).right), cur)
		if (*cur).right != nil {
			inorderTraversal((*cur).right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

func (tt *BinaryTree[T]) Delete(find *T) (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.nlDelete(find)
}

func (tt *BinaryTree[T]) nlDelete(find *T) (found bool) {

	if (*tt).nlIsEmpty() {
		return false
	}

	findLeftMostInRightSubtree := func(parent **BinaryTreeElement[T]) (found bool, pAtIt **BinaryTreeElement[T]) {
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
	cur := &tt.root // ptr to ptr to tree
	for tt != nil {
		// fmt.Printf ( "at:%s\n", godebug.LF())
		c := (*find).Compare(*(*cur).data)
		if c == 0 {
			// fmt.Printf ( "FOUND! now remove it! at:%s\n", godebug.LF())
			(*tt).length--
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
				found, pAtIt := findLeftMostInRightSubtree(&((*cur).right)) // Find lft mos of right sub-tree
				if !found {
					// fmt.Printf ( "%sAbout to Panic: Failed to have a subtree. AT:%s%s\n", MiscLib.ColorRed, godebug.LF(), MiscLib.ColorReset)
					panic("Can't have a missing sub-tree.")
				}
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur).data = (*pAtIt).data // promote node's data.
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

func (tt *BinaryTree[T]) FindMin() (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMin()

}

func (tt *BinaryTree[T]) nlFindMin() (item *T) {
	if (*tt).nlIsEmpty() {
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

func (tt *BinaryTree[T]) FindMax() (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMax()
}

func (tt *BinaryTree[T]) nlFindMax() (item *T) {
	if (*tt).nlIsEmpty() {
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

func (tt *BinaryTree[T]) DeleteAtHead() (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if (*tt).nlIsEmpty() {
		return false
	}

	x := tt.nlFindMin()
	tt.nlDelete(x)
	return true
}

func (tt *BinaryTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if (*tt).nlIsEmpty() {
		return false
	}

	x := tt.nlFindMax()
	tt.nlDelete(x)
	return true
}

func (tt *BinaryTree[T]) Reverse() {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if (*tt).nlIsEmpty() {
		return
	}

	var postTraversal func(cur *BinaryTreeElement[T])
	postTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			postTraversal((*cur).left)
		}
		if (*cur).right != nil {
			postTraversal((*cur).right)
		}
		(*cur).left, (*cur).right = (*cur).right, (*cur).left
	}
	postTraversal(tt.root)
}

func (tt *BinaryTree[T]) Index(pos int) (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if (*tt).nlIsEmpty() {
		return nil
	}

	if pos < 0 || pos >= tt.length {
		return nil
	}

	var n = 0
	var done = false
	var inorderTraversal func(cur *BinaryTreeElement[T])
	inorderTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if !done {
			if (*cur).left != nil {
				inorderTraversal((*cur).left)
			}
		}
		// fmt.Printf ( "InOrder - Before Set, Top n=%d, pos=%d,    value=%+v     at:%s\n", n, pos, item, godebug.LF() )
		if n == pos {
			item = (*cur).data
			// fmt.Printf ( "*********** Set \n")
			done = true
		}
		n++
		if !done {
			if (*cur).right != nil {
				inorderTraversal((*cur).right)
			}
		}
	}
	inorderTraversal(tt.root)
	return
}

func (tt *BinaryTree[T]) Depth() (d int) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlDepth()
}

func (tt *BinaryTree[T]) nlDepth() (d int) {

	if (*tt).nlIsEmpty() {
		return 0
	}

	d = 0
	var inorderTraversal func(cur *BinaryTreeElement[T])
	inorderTraversal = func(cur *BinaryTreeElement[T]) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal((*cur).left)
			d = g_lib.Max[int](d, d+1)
		}
		if (*cur).right != nil {
			inorderTraversal((*cur).right)
			d = g_lib.Max[int](d, d+1)
		}
	}
	if tt.root != nil {
		inorderTraversal(tt.root)
	}
	return
}

type ApplyFunction[T comparable.Comparable] func(pos, depth int, data *T, userData interface{}) bool

func (tt *BinaryTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var inorderTraversal func(cur *BinaryTreeElement[T], n int)
	inorderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			if (*cur).left != nil {
				inorderTraversal((*cur).left, n+1)
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, (*cur).data, userData)
		p++
		// ----------------------------------------------------------------------
		if b {
			if (*cur).right != nil {
				inorderTraversal((*cur).right, n+1)
			}
		}
	}
	inorderTraversal(tt.root, 0)
}

func (tt *BinaryTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var preOrderTraversal func(cur *BinaryTreeElement[T], n int)
	preOrderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, (*cur).data, userData)
		// ----------------------------------------------------------------------
		if b {
			if (*cur).left != nil {
				preOrderTraversal((*cur).left, n+1)
			}
		}
		p++
		if b {
			if (*cur).right != nil {
				preOrderTraversal((*cur).right, n+1)
			}
		}
	}
	preOrderTraversal(tt.root, 0)
}

func (tt *BinaryTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var postOrderTraversal func(cur *BinaryTreeElement[T], n int)
	postOrderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if b {
			if (*cur).left != nil {
				postOrderTraversal((*cur).left, n+1)
			}
		}
		p++
		if b {
			if (*cur).right != nil {
				postOrderTraversal((*cur).right, n+1)
			}
		}
		// ----------------------------------------------------------------------
		b = b && fx(p, n, (*cur).data, userData)
		// ----------------------------------------------------------------------
	}
	postOrderTraversal(tt.root, 0)
}

/*
func (tt *Bi8naryTree[T]) DeleteMatch(fx ApplyFunction[T], userData interface{}) {

	p := 0
	var inorderTraversal func(cur *BinaryTreeElement[T], n int)
	inorderTraversal = func(cur *BinaryTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal((*cur).left, n+1)
		}

		// ----------------------------------------------------------------------
		// xyzzy TODO - how to delte at this point!
		// ----------------------------------------------------------------------
		if fx(p, n, (*cur).data, userData) {
			// tt . nlDelete(find *T) (found bool) {
			// xyzzy2
		}
		p++
		// ----------------------------------------------------------------------
		if (*cur).right != nil {
			inorderTraversal((*cur).right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}
*/

func (tt *BinaryTree[T]) DeleteMatch(find *T, fx func(a, b *T) int) (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.nlDeleteMatch(find, fx)
}

func (tt *BinaryTree[T]) nlDeleteMatch(find *T, fx func(a, b *T) int) (found bool) {

	if (*tt).nlIsEmpty() {
		return false
	}

	findLeftMostInRightSubtree := func(parent **BinaryTreeElement[T]) (found bool, pAtIt **BinaryTreeElement[T]) {
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
	cur := &tt.root // ptr to ptr to tree
	for tt != nil {
		// fmt.Printf ( "at:%s\n", godebug.LF())
		c := fx(find, (*cur).data) // OLD: c := (*find).Compare(*(*cur).data)
		if c == 0 {
			// fmt.Printf ( "FOUND! now remove it! at:%s\n", godebug.LF())
			(*tt).length--
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
				found, pAtIt := findLeftMostInRightSubtree(&((*cur).right)) // Find lft mos of right sub-tree
				if !found {
					// fmt.Printf ( "%sAbout to Panic: Failed to have a subtree. AT:%s%s\n", MiscLib.ColorRed, godebug.LF(), MiscLib.ColorReset)
					panic("Can't have a missing sub-tree.")
				}
				// fmt.Printf ( "at:%s\n", godebug.LF())
				(*cur).data = (*pAtIt).data // promote node's data.
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

const db1 = false // print in IsEmpty

/* vim: set noai ts=4 sw=4: */

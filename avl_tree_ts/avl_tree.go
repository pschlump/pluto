package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

/*

Basic operations on a AVL Binary Tree.

* 	Insert - create a new element in tree.														O(log|2(n))
*		Duplicates replace the current node with a new node - this is not reported as
*       a duplicate.
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
	"sync"

	"github.com/pschlump/godebug"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/g_lib"
	// "github.com/pschlump/MiscLib"
)

type AvlTreeElement[T comparable.Comparable] struct {
	data        *T
	height      int
	left, right *AvlTreeElement[T]
}

// AvlTree is a generic binary tree
type AvlTree[T comparable.Comparable] struct {
	root   *AvlTreeElement[T]
	length int
	lock   sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

func NewAvlTreeElement[T comparable.Comparable](x *T) *AvlTreeElement[T] {
	return &AvlTreeElement[T]{
		data:   x,
		height: 1,
		left:   nil,
		right:  nil,
	}
}

func (tt *AvlTree[T]) Height(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return e.height
}

// Complexity is O(1).
func (tt *AvlTree[T]) calcAvlBalance(e *AvlTreeElement[T]) int {
	if e == nil {
		return 0
	}
	return tt.Height(e.left) - tt.Height(e.right)
}

// Create a new AvlTree and return it.
// Complexity is O(1).
func NewAvlTree[T comparable.Comparable]() *AvlTree[T] {
	return &AvlTree[T]{
		root:   nil,
		length: 0,
	}
}

// Complexity is O(1).
func (ee *AvlTreeElement[T]) GetData() *T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the binary-tree is empty
func (tt *AvlTree[T]) IsEmpty() bool {
	if db1 {
		fmt.Printf("at:%s\n", godebug.LF())
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.root == nil
}

// nlIsEmpty a no-lock interal version that will return true if the binary-tree is empty
func (tt *AvlTree[T]) nlIsEmpty() bool {
	if db1 {
		fmt.Printf("at:%s\n", godebug.LF())
	}
	return tt.root == nil
}

// Truncate removes all data from the tree.
func (tt *AvlTree[T]) Truncate() {
	tt.lock.Lock()
	defer tt.lock.Unlock()
	(*tt).root = nil
	(*tt).length = 0
}

/*

Insert:

Steps to follow for insertion
Let the newly inserted node be w
1) Perform standard BST insert for w.
2) Starting from w, travel up and find the first unbalanced node. Let z be the first unbalanced node, y be the child of z that comes on the path from w to z and x be the grandchild of z that comes on the path from w to z.
3) Re-balance the tree by performing appropriate rotations on the subtree rooted with z. There can be 4 possible cases that needs to be handled as x, y and z can be arranged in 4 ways. Following are the possible 4 arrangements:
	a) y is left child of z and x is left child of y (Left Left Case)
	b) y is left child of z and x is right child of y (Left Right Case)
	c) y is right child of z and x is right child of y (Right Right Case)
	d) y is right child of z and x is left child of y (Right Left Case)
Following are the operations to be performed in above mentioned 4 cases. In all of the cases, we only need to re-balance the subtree rooted with z and the complete tree
becomes balanced as the height of subtree (After appropriate rotations) rooted with z becomes same as it was before insertion. (See this video lecture for proof)
a) Left Left Case

T1, T2, T3 and T4 are subtrees.
         z                                      y
        / \                                   /   \
       y   T4      Right Rotate (z)          x      z
      / \          - - - - - - - - ->      /  \    /  \
     x   T3                               T1  T2  T3  T4
    / \
  T1   T2

b) Left Right Case

     z                               z                           x
    / \                            /   \                        /  \
   y   T4  Left Rotate (y)        x    T4  Right Rotate(z)    y      z
  / \      - - - - - - - - ->    /  \      - - - - - - - ->  / \    / \
T1   x                          y    T3                    T1  T2 T3  T4
    / \                        / \
  T2   T3                    T1   T2

c) Right Right Case

  z                                y
 /  \                            /   \
T1   y     Left Rotate(z)       z      x
    /  \   - - - - - - - ->    / \    / \
   T2   x                     T1  T2 T3  T4
       / \
     T3  T4

d) Right Left Case

   z                            z                            x
  / \                          / \                          /  \
T1   y   Right Rotate (y)    T1   x      Left Rotate(z)   z      y
    / \  - - - - - - - - ->     /  \   - - - - - - - ->  / \    / \
   x   T4                      T2   y                  T1  T2  T3  T4
  / \                              /  \
T2   T3                           T3   T4


i
*/

// Insert will add a new item to the tree.  If it is a duplicate of an exiting
// item the new item will replace the existing one.
func (tt *AvlTree[T]) Insert(item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	node := NewAvlTreeElement[T](item)
	if (*tt).nlIsEmpty() {
		tt.root = node
		tt.length = 1
		return
	}

	// Recursive with tail-recursion handeling the AVL rotation.
	var insert func(root **AvlTreeElement[T])
	insert = func(root **AvlTreeElement[T]) {
		if *root == nil {
			*root = node
			tt.length++
		} else if c := (*item).Compare(*((*root).data)); c == 0 {
			// Replace duplicate node with new node.
			node.left = (*root).left
			node.right = (*root).right
			(*root) = node
		} else if c < 0 {
			insert(&((*root).left))
		} else {
			insert(&((*root).right))
		}

		// AVL section ----------------------------------------------------------------------------------
		(*root).height = g_lib.Max(tt.Height((*root).left), tt.Height((*root).right)) + 1

		b := tt.calcAvlBalance(*root)

		if g_lib.Abs(b) > 1 { // If we have a height difference that is larer than 1 ( may be < -2, or +2.

			z := (*root) // can change 'z' via *root
			if z != nil && z.left != nil && z.left.left != nil {
				// a) Left Left Case
				// t1, t2, t3 and t4 are subtrees.
				//          z                                      y
				//        / \                                   /   \
				//       y   T4      Right Rotate (z)          x      z
				//      / \          - - - - - - - - ->      /  \    /  \
				//     x   T3                               T1  T2  T3  T4
				//    / \
				//  T1   T2
				y := z.left
				x := y.left
				t4 := z.right
				t3 := y.right
				t2 := x.right
				t1 := x.left
				y.left = x
				y.right = z
				x.left = t1
				x.right = t2
				z.left = t3
				z.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				x.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				z.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				y.height = g_lib.Max(tt.Height(x), tt.Height(z)) + 1
				(*root) = y

			} else if z != nil && z.left != nil && z.left.right != nil {
				// b) Left Right Case
				// T1, T2, T3 and T4 are subtrees.
				//      z                               z                           x
				//     / \                            /   \                        /  \
				//    y   T4  Left Rotate (y)        x    T4  Right Rotate(z)    y      z
				//   / \      - - - - - - - - ->    /  \      - - - - - - - ->  / \    / \
				// T1   x                          y    T3                    T1  T2 T3  T4
				//     / \                        / \
				//   T2   T3                    T1   T2
				y := z.left
				x := y.right
				t4 := z.right
				t3 := x.right
				t2 := x.left
				t1 := y.left
				x.left = y
				x.right = z
				y.left = t1
				y.right = t2
				z.left = t3
				z.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				y.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				z.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				x.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = x

			} else if z != nil && z.right != nil && z.right.right != nil {
				// c) Right Right Case
				// T1, T2, T3 and T4 are subtrees.
				//   z                                y
				//  /  \                            /   \
				// T1   y     Left Rotate(z)       z      x
				//     /  \   - - - - - - - ->    / \    / \
				//    T2   x                     T1  T2 T3  T4
				//        / \
				//      T3  T4
				y := z.right
				x := y.right
				t4 := x.right
				t3 := x.left
				t2 := y.left
				t1 := z.left
				y.left = z
				y.right = x
				z.left = t1
				z.right = t2
				x.left = t3
				x.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				z.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				x.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				y.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = y

			} else if z != nil && z.right != nil && z.right.left != nil {
				// d) Right Left Case
				// T1, T2, T3 and T4 are subtrees.
				//    z                            z                            x
				//   / \                          / \                          /  \
				// T1   y   Right Rotate (y)    T1   x      Left Rotate(z)   z      y
				//     / \  - - - - - - - - ->     /  \   - - - - - - - ->  / \    / \
				//    x   T4                      T2   y                  T1  T2  T3  T4
				//   / \                              /  \
				// T2   T3                           T3   T4
				y := z.right
				x := y.left
				t4 := y.right
				t3 := x.right
				t2 := x.left
				t1 := z.left
				x.left = z
				x.right = y
				z.left = t1
				z.right = t2
				y.left = t3
				y.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				z.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				y.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				x.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = x

			} else {
				panic("should never get to this point")
			}
		}
	}

	insert(&((*tt).root))

}

// Length returns the number of elements in the list.
func (tt *AvlTree[T]) Length() int {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return (*tt).length
}

// Search will walk the tree looking for `find` and retrn the found item
// if it is in the tree. If it is not found then `nil` will be returned.
func (tt *AvlTree[T]) Search(find *T) (item *T) {
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
func (tt *AvlTree[T]) Dump(fo *os.File) {
	tt.lock.RLock()
	defer tt.lock.RUnlock()

	k := tt.nlDepth() * 4
	var inorderTraversal func(cur *AvlTreeElement[T], n int)
	inorderTraversal = func(cur *AvlTreeElement[T], n int) {
		if cur == nil {
			return
		}
		if (*cur).left != nil {
			inorderTraversal((*cur).left, n+1)
		}
		fmt.Printf("%s%v%s (left=%p/%p, right=%p/%p) self=%p\n", strings.Repeat(" ", 4*n), *((*cur).data), strings.Repeat(" ", k-(4*n)), (*cur).left, &((*cur).left), (*cur).right, &((*cur).right), cur)
		if (*cur).right != nil {
			inorderTraversal((*cur).right, n+1)
		}
	}
	inorderTraversal(tt.root, 0)
}

func (tt *AvlTree[T]) Delete(find *T) (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	return tt.nlDelete(find)
}

func (tt *AvlTree[T]) nlDelete(find *T) (found bool) {

	if (*tt).nlIsEmpty() {
		return false
	}

	findLeftMostInRightSubtree := func(parent **AvlTreeElement[T]) (found bool, pAtIt **AvlTreeElement[T]) {
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
			// return true
			goto rb
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

	// Not Found
	return false

rb:
	// Recursive with tail-recursion handeling the AVL rotation.
	var rebalance func(root **AvlTreeElement[T])
	rebalance = func(root **AvlTreeElement[T]) {
		if *root == nil {
			// *root = node
			// tt.length++
			// } else if c := (*(node.data)).Compare( (*root).data ); c == 0 {
			return
		} else if c := (*find).Compare(*((*root).data)); c == 0 {
			// (*root) = node
			return
		} else if c < 0 {
			rebalance(&((*root).left))
		} else {
			rebalance(&((*root).right))
		}

		// AVL section ----------------------------------------------------------------------------------
		(*root).height = g_lib.Max(tt.Height((*root).left), tt.Height((*root).right)) + 1

		b := tt.calcAvlBalance(*root)

		if g_lib.Abs(b) > 1 { // If we have a height difference that is larer than 1 ( may be < -2, or +2.

			z := (*root) // can change 'z' via *root
			if z != nil && z.left != nil && z.left.left != nil {
				// a) Left Left Case
				// t1, t2, t3 and t4 are subtrees.
				//          z                                      y
				//        / \                                   /   \
				//       y   T4      Right Rotate (z)          x      z
				//      / \          - - - - - - - - ->      /  \    /  \
				//     x   T3                               T1  T2  T3  T4
				//    / \
				//  T1   T2
				y := z.left
				x := y.left
				t4 := z.right
				t3 := y.right
				t2 := x.right
				t1 := x.left
				y.left = x
				y.right = z
				x.left = t1
				x.right = t2
				z.left = t3
				z.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				x.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				z.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				y.height = g_lib.Max(tt.Height(x), tt.Height(z)) + 1
				(*root) = y

			} else if z != nil && z.left != nil && z.left.right != nil {
				// b) Left Right Case
				// T1, T2, T3 and T4 are subtrees.
				//      z                               z                           x
				//     / \                            /   \                        /  \
				//    y   T4  Left Rotate (y)        x    T4  Right Rotate(z)    y      z
				//   / \      - - - - - - - - ->    /  \      - - - - - - - ->  / \    / \
				// T1   x                          y    T3                    T1  T2 T3  T4
				//     / \                        / \
				//   T2   T3                    T1   T2
				y := z.left
				x := y.right
				t4 := z.right
				t3 := x.right
				t2 := x.left
				t1 := y.left
				x.left = y
				x.right = z
				y.left = t1
				y.right = t2
				z.left = t3
				z.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				y.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				z.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				x.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = x

			} else if z != nil && z.right != nil && z.right.right != nil {
				// c) Right Right Case
				// T1, T2, T3 and T4 are subtrees.
				//   z                                y
				//  /  \                            /   \
				// T1   y     Left Rotate(z)       z      x
				//     /  \   - - - - - - - ->    / \    / \
				//    T2   x                     T1  T2 T3  T4
				//        / \
				//      T3  T4
				y := z.right
				x := y.right
				t4 := x.right
				t3 := x.left
				t2 := y.left
				t1 := z.left
				y.left = z
				y.right = x
				z.left = t1
				z.right = t2
				x.left = t3
				x.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				z.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				x.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				y.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = y

			} else if z != nil && z.right != nil && z.right.left != nil {
				// d) Right Left Case
				// T1, T2, T3 and T4 are subtrees.
				//    z                            z                            x
				//   / \                          / \                          /  \
				// T1   y   Right Rotate (y)    T1   x      Left Rotate(z)   z      y
				//     / \  - - - - - - - - ->     /  \   - - - - - - - ->  / \    / \
				//    x   T4                      T2   y                  T1  T2  T3  T4
				//   / \                              /  \
				// T2   T3                           T3   T4
				y := z.right
				x := y.left
				t4 := y.right
				t3 := x.right
				t2 := x.left
				t1 := z.left
				x.left = z
				x.right = y
				z.left = t1
				z.right = t2
				y.left = t3
				y.right = t4
				// re-calculate - the heights based on the "subtrees" (t1, t2, t3, t4)
				z.height = g_lib.Max(tt.Height(t1), tt.Height(t2)) + 1
				y.height = g_lib.Max(tt.Height(t3), tt.Height(t4)) + 1
				x.height = g_lib.Max(tt.Height(y), tt.Height(z)) + 1
				(*root) = x

			} else {
				panic("should never get to this point")
			}
		}
	}

	rebalance(&((*tt).root))

	return true
}

/*
        {00}
    {02}
        {03}
{05}
    {09}
*/

func (tt *AvlTree[T]) FindMin() (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMin()
}

func (tt *AvlTree[T]) nlFindMin() (item *T) {
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

// FindMax returns the largest value in the tree.
func (tt *AvlTree[T]) FindMax() (item *T) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlFindMax()
}

// nlFindMax returns the largest value in the tree without locking.
func (tt *AvlTree[T]) nlFindMax() (item *T) {
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

func (tt *AvlTree[T]) DeleteAtHead() (found bool) {
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

func (tt *AvlTree[T]) DeleteAtTail() (found bool) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if (*tt).nlIsEmpty() {
		return false
	}

	x := tt.nlFindMax()
	tt.nlDelete(x)
	return true
}

// Reverse swaps the order of all the nodes in the AVL Tree
func (tt *AvlTree[T]) Reverse() {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()

	if (*tt).nlIsEmpty() {
		return
	}

	var postTraversal func(cur *AvlTreeElement[T])
	postTraversal = func(cur *AvlTreeElement[T]) {
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

func (tt *AvlTree[T]) Index(pos int) (item *T) {
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
	var inorderTraversal func(cur *AvlTreeElement[T])
	inorderTraversal = func(cur *AvlTreeElement[T]) {
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

func (tt *AvlTree[T]) Depth() (d int) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	return tt.nlDepth()
}

func (tt *AvlTree[T]) nlDepth() (d int) {
	if tt == nil {
		panic("tree sholud not be a nil")
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if (*tt).nlIsEmpty() {
		return 0
	}

	d = 0
	var inorderTraversal func(cur *AvlTreeElement[T])
	inorderTraversal = func(cur *AvlTreeElement[T]) {
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

func (tt *AvlTree[T]) WalkInOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var inorderTraversal func(cur *AvlTreeElement[T], n int)
	inorderTraversal = func(cur *AvlTreeElement[T], n int) {
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

func (tt *AvlTree[T]) WalkPreOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var preOrderTraversal func(cur *AvlTreeElement[T], n int)
	preOrderTraversal = func(cur *AvlTreeElement[T], n int) {
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

func (tt *AvlTree[T]) WalkPostOrder(fx ApplyFunction[T], userData interface{}) {

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	p := 0
	b := true
	var postOrderTraversal func(cur *AvlTreeElement[T], n int)
	postOrderTraversal = func(cur *AvlTreeElement[T], n int) {
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

const db1 = false // print in IsEmpty

/* vim: set noai ts=4 sw=4: */

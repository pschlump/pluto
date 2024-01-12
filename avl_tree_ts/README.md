# AVL Trees with Locks

If you have an application that is not subject to concurrency but still requires a balanced tree 
use the `../avl_tree` version of this code.  They have the same interface.

AVL Trees are a balanced binary tree.  They are a little bit more complicated to implement than
red-black balanced trees but in most cases perform a little better.   There is a difference in
the amount of storage between them but this has no effect when the minimum amount of storage is
1 byte in size.

With AVL Trees the depth of the tree is always within 1 of the maximum depth.  Searches, on average,
benefit from this balance.

Detailed analysis can be found at [https://en.wikipedia.org/wiki/AVL_tree](https://en.wikipedia.org/wiki/AVL_tree).

## Operations

### Insert

Create a new element in tree. Duplicates replace the current node with a new node - this is not reported as
an error.

Time: O(log|2(n))

```

	var Tree1 AvlTree[DataType]
    // ...
    Tree1.Insert(&DataType{...})

```

* 	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(log|2(n))
* 	Index - return the Nth item	in the list - in a format usable with Delete.					O(n)
* 	IsEmpty — Returns true if the linked list is empty											O(1)
* 	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
* 	Reverse - Reverse all the nodes in list. 													O(n)
* 	Search — Returns the given element from a linked list.  Search is from head to tail.		O(log|2(n))
* 	Truncate - Delete all the nodes in list. 													O(1)
*	FindMin - Find the smallest element in the tree.
*	FindMax - Find the largest element in the tree.
*	Depth -> int to get deepest part of tree

* 	DeleteAtHead — Deletes the first element of the linked list.  								O(log|2(n))
		=== Delete ( FindMin ( ) )
* 	DeleteAtTail — Deletes the last element of the linked list. 								O(log|2(n))
		=== Delete ( FindMax ( ) )

*	WalkInOrder	- Apply a function to all the nodes in the tree using an inorder traversal.		O(n)
*	WalkPreOrder - Apply a function to all the nodes in the tree using a preorder traversal.	O(n)
*	WalkPostOrder - Apply a function to all the nodes in the tree using a postorder traversal.	O(n)

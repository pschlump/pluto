/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_tree_ts

// WalkFunc applies `Fx` to every element of the tree in pre-order
// (node, left, right) order.  The read lock is held for the whole walk:
// Fx must not call methods on the same tree, or the call can deadlock.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkFunc(Fx func(a T)) {
	if tt == nil {
		return
	}

	tt.lock.RLock()
	defer tt.lock.RUnlock()

	if tt.nlIsEmpty() {
		return
	}

	var apply func(root *BinaryTreeElement[T])
	apply = func(root *BinaryTreeElement[T]) {
		if root == nil {
			return
		}
		Fx(root.data)
		apply(root.left)
		apply(root.right)
	}

	apply(tt.root)
}

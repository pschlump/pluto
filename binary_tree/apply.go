/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_tree

// WalkFunc applies `Fx` to every element of the tree in pre-order
// (node, left, right) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkFunc(Fx func(a T)) {
	if tt == nil || tt.IsEmpty() {
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

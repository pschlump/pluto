package binary_tree

// WalkFunc applies `Fx` to every element of the tree in pre-order
// (node, left, right) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkFunc(Fx func(a *T)) {
	if tt == nil {
		panic("binary_tree: WalkFunc called on a nil tree")
	}
	if tt.IsEmpty() {
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

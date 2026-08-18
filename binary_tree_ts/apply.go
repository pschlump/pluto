package binary_tree_ts

// WalkFunc applies `Fx` to every element of the tree in pre-order
// (node, left, right) order.
// Complexity is O(n).
func (tt *BinaryTree[T]) WalkFunc(Fx func(a *T)) {
	if tt == nil {
		panic("binary_tree_ts: WalkFunc called on a nil tree")
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

package binary_tree_ts

func (tt *BinaryTree[T]) WalkFunc(Fx func(a *T)) {
	//	for i := 0; i < tt.length; i++ {
	//		if tt.buckets[i] != nil {
	//			tt.buckets[i].WalkFunc(Fx)
	//		}
	//	}
	if tt == nil {
		panic("tree sholud not be a nil")
	}
	if (*tt).IsEmpty() {
		return
	}

	// Simple is recursive, can be replce with an iterative tree traversal.
	var apply func(root **BinaryTreeElement[T])
	apply = func(root **BinaryTreeElement[T]) {
		if *root == nil {
			return
		} else {
			Fx((*root).data)
			apply(&((*root).left))
			apply(&((*root).right))
		}
	}

	apply(&((*tt).root))
	return
}

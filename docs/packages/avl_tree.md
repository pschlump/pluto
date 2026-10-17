# avl_tree notes

`AvlTree[T any]` mirrors the binary_tree contract but every node caches its subtree height (`Depth` is O(1)), and `rebalanceNode`/`rotateLeft`/`rotateRight` restore the AVL invariant on the way back up from each insert/delete (delete's two-children case unlinks the in-order successor via `removeMin`). Unlike binary_tree: `Insert` returns bool (added vs duplicate-replaced), there is no `DeleteMatch`/`WalkFunc`, `Len` aliases `Length`, and there are set operations — `Copy`/`Union`/`Minus`/`Intersect` snapshot their operands before rebuilding the destination (so a source may alias the destination) and a zero-value destination adopts an operand's comparison function. `validateAVLNode` in avl_more_test.go checks the BST order, AVL balance, and cached-height invariants — call it after any structural change.

`avl_tree_ts` adds the `lock sync.RWMutex` and `nl*` lock-free internals to the same structure; its `snapshot()` returns both the in-order data and the tree's comparison function read under the read lock, which is how the set operations adopt a comparator without racing with concurrent adoption on the operand.


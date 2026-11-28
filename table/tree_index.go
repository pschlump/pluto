/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import "iter"

// treeNode is one AVL node. key and value live in the node itself (no wrapper
// entry), and height is the node's subtree height (a leaf has height 1).
type treeNode[K any, V any] struct {
	left, right *treeNode[K, V]
	key         K
	value       V
	height      int32
}

// treeIndex is the private ordered key/value index behind TreeTable: an AVL
// tree ordered by the stored comparison function.
//
// Writes descend once. put and delete record the descent in a fixed-size
// stack array of link addresses, then rebalance bottom-up along it; a
// replacement updates the found node in place and allocates nothing.
// Traversals are method recursion or an explicit stack, never a recursive
// closure, so they do not allocate either.
//
// Bulk builders (buildTreeIndex, clone) allocate a subtree's nodes as one
// slab. A node unlinked from a slab stays allocated until every node of that
// slab is unreachable; the slab is bounded by the index size when it was
// built. Unlinked nodes are zeroed so they retain no key or value.
type treeIndex[K any, V any] struct {
	root    *treeNode[K, V]
	count   int
	compare func(K, K) int
}

// maxTreeHeight bounds the descent stack. An AVL tree of height h holds at
// least Fib(h+2)-1 nodes, so height 92 would need more than 2^63 nodes.
const maxTreeHeight = 92

func newTreeIndex[K any, V any](compare func(K, K) int) *treeIndex[K, V] {
	return &treeIndex[K, V]{compare: compare}
}

func treeHeight[K any, V any](node *treeNode[K, V]) int32 {
	if node == nil {
		return 0
	}
	return node.height
}

func (node *treeNode[K, V]) fixHeight() {
	node.height = max(treeHeight(node.left), treeHeight(node.right)) + 1
}

func rotateRight[K any, V any](node *treeNode[K, V]) *treeNode[K, V] {
	left := node.left
	node.left = left.right
	left.right = node
	node.fixHeight()
	left.fixHeight()
	return left
}

func rotateLeft[K any, V any](node *treeNode[K, V]) *treeNode[K, V] {
	right := node.right
	node.right = right.left
	right.left = node
	node.fixHeight()
	right.fixHeight()
	return right
}

// rebalance refreshes the height of *link and restores the AVL balance there
// with a single or double rotation.
func rebalance[K any, V any](link **treeNode[K, V]) {
	node := *link
	node.fixHeight()
	switch balance := treeHeight(node.left) - treeHeight(node.right); {
	case balance > 1:
		if treeHeight(node.left.left) < treeHeight(node.left.right) {
			node.left = rotateLeft(node.left)
		}
		*link = rotateRight(node)
	case balance < -1:
		if treeHeight(node.right.right) < treeHeight(node.right.left) {
			node.right = rotateRight(node.right)
		}
		*link = rotateLeft(node)
	}
}

// node returns the node holding key, or nil. Complexity is O(log n).
func (index *treeIndex[K, V]) node(key K) *treeNode[K, V] {
	if index == nil {
		return nil
	}
	for node := index.root; node != nil; {
		switch c := index.compare(key, node.key); {
		case c < 0:
			node = node.left
		case c > 0:
			node = node.right
		default:
			return node
		}
	}
	return nil
}

// get returns key's value. Complexity is O(log n).
func (index *treeIndex[K, V]) get(key K) (V, bool) {
	if node := index.node(key); node != nil {
		return node.value, true
	}
	var zero V
	return zero, false
}

// put inserts or replaces key's entry and returns the previous value. A
// replacement stores the new key as well as the value, as an equal key
// replaces the whole entry. Complexity is O(log n).
func (index *treeIndex[K, V]) put(key K, value V) (V, bool) {
	var path [maxTreeHeight]**treeNode[K, V]
	depth := 0
	link := &index.root
	for *link != nil {
		node := *link
		c := index.compare(key, node.key)
		if c == 0 {
			old := node.value
			node.key, node.value = key, value
			return old, true
		}
		path[depth] = link
		depth++
		if c < 0 {
			link = &node.left
		} else {
			link = &node.right
		}
	}
	*link = &treeNode[K, V]{key: key, value: value, height: 1}
	index.count++
	// An insertion changes heights only up to the first ancestor whose height
	// survives rebalancing; a rotation restores the pre-insert height.
	for depth > 0 {
		depth--
		ancestor := path[depth]
		before := (*ancestor).height
		rebalance(ancestor)
		if (*ancestor).height == before {
			break
		}
	}
	var zero V
	return zero, false
}

// delete removes key's entry and returns its value. Complexity is O(log n).
func (index *treeIndex[K, V]) delete(key K) (V, bool) {
	var path [maxTreeHeight]**treeNode[K, V]
	depth := 0
	link := &index.root
	for {
		node := *link
		if node == nil {
			var zero V
			return zero, false
		}
		c := index.compare(key, node.key)
		if c == 0 {
			break
		}
		path[depth] = link
		depth++
		if c < 0 {
			link = &node.left
		} else {
			link = &node.right
		}
	}
	target := *link
	old := target.value
	removed := target
	switch {
	case target.left == nil:
		*link = target.right
	case target.right == nil:
		*link = target.left
	default:
		// Two children: move the in-order successor's entry into target and
		// unlink the successor, which has no left child.
		path[depth] = link
		depth++
		successorLink := &target.right
		for (*successorLink).left != nil {
			path[depth] = successorLink
			depth++
			successorLink = &(*successorLink).left
		}
		successor := *successorLink
		target.key, target.value = successor.key, successor.value
		*successorLink = successor.right
		removed = successor
	}
	*removed = treeNode[K, V]{}
	index.count--
	for depth > 0 {
		depth--
		rebalance(path[depth])
	}
	return old, true
}

func (index *treeIndex[K, V]) len() int {
	if index == nil {
		return 0
	}
	return index.count
}

// all iterates in ascending key order with an explicit stack. Do not mutate
// while iterating.
func (index *treeIndex[K, V]) all() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if index == nil {
			return
		}
		var stack [maxTreeHeight]*treeNode[K, V]
		depth := 0
		node := index.root
		for node != nil || depth > 0 {
			for node != nil {
				stack[depth] = node
				depth++
				node = node.left
			}
			depth--
			node = stack[depth]
			if !yield(node.key, node.value) {
				return
			}
			node = node.right
		}
	}
}

// appendRowCells appends the subtree's entries, in order, as cells of row.
func appendRowCells[R any, C any, V any](cells []Cell[R, C, V], row R, node *treeNode[C, V]) []Cell[R, C, V] {
	for node != nil {
		cells = appendRowCells(cells, row, node.left)
		cells = append(cells, Cell[R, C, V]{Row: row, Column: node.key, Value: node.value})
		node = node.right
	}
	return cells
}

// buildTreeIndex builds a perfectly balanced index from entries already in
// strictly ascending key order, allocating all nodes as one slab.
// Complexity is O(n).
func buildTreeIndex[K any, V any](compare func(K, K) int, keys []K, values []V) treeIndex[K, V] {
	index := treeIndex[K, V]{compare: compare, count: len(keys)}
	if len(keys) > 0 {
		slab := make([]treeNode[K, V], len(keys))
		index.root = buildBalanced(slab, keys, values)
	}
	return index
}

func buildBalanced[K any, V any](slab []treeNode[K, V], keys []K, values []V) *treeNode[K, V] {
	if len(keys) == 0 {
		return nil
	}
	middle := len(keys) / 2
	node := &slab[middle]
	node.key, node.value = keys[middle], values[middle]
	node.left = buildBalanced(slab[:middle], keys[:middle], values[:middle])
	node.right = buildBalanced(slab[middle+1:], keys[middle+1:], values[middle+1:])
	node.fixHeight()
	return node
}

// clone copies the index structurally into one slab: same shape, same
// entries, no comparisons. Values are copied by assignment.
// Complexity is O(n).
func (index *treeIndex[K, V]) clone() *treeIndex[K, V] {
	result := &treeIndex[K, V]{compare: index.compare, count: index.count}
	if index.count > 0 {
		slab := make([]treeNode[K, V], index.count)
		next := 0
		result.root = cloneNodes(index.root, slab, &next)
	}
	return result
}

func cloneNodes[K any, V any](node *treeNode[K, V], slab []treeNode[K, V], next *int) *treeNode[K, V] {
	if node == nil {
		return nil
	}
	copied := &slab[*next]
	*next++
	copied.key, copied.value, copied.height = node.key, node.value, node.height
	copied.left = cloneNodes(node.left, slab, next)
	copied.right = cloneNodes(node.right, slab, next)
	return copied
}

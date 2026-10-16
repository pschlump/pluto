/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"iter"

	"github.com/pschlump/pluto/g_lib"
)

// All returns a range-over-func iterator that visits every point of the
// tree in tree order — node, then left subtree, then right subtree
// (preorder) — which is NOT sorted order; the partition alternates
// coordinates by depth, so no single sort matches it.  Sort the visited
// points when order matters.
//
//	for p := range tree.All() { ... }
//
// A nil *KdTree iterates as an empty one.  The iterator walks the live
// tree: the tree must not be modified while the iterator is being
// consumed.
// Complexity is O(n) for a full traversal, O(1) extra space.
func (t *KdTree) All() iter.Seq[g_lib.Point2D] {
	return func(yield func(g_lib.Point2D) bool) {
		if t == nil {
			return
		}
		var walk func(n *kdNode) bool
		walk = func(n *kdNode) bool {
			if n == nil {
				return true
			}
			if !yield(n.p) {
				return false
			}
			return walk(n.left) && walk(n.right)
		}
		walk(t.root)
	}
}

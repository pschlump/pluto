/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree

// Linear-time machinery behind the whole-tree set operations: a merge of
// two ascending sequences and a perfectly balanced build from the merged
// result.
//
// The operands' in-order snapshots are ascending under their OWN
// comparison functions; the result must be ordered by the destination's.
// Those agree in practice (a zero-value destination adopts an operand's
// function, tree_set passes its own to both sides), but they need not —
// a destination with a different comparator, a Reverse()d operand, or an
// operand ordered by a finer comparator (adjacent elements equal under
// the destination's) yields a snapshot that is not strictly ascending
// under the destination's ordering.  Function values cannot be compared,
// so every snapshot is checked (one comparison per adjacent pair; Copy
// checks its node slab in place); one that is not strictly ascending
// sends the operation to the original insert-per-element algorithm,
// which is correct for any input, so results are identical to it in
// every case.
//
// Memory: the balanced build allocates all of a result's nodes as one
// slab (one allocation instead of one per element).  A node unlinked from
// a slab by a later Delete stays allocated until every node of that slab
// is unreachable, so a result that is mostly deleted afterwards can
// retain up to (size at build time) × node size until the tree is
// truncated or rebuilt.  Unlinked nodes are zeroed (see remove), so the
// retained memory holds no element data and keeps nothing else alive.
// Nodes added by later Inserts are allocated individually, as always.

// setOp selects which elements a merge of two ascending sequences keeps.
type setOp uint8

const (
	opUnion     setOp = iota // a ∪ b; on equal elements b's is kept
	opMinus                  // a − b; a's elements
	opIntersect              // a ∩ b; on equal elements a's is kept
	opSymDiff                // a △ b; equal elements are dropped
)

// strictlyAscending reports whether s is strictly ascending under cmp.
// Complexity is O(n).
func strictlyAscending[T any](cmp func(a, b T) int, s []T) bool {
	for i := 1; i < len(s); i++ {
		if cmp(s[i-1], s[i]) >= 0 {
			return false
		}
	}
	return true
}

// mergeInto merges the strictly ascending sequences a and b under cmp for
// Union (union=true: every element, b's on equal) or SymmetricDifference
// (union=false: equal elements dropped) and returns how many elements
// that keeps.  When dst is non-nil the kept elements are written, in
// ascending order, into dst[0:k].data; the caller counts with dst == nil
// first and then allocates an exact slab, so no intermediate n+m slice is
// needed.
// Complexity is O(n+m).
func mergeInto[T any](cmp func(a, b T) int, a, b []T, union bool, dst []AvlTreeElement[T]) int {
	k := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch c := cmp(a[i], b[j]); {
		case c < 0:
			if dst != nil {
				dst[k].data = a[i]
			}
			k++
			i++
		case c > 0:
			if dst != nil {
				dst[k].data = b[j]
			}
			k++
			j++
		default:
			if union { // the later operand wins, as Insert would
				if dst != nil {
					dst[k].data = b[j]
				}
				k++
			}
			i++
			j++
		}
	}
	if dst != nil {
		for ; i < len(a); i++ {
			dst[k].data = a[i]
			k++
		}
		for ; j < len(b); j++ {
			dst[k].data = b[j]
			k++
		}
		return k
	}
	return k + len(a) - i + len(b) - j
}

// filterInPlace keeps the elements of the strictly ascending a that are
// present in the strictly ascending b (Intersect: keepPresent=true — a's
// element is kept on equal) or absent from it (Minus: keepPresent=false),
// compacting them to the front of a — the caller's private snapshot; the
// write index never passes the read index — and returns that prefix.
// Complexity is O(n+m).
func filterInPlace[T any](cmp func(a, b T) int, a, b []T, keepPresent bool) []T {
	out := a[:0]
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch c := cmp(a[i], b[j]); {
		case c < 0:
			if !keepPresent {
				out = append(out, a[i])
			}
			i++
		case c > 0:
			j++
		default:
			if keepPresent {
				out = append(out, a[i])
			}
			i++
			j++
		}
	}
	if !keepPresent {
		out = append(out, a[i:]...) // overlapping move: out ends at or before a[i]
	}
	return out
}

// linkBalanced links slab — whose data fields already hold the elements
// in ascending order — into a perfectly balanced tree and returns its
// root.  The middle element of every range becomes that range's root, so
// the two subtrees of every node differ in size by at most one and their
// heights by at most one (a valid AVL tree of the minimum height,
// bits.Len(n)); each node's cached height is set from its children.  The
// recursion depth is that height.
// Complexity is O(n).
func linkBalanced[T any](slab []AvlTreeElement[T]) *AvlTreeElement[T] {
	if len(slab) == 0 {
		return nil
	}
	mid := len(slab) / 2
	n := &slab[mid]
	n.left = linkBalanced(slab[:mid])
	n.right = linkBalanced(slab[mid+1:])
	lh, rh := 0, 0
	if n.left != nil {
		lh = n.left.height
	}
	if n.right != nil {
		rh = n.right.height
	}
	n.height = max(lh, rh) + 1
	return n
}

// buildFrom replaces the contents of tt with the strictly ascending
// elements of sorted, as one perfectly balanced slab-allocated tree.
// Complexity is O(n).
func (tt *AvlTree[T]) buildFrom(sorted []T) {
	tt.length = len(sorted)
	if len(sorted) == 0 {
		tt.root = nil
		return
	}
	slab := make([]AvlTreeElement[T], len(sorted))
	for i := range sorted {
		slab[i].data = sorted[i]
	}
	tt.root = linkBalanced(slab)
}

// setOpBuild sets tt to op applied to the snapshots a and b (tt already
// emptied and holding its comparison function).  Ascending snapshots take
// the merge-and-build path; anything else takes the original
// insert-per-element algorithm (setOpInsert), whose results the merge
// reproduces exactly.
//
// Minus and Intersect compact their result into the yy snapshot in place
// and build from it.  Union and SymmetricDifference, whose results can be
// longer than either snapshot, count first and then merge straight into
// an exact slab: one more comparison pass instead of an n+m scratch slice
// (measured: same time, a sixth less memory allocated per operation).
//
// SymmetricDifference skips the two input checks: the original algorithm
// was this very merge loop inserting each emitted element, for ANY input
// order, so the emitted sequence is the same either way.  If that
// sequence is strictly ascending under tt's ordering, inserting it would
// store exactly those elements, which is what the balanced build holds;
// if not, the same sequence is inserted one by one, as before.  Checking
// the k-element output instead of the n+m inputs keeps the
// identical-operands case (k = 0) as cheap as the original.
func (tt *AvlTree[T]) setOpBuild(a, b []T, op setOp) {
	if op != opSymDiff && (!strictlyAscending(tt.cmp, a) || !strictlyAscending(tt.cmp, b)) {
		tt.setOpInsert(a, b, op)
		return
	}
	switch op {
	case opMinus, opIntersect:
		tt.buildFrom(filterInPlace(tt.cmp, a, b, op == opIntersect))
	default:
		union := op == opUnion
		k := mergeInto(tt.cmp, a, b, union, nil)
		if k == 0 {
			tt.root, tt.length = nil, 0
			return
		}
		slab := make([]AvlTreeElement[T], k)
		mergeInto(tt.cmp, a, b, union, slab)
		if !union {
			for i := 1; i < k; i++ {
				if tt.cmp(slab[i-1].data, slab[i].data) >= 0 {
					for j := range slab { // not ascending: the original inserts
						tt.insert(slab[j].data)
					}
					return
				}
			}
		}
		tt.length = k
		tt.root = linkBalanced(slab)
	}
}

// setOpInsert is the original insert-per-element set algebra, used when a
// snapshot is not ascending under tt's comparison function.  Each case is
// the previous implementation of that operation verbatim.
// Complexity is O((n+m) log₂(n+m)).
func (tt *AvlTree[T]) setOpInsert(a, b []T, op setOp) {
	switch op {
	case opUnion:
		for _, d := range a {
			tt.insert(d)
		}
		for _, d := range b {
			tt.insert(d)
		}
	case opMinus, opIntersect:
		var zzTree AvlTree[T]
		zzTree.cmp = tt.cmp
		for _, d := range b {
			zzTree.insert(d)
		}
		for _, d := range a {
			if _, found := zzTree.Search(d); found == (op == opIntersect) {
				tt.insert(d)
			}
		}
	case opSymDiff:
		i, j := 0, 0
		for i < len(a) && j < len(b) {
			switch c := tt.cmp(a[i], b[j]); {
			case c < 0:
				tt.insert(a[i])
				i++
			case c > 0:
				tt.insert(b[j])
				j++
			default:
				i++
				j++
			}
		}
		for ; i < len(a); i++ {
			tt.insert(a[i])
		}
		for ; j < len(b); j++ {
			tt.insert(b[j])
		}
	}
}

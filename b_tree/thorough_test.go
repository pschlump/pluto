package b_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the B-tree structural invariant checker (called after
// every structural change), sequential fill/drain stress across orders,
// Dump, and a fixed-seed randomized property test cross-checked against a
// sorted-slice reference model.

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

// checkBTreeInvariants verifies the structural invariants of a B-tree:
//
//   - every non-root node holds between ceil(order/2)-1 and order-1 keys
//     (the root holds between 1 and order-1),
//   - keys are strictly ascending within every node,
//   - an internal node with k keys has exactly k+1 children, and every
//     subtree respects the key ranges of its separating keys,
//   - all leaves are at the same depth,
//   - the total number of keys equals tree.Length().
//
// It returns the number of keys in the tree.
func checkBTreeInvariants[T any](t *testing.T, tree *BTree[T]) int {
	t.Helper()
	if tree.root == nil {
		if tree.length != 0 {
			t.Fatalf("empty tree has length %d, expected 0", tree.length)
		}
		return 0
	}
	minKeys := tree.minKeys()
	leafDepth := -1

	var walk func(n *BTreeNode[T], depth int, lo, hi *T) int
	walk = func(n *BTreeNode[T], depth int, lo, hi *T) int {
		if len(n.keys) > tree.order-1 {
			t.Fatalf("node %v at depth %d has %d keys, maximum is %d", n.keys, depth, len(n.keys), tree.order-1)
		}
		if n == tree.root {
			if len(n.keys) == 0 {
				t.Fatalf("root has 0 keys")
			}
		} else if len(n.keys) < minKeys {
			t.Fatalf("node %v at depth %d has %d keys, minimum for a non-root is %d", n.keys, depth, len(n.keys), minKeys)
		}
		for i, k := range n.keys {
			if i > 0 && tree.cmp(n.keys[i-1], k) >= 0 {
				t.Fatalf("keys not strictly ascending in node %v", n.keys)
			}
			if lo != nil && tree.cmp(*lo, k) >= 0 {
				t.Fatalf("key %v not greater than the left separator %v", k, *lo)
			}
			if hi != nil && tree.cmp(k, *hi) >= 0 {
				t.Fatalf("key %v not less than the right separator %v", k, *hi)
			}
		}
		if n.children == nil {
			if leafDepth == -1 {
				leafDepth = depth
			} else if depth != leafDepth {
				t.Fatalf("leaf at depth %d, another leaf is at depth %d", depth, leafDepth)
			}
			return len(n.keys)
		}
		if len(n.children) != len(n.keys)+1 {
			t.Fatalf("internal node %v has %d keys and %d children", n.keys, len(n.keys), len(n.children))
		}
		total := len(n.keys)
		for i, c := range n.children {
			clo, chi := lo, hi
			if i > 0 {
				clo = &n.keys[i-1]
			}
			if i < len(n.keys) {
				chi = &n.keys[i]
			}
			total += walk(c, depth+1, clo, chi)
		}
		return total
	}

	count := walk(tree.root, 0, nil, nil)
	if count != tree.length {
		t.Fatalf("node key count %d does not match Length %d", count, tree.length)
	}
	return count
}

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// failingWriter always returns an error from Write, to exercise the error
// branch in Dump.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func TestTreeDump(t *testing.T) {
	tree := newTestTree(4)

	// Dump on an empty tree must not panic and must produce no output.
	var sb strings.Builder
	tree.Dump(&sb)
	if sb.Len() != 0 {
		t.Errorf("Dump on empty tree: expected no output, got %q", sb.String())
	}

	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	sb.Reset()
	tree.Dump(&sb)
	out := sb.String()
	for _, v := range []int{0, 2, 3, 5, 9} {
		if !strings.Contains(out, mkKey(v).S) {
			t.Errorf("Dump: expected output to contain %q, got %q", mkKey(v).S, out)
		}
	}

	// Dump must not panic when the writer fails.
	tree.Dump(failingWriter{})
}

// TestTreeSequentialFills fills and drains trees of several orders with
// ascending and descending keys, checking the B-tree invariants after
// every structural change.  Ascending/descending input is the worst case
// for node splits and merges.
func TestTreeSequentialFills(t *testing.T) {
	const n = 300
	for _, order := range []int{3, 4, 5, 6, 7} {
		// Ascending fill, then ascending drain.
		tree := newTestTree(order)
		for i := range n {
			tree.Insert(mkKey(i))
			checkBTreeInvariants(t, tree)
		}
		if got := tree.Length(); got != n {
			t.Fatalf("order %d: expected length %d, got %d", order, n, got)
		}
		for i := range n {
			if !tree.Delete(mkKey(i)) {
				t.Fatalf("order %d: expected to delete %d", order, i)
			}
			checkBTreeInvariants(t, tree)
		}
		if !tree.IsEmpty() {
			t.Fatalf("order %d: expected empty tree after ascending drain", order)
		}

		// Descending fill, then drain alternately from both ends.
		for i := n - 1; i >= 0; i-- {
			tree.Insert(mkKey(i))
			checkBTreeInvariants(t, tree)
		}
		for i := range n {
			if i%2 == 0 {
				if !tree.DeleteAtHead() {
					t.Fatalf("order %d: DeleteAtHead failed at step %d", order, i)
				}
			} else {
				if !tree.DeleteAtTail() {
					t.Fatalf("order %d: DeleteAtTail failed at step %d", order, i)
				}
			}
			checkBTreeInvariants(t, tree)
		}
		if !tree.IsEmpty() || tree.Length() != 0 {
			t.Fatalf("order %d: expected empty tree after draining from both ends", order)
		}
	}
}

// TestTreeRandomModel is a fixed-seed randomized property test.  It
// performs hundreds of mixed operations (Insert, Delete, Search, FindMin,
// FindMax, DeleteAtHead, DeleteAtTail, iteration) on trees of several
// orders and cross-checks every result against a plain sorted-slice
// reference model.
func TestTreeRandomModel(t *testing.T) {
	for _, order := range []int{3, 4, 6} {
		rng := rand.New(rand.NewPCG(42, uint64(order)))
		const keySpace = 100 // small key space to force duplicate inserts

		tree := NewBTree[int](order)
		var model []int // sorted, unique

		contains := func(v int) bool {
			i := sort.SearchInts(model, v)
			return i < len(model) && model[i] == v
		}
		modelInsert := func(v int) {
			if !contains(v) {
				i := sort.SearchInts(model, v)
				model = append(model, 0)
				copy(model[i+1:], model[i:])
				model[i] = v
			}
		}
		modelDelete := func(v int) bool {
			i := sort.SearchInts(model, v)
			if i < len(model) && model[i] == v {
				model = append(model[:i], model[i+1:]...)
				return true
			}
			return false
		}
		checkAll := func(step int) {
			t.Helper()
			if got := tree.Length(); got != len(model) {
				t.Fatalf("order %d step %d: length mismatch: tree=%d model=%d", order, step, got, len(model))
			}
			// Full forward iteration must match the sorted model exactly.
			i := 0
			for pos, v := range tree.All() {
				if pos != i {
					t.Fatalf("order %d step %d: All() index %d, expected %d", order, step, pos, i)
				}
				if i >= len(model) || v != model[i] {
					t.Fatalf("order %d step %d: All()[%d]=%d, model has %v", order, step, i, v, model)
				}
				i++
			}
			if i != len(model) {
				t.Fatalf("order %d step %d: All() visited %d items, model has %d", order, step, i, len(model))
			}
			// Backward iteration must be the reverse.
			i = len(model) - 1
			for _, v := range tree.Backward() {
				if i < 0 || v != model[i] {
					t.Fatalf("order %d step %d: Backward()[%d]=%d, model has %v", order, step, i, v, model)
				}
				i--
			}
			checkBTreeInvariants(t, tree)
		}

		for step := range 800 {
			v := rng.IntN(keySpace)
			switch rng.IntN(8) {
			case 0, 1, 2: // Insert (may be a duplicate -> replace)
				added := tree.Insert(v)
				if added == contains(v) {
					t.Fatalf("order %d step %d: Insert(%d)=%v, model said present=%v", order, step, v, added, contains(v))
				}
				modelInsert(v)
			case 3, 4: // Delete
				if got := tree.Delete(v); got != modelDelete(v) {
					t.Fatalf("order %d step %d: Delete(%d) returned %v, model said %v", order, step, v, got, contains(v))
				}
			case 5: // Search
				_, found := tree.Search(v)
				if want := contains(v); found != want {
					t.Fatalf("order %d step %d: Search(%d) found=%v, model says %v", order, step, v, found, want)
				}
			case 6: // FindMin / FindMax
				mn, mnOK := tree.FindMin()
				mx, mxOK := tree.FindMax()
				if len(model) == 0 {
					if mnOK || mxOK {
						t.Fatalf("order %d step %d: FindMin/FindMax on empty tree returned %v/%v", order, step, mn, mx)
					}
				} else if mn != model[0] || mx != model[len(model)-1] {
					t.Fatalf("order %d step %d: FindMin=%d FindMax=%d, model %v", order, step, mn, mx, model)
				}
			case 7: // DeleteAtHead / DeleteAtTail
				if rng.IntN(2) == 0 {
					got := tree.DeleteAtHead()
					if len(model) == 0 {
						if got {
							t.Fatalf("order %d step %d: DeleteAtHead on empty tree returned true", order, step)
						}
					} else if got {
						modelDelete(model[0])
					} else {
						t.Fatalf("order %d step %d: DeleteAtHead failed on non-empty tree", order, step)
					}
				} else {
					got := tree.DeleteAtTail()
					if len(model) == 0 {
						if got {
							t.Fatalf("order %d step %d: DeleteAtTail on empty tree returned true", order, step)
						}
					} else if got {
						modelDelete(model[len(model)-1])
					} else {
						t.Fatalf("order %d step %d: DeleteAtTail failed on non-empty tree", order, step)
					}
				}
			}
			checkBTreeInvariants(t, tree)
			if step%50 == 0 {
				checkAll(step)
			}
		}
		checkAll(800)
	}
}

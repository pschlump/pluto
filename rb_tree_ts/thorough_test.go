package rb_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"iter"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

// mustPanic runs fn and fails the test unless fn panics; the recovered
// value is returned for further checks.
func mustPanic(t *testing.T, what string, fn func()) (rv any) {
	t.Helper()
	defer func() {
		rv = recover()
		if rv == nil {
			t.Errorf("Expected %s on a nil tree to panic, it did not", what)
		}
	}()
	fn()
	return nil
}

// TestTreeNilTreePanics verifies the documented panics on a nil *RbTree
// receiver for Insert, Delete, DeleteAtHead, DeleteAtTail and Depth.
func TestTreeNilTreePanics(t *testing.T) {
	var Tree1 *RbTree[TestRbTreeNode]

	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"Insert", func() { Tree1.Insert(TestRbTreeNode{S: "01"}) }},
		{"Delete", func() { Tree1.Delete(TestRbTreeNode{S: "01"}) }},
		{"DeleteAtHead", func() { Tree1.DeleteAtHead() }},
		{"DeleteAtTail", func() { Tree1.DeleteAtTail() }},
		{"Depth", func() { Tree1.Depth() }},
	} {
		rv := mustPanic(t, tc.name, tc.fn)
		if msg, ok := rv.(string); !ok || !strings.Contains(msg, "nil") {
			t.Errorf("%s: expected panic message to mention nil, got %v", tc.name, rv)
		}
	}

	// The read-only accessors must NOT panic on a nil-receiver zero use; only
	// the ones documented to panic do.  A nil tree is not the zero value, so
	// only verify the documented set above.
}

// TestTreeZeroValueBehavior exercises every accessor on a freshly declared
// (zero-value) tree.
func TestTreeZeroValueBehavior(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if !Tree1.IsEmpty() {
		t.Errorf("Expected zero-value tree to be empty")
	}
	if Tree1.Length() != 0 {
		t.Errorf("Expected zero-value length of 0, got %d", Tree1.Length())
	}
	if d := Tree1.Depth(); d != 0 {
		t.Errorf("Expected zero-value depth of 0, got %d", d)
	}
	if Tree1.FindMin() != nil || Tree1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax on zero-value tree to return nil")
	}
	if Tree1.Search(TestRbTreeNode{S: "00"}) != nil {
		t.Errorf("Expected Search on zero-value tree to return nil")
	}
	if Tree1.Delete(TestRbTreeNode{S: "00"}) {
		t.Errorf("Expected Delete on zero-value tree to return false")
	}
	if Tree1.DeleteAtHead() || Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on zero-value tree to return false")
	}
	n := 0
	for range Tree1.All() {
		n++
	}
	for range Tree1.Backward() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected iterators on zero-value tree to yield nothing, got %d items", n)
	}
	checkInvariants(t, &Tree1)
}

// TestTreeSingleElement covers the one-element edge cases of every
// operation.
func TestTreeSingleElement(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]
	Tree1.Insert(TestRbTreeNode{S: "42"})

	if d := Tree1.Depth(); d != 1 {
		t.Errorf("Expected depth of 1 for single-element tree, got %d", d)
	}
	if mn := Tree1.FindMin(); mn == nil || mn.S != "42" {
		t.Errorf("Expected min of 42, got %+v", mn)
	}
	if mx := Tree1.FindMax(); mx == nil || mx.S != "42" {
		t.Errorf("Expected max of 42, got %+v", mx)
	}

	// A single-element tree yields exactly one item in both directions.
	checkOne := func(name string, seq iter.Seq[TestRbTreeNode]) {
		n := 0
		for v := range seq {
			if v.S != "42" {
				t.Errorf("%s: expected 42, got %s", name, v.S)
			}
			n++
		}
		if n != 1 {
			t.Errorf("%s: expected 1 item from single-element tree, got %d", name, n)
		}
	}
	checkOne("All", Tree1.All())
	checkOne("Backward", Tree1.Backward())
	checkInvariants(t, &Tree1)

	// Delete the single element; the tree must return to the empty state.
	if !Tree1.Delete(TestRbTreeNode{S: "42"}) {
		t.Errorf("Expected delete of the single element to return true")
	}
	if !Tree1.IsEmpty() || Tree1.Length() != 0 || Tree1.Depth() != 0 {
		t.Errorf("Expected empty state after deleting the only element")
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDeleteAtHeadTailSingleElement drains a one-element tree from both
// ends.
func TestTreeDeleteAtHeadTailSingleElement(t *testing.T) {
	var Head, Tail RbTree[TestRbTreeNode]
	Head.Insert(TestRbTreeNode{S: "42"})
	Tail.Insert(TestRbTreeNode{S: "42"})

	if !Head.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on single-element tree to return true")
	}
	if !Head.IsEmpty() {
		t.Errorf("Expected empty tree after DeleteAtHead of the only element")
	}
	checkInvariants(t, &Head)

	if !Tail.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on single-element tree to return true")
	}
	if !Tail.IsEmpty() {
		t.Errorf("Expected empty tree after DeleteAtTail of the only element")
	}
	checkInvariants(t, &Tail)
}

// TestTreeSearchReturnsCopy verifies that Search, FindMin and FindMax hand
// back copies: mutating the returned pointer must not change the tree.
func TestTreeSearchReturnsCopy(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]
	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if ptr := Tree1.Search(TestRbTreeNode{S: "05"}); ptr != nil {
		ptr.S = "ZZ"
	} else {
		t.Fatalf("Expected to find 05 in tree")
	}
	if ptr := Tree1.Search(TestRbTreeNode{S: "05"}); ptr == nil || ptr.S != "05" {
		t.Errorf("Expected stored item to be unchanged after mutating the Search result, got %+v", ptr)
	}

	if mn := Tree1.FindMin(); mn != nil {
		mn.S = "ZZ"
	}
	if mn := Tree1.FindMin(); mn == nil || mn.S != "02" {
		t.Errorf("Expected min to be unchanged after mutating the FindMin result, got %+v", mn)
	}

	if mx := Tree1.FindMax(); mx != nil {
		mx.S = "ZZ"
	}
	if mx := Tree1.FindMax(); mx == nil || mx.S != "09" {
		t.Errorf("Expected max to be unchanged after mutating the FindMax result, got %+v", mx)
	}

	// Iterators yield values; mutating them cannot affect the tree either.
	for v := range Tree1.All() {
		v.S = "ZZ"
	}
	if Tree1.Length() != 3 {
		t.Errorf("Expected length of 3, got %d", Tree1.Length())
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDuplicateInsertReplacesData verifies that re-inserting a
// Compare-equal item replaces the stored item rather than adding a second
// node, and that this holds at many positions in the tree.
func TestTreeDuplicateInsertReplacesData(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	keys := []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09"}
	for _, s := range keys {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	for _, s := range keys {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	if Tree1.Length() != len(keys) {
		t.Errorf("Expected duplicates to replace, length should be %d, got %d", len(keys), Tree1.Length())
	}

	// In-order traversal must still be strictly increasing (no duplicates).
	prev := ""
	for v := range Tree1.All() {
		if prev != "" && v.S <= prev {
			t.Errorf("Duplicate or out-of-order item %q after %q", v.S, prev)
		}
		prev = v.S
	}
	checkInvariants(t, &Tree1)
}

// TestTreeIteratorSnapshot verifies the documented snapshot semantics: the
// iterator walks the tree as it was when iteration started, so mutating the
// tree inside the loop body is safe and does not change what is yielded.
func TestTreeIteratorSnapshot(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]
	for _, s := range []string{"00", "01", "02", "03", "04"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	var seen []string
	for v := range Tree1.All() {
		seen = append(seen, v.S)
		// Mutate while iterating: delete an upcoming item and insert a new one.
		Tree1.Delete(TestRbTreeNode{S: "03"})
		Tree1.Insert(TestRbTreeNode{S: "99"})
	}
	want := []string{"00", "01", "02", "03", "04"}
	if fmt.Sprintf("%v", seen) != fmt.Sprintf("%v", want) {
		t.Errorf("Expected snapshot iteration of %v, got %v", want, seen)
	}

	// The mutations did happen; they just were not visible to the snapshot.
	if Tree1.Search(TestRbTreeNode{S: "03"}) != nil {
		t.Errorf("Expected 03 to be deleted after the loop")
	}
	if Tree1.Search(TestRbTreeNode{S: "99"}) == nil {
		t.Errorf("Expected 99 to be inserted after the loop")
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDepth checks Depth against hand-computed expectations: sorted
// insertion of 1..15 yields a perfectly balanced tree of depth 4, and depth
// must shrink back down as the tree is drained.
func TestTreeDepth(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]
	for i := 0; i < 15; i++ {
		Tree1.Insert(TestRbTreeNode{S: fmt.Sprintf("%02d", i)})
	}
	// 15 nodes perfectly balance to depth 4; a red-black tree over sorted
	// input must be at most 2*log2(16) = 8 deep, and this sequence lands on 4
	// or 5.
	if d := Tree1.Depth(); d < 4 || d > 8 {
		t.Errorf("Expected depth between 4 and 8 for 15 sorted inserts, got %d", d)
	}
	checkInvariants(t, &Tree1)

	for Tree1.DeleteAtTail() {
	}
	if d := Tree1.Depth(); d != 0 {
		t.Errorf("Expected depth of 0 after draining the tree, got %d", d)
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDump verifies that Dump produces the documented output for an
// empty and a populated tree.
func TestTreeDump(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	var buf bytes.Buffer
	Tree1.Dump(&buf)
	if got := buf.String(); !strings.Contains(got, "RbTree (empty)") {
		t.Errorf("Expected dump of empty tree to say \"RbTree (empty)\", got %q", got)
	}

	for _, s := range []string{"05", "02", "09", "00", "03", "07", "12"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	buf.Reset()
	Tree1.Dump(&buf)
	got := buf.String()
	if !strings.Contains(got, "RbTree length=7") {
		t.Errorf("Expected dump header with length=7, got %q", got)
	}
	if !strings.Contains(got, "depth=") {
		t.Errorf("Expected dump header with depth=, got %q", got)
	}
	// The root is always black and every inserted key must appear with a
	// color marker.
	if !strings.Contains(got, "(B)") {
		t.Errorf("Expected at least one black node in dump, got %q", got)
	}
	for _, s := range []string{"00", "02", "03", "05", "07", "09", "12"} {
		if !strings.Contains(got, "{"+s+"}(") {
			t.Errorf("Expected dump to contain node %s, got %q", s, got)
		}
	}
	// With 7 nodes there must be 7 node lines plus the header.
	if n := strings.Count(got, "\n"); n != 8 {
		t.Errorf("Expected 8 lines in dump (header + 7 nodes), got %d", n)
	}
}

// TestTreeMixedOpsReference is a randomized property test with a fixed seed.
// It drives the tree through hundreds of mixed operations and cross-checks
// it after each batch against a simple sorted-slice reference model.
func TestTreeMixedOpsReference(t *testing.T) {
	const Ops = 3000
	const KeySpace = 200

	rng := rand.New(rand.NewPCG(2024, 8))
	var Tree1 RbTree[TestRbTreeNode]
	var ref []string // sorted, unique

	refIndex := func(key string) (int, bool) {
		i := sort.SearchStrings(ref, key)
		return i, i < len(ref) && ref[i] == key
	}

	for op := 0; op < Ops; op++ {
		key := fmt.Sprintf("%04d", rng.IntN(KeySpace))
		switch rng.IntN(9) {
		case 0, 1, 2: // Insert (may be a duplicate).
			Tree1.Insert(TestRbTreeNode{S: key})
			if i, found := refIndex(key); !found {
				ref = append(ref, "")
				copy(ref[i+1:], ref[i:])
				ref[i] = key
			}
		case 3, 4, 5: // Delete (may be absent).
			got := Tree1.Delete(TestRbTreeNode{S: key})
			i, found := refIndex(key)
			if got != found {
				t.Fatalf("op %d: Delete(%s) returned %v, model says %v", op, key, got, found)
			}
			if found {
				ref = append(ref[:i], ref[i+1:]...)
			}
		case 6: // DeleteAtHead.
			got := Tree1.DeleteAtHead()
			if (len(ref) == 0) == got {
				t.Fatalf("op %d: DeleteAtHead returned %v, model has %d items", op, got, len(ref))
			}
			if len(ref) > 0 {
				ref = ref[1:]
			}
		case 7: // DeleteAtTail.
			got := Tree1.DeleteAtTail()
			if (len(ref) == 0) == got {
				t.Fatalf("op %d: DeleteAtTail returned %v, model has %d items", op, got, len(ref))
			}
			if len(ref) > 0 {
				ref = ref[:len(ref)-1]
			}
		case 8: // Search an existing and a random key.
			if ptr := Tree1.Search(TestRbTreeNode{S: key}); (ptr != nil) != func() bool { _, f := refIndex(key); return f }() {
				t.Fatalf("op %d: Search(%s) disagrees with model", op, key)
			}
		}

		// Cross-check the whole state every 100 ops.
		if op%100 == 99 || op == Ops-1 {
			if Tree1.Length() != len(ref) {
				t.Fatalf("op %d: length %d, model has %d", op, Tree1.Length(), len(ref))
			}
			if Tree1.IsEmpty() != (len(ref) == 0) {
				t.Fatalf("op %d: IsEmpty %v, model has %d items", op, Tree1.IsEmpty(), len(ref))
			}
			if len(ref) > 0 {
				if mn := Tree1.FindMin(); mn == nil || mn.S != ref[0] {
					t.Fatalf("op %d: FindMin %+v, model min %s", op, mn, ref[0])
				}
				if mx := Tree1.FindMax(); mx == nil || mx.S != ref[len(ref)-1] {
					t.Fatalf("op %d: FindMax %+v, model max %s", op, mx, ref[len(ref)-1])
				}
			} else {
				if Tree1.FindMin() != nil || Tree1.FindMax() != nil {
					t.Fatalf("op %d: FindMin/FindMax non-nil on empty tree", op)
				}
			}

			// Full forward and backward iteration must match the model.
			var fwd []string
			for v := range Tree1.All() {
				fwd = append(fwd, v.S)
			}
			if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", ref) {
				t.Fatalf("op %d: All() = %v, model = %v", op, fwd, ref)
			}
			var bwd []string
			for v := range Tree1.Backward() {
				bwd = append(bwd, v.S)
			}
			for i := range ref {
				if bwd[len(ref)-1-i] != ref[i] {
					t.Fatalf("op %d: Backward() does not mirror the model at %d", op, i)
				}
			}

			// Every key in the model must be found; a handful of absent
			// keys must not be.
			for _, k := range ref {
				if Tree1.Search(TestRbTreeNode{S: k}) == nil {
					t.Fatalf("op %d: Search(%s) returned nil for a model key", op, k)
				}
			}
			for i := 0; i < 10; i++ {
				absent := fmt.Sprintf("%04d", KeySpace+rng.IntN(KeySpace))
				if Tree1.Search(TestRbTreeNode{S: absent}) != nil {
					t.Fatalf("op %d: Search(%s) found an absent key", op, absent)
				}
			}
			checkInvariants(t, &Tree1)
		}
	}

	// Depth bound: a red-black tree with n nodes has depth <= 2*log2(n+1).
	if n := Tree1.Length(); n > 0 {
		depth := Tree1.Depth()
		maxDepth := 2
		for lim := n + 1; lim > 1; lim >>= 1 {
			maxDepth += 2
		}
		if depth > maxDepth {
			t.Errorf("Depth %d exceeds red-black bound %d for %d nodes", depth, maxDepth, n)
		}
	}

	// Truncate must reset the tree to the zero state.
	Tree1.Truncate()
	if !Tree1.IsEmpty() || Tree1.Length() != 0 || Tree1.Depth() != 0 {
		t.Errorf("Expected empty state after Truncate")
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDeleteAllInsertOrders deletes in ascending and descending order —
// the skewed cases that stress the delete fixup rotations in one direction.
func TestTreeDeleteAllInsertOrders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		insert []string
		del    []string
	}{
		{
			name:   "ascending insert, ascending delete",
			insert: []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11"},
			del:    []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11"},
		},
		{
			name:   "ascending insert, descending delete",
			insert: []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11"},
			del:    []string{"11", "10", "09", "08", "07", "06", "05", "04", "03", "02", "01", "00"},
		},
		{
			name:   "descending insert, ascending delete",
			insert: []string{"11", "10", "09", "08", "07", "06", "05", "04", "03", "02", "01", "00"},
			del:    []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11"},
		},
	} {
		var Tree1 RbTree[TestRbTreeNode]
		for _, s := range tc.insert {
			Tree1.Insert(TestRbTreeNode{S: s})
		}
		for _, s := range tc.del {
			if !Tree1.Delete(TestRbTreeNode{S: s}) {
				t.Errorf("%s: Delete(%s) returned false", tc.name, s)
			}
			checkInvariants(t, &Tree1)
		}
		if !Tree1.IsEmpty() {
			t.Errorf("%s: expected empty tree after deleting everything", tc.name)
		}
	}
}

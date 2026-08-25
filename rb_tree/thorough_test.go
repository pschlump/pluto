package rb_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

// expectPanic runs f and fails the test unless f panics with the expected
// message.
func expectPanic(t *testing.T, name, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected panic %q, got none", name, want)
			return
		}
		if s, ok := r.(string); !ok || s != want {
			t.Errorf("%s: expected panic %q, got %v", name, want, r)
		}
	}()
	f()
}

// TestTreeNilPanics verifies that the operations documented to panic on a
// nil tree actually do so.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *RbTree[TestRbTreeNode]
	const want = "tree should not be nil"

	expectPanic(t, "Insert", want, func() { nilTree.Insert(TestRbTreeNode{S: "00"}) })
	expectPanic(t, "Delete", want, func() { nilTree.Delete(TestRbTreeNode{S: "00"}) })
	expectPanic(t, "DeleteAtHead", want, func() { nilTree.DeleteAtHead() })
	expectPanic(t, "DeleteAtTail", want, func() { nilTree.DeleteAtTail() })
	expectPanic(t, "Depth", want, func() { nilTree.Depth() })
}

// TestTreeDepth verifies Depth on an empty tree, a single-node tree and a
// small balanced tree.
func TestTreeDepth(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]

	if d := Tree1.Depth(); d != 0 {
		t.Errorf("Expected depth 0 on empty tree, got %d", d)
	}

	Tree1.Insert(TestRbTreeNode{S: "02"})
	if d := Tree1.Depth(); d != 1 {
		t.Errorf("Expected depth 1 on single-node tree, got %d", d)
	}

	for _, s := range []string{"01", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	if d := Tree1.Depth(); d != 2 {
		t.Errorf("Expected depth 2 on 3-node balanced tree, got %d", d)
	}
	checkInvariants(t, &Tree1)
}

// TestTreeSingleElement exercises every operation on a tree holding exactly
// one item.
func TestTreeSingleElement(t *testing.T) {
	var Tree1 RbTree[TestRbTreeNode]
	Tree1.Insert(TestRbTreeNode{S: "42"})

	if mn := Tree1.FindMin(); mn == nil || mn.S != "42" {
		t.Errorf("Expected min of 42, got %+v", mn)
	}
	if mx := Tree1.FindMax(); mx == nil || mx.S != "42" {
		t.Errorf("Expected max of 42, got %+v", mx)
	}

	var fwd, bwd []string
	for v := range Tree1.All() {
		fwd = append(fwd, v.S)
	}
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	if len(fwd) != 1 || fwd[0] != "42" {
		t.Errorf("All: expected [42], got %v", fwd)
	}
	if len(bwd) != 1 || bwd[0] != "42" {
		t.Errorf("Backward: expected [42], got %v", bwd)
	}

	// Removing the only element from the head and from the tail.
	var Head RbTree[TestRbTreeNode]
	Head.Insert(TestRbTreeNode{S: "42"})
	if !Head.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on single-node tree to return true")
	}
	if !Head.IsEmpty() || Head.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead, length=%d", Head.Length())
	}
	checkInvariants(t, &Head)

	var Tail RbTree[TestRbTreeNode]
	Tail.Insert(TestRbTreeNode{S: "42"})
	if !Tail.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on single-node tree to return true")
	}
	if !Tail.IsEmpty() || Tail.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtTail, length=%d", Tail.Length())
	}
	checkInvariants(t, &Tail)

	if !Tree1.Delete(TestRbTreeNode{S: "42"}) {
		t.Errorf("Expected Delete of the only item to return true")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting the only item")
	}
	if Tree1.FindMin() != nil || Tree1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax to return nil after deleting the only item")
	}
	checkInvariants(t, &Tree1)
}

// TestTreeDeleteTwoChildren covers the delete paths for a node with two
// children: both when the successor is the node's right child and when the
// successor is deeper in the right subtree.
func TestTreeDeleteTwoChildren(t *testing.T) {
	// Successor is the direct right child of the deleted node.
	var T1 RbTree[TestRbTreeNode]
	for _, s := range []string{"02", "01", "03"} {
		T1.Insert(TestRbTreeNode{S: s})
	}
	if !T1.Delete(TestRbTreeNode{S: "02"}) {
		t.Errorf("Expected delete of root to return true")
	}
	var got []string
	for v := range T1.All() {
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != "[01 03]" {
		t.Errorf("Expected [01 03] after delete, got %v", got)
	}
	checkInvariants(t, &T1)

	// Successor is deeper in the right subtree (its parent is not the
	// deleted node).
	var T2 RbTree[TestRbTreeNode]
	for _, s := range []string{"10", "05", "20", "15", "25", "12"} {
		T2.Insert(TestRbTreeNode{S: s})
	}
	if !T2.Delete(TestRbTreeNode{S: "10"}) {
		t.Errorf("Expected delete of root to return true")
	}
	got = got[:0]
	for v := range T2.All() {
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != "[05 12 15 20 25]" {
		t.Errorf("Expected [05 12 15 20 25] after delete, got %v", got)
	}
	checkInvariants(t, &T2)

	// Deleting a leaf and a node with a single child.
	if !T2.Delete(TestRbTreeNode{S: "25"}) { // leaf
		t.Errorf("Expected delete of leaf to return true")
	}
	checkInvariants(t, &T2)
	if !T2.Delete(TestRbTreeNode{S: "20"}) { // single child (15)
		t.Errorf("Expected delete of single-child node to return true")
	}
	checkInvariants(t, &T2)
	got = got[:0]
	for v := range T2.All() {
		got = append(got, v.S)
	}
	if fmt.Sprintf("%v", got) != "[05 12 15]" {
		t.Errorf("Expected [05 12 15] after deletes, got %v", got)
	}
}

// TestTreeDump verifies that Dump produces the expected empty-tree message
// and includes every node, its color and the header on a populated tree.
func TestTreeDump(t *testing.T) {
	var Empty RbTree[TestRbTreeNode]
	var buf bytes.Buffer
	Empty.Dump(&buf)
	if buf.String() != "RbTree (empty)\n" {
		t.Errorf("Expected empty dump message, got %q", buf.String())
	}

	var Tree1 RbTree[TestRbTreeNode]
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	buf.Reset()
	Tree1.Dump(&buf)
	out := buf.String()

	if !strings.HasPrefix(out, fmt.Sprintf("RbTree length=%d depth=%d\n", Tree1.Length(), Tree1.Depth())) {
		t.Errorf("Dump header mismatch, output:\n%s", out)
	}
	for _, s := range []string{"00", "02", "03", "05", "09"} {
		if !strings.Contains(out, "{"+s+"}(B)") && !strings.Contains(out, "{"+s+"}(R)") {
			t.Errorf("Dump output missing node %s, output:\n%s", s, out)
		}
	}
	// One line per node plus the header.
	if n := strings.Count(out, "\n"); n != Tree1.Length()+1 {
		t.Errorf("Expected %d lines in dump, got %d", Tree1.Length()+1, n)
	}
}

// TestTreeModelBased runs hundreds of mixed operations against a simple
// reference model (a set plus a sorted slice) with a fixed seed, checking
// results and invariants along the way.
func TestTreeModelBased(t *testing.T) {
	const (
		keySpace = 300  // keys are "000000".."000299"
		nOps     = 5000 // mixed operations
	)

	rng := rand.New(rand.NewPCG(1234, 5678))
	key := func(n int) string { return fmt.Sprintf("%06d", n) }

	var Tree1 RbTree[TestRbTreeNode]
	model := make(map[string]bool)

	modelMinMax := func() (mn, mx string, ok bool) {
		var keys []string
		for k := range model {
			keys = append(keys, k)
		}
		if len(keys) == 0 {
			return "", "", false
		}
		sort.Strings(keys)
		return keys[0], keys[len(keys)-1], true
	}

	for i := 0; i < nOps; i++ {
		k := key(rng.IntN(keySpace))
		switch rng.IntN(10) {
		case 0, 1, 2, 3, 4, 5: // Insert
			Tree1.Insert(TestRbTreeNode{S: k})
			model[k] = true
		case 6, 7, 8: // Delete
			want := model[k]
			if got := Tree1.Delete(TestRbTreeNode{S: k}); got != want {
				t.Fatalf("op %d: Delete(%s) = %v, model says %v", i, k, got, want)
			}
			delete(model, k)
		default: // Search
			got := Tree1.Search(TestRbTreeNode{S: k})
			if model[k] && (got == nil || got.S != k) {
				t.Fatalf("op %d: Search(%s) = %+v, model says present", i, k, got)
			}
			if !model[k] && got != nil {
				t.Fatalf("op %d: Search(%s) = %+v, model says absent", i, k, got)
			}
		}

		if Tree1.Length() != len(model) {
			t.Fatalf("op %d: Length() = %d, model has %d", i, Tree1.Length(), len(model))
		}

		mn, mx, ok := modelMinMax()
		if got := Tree1.FindMin(); (got == nil) == ok || (ok && got.S != mn) {
			t.Fatalf("op %d: FindMin() = %+v, model says %s", i, got, mn)
		}
		if got := Tree1.FindMax(); (got == nil) == ok || (ok && got.S != mx) {
			t.Fatalf("op %d: FindMax() = %+v, model says %s", i, got, mx)
		}
		if Tree1.IsEmpty() == ok {
			t.Fatalf("op %d: IsEmpty() = %v, model has %d items", i, Tree1.IsEmpty(), len(model))
		}

		if i%250 == 0 {
			checkInvariants(t, &Tree1)
		}
	}

	// Full iteration must match the sorted model exactly, both directions.
	var want []string
	for k := range model {
		want = append(want, k)
	}
	sort.Strings(want)

	var fwd []string
	for v := range Tree1.All() {
		fwd = append(fwd, v.S)
	}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Fatalf("All: expected %v, got %v", want, fwd)
	}
	var bwd []string
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	for j := range want {
		if bwd[j] != want[len(want)-1-j] {
			t.Fatalf("Backward: position %d: expected %s, got %s", j, want[len(want)-1-j], bwd[j])
		}
	}

	// Drain via DeleteAtHead / DeleteAtTail, checking order against the model.
	var fromHead []string
	var Head RbTree[TestRbTreeNode]
	for _, k := range want {
		Head.Insert(TestRbTreeNode{S: k})
	}
	rest := append([]string(nil), want...)
	for len(rest) > 0 {
		mn := rest[0]
		if got := Head.FindMin(); got == nil || got.S != mn {
			t.Fatalf("DeleteAtHead drain: expected min %s, got %+v", mn, got)
		}
		if !Head.DeleteAtHead() {
			t.Fatalf("DeleteAtHead drain: returned false with %d items left", len(rest))
		}
		fromHead = append(fromHead, mn)
		rest = rest[1:]
	}
	if Head.DeleteAtHead() || !Head.IsEmpty() {
		t.Fatalf("DeleteAtHead drain: expected empty tree and false at the end")
	}
	if fmt.Sprintf("%v", fromHead) != fmt.Sprintf("%v", want) {
		t.Fatalf("DeleteAtHead drain order mismatch: expected %v, got %v", want, fromHead)
	}

	var fromTail []string
	var Tail RbTree[TestRbTreeNode]
	for _, k := range want {
		Tail.Insert(TestRbTreeNode{S: k})
	}
	rest = append([]string(nil), want...)
	for len(rest) > 0 {
		mx := rest[len(rest)-1]
		if got := Tail.FindMax(); got == nil || got.S != mx {
			t.Fatalf("DeleteAtTail drain: expected max %s, got %+v", mx, got)
		}
		if !Tail.DeleteAtTail() {
			t.Fatalf("DeleteAtTail drain: returned false with %d items left", len(rest))
		}
		fromTail = append(fromTail, mx)
		rest = rest[:len(rest)-1]
	}
	if Tail.DeleteAtTail() || !Tail.IsEmpty() {
		t.Fatalf("DeleteAtTail drain: expected empty tree and false at the end")
	}
	checkInvariants(t, &Head)
	checkInvariants(t, &Tail)
}

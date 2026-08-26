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

// expectPanic runs f and fails the test unless it panics.
func expectPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	f()
}

// TestTreeNilPanics verifies the documented panic when Insert is called on
// a nil tree — the one operation with no sane answer.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *RbTree[TestRbTreeNode]
	key := TestRbTreeNode{S: "00"}

	expectPanic(t, "Insert", func() { nilTree.Insert(key) })

	// Verify the panic message names the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert to panic on nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilTree.Insert(key)
	}()
}

// TestTreeNilTolerated verifies that every operation other than Insert
// treats a nil tree as an empty tree instead of panicking.
func TestTreeNilTolerated(t *testing.T) {
	var nilTree *RbTree[TestRbTreeNode]
	key := TestRbTreeNode{S: "00"}

	if !nilTree.IsEmpty() {
		t.Errorf("Expected nil tree to be empty.")
	}
	if nilTree.Len() != 0 || nilTree.Length() != 0 {
		t.Errorf("Expected nil tree to have length 0.")
	}
	if nilTree.Depth() != 0 {
		t.Errorf("Expected depth 0 on nil tree.")
	}
	if _, found := nilTree.Search(key); found {
		t.Errorf("Expected not-found from Search on nil tree.")
	}
	if nilTree.Delete(key) {
		t.Errorf("Expected false from Delete on nil tree.")
	}
	if _, found := nilTree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on nil tree.")
	}
	if _, found := nilTree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on nil tree.")
	}
	if nilTree.DeleteAtHead() || nilTree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on nil tree.")
	}
	nilTree.Truncate() // no-op, must not panic

	for range nilTree.All() {
		t.Errorf("Expected no values from All on nil tree.")
	}
	for range nilTree.Backward() {
		t.Errorf("Expected no values from Backward on nil tree.")
	}

	var buf bytes.Buffer
	nilTree.Dump(&buf)
	if buf.String() != "RbTree (empty)\n" {
		t.Errorf("Expected empty dump message on nil tree, got %q", buf.String())
	}
}

// TestTreeDepth verifies Depth on an empty tree, a single-node tree and a
// small balanced tree.
func TestTreeDepth(t *testing.T) {
	Tree1 := newTestTree()

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
	checkInvariants(t, Tree1)
}

// TestTreeSingleElement exercises every operation on a tree holding
// exactly one item.
func TestTreeSingleElement(t *testing.T) {
	Tree1 := newTestTree()
	Tree1.Insert(TestRbTreeNode{S: "42"})

	if mn, found := Tree1.FindMin(); !found || mn.S != "42" {
		t.Errorf("Expected min of 42, got %+v", mn)
	}
	if mx, found := Tree1.FindMax(); !found || mx.S != "42" {
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
	Head := newTestTree()
	Head.Insert(TestRbTreeNode{S: "42"})
	if !Head.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on single-node tree to return true")
	}
	if !Head.IsEmpty() || Head.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead, length=%d", Head.Length())
	}
	checkInvariants(t, Head)

	Tail := newTestTree()
	Tail.Insert(TestRbTreeNode{S: "42"})
	if !Tail.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on single-node tree to return true")
	}
	if !Tail.IsEmpty() || Tail.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtTail, length=%d", Tail.Length())
	}
	checkInvariants(t, Tail)

	if !Tree1.Delete(TestRbTreeNode{S: "42"}) {
		t.Errorf("Expected Delete of the only item to return true")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting the only item")
	}
	if _, found := Tree1.FindMin(); found {
		t.Errorf("Expected FindMin to report not-found after deleting the only item")
	}
	if _, found := Tree1.FindMax(); found {
		t.Errorf("Expected FindMax to report not-found after deleting the only item")
	}
	checkInvariants(t, Tree1)
}

// TestTreeDeleteTwoChildren covers the delete paths for a node with two
// children: both when the successor is the node's right child and when the
// successor is deeper in the right subtree.
func TestTreeDeleteTwoChildren(t *testing.T) {
	// Successor is the direct right child of the deleted node.
	T1 := newTestTree()
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
	checkInvariants(t, T1)

	// Successor is deeper in the right subtree (its parent is not the
	// deleted node).
	T2 := newTestTree()
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
	checkInvariants(t, T2)

	// Deleting a leaf and a node with a single child.
	if !T2.Delete(TestRbTreeNode{S: "25"}) { // leaf
		t.Errorf("Expected delete of leaf to return true")
	}
	checkInvariants(t, T2)
	if !T2.Delete(TestRbTreeNode{S: "20"}) { // single child (15)
		t.Errorf("Expected delete of single-child node to return true")
	}
	checkInvariants(t, T2)
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
	Empty := newTestTree()
	var buf bytes.Buffer
	Empty.Dump(&buf)
	if buf.String() != "RbTree (empty)\n" {
		t.Errorf("Expected empty dump message, got %q", buf.String())
	}

	Tree1 := newTestTree()
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
		if !strings.Contains(out, "{"+s+" ") {
			t.Errorf("Dump output missing node %s, output:\n%s", s, out)
		}
	}
	// Every node is printed red or black, one line each, plus the header.
	if n := strings.Count(out, "(R)\n") + strings.Count(out, "(B)\n"); n != Tree1.Length() {
		t.Errorf("Expected %d colored node lines in dump, got %d", Tree1.Length(), n)
	}
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

	Tree1 := newTestTree()
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

	for i := range nOps {
		k := key(rng.IntN(keySpace))
		switch rng.IntN(10) {
		case 0, 1, 2, 3, 4, 5: // Insert
			added := Tree1.Insert(TestRbTreeNode{S: k})
			if added == model[k] {
				t.Fatalf("op %d: Insert(%s) = %v, model says present=%v", i, k, added, model[k])
			}
			model[k] = true
		case 6, 7, 8: // Delete
			want := model[k]
			if got := Tree1.Delete(TestRbTreeNode{S: k}); got != want {
				t.Fatalf("op %d: Delete(%s) = %v, model says %v", i, k, got, want)
			}
			delete(model, k)
		default: // Search
			got, found := Tree1.Search(TestRbTreeNode{S: k})
			if model[k] && (!found || got.S != k) {
				t.Fatalf("op %d: Search(%s) = %+v, model says present", i, k, got)
			}
			if !model[k] && found {
				t.Fatalf("op %d: Search(%s) = %+v, model says absent", i, k, got)
			}
		}

		if Tree1.Length() != len(model) {
			t.Fatalf("op %d: Length() = %d, model has %d", i, Tree1.Length(), len(model))
		}

		mn, mx, ok := modelMinMax()
		if got, found := Tree1.FindMin(); found != ok || (ok && got.S != mn) {
			t.Fatalf("op %d: FindMin() = %+v, model says %s", i, got, mn)
		}
		if got, found := Tree1.FindMax(); found != ok || (ok && got.S != mx) {
			t.Fatalf("op %d: FindMax() = %+v, model says %s", i, got, mx)
		}
		if Tree1.IsEmpty() == ok {
			t.Fatalf("op %d: IsEmpty() = %v, model has %d items", i, Tree1.IsEmpty(), len(model))
		}

		if i%250 == 0 {
			checkInvariants(t, Tree1)
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
	Head := newTestTree()
	for _, k := range want {
		Head.Insert(TestRbTreeNode{S: k})
	}
	rest := append([]string(nil), want...)
	for len(rest) > 0 {
		mn := rest[0]
		if got, found := Head.FindMin(); !found || got.S != mn {
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
	Tail := newTestTree()
	for _, k := range want {
		Tail.Insert(TestRbTreeNode{S: k})
	}
	rest = append([]string(nil), want...)
	for len(rest) > 0 {
		mx := rest[len(rest)-1]
		if got, found := Tail.FindMax(); !found || got.S != mx {
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
	// The tail drain must have removed the items largest-first: want reversed.
	for j := range want {
		if fromTail[j] != want[len(want)-1-j] {
			t.Fatalf("DeleteAtTail drain order mismatch: position %d: expected %s, got %s", j, want[len(want)-1-j], fromTail[j])
		}
	}
	checkInvariants(t, Head)
	checkInvariants(t, Tail)
}

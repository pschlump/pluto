package skip_list_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// TestSkipListNode is an Interface Matching data type for the nodes that
// supports the Comparable interface.  This means that it has a Compare
// function.
type TestSkipListNode struct {
	S string
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestSkipListNode)(nil)

// Compare implements the Compare function to satisfy the interface
// requirements.
func (aa TestSkipListNode) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestSkipListNode); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestSkipListNode); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

func TestListInsertSearch(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after decleration, failed to get one.")
	}

	List1.Insert(TestSkipListNode{S: "12"})

	if List1.IsEmpty() {
		t.Errorf("Expected non-empty list after insert, failed to get one.")
	}
	if List1.Length() != 1 {
		t.Errorf("Expected length of 1, got %d", List1.Length())
	}

	ptr := List1.Search(TestSkipListNode{S: "12"})
	if ptr == nil {
		t.Errorf("Expected to find node in list, returned nil instead")
	}

	ptr = List1.Search(TestSkipListNode{S: "11"})
	if ptr != nil {
		t.Errorf("Expected *NOT* to find node in list, returned value [%+v] instead", *ptr)
	}

	List1.Insert(TestSkipListNode{S: "11"})
	List1.Insert(TestSkipListNode{S: "13"})
	List1.Insert(TestSkipListNode{S: "10"})
	for _, s := range []string{"10", "11", "13"} {
		if List1.Search(TestSkipListNode{S: s}) == nil {
			t.Errorf("Expected to find node %s in list, returned nil instead", s)
		}
	}
	if ptr := List1.Search(TestSkipListNode{S: "14"}); ptr != nil {
		t.Errorf("Expected *NOT* to find node in list, returned value [%+v] instead", *ptr)
	}
	if List1.Length() != 4 {
		t.Errorf("Expected length of 4, got %d", List1.Length())
	}
}

func TestListInsertDuplicateReplaces(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	List1.Insert(TestSkipListNode{S: "12"})
	List1.Insert(TestSkipListNode{S: "12"})

	if List1.Length() != 1 {
		t.Errorf("Expected duplicate insert to replace, length should be 1, got %d", List1.Length())
	}
	if ptr := List1.Search(TestSkipListNode{S: "12"}); ptr == nil {
		t.Errorf("Expected to find node in list, returned nil instead")
	}
}

func TestListDelete(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	if List1.Delete(TestSkipListNode{S: "12"}) {
		t.Errorf("Expected delete on empty list to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03", "07"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if List1.Delete(TestSkipListNode{S: "12"}) {
		t.Errorf("Expected delete of absent item to return false")
	}
	if !List1.Delete(TestSkipListNode{S: "00"}) {
		t.Errorf("Expected delete of present item to return true")
	}
	if List1.Search(TestSkipListNode{S: "00"}) != nil {
		t.Errorf("Expected node to be gone after delete")
	}
	if List1.Length() != 5 {
		t.Errorf("Expected length of 5, got %d", List1.Length())
	}
	if List1.Delete(TestSkipListNode{S: "00"}) {
		t.Errorf("Expected second delete of same item to return false")
	}
}

func TestListTruncate(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	for _, s := range []string{"05", "02", "09"} {
		List1.Insert(TestSkipListNode{S: s})
	}
	List1.Truncate()

	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after truncate")
	}
	if List1.Length() != 0 {
		t.Errorf("Expected length of 0 after truncate, got %d", List1.Length())
	}
	if List1.Search(TestSkipListNode{S: "05"}) != nil {
		t.Errorf("Expected search after truncate to return nil")
	}

	// List should still be usable after a truncate.
	List1.Insert(TestSkipListNode{S: "07"})
	if List1.Length() != 1 {
		t.Errorf("Expected length of 1 after insert into truncated list, got %d", List1.Length())
	}
}

func TestListFindMinMax(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	if List1.FindMin() != nil || List1.FindMax() != nil {
		t.Errorf("Expected FindMin/FindMax on empty list to return nil")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if mn := List1.FindMin(); mn == nil || mn.S != "00" {
		t.Errorf("Expected min of 00, got %+v", mn)
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "09" {
		t.Errorf("Expected max of 09, got %+v", mx)
	}
}

func TestListDeleteAtHeadTail(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	if List1.DeleteAtHead() || List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on empty list to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if !List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true")
	}
	if mn := List1.FindMin(); mn == nil || mn.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", mn)
	}
	if !List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true")
	}
	if mx := List1.FindMax(); mx == nil || mx.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", mx)
	}
	if List1.Length() != 3 {
		t.Errorf("Expected length of 3, got %d", List1.Length())
	}

	// Drain the list from both ends.
	if !List1.DeleteAtHead() || !List1.DeleteAtTail() || !List1.DeleteAtHead() {
		t.Errorf("Expected draining deletes to return true")
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after draining")
	}
	if List1.DeleteAtHead() || List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on drained list to return false")
	}
}

func TestListIterators(t *testing.T) {
	var List1 SkipList[TestSkipListNode]

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	var fwd []string
	for v := range List1.All() {
		fwd = append(fwd, v.S)
	}
	want := []string{"00", "02", "03", "05", "09"}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Errorf("All: expected %v, got %v", want, fwd)
	}

	var bwd []string
	for v := range List1.Backward() {
		bwd = append(bwd, v.S)
	}
	want = []string{"09", "05", "03", "02", "00"}
	if fmt.Sprintf("%v", bwd) != fmt.Sprintf("%v", want) {
		t.Errorf("Backward: expected %v, got %v", want, bwd)
	}

	// Early exit via break.
	n := 0
	for range List1.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Iterating an empty list yields nothing.
	var Empty SkipList[TestSkipListNode]
	for range Empty.All() {
		t.Errorf("Expected no items from All on empty list")
	}
	for range Empty.Backward() {
		t.Errorf("Expected no items from Backward on empty list")
	}
}

// TestListRandomized inserts a large number of items in random order and
// checks the list against a sorted reference slice, then deletes them all in
// random order.
func TestListRandomized(t *testing.T) {
	const N = 10000

	rng := rand.New(rand.NewPCG(42, 42))
	perm := rng.Perm(N)

	var List1 SkipList[TestSkipListNode]
	for _, n := range perm {
		List1.Insert(TestSkipListNode{S: fmt.Sprintf("%06d", n)})
	}
	if List1.Length() != N {
		t.Fatalf("Expected length of %d, got %d", N, List1.Length())
	}

	// The list must iterate in sorted order.
	ref := make([]string, 0, N)
	for v := range List1.All() {
		ref = append(ref, v.S)
	}
	if !sort.StringsAreSorted(ref) {
		t.Fatalf("All: items are not in ascending order")
	}
	if len(ref) != N {
		t.Fatalf("All: expected %d items, got %d", N, len(ref))
	}
	if ref[0] != "000000" || ref[N-1] != fmt.Sprintf("%06d", N-1) {
		t.Errorf("All: unexpected endpoints %s ... %s", ref[0], ref[N-1])
	}

	// Spot-check searches, then delete everything in a different random order.
	for i := 0; i < N; i += 997 {
		if List1.Search(TestSkipListNode{S: fmt.Sprintf("%06d", i)}) == nil {
			t.Errorf("Expected to find %06d in list", i)
		}
	}

	del := rand.New(rand.NewPCG(7, 7)).Perm(N)
	for i, n := range del {
		key := TestSkipListNode{S: fmt.Sprintf("%06d", n)}
		if !List1.Delete(key) {
			t.Fatalf("Delete %d of %s failed", i, key.S)
		}
		if List1.Search(key) != nil {
			t.Fatalf("Found %s after delete", key.S)
		}
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after deleting everything, length=%d", List1.Length())
	}
}

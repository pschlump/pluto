package skip_list_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestSkipListNode is the test element type.  Note what is missing
// compared to the pluto version of this test file: no Compare method, no
// interface assertion, no type assertions inside a comparison.  Ordering
// is supplied to the list as a plain function (cmpTestSkipListNode below).
type TestSkipListNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

// cmpTestSkipListNode orders TestSkipListNode by its S field.
func cmpTestSkipListNode(a, b TestSkipListNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestList builds a SkipList of TestSkipListNode ordered by S.
func newTestList() *SkipList[TestSkipListNode] {
	return NewSkipListFunc(cmpTestSkipListNode)
}

func TestListInsertSearch(t *testing.T) {
	List1 := newTestList()

	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after declaration, failed to get one.")
	}

	if !List1.Insert(TestSkipListNode{S: "12"}) {
		t.Errorf("Expected Insert of a new item to return true.")
	}

	if List1.IsEmpty() {
		t.Errorf("Expected non-empty list after insert, failed to get one.")
	}
	if List1.Length() != 1 {
		t.Errorf("Expected length of 1, got %d", List1.Length())
	}

	if item, found := List1.Search(TestSkipListNode{S: "12"}); !found || item.S != "12" {
		t.Errorf("Expected to find 12 in list, got %v found=%v", item, found)
	}

	if _, found := List1.Search(TestSkipListNode{S: "11"}); found {
		t.Errorf("Expected *NOT* to find node in list, but did.")
	}

	List1.Insert(TestSkipListNode{S: "11"})
	List1.Insert(TestSkipListNode{S: "13"})
	List1.Insert(TestSkipListNode{S: "10"})
	for _, s := range []string{"10", "11", "13"} {
		if _, found := List1.Search(TestSkipListNode{S: s}); !found {
			t.Errorf("Expected to find node %s in list, did not.", s)
		}
	}
	if _, found := List1.Search(TestSkipListNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in list, but did.")
	}
	if List1.Length() != 4 {
		t.Errorf("Expected length of 4, got %d", List1.Length())
	}
}

func TestListInsertDuplicateReplaces(t *testing.T) {
	List1 := newTestList()

	List1.Insert(TestSkipListNode{S: "12", N: 1})
	if List1.Insert(TestSkipListNode{S: "12", N: 7}) {
		t.Errorf("Expected duplicate insert to return false.")
	}

	if List1.Length() != 1 {
		t.Errorf("Expected duplicate insert to replace, length should be 1, got %d", List1.Length())
	}
	if item, found := List1.Search(TestSkipListNode{S: "12"}); !found || item.N != 7 {
		t.Errorf("Expected duplicate insert to replace the stored value, got %+v found=%v", item, found)
	}
}

func TestListDelete(t *testing.T) {
	List1 := newTestList()

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
	if _, found := List1.Search(TestSkipListNode{S: "00"}); found {
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
	List1 := newTestList()

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
	if _, found := List1.Search(TestSkipListNode{S: "05"}); found {
		t.Errorf("Expected search after truncate to return not-found")
	}

	// List should still be usable after a truncate.
	List1.Insert(TestSkipListNode{S: "07"})
	if List1.Length() != 1 {
		t.Errorf("Expected length of 1 after insert into truncated list, got %d", List1.Length())
	}
}

func TestListFindMinMax(t *testing.T) {
	List1 := newTestList()

	if _, found := List1.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on empty list")
	}
	if _, found := List1.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on empty list")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if mn, found := List1.FindMin(); !found || mn.S != "00" {
		t.Errorf("Expected min of 00, got %+v found=%v", mn, found)
	}
	if mx, found := List1.FindMax(); !found || mx.S != "09" {
		t.Errorf("Expected max of 09, got %+v found=%v", mx, found)
	}
}

func TestListDeleteAtHeadTail(t *testing.T) {
	List1 := newTestList()

	if List1.DeleteAtHead() || List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on empty list to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	if !List1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true")
	}
	if mn, found := List1.FindMin(); !found || mn.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", mn)
	}
	if !List1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true")
	}
	if mx, found := List1.FindMax(); !found || mx.S != "05" {
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
	List1 := newTestList()

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		List1.Insert(TestSkipListNode{S: s})
	}

	var fwd []string
	for v := range List1.All() {
		fwd = append(fwd, v.S)
	}
	want := []string{"00", "02", "03", "05", "09"}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Errorf("All: expected %v got %v", want, fwd)
	}

	var bwd []string
	for v := range List1.Backward() {
		bwd = append(bwd, v.S)
	}
	want = []string{"09", "05", "03", "02", "00"}
	if fmt.Sprintf("%v", bwd) != fmt.Sprintf("%v", want) {
		t.Errorf("Backward: expected %v got %v", want, bwd)
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
	Empty := newTestList()
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

	List1 := newTestList()
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
		if _, found := List1.Search(TestSkipListNode{S: fmt.Sprintf("%06d", i)}); !found {
			t.Errorf("Expected to find %06d in list", i)
		}
	}

	del := rand.New(rand.NewPCG(7, 7)).Perm(N)
	for i, n := range del {
		key := TestSkipListNode{S: fmt.Sprintf("%06d", n)}
		if !List1.Delete(key) {
			t.Fatalf("Delete %d of %s failed", i, key.S)
		}
		if _, found := List1.Search(key); found {
			t.Fatalf("Found %s after delete", key.S)
		}
	}
	if !List1.IsEmpty() {
		t.Errorf("Expected empty list after deleting everything, length=%d", List1.Length())
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewSkipList.
func TestCompare(t *testing.T) {
	if c := Compare(1, 2); c != -1 {
		t.Errorf("Compare(1,2) = %d, expected -1", c)
	}
	if c := Compare(2, 1); c != 1 {
		t.Errorf("Compare(2,1) = %d, expected +1", c)
	}
	if c := Compare(1, 1); c != 0 {
		t.Errorf("Compare(1,1) = %d, expected 0", c)
	}
	if c := Compare("abc", "abd"); c != -1 {
		t.Errorf("Compare(abc,abd) = %d, expected -1", c)
	}
	if c := Compare(2.5, 2.25); c != 1 {
		t.Errorf("Compare(2.5,2.25) = %d, expected +1", c)
	}
}

// TestNewSkipListOrdered verifies the constructor for naturally ordered
// key types.
func TestNewSkipListOrdered(t *testing.T) {
	ints := NewSkipList[int]()
	for i := range 100 {
		if !ints.Insert(i) {
			t.Errorf("Expected Insert of %d to return true.", i)
		}
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 99 {
		t.Errorf("Expected max 99, got %d found=%v", x, found)
	}

	strs := NewSkipList[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		strs.Insert(s)
	}
	if x, found := strs.FindMin(); !found || x != "apple" {
		t.Errorf("Expected min apple, got %q found=%v", x, found)
	}
	if x, found := strs.FindMax(); !found || x != "pear" {
		t.Errorf("Expected max pear, got %q found=%v", x, found)
	}
}

// TestNewSkipListFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order of
// the struct.
func TestNewSkipListFunc(t *testing.T) {
	list := NewSkipListFunc(func(a, b TestSkipListNode) int {
		return a.N - b.N
	})
	for _, n := range []TestSkipListNode{{S: "five", N: 5}, {S: "two", N: 2}, {S: "nine", N: 9}} {
		list.Insert(n)
	}
	if x, found := list.FindMin(); !found || x.S != "two" {
		t.Errorf("Expected min two, got %+v found=%v", x, found)
	}
	var got []string
	for v := range list.All() {
		got = append(got, v.S)
	}
	if expect := []string{"two", "five", "nine"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("NewSkipListFunc order error, expected %v got %v", expect, got)
	}
}

// TestNewSkipListFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewSkipListFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewSkipListFunc(nil) to panic.")
		}
	}()
	NewSkipListFunc[TestSkipListNode](nil)
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkListSize = 4096

func BenchmarkInsert(b *testing.B) {
	list := NewSkipList[string]()
	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Insert(keys[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	list := NewSkipList[string]()
	keys := make([]string, benchmarkListSize)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
		list.Insert(keys[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Search(keys[i%benchmarkListSize])
	}
}

func BenchmarkDelete(b *testing.B) {
	list := NewSkipList[string]()
	keys := make([]string, benchmarkListSize)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
	}
	fill := func() {
		for _, k := range keys {
			list.Insert(k)
		}
	}
	fill()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if list.IsEmpty() {
			fill()
		}
		list.Delete(keys[i%benchmarkListSize])
	}
}

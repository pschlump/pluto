package rb_tree

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

// TestRbTreeNode is the test element type.  Note what is missing compared
// to the pluto version of this test file: no Compare method, no interface
// assertion, no type assertions inside a comparison.  Ordering is supplied
// to the tree as a plain function (cmpTestRbTreeNode below).
type TestRbTreeNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

// cmpTestRbTreeNode orders TestRbTreeNode by its S field.
func cmpTestRbTreeNode(a, b TestRbTreeNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestTree builds an RbTree of TestRbTreeNode ordered by S.
func newTestTree() *RbTree[TestRbTreeNode] {
	return NewRbTreeFunc(cmpTestRbTreeNode)
}

// checkInvariants verifies the red-black properties of the tree:
//
//  1. The root is black.
//  2. No red node has a red child.
//  3. Every root-to-nil-leaf path has the same number of black nodes.
//  4. The tree is a valid BST (in-order traversal is sorted) of the
//     expected size.
//  5. Every child's parent pointer points back at its parent.
func checkInvariants(t *testing.T, tt *RbTree[TestRbTreeNode]) {
	t.Helper()

	if tt.root != nil && tt.root.red {
		t.Errorf("root is red")
	}
	if tt.root != nil && tt.root.parent != nil {
		t.Errorf("root has a parent")
	}
	if tt.root == nil && tt.length != 0 {
		t.Errorf("empty root with length %d", tt.length)
	}

	// BST order, size and parent-pointer integrity.
	n := 0
	prev := ""
	var checkParents func(cur, parent *RbTreeNode[TestRbTreeNode])
	checkParents = func(cur, parent *RbTreeNode[TestRbTreeNode]) {
		if cur == nil {
			return
		}
		if cur.parent != parent {
			t.Errorf("node %v has wrong parent pointer", cur.data)
		}
		checkParents(cur.left, cur)
		checkParents(cur.right, cur)
	}
	checkParents(tt.root, nil)
	for v := range tt.All() {
		if n > 0 && v.S <= prev {
			t.Errorf("in-order traversal not sorted: %q after %q", v.S, prev)
		}
		prev = v.S
		n++
	}
	if n != tt.length {
		t.Errorf("in-order traversal visited %d nodes, length is %d", n, tt.length)
	}

	// Black height and red-red property.
	var blackHeight func(cur *RbTreeNode[TestRbTreeNode]) int
	blackHeight = func(cur *RbTreeNode[TestRbTreeNode]) int {
		if cur == nil {
			return 1 // nil leaves are black.
		}
		if cur.red && (isRed(cur.left) || isRed(cur.right)) {
			t.Errorf("red node %v has a red child", cur.data)
		}
		lb := blackHeight(cur.left)
		rb := blackHeight(cur.right)
		if lb != rb {
			t.Errorf("black height mismatch at node %v: left=%d right=%d", cur.data, lb, rb)
		}
		if cur.red {
			return lb
		}
		return lb + 1
	}
	blackHeight(tt.root)
}

func TestTreeInsertSearch(t *testing.T) {
	Tree1 := newTestTree()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after declaration, failed to get one.")
	}

	if !Tree1.Insert(TestRbTreeNode{S: "12"}) {
		t.Errorf("Expected Insert of a new item to return true.")
	}

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if Tree1.Length() != 1 {
		t.Errorf("Expected length of 1, got %d", Tree1.Length())
	}

	if item, found := Tree1.Search(TestRbTreeNode{S: "12"}); !found || item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %v found=%v", item, found)
	}

	if _, found := Tree1.Search(TestRbTreeNode{S: "11"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}

	Tree1.Insert(TestRbTreeNode{S: "11"})
	Tree1.Insert(TestRbTreeNode{S: "13"})
	Tree1.Insert(TestRbTreeNode{S: "10"})
	for _, s := range []string{"10", "11", "13"} {
		if _, found := Tree1.Search(TestRbTreeNode{S: s}); !found {
			t.Errorf("Expected to find node %s in tree, did not.", s)
		}
	}
	if _, found := Tree1.Search(TestRbTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}
	if Tree1.Length() != 4 {
		t.Errorf("Expected length of 4, got %d", Tree1.Length())
	}
	checkInvariants(t, Tree1)
}

func TestTreeInsertDuplicateReplaces(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestRbTreeNode{S: "12", N: 1})
	if Tree1.Insert(TestRbTreeNode{S: "12", N: 7}) {
		t.Errorf("Expected duplicate insert to return false.")
	}

	if Tree1.Length() != 1 {
		t.Errorf("Expected duplicate insert to replace, length should be 1, got %d", Tree1.Length())
	}
	if item, found := Tree1.Search(TestRbTreeNode{S: "12"}); !found || item.N != 7 {
		t.Errorf("Expected duplicate insert to replace the stored value, got %+v found=%v", item, found)
	}
	checkInvariants(t, Tree1)
}

func TestTreeDelete(t *testing.T) {
	Tree1 := newTestTree()

	if Tree1.Delete(TestRbTreeNode{S: "12"}) {
		t.Errorf("Expected delete on empty tree to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03", "07"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if Tree1.Delete(TestRbTreeNode{S: "12"}) {
		t.Errorf("Expected delete of absent item to return false")
	}
	if !Tree1.Delete(TestRbTreeNode{S: "00"}) {
		t.Errorf("Expected delete of present item to return true")
	}
	if _, found := Tree1.Search(TestRbTreeNode{S: "00"}); found {
		t.Errorf("Expected node to be gone after delete")
	}
	if Tree1.Length() != 5 {
		t.Errorf("Expected length of 5, got %d", Tree1.Length())
	}
	if Tree1.Delete(TestRbTreeNode{S: "00"}) {
		t.Errorf("Expected second delete of same item to return false")
	}
	checkInvariants(t, Tree1)
}

func TestTreeTruncate(t *testing.T) {
	Tree1 := newTestTree()

	for _, s := range []string{"05", "02", "09"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}
	Tree1.Truncate()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after truncate")
	}
	if Tree1.Length() != 0 {
		t.Errorf("Expected length of 0 after truncate, got %d", Tree1.Length())
	}
	if _, found := Tree1.Search(TestRbTreeNode{S: "05"}); found {
		t.Errorf("Expected search after truncate to return not-found")
	}

	// Tree should still be usable after a truncate.
	Tree1.Insert(TestRbTreeNode{S: "07"})
	if Tree1.Length() != 1 {
		t.Errorf("Expected length of 1 after insert into truncated tree, got %d", Tree1.Length())
	}
	checkInvariants(t, Tree1)
}

func TestTreeFindMinMax(t *testing.T) {
	Tree1 := newTestTree()

	if _, found := Tree1.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on empty tree")
	}
	if _, found := Tree1.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on empty tree")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if mn, found := Tree1.FindMin(); !found || mn.S != "00" {
		t.Errorf("Expected min of 00, got %+v found=%v", mn, found)
	}
	if mx, found := Tree1.FindMax(); !found || mx.S != "09" {
		t.Errorf("Expected max of 09, got %+v found=%v", mx, found)
	}
}

func TestTreeDeleteAtHeadTail(t *testing.T) {
	Tree1 := newTestTree()

	if Tree1.DeleteAtHead() || Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on empty tree to return false")
	}

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true")
	}
	if mn, found := Tree1.FindMin(); !found || mn.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", mn)
	}
	if !Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true")
	}
	if mx, found := Tree1.FindMax(); !found || mx.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", mx)
	}
	if Tree1.Length() != 3 {
		t.Errorf("Expected length of 3, got %d", Tree1.Length())
	}

	// Drain the tree from both ends.
	if !Tree1.DeleteAtHead() || !Tree1.DeleteAtTail() || !Tree1.DeleteAtHead() {
		t.Errorf("Expected draining deletes to return true")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after draining")
	}
	if Tree1.DeleteAtHead() || Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtHead/DeleteAtTail on drained tree to return false")
	}
	checkInvariants(t, Tree1)
}

func TestTreeIterators(t *testing.T) {
	Tree1 := newTestTree()

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestRbTreeNode{S: s})
	}

	var fwd []string
	for v := range Tree1.All() {
		fwd = append(fwd, v.S)
	}
	want := []string{"00", "02", "03", "05", "09"}
	if fmt.Sprintf("%v", fwd) != fmt.Sprintf("%v", want) {
		t.Errorf("All: expected %v got %v", want, fwd)
	}

	var bwd []string
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	want = []string{"09", "05", "03", "02", "00"}
	if fmt.Sprintf("%v", bwd) != fmt.Sprintf("%v", want) {
		t.Errorf("Backward: expected %v got %v", want, bwd)
	}

	// Early exit via break, both directions.
	n := 0
	for range Tree1.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break in All to yield exactly 1 item, got %d", n)
	}
	n = 0
	for range Tree1.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break in Backward to yield exactly 1 item, got %d", n)
	}

	// Iterating an empty tree yields nothing.
	Empty := newTestTree()
	for range Empty.All() {
		t.Errorf("Expected no items from All on empty tree")
	}
	for range Empty.Backward() {
		t.Errorf("Expected no items from Backward on empty tree")
	}
}

// TestTreeSortedInsert verifies that inserting in sorted order — the worst
// case for an unbalanced tree — stays balanced and correct.
func TestTreeSortedInsert(t *testing.T) {
	const N = 1000

	Tree1 := newTestTree()
	for i := range N {
		Tree1.Insert(TestRbTreeNode{S: fmt.Sprintf("%06d", i)})
	}
	if Tree1.Length() != N {
		t.Fatalf("Expected length of %d, got %d", N, Tree1.Length())
	}
	// A red-black tree with N nodes has depth at most 2*log2(N+1).
	if d := Tree1.Depth(); d > 25 {
		t.Errorf("Expected depth <= 25 after sorted insert of %d nodes, got %d", N, d)
	}
	checkInvariants(t, Tree1)
}

// TestTreeRandomized inserts a large number of items in random order,
// checking the red-black invariants along the way, then deletes them all in
// a different random order.
func TestTreeRandomized(t *testing.T) {
	const N = 10000

	rng := rand.New(rand.NewPCG(42, 42))
	perm := rng.Perm(N)

	Tree1 := newTestTree()
	for i, n := range perm {
		Tree1.Insert(TestRbTreeNode{S: fmt.Sprintf("%06d", n)})
		if i%1000 == 0 {
			checkInvariants(t, Tree1)
		}
	}
	if Tree1.Length() != N {
		t.Fatalf("Expected length of %d, got %d", N, Tree1.Length())
	}
	checkInvariants(t, Tree1)

	// The tree must iterate in sorted order in both directions.
	ref := make([]string, 0, N)
	for v := range Tree1.All() {
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
	var bwd []string
	for v := range Tree1.Backward() {
		bwd = append(bwd, v.S)
	}
	if len(bwd) != N {
		t.Fatalf("Backward: expected %d items, got %d", N, len(bwd))
	}
	for i := range ref {
		if bwd[N-1-i] != ref[i] {
			t.Fatalf("Backward does not mirror All at position %d", i)
		}
	}

	// Spot-check searches, then delete everything in a different random order.
	for i := 0; i < N; i += 997 {
		if _, found := Tree1.Search(TestRbTreeNode{S: fmt.Sprintf("%06d", i)}); !found {
			t.Errorf("Expected to find %06d in tree", i)
		}
	}

	del := rand.New(rand.NewPCG(7, 7)).Perm(N)
	for i, n := range del {
		key := TestRbTreeNode{S: fmt.Sprintf("%06d", n)}
		if !Tree1.Delete(key) {
			t.Fatalf("Delete %d of %s failed", i, key.S)
		}
		if _, found := Tree1.Search(key); found {
			t.Fatalf("Found %s after delete", key.S)
		}
		if i%1000 == 0 {
			checkInvariants(t, Tree1)
		}
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting everything, length=%d", Tree1.Length())
	}
	checkInvariants(t, Tree1)
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewRbTree.
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
	if c := Compare("b", "a"); c != 1 {
		t.Errorf("Compare(b,a) = %d, expected +1", c)
	}
	if c := Compare(2.5, 2.25); c != 1 {
		t.Errorf("Compare(2.5,2.25) = %d, expected +1", c)
	}
}

// TestNewRbTreeOrdered verifies the constructor for naturally ordered key
// types and the red-black balance guarantee: sorted input still produces a
// shallow tree.
func TestNewRbTreeOrdered(t *testing.T) {
	ints := NewRbTree[int]()
	for i := range 100 {
		if !ints.Insert(i) {
			t.Errorf("Expected Insert of %d to return true.", i)
		}
	}
	// 100 nodes inserted in sorted order: an unbalanced tree would be a
	// chain of depth 100; the red-black tree stays logarithmic.
	if d := ints.Depth(); d > 14 {
		t.Errorf("Expected balanced depth <= 14 for 100 sorted inserts, got %d", d)
	}
	if d := ints.Depth(); d < 7 { // at least log2(100)
		t.Errorf("Depth %d implausibly small for 100 nodes", d)
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 99 {
		t.Errorf("Expected max 99, got %d found=%v", x, found)
	}

	strs := NewRbTree[string]()
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

// TestNewRbTreeFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order of
// the struct.
func TestNewRbTreeFunc(t *testing.T) {
	tree := NewRbTreeFunc(func(a, b TestRbTreeNode) int {
		return a.N - b.N
	})
	for _, n := range []TestRbTreeNode{{S: "five", N: 5}, {S: "two", N: 2}, {S: "nine", N: 9}} {
		tree.Insert(n)
	}
	if x, found := tree.FindMin(); !found || x.S != "two" {
		t.Errorf("Expected min two, got %+v found=%v", x, found)
	}
	var got []string
	for v := range tree.All() {
		got = append(got, v.S)
	}
	if expect := []string{"two", "five", "nine"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("NewRbTreeFunc order error, expected %v got %v", expect, got)
	}
}

// TestNewRbTreeFuncNil verifies that a nil comparison function is rejected
// at construction time, not on first use.
func TestNewRbTreeFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewRbTreeFunc(nil) to panic.")
		}
	}()
	NewRbTreeFunc[TestRbTreeNode](nil)
}

// TestZeroValueTree verifies that the zero value of RbTree behaves as an
// empty tree for every non-ordering operation and that Insert fails loudly
// because no comparison function has been set.
func TestZeroValueTree(t *testing.T) {
	var tree RbTree[TestRbTreeNode]

	if !tree.IsEmpty() {
		t.Errorf("Expected zero value tree to be empty.")
	}
	if tree.Len() != 0 || tree.Length() != 0 {
		t.Errorf("Expected zero value tree to have length 0.")
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected zero value tree to have depth 0.")
	}
	tree.Truncate() // must not panic
	if _, found := tree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on zero value tree.")
	}
	if _, found := tree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on zero value tree.")
	}
	if _, found := tree.Search(TestRbTreeNode{S: "05"}); found {
		t.Errorf("Expected not-found from Search on zero value tree.")
	}
	if tree.Delete(TestRbTreeNode{S: "05"}) {
		t.Errorf("Expected false from Delete on zero value tree.")
	}
	if tree.DeleteAtHead() || tree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on zero value tree.")
	}
	for range tree.All() {
		t.Errorf("Expected no values from All on zero value tree.")
	}
	for range tree.Backward() {
		t.Errorf("Expected no values from Backward on zero value tree.")
	}

	// Insert without a comparison function panics with a clear message.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero value tree to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewRbTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		tree.Insert(TestRbTreeNode{S: "05"})
	}()
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkTreeSize = 4096

func BenchmarkInsert(b *testing.B) {
	tree := NewRbTree[string]()
	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(keys[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	tree := NewRbTree[string]()
	keys := make([]string, benchmarkTreeSize)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
		tree.Insert(keys[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(keys[i%benchmarkTreeSize])
	}
}

func BenchmarkDelete(b *testing.B) {
	tree := NewRbTree[string]()
	keys := make([]string, benchmarkTreeSize)
	for i := range keys {
		keys[i] = fmt.Sprintf("%08d", i)
	}
	fill := func() {
		for _, k := range keys {
			tree.Insert(k)
		}
	}
	fill()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if tree.IsEmpty() {
			fill()
		}
		tree.Delete(keys[i%benchmarkTreeSize])
	}
}

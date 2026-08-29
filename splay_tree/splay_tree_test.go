package splay_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestTreeNode is the test element type.  It implements no interface: no
// Compare method, no interface assertion, no type assertions inside a
// comparison.  Ordering is supplied to the tree as a plain function
// (cmpTestTreeNode below).
type TestTreeNode struct {
	S string
	// N is satellite data that the comparison ignores.  It is used to
	// verify that duplicate inserts replace the stored value.
	N int
}

// cmpTestTreeNode orders TestTreeNode by its S field.
func cmpTestTreeNode(a, b TestTreeNode) int {
	return strings.Compare(a.S, b.S)
}

// newTestTree builds a tree of TestTreeNode ordered by S.
func newTestTree() *SplayTree[TestTreeNode] {
	return NewSplayTreeFunc(cmpTestTreeNode)
}

func TestTreeInsertSearch(t *testing.T) {
	Tree1 := newTestTree()

	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after declaration, failed to get one.")
	}

	v1 := Tree1.Insert(TestTreeNode{S: "12"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree after insert, failed to get one.")
	}
	if v1 == false {
		t.Errorf("Expected to insert new node, got back false for new.")
	}

	v1 = Tree1.Insert(TestTreeNode{S: "12"})

	if v1 == true {
		t.Errorf("Expected to insert duplicate node, got back true for new.")
	}
	if Tree1.Len() != 1 {
		t.Errorf("Expected 1 node in tree, got %d", Tree1.Len())
	}

	if item, found := Tree1.Search(TestTreeNode{S: "12"}); !found {
		t.Errorf("Expected to find node in tree, search returned not-found")
	} else if item.S != "12" {
		t.Errorf("Expected to find 12 in tree, got %s", item.S)
	}

	if _, found := Tree1.Search(TestTreeNode{S: "11"}); found {
		t.Errorf("Expected *NOT* to find node in tree, search found it")
	}

	Tree1.Insert(TestTreeNode{S: "11"})
	Tree1.Insert(TestTreeNode{S: "13"})
	Tree1.Insert(TestTreeNode{S: "10"})
	if _, found := Tree1.Search(TestTreeNode{S: "10"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "13"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "11"}); !found {
		t.Errorf("Expected to find node in tree, did not.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "14"}); found {
		t.Errorf("Expected *NOT* to find node in tree, but did.")
	}
	if Tree1.Length() != 4 {
		t.Errorf("Expected 4 nodes in tree, got %d", Tree1.Length())
	}
}

// TestTreeInsertDuplicateReplaces verifies that inserting a duplicate key
// replaces the stored value (the new satellite data is what Search
// returns) while preserving the tree length.
func TestTreeInsertDuplicateReplaces(t *testing.T) {
	tree := newTestTree()

	tree.Insert(TestTreeNode{S: "05", N: 1})
	tree.Insert(TestTreeNode{S: "03", N: 1})
	tree.Insert(TestTreeNode{S: "08", N: 1})

	// Insert duplicates at the current root, a smaller key and a larger key.
	for _, s := range []string{"08", "03", "05"} {
		if tree.Insert(TestTreeNode{S: s, N: 7}) {
			t.Errorf("Expected duplicate insert of %s to return false.", s)
		}
	}
	if tree.Length() != 3 {
		t.Errorf("Expected length 3 after duplicate inserts, got %d", tree.Length())
	}
	got, found := tree.Search(TestTreeNode{S: "05"})
	if !found {
		t.Fatalf("Expected to find 05.")
	}
	if got.N != 7 {
		t.Errorf("Expected duplicate insert to replace the stored value, got N=%d want 7.", got.N)
	}
}

// TestTreeSearchSplaysToRoot verifies the defining splay behavior: a
// successful Search moves the found node to the root, and a miss moves the
// last visited node to the root.
func TestTreeSearchSplaysToRoot(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	// Insert splays the inserted node to the root.
	if Tree1.root == nil || Tree1.root.data.S != "03" {
		t.Fatalf("Expected last inserted node 03 at the root, got %+v", Tree1.root.data)
	}

	// A successful Search splays the found node to the root.
	if _, found := Tree1.Search(TestTreeNode{S: "00"}); !found {
		t.Fatalf("Expected to find 00.")
	}
	if Tree1.root.data.S != "00" {
		t.Errorf("Expected found node 00 splayed to the root, got %s", Tree1.root.data.S)
	}

	// A miss splays the last visited node to the root: with 00 at the root,
	// searching for 01 visits 00's right subtree down to its deepest node.
	Tree1.Search(TestTreeNode{S: "01"})
	lastRoot := Tree1.root.data.S
	if lastRoot == "01" {
		t.Errorf("01 is not in the tree, it cannot be the root.")
	}
	// The miss root must be a node adjacent to the probe in sorted order.
	if lastRoot != "00" && lastRoot != "02" && lastRoot != "03" && lastRoot != "05" {
		t.Errorf("Unexpected root %s after a miss on 01.", lastRoot)
	}
	if _, found := Tree1.Search(TestTreeNode{S: "01"}); found {
		t.Errorf("Expected *NOT* to find 01.")
	}
}

// TestTreeRepeatedSearchIsCheap verifies the working-set property in
// observable form: after a Search, the found node is at the root, so a
// second Search for the same key is a single comparison at the root.
func TestTreeRepeatedSearchIsCheap(t *testing.T) {
	Tree1 := newTestTree()
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestTreeNode{S: s})
	}

	for range 3 {
		if _, found := Tree1.Search(TestTreeNode{S: "02"}); !found {
			t.Fatalf("Expected to find 02.")
		}
		if Tree1.root.data.S != "02" {
			t.Errorf("Expected 02 to stay at the root across repeated searches, got %s", Tree1.root.data.S)
		}
	}
}

func TestTreeDelete(t *testing.T) {
	Tree1 := newTestTree()

	// Delete from an empty tree.
	if Tree1.Delete(TestTreeNode{S: "05"}) {
		t.Errorf("Found node in empty tree.")
	}

	// Delete the only node.
	Tree1.Insert(TestTreeNode{S: "05"})
	if !Tree1.Delete(TestTreeNode{S: "05"}) {
		t.Errorf("Expected to find a node to delete, did not.")
	}
	if size := Tree1.Length(); size != 0 {
		t.Errorf("Expected empty tree, got %d", size)
	}

	// Deleting a missing key returns false and keeps every element.
	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	if Tree1.Delete(TestTreeNode{S: "07"}) {
		t.Errorf("Expected Delete of missing key to return false.")
	}
	if size := Tree1.Length(); size != 5 {
		t.Errorf("Expected 5 nodes after failed delete, got %d", size)
	}

	// Delete every node, one at a time.
	for i, s := range []string{"03", "00", "09", "02", "05"} {
		if !Tree1.Delete(TestTreeNode{S: s}) {
			t.Fatalf("Expected Delete(%s) to return true.", s)
		}
		if size := Tree1.Length(); size != 4-i {
			t.Errorf("Expected %d nodes after deleting %s, got %d", 4-i, s, size)
		}
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after deleting all nodes.")
	}
}

// TestTreeDeleteSplays verifies the delete join: the victim ends up at the
// root and is removed, and the surviving keys are all still searchable.
func TestTreeDeleteSplays(t *testing.T) {
	Tree1 := newTestTree()
	for _, s := range []string{"20", "10", "30", "05", "15", "25", "40", "28"} {
		Tree1.Insert(TestTreeNode{S: s})
	}

	if !Tree1.Delete(TestTreeNode{S: "20"}) {
		t.Fatalf("Expected Delete(20) to return true.")
	}
	if _, found := Tree1.Search(TestTreeNode{S: "20"}); found {
		t.Errorf("Expected 20 to be gone after Delete.")
	}
	for _, s := range []string{"05", "10", "15", "25", "28", "30", "40"} {
		if _, found := Tree1.Search(TestTreeNode{S: s}); !found {
			t.Errorf("Expected to find %s after Delete(20), did not.", s)
		}
	}
	if size := Tree1.Length(); size != 7 {
		t.Errorf("Expected 7 nodes after Delete(20), got %d", size)
	}
}

func TestTreeSetGetData(t *testing.T) {
	el := &SplayTreeElement[TestTreeNode]{}
	el.SetData(TestTreeNode{S: "42"})
	if d := el.GetData(); d.S != "42" {
		t.Errorf("Expected GetData to return 42, got %+v", d)
	}
}

func TestTreeMinMax(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	if x, found := Tree1.FindMax(); !found || x.S != "09" {
		t.Errorf("Unexpected Max, got %+v found=%v", x, found)
	}
	// FindMax splays the maximum to the root.
	if Tree1.root.data.S != "09" {
		t.Errorf("Expected 09 splayed to the root by FindMax, got %s", Tree1.root.data.S)
	}

	if x, found := Tree1.FindMin(); !found || x.S != "00" {
		t.Errorf("Unexpected Min, got %+v found=%v", x, found)
	}
	if Tree1.root.data.S != "00" {
		t.Errorf("Expected 00 splayed to the root by FindMin, got %s", Tree1.root.data.S)
	}
}

func TestTreeDepth(t *testing.T) {
	Tree1 := newTestTree()

	if n := Tree1.Depth(); n != 0 {
		t.Errorf("Unexpected Depth for empty tree, got %d expected 0", n)
	}

	Tree1.Insert(TestTreeNode{S: "05"})
	if n := Tree1.Depth(); n != 1 {
		t.Errorf("Unexpected Depth for root-only tree, got %d expected 1", n)
	}

	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	// Unlike an unbalanced BST, a splay tree self-adjusts: after the
	// inserts above (each splaying its node to the root) the depth is
	// bounded well under n.
	n := Tree1.Depth()
	if n < 2 || n > 5 {
		t.Errorf("Unexpected Depth, got %d (5 nodes)", n)
	}
}

func TestTreeDeleteAtTail(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	if !Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to find a node, did not.")
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Expected 4 nodes after DeleteAtTail, got %d", size)
	}
	if x, found := Tree1.FindMax(); !found || x.S != "05" {
		t.Errorf("Expected max of 05 after DeleteAtTail, got %+v", x)
	}
}

func TestTreeDeleteAtHead(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to find a node, did not.")
	}

	if size := Tree1.Length(); size != 4 {
		t.Errorf("Expected 4 nodes after DeleteAtHead, got %d", size)
	}
	if x, found := Tree1.FindMin(); !found || x.S != "02" {
		t.Errorf("Expected min of 02 after DeleteAtHead, got %+v", x)
	}
}

// TestTreeEmptyOps verifies that operations on an empty tree behave sanely.
func TestTreeEmptyOps(t *testing.T) {
	Tree1 := newTestTree()

	if _, found := Tree1.Search(TestTreeNode{S: "05"}); found {
		t.Errorf("Expected not-found from Search on empty tree.")
	}
	if _, found := Tree1.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on empty tree.")
	}
	if _, found := Tree1.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on empty tree.")
	}
	if Tree1.DeleteAtHead() {
		t.Errorf("Expected false from DeleteAtHead on empty tree.")
	}
	if Tree1.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtTail on empty tree.")
	}
	if Tree1.Delete(TestTreeNode{S: "05"}) {
		t.Errorf("Expected false from Delete on empty tree.")
	}
	if n := 0; Tree1.Len() != n || Tree1.Length() != n {
		t.Errorf("Expected 0 length on empty tree.")
	}
}

// TestTreeTruncate verifies that Truncate empties the tree but keeps it
// usable (the comparison function is kept).
func TestTreeTruncate(t *testing.T) {
	Tree1 := newTestTree()

	for _, s := range []string{"05", "02", "09", "00", "03"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	Tree1.Truncate()
	if !Tree1.IsEmpty() || Tree1.Length() != 0 || Tree1.Depth() != 0 {
		t.Errorf("Expected fully empty tree after Truncate.")
	}
	// The tree must be fully reusable after Truncate.
	for _, s := range []string{"50", "20", "90"} {
		Tree1.Insert(TestTreeNode{S: s})
	}
	if Tree1.Length() != 3 {
		t.Errorf("Expected length 3 after re-fill, got %d", Tree1.Length())
	}
	if _, found := Tree1.Search(TestTreeNode{S: "20"}); !found {
		t.Errorf("Expected to find 20 after re-fill.")
	}
	// Truncating an already-empty tree is fine.
	Tree1.Truncate()
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after Truncate on empty tree.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors: natural ordering, comparison functions, zero value
// -------------------------------------------------------------------------------------------------------

// TestCompare verifies the exported Compare helper used by NewSplayTree.
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
	if c := Compare("a", "a"); c != 0 {
		t.Errorf("Compare(a,a) = %d, expected 0", c)
	}
	if c := Compare(2.5, 2.25); c != 1 {
		t.Errorf("Compare(2.5,2.25) = %d, expected +1", c)
	}
}

// TestNewSplayTreeOrdered verifies the constructor for naturally ordered
// key types: integers, floats and strings ordered by their own operators,
// with no comparison function supplied.
func TestNewSplayTreeOrdered(t *testing.T) {
	ints := NewSplayTree[int]()
	for _, v := range []int{5, 2, 9, 0, 3} {
		if !ints.Insert(v) {
			t.Errorf("Expected Insert of %d to return true.", v)
		}
	}
	var gotInts []int
	for _, v := range ints.All() {
		gotInts = append(gotInts, v)
	}
	if expect := []int{0, 2, 3, 5, 9}; !reflect.DeepEqual(gotInts, expect) {
		t.Errorf("NewSplayTree[int] order error, expected %v got %v", expect, gotInts)
	}
	if x, found := ints.FindMin(); !found || x != 0 {
		t.Errorf("Expected min 0, got %d found=%v", x, found)
	}
	if x, found := ints.FindMax(); !found || x != 9 {
		t.Errorf("Expected max 9, got %d found=%v", x, found)
	}

	strs := NewSplayTree[string]()
	for _, s := range []string{"pear", "apple", "fig"} {
		strs.Insert(s)
	}
	if x, found := strs.FindMin(); !found || x != "apple" {
		t.Errorf("Expected min apple, got %q found=%v", x, found)
	}
	if x, found := strs.FindMax(); !found || x != "pear" {
		t.Errorf("Expected max pear, got %q found=%v", x, found)
	}

	floats := NewSplayTree[float64]()
	for _, f := range []float64{2.5, 1.5, 3.5} {
		floats.Insert(f)
	}
	if x, found := floats.FindMin(); !found || x != 1.5 {
		t.Errorf("Expected min 1.5, got %v found=%v", x, found)
	}

	// Duplicates replace: 5 replaces 5, length stays.
	if ints.Insert(5) {
		t.Errorf("Expected duplicate Insert to return false.")
	}
	if ints.Length() != 5 {
		t.Errorf("Expected length 5 after duplicate, got %d", ints.Length())
	}
}

// TestNewSplayTreeFunc verifies the constructor that takes a comparison
// function, including ordering by a field that is not the natural order of
// the struct.
func TestNewSplayTreeFunc(t *testing.T) {
	// Order TestTreeNode by N instead of S.
	tree := NewSplayTreeFunc(func(a, b TestTreeNode) int {
		return a.N - b.N
	})
	for _, n := range []TestTreeNode{{S: "five", N: 5}, {S: "two", N: 2}, {S: "nine", N: 9}} {
		tree.Insert(n)
	}
	if x, found := tree.FindMin(); !found || x.S != "two" {
		t.Errorf("Expected min two, got %+v found=%v", x, found)
	}
	var got []string
	for _, v := range tree.All() {
		got = append(got, v.S)
	}
	if expect := []string{"two", "five", "nine"}; !reflect.DeepEqual(got, expect) {
		t.Errorf("NewSplayTreeFunc order error, expected %v got %v", expect, got)
	}
}

// TestNewSplayTreeFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewSplayTreeFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewSplayTreeFunc(nil) to panic.")
		}
	}()
	NewSplayTreeFunc[TestTreeNode](nil)
}

// TestZeroValueTree verifies that the zero value of SplayTree behaves as
// an empty tree for every non-ordering operation and that Insert fails
// loudly because no comparison function has been set.
func TestZeroValueTree(t *testing.T) {
	var tree SplayTree[TestTreeNode]

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
	if _, found := tree.Search(TestTreeNode{S: "05"}); found {
		t.Errorf("Expected not-found from Search on zero value tree.")
	}
	if tree.Delete(TestTreeNode{S: "05"}) {
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSplayTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		tree.Insert(TestTreeNode{S: "05"})
	}()
}

// -------------------------------------------------------------------------------------------------------
// Iterators
// -------------------------------------------------------------------------------------------------------

func TestTreeAll(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	var idx []int
	for i, v := range Tree1.All() {
		idx = append(idx, i)
		x = append(x, v.S)
	}
	expect := []string{"00", "02", "03", "05", "09"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("All error, expected %s got %s", expect, x)
	}
	if expectIdx := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(idx, expectIdx) {
		t.Errorf("All index error, expected %v got %v", expectIdx, idx)
	}

	// A single-variable range yields the INDEX, not the value.
	var onlyIdx []int
	for i := range Tree1.All() {
		onlyIdx = append(onlyIdx, i)
	}
	if expectIdx := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(onlyIdx, expectIdx) {
		t.Errorf("All single-variable range error, expected indexes %v got %v", expectIdx, onlyIdx)
	}

	// Early break must stop the iteration.
	x = x[:0]
	for _, v := range Tree1.All() {
		x = append(x, v.S)
		if v.S == "02" {
			break
		}
	}
	if expect := []string{"00", "02"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("All early break error, expected %s got %s", expect, x)
	}

	// Empty tree yields nothing.
	empty := newTestTree()
	for range empty.All() {
		t.Errorf("All on empty tree yielded a value.")
	}
}

func TestTreeBackward(t *testing.T) {
	Tree1 := newTestTree()

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	var x []string
	var idx []int
	for i, v := range Tree1.Backward() {
		idx = append(idx, i)
		x = append(x, v.S)
	}
	expect := []string{"09", "05", "03", "02", "00"}
	if !reflect.DeepEqual(x, expect) {
		t.Errorf("Backward error, expected %s got %s", expect, x)
	}
	// Indexes count down from Len()-1, matching All's assignment.
	if expectIdx := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(idx, expectIdx) {
		t.Errorf("Backward index error, expected %v got %v", expectIdx, idx)
	}

	// Early break must stop the iteration.
	x = x[:0]
	for _, v := range Tree1.Backward() {
		x = append(x, v.S)
		if v.S == "05" {
			break
		}
	}
	if expect := []string{"09", "05"}; !reflect.DeepEqual(x, expect) {
		t.Errorf("Backward early break error, expected %s got %s", expect, x)
	}

	// Empty tree yields nothing.
	empty := newTestTree()
	for range empty.Backward() {
		t.Errorf("Backward on empty tree yielded a value.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkTreeSize = 1000

func BenchmarkInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tree := newTestTree()
		for j := range benchmarkTreeSize {
			tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	tree := newTestTree()
	for j := range benchmarkTreeSize {
		tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	find := TestTreeNode{S: fmt.Sprintf("%06d", benchmarkTreeSize/2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(find)
	}
}

func BenchmarkDelete(b *testing.B) {
	tree := newTestTree()
	for j := range benchmarkTreeSize {
		tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range benchmarkTreeSize {
			tree.Delete(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
		for j := range benchmarkTreeSize {
			tree.Insert(TestTreeNode{S: fmt.Sprintf("%06d", j)})
		}
	}
}

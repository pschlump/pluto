/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set_ts_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/tree_set_ts"
)

// One example for every exported function and method, with the locking
// behavior called out where it matters.  Outputs are deterministic —
// the snapshots are ordered by the comparator.

func ExampleNewTreeSet() {
	set := tree_set_ts.NewTreeSet[int]() // ordered by the built-in < on int
	set.Insert(3)
	set.Insert(1)
	fmt.Println(set)
	// Output: [1 3]
}

func ExampleNewTreeSetFunc() {
	set := tree_set_ts.NewTreeSetFunc(func(a, b string) int { return len(a) - len(b) }) // by length
	set.Insert("pear")                                                                  // length 4
	set.Insert("fig")                                                                   // length 3
	set.Insert("date")                                                                  // length 4: a duplicate under this comparator — replaces "pear"
	fmt.Println(set)
	// Output: [fig date]
}

func ExampleFromSlice() {
	set := tree_set_ts.FromSlice([]string{"b", "a", "b"})
	fmt.Println(set, set.Len())
	// Output: [a b] 2
}

func ExampleFromSliceFunc() {
	set := tree_set_ts.FromSliceFunc(func(a, b int) int { return b - a }, []int{1, 3, 2})
	fmt.Println(set)
	// Output: [3 2 1]
}

func ExampleTreeSet_Insert() {
	set := tree_set_ts.NewTreeSet[string]()
	fmt.Println(set.Insert("a"), set.Insert("a"))
	// Output: true false
}

func ExampleTreeSet_Delete() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	fmt.Println(set.Delete("a"), set.Delete("z"))
	// Output: true false
}

func ExampleTreeSet_Search() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	item, found := set.Search("b")
	fmt.Println(item, found)
	// Output: b true
}

func ExampleTreeSet_Contains() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	fmt.Println(set.Contains("b"), set.Contains("z"))
	// Output: true false
}

func ExampleTreeSet_Len() {
	fmt.Println(tree_set_ts.FromSlice([]string{"a", "b", "a"}).Len())
	// Output: 2
}

func ExampleTreeSet_Length() {
	fmt.Println(tree_set_ts.NewTreeSet[int]().Length())
	// Output: 0
}

func ExampleTreeSet_IsEmpty() {
	set := tree_set_ts.NewTreeSet[int]()
	fmt.Println(set.IsEmpty())
	set.Insert(1)
	fmt.Println(set.IsEmpty())
	// Output:
	// true
	// false
}

func ExampleTreeSet_Truncate() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	set.Truncate()
	fmt.Println(set.IsEmpty(), set.Insert("c")) // the comparator is kept
	// Output: true true
}

func ExampleTreeSet_FindMin() {
	set := tree_set_ts.FromSlice([]string{"pear", "apple"})
	min, found := set.FindMin()
	fmt.Println(min, found)
	// Output: apple true
}

func ExampleTreeSet_FindMax() {
	set := tree_set_ts.FromSlice([]string{"pear", "apple"})
	max, found := set.FindMax()
	fmt.Println(max, found)
	// Output: pear true
}

func ExampleTreeSet_Floor() {
	set := tree_set_ts.FromSlice([]string{"ant", "cat", "emu"})
	v, found := set.Floor("dog")
	fmt.Println(v, found)
	// Output: cat true
}

func ExampleTreeSet_Ceil() {
	set := tree_set_ts.FromSlice([]string{"ant", "cat", "emu"})
	v, found := set.Ceil("dog")
	fmt.Println(v, found)
	// Output: emu true
}

func ExampleTreeSet_DeleteAtHead() {
	set := tree_set_ts.FromSlice([]string{"b", "a", "c"})
	fmt.Println(set.DeleteAtHead(), set)
	// Output: true [b c]
}

func ExampleTreeSet_DeleteAtTail() {
	set := tree_set_ts.FromSlice([]string{"b", "a", "c"})
	fmt.Println(set.DeleteAtTail(), set)
	// Output: true [a b]
}

func ExampleTreeSet_Index() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	v, found := set.Index(0)
	fmt.Println(v, found)
	// Output: a true
}

func ExampleTreeSet_Slice() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.Slice()) // a consistent snapshot under the read lock
	// Output: [a b c]
}

func ExampleTreeSet_String() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.String())
	// Output: [a b c]
}

func ExampleTreeSet_StringFunc() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.StringFunc(strings.ToUpper)) // format runs with no lock held
	// Output: [A B C]
}

func ExampleTreeSet_Equal() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"b", "a"})
	c := tree_set_ts.FromSlice([]string{"a"})
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleTreeSet_Clone() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	dup := set.Clone() // a consistent snapshot; no memory shared
	set.Insert("c")
	fmt.Println(set, dup)
	// Output: [a b c] [a b]
}

func ExampleTreeSet_InsertSlice() {
	set := tree_set_ts.FromSlice([]string{"a"})
	fmt.Println(set.InsertSlice([]string{"a", "b"}), set) // atomic batch
	// Output: true [a b]
}

func ExampleTreeSet_InsertIter() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.NewTreeSet[string]()
	fmt.Println(b.InsertIter(a.All()), b) // drained before the write lock
	// Output: true [a b]
}

func ExampleTreeSet_DeleteSlice() {
	set := tree_set_ts.FromSlice([]string{"a", "b", "c"})
	fmt.Println(set.DeleteSlice([]string{"a", "z"}), set) // atomic batch
	// Output: true [b c]
}

func ExampleTreeSet_DeleteFunc() {
	set := tree_set_ts.FromSlice([]string{"a1", "b2"})
	fmt.Println(set.DeleteFunc(func(v string) bool { return strings.HasSuffix(v, "2") }), set)
	// Output: true [a1]
}

func ExampleTreeSet_ContainsSlice() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	fmt.Println(set.ContainsSlice([]string{"a", "b"}), set.ContainsSlice([]string{"a", "z"}))
	// Output: true false
}

func ExampleTreeSet_ContainsAny() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	fmt.Println(set.ContainsAny([]string{"x", "b"}), set.ContainsAny([]string{"x", "y"}))
	// Output: true false
}

func ExampleTreeSet_Union() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"b", "c"})
	dst := tree_set_ts.NewTreeSet[string]()
	dst.Union(a, b) // operands snapshotted under their own locks
	fmt.Println(dst)
	// Output: [a b c]
}

func ExampleTreeSet_Intersect() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"b", "c"})
	dst := tree_set_ts.NewTreeSet[string]()
	dst.Intersect(a, b)
	fmt.Println(dst)
	// Output: [b]
}

func ExampleTreeSet_Minus() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"b", "c"})
	dst := tree_set_ts.NewTreeSet[string]()
	dst.Minus(a, b)
	fmt.Println(dst)
	// Output: [a]
}

func ExampleTreeSet_SymmetricDifference() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"b", "c"})
	dst := tree_set_ts.NewTreeSet[string]()
	dst.SymmetricDifference(a, b)
	fmt.Println(dst)
	// Output: [a c]
}

func ExampleTreeSet_IsSubsetOf() {
	s := tree_set_ts.FromSlice([]string{"a", "b"})
	sup := tree_set_ts.FromSlice([]string{"a", "b", "c"})
	fmt.Println(s.IsSubsetOf(sup), sup.IsSubsetOf(s))
	// Output: true false
}

func ExampleTreeSet_IsProperSubsetOf() {
	s := tree_set_ts.FromSlice([]string{"a", "b"})
	sup := tree_set_ts.FromSlice([]string{"a", "b", "c"})
	fmt.Println(s.IsProperSubsetOf(sup))
	// Output: true
}

func ExampleTreeSet_IsSupersetOf() {
	s := tree_set_ts.FromSlice([]string{"a", "b"})
	sub := tree_set_ts.FromSlice([]string{"a"})
	fmt.Println(s.IsSupersetOf(sub))
	// Output: true
}

func ExampleTreeSet_IsProperSupersetOf() {
	s := tree_set_ts.FromSlice([]string{"a", "b"})
	sub := tree_set_ts.FromSlice([]string{"a"})
	fmt.Println(s.IsProperSupersetOf(sub))
	// Output: true
}

func ExampleTreeSet_IsDisjointWith() {
	a := tree_set_ts.FromSlice([]string{"a", "b"})
	b := tree_set_ts.FromSlice([]string{"x", "y"})
	fmt.Println(a.IsDisjointWith(b))
	// Output: true
}

func ExampleTreeSet_All() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	for v := range set.All() { // a snapshot: mutate the set freely, even in the loop
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: a b c
}

func ExampleTreeSet_Backward() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	for v := range set.Backward() {
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: c b a
}

func ExampleTreeSet_Range() {
	set := tree_set_ts.FromSlice([]string{"ant", "cat", "emu", "fox"})
	for v := range set.Range("b", "fox") {
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: cat emu fox
}

func ExampleTreeSet_WalkInOrder() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	set.WalkInOrder(func(pos, depth int, data string) bool {
		fmt.Println(pos, data) // read lock held: do NOT call back into the set
		return true
	})
	// Output:
	// 0 a
	// 1 b
	// 2 c
}

func ExampleTreeSet_WalkPreOrder() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	set.WalkPreOrder(func(pos, depth int, data string) bool {
		fmt.Println("visited", data)
		return true
	})
	// Pre-order visits subtree roots first; the exact sequence depends
	// on the tree's shape, so this example has no fixed output.
}

func ExampleTreeSet_WalkPostOrder() {
	set := tree_set_ts.FromSlice([]string{"c", "a", "b"})
	set.WalkPostOrder(func(pos, depth int, data string) bool {
		fmt.Println("visited", data)
		return true
	})
	// Post-order visits both subtrees before the node; the exact
	// sequence depends on the tree's shape, so this example has no
	// fixed output.
}

func ExampleTreeSet_MarshalJSON() {
	set := tree_set_ts.FromSlice([]string{"b", "a"})
	data, err := set.MarshalJSON()
	fmt.Println(string(data), err)
	// Output: ["a","b"] <nil>
}

func ExampleTreeSet_UnmarshalJSON() {
	set := tree_set_ts.NewTreeSet[string]()
	err := set.UnmarshalJSON([]byte(`["b","a","b"]`))
	fmt.Println(set, err)
	// Output: [a b] <nil>
}

func ExampleTreeSet_Lock() {
	set := tree_set_ts.NewTreeSet[string]()
	set.Lock() // hold the exclusive lock across a multi-step sequence
	set.NlInsert("b")
	set.NlInsert("a")
	set.Unlock()
	fmt.Println(set)
	// Output: [a b]
}

func ExampleTreeSet_Unlock() {
	set := tree_set_ts.FromSlice([]string{"a"})
	set.Lock()
	set.NlInsert("b")
	set.Unlock() // release; every public method is safe again
	fmt.Println(set.Len())
	// Output: 2
}

func ExampleTreeSet_NlInsert() {
	set := tree_set_ts.NewTreeSet[string]()
	set.Lock()
	set.NlInsert("a") // no lock taken: the caller holds it
	set.Unlock()
	fmt.Println(set)
	// Output: [a]
}

func ExampleTreeSet_NlDelete() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	set.Lock()
	set.NlDelete("a")
	set.Unlock()
	fmt.Println(set)
	// Output: [b]
}

func ExampleTreeSet_NlSearch() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	set.Lock()
	item, found := set.NlSearch("b")
	set.Unlock()
	fmt.Println(item, found)
	// Output: b true
}

func ExampleTreeSet_NlContains() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	set.Lock()
	fmt.Println(set.NlContains("a"))
	set.Unlock()
	// Output: true
}

func ExampleTreeSet_NlLen() {
	set := tree_set_ts.FromSlice([]string{"a", "b"})
	set.Lock()
	fmt.Println(set.NlLen())
	set.Unlock()
	// Output: 2
}

func ExampleTreeSet_NlIsEmpty() {
	set := tree_set_ts.NewTreeSet[string]()
	set.Lock()
	fmt.Println(set.NlIsEmpty())
	set.Unlock()
	// Output: true
}

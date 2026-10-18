/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/tree_set"
)

// The examples double as documentation: one for every exported function
// and method.  Outputs are deterministic — the set walks its elements in
// its comparator's ascending order.

func ExampleNewTreeSet() {
	set := tree_set.NewTreeSet[string]() // ordering by the built-in < on string
	set.Insert("pear")
	set.Insert("apple")
	fmt.Println(set)
	// Output: [apple pear]
}

func ExampleNewTreeSetFunc() {
	// A struct ordered by one field, with a satellite payload the
	// comparison ignores.
	type person struct {
		Name string
		Age  int
	}
	set := tree_set.NewTreeSetFunc(func(a, b person) int { return strings.Compare(a.Name, b.Name) })
	set.Insert(person{"Zoe", 31})
	set.Insert(person{"Ari", 25})
	fmt.Println(set)
	// Output: [{Ari 25} {Zoe 31}]
}

func ExampleFromSlice() {
	set := tree_set.FromSlice([]string{"b", "a", "b", "c", "a"}) // duplicates collapse
	fmt.Println(set, set.Len())
	// Output: [a b c] 3
}

func ExampleFromSliceFunc() {
	set := tree_set.FromSliceFunc(func(a, b int) int { return b - a }, []int{1, 3, 2}) // descending
	fmt.Println(set)
	// Output: [3 2 1]
}

func ExampleTreeSet_Insert() {
	set := tree_set.NewTreeSet[int]()
	fmt.Println(set.Insert(3), set.Insert(3))
	// Output: true false
}

func ExampleTreeSet_Delete() {
	set := tree_set.FromSlice([]string{"a", "b"})
	fmt.Println(set.Delete("a"), set.Delete("zz"))
	// Output: true false
}

func ExampleTreeSet_Search() {
	set := tree_set.NewTreeSetFunc(func(a, b string) int { return strings.Compare(a, b) })
	set.Insert("full-payload")
	item, found := set.Search("full-payload") // a probe equal by comparison
	fmt.Println(item, found)
	// Output: full-payload true
}

func ExampleTreeSet_Contains() {
	set := tree_set.FromSlice([]string{"a", "b"})
	fmt.Println(set.Contains("b"), set.Contains("z"))
	// Output: true false
}

func ExampleTreeSet_Len() {
	fmt.Println(tree_set.FromSlice([]string{"a", "b", "a"}).Len())
	// Output: 2
}

func ExampleTreeSet_Length() {
	fmt.Println(tree_set.NewTreeSet[int]().Length())
	// Output: 0
}

func ExampleTreeSet_IsEmpty() {
	set := tree_set.NewTreeSet[int]()
	fmt.Println(set.IsEmpty())
	set.Insert(1)
	fmt.Println(set.IsEmpty())
	// Output:
	// true
	// false
}

func ExampleTreeSet_Truncate() {
	set := tree_set.FromSlice([]string{"a", "b"})
	set.Truncate()
	fmt.Println(set.IsEmpty(), set.Insert("c")) // the comparator is kept
	// Output: true true
}

func ExampleTreeSet_FindMin() {
	set := tree_set.FromSlice([]string{"pear", "apple", "fig"})
	min, found := set.FindMin()
	fmt.Println(min, found)
	// Output: apple true
}

func ExampleTreeSet_FindMax() {
	set := tree_set.FromSlice([]string{"pear", "apple", "fig"})
	max, found := set.FindMax()
	fmt.Println(max, found)
	// Output: pear true
}

func ExampleTreeSet_Floor() {
	set := tree_set.FromSlice([]string{"ant", "cat", "emu"})
	v, found := set.Floor("dog") // largest element <= dog
	fmt.Println(v, found)
	// Output: cat true
}

func ExampleTreeSet_Ceil() {
	set := tree_set.FromSlice([]string{"ant", "cat", "emu"})
	v, found := set.Ceil("dog") // smallest element >= dog
	fmt.Println(v, found)
	// Output: emu true
}

func ExampleTreeSet_DeleteAtHead() {
	set := tree_set.FromSlice([]string{"b", "a", "c"})
	fmt.Println(set.DeleteAtHead(), set)
	// Output: true [b c]
}

func ExampleTreeSet_DeleteAtTail() {
	set := tree_set.FromSlice([]string{"b", "a", "c"})
	fmt.Println(set.DeleteAtTail(), set)
	// Output: true [a b]
}

func ExampleTreeSet_Index() {
	set := tree_set.FromSlice([]string{"c", "a", "b"}) // ascending: a, b, c
	v, found := set.Index(2)
	fmt.Println(v, found)
	// Output: c true
}

func ExampleTreeSet_Slice() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.Slice())
	// Output: [a b c]
}

func ExampleTreeSet_String() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.String())
	// Output: [a b c]
}

func ExampleTreeSet_StringFunc() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	fmt.Println(set.StringFunc(strings.ToUpper))
	// Output: [A B C]
}

func ExampleTreeSet_Equal() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"b", "a", "a"}) // same set, other history
	c := tree_set.FromSlice([]string{"a"})
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleTreeSet_Clone() {
	set := tree_set.FromSlice([]string{"a", "b"})
	dup := set.Clone()
	set.Insert("c")
	fmt.Println(set, dup)
	// Output: [a b c] [a b]
}

func ExampleTreeSet_InsertSlice() {
	set := tree_set.FromSlice([]string{"a"})
	fmt.Println(set.InsertSlice([]string{"a", "b"}), set)
	// Output: true [a b]
}

func ExampleTreeSet_InsertIter() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.NewTreeSet[string]()
	fmt.Println(b.InsertIter(a.All()), b) // every pluto All() feeds it
	// Output: true [a b]
}

func ExampleTreeSet_DeleteSlice() {
	set := tree_set.FromSlice([]string{"a", "b", "c"})
	fmt.Println(set.DeleteSlice([]string{"a", "zz"}), set)
	// Output: true [b c]
}

func ExampleTreeSet_DeleteFunc() {
	set := tree_set.FromSlice([]string{"a1", "b2", "c3"})
	fmt.Println(set.DeleteFunc(func(v string) bool { return strings.HasSuffix(v, "2") }), set)
	// Output: true [a1 c3]
}

func ExampleTreeSet_ContainsSlice() {
	set := tree_set.FromSlice([]string{"a", "b"})
	fmt.Println(set.ContainsSlice([]string{"a", "b"}), set.ContainsSlice([]string{"a", "z"}))
	// Output: true false
}

func ExampleTreeSet_ContainsAny() {
	set := tree_set.FromSlice([]string{"a", "b"})
	fmt.Println(set.ContainsAny([]string{"x", "b"}), set.ContainsAny([]string{"x", "y"}))
	// Output: true false
}

func ExampleTreeSet_Union() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"b", "c"})
	dst := tree_set.NewTreeSet[string]()
	dst.Union(a, b)
	fmt.Println(dst)
	// Output: [a b c]
}

func ExampleTreeSet_Intersect() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"b", "c"})
	dst := tree_set.NewTreeSet[string]()
	dst.Intersect(a, b)
	fmt.Println(dst)
	// Output: [b]
}

func ExampleTreeSet_Minus() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"b", "c"})
	dst := tree_set.NewTreeSet[string]()
	dst.Minus(a, b)
	fmt.Println(dst)
	// Output: [a]
}

func ExampleTreeSet_SymmetricDifference() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"b", "c"})
	dst := tree_set.NewTreeSet[string]()
	dst.SymmetricDifference(a, b)
	fmt.Println(dst)
	// Output: [a c]
}

func ExampleTreeSet_IsSubsetOf() {
	s := tree_set.FromSlice([]string{"a", "b"})
	sup := tree_set.FromSlice([]string{"a", "b", "c"})
	fmt.Println(s.IsSubsetOf(sup), sup.IsSubsetOf(s))
	// Output: true false
}

func ExampleTreeSet_IsProperSubsetOf() {
	s := tree_set.FromSlice([]string{"a", "b"})
	same := tree_set.FromSlice([]string{"b", "a"})
	sup := tree_set.FromSlice([]string{"a", "b", "c"})
	fmt.Println(s.IsProperSubsetOf(same), s.IsProperSubsetOf(sup))
	// Output: false true
}

func ExampleTreeSet_IsSupersetOf() {
	s := tree_set.FromSlice([]string{"a", "b"})
	sub := tree_set.FromSlice([]string{"a"})
	fmt.Println(s.IsSupersetOf(sub), sub.IsSupersetOf(s))
	// Output: true false
}

func ExampleTreeSet_IsProperSupersetOf() {
	s := tree_set.FromSlice([]string{"a", "b"})
	same := tree_set.FromSlice([]string{"b", "a"})
	sub := tree_set.FromSlice([]string{"a"})
	fmt.Println(s.IsProperSupersetOf(same), s.IsProperSupersetOf(sub))
	// Output: false true
}

func ExampleTreeSet_IsDisjointWith() {
	a := tree_set.FromSlice([]string{"a", "b"})
	b := tree_set.FromSlice([]string{"x", "y"})
	c := tree_set.FromSlice([]string{"b", "x"})
	fmt.Println(a.IsDisjointWith(b), a.IsDisjointWith(c))
	// Output: true false
}

func ExampleTreeSet_All() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	for v := range set.All() { // ascending
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: a b c
}

func ExampleTreeSet_Backward() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	for v := range set.Backward() { // descending
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: c b a
}

func ExampleTreeSet_Range() {
	set := tree_set.FromSlice([]string{"ant", "cat", "emu", "fox"})
	for v := range set.Range("b", "fox") { // inclusive on both ends
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: cat emu fox
}

func ExampleTreeSet_WalkInOrder() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	set.WalkInOrder(func(pos, depth int, data string) bool {
		fmt.Println(pos, data)
		return true
	})
	// Output:
	// 0 a
	// 1 b
	// 2 c
}

func ExampleTreeSet_WalkPreOrder() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	set.WalkPreOrder(func(pos, depth int, data string) bool {
		fmt.Println(depth, data)
		return true
	})
	// Pre-order visits the subtree roots first; the exact (depth, data)
	// pairs depend on the tree's shape, so this example prints nothing
	// fixed — it only shows the callback shape.
}

func ExampleTreeSet_WalkPostOrder() {
	set := tree_set.FromSlice([]string{"c", "a", "b"})
	set.WalkPostOrder(func(pos, depth int, data string) bool {
		fmt.Println(pos, data)
		return true
	})
	// Post-order visits both subtrees before the node; as with
	// WalkPreOrder the exact sequence depends on the tree's shape, so
	// this example prints nothing fixed.
}

func ExampleTreeSet_MarshalJSON() {
	set := tree_set.FromSlice([]string{"b", "a"})
	data, err := set.MarshalJSON()
	fmt.Println(string(data), err)
	// Output: ["a","b"] <nil>
}

func ExampleTreeSet_UnmarshalJSON() {
	set := tree_set.NewTreeSet[string]()
	err := set.UnmarshalJSON([]byte(`["b","a","b"]`)) // duplicates collapse
	fmt.Println(set, err)
	// Output: [a b] <nil>
}

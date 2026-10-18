/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pschlump/pluto/hash_set"
)

func ExampleNewHashSet() {
	ss := hash_set.NewHashSet[string]()
	ss.Insert("pear")
	ss.Insert("apple")
	fmt.Println(ss.Len(), ss.Contains("apple"), ss.Contains("plum"))
	// Output: 2 true false
}

func ExampleNewHashSetFunc() {
	// Element types that cannot be compared with == — or equality by
	// value rather than identity — use NewHashSetFunc.  The hash must
	// agree with the equality (equal strings hash equal), which
	// lowercasing guarantees here.
	ss := hash_set.NewHashSetFunc(
		func(a, b []byte) bool { return string(a) == string(b) },
		func(a []byte) uint64 {
			var h uint64 = 14695981039346656037
			for _, c := range strings.ToLower(string(a)) {
				h ^= uint64(c)
				h *= 1099511628211
			}
			return h
		},
	)
	ss.Insert([]byte("Go"))
	fmt.Println(ss.Contains([]byte("Go")), ss.Len())
	// Output: true 1
}

func ExampleFromSlice() {
	ss := hash_set.FromSlice([]int{3, 1, 4, 1, 5, 9, 2, 6, 5})
	fmt.Println(ss.Len(), ss.String())
	// Output: 7 [1 2 3 4 5 6 9]
}

func ExampleFromSliceFunc() {
	eq := func(a, b string) bool { return strings.EqualFold(a, b) }
	hash := func(a string) uint64 {
		var h uint64 = 14695981039346656037
		for _, c := range strings.ToLower(a) {
			h ^= uint64(c)
			h *= 1099511628211
		}
		return h
	}
	ss := hash_set.FromSliceFunc(eq, hash, []string{"red", "RED", "green"})
	fmt.Println(ss.Len(), ss.Contains("Red"))
	// Output: 2 true
}

func ExampleHashSet_Insert() {
	ss := hash_set.NewHashSet[int]()
	fmt.Println(ss.Insert(10), ss.Insert(10)) // new, duplicate
	// Output: true false
}

func ExampleHashSet_Delete() {
	ss := hash_set.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.Delete(2), ss.Delete(99), ss.Len())
	// Output: true false 2
}

func ExampleHashSet_Search() {
	type point struct{ X, Y int }
	ss := hash_set.FromSlice([]point{{1, 2}, {3, 4}})
	p, found := ss.Search(point{3, 4})
	fmt.Println(p, found)
	_, found = ss.Search(point{9, 9})
	fmt.Println(found)
	// Output: {3 4} true
	// false
}

func ExampleHashSet_Contains() {
	ss := hash_set.FromSlice([]string{"a", "b"})
	fmt.Println(ss.Contains("a"), ss.Contains("z"))
	// Output: true false
}

func ExampleHashSet_Len() {
	ss := hash_set.FromSlice([]int{1, 2, 2, 3})
	fmt.Println(ss.Len())
	// Output: 3
}

func ExampleHashSet_IsEmpty() {
	ss := hash_set.NewHashSet[int]()
	fmt.Println(ss.IsEmpty())
	ss.Insert(1)
	fmt.Println(ss.IsEmpty())
	// Output: true
	// false
}

func ExampleHashSet_Truncate() {
	ss := hash_set.FromSlice([]int{1, 2, 3})
	ss.Truncate()
	fmt.Println(ss.Len(), ss.IsEmpty())
	ss.Insert(7) // the set stays usable: capacity and functions kept
	fmt.Println(ss.String())
	// Output: 0 true
	// [7]
}

func ExampleHashSet_Eq() {
	ss := hash_set.NewHashSet[int]()
	fmt.Println(ss.Eq() != nil, ss.Hash() != nil)
	var zero hash_set.HashSet[int] // zero value: no functions yet
	fmt.Println(zero.Eq() == nil, zero.Hash() == nil)
	// Output: true true
	// true true
}

func ExampleHashSet_InsertSlice() {
	ss := hash_set.NewHashSet[int]()
	fmt.Println(ss.InsertSlice([]int{1, 2, 2}), ss.InsertSlice([]int{1}), ss.String())
	// Output: true false [1 2]
}

func ExampleHashSet_InsertIter() {
	// Any Go 1.23 iterator feeds the set — here another set's All().
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.NewHashSet[int]()
	fmt.Println(b.InsertIter(a.All()), b.InsertIter(a.All()), b.String())
	// Output: true false [1 2 3]
}

func ExampleHashSet_DeleteSlice() {
	ss := hash_set.FromSlice([]int{1, 2, 3, 4})
	fmt.Println(ss.DeleteSlice([]int{2, 99, 4}), ss.DeleteSlice([]int{99}), ss.String())
	// Output: true false [1 3]
}

func ExampleHashSet_DeleteFunc() {
	ss := hash_set.FromSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(ss.DeleteFunc(func(v int) bool { return v%2 == 0 }), ss.String())
	// Output: true [1 3 5]
}

func ExampleHashSet_ContainsSlice() {
	ss := hash_set.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.ContainsSlice([]int{1, 2}), ss.ContainsSlice([]int{1, 9}), ss.ContainsSlice(nil))
	// Output: true false true
}

func ExampleHashSet_ContainsAny() {
	ss := hash_set.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.ContainsAny([]int{9, 2}), ss.ContainsAny([]int{9, 8}), ss.ContainsAny(nil))
	// Output: true false false
}

func ExampleHashSet_Slice() {
	ss := hash_set.FromSlice([]int{3, 1, 2})
	items := ss.Slice() // UNSPECIFIED order: sort it when order matters
	slices.Sort(items)
	fmt.Println(items)
	empty := hash_set.NewHashSet[int]()
	fmt.Println(empty.Slice() == nil, len(empty.Slice()))
	// Output: [1 2 3]
	// false 0
}

func ExampleHashSet_String() {
	// String sorts the formatted strings, so the output is deterministic
	// whatever order the per-set random hash seed scattered the slots.
	ss := hash_set.FromSlice([]int{10, 2, 33})
	fmt.Println(ss.String())
	// Output: [10 2 33]
}

func ExampleHashSet_StringFunc() {
	ss := hash_set.FromSlice([]string{"ant", "bee"})
	fmt.Println(ss.StringFunc(func(s string) string { return strings.ToUpper(s) }))
	// Output: [ANT BEE]
}

func ExampleHashSet_Equal() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{3, 1, 2}) // same content, different seed and slots
	c := hash_set.FromSlice([]int{1, 2})
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleHashSet_Clone() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := a.Clone()
	a.Insert(4) // deep copy: neither side sees the other's changes
	b.Delete(1)
	fmt.Println(a.String(), b.String())
	// Output: [1 2 3 4] [2 3]
}

func ExampleHashSet_Union() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{3, 4})
	r := hash_set.NewHashSet[int]()
	r.Union(a, b)
	fmt.Println(r.String())
	// Output: [1 2 3 4]
}

func ExampleHashSet_Intersect() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{3, 4})
	r := hash_set.NewHashSet[int]()
	r.Intersect(a, b)
	fmt.Println(r.String())
	// Output: [3]
}

func ExampleHashSet_Minus() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{3, 4})
	r := hash_set.NewHashSet[int]()
	r.Minus(a, b)
	fmt.Println(r.String())
	// Output: [1 2]
}

func ExampleHashSet_SymmetricDifference() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{3, 4})
	r := hash_set.NewHashSet[int]()
	r.SymmetricDifference(a, b)
	fmt.Println(r.String())
	// Output: [1 2 4]
}

func ExampleHashSet_IsSubsetOf() {
	a := hash_set.FromSlice([]int{1, 2, 3, 4})
	sub := hash_set.FromSlice([]int{2, 4})
	other := hash_set.FromSlice([]int{2, 9})
	fmt.Println(sub.IsSubsetOf(a), other.IsSubsetOf(a))
	// Output: true false
}

func ExampleHashSet_IsProperSubsetOf() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{1, 2, 3})
	sub := hash_set.FromSlice([]int{1, 2})
	fmt.Println(b.IsProperSubsetOf(a), sub.IsProperSubsetOf(a))
	// Output: false true
}

func ExampleHashSet_IsSupersetOf() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	sub := hash_set.FromSlice([]int{1, 2})
	fmt.Println(a.IsSupersetOf(sub), sub.IsSupersetOf(a))
	// Output: true false
}

func ExampleHashSet_IsProperSupersetOf() {
	a := hash_set.FromSlice([]int{1, 2, 3})
	b := hash_set.FromSlice([]int{1, 2, 3})
	sub := hash_set.FromSlice([]int{1, 2})
	fmt.Println(a.IsProperSupersetOf(sub), a.IsProperSupersetOf(b))
	// Output: true false
}

func ExampleHashSet_IsDisjointWith() {
	a := hash_set.FromSlice([]int{1, 2})
	b := hash_set.FromSlice([]int{3, 4})
	c := hash_set.FromSlice([]int{2, 9})
	fmt.Println(a.IsDisjointWith(b), a.IsDisjointWith(c))
	// Output: true false
}

func ExampleHashSet_All() {
	ss := hash_set.FromSlice([]int{3, 1, 2})
	items := make([]int, 0)
	for v := range ss.All() { // UNSPECIFIED order: sort when it matters
		items = append(items, v)
	}
	slices.Sort(items)
	fmt.Println(items)
	// Output: [1 2 3]
}

func ExampleHashSet_Walk() {
	ss := hash_set.FromSlice([]int{3, 1, 2})
	items := make([]int, 0)
	complete := ss.Walk(func(_ int, v int) bool { // false stops the walk
		items = append(items, v)
		return true
	})
	slices.Sort(items)
	fmt.Println(complete, items)
	// Output: true [1 2 3]
}

func ExampleHashSet_MarshalJSON() {
	ss := hash_set.FromSlice([]int{2, 1})
	data, err := json.Marshal(ss)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	// The array is in slot order, which varies from process to process —
	// decode and sort when a fixed order matters.
	var items []int
	if err := json.Unmarshal(data, &items); err != nil {
		fmt.Println("error:", err)
		return
	}
	slices.Sort(items)
	fmt.Println(items)
	// Output: [1 2]
}

func ExampleHashSet_UnmarshalJSON() {
	// Unmarshal REPLACES the contents; the equality/hash functions are
	// kept, so a set built with case-insensitive functions keeps them.
	eq := func(a, b string) bool { return strings.EqualFold(a, b) }
	hash := func(a string) uint64 {
		var h uint64 = 14695981039346656037
		for _, c := range strings.ToLower(a) {
			h ^= uint64(c)
			h *= 1099511628211
		}
		return h
	}
	ss := hash_set.NewHashSetFunc(eq, hash)
	if err := ss.UnmarshalJSON([]byte(`["Alpha","beta"]`)); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(ss.Len(), ss.Contains("ALPHA"))
	// Output: 2 true
}

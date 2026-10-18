/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set_ts_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pschlump/pluto/hash_set_ts"
)

func ExampleNewHashSet() {
	ss := hash_set_ts.NewHashSet[string]()
	ss.Insert("pear")
	ss.Insert("apple")
	fmt.Println(ss.Len(), ss.Contains("apple"))
	// Output: 2 true
}

func ExampleNewHashSetFunc() {
	ss := hash_set_ts.NewHashSetFunc(
		func(a, b string) bool { return strings.EqualFold(a, b) },
		func(a string) uint64 {
			var h uint64 = 14695981039346656037
			for _, c := range strings.ToLower(a) {
				h ^= uint64(c)
				h *= 1099511628211
			}
			return h
		},
	)
	ss.Insert("Go")
	fmt.Println(ss.Contains("GO"), ss.Len())
	// Output: true 1
}

func ExampleFromSlice() {
	ss := hash_set_ts.FromSlice([]int{3, 1, 4, 1, 5})
	fmt.Println(ss.Len(), ss.String())
	// Output: 4 [1 3 4 5]
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
	ss := hash_set_ts.FromSliceFunc(eq, hash, []string{"red", "RED", "green"})
	fmt.Println(ss.Len(), ss.Contains("Red"))
	// Output: 2 true
}

func ExampleHashSet_Lock() {
	// Lock + the Nl-prefixed methods build an atomic read-modify-write
	// compound; Unlock releases it.  Both are no-ops on a nil set.
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	ss.Lock()
	if _, found := ss.NlSearch(4); !found {
		ss.NlInsert(4)
	}
	l := ss.NlLen()
	ss.Unlock()
	fmt.Println(l, ss.String())
	// Output: 4 [1 2 3 4]
}

func ExampleHashSet_Unlock() {
	ss := hash_set_ts.FromSlice([]int{1, 2})
	ss.Lock()
	ss.NlDelete(2)
	ss.Unlock() // regular locking methods work again after Unlock
	fmt.Println(ss.String())
	// Output: [1]
}

func ExampleHashSet_Insert() {
	ss := hash_set_ts.NewHashSet[int]()
	fmt.Println(ss.Insert(10), ss.Insert(10))
	// Output: true false
}

func ExampleHashSet_Delete() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.Delete(2), ss.Delete(99), ss.Len())
	// Output: true false 2
}

func ExampleHashSet_Search() {
	ss := hash_set_ts.FromSlice([]string{"a", "b"})
	v, found := ss.Search("b")
	fmt.Println(v, found)
	// Output: b true
}

func ExampleHashSet_Contains() {
	ss := hash_set_ts.FromSlice([]int{1, 2})
	fmt.Println(ss.Contains(1), ss.Contains(9))
	// Output: true false
}

func ExampleHashSet_Len() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 2, 3})
	fmt.Println(ss.Len())
	// Output: 3
}

func ExampleHashSet_IsEmpty() {
	ss := hash_set_ts.NewHashSet[int]()
	fmt.Println(ss.IsEmpty())
	ss.Insert(1)
	fmt.Println(ss.IsEmpty())
	// Output: true
	// false
}

func ExampleHashSet_Truncate() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	ss.Truncate()
	fmt.Println(ss.Len())
	ss.Insert(7) // stays usable: capacity and functions kept
	fmt.Println(ss.String())
	// Output: 0
	// [7]
}

func ExampleHashSet_Eq() {
	ss := hash_set_ts.NewHashSet[int]()
	fmt.Println(ss.Eq() != nil, ss.Hash() != nil)
	var zero hash_set_ts.HashSet[int]
	fmt.Println(zero.Eq() == nil, zero.Hash() == nil)
	// Output: true true
	// true true
}

func ExampleHashSet_NlInsert() {
	ss := hash_set_ts.FromSlice([]int{1})
	ss.Lock() // the caller must hold the write lock
	defer ss.Unlock()
	fmt.Println(ss.NlInsert(2), ss.NlInsert(1))
	// Output: true false
}

func ExampleHashSet_NlDelete() {
	ss := hash_set_ts.FromSlice([]int{1, 2})
	ss.Lock()
	defer ss.Unlock()
	fmt.Println(ss.NlDelete(1), ss.NlDelete(9))
	// Output: true false
}

func ExampleHashSet_NlSearch() {
	ss := hash_set_ts.FromSlice([]int{1, 2})
	ss.Lock() // read or write lock both suffice for NlSearch
	v, found := ss.NlSearch(2)
	ss.Unlock()
	fmt.Println(v, found)
	// Output: 2 true
}

func ExampleHashSet_NlLen() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	ss.Lock()
	defer ss.Unlock()
	fmt.Println(ss.NlLen(), ss.NlIsEmpty())
	// Output: 3 false
}

func ExampleHashSet_NlIsEmpty() {
	ss := hash_set_ts.NewHashSet[int]()
	ss.Lock()
	defer ss.Unlock()
	fmt.Println(ss.NlIsEmpty())
	// Output: true
}

func ExampleHashSet_InsertSlice() {
	ss := hash_set_ts.NewHashSet[int]()
	fmt.Println(ss.InsertSlice([]int{1, 2, 2}), ss.String())
	// Output: true [1 2]
}

func ExampleHashSet_InsertIter() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.NewHashSet[int]()
	fmt.Println(b.InsertIter(a.All()), b.InsertIter(a.All()), b.String())
	// Output: true false [1 2 3]
}

func ExampleHashSet_DeleteSlice() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3, 4})
	fmt.Println(ss.DeleteSlice([]int{2, 99, 4}), ss.String())
	// Output: true [1 3]
}

func ExampleHashSet_DeleteFunc() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3, 4, 5, 6})
	// fx runs with NO lock held, so it may call back into the set.
	fmt.Println(ss.DeleteFunc(func(v int) bool { return v%2 == 0 }), ss.String())
	// Output: true [1 3 5]
}

func ExampleHashSet_ContainsSlice() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.ContainsSlice([]int{1, 2}), ss.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleHashSet_ContainsAny() {
	ss := hash_set_ts.FromSlice([]int{1, 2, 3})
	fmt.Println(ss.ContainsAny([]int{9, 2}), ss.ContainsAny([]int{9}))
	// Output: true false
}

func ExampleHashSet_Slice() {
	ss := hash_set_ts.FromSlice([]int{3, 1, 2})
	items := ss.Slice() // snapshot under the read lock; UNSPECIFIED order
	slices.Sort(items)
	fmt.Println(items)
	// Output: [1 2 3]
}

func ExampleHashSet_String() {
	ss := hash_set_ts.FromSlice([]int{10, 2, 33})
	fmt.Println(ss.String()) // deterministic: formatted strings sorted
	// Output: [10 2 33]
}

func ExampleHashSet_StringFunc() {
	ss := hash_set_ts.FromSlice([]string{"ant", "bee"})
	fmt.Println(ss.StringFunc(func(s string) string { return strings.ToUpper(s) }))
	// Output: [ANT BEE]
}

func ExampleHashSet_Equal() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{3, 1, 2})
	fmt.Println(a.Equal(b), a.Equal(hash_set_ts.FromSlice([]int{1})))
	// Output: true false
}

func ExampleHashSet_Clone() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := a.Clone()
	a.Insert(4)
	b.Delete(1)
	fmt.Println(a.String(), b.String())
	// Output: [1 2 3 4] [2 3]
}

func ExampleHashSet_Union() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{3, 4})
	r := hash_set_ts.NewHashSet[int]()
	r.Union(a, b)
	fmt.Println(r.String())
	// Output: [1 2 3 4]
}

func ExampleHashSet_Intersect() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{3, 4})
	r := hash_set_ts.NewHashSet[int]()
	r.Intersect(a, b)
	fmt.Println(r.String())
	// Output: [3]
}

func ExampleHashSet_Minus() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{3, 4})
	r := hash_set_ts.NewHashSet[int]()
	r.Minus(a, b)
	fmt.Println(r.String())
	// Output: [1 2]
}

func ExampleHashSet_SymmetricDifference() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{3, 4})
	r := hash_set_ts.NewHashSet[int]()
	r.SymmetricDifference(a, b)
	fmt.Println(r.String())
	// Output: [1 2 4]
}

func ExampleHashSet_IsSubsetOf() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3, 4})
	sub := hash_set_ts.FromSlice([]int{2, 4})
	other := hash_set_ts.FromSlice([]int{2, 9})
	fmt.Println(sub.IsSubsetOf(a), other.IsSubsetOf(a))
	// Output: true false
}

func ExampleHashSet_IsProperSubsetOf() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{1, 2, 3})
	sub := hash_set_ts.FromSlice([]int{1, 2})
	fmt.Println(b.IsProperSubsetOf(a), sub.IsProperSubsetOf(a))
	// Output: false true
}

func ExampleHashSet_IsSupersetOf() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	sub := hash_set_ts.FromSlice([]int{1, 2})
	fmt.Println(a.IsSupersetOf(sub), sub.IsSupersetOf(a))
	// Output: true false
}

func ExampleHashSet_IsProperSupersetOf() {
	a := hash_set_ts.FromSlice([]int{1, 2, 3})
	b := hash_set_ts.FromSlice([]int{1, 2, 3})
	sub := hash_set_ts.FromSlice([]int{1, 2})
	fmt.Println(a.IsProperSupersetOf(sub), a.IsProperSupersetOf(b))
	// Output: true false
}

func ExampleHashSet_IsDisjointWith() {
	a := hash_set_ts.FromSlice([]int{1, 2})
	b := hash_set_ts.FromSlice([]int{3, 4})
	c := hash_set_ts.FromSlice([]int{2, 9})
	fmt.Println(a.IsDisjointWith(b), a.IsDisjointWith(c))
	// Output: true false
}

func ExampleHashSet_All() {
	ss := hash_set_ts.FromSlice([]int{3, 1, 2})
	items := make([]int, 0)
	for v := range ss.All() { // snapshot iterator: safe under mutation
		items = append(items, v)
	}
	slices.Sort(items)
	fmt.Println(items)
	// Output: [1 2 3]
}

func ExampleHashSet_Walk() {
	ss := hash_set_ts.FromSlice([]int{3, 1, 2})
	items := make([]int, 0)
	complete := ss.Walk(func(_ int, v int) bool { // callback must NOT call back in
		items = append(items, v)
		return true
	})
	slices.Sort(items)
	fmt.Println(complete, items)
	// Output: true [1 2 3]
}

func ExampleHashSet_MarshalJSON() {
	ss := hash_set_ts.FromSlice([]int{2, 1})
	data, err := json.Marshal(ss)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var items []int
	if err := json.Unmarshal(data, &items); err != nil {
		fmt.Println("error:", err)
		return
	}
	slices.Sort(items) // slot order varies; sort when it matters
	fmt.Println(items)
	// Output: [1 2]
}

func ExampleHashSet_UnmarshalJSON() {
	// Unmarshal replaces the contents under one write lock and keeps the
	// equality/hash functions.
	ss := hash_set_ts.NewHashSet[string]()
	if err := ss.UnmarshalJSON([]byte(`["alpha","beta"]`)); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(ss.Len(), ss.Contains("alpha"))
	// Output: 2 true
}

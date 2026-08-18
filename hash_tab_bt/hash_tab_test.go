package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pschlump/HashStr"
	"github.com/pschlump/pluto/comparable"
)

// TestData is an interface matching data type for the elements that
// supports the Comparable interface.  This means that it has a Compare
// function.
type TestData struct {
	S string
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestData)(nil)
var _ Hashable = (*TestData)(nil)
var _ comparable.Equality = (*TestData)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa TestData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestData); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestData); ok {
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

// IsEqual implements the IsEqual function to satisfy the interface requirements.
func (aa TestData) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(TestData); ok {
		return aa.S == bb.S
	} else if bb, ok := x.(*TestData); ok {
		return aa.S == bb.S
	}
	panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
}

// HashKey implements the Hashable interface.
func (aa TestData) HashKey(x any) (rv int) {
	if v, ok := x.(*TestData); ok {
		return HashStr.HashStr([]byte(v.S))
	}
	if v, ok := x.(TestData); ok {
		return HashStr.HashStr([]byte(v.S))
	}
	return
}

func TestHashFunction(t *testing.T) {
	a := hash(&TestData{S: fmt.Sprintf("%4d", 8)})
	b := hash(TestData{S: fmt.Sprintf("%4d", 8)})
	if a != b {
		t.Errorf("pointer and value receiver should hash the same, got %d and %d", a, b)
	}
}

func TestNewHashTabPanicsOnSmallSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("Expected NewHashTab to panic for n < 5, it did not")
		}
	}()
	NewHashTab[TestData](4)
}

func TestHashTabOperations(t *testing.T) {
	ht := NewHashTab[TestData](7)

	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash table after creation.")
	}

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Re-insert the same values: they replace, they do not add.
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40 after re-insert, got %d", ht.Len())
	}

	if ht.IsEmpty() {
		t.Errorf("Expected to not be empty hash table, failed.")
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Length())
	}

	// Search - find
	it := ht.Search(&TestData{S: "   8"})
	if it == nil {
		t.Fatalf("Expected to find it, did not")
	}
	if !ht.ItemExists(&TestData{S: "   8"}) {
		t.Errorf("Expected ItemExists to be true, got false")
	}
	if ht.ItemExists(&TestData{S: "no-such-key"}) {
		t.Errorf("Expected ItemExists to be false, got true")
	}

	// Delete
	found := ht.Delete(it)
	if !found {
		t.Errorf("Expected to delete it, did not")
	}
	if ht.Len() != 39 {
		t.Errorf("Expected length of 39, got %d", ht.Len())
	}

	// Delete the same element again must report not found.
	if ht.Delete(it) {
		t.Errorf("Expected second delete of same element to fail")
	}
	if ht.Len() != 39 {
		t.Errorf("Expected length of 39 after failed delete, got %d", ht.Len())
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "   8"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

	// Insert
	ht.Insert(&TestData{S: "abcd"})
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Length())
	}

	// Search - find
	it = ht.Search(&TestData{S: "abcd"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}

	// Truncate
	ht.Truncate()
	if ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Length())
	}
	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash table after Truncate.")
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "abcd"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}
}

func TestEmptyTable(t *testing.T) {
	ht := NewHashTab[TestData](7)

	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash table after creation.")
	}
	if ht.Len() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "x"}); it != nil {
		t.Errorf("Expected Search on empty table to return nil")
	}
	if ht.ItemExists(&TestData{S: "x"}) {
		t.Errorf("Expected ItemExists on empty table to return false")
	}
	if ht.Delete(&TestData{S: "x"}) {
		t.Errorf("Expected Delete on empty table to return false")
	}
	if ht.Delete(nil) {
		t.Errorf("Expected Delete of nil to return false")
	}

	n := 0
	ht.WalkFunc(func(a *TestData) { n++ })
	if n != 0 {
		t.Errorf("Expected WalkFunc on empty table to visit 0 elements, got %d", n)
	}
	for range ht.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected All on empty table to yield 0 elements, got %d", n)
	}
}

func TestWalkFuncAndAll(t *testing.T) {
	ht := NewHashTab[TestData](11)
	const total = 500
	want := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		s := fmt.Sprintf("value-%4d", i)
		want[s] = true
		ht.Insert(&TestData{S: s})
	}
	if ht.Len() != total {
		t.Fatalf("Expected length of %d, got %d", total, ht.Len())
	}

	// WalkFunc must visit every element exactly once, including elements
	// in buckets with an index >= length (regression test for the old
	// loop bound of i < tt.length).
	seen := make(map[string]bool, total)
	ht.WalkFunc(func(a *TestData) {
		if seen[a.S] {
			t.Errorf("WalkFunc visited %q twice", a.S)
		}
		seen[a.S] = true
	})
	if len(seen) != total {
		t.Errorf("Expected WalkFunc to visit %d elements, got %d", total, len(seen))
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("WalkFunc did not visit %q", k)
		}
	}

	// Walk must visit every element exactly once.
	count := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		count++
		return true
	}, nil)
	if count != total {
		t.Errorf("Expected Walk to visit %d elements, got %d", total, count)
	}

	// All (range-over-func) must yield every element exactly once.
	seen = make(map[string]bool, total)
	for item := range ht.All() {
		if seen[item.S] {
			t.Errorf("All yielded %q twice", item.S)
		}
		seen[item.S] = true
	}
	if len(seen) != total {
		t.Errorf("Expected All to yield %d elements, got %d", total, len(seen))
	}

	// All must stop when the loop body breaks.
	n := 0
	for range ht.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break from All after 1 element, got %d", n)
	}
}

func TestDump(t *testing.T) {
	ht := NewHashTab[TestData](7)
	var sb strings.Builder
	ht.Dump(&sb)
	if !strings.Contains(sb.String(), "Elements: 0") {
		t.Errorf("Expected Dump of empty table to report 0 elements, got %q", sb.String())
	}
	ht.Insert(&TestData{S: "abcd"})
	sb.Reset()
	ht.Dump(&sb)
	if !strings.Contains(sb.String(), "Elements: 1") {
		t.Errorf("Expected Dump to report 1 element, got %q", sb.String())
	}
}

func TestNlAPI(t *testing.T) {
	ht := NewHashTab[TestData](7)
	ht.Insert(&TestData{S: "a"})

	// The lock methods are no-ops in this non-thread-safe variant; they
	// exist for API compatibility with the hash_tab_bt_ts package, as do
	// the Nl-prefixed methods.
	ht.ReadLock()
	it := ht.NlSearch(&TestData{S: "a"})
	ht.ReadUnlock()
	if it == nil {
		t.Errorf("Expected NlSearch to find the item")
	}

	ht.WriteLock()
	if !ht.NlDelete(&TestData{S: "a"}) {
		t.Errorf("Expected NlDelete to remove the item")
	}
	ht.WriteUnlock()
	if ht.Len() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}
}

var benchmarkSink *TestData

func BenchmarkInsert(b *testing.B) {
	ht := NewHashTab[TestData](97)
	items := make([]TestData, b.N)
	for i := range items {
		items[i] = TestData{S: fmt.Sprintf("key-%d", i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Insert(&items[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	ht := NewHashTab[TestData](97)
	const n = 10000
	items := make([]TestData, n)
	for i := range items {
		items[i] = TestData{S: fmt.Sprintf("key-%d", i)}
		ht.Insert(&items[i])
	}
	find := TestData{S: "key-9999"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSink = ht.Search(&find)
	}
}

func BenchmarkDelete(b *testing.B) {
	ht := NewHashTab[TestData](97)
	items := make([]TestData, b.N)
	for i := range items {
		items[i] = TestData{S: fmt.Sprintf("key-%d", i)}
		ht.Insert(&items[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Delete(&items[i])
	}
}

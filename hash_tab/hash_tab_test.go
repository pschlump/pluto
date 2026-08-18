package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/pschlump/HashStr"
	"github.com/pschlump/pluto/comparable"
)

// TestData is an Inteface Matcing data type for the Nodes that supports the Comparable
// interface.  This means that it has a Compare fucntion.

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

func (aa TestData) IsEqual(x comparable.Equality) bool {
	switch bb := x.(type) {
	case TestData:
		return aa.S == bb.S
	case *TestData:
		return aa.S == bb.S
	default:
		panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
	}
}

func (aa TestData) HashKey(x interface{}) (rv int) {
	if v, ok := x.(*TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		return
	}
	if v, ok := x.(TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		return
	}
	return
}

func TestTest(t *testing.T) {

	ht := NewHashTab[TestData](7)

	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after decleration, failed to get one.")
	}

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	if db8 {
		ht.Dump(os.Stdout)
	}

	// Check setup of hash tab
	if ht.IsEmpty() {
		t.Errorf("Expected to not be empty hash-tab, failed.")
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Search - find
	it := ht.Search(&TestData{S: "   8"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}

	// Delete
	found := ht.Delete(it) // func (tt *HashTab[T]) Delete(find *T) (found bool) {
	if !found {
		t.Errorf("Expected to delete it, did not")
	}
	// Len
	if ht.Len() != 39 {
		t.Errorf("Expected length of 39, got %d", ht.Len())
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "   8"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

	// Insert
	ht.Insert(&TestData{S: "abcd"})

	// Len
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Search - find
	it = ht.Search(&TestData{S: "abcd"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}

	// Truncate
	ht.Truncate()

	// Len
	if ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "abcd"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

}

const db8 = false

// TestEmptyTableOps verifies that operations on an empty table behave sanely.
func TestEmptyTableOps(t *testing.T) {
	ht := NewHashTab[TestData](5)

	if !ht.IsEmpty() {
		t.Errorf("Expected empty table")
	}
	if ht.Len() != 0 || ht.Length() != 0 {
		t.Errorf("Expected length 0, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "nope"}); it != nil {
		t.Errorf("Expected Search on empty table to return nil")
	}
	if ht.Delete(&TestData{S: "nope"}) {
		t.Errorf("Expected Delete on empty table to return false")
	}
	n := 0
	for range ht.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected All on empty table to yield 0 items, got %d", n)
	}
}

// TestSingleElementDelete verifies deleting the only element in the table
// (and therefore the only element in its bucket).
func TestSingleElementDelete(t *testing.T) {
	ht := NewHashTab[TestData](5)
	x := TestData{S: "only"}
	ht.Insert(&x)

	if ht.Len() != 1 {
		t.Fatalf("Expected length 1, got %d", ht.Len())
	}
	if !ht.Delete(&x) {
		t.Fatalf("Expected Delete to succeed")
	}
	if ht.Len() != 0 || !ht.IsEmpty() {
		t.Errorf("Expected empty table after delete, got length %d", ht.Len())
	}
	if it := ht.Search(&x); it != nil {
		t.Errorf("Expected Search to return nil after delete")
	}
	if ht.Delete(&x) {
		t.Errorf("Expected second Delete to return false")
	}
}

// TestDeleteNotFound verifies Delete of a missing element on a non-empty table.
func TestDeleteNotFound(t *testing.T) {
	ht := NewHashTab[TestData](7)
	ht.Insert(&TestData{S: "a"})
	ht.Insert(&TestData{S: "b"})

	if ht.Delete(&TestData{S: "not-present"}) {
		t.Errorf("Expected Delete of missing element to return false")
	}
	if ht.Len() != 2 {
		t.Errorf("Expected length unchanged at 2, got %d", ht.Len())
	}
}

// TestDuplicates verifies that duplicate inserts stack and each Delete
// removes exactly one copy.
func TestDuplicates(t *testing.T) {
	ht := NewHashTab[TestData](5)
	dup := TestData{S: "dup"}

	ht.Insert(&dup)
	ht.Insert(&dup)
	if ht.Len() != 2 {
		t.Fatalf("Expected length 2 after duplicate inserts, got %d", ht.Len())
	}

	if !ht.Delete(&dup) {
		t.Fatalf("Expected first Delete to succeed")
	}
	if ht.Len() != 1 {
		t.Errorf("Expected length 1 after first delete, got %d", ht.Len())
	}
	if it := ht.Search(&dup); it == nil {
		t.Errorf("Expected remaining duplicate to still be found")
	}

	if !ht.Delete(&dup) {
		t.Fatalf("Expected second Delete to succeed")
	}
	if ht.Len() != 0 {
		t.Errorf("Expected length 0 after second delete, got %d", ht.Len())
	}
	if it := ht.Search(&dup); it != nil {
		t.Errorf("Expected no match after both duplicates deleted")
	}
}

// TestTruncateReuse verifies the table is usable after Truncate.
func TestTruncateReuse(t *testing.T) {
	ht := NewHashTab[TestData](7)
	for i := 0; i < 20; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after Truncate, got length %d", ht.Len())
	}

	ht.Insert(&TestData{S: "back"})
	if ht.Len() != 1 {
		t.Errorf("Expected length 1 after re-insert, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "back"}); it == nil {
		t.Errorf("Expected to find re-inserted element")
	}
}

// TestAllIterator verifies the range-over-func iterator visits every element
// exactly once, and honors early termination.
func TestAllIterator(t *testing.T) {
	ht := NewHashTab[TestData](7)
	const n = 40
	seen := make(map[string]int, n)
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("%4d", i)
		seen[s] = 0
		ht.Insert(&TestData{S: s})
	}

	count := 0
	for _, v := range ht.All() {
		count++
		if _, ok := seen[v.S]; !ok {
			t.Errorf("Iterator yielded unexpected element %q", v.S)
		}
		seen[v.S]++
	}
	if count != n {
		t.Errorf("Expected iterator to yield %d elements, got %d", n, count)
	}
	for s, c := range seen {
		if c != 1 {
			t.Errorf("Expected element %q yielded once, got %d", s, c)
		}
	}

	// Early break must stop iteration.
	count = 0
	for range ht.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early break after 1 element, got %d", count)
	}
}

// TestDump exercises the Dump diagnostic output.
func TestDump(t *testing.T) {
	ht := NewHashTab[TestData](5)
	ht.Insert(&TestData{S: "x"})
	var buf strings.Builder
	ht.Dump(&buf)
	if !strings.Contains(buf.String(), "Elements: 1") {
		t.Errorf("Expected Dump to report 1 element, got %q", buf.String())
	}
}

func benchmarkItems(n int) []TestData {
	items := make([]TestData, n)
	for i := range items {
		items[i] = TestData{S: fmt.Sprintf("%8d", i)}
	}
	return items
}

func BenchmarkInsert(b *testing.B) {
	ht := NewHashTab[TestData](101)
	items := benchmarkItems(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Insert(&items[i%1024])
	}
}

func BenchmarkSearch(b *testing.B) {
	ht := NewHashTab[TestData](101)
	items := benchmarkItems(1024)
	for i := range items {
		ht.Insert(&items[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Search(&items[i%1024])
	}
}

func BenchmarkDelete(b *testing.B) {
	ht := NewHashTab[TestData](101)
	items := benchmarkItems(1024)
	for i := range items {
		ht.Insert(&items[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Re-insert after delete to keep the table population steady.
		ht.Delete(&items[i%1024])
		ht.Insert(&items[i%1024])
	}
}

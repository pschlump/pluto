package hash_grow_ts

// Thread-safe version of hash_grow table.

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/HashStr"
	"github.com/pschlump/pluto/comparable"
)

// TestData is an interface matching data type for the table that supports the
// Comparable interface.  This means that it has a Compare function.
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
	if bb, ok := x.(TestData); ok {
		return aa.S == bb.S
	} else if bb, ok := x.(*TestData); ok {
		return aa.S == bb.S
	}
	panic(fmt.Sprintf("Passed invalid type %T to a IsEqual function.", x))
}

func (aa TestData) HashKey(x any) (rv int) {
	if v, ok := x.(*TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		if rv == 0 {
			rv = 1
		}
		return
	}
	if v, ok := x.(TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		if rv == 0 {
			rv = 1
		}
		return
	}
	return
}

func TestHashFunction(t *testing.T) {
	a := hash(&TestData{S: fmt.Sprintf("%4d", 8)})
	b := hash(TestData{S: fmt.Sprintf("%4d", 8)})
	if a != b {
		t.Errorf("Boom")
	}
	if a <= 0 {
		t.Errorf("hash must return a positive value, got %d", a)
	}
}

func TestTest1(t *testing.T) {

	ht := NewHashTab[TestData](7, 0)

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
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
	found := ht.Delete(it)
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
	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after Truncate.")
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "abcd"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

}

func TestTest2(t *testing.T) {

	ht := NewHashTab[TestData](7, 0)

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Delete
	it := ht.Search(&TestData{S: "  13"})
	found := ht.Delete(it)
	if !found {
		t.Errorf("Expected to delete it, did not")
	}

	it = ht.Search(&TestData{S: "   6"})
	found = ht.Delete(it)
	if !found {
		t.Errorf("Expected to delete it, did not")
	}

	// Search - find
	it = ht.Search(&TestData{S: "  38"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}
}

func TestTestPrint(t *testing.T) {
	expect := `{   3}
{  10}
{  26}
{  39}
{   2}
{  31}
{  36}
{  35}
{   4}
{  21}
{  34}
{  11}
{  17}
{  20}
{  30}
{  33}
{   9}
{  37}
{  22}
{   0}
{  19}
{  12}
{   5}
{  27}
{  16}
{  32}
{  23}
{  15}
{   7}
{  29}
{  14}
{  13}
{  18}
{   6}
{  25}
{  28}
{   1}
{  38}
{  24}
{   8}
`
	ht := NewHashTab[TestData](7, 0)

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	buf := new(bytes.Buffer)
	ht.Print(buf)
	got := buf.String()
	if got != expect {
		t.Errorf("Expected ->%s<- got ->%s<-\n", expect, got)
	}

}

// TestEmptyTable verifies operations on an empty table.
func TestEmptyTable(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after declaration, failed to get one.")
	}
	if ht.Len() != 0 || ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "nope"}); it != nil {
		t.Errorf("Expected nil from Search on empty table, got %v", it)
	}
	if ht.Delete(&TestData{S: "nope"}) {
		t.Errorf("Expected false from Delete on empty table")
	}
	if ht.Delete(nil) {
		t.Errorf("Expected false from Delete(nil)")
	}
	n := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		n++
		return true
	}, nil)
	if n != 0 {
		t.Errorf("Expected Walk on empty table to call function 0 times, got %d", n)
	}
	buf := new(bytes.Buffer)
	ht.Print(buf)
	if buf.Len() != 0 {
		t.Errorf("Expected empty output from Print on empty table, got %q", buf.String())
	}
}

// TestNewHashTabPanics verifies that an initial size below 5 panics.
func TestNewHashTabPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("Expected NewHashTab with n < 5 to panic")
		}
	}()
	NewHashTab[TestData](4, 0)
}

// TestTruncateReuse verifies that a table is fully reusable after Truncate
// and that Truncate clears all internal state (no stale probe markers).
func TestTruncateReuse(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 100; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after Truncate, got length %d", ht.Len())
	}
	for i := 0; i < 100; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 100 {
		t.Errorf("Expected length of 100, got %d", ht.Len())
	}
	for i := 0; i < 100; i++ {
		if it := ht.Search(&TestData{S: fmt.Sprintf("%4d", i)}); it == nil {
			t.Errorf("Expected to find %4d after Truncate+re-insert, did not", i)
		}
	}
}

// TestOracleVsMap runs a long sequence of pseudo-random Insert/Delete/Search
// operations and compares the results against a map oracle.  This exercises
// the collision, growth and backward-shift-delete paths.
func TestOracleVsMap(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ht := NewHashTab[TestData](7, 0)
	oracle := make(map[string]bool)

	key := func(i int) string { return fmt.Sprintf("%6d", i) }

	for op := 0; op < 2500; op++ {
		k := key(rng.Intn(400))
		switch rng.Intn(3) {
		case 0, 1: // insert
			ht.Insert(&TestData{S: k})
			oracle[k] = true
		case 2: // delete
			want := oracle[k]
			got := ht.Delete(&TestData{S: k})
			if got != want {
				t.Fatalf("op %d: Delete(%q) = %v, oracle says %v", op, k, got, want)
			}
			delete(oracle, k)
		}
		if ht.Len() != len(oracle) {
			t.Fatalf("op %d: Len() = %d, oracle says %d", op, ht.Len(), len(oracle))
		}
	}

	// Every key in the oracle must be found; a sample of missing keys must not.
	for k := range oracle {
		if it := ht.Search(&TestData{S: k}); it == nil {
			t.Errorf("Expected to find %q, did not", k)
		}
	}
	for i := 400; i < 600; i++ {
		if it := ht.Search(&TestData{S: key(i)}); it != nil {
			t.Errorf("Expected to NOT find %q, did", key(i))
		}
	}
}

// TestInsertManyThenDeleteAll inserts 2500 values and then searches for and
// deletes every one of them, verifying each step.
func TestInsertManyThenDeleteAll(t *testing.T) {
	const N = 2500
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < N; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%6d", i)})
	}
	if ht.Len() != N {
		t.Fatalf("Expected length of %d, got %d", N, ht.Len())
	}
	// Re-inserting the same values must not grow the element count.
	for i := 0; i < N; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%6d", i)})
	}
	if ht.Len() != N {
		t.Fatalf("Expected length of %d after duplicate insert, got %d", N, ht.Len())
	}
	for i := 0; i < N; i++ {
		if it := ht.Search(&TestData{S: fmt.Sprintf("%6d", i)}); it == nil {
			t.Fatalf("Expected to find %6d, did not", i)
		}
	}
	for i := 0; i < N; i += 2 {
		if !ht.Delete(&TestData{S: fmt.Sprintf("%6d", i)}) {
			t.Fatalf("Expected to delete %6d, did not", i)
		}
	}
	if ht.Len() != N/2 {
		t.Fatalf("Expected length of %d, got %d", N/2, ht.Len())
	}
	// All the odd values must still be findable after half the table was deleted.
	for i := 1; i < N; i += 2 {
		if it := ht.Search(&TestData{S: fmt.Sprintf("%6d", i)}); it == nil {
			t.Fatalf("Expected to find %6d after deletes, did not", i)
		}
	}
	for i := 1; i < N; i += 2 {
		if !ht.Delete(&TestData{S: fmt.Sprintf("%6d", i)}) {
			t.Fatalf("Expected to delete %6d, did not", i)
		}
	}
	if !ht.IsEmpty() {
		t.Fatalf("Expected empty table, got length %d", ht.Len())
	}
}

// TestIterators verifies the range-over-func iterators All and Values.
func TestIterators(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	seen := make(map[string]int)
	count := 0
	for pos, item := range ht.All() {
		if pos < 0 {
			t.Errorf("Invalid bucket position %d", pos)
		}
		seen[item.S]++
		count++
	}
	if count != 40 {
		t.Errorf("Expected All to yield 40 elements, got %d", count)
	}
	for i := 0; i < 40; i++ {
		k := fmt.Sprintf("%4d", i)
		if seen[k] != 1 {
			t.Errorf("Expected to see %q exactly once, saw %d", k, seen[k])
		}
	}

	count = 0
	for range ht.Values() {
		count++
	}
	if count != 40 {
		t.Errorf("Expected Values to yield 40 elements, got %d", count)
	}

	// Early termination of the range loop.
	count = 0
	for range ht.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early break after 1 element, got %d", count)
	}

	// Empty table yields nothing.
	ht.Truncate()
	for range ht.All() {
		t.Errorf("Expected no elements from All on empty table")
	}
	for range ht.Values() {
		t.Errorf("Expected no elements from Values on empty table")
	}
}

// TestWalk verifies the Walk callback API, including early termination.
func TestWalk(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	n := 0
	b := ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		n++
		return true
	}, nil)
	if !b || n != 40 {
		t.Errorf("Expected complete walk over 40 elements, got b=%v n=%d", b, n)
	}
	n = 0
	b = ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		n++
		return false
	}, nil)
	if b || n != 1 {
		t.Errorf("Expected walk to stop after 1 element, got b=%v n=%d", b, n)
	}
}

func BenchmarkInsert(b *testing.B) {
	ht := NewHashTab[TestData](1024, 0)
	items := make([]*TestData, b.N)
	for i := range items {
		items[i] = &TestData{S: fmt.Sprintf("%8d", i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Insert(items[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	ht := NewHashTab[TestData](1024, 0)
	const n = 1000
	for i := 0; i < n; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%8d", i)})
	}
	probe := &TestData{S: fmt.Sprintf("%8d", n/2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Search(probe)
	}
}

func BenchmarkDelete(b *testing.B) {
	ht := NewHashTab[TestData](1024, 0)
	const n = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ht.IsEmpty() {
			b.StopTimer()
			for j := 0; j < n; j++ {
				ht.Insert(&TestData{S: fmt.Sprintf("%8d", j)})
			}
			b.StartTimer()
		}
		ht.Delete(&TestData{S: fmt.Sprintf("%8d", i%n)})
	}
}

// TestConcurrentAccess runs concurrent inserts, searches and deletes on
// disjoint key ranges; run with -race to verify the locking.
func TestConcurrentAccess(t *testing.T) {
	ht := NewHashTab[TestData](64, 0)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			base := g * 1000
			for i := 0; i < 200; i++ {
				ht.Insert(&TestData{S: fmt.Sprintf("%8d", base+i)})
			}
			for i := 0; i < 200; i++ {
				if it := ht.Search(&TestData{S: fmt.Sprintf("%8d", base+i)}); it == nil {
					t.Errorf("goroutine %d: expected to find %d", g, base+i)
				}
			}
			for i := 0; i < 100; i++ {
				ht.Delete(&TestData{S: fmt.Sprintf("%8d", base+i)})
			}
			// Exercise the snapshot iterator concurrently with writers.
			for range ht.Values() {
			}
		}(g)
	}
	wg.Wait()
	if ht.Len() != 8*100 {
		t.Errorf("Expected length of %d, got %d", 8*100, ht.Len())
	}
}

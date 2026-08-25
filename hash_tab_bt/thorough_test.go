package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// negHashData is a test element type whose HashKey always returns a
// negative value, to exercise the negative-hash branch of bucketOf, and
// whose constant hash forces every element into a single bucket.
type negHashData struct {
	N int
}

var _ comparable.Comparable = (*negHashData)(nil)
var _ Hashable = (*negHashData)(nil)

func (aa negHashData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(negHashData); ok {
		if aa.N < bb.N {
			return -1
		} else if aa.N > bb.N {
			return 1
		}
	} else if bb, ok := x.(*negHashData); ok {
		if aa.N < bb.N {
			return -1
		} else if aa.N > bb.N {
			return 1
		}
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

func (aa negHashData) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(negHashData); ok {
		return aa.N == bb.N
	} else if bb, ok := x.(*negHashData); ok {
		return aa.N == bb.N
	}
	panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
}

// HashKey returns a constant negative hash so that every element lands in
// the same bucket and the h < 0 branch of bucketOf is taken.
func (aa negHashData) HashKey(x any) (rv int) {
	return -7
}

// stringerData is a test element type that implements fmt.Stringer but NOT
// Hashable, to exercise the fmt.Stringer branch of hash.
type stringerData struct {
	S string
}

var _ comparable.Comparable = (*stringerData)(nil)
var _ fmt.Stringer = (*stringerData)(nil)

func (aa stringerData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(stringerData); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*stringerData); ok {
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

func (aa stringerData) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(stringerData); ok {
		return aa.S == bb.S
	} else if bb, ok := x.(*stringerData); ok {
		return aa.S == bb.S
	}
	panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
}

func (aa stringerData) String() string {
	return aa.S
}

// plainData is a test element type that implements neither Hashable nor
// fmt.Stringer, to exercise the panic branch of hash.
type plainData struct {
	S string
}

var _ comparable.Comparable = (*plainData)(nil)

func (aa plainData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(plainData); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*plainData); ok {
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

func (aa plainData) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(plainData); ok {
		return aa.S == bb.S
	} else if bb, ok := x.(*plainData); ok {
		return aa.S == bb.S
	}
	panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
}

// TestHashKeyDerivation exercises every branch of the internal hash
// function: Hashable, string, fmt.Stringer, and the panic for types that
// implement neither.
func TestHashKeyDerivation(t *testing.T) {
	// string branch: deterministic and equal to the fmt.Stringer branch
	// for the same string.
	a := hash("some-string")
	b := hash("some-string")
	if a != b {
		t.Errorf("Expected hash of a string to be deterministic, got %d and %d", a, b)
	}
	c := hash(stringerData{S: "some-string"})
	if a != c {
		t.Errorf("Expected Stringer branch to hash the string, got %d for string and %d for Stringer", a, c)
	}
	d := hash(&stringerData{S: "some-string"})
	if a != d {
		t.Errorf("Expected *Stringer branch to hash the string, got %d for string and %d for *Stringer", a, d)
	}

	// Hashable branch takes precedence over fmt.Stringer.
	e := hash(&TestData{S: "some-string"})
	if e != hash(TestData{S: "some-string"}) {
		t.Errorf("Expected Hashable branch to be used for both pointer and value, got differing hashes")
	}
}

// TestHashPanicsOnUnhashableType verifies the documented panic when an item
// implements neither Hashable nor fmt.Stringer.
func TestHashPanicsOnUnhashableType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("Expected hash to panic for a type that is neither Hashable nor Stringer")
		}
	}()
	_ = hash(3.14159)
}

// TestInsertPanicsOnUnhashableItem verifies that Insert propagates the
// panic for an item that supplies no hash key.
func TestInsertPanicsOnUnhashableItem(t *testing.T) {
	ht := NewHashTab[plainData](7)
	defer func() {
		if recover() == nil {
			t.Errorf("Expected Insert to panic for an item that is neither Hashable nor Stringer")
		}
	}()
	ht.Insert(&plainData{S: "x"})
}

// TestStringerKeyEndToEnd inserts, searches, and deletes items whose hash
// key comes from the fmt.Stringer branch.
func TestStringerKeyEndToEnd(t *testing.T) {
	ht := NewHashTab[stringerData](7)
	for i := 0; i < 50; i++ {
		ht.Insert(&stringerData{S: fmt.Sprintf("s-%4d", i)})
	}
	if ht.Len() != 50 {
		t.Fatalf("Expected length of 50, got %d", ht.Len())
	}
	if it := ht.Search(&stringerData{S: "s-  17"}); it == nil {
		t.Errorf("Expected to find s-  17, did not")
	}
	if !ht.Delete(&stringerData{S: "s-  17"}) {
		t.Errorf("Expected to delete s-  17, did not")
	}
	if ht.Len() != 49 {
		t.Errorf("Expected length of 49, got %d", ht.Len())
	}
}

// TestNegativeHashAndSingleBucket forces all elements into one bucket via a
// constant negative hash, exercising the h < 0 branch of bucketOf and deep
// per-bucket trees.
func TestNegativeHashAndSingleBucket(t *testing.T) {
	ht := NewHashTab[negHashData](5)
	const total = 100
	for i := 0; i < total; i++ {
		ht.Insert(&negHashData{N: i})
	}
	if ht.Len() != total {
		t.Fatalf("Expected length of %d, got %d", total, ht.Len())
	}
	for i := 0; i < total; i++ {
		if it := ht.Search(&negHashData{N: i}); it == nil || it.N != i {
			t.Errorf("Expected to find %d, got %v", i, it)
		}
	}
	// Delete every other element.
	for i := 0; i < total; i += 2 {
		if !ht.Delete(&negHashData{N: i}) {
			t.Errorf("Expected to delete %d, did not", i)
		}
	}
	if ht.Len() != total/2 {
		t.Errorf("Expected length of %d, got %d", total/2, ht.Len())
	}
	for i := 0; i < total; i++ {
		it := ht.Search(&negHashData{N: i})
		if i%2 == 0 && it != nil {
			t.Errorf("Expected %d to be deleted, found %v", i, it)
		}
		if i%2 == 1 && (it == nil || it.N != i) {
			t.Errorf("Expected to find %d, got %v", i, it)
		}
	}
}

// TestSingleElementLifecycle covers the single-element edge cases: insert,
// search identity, duplicate-replace, delete, and re-insert.
func TestSingleElementLifecycle(t *testing.T) {
	ht := NewHashTab[TestData](7)

	p1 := &TestData{S: "only"}
	ht.Insert(p1)
	if ht.Len() != 1 {
		t.Fatalf("Expected length of 1, got %d", ht.Len())
	}
	got := ht.Search(&TestData{S: "only"})
	if got != p1 {
		t.Errorf("Expected Search to return the inserted pointer")
	}

	// A duplicate insert replaces the stored item and does not grow the
	// table.
	p2 := &TestData{S: "only"}
	ht.Insert(p2)
	if ht.Len() != 1 {
		t.Errorf("Expected length of 1 after duplicate insert, got %d", ht.Len())
	}
	got = ht.Search(&TestData{S: "only"})
	if got != p2 {
		t.Errorf("Expected Search to return the replacement pointer after duplicate insert")
	}

	if !ht.Delete(p2) {
		t.Errorf("Expected to delete the only element")
	}
	if ht.Len() != 0 || !ht.IsEmpty() {
		t.Errorf("Expected empty table after deleting the only element, len=%d", ht.Len())
	}
	if ht.Search(&TestData{S: "only"}) != nil {
		t.Errorf("Expected Search after delete to return nil")
	}

	// The table remains usable after being emptied.
	ht.Insert(&TestData{S: "again"})
	if ht.Len() != 1 {
		t.Errorf("Expected length of 1 after re-insert, got %d", ht.Len())
	}
}

// TestZeroValueReadOnly verifies that the read-only operations behave on a
// zero-value (not constructed) table.
func TestZeroValueReadOnly(t *testing.T) {
	var ht HashTab[TestData]
	if !ht.IsEmpty() {
		t.Errorf("Expected zero-value table to be empty")
	}
	if ht.Len() != 0 || ht.Length() != 0 {
		t.Errorf("Expected zero-value table length of 0, got %d/%d", ht.Len(), ht.Length())
	}
	if ht.Search(&TestData{S: "x"}) != nil {
		t.Errorf("Expected Search on zero-value table to return nil")
	}
	if ht.ItemExists(&TestData{S: "x"}) {
		t.Errorf("Expected ItemExists on zero-value table to return false")
	}
	if ht.Delete(&TestData{S: "x"}) {
		t.Errorf("Expected Delete on zero-value table to return false")
	}
	n := 0
	for range ht.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected All on zero-value table to yield 0 elements, got %d", n)
	}
	ht.WalkFunc(func(a *TestData) { n++ })
	if n != 0 {
		t.Errorf("Expected WalkFunc on zero-value table to visit 0 elements, got %d", n)
	}
	var sb strings.Builder
	ht.Dump(&sb)
	if !strings.Contains(sb.String(), "Elements: 0") {
		t.Errorf("Expected Dump of zero-value table to report 0 elements, got %q", sb.String())
	}
}

// TestTruncateThenReuse fills a table, truncates it, and fills it again to
// verify the table is fully reset and reusable.
func TestTruncateThenReuse(t *testing.T) {
	ht := NewHashTab[TestData](9)
	for i := 0; i < 100; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("t-%4d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after Truncate, len=%d", ht.Len())
	}
	n := 0
	for range ht.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected All after Truncate to yield 0 elements, got %d", n)
	}
	for i := 0; i < 100; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("u-%4d", i)})
	}
	if ht.Len() != 100 {
		t.Errorf("Expected length of 100 after refill, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "u-  42"}); it == nil {
		t.Errorf("Expected to find u-  42 after refill, did not")
	}
	if it := ht.Search(&TestData{S: "t-  42"}); it != nil {
		t.Errorf("Expected pre-Truncate data to be gone, found %v", it)
	}
}

// TestWalkEarlyTermination verifies that returning false from the Walk
// callback stops the walk within a bucket.  All elements are forced into a
// single bucket (constant hash) so the whole table is one tree; the walk
// must stop after the callback returns false.
func TestWalkEarlyTermination(t *testing.T) {
	ht := NewHashTab[negHashData](5)
	const total = 60
	for i := 0; i < total; i++ {
		ht.Insert(&negHashData{N: i})
	}
	count := 0
	ht.Walk(func(pos, depth int, data *negHashData, userData any) bool {
		count++
		return count < 5
	}, nil)
	if count != 5 {
		t.Errorf("Expected Walk callback to be invoked exactly 5 times, got %d", count)
	}
}

// TestWalkEarlyTerminationAcrossBuckets is a regression test: returning
// false from the Walk callback must stop the walk across ALL buckets, not
// just within the current bucket's tree.  Elements are spread over multiple
// buckets (normal string hashing), so a bug that restarts the walk on the
// next non-empty bucket would produce more than 5 invocations.
func TestWalkEarlyTerminationAcrossBuckets(t *testing.T) {
	ht := NewHashTab[TestData](7)
	const total = 60
	for i := 0; i < total; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("walk-%4d", i)})
	}
	count := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		count++
		return count < 5
	}, nil)
	if count != 5 {
		t.Errorf("Expected Walk to stop across buckets after 5 invocations, got %d", count)
	}

	// Sanity: a walk that never stops visits every element exactly once.
	count = 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		count++
		return true
	}, nil)
	if count != total {
		t.Errorf("Expected full Walk to visit %d elements, got %d", total, count)
	}
}

// TestWalkPassesUserData verifies that the userData argument is handed
// through to every callback invocation, along with plausible pos/depth
// values (pos starts at 0 and increments per element within a bucket).
func TestWalkPassesUserData(t *testing.T) {
	ht := NewHashTab[negHashData](5)
	const total = 10
	for i := 0; i < total; i++ {
		ht.Insert(&negHashData{N: i})
	}
	type ctx struct {
		seen int
	}
	c := &ctx{}
	lastPos := -1
	ht.Walk(func(pos, depth int, data *negHashData, userData any) bool {
		got, ok := userData.(*ctx)
		if !ok || got != c {
			t.Errorf("Expected userData to be passed through, got %v", userData)
			return false
		}
		got.seen++
		if pos != lastPos+1 {
			t.Errorf("Expected pos to increment by 1, got %d after %d", pos, lastPos)
		}
		lastPos = pos
		if depth < 0 {
			t.Errorf("Expected non-negative depth, got %d", depth)
		}
		return true
	}, c)
	if c.seen != total {
		t.Errorf("Expected Walk to visit %d elements, got %d", total, c.seen)
	}
}

// TestZeroValueInsertPanics documents that the zero value of HashTab is
// usable for read-only operations only: Insert divides by tt.size, which is
// 0 for an unconstructed table, so it panics.
func TestZeroValueInsertPanics(t *testing.T) {
	var ht HashTab[TestData]
	defer func() {
		if recover() == nil {
			t.Errorf("Expected Insert on a zero-value table to panic (modulo by zero)")
		}
	}()
	ht.Insert(&TestData{S: "x"})
}

// TestAllEarlyBreakAfterK verifies that breaking out of the range-over-func
// iterator after k elements yields exactly k elements.
func TestAllEarlyBreakAfterK(t *testing.T) {
	ht := NewHashTab[TestData](7)
	const total = 60
	for i := 0; i < total; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("b-%4d", i)})
	}
	const k = 10
	n := 0
	for range ht.All() {
		n++
		if n >= k {
			break
		}
	}
	if n != k {
		t.Errorf("Expected early break from All after %d elements, got %d", k, n)
	}
	// Iterating again after a break must still yield every element.
	n = 0
	for range ht.All() {
		n++
	}
	if n != total {
		t.Errorf("Expected full iteration after break to yield %d elements, got %d", total, n)
	}
}

// TestDumpNonEmptyBuckets verifies that Dump prints the per-bucket detail
// for a populated table.
func TestDumpNonEmptyBuckets(t *testing.T) {
	ht := NewHashTab[TestData](5)
	for i := 0; i < 30; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("d-%4d", i)})
	}
	var sb strings.Builder
	ht.Dump(&sb)
	out := sb.String()
	if !strings.Contains(out, "Elements: 30, mod size:5") {
		t.Errorf("Expected Dump header to report 30 elements and mod size 5, got %q", out)
	}
	if !strings.Contains(out, "bucket [") {
		t.Errorf("Expected Dump to print per-bucket detail, got %q", out)
	}
}

// TestRandomizedAgainstMap cross-checks the table against a map reference
// model over a long run of mixed insert/delete/search operations with a
// fixed seed.
func TestRandomizedAgainstMap(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ht := NewHashTab[TestData](17)
	model := make(map[string]*TestData)
	const keySpace = 200
	const ops = 4000

	key := func() string {
		return fmt.Sprintf("k-%4d", rng.Intn(keySpace))
	}

	for i := 0; i < ops; i++ {
		k := key()
		switch rng.Intn(100) {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
			10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
			20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
			30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
			40, 41, 42, 43, 44: // 45% insert
			p := &TestData{S: k}
			ht.Insert(p)
			model[k] = p
		case 45, 46, 47, 48, 49,
			50, 51, 52, 53, 54,
			55, 56, 57, 58, 59,
			60, 61, 62, 63, 64,
			65, 66, 67, 68, 69,
			70, 71, 72, 73, 74: // 30% delete
			_, want := model[k]
			got := ht.Delete(&TestData{S: k})
			if got != want {
				t.Fatalf("op %d: Delete(%q) = %v, model says %v", i, k, got, want)
			}
			delete(model, k)
		default: // 25% search
			want, ok := model[k]
			got := ht.Search(&TestData{S: k})
			if !ok {
				if got != nil {
					t.Fatalf("op %d: Search(%q) = %v, model says absent", i, k, got)
				}
			} else {
				if got == nil || got.S != want.S {
					t.Fatalf("op %d: Search(%q) = %v, model says %v", i, k, got, want)
				}
				if got != want {
					t.Fatalf("op %d: Search(%q) did not return the most recently inserted pointer", i, k)
				}
			}
			if ht.ItemExists(&TestData{S: k}) != ok {
				t.Fatalf("op %d: ItemExists(%q) disagrees with model", i, k)
			}
		}
		if ht.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, model has %d", i, ht.Len(), len(model))
		}
	}

	// Final invariant: the iterator yields exactly the model's key set.
	seen := make(map[string]bool, len(model))
	for item := range ht.All() {
		if seen[item.S] {
			t.Fatalf("All yielded %q twice", item.S)
		}
		seen[item.S] = true
		if _, ok := model[item.S]; !ok {
			t.Fatalf("All yielded %q which is not in the model", item.S)
		}
	}
	if len(seen) != len(model) {
		t.Fatalf("All yielded %d elements, model has %d", len(seen), len(model))
	}

	// Deleting everything in model order must empty the table.
	for k := range model {
		if !ht.Delete(&TestData{S: k}) {
			t.Fatalf("Expected final delete of %q to succeed", k)
		}
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after deleting all model keys, len=%d", ht.Len())
	}
}

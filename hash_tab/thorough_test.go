package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// StringerData implements comparable.Equality and fmt.Stringer, but NOT the
// Hashable interface, so the table falls back to hashing String().
type StringerData struct {
	S string
}

var _ comparable.Equality = (*StringerData)(nil)
var _ fmt.Stringer = (*StringerData)(nil)

func (aa StringerData) IsEqual(x comparable.Equality) bool {
	switch bb := x.(type) {
	case StringerData:
		return aa.S == bb.S
	case *StringerData:
		return aa.S == bb.S
	default:
		panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
	}
}

func (aa StringerData) String() string { return aa.S }

// PlainData implements only comparable.Equality; it is neither Hashable nor
// a fmt.Stringer, so hashing it must panic.
type PlainData struct {
	N int
}

var _ comparable.Equality = (*PlainData)(nil)

func (aa PlainData) IsEqual(x comparable.Equality) bool {
	switch bb := x.(type) {
	case PlainData:
		return aa.N == bb.N
	case *PlainData:
		return aa.N == bb.N
	default:
		panic(fmt.Sprintf("Passed invalid type %T to an IsEqual function.", x))
	}
}

// TestNewHashTabPanicsOnSmallN verifies the documented panic when the
// requested bucket count is below 5, and that the boundary value 5 works.
func TestNewHashTabPanicsOnSmallN(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 4} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("Expected NewHashTab(%d) to panic, it did not", n)
				}
			}()
			_ = NewHashTab[TestData](n)
		}()
	}

	ht := NewHashTab[TestData](5) // boundary: must not panic
	if !ht.IsEmpty() {
		t.Errorf("Expected NewHashTab(5) to produce an empty table")
	}
}

// TestHashStringBranch covers the raw-string branch of the internal hash
// function.  Through the public API hash always receives a *T, so this
// branch is exercised by calling hash directly.
func TestHashStringBranch(t *testing.T) {
	ht := NewHashTab[TestData](5)
	got := ht.hash("hello")

	f := fnv.New32a()
	_, _ = f.Write([]byte("hello"))
	want := int(f.Sum32())
	if got != want {
		t.Errorf("Expected hash of raw string to be FNV-32a (%d), got %d", want, got)
	}
}

// TestStringerFallback verifies a type that provides String() but no HashKey
// method is hashed via its String() value and supports full operation.
func TestStringerFallback(t *testing.T) {
	ht := NewHashTab[StringerData](7)

	if got := ht.hash(&StringerData{S: "abc"}); true {
		f := fnv.New32a()
		_, _ = f.Write([]byte("abc"))
		if want := int(f.Sum32()); got != want {
			t.Errorf("Expected Stringer fallback hash %d, got %d", want, got)
		}
	}

	for i := 0; i < 25; i++ {
		ht.Insert(&StringerData{S: fmt.Sprintf("s%02d", i)})
	}
	if ht.Len() != 25 {
		t.Fatalf("Expected length 25, got %d", ht.Len())
	}
	if it := ht.Search(&StringerData{S: "s07"}); it == nil || it.S != "s07" {
		t.Errorf("Expected to find s07, got %v", it)
	}
	if !ht.Delete(&StringerData{S: "s07"}) {
		t.Errorf("Expected Delete of s07 to succeed")
	}
	if it := ht.Search(&StringerData{S: "s07"}); it != nil {
		t.Errorf("Expected s07 to be gone after Delete")
	}
	if ht.Len() != 24 {
		t.Errorf("Expected length 24, got %d", ht.Len())
	}
}

// TestHashPanicsOnInvalidType verifies the documented panic when an element
// is neither Hashable, a string, nor a fmt.Stringer.
func TestHashPanicsOnInvalidType(t *testing.T) {
	ht := NewHashTab[PlainData](5)
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected Insert of non-Stringer/non-Hashable element to panic")
		} else if s, ok := r.(string); !ok || !strings.Contains(s, "needs to be Stringer or Hashable") {
			t.Errorf("Unexpected panic value: %v", r)
		}
	}()
	ht.Insert(&PlainData{N: 1})
}

// TestSearchReturnsNewestDuplicate verifies the documented stack-like
// behavior of buckets: a duplicate is inserted before the old one, so Search
// finds the most recently inserted copy, and deleting it un-hides the older
// copy.
func TestSearchReturnsNewestDuplicate(t *testing.T) {
	ht := NewHashTab[TestData](5)
	older := &TestData{S: "dup"}
	newer := &TestData{S: "dup"}

	ht.Insert(older)
	ht.Insert(newer)

	if got := ht.Search(&TestData{S: "dup"}); got != newer {
		t.Errorf("Expected Search to return the most recently inserted duplicate")
	}

	if !ht.Delete(&TestData{S: "dup"}) {
		t.Fatalf("Expected Delete to succeed")
	}
	if got := ht.Search(&TestData{S: "dup"}); got != older {
		t.Errorf("Expected older duplicate to be un-hidden after Delete")
	}
}

// TestAllIteratorIndices verifies the iterator index is a dense 0..n-1
// sequence regardless of bucket layout.
func TestAllIteratorIndices(t *testing.T) {
	ht := NewHashTab[TestData](11)
	const n = 50
	for i := 0; i < n; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	expect := 0
	for i, v := range ht.All() {
		if i != expect {
			t.Errorf("Expected iterator index %d, got %d", expect, i)
		}
		if v == nil {
			t.Errorf("Iterator yielded nil element at index %d", i)
		}
		expect++
	}
	if expect != n {
		t.Errorf("Expected %d iterations, got %d", n, expect)
	}
}

// TestDumpEmptyTable verifies Dump output on an empty table.
func TestDumpEmptyTable(t *testing.T) {
	ht := NewHashTab[TestData](5)
	var buf strings.Builder
	ht.Dump(&buf)
	if !strings.Contains(buf.String(), "Elements: 0") {
		t.Errorf("Expected Dump to report 0 elements, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "bucket [") {
		t.Errorf("Expected no bucket lines for an empty table, got %q", buf.String())
	}
}

// TestRandomizedModel cross-checks the table against a multiset reference
// model over hundreds of seeded, mixed Insert/Delete/Search operations.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20240825)) // fixed seed: deterministic
	ht := NewHashTab[TestData](7)             // small table forces collisions

	const keySpace = 30
	key := func(i int) string { return fmt.Sprintf("key-%02d", i) }

	model := make(map[string]int) // multiset of live elements
	total := 0

	for op := 0; op < 600; op++ {
		k := key(rng.Intn(keySpace))
		switch r := rng.Intn(100); {
		case r < 55: // Insert
			ht.Insert(&TestData{S: k})
			model[k]++
			total++
		case r < 85: // Delete
			got := ht.Delete(&TestData{S: k})
			want := model[k] > 0
			if got != want {
				t.Fatalf("op %d: Delete(%q) = %v, want %v", op, k, got, want)
			}
			if got {
				model[k]--
				total--
			}
		default: // Search
			it := ht.Search(&TestData{S: k})
			if model[k] > 0 {
				if it == nil || it.S != k {
					t.Fatalf("op %d: Search(%q) = %v, want a hit", op, k, it)
				}
			} else if it != nil {
				t.Fatalf("op %d: Search(%q) found %v, want a miss", op, k, it)
			}
		}

		if ht.Len() != total || ht.Length() != total {
			t.Fatalf("op %d: length = %d, model = %d", op, ht.Len(), total)
		}
		if ht.IsEmpty() != (total == 0) {
			t.Fatalf("op %d: IsEmpty = %v, model size = %d", op, ht.IsEmpty(), total)
		}
	}

	// Final invariant: full iteration must yield exactly the model multiset.
	seen := make(map[string]int)
	count := 0
	for _, v := range ht.All() {
		seen[v.S]++
		count++
	}
	if count != total {
		t.Errorf("Iterator yielded %d elements, model has %d", count, total)
	}
	for k, c := range model {
		if seen[k] != c {
			t.Errorf("Key %q: iterated %d copies, model has %d", k, seen[k], c)
		}
	}
	for k := range seen {
		if _, ok := model[k]; !ok {
			t.Errorf("Iterator yielded unexpected key %q", k)
		}
	}

	// Every key in the key space must Search consistently with the model.
	for i := 0; i < keySpace; i++ {
		k := key(i)
		it := ht.Search(&TestData{S: k})
		if (it != nil) != (model[k] > 0) {
			t.Errorf("Final Search(%q) inconsistent with model", k)
		}
	}

	// Truncate must bring the table back to the empty state.
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Errorf("Expected empty table after Truncate, got length %d", ht.Len())
	}
	for i := 0; i < keySpace; i++ {
		if it := ht.Search(&TestData{S: key(i)}); it != nil {
			t.Errorf("Expected no elements after Truncate, found %q", key(i))
		}
	}
}

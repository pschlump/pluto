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
	"github.com/pschlump/pluto/dll"
)

// StringerData is an element type that implements comparable.Equality and
// fmt.Stringer, but NOT the Hashable interface, so the table falls back to
// hashing its String() value.
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

func (aa StringerData) String() string {
	return "StringerData{" + aa.S + "}"
}

// expectPanic runs fx and verifies that it panics; the recovered value is
// returned for further inspection.
func expectPanic(t *testing.T, name string, fx func()) (rv interface{}) {
	t.Helper()
	defer func() {
		rv = recover()
		if rv == nil {
			t.Errorf("Expected %s to panic, but it did not", name)
		}
	}()
	fx()
	return nil
}

// bucketTotal sums the lengths of all buckets; used to check the structural
// invariant that the buckets together hold exactly Len() elements.
func bucketTotal[T comparable.Equality](ht *HashTab[T]) int {
	total := 0
	for _, b := range ht.buckets {
		total += b.Length()
	}
	return total
}

// TestNewHashTabSize verifies that a table with fewer than 5 buckets panics
// and that the minimum size of 5 is accepted.
func TestNewHashTabSize(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 4} {
		n := n
		expectPanic(t, fmt.Sprintf("NewHashTab(%d)", n), func() {
			NewHashTab[TestData](n)
		})
	}

	ht := NewHashTab[TestData](5)
	if ht == nil {
		t.Fatalf("Expected NewHashTab(5) to succeed")
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Errorf("Expected new table to be empty, got length %d", ht.Len())
	}
	if got := len(ht.buckets); got != 5 {
		t.Errorf("Expected 5 buckets, got %d", got)
	}
	for i, b := range ht.buckets {
		if b == nil {
			t.Errorf("Expected bucket %d to be initialized", i)
		}
	}
}

// TestDeleteFoundEmpty verifies DeleteFound on an empty table returns false.
func TestDeleteFoundEmpty(t *testing.T) {
	ht := NewHashTab[TestData](5)

	// Obtain a real element from a standalone list so the argument is a
	// valid, non-nil *dll.DllElement[TestData].
	d := dll.NewDll[TestData]()
	d.InsertBeforeHead(&TestData{S: "x"})
	el, pos := d.Search(&TestData{S: "x"})
	if el == nil || pos < 0 {
		t.Fatalf("Setup failed: could not create a DllElement")
	}

	if ht.DeleteFound(el) {
		t.Errorf("Expected DeleteFound on empty table to return false")
	}
	if ht.Len() != 0 {
		t.Errorf("Expected length to remain 0, got %d", ht.Len())
	}
}

// TestWalkEmpty verifies Walk on an empty table returns nil, -1 and never
// invokes the callback.
func TestWalkEmpty(t *testing.T) {
	ht := NewHashTab[TestData](5)
	called := false
	el, pos := ht.Walk(func(p int, data TestData, userData interface{}) bool {
		called = true
		return true
	}, nil)
	if el != nil || pos != -1 {
		t.Errorf("Expected Walk on empty table to return nil, -1; got %v, %d", el, pos)
	}
	if called {
		t.Errorf("Expected callback not to be called on empty table")
	}
}

// TestWalkUserData verifies that userData is passed through to the callback
// unchanged, and that the returned element matches the stopped-at element.
func TestWalkUserData(t *testing.T) {
	ht := NewHashTab[TestData](7)
	for i := 0; i < 10; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("w%02d", i)})
	}

	token := &struct{ name string }{name: "tok"}
	gotUserData := false
	el, pos := ht.Walk(func(p int, data TestData, userData interface{}) bool {
		if userData != token {
			t.Errorf("Expected userData to be passed through, got %v", userData)
		}
		gotUserData = true
		return data.S == "w05"
	}, token)
	if !gotUserData {
		t.Fatalf("Expected callback to run")
	}
	if el == nil || pos < 0 {
		t.Fatalf("Expected Walk to stop at w05, got %v, %d", el, pos)
	}
	if d := el.GetData(); d == nil || d.S != "w05" {
		t.Errorf("Expected returned element to hold w05, got %v", d)
	}

	// The located element must be removable via DeleteFound.
	before := ht.Len()
	if !ht.DeleteFound(el) {
		t.Errorf("Expected DeleteFound of Walk-located element to succeed")
	}
	if ht.Len() != before-1 {
		t.Errorf("Expected length %d after DeleteFound, got %d", before-1, ht.Len())
	}
	if ht.ItemExists(&TestData{S: "w05"}) {
		t.Errorf("Expected w05 to be gone after DeleteFound")
	}
}

// TestSearchReturnsStoredPointer verifies Search hands back the element
// wrapping the exact *T that was inserted (not a copy), and that for
// duplicate values the most recently inserted copy is found first (the
// bucket acts like a stack).
func TestSearchReturnsStoredPointer(t *testing.T) {
	ht := NewHashTab[TestData](5)

	first := &TestData{S: "same"}
	second := &TestData{S: "same"}
	ht.Insert(first)
	ht.Insert(second)

	el := ht.Search(&TestData{S: "same"})
	if el == nil {
		t.Fatalf("Expected to find element")
	}
	if el.GetData() != second {
		t.Errorf("Expected Search to return the most recently inserted duplicate")
	}

	// DeleteFound removes exactly the located copy; the older copy remains.
	if !ht.DeleteFound(el) {
		t.Fatalf("Expected DeleteFound to succeed")
	}
	el = ht.Search(&TestData{S: "same"})
	if el == nil {
		t.Fatalf("Expected remaining duplicate to be found")
	}
	if el.GetData() != first {
		t.Errorf("Expected Search to return the original insert after the newer copy was removed")
	}
}

// TestHashFunc exercises the internal hash dispatch directly: Hashable
// elements, plain strings, fmt.Stringer values, and the panic on an
// unsupported type.
func TestHashFunc(t *testing.T) {
	ht := NewHashTab[TestData](7)

	// Hashable: TestData supplies its own HashKey.
	h1 := ht.hash(&TestData{S: "alpha"})
	h2 := ht.hash(&TestData{S: "alpha"})
	if h1 != h2 {
		t.Errorf("Expected hash to be deterministic, got %d then %d", h1, h2)
	}

	// Plain string value.
	hs1 := ht.hash("beta")
	hs2 := ht.hash("beta")
	if hs1 != hs2 {
		t.Errorf("Expected string hash to be deterministic, got %d then %d", hs1, hs2)
	}

	// fmt.Stringer value (not Hashable).
	hstr1 := ht.hash(StringerData{S: "gamma"})
	hstr2 := ht.hash(StringerData{S: "gamma"})
	if hstr1 != hstr2 {
		t.Errorf("Expected Stringer hash to be deterministic, got %d then %d", hstr1, hstr2)
	}

	// Unsupported type panics.
	rv := expectPanic(t, "hash(unsupported type)", func() {
		ht.hash(42)
	})
	if msg, ok := rv.(string); ok {
		if !strings.Contains(msg, "Invalid type") {
			t.Errorf("Expected panic message to mention invalid type, got %q", msg)
		}
	} else {
		t.Errorf("Expected string panic value, got %v (%T)", rv, rv)
	}
}

// TestStringerElements verifies a table works end-to-end with an element
// type that is hashed via its String() method (no Hashable implementation).
func TestStringerElements(t *testing.T) {
	ht := NewHashTab[StringerData](7)

	for i := 0; i < 25; i++ {
		ht.Insert(&StringerData{S: fmt.Sprintf("s%02d", i)})
	}
	if ht.Len() != 25 {
		t.Fatalf("Expected length 25, got %d", ht.Len())
	}
	if bucketTotal(ht) != ht.Len() {
		t.Errorf("Invariant broken: buckets hold %d, Len() = %d", bucketTotal(ht), ht.Len())
	}

	find := &StringerData{S: "s11"}
	if !ht.ItemExists(find) {
		t.Errorf("Expected to find s11")
	}
	if el := ht.Search(find); el == nil {
		t.Errorf("Expected Search to locate s11")
	} else if el.GetData().S != "s11" {
		t.Errorf("Expected Search to return s11, got %q", el.GetData().S)
	}
	if !ht.Delete(find) {
		t.Errorf("Expected Delete of s11 to succeed")
	}
	if ht.ItemExists(find) {
		t.Errorf("Expected s11 to be gone after Delete")
	}
	if ht.Len() != 24 || bucketTotal(ht) != 24 {
		t.Errorf("Expected length 24, got Len() = %d, buckets = %d", ht.Len(), bucketTotal(ht))
	}
}

// TestAllIteratorIndices verifies the iterator index runs 0..n-1 in
// sequence, once per element.
func TestAllIteratorIndices(t *testing.T) {
	ht := NewHashTab[TestData](11)
	const n = 50
	for i := 0; i < n; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("i%03d", i)})
	}

	expect := 0
	for i := range ht.All() {
		if i != expect {
			t.Errorf("Expected iterator index %d, got %d", expect, i)
		}
		expect++
	}
	if expect != n {
		t.Errorf("Expected %d iterations, got %d", n, expect)
	}

	// Early break after several elements must not corrupt the table.
	count := 0
	for range ht.All() {
		count++
		if count == 7 {
			break
		}
	}
	if count != 7 {
		t.Errorf("Expected early break at 7, got %d", count)
	}
	if ht.Len() != n || bucketTotal(ht) != n {
		t.Errorf("Expected table intact after early break, Len() = %d, buckets = %d", ht.Len(), bucketTotal(ht))
	}
}

// TestDumpEmpty verifies Dump output for an empty table.
func TestDumpEmpty(t *testing.T) {
	ht := NewHashTab[TestData](5)
	var buf strings.Builder
	ht.Dump(&buf)
	out := buf.String()
	if !strings.Contains(out, "Elements: 0") {
		t.Errorf("Expected Dump to report 0 elements, got %q", out)
	}
	if strings.Contains(out, "bucket [") {
		t.Errorf("Expected no bucket lines for an empty table, got %q", out)
	}
}

// TestRandomizedAgainstModel performs hundreds of mixed operations with a
// fixed seed and cross-checks the table against a map-based reference model
// (a multiset, since duplicate values are permitted).
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ht := NewHashTab[TestData](13)
	model := make(map[string]int) // value -> number of copies in table

	key := func() string {
		return fmt.Sprintf("k%03d", rng.Intn(60))
	}
	modelLen := func() (n int) {
		for _, c := range model {
			n += c
		}
		return
	}
	checkInvariant := func(op string) {
		t.Helper()
		want := modelLen()
		if ht.Len() != want {
			t.Fatalf("After %s: expected length %d, got %d", op, want, ht.Len())
		}
		if bt := bucketTotal(ht); bt != want {
			t.Fatalf("After %s: bucket total %d does not match model length %d", op, bt, want)
		}
		if (want == 0) != ht.IsEmpty() {
			t.Fatalf("After %s: IsEmpty() = %v inconsistent with model length %d", op, ht.IsEmpty(), want)
		}
	}
	crossCheck := func() {
		t.Helper()
		got := make(map[string]int)
		idx := 0
		for i, v := range ht.All() {
			if i != idx {
				t.Fatalf("Iterator index out of sequence: expected %d, got %d", idx, i)
			}
			idx++
			got[v.S]++
		}
		if len(got) != len(model) {
			t.Fatalf("Model has %d distinct keys, table iteration found %d", len(model), len(got))
		}
		for s, c := range model {
			if got[s] != c {
				t.Fatalf("Key %q: model has %d copies, table has %d", s, c, got[s])
			}
		}
	}

	for op := 0; op < 2000; op++ {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // insert
			s := key()
			ht.Insert(&TestData{S: s})
			model[s]++
			checkInvariant("insert")
		case 4, 5, 6: // delete
			s := key()
			find := &TestData{S: s}
			deleted := ht.Delete(find)
			if model[s] > 0 {
				if !deleted {
					t.Fatalf("Expected Delete of %q to succeed (model has %d copies)", s, model[s])
				}
				model[s]--
				if model[s] == 0 {
					delete(model, s)
				}
			} else if deleted {
				t.Fatalf("Expected Delete of absent key %q to fail", s)
			}
			checkInvariant("delete")
		case 7, 8: // search / itemexists
			s := key()
			find := &TestData{S: s}
			_, present := model[s]
			if got := ht.ItemExists(find); got != present {
				t.Fatalf("ItemExists(%q) = %v, model says %v", s, got, present)
			}
			el := ht.Search(find)
			if present {
				if el == nil {
					t.Fatalf("Expected Search of %q to succeed", s)
				}
				if el.GetData().S != s {
					t.Fatalf("Search(%q) returned element holding %q", s, el.GetData().S)
				}
				// Occasionally remove via DeleteFound to exercise that path.
				if rng.Intn(3) == 0 {
					if !ht.DeleteFound(el) {
						t.Fatalf("Expected DeleteFound of %q to succeed", s)
					}
					model[s]--
					if model[s] == 0 {
						delete(model, s)
					}
					checkInvariant("deletefound")
				}
			} else if el != nil {
				t.Fatalf("Expected Search of absent key %q to return nil", s)
			}
		case 9: // truncate, then start refilling
			ht.Truncate()
			model = make(map[string]int)
			checkInvariant("truncate")
		}
		if op%200 == 199 {
			crossCheck()
		}
	}
	crossCheck()
	checkInvariant("final")
}

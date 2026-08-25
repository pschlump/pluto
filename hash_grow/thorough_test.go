package hash_grow

// Thorough tests for the hash_grow table: hash function variants, Dump,
// zero-value behavior, replacement semantics, wrap-around probe chains,
// growth, and a randomized cross-check against a map model.
// TestData is defined in hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// stringerOnly is hashed via the fmt.Stringer branch of hash().
type stringerOnly struct {
	s string
}

func (ss stringerOnly) String() string { return ss.s }

// hashKeyZero implements Hashable but always returns 0; hash() must map that
// to 1 (0 is reserved to mark empty slots).
type hashKeyZero struct{}

func (hashKeyZero) HashKey(x any) int { return 0 }

// hashKeyNegative implements Hashable and returns a negative key; hash() must
// take its absolute value.
type hashKeyNegative struct{}

func (hashKeyNegative) HashKey(x any) int { return -42 }

// TestHashVariants exercises every branch of the internal hash function:
// Hashable, string, fmt.Stringer, the 0->1 remap, negative->abs, and the
// panic on an unsupported type.
func TestHashVariants(t *testing.T) {
	// string branch
	if h := hash("hello"); h <= 0 {
		t.Errorf("hash(string) must be positive, got %d", h)
	}
	// fmt.Stringer branch
	a := hash(stringerOnly{s: "same"})
	b := hash(stringerOnly{s: "same"})
	if a != b || a <= 0 {
		t.Errorf("hash(Stringer) must be deterministic and positive, got %d, %d", a, b)
	}
	// Hashable returning 0 is remapped to 1.
	if h := hash(hashKeyZero{}); h != 1 {
		t.Errorf("hash(Hashable returning 0) must be 1, got %d", h)
	}
	// Hashable returning a negative value is absolutized.
	if h := hash(hashKeyNegative{}); h != 42 {
		t.Errorf("hash(Hashable returning -42) must be 42, got %d", h)
	}
	// Unsupported type panics.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected hash() of an int to panic")
			}
		}()
		hash(42)
	}()
}

// TestInsertNilPanics verifies that Insert(nil) panics (nil is not string,
// Stringer or Hashable).
func TestInsertNilPanics(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	defer func() {
		if recover() == nil {
			t.Errorf("Expected Insert(nil) to panic")
		}
	}()
	ht.Insert(nil)
}

// TestDumpOutput verifies Dump writes the header and one line per bucket.
func TestDumpOutput(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	// Empty table: header only.
	buf := new(bytes.Buffer)
	ht.Dump(buf)
	if !strings.HasPrefix(buf.String(), "Elements: 0, mod size:7\n") {
		t.Errorf("Unexpected Dump header for empty table: %q", buf.String())
	}
	if n := strings.Count(buf.String(), "\n"); n != 8 { // header + 7 buckets
		t.Errorf("Expected 8 lines from Dump of empty size-7 table, got %d", n)
	}

	// Non-empty table: every stored element appears in the dump.
	for i := 0; i < 20; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("d%03d", i)})
	}
	buf.Reset()
	ht.Dump(buf)
	out := buf.String()
	if !strings.HasPrefix(out, fmt.Sprintf("Elements: 20, mod size:%d\n", ht.size)) {
		t.Errorf("Unexpected Dump header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	for i := 0; i < 20; i++ {
		if !strings.Contains(out, fmt.Sprintf("d%03d", i)) {
			t.Errorf("Dump output missing element d%03d", i)
		}
	}
}

// TestZeroValue verifies that the zero value of HashTab is usable for
// read-only operations (constructor is required only before Insert).
func TestZeroValue(t *testing.T) {
	var ht HashTab[TestData]
	if !ht.IsEmpty() {
		t.Errorf("Zero value should be empty")
	}
	if ht.Len() != 0 || ht.Length() != 0 {
		t.Errorf("Zero value should have length 0, got %d", ht.Len())
	}
	if it := ht.Search(&TestData{S: "x"}); it != nil {
		t.Errorf("Search on zero value should return nil, got %v", it)
	}
	if ht.Delete(&TestData{S: "x"}) {
		t.Errorf("Delete on zero value should return false")
	}
	n := 0
	b := ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		n++
		return true
	}, nil)
	if !b || n != 0 {
		t.Errorf("Walk on zero value should complete without calls, got b=%v n=%d", b, n)
	}
	for range ht.All() {
		t.Errorf("All on zero value should yield nothing")
	}
	for range ht.Values() {
		t.Errorf("Values on zero value should yield nothing")
	}
	buf := new(bytes.Buffer)
	ht.Print(buf)
	if buf.Len() != 0 {
		t.Errorf("Print on zero value should produce no output, got %q", buf.String())
	}
	ht.Truncate() // must not panic on a zero value
}

// TestSearchDeleteNilAndMissing covers nil and not-found handling on a
// non-empty table.
func TestSearchDeleteNilAndMissing(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 10; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("m%03d", i)})
	}
	if it := ht.Search(nil); it != nil {
		t.Errorf("Search(nil) should return nil, got %v", it)
	}
	if ht.Delete(nil) {
		t.Errorf("Delete(nil) should return false")
	}
	if it := ht.Search(&TestData{S: "absent"}); it != nil {
		t.Errorf("Search of missing key should return nil, got %v", it)
	}
	if ht.Delete(&TestData{S: "absent"}) {
		t.Errorf("Delete of missing key should return false")
	}
	if ht.Len() != 10 {
		t.Errorf("Length should be unchanged at 10, got %d", ht.Len())
	}
}

// TestSingleElement covers the single-element table through its full life.
func TestSingleElement(t *testing.T) {
	ht := NewHashTab[TestData](5, 0)
	item := &TestData{S: "only"}
	ht.Insert(item)
	if ht.IsEmpty() || ht.Len() != 1 {
		t.Fatalf("Expected single element, got length %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "only"}); got != item {
		t.Errorf("Search should return the inserted pointer, got %v", got)
	}
	n := 0
	for _, v := range ht.All() {
		n++
		if v != item {
			t.Errorf("All should yield the inserted pointer, got %v", v)
		}
	}
	if n != 1 {
		t.Errorf("All should yield exactly 1 element, got %d", n)
	}
	if !ht.Delete(item) {
		t.Fatalf("Delete of the only element should succeed")
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Errorf("Table should be empty after deleting the only element")
	}
	if ht.Search(&TestData{S: "only"}) != nil {
		t.Errorf("Deleted element should not be found")
	}
	if ht.Delete(&TestData{S: "only"}) {
		t.Errorf("Second delete of the same element should return false")
	}
}

// TestInsertReplacesExisting verifies that re-inserting an equal key replaces
// the stored pointer and does not change the length, including when the key
// is not at its home bucket (collision path).
func TestInsertReplacesExisting(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	p1 := &TestData{S: "dup"}
	p2 := &TestData{S: "dup"}
	ht.Insert(p1)
	ht.Insert(p2)
	if ht.Len() != 1 {
		t.Fatalf("Duplicate insert should keep length 1, got %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "dup"}); got != p2 {
		t.Errorf("Search should return the replacement pointer, got %v", got)
	}

	// Force collisions so the replacement happens away from the home bucket.
	home := hash(p1) % ht.size
	used := map[string]bool{"dup": true}
	c1 := findKeyWithHome(t, ht.size, home, used)
	c2 := findKeyWithHome(t, ht.size, home, used)
	q1 := &TestData{S: c1}
	q2 := &TestData{S: c1}
	ht.Insert(q1)
	ht.Insert(&TestData{S: c2}) // pushes the chain along
	ht.Insert(q2)               // replace inside the probe chain
	if got := ht.Search(&TestData{S: c1}); got != q2 {
		t.Errorf("Replacement in collision chain should return new pointer, got %v", got)
	}
	if ht.Len() != 3 {
		t.Errorf("Expected length 3 after collision replacements, got %d", ht.Len())
	}
}

// findKeyWithHome returns a fresh key whose home bucket is `home` in a table
// of `size` buckets, recording it in `used` so keys are never repeated.
func findKeyWithHome(t *testing.T, size, home int, used map[string]bool) string {
	t.Helper()
	for i := 0; ; i++ {
		s := fmt.Sprintf("w%06d", i)
		if used[s] {
			continue
		}
		if hash(&TestData{S: s})%size == home {
			used[s] = true
			return s
		}
	}
}

// TestDeleteWrapAround builds a probe chain that wraps past the end of the
// table (buckets 4, 0, 1 in a size-5 table) and then deletes from the middle
// of the chain, exercising the backward-shift deletion across the wrap point
// and both arms of inProbeRange.
func TestDeleteWrapAround(t *testing.T) {
	const size = 5
	// Saturation 1.0 keeps the table from growing while length <= size.
	ht := NewHashTab[TestData](size, 1.0)
	used := map[string]bool{}
	d := findKeyWithHome(t, size, 0, used) // lands in bucket 0
	a := findKeyWithHome(t, size, 4, used) // lands in bucket 4
	b := findKeyWithHome(t, size, 4, used) // collides: 4 and 0 taken -> bucket 1

	ht.Insert(&TestData{S: d})
	ht.Insert(&TestData{S: a})
	ht.Insert(&TestData{S: b})
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	if ht.buckets[4] == nil || ht.buckets[0] == nil || ht.buckets[1] == nil {
		t.Fatalf("Expected chain across buckets 4,0,1; got %v", ht.buckets)
	}

	// Delete the middle of the chain (bucket 4).  The element in bucket 1
	// (home 4) must be shifted back into bucket 4 across the wrap point,
	// while the element in bucket 0 (home 0) must stay put.
	if !ht.Delete(&TestData{S: a}) {
		t.Fatalf("Expected to delete %q", a)
	}
	if ht.Len() != 2 {
		t.Errorf("Expected length 2 after delete, got %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: b}); got == nil {
		t.Errorf("Element %q shifted across wrap point must still be found", b)
	}
	if got := ht.Search(&TestData{S: d}); got == nil {
		t.Errorf("Element %q at its home bucket must still be found", d)
	}
	if ht.buckets[4] == nil || ht.buckets[4].S != b {
		t.Errorf("Expected %q to be shifted into bucket 4, got %v", b, ht.buckets[4])
	}

	// A missing key with home bucket 4 must probe across the wrap and stop
	// at the first truly empty slot without finding anything.
	missing := findKeyWithHome(t, size, 4, used)
	if it := ht.Search(&TestData{S: missing}); it != nil {
		t.Errorf("Missing key with wrapped probe should not be found, got %v", it)
	}
	if ht.Delete(&TestData{S: missing}) {
		t.Errorf("Delete of missing key with wrapped probe should return false")
	}

	// Delete the rest; the table must drain cleanly.
	if !ht.Delete(&TestData{S: b}) || !ht.Delete(&TestData{S: d}) {
		t.Errorf("Expected to delete remaining elements")
	}
	if !ht.IsEmpty() {
		t.Errorf("Table should be empty, got length %d", ht.Len())
	}
}

// TestInProbeRange directly covers both arms of the cyclic interval test
// used by backward-shift deletion.
func TestInProbeRange(t *testing.T) {
	tests := []struct {
		gap, hf, home int
		want          bool
	}{
		// gap < hf: in range iff gap < home <= hf.
		{2, 5, 3, true},
		{2, 5, 5, true},
		{2, 5, 2, false},
		{2, 5, 6, false},
		// gap > hf (wrap): in range iff home > gap or home <= hf.
		{5, 2, 7, true},
		{5, 2, 1, true},
		{5, 2, 4, false},
	}
	for _, tc := range tests {
		if got := inProbeRange(tc.gap, tc.hf, tc.home); got != tc.want {
			t.Errorf("inProbeRange(%d, %d, %d) = %v, want %v", tc.gap, tc.hf, tc.home, got, tc.want)
		}
	}
}

// TestGrowthDoubles verifies that the table doubles when the load factor
// passes the saturation threshold and that all elements survive re-hashing.
func TestGrowthDoubles(t *testing.T) {
	ht := NewHashTab[TestData](5, 0.5)
	if ht.size != 5 {
		t.Fatalf("Expected initial size 5, got %d", ht.size)
	}
	keys := []string{"g0", "g1", "g2", "g3", "g4", "g5"}
	for i, k := range keys {
		ht.Insert(&TestData{S: k})
		switch {
		case i < 2 && ht.size != 5: // 1/5, 2/5 stay below 0.5
			t.Fatalf("After %d inserts size should be 5, got %d", i+1, ht.size)
		case i == 2 && ht.size != 10: // 3/5 = 0.6 > 0.5 -> grow
			t.Fatalf("After 3 inserts size should be 10, got %d", ht.size)
		case i >= 3 && i < 5 && ht.size != 10:
			t.Fatalf("After %d inserts size should be 10, got %d", i+1, ht.size)
		case i == 5 && ht.size != 20: // 6/10 = 0.6 > 0.5 -> grow
			t.Fatalf("After 6 inserts size should be 20, got %d", ht.size)
		}
	}
	if ht.Len() != len(keys) {
		t.Errorf("Expected length %d, got %d", len(keys), ht.Len())
	}
	for _, k := range keys {
		if it := ht.Search(&TestData{S: k}); it == nil {
			t.Errorf("Expected to find %q after growth, did not", k)
		}
	}
	// A saturation of 0 selects the documented default of 0.5.
	ht2 := NewHashTab[TestData](5, 0)
	if ht2.saturationThreshold != 0.5 {
		t.Errorf("Expected default saturation 0.5, got %v", ht2.saturationThreshold)
	}
}

// TestValuesEarlyBreak covers early termination of the Values iterator and
// the bucket-position invariant of All.
func TestValuesEarlyBreak(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 30; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("v%03d", i)})
	}
	n := 0
	for range ht.Values() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected Values to stop after 1 element, got %d", n)
	}
	// Positions reported by All must be valid bucket indexes and match the
	// bucket the element is actually stored in.
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Errorf("All reported out-of-range position %d", pos)
		}
		if ht.buckets[pos] != item {
			t.Errorf("All reported position %d for %v but bucket holds %v", pos, item, ht.buckets[pos])
		}
	}
}

// TestWalkUserData verifies that Walk passes userData through, reports
// depth 0 and the real bucket position for each element.
func TestWalkUserData(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 25; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("u%03d", i)})
	}
	type marker struct{ name string }
	ud := &marker{name: "cookie"}
	n := 0
	b := ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		n++
		if depth != 0 {
			t.Errorf("Walk depth should be 0 for a flat table, got %d", depth)
		}
		m, ok := userData.(*marker)
		if !ok || m != ud {
			t.Errorf("Walk userData not passed through: %v", userData)
		}
		if pos < 0 || pos >= ht.size || ht.buckets[pos] != data {
			t.Errorf("Walk reported wrong position %d for %v", pos, data)
		}
		return true
	}, ud)
	if !b || n != 25 {
		t.Errorf("Expected complete walk of 25 elements, got b=%v n=%d", b, n)
	}
}

// TestRandomizedModel runs a fixed-seed pseudo-random mix of Insert, Delete
// and Search operations, cross-checking every result against a map model and
// periodically validating structural invariants.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	ht := NewHashTab[TestData](7, 0)
	model := make(map[string]int) // key -> number of times "present" (0/1, map presence)

	key := func(i int) string { return fmt.Sprintf("r%05d", i) }

	for op := 0; op < 4000; op++ {
		k := key(rng.Intn(300))
		switch rng.Intn(5) {
		case 0, 1, 2: // insert (weighted heavier)
			ht.Insert(&TestData{S: k})
			model[k] = 1
		case 3: // delete
			_, want := model[k]
			if got := ht.Delete(&TestData{S: k}); got != want {
				t.Fatalf("op %d: Delete(%q) = %v, model says %v", op, k, got, want)
			}
			delete(model, k)
		case 4: // search
			_, want := model[k]
			got := ht.Search(&TestData{S: k})
			if (got != nil) != want {
				t.Fatalf("op %d: Search(%q) found=%v, model says %v", op, k, got != nil, want)
			}
		}
		if ht.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, model has %d", op, ht.Len(), len(model))
		}
		if op%250 == 0 {
			checkTestDataInvariants(t, ht, model)
		}
	}
	checkTestDataInvariants(t, ht, model)

	// Drain the table through the iterators and confirm it empties cleanly.
	toDelete := make([]string, 0, len(model))
	for item := range ht.Values() {
		toDelete = append(toDelete, item.S)
	}
	for _, k := range toDelete {
		if !ht.Delete(&TestData{S: k}) {
			t.Fatalf("Expected to delete %q during drain", k)
		}
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Table should be empty after drain, got length %d", ht.Len())
	}
	checkTestDataInvariants(t, ht, map[string]int{})
}

// checkTestDataInvariants is checkInvariants specialized to TestData.
func checkTestDataInvariants(t *testing.T, ht *HashTab[TestData], model map[string]int) {
	t.Helper()
	if ht.Len() != len(model) {
		t.Fatalf("Len() = %d, model has %d", ht.Len(), len(model))
	}
	occupied := 0
	for i, v := range ht.buckets {
		if v == nil {
			if ht.originalHash[i] != 0 {
				t.Fatalf("bucket %d empty but originalHash = %d", i, ht.originalHash[i])
			}
			continue
		}
		occupied++
		if ht.originalHash[i] == 0 {
			t.Fatalf("bucket %d occupied but originalHash = 0", i)
		}
		if got := ht.Search(v); got == nil {
			t.Fatalf("element in bucket %d (%v) not found by Search", i, *v)
		}
	}
	if occupied != ht.Len() {
		t.Fatalf("occupied buckets = %d, Len() = %d", occupied, ht.Len())
	}
	seen := make(map[string]int)
	for _, item := range ht.All() {
		seen[item.S]++
	}
	nWalk := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		nWalk++
		return true
	}, nil)
	if nWalk != ht.Len() {
		t.Fatalf("Walk visited %d, Len() = %d", nWalk, ht.Len())
	}
	for k := range model {
		if seen[k] != 1 {
			t.Fatalf("key %q seen %d times by All, want 1", k, seen[k])
		}
		if got := ht.Search(&TestData{S: k}); got == nil {
			t.Fatalf("model key %q not found by Search", k)
		}
	}
}

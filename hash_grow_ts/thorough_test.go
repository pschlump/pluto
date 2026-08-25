package hash_grow_ts

// Additional thorough tests for the thread-safe growing hash table.

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// stringerKey implements fmt.Stringer but not Hashable, to exercise the
// fmt.Stringer branch of the internal hash function.
type stringerKey int

func (s stringerKey) String() string { return fmt.Sprintf("str-%d", int(s)) }

// zeroHashable implements Hashable but always returns 0, to exercise the
// "hash must never be 0" fixup in the internal hash function.
type zeroHashable struct{}

func (zeroHashable) HashKey(x any) int { return 0 }

// negHashable implements Hashable and returns a negative key, to exercise
// the absInt normalization path.
type negHashable struct{}

func (negHashable) HashKey(x any) int { return -42 }

// TestHashVariantPaths exercises the string, fmt.Stringer, zero-fixup and
// negative-fixup branches of the internal hash function.
func TestHashVariantPaths(t *testing.T) {
	h1 := hash("hello")
	h2 := hash("hello")
	if h1 != h2 {
		t.Errorf("hash of same string differs: %d vs %d", h1, h2)
	}
	if h1 <= 0 {
		t.Errorf("hash of string must be positive, got %d", h1)
	}

	// A fmt.Stringer must hash identically to its String() output.
	hs := hash(stringerKey(7))
	hstr := hash("str-7")
	if hs != hstr {
		t.Errorf("Stringer hash %d should equal string hash %d", hs, hstr)
	}

	// A Hashable returning 0 must be normalized to a non-zero value.
	if hz := hash(zeroHashable{}); hz <= 0 {
		t.Errorf("hash of zeroHashable must be positive, got %d", hz)
	}

	// A Hashable returning a negative key must be normalized to positive.
	if hn := hash(negHashable{}); hn != 42 {
		t.Errorf("hash of negHashable should be 42, got %d", hn)
	}
}

// TestHashInvalidTypePanics verifies that hash panics on a type that is
// neither a string, a fmt.Stringer nor Hashable.
func TestHashInvalidTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("Expected hash of an invalid type to panic")
		}
	}()
	hash(3.14)
}

// TestDump verifies that Dump writes a header and one line per bucket.
func TestDump(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 20; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	buf := new(bytes.Buffer)
	ht.Dump(buf)
	out := buf.String()
	if !strings.Contains(out, "Elements: 20") {
		t.Errorf("Expected Dump output to contain %q, got:\n%s", "Elements: 20", out)
	}
	lines := strings.Count(out, "bucket [")
	if lines != ht.size {
		t.Errorf("Expected one bucket line per slot (%d), got %d", ht.size, lines)
	}

	// Dump on an empty table still writes the header.
	ht.Truncate()
	buf.Reset()
	ht.Dump(buf)
	if !strings.Contains(buf.String(), "Elements: 0") {
		t.Errorf("Expected Dump of empty table to contain %q, got:\n%s", "Elements: 0", buf.String())
	}
}

// TestInProbeRange directly covers both the linear and the wrap-around
// branches of inProbeRange.
func TestInProbeRange(t *testing.T) {
	// gap < hf: home in (gap, hf] is in range.
	if !inProbeRange(2, 5, 3) {
		t.Errorf("inProbeRange(2,5,3) should be true")
	}
	if !inProbeRange(2, 5, 5) {
		t.Errorf("inProbeRange(2,5,5) should be true")
	}
	if inProbeRange(2, 5, 2) {
		t.Errorf("inProbeRange(2,5,2) should be false")
	}
	if inProbeRange(2, 5, 1) {
		t.Errorf("inProbeRange(2,5,1) should be false")
	}
	if inProbeRange(2, 5, 6) {
		t.Errorf("inProbeRange(2,5,6) should be false")
	}
	// gap >= hf (probe scan wrapped around the end of the table):
	// home in (gap, size) or [0, hf] is in range.
	if !inProbeRange(7, 2, 8) {
		t.Errorf("inProbeRange(7,2,8) should be true")
	}
	if !inProbeRange(7, 2, 1) {
		t.Errorf("inProbeRange(7,2,1) should be true")
	}
	if inProbeRange(7, 2, 5) {
		t.Errorf("inProbeRange(7,2,5) should be false")
	}
	if inProbeRange(7, 2, 7) {
		t.Errorf("inProbeRange(7,2,7) should be false")
	}
}

// TestSearchNil verifies that Search(nil) returns nil.
func TestSearchNil(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	ht.Insert(&TestData{S: "abcd"})
	if it := ht.Search(nil); it != nil {
		t.Errorf("Expected nil from Search(nil), got %v", it)
	}
}

// TestReplaceSemantics verifies that inserting a duplicate key replaces the
// stored item (the new pointer is what Search returns) without changing the
// element count.
func TestReplaceSemantics(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	p1 := &TestData{S: "key"}
	ht.Insert(p1)
	if got := ht.Search(&TestData{S: "key"}); got != p1 {
		t.Fatalf("Expected Search to return the inserted pointer")
	}
	p2 := &TestData{S: "key"}
	ht.Insert(p2)
	if ht.Len() != 1 {
		t.Errorf("Expected length of 1 after duplicate insert, got %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "key"}); got != p2 {
		t.Errorf("Expected Search to return the replacement pointer")
	}
}

// TestMinimumSizeAndCustomSaturation verifies the minimum initial size and
// that a custom saturation threshold controls when the table doubles.
func TestMinimumSizeAndCustomSaturation(t *testing.T) {
	ht := NewHashTab[TestData](5, 0.9)
	for i := 0; i < 4; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.size != 5 {
		t.Errorf("Expected no growth at load factor 4/5 with threshold 0.9, size = %d", ht.size)
	}
	ht.Insert(&TestData{S: "   4"}) // load factor 5/5 = 1.0 > 0.9 -> grow
	if ht.size != 10 {
		t.Errorf("Expected table to double to 10, size = %d", ht.size)
	}
	for i := 0; i < 5; i++ {
		if it := ht.Search(&TestData{S: fmt.Sprintf("%4d", i)}); it == nil {
			t.Errorf("Expected to find %4d after growth, did not", i)
		}
	}
}

// TestValuesEarlyBreak verifies that breaking out of a Values range loop
// terminates the iterator after the first element.
func TestValuesEarlyBreak(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 20; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}
	count := 0
	for range ht.Values() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early break after 1 element, got %d", count)
	}
}

// TestIteratorSnapshotSemantics verifies that All and Values iterate over a
// snapshot: mutating the table from the loop body is safe and does not
// change the sequence being iterated.
func TestIteratorSnapshotSemantics(t *testing.T) {
	ht := NewHashTab[TestData](7, 0)
	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	count := 0
	for range ht.All() {
		count++
		ht.Insert(&TestData{S: fmt.Sprintf("extra-%d", count)})
	}
	if count != 40 {
		t.Errorf("Expected All to iterate over the 40-element snapshot, got %d", count)
	}
	if ht.Len() != 80 {
		t.Errorf("Expected length of 80 after inserts during iteration, got %d", ht.Len())
	}

	// Deleting during a Values iteration still yields the full snapshot.
	count = 0
	for item := range ht.Values() {
		count++
		if !strings.HasPrefix(item.S, "extra") {
			ht.Delete(item)
		}
	}
	if count != 80 {
		t.Errorf("Expected Values to iterate over the 80-element snapshot, got %d", count)
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40 after deletes during iteration, got %d", ht.Len())
	}
}

// TestRandomizedModelCheck runs a fixed-seed sequence of mixed operations,
// verifying Search results (by pointer identity) and length against a map
// reference model after every single operation.
func TestRandomizedModelCheck(t *testing.T) {
	rng := rand.New(rand.NewSource(2023))
	ht := NewHashTab[TestData](5, 0)
	model := make(map[string]*TestData)

	key := func(i int) string { return fmt.Sprintf("key-%04d", i) }

	check := func(op int) {
		t.Helper()
		if ht.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, model has %d", op, ht.Len(), len(model))
		}
	}

	for op := 0; op < 1500; op++ {
		k := key(rng.Intn(150))
		switch rng.Intn(4) {
		case 0, 1: // insert (possibly replacing)
			p := &TestData{S: k}
			ht.Insert(p)
			model[k] = p
		case 2: // delete
			_, present := model[k]
			if got := ht.Delete(&TestData{S: k}); got != present {
				t.Fatalf("op %d: Delete(%q) = %v, model says %v", op, k, got, present)
			}
			delete(model, k)
		case 3: // search
			want, present := model[k]
			got := ht.Search(&TestData{S: k})
			if !present {
				if got != nil {
					t.Fatalf("op %d: Search(%q) = %v, expected nil", op, k, got)
				}
			} else if got != want {
				t.Fatalf("op %d: Search(%q) returned the wrong item", op, k)
			}
		}
		check(op)
	}

	// Final full cross-check through the iterator: every element yielded
	// must be the exact pointer stored in the model.
	seen := 0
	for _, item := range ht.All() {
		if model[item.S] != item {
			t.Errorf("All yielded an item not matching the model for key %q", item.S)
		}
		seen++
	}
	if seen != len(model) {
		t.Errorf("All yielded %d elements, model has %d", seen, len(model))
	}
}

// TestConcurrentReadersWritersIterators runs concurrent writers, readers and
// iterators (All, Values, Walk, Print) over overlapping key ranges; run with
// -race to verify the locking.
func TestConcurrentReadersWritersIterators(t *testing.T) {
	ht := NewHashTab[TestData](64, 0)
	const writers = 4
	const perWriter = 300

	var wg sync.WaitGroup

	// Writers on disjoint key ranges.
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			base := g * 10000
			for i := 0; i < perWriter; i++ {
				ht.Insert(&TestData{S: fmt.Sprintf("%8d", base+i)})
			}
			for i := 0; i < perWriter/2; i++ {
				ht.Delete(&TestData{S: fmt.Sprintf("%8d", base+i)})
			}
		}(g)
	}

	// Readers probing for whatever is currently present.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				ht.Search(&TestData{S: fmt.Sprintf("%8d", i%2000)})
				ht.Len()
				ht.IsEmpty()
			}
		}()
	}

	// Iterators running concurrently with the writers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			for range ht.All() {
			}
			for range ht.Values() {
			}
			ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
				return true
			}, nil)
			ht.Print(new(bytes.Buffer))
		}
	}()

	wg.Wait()
	if ht.Len() != writers*perWriter/2 {
		t.Errorf("Expected length of %d, got %d", writers*perWriter/2, ht.Len())
	}
}

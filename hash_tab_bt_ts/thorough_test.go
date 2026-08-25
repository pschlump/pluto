package hash_tab_bt_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fnvHash mirrors the FNV-1a hashing used by the hash function for the
// string and fmt.Stringer fallback paths.
func fnvHash(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}

func TestHashStringBranch(t *testing.T) {
	// A plain string takes the `string` branch of hash (no Hashable, no
	// Stringer).
	if got, want := hash("a plain string"), fnvHash("a plain string"); got != want {
		t.Errorf("Expected hash of string to be FNV-1a %d, got %d", want, got)
	}
	if hash("a plain string") != hash("a plain string") {
		t.Errorf("Expected hash to be deterministic for equal strings")
	}
}

func TestHashStringerBranch(t *testing.T) {
	// time.Duration implements fmt.Stringer but not Hashable, so it takes
	// the fmt.Stringer branch of hash.
	d := 90 * time.Second
	if got, want := hash(d), fnvHash(d.String()); got != want {
		t.Errorf("Expected hash of Stringer to be FNV-1a of String() %d, got %d", want, got)
	}
	if hash(d) != hash(90*time.Second) {
		t.Errorf("Expected hash to be deterministic for equal Stringer values")
	}
}

func TestHashPanicsOnUnhashableType(t *testing.T) {
	// A value that is neither Hashable, a string, nor a fmt.Stringer must
	// cause hash to panic.
	defer func() {
		if recover() == nil {
			t.Errorf("Expected hash to panic for a type with no Hashable/Stringer support")
		}
	}()
	hash(struct{ X int }{X: 1})
}

func TestNegativeHashKey(t *testing.T) {
	// Items whose HashKey returns a negative value must still land in a
	// valid bucket and support the full set of operations.
	ht := NewHashTab[TestData](7)
	const n = 30
	for i := 0; i < n; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("neg-%4d", i), Neg: true})
	}
	if ht.Len() != n {
		t.Fatalf("Expected length of %d, got %d", n, ht.Len())
	}
	for i := 0; i < n; i++ {
		find := TestData{S: fmt.Sprintf("neg-%4d", i), Neg: true}
		if got := ht.Search(&find); got == nil {
			t.Errorf("Expected to find %q", find.S)
		}
	}
	// An item with the same key but a non-negative hash usually lands in a
	// different bucket; if it hashes to bucket 0 it legitimately matches
	// its negated twin, so only the negative-hash items are asserted on.
	for i := 0; i < n; i += 2 {
		if !ht.Delete(&TestData{S: fmt.Sprintf("neg-%4d", i), Neg: true}) {
			t.Errorf("Expected Delete of neg-%4d to succeed", i)
		}
	}
	if ht.Len() != n/2 {
		t.Errorf("Expected length of %d, got %d", n/2, ht.Len())
	}
	count := 0
	for range ht.All() {
		count++
	}
	if count != n/2 {
		t.Errorf("Expected All to yield %d elements, got %d", n/2, count)
	}
}

func TestWalkAndDumpOnEmptyTable(t *testing.T) {
	ht := NewHashTab[TestData](5)

	// Walk on an empty table must not invoke the callback at all.
	count := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		count++
		return true
	}, nil)
	if count != 0 {
		t.Errorf("Expected Walk on empty table to visit 0 elements, got %d", count)
	}

	// Dump on an empty table reports 0 elements and no bucket sections.
	var sb strings.Builder
	ht.Dump(&sb)
	if strings.Contains(sb.String(), "bucket") {
		t.Errorf("Expected Dump of empty table to contain no bucket sections, got %q", sb.String())
	}
}

func TestSingleElement(t *testing.T) {
	ht := NewHashTab[TestData](5)
	item := &TestData{S: "only"}
	ht.Insert(item)
	if ht.IsEmpty() || ht.Len() != 1 {
		t.Fatalf("Expected a single-element table, Len=%d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "only"}); got != item {
		t.Errorf("Expected Search to return the inserted pointer")
	}
	n := 0
	ht.WalkFunc(func(a *TestData) {
		n++
		if a != item {
			t.Errorf("Expected WalkFunc to visit the inserted pointer")
		}
	})
	if n != 1 {
		t.Errorf("Expected WalkFunc to visit 1 element, got %d", n)
	}
	n = 0
	for a := range ht.All() {
		n++
		if a != item {
			t.Errorf("Expected All to yield the inserted pointer")
		}
	}
	if n != 1 {
		t.Errorf("Expected All to yield 1 element, got %d", n)
	}
	if !ht.Delete(&TestData{S: "only"}) {
		t.Errorf("Expected Delete of the single element to succeed")
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Errorf("Expected empty table after deleting the single element, Len=%d", ht.Len())
	}
}

func TestDuplicateInsertReplacesStoredItem(t *testing.T) {
	ht := NewHashTab[TestData](7)
	first := &TestData{S: "dup"}
	second := &TestData{S: "dup"}
	ht.Insert(first)
	ht.Insert(second)
	if ht.Len() != 1 {
		t.Fatalf("Expected duplicate insert to keep length 1, got %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "dup"}); got != second {
		t.Errorf("Expected the duplicate insert to replace the stored item with the new pointer")
	}
}

func TestTruncateThenReuse(t *testing.T) {
	ht := NewHashTab[TestData](7)
	for i := 0; i < 50; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("k-%d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after Truncate, Len=%d", ht.Len())
	}
	// The table must be fully usable after a Truncate.
	for i := 0; i < 50; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("k-%d", i)})
	}
	if ht.Len() != 50 {
		t.Errorf("Expected length of 50 after re-insert, got %d", ht.Len())
	}
	if got := ht.Search(&TestData{S: "k-49"}); got == nil {
		t.Errorf("Expected to find k-49 after Truncate and re-insert")
	}
}

func TestWalkEarlyStopAndUserData(t *testing.T) {
	ht := NewHashTab[TestData](5)
	const total = 40
	for i := 0; i < total; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("w-%4d", i)})
	}

	// Returning false from the callback stops the walk immediately, across
	// all buckets: the callback runs exactly once.
	count := 0
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		count++
		return false
	}, nil)
	if count != 1 {
		t.Errorf("Expected Walk to stop across buckets after 1 invocation, got %d", count)
	}

	// userData is passed through to every callback.
	type ctx struct{ n int }
	c := &ctx{}
	ht.Walk(func(pos, depth int, data *TestData, userData any) bool {
		uc, ok := userData.(*ctx)
		if !ok {
			t.Errorf("Expected userData to be passed through to the callback")
			return false
		}
		uc.n++
		return true
	}, c)
	if c.n != total {
		t.Errorf("Expected Walk to visit %d elements, got %d", total, c.n)
	}
}

// TestWalkEarlyStopAcrossBuckets is a regression test: returning false from
// the Walk callback must stop the walk across ALL buckets, not just within
// the current bucket's tree.  Elements are spread over multiple buckets
// (normal string hashing), so a bug that restarts the walk on the next
// non-empty bucket would produce more than 5 invocations.
func TestWalkEarlyStopAcrossBuckets(t *testing.T) {
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
}

func TestRandomizedAgainstModel(t *testing.T) {
	// Cross-check the table against a plain map model over a long run of
	// mixed operations, using a fixed seed for reproducibility.
	rng := rand.New(rand.NewSource(42))
	ht := NewHashTab[TestData](23)
	model := make(map[string]*TestData)

	key := func() string { return fmt.Sprintf("key-%d", rng.Intn(160)) }

	verify := func(op int) {
		if ht.Len() != len(model) {
			t.Fatalf("op %d: Expected length of %d, got %d", op, len(model), ht.Len())
		}
		seen := make(map[string]*TestData, len(model))
		for item := range ht.All() {
			if _, dup := seen[item.S]; dup {
				t.Fatalf("op %d: All yielded %q twice", op, item.S)
			}
			seen[item.S] = item
		}
		if len(seen) != len(model) {
			t.Fatalf("op %d: Expected All to yield %d elements, got %d", op, len(model), len(seen))
		}
		for k, v := range model {
			if seen[k] != v {
				t.Fatalf("op %d: model has %q but table does not have the same stored item", op, k)
			}
		}
	}

	const ops = 1200
	for i := 0; i < ops; i++ {
		switch rng.Intn(4) {
		case 0, 1: // insert (duplicates replace)
			item := &TestData{S: key()}
			ht.Insert(item)
			model[item.S] = item
		case 2: // delete (possibly absent)
			find := &TestData{S: key()}
			_, inModel := model[find.S]
			if got := ht.Delete(find); got != inModel {
				t.Fatalf("op %d: Expected Delete(%q) to return %v, got %v", i, find.S, inModel, got)
			}
			delete(model, find.S)
		case 3: // search (possibly absent)
			find := &TestData{S: key()}
			want, inModel := model[find.S]
			got := ht.Search(find)
			if inModel {
				if got != want {
					t.Fatalf("op %d: Expected Search(%q) to return the model item", i, find.S)
				}
			} else if got != nil {
				t.Fatalf("op %d: Expected Search(%q) to return nil", i, find.S)
			}
			if ht.ItemExists(find) != inModel {
				t.Fatalf("op %d: Expected ItemExists(%q) to return %v", i, find.S, inModel)
			}
		}
		if i%97 == 0 {
			verify(i)
		}
	}
	verify(ops)

	// Sorted cross-check: drain the model and the table and compare keys.
	var wantKeys []string
	for k := range model {
		wantKeys = append(wantKeys, k)
	}
	sort.Strings(wantKeys)
	var gotKeys []string
	ht.WalkFunc(func(a *TestData) { gotKeys = append(gotKeys, a.S) })
	sort.Strings(gotKeys)
	if strings.Join(wantKeys, ",") != strings.Join(gotKeys, ",") {
		t.Errorf("Expected table and model to contain identical key sets")
	}
}

func TestConcurrentWritersReadersIterators(t *testing.T) {
	// Writers, readers and iterators run concurrently; must be clean under
	// the race detector.
	ht := NewHashTab[TestData](97)
	const workers = 4
	const perWorker = 200

	var wg sync.WaitGroup

	// Writers: each owns a disjoint key range, inserting and deleting.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				item := TestData{S: fmt.Sprintf("cw-%d-i-%d", w, i)}
				ht.Insert(&item)
				if i%3 == 0 {
					ht.Delete(&item)
				}
			}
		}(w)
	}

	// Readers: search for keys that may or may not be present.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				find := TestData{S: fmt.Sprintf("cw-%d-i-%d", w, i)}
				_ = ht.Search(&find)
				_ = ht.ItemExists(&find)
			}
			_ = ht.Len()
			_ = ht.IsEmpty()
		}(w)
	}

	// Iterators: walk snapshots while mutations are in flight.  All takes
	// a snapshot, so the count may vary but must never exceed the maximum
	// possible population.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				n := 0
				for range ht.All() {
					n++
				}
				if n > workers*perWorker {
					t.Errorf("All yielded %d elements, more than ever inserted", n)
				}
				m := 0
				ht.WalkFunc(func(a *TestData) { m++ })
				if m > workers*perWorker {
					t.Errorf("WalkFunc visited %d elements, more than ever inserted", m)
				}
				var sb strings.Builder
				ht.Dump(&sb)
			}
		}()
	}

	wg.Wait()

	// Final state: the keys with i%3 != 0 remain.
	want := 0
	for w := 0; w < workers; w++ {
		for i := 0; i < perWorker; i++ {
			if i%3 != 0 {
				want++
				item := TestData{S: fmt.Sprintf("cw-%d-i-%d", w, i)}
				if !ht.ItemExists(&item) {
					t.Errorf("Expected %q to be present after concurrent run", item.S)
				}
			}
		}
	}
	if ht.Len() != want {
		t.Errorf("Expected length of %d after concurrent run, got %d", want, ht.Len())
	}
}

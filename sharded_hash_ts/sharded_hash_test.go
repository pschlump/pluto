package sharded_hash_ts

// Basic API tests for the striped hash table: construction and parameter
// normalization, insert/search/delete round trips, replacement semantics,
// StripeCount/StripeLen metrics, nil and zero-value tolerance, and the panic
// contract with its messages.  Structural invariants, Scan coverage, the
// randomized model, and the concurrency tests live in thorough_test.go.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"
	"testing"
)

// TestData is the standard test element: S is the key that the equality and
// hash functions read; N is satellite data both ignore, used to verify that
// duplicate inserts replace the stored value.
type TestData struct {
	S string
	N int
}

// newTestHash builds a table with deterministic FNV-1a-64 hashing over S, so
// tests can reason about stripe routing and bucket placement.  The default
// NewShardedHash hashes with a per-process random maphash seed, which is
// fine for membership assertions but not for placement.
func newTestHash(stripes, initialCapacity int, loadFactor float64) *ShardedHash[TestData] {
	return NewShardedHashFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(v TestData) uint64 {
			h := fnv.New64a()
			_, _ = h.Write([]byte(v.S))
			return h.Sum64()
		},
		stripes, initialCapacity, loadFactor,
	)
}

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// expectPanicMessage additionally checks that the panic message contains
// `want` — the contract says each message names the method and the fix.
func expectPanicMessage(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
			t.Errorf("Expected the %s panic message to contain %q, got %v", name, want, r)
		}
	}()
	fx()
}

// TestConstructorNormalization verifies that the stripe count is rounded up
// to a power of two, the per-stripe capacity is rounded up to a power of two
// with a floor of 16, and a bad load factor selects the 0.75 default.
func TestConstructorNormalization(t *testing.T) {
	h := newTestHash(5, 0, 0)
	if h.StripeCount() != 8 {
		t.Errorf("5 stripes should round up to 8, got %d", h.StripeCount())
	}
	if got := len(h.stripes[0].tab.heads); got != 16 {
		t.Errorf("capacity 0 should select the 16 default, got %d heads", got)
	}
	if h.loadFac != 0.75 {
		t.Errorf("load factor 0 should select the 0.75 default, got %v", h.loadFac)
	}

	h = newTestHash(100, 17, 0.5)
	if h.StripeCount() != 128 {
		t.Errorf("100 stripes should round up to 128, got %d", h.StripeCount())
	}
	if got := len(h.stripes[0].tab.heads); got != 32 {
		t.Errorf("capacity 17 should round up to 32, got %d heads", got)
	}
	if h.loadFac != 0.5 {
		t.Errorf("expected load factor 0.5, got %v", h.loadFac)
	}

	// 1 and 2 are legal (a 1-stripe table is the degenerate single-lock case).
	if got := newTestHash(1, 0, 0).StripeCount(); got != 1 {
		t.Errorf("1 stripe should stay 1, got %d", got)
	}
	if got := newTestHash(2, 0, 0).StripeCount(); got != 2 {
		t.Errorf("2 stripes should stay 2, got %d", got)
	}

	// The routing shift must address exactly the stripe range.
	for _, n := range []int{1, 2, 100, 256} {
		h := newTestHash(n, 16, 0)
		seen := make(map[int]bool)
		for i := range 10000 {
			s := h.routeHash(h.hash(TestData{S: fmt.Sprintf("spread-%d", i)}))
			idx := -1
			for j, sp := range h.stripes {
				if sp == s {
					idx = j
					break
				}
			}
			if idx < 0 {
				t.Fatalf("routing returned a stripe not in the table")
			}
			seen[idx] = true
		}
		if len(seen) < min(h.StripeCount(), 2) { // 1-stripe tables can only hit 1
			t.Errorf("%d stripes: routing hit only %d distinct stripes over 10000 keys", h.StripeCount(), len(seen))
		}
	}
}

// TestBasics covers the insert/search/delete round trip, replacement
// semantics, Len/IsEmpty, Truncate, and the per-stripe metrics.
func TestBasics(t *testing.T) {
	h := newTestHash(4, 16, 0)

	if !h.IsEmpty() || h.Len() != 0 || h.Length() != 0 {
		t.Errorf("a fresh table should be empty")
	}
	if added := h.Insert(TestData{S: "a", N: 1}); !added {
		t.Errorf("first insert should report added=true")
	}
	if added := h.Insert(TestData{S: "a", N: 2}); added {
		t.Errorf("duplicate insert should report added=false (replaced)")
	}
	if h.Len() != 1 {
		t.Errorf("duplicate insert should keep length 1, got %d", h.Len())
	}
	if v, found := h.Search(TestData{S: "a"}); !found || v.N != 2 {
		t.Errorf("Search should return the replacement, got %v found=%v", v, found)
	}

	for i := range 100 {
		h.Insert(TestData{S: fmt.Sprintf("k%03d", i), N: i})
	}
	if h.Len() != 101 {
		t.Errorf("expected length 101, got %d", h.Len())
	}
	if h.IsEmpty() {
		t.Errorf("table with 101 elements should not be empty")
	}

	// Per-stripe metrics add up to the whole.
	total := 0
	for i := range h.StripeCount() {
		total += h.StripeLen(i)
	}
	if total != h.Len() {
		t.Errorf("stripe lengths sum to %d, Len() = %d", total, h.Len())
	}
	if h.StripeLen(-1) != 0 || h.StripeLen(h.StripeCount()) != 0 {
		t.Errorf("out-of-range StripeLen should report 0")
	}

	// Delete across the board.
	for i := range 100 {
		if !h.Delete(TestData{S: fmt.Sprintf("k%03d", i)}) {
			t.Errorf("expected to delete k%03d", i)
		}
	}
	if h.Delete(TestData{S: "no-such-key"}) {
		t.Errorf("deleting a missing key should return false")
	}
	if h.Len() != 1 {
		t.Errorf("expected length 1 after deletes, got %d", h.Len())
	}

	h.Truncate()
	if !h.IsEmpty() || h.Len() != 0 {
		t.Errorf("Truncate should empty the table, got length %d", h.Len())
	}
	if v, found := h.Search(TestData{S: "a"}); found {
		t.Errorf("Search after Truncate should not find %v", v)
	}
	// A truncated table is refillable.
	if !h.Insert(TestData{S: "again"}) {
		t.Errorf("insert after Truncate should be an add")
	}
	if h.Len() != 1 {
		t.Errorf("expected length 1 after refill, got %d", h.Len())
	}
}

// TestTruncateResetsHeads verifies that Truncate re-creates each stripe's
// table at the configured initial capacity (growth is undone).
func TestTruncateResetsHeads(t *testing.T) {
	h := newTestHash(2, 8, 0.75)
	for i := range 500 { // plenty of doubling
		h.Insert(TestData{S: fmt.Sprintf("t%04d", i)})
	}
	grew := false
	for i := range h.StripeCount() {
		if len(h.stripes[i].tab.heads) > 8 {
			grew = true
		}
	}
	if !grew {
		t.Fatalf("expected some stripe to have grown past 8 heads")
	}
	h.Truncate()
	for i := range h.StripeCount() {
		if got := len(h.stripes[i].tab.heads); got != 16 { // capacity 8 floors at the 16 default
			t.Errorf("stripe %d should reset to 16 heads, got %d", i, got)
		}
	}
	if h.Len() != 0 {
		t.Errorf("expected an empty table, got length %d", h.Len())
	}
}

// TestNilTolerated verifies that a nil table and the zero value behave as
// empty tables for every operation that has a sane answer.
func TestNilTolerated(t *testing.T) {
	var nilTable *ShardedHash[TestData]
	var zeroTable ShardedHash[TestData]

	for name, h := range map[string]*ShardedHash[TestData]{
		"nil table":  nilTable,
		"zero value": &zeroTable,
	} {
		if !h.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if h.Len() != 0 || h.Length() != 0 {
			t.Errorf("%s: length should be 0, got %d/%d", name, h.Len(), h.Length())
		}
		if _, found := h.Search(TestData{S: "x"}); found {
			t.Errorf("%s: Search should report not-found", name)
		}
		if h.Delete(TestData{S: "x"}) {
			t.Errorf("%s: Delete should return false", name)
		}
		if h.StripeCount() != 0 || h.StripeLen(0) != 0 {
			t.Errorf("%s: StripeCount/StripeLen should report 0", name)
		}
		n := 0
		if !h.Walk(func(pos int, data TestData) bool {
			n++
			return true
		}) || n != 0 {
			t.Errorf("%s: Walk should complete without calls, got n=%d", name, n)
		}
		for range h.All() {
			t.Errorf("%s: All should yield nothing", name)
		}
		for range h.Values() {
			t.Errorf("%s: Values should yield nothing", name)
		}
		items, next := h.Scan(0, 10)
		if len(items) != 0 || next != 0 {
			t.Errorf("%s: Scan should return nothing and cursor 0, got %d items cursor %d", name, len(items), next)
		}
		buf := new(bytes.Buffer)
		h.Dump(buf) // must not panic
		if !strings.Contains(buf.String(), "nil table") && name == "nil table" {
			t.Errorf("%s: unexpected Dump output %q", name, buf.String())
		}
		h.Truncate() // must not panic
	}

	// LockKey on a nil/zero-value table returns a working no-op unlock.
	unlock := nilTable.LockKey(TestData{S: "x"})
	unlock()
	unlock = zeroTable.LockKey(TestData{S: "x"})
	unlock()

	// The Nl reads tolerate a zero-value table (no functions set).
	if zeroTable.NlLen() != 0 || !zeroTable.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty on zero value should report empty")
	}
	if _, found := zeroTable.NlSearch(TestData{S: "x"}); found {
		t.Errorf("NlSearch on zero value should report not-found")
	}
	if zeroTable.NlDelete(TestData{S: "x"}) {
		t.Errorf("NlDelete on zero value should return false")
	}
}

// TestNilPanics verifies the panic contract and that each message names the
// method and the fix.
func TestNilPanics(t *testing.T) {
	var nilTable *ShardedHash[TestData]
	expectPanicMessage(t, "Insert on nil table", "Insert called on a nil table",
		func() { nilTable.Insert(TestData{S: "x"}) })

	var zeroTable ShardedHash[TestData]
	expectPanicMessage(t, "Insert on zero value", "NewShardedHash",
		func() { zeroTable.Insert(TestData{S: "x"}) })

	validHash := func(v TestData) uint64 { return 7 }
	validEq := func(a, b TestData) bool { return a.S == b.S }
	expectPanicMessage(t, "NewShardedHashFunc(nil eq)", "nil equality function",
		func() { NewShardedHashFunc(nil, validHash, 8, 16, 0) })
	expectPanicMessage(t, "NewShardedHashFunc(nil hash)", "nil hash function",
		func() { NewShardedHashFunc(validEq, nil, 8, 16, 0) })
	expectPanicMessage(t, "NewShardedHashFunc stripes 0", "at least 1",
		func() { NewShardedHashFunc(validEq, validHash, 0, 16, 0) })
	expectPanicMessage(t, "NewShardedHash stripes -3", "at least 1",
		func() { NewShardedHash[TestData](-3, 16, 0) })
	expectPanic(t, "NewShardedHash stripes beyond the cursor maximum", func() {
		NewShardedHash[TestData](1<<24+1, 16, 0) // rounds past 2^24 stripes
	})

	// NlInsert (the Insert body) carries the same zero-value panic.
	zero := &ShardedHash[TestData]{}
	expectPanicMessage(t, "NlInsert on zero value", "NewShardedHash",
		func() { zero.NlInsert(TestData{S: "x"}) })
}

// TestHashZeroLegal verifies that a hash function returning 0 works — chain
// membership needs no empty-slot marker, unlike hash_grow (which must remap
// 0 to 1).  Everything lands in bucket 0 of stripe 0 and must still behave.
func TestHashZeroLegal(t *testing.T) {
	h := NewShardedHashFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 0 }, // every key collides in one chain
		2, 16, 0,
	)
	for _, k := range []string{"a", "b", "c"} {
		if !h.Insert(k) {
			t.Errorf("expected insert of %q to be an add", k)
		}
	}
	if h.Len() != 3 {
		t.Fatalf("expected length 3, got %d", h.Len())
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, found := h.Search(k); !found {
			t.Errorf("expected to find %q in the degenerate chain", k)
		}
	}
	if !h.Delete("b") {
		t.Errorf("expected to delete b from the middle of the chain")
	}
	if _, found := h.Search("b"); found {
		t.Errorf("b should be gone")
	}
	if _, found := h.Search("c"); !found {
		t.Errorf("c must survive the unlink")
	}
	// The full scan still sees the two survivors and terminates.
	got := make(map[string]bool)
	cursor := uint64(0)
	for {
		items, next := h.Scan(cursor, 0)
		for _, it := range items {
			got[it] = true
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if len(got) != 2 || !got["a"] || !got["c"] {
		t.Errorf("scan should return a and c, got %v", got)
	}
}

// TestMapPattern verifies the documented two-field struct pattern for
// key/value use: eq and hash read only the key field, so Insert replaces
// and Search returns the stored pair with its value.
func TestMapPattern(t *testing.T) {
	type kv struct {
		Key string
		Val int
	}
	h := NewShardedHashFunc(
		func(a, b kv) bool { return a.Key == b.Key },
		func(v kv) uint64 {
			hh := fnv.New64a()
			_, _ = hh.Write([]byte(v.Key))
			return hh.Sum64()
		},
		8, 16, 0,
	)
	if added := h.Insert(kv{Key: "port", Val: 6379}); !added {
		t.Errorf("first insert of port should be an add")
	}
	if added := h.Insert(kv{Key: "port", Val: 6380}); added {
		t.Errorf("re-insert of port should replace")
	}
	// Search with a bare-key probe returns the stored pair, value included.
	got, found := h.Search(kv{Key: "port"})
	if !found || got.Val != 6380 {
		t.Errorf("Search should return the pair with the latest value, got %v found=%v", got, found)
	}
	if h.Len() != 1 {
		t.Errorf("expected length 1, got %d", h.Len())
	}
}

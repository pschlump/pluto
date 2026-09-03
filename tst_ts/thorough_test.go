package tst_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: structural invariants, the fixed-seed randomized
// property test against a map[string]int reference model, snapshot
// iterator semantics, the Lock/Nl* compound-operation surface, and the
// race-detector concurrency tests.

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Panic assertion helper
// -------------------------------------------------------------------------------------------------------

// expectPanic runs fx, which must panic; when want is not empty the
// panic message must contain it.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if want != "" {
			if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
				t.Errorf("Unexpected panic message from %s: %v (expected it to contain %q)", name, r, want)
			}
		}
	}()
	fx()
}

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the structural invariants of the trie:
// All visits exactly Length distinct keys in strictly ascending key
// order, every key All visits is found by Contains, and the node graph
// carries no dead weight — every node without a value has at least one
// child (the pruning invariant of Delete).  Call it after any
// structural change.
//
// The node-graph walk reads root/left/mid/right WITHOUT holding the
// lock, so call this from single-goroutine tests only.
func checkInvariants[T any](t *testing.T, tt *Tst[T]) {
	t.Helper()

	var keys []string
	for key := range tt.All() {
		keys = append(keys, key)
	}
	if !slices.IsSorted(keys) {
		t.Fatalf("All() did not visit keys in ascending order: %v", keys)
	}
	if len(keys) != tt.Length() {
		t.Fatalf("All() visited %d keys but Length()=%d", len(keys), tt.Length())
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if key == "" {
			t.Fatalf("All() visited the empty key, which Insert rejects")
		}
		if seen[key] {
			t.Fatalf("All() visited %q twice", key)
		}
		seen[key] = true
		if !tt.Contains(key) {
			t.Fatalf("Contains(%q)=false for a key All() visited", key)
		}
	}

	// Every node without a value must have at least one child — Delete
	// prunes childless value-less nodes, and Insert never leaves one.
	var check func(x *tstNode[T])
	check = func(x *tstNode[T]) {
		if x == nil {
			return
		}
		if !x.hasValue && x.left == nil && x.mid == nil && x.right == nil {
			t.Fatalf("node %q has no value and no children (pruning invariant violated)", x.c)
		}
		check(x.left)
		check(x.mid)
		check(x.right)
	}
	check(tt.root)
	if tt.Length() == 0 && tt.root != nil {
		t.Fatalf("Length()=0 but root is not nil (unpruned dead branches)")
	}
	if tt.Length() != 0 && tt.root == nil {
		t.Fatalf("Length()=%d but root is nil", tt.Length())
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestTstRandomizedModel hammers the trie with random Insert, Search,
// Contains, Delete and key-query operations over a small key space and
// cross-checks every result against a map[string]int reference model.
// The seed is fixed (42), so the run is deterministic.
func TestTstRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "abcd"
	randKey := func() string {
		n := 1 + rng.Intn(6)
		var sb strings.Builder
		for range n {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}
	randPattern := func() string {
		n := 1 + rng.Intn(5)
		var sb strings.Builder
		for range n {
			if rng.Intn(4) == 0 {
				sb.WriteByte('.')
			} else {
				sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
		}
		return sb.String()
	}

	var tt Tst[int]
	model := make(map[string]int)

	modelLongestPrefixOf := func(query string) string {
		best := ""
		for key := range model {
			if strings.HasPrefix(query, key) && len(key) > len(best) {
				best = key
			}
		}
		return best
	}
	modelKeysWithPrefix := func(prefix string) []string {
		var keys []string
		for key := range model {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		}
		slices.Sort(keys)
		return keys
	}
	modelKeysThatMatch := func(pattern string) []string {
		var keys []string
	outer:
		for key := range model {
			if len(key) != len(pattern) {
				continue
			}
			for i := range key {
				if pattern[i] != '.' && pattern[i] != key[i] {
					continue outer
				}
			}
			keys = append(keys, key)
		}
		slices.Sort(keys)
		return keys
	}

	verify := func(step int) {
		if tt.Length() != len(model) {
			t.Fatalf("step %d: Length()=%d, model has %d keys", step, tt.Length(), len(model))
		}
		if tt.IsEmpty() != (len(model) == 0) {
			t.Fatalf("step %d: IsEmpty()=%v, model has %d keys", step, tt.IsEmpty(), len(model))
		}
		if tt.Len() != tt.Length() {
			t.Fatalf("step %d: Len()=%d disagrees with Length()=%d", step, tt.Len(), tt.Length())
		}
		// Every model key must be found with its model value.
		for key, want := range model {
			got, ok := tt.Search(key)
			if !ok || got != want {
				t.Fatalf("step %d: Search(%q)=(%d, %v), model has (%d, true)", step, key, got, ok, want)
			}
		}
		// All must visit exactly the model keys, ascending.
		var allKeys []string
		for key := range tt.All() {
			allKeys = append(allKeys, key)
		}
		slices.Sort(allKeys)
		if !slices.Equal(allKeys, modelKeysWithPrefix("")) {
			t.Fatalf("step %d: All() visited %v, model keys are %v", step, allKeys, modelKeysWithPrefix(""))
		}
		checkInvariants(t, &tt)
	}

	for step := range 1200 {
		key := randKey()
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert
			value := rng.Intn(10000)
			got := tt.Insert(key, value)
			_, present := model[key]
			model[key] = value
			if got != !present {
				t.Fatalf("step %d: Insert(%q)=%v, model says the key was present=%v", step, key, got, present)
			}
		case 4, 5, 6: // Search
			got, ok := tt.Search(key)
			want, present := model[key]
			if ok != present || (present && got != want) {
				t.Fatalf("step %d: Search(%q)=(%d, %v), model has (%d, %v)", step, key, got, ok, want, present)
			}
		case 7: // Contains
			if got, want := tt.Contains(key), func() bool { _, ok := model[key]; return ok }(); got != want {
				t.Fatalf("step %d: Contains(%q)=%v, model says %v", step, key, got, want)
			}
		case 8: // Delete
			_, present := model[key]
			delete(model, key)
			if got := tt.Delete(key); got != present {
				t.Fatalf("step %d: Delete(%q)=%v, model says the key was present=%v", step, key, got, present)
			}
		case 9: // Key queries
			if got, want := tt.LongestPrefixOf(key), modelLongestPrefixOf(key); got != want {
				t.Fatalf("step %d: LongestPrefixOf(%q)=%q, model says %q", step, key, got, want)
			}
			prefix := key[:1+rng.Intn(len(key))]
			got := tt.KeysWithPrefix(prefix)
			want := modelKeysWithPrefix(prefix)
			if len(got) == 0 && len(want) == 0 {
				// nil and empty are both "none" here.
			} else if !slices.Equal(got, want) {
				t.Fatalf("step %d: KeysWithPrefix(%q)=%v, model says %v", step, prefix, got, want)
			}
			pattern := randPattern()
			got = tt.KeysThatMatch(pattern)
			want = modelKeysThatMatch(pattern)
			if len(got) == 0 && len(want) == 0 {
			} else if !slices.Equal(got, want) {
				t.Fatalf("step %d: KeysThatMatch(%q)=%v, model says %v", step, pattern, got, want)
			}
		}
		if step%23 == 0 {
			verify(step)
		}
	}
	verify(1200)

	// Drain the trie: every Delete must report true, and the trie must
	// return to its zero shape.
	keys := modelKeysWithPrefix("")
	for _, key := range keys {
		if !tt.Delete(key) {
			t.Fatalf("Delete(%q) of a model key returned false during the drain", key)
		}
	}
	if !tt.IsEmpty() || tt.root != nil {
		t.Fatalf("Expected an empty, fully pruned trie after the drain, IsEmpty()=%v root=%v", tt.IsEmpty(), tt.root != nil)
	}
	checkInvariants(t, &tt)
}

// -------------------------------------------------------------------------------------------------------
// Snapshot iterators
// -------------------------------------------------------------------------------------------------------

// TestIteratorSnapshotSemantics is the opposite of the plain tst's
// live-trie contract: All takes an eager snapshot of the (key, value)
// pairs when it is called — later modifications, even deleting every
// key, are not observed — and mutating the trie from inside the loop is
// safe.  KeysWithPrefix and KeysThatMatch return snapshot slices the
// same way.
func TestIteratorSnapshotSemantics(t *testing.T) {
	var tt Tst[int]
	keys := []string{"she", "sells", "sea", "shells", "by"}
	for i, key := range keys {
		tt.Insert(key, i)
	}

	all := tt.All()
	prefixed := tt.KeysWithPrefix("sh")
	matched := tt.KeysThatMatch("s..")

	// Delete every key; the iterators and slices above must not observe
	// any of this.
	for _, key := range keys {
		tt.Delete(key)
	}
	if !tt.IsEmpty() {
		t.Fatalf("Expected an empty trie after deleting all keys.")
	}

	var got []string
	for key := range all {
		got = append(got, key)
	}
	want := slices.Clone(keys)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("All after deleting every key: expected %v, got %v", want, got)
	}
	if want := []string{"she", "shells"}; !slices.Equal(prefixed, want) {
		t.Errorf("KeysWithPrefix snapshot: expected %v, got %v", want, prefixed)
	}
	if want := []string{"sea", "she"}; !slices.Equal(matched, want) {
		t.Errorf("KeysThatMatch snapshot: expected %v, got %v", want, matched)
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	for i, key := range keys {
		tt.Insert(key, i)
	}
	visited := 0
	for key := range tt.All() {
		visited++
		tt.Delete(key)
	}
	if visited != len(keys) {
		t.Errorf("Expected %d visits while deleting during iteration, got %d", len(keys), visited)
	}
	if !tt.IsEmpty() {
		t.Errorf("Expected empty trie after deleting during iteration.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Lock / Unlock + Nl* compound operations
// -------------------------------------------------------------------------------------------------------

// TestLockNlCompound verifies the Lock/Unlock + Nl* escape hatch for
// compound operations: a search followed by a delete (or a bulk insert)
// runs atomically under one lock hold.
func TestLockNlCompound(t *testing.T) {
	var tt Tst[int]
	for i := range 40 {
		tt.Insert(fmt.Sprintf("c%03d", i), i)
	}

	tt.Lock()
	if v, found := tt.NlSearch("c021"); found {
		if v != 21 {
			t.Errorf("NlSearch returned stale value %d, want 21", v)
		}
		if !tt.NlDelete("c021") {
			t.Errorf("NlDelete inside the held lock should succeed")
		}
	} else {
		t.Errorf("NlSearch should have found c021")
	}
	if tt.NlLen() != 39 || tt.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty should report 39/false, got %d/%v", tt.NlLen(), tt.NlIsEmpty())
	}
	// The empty key is rejected by the Nl methods too.
	if tt.NlInsert("", 1) || tt.NlDelete("") {
		t.Errorf("Expected NlInsert(\"\")/NlDelete(\"\") to return false.")
	}
	// A bulk insert under the same single lock hold.
	for i := 100; i < 150; i++ {
		tt.NlInsert(fmt.Sprintf("c%03d", i), i)
	}
	tt.Unlock()

	if tt.Len() != 89 {
		t.Fatalf("Expected length 89 after the compound section, got %d", tt.Len())
	}
	if _, found := tt.Search("c021"); found {
		t.Errorf("c021 should be gone")
	}
	for i := 100; i < 150; i++ {
		if v, found := tt.Search(fmt.Sprintf("c%03d", i)); !found || v != i {
			t.Errorf("Expected to find c%03d with value %d, got %v found=%v", i, i, v, found)
		}
	}
	checkInvariants(t, &tt)
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: concurrent access (race-detector targets)
// -------------------------------------------------------------------------------------------------------

// TestTstConcurrent runs producers and consumers against one shared
// trie, with observers hammering the read operations and the snapshot
// iterators while they work.  It is primarily a test for the race
// detector (`make race`); the accounting is exact: the producers insert
// disjoint key ranges, the consumers delete them, and the trie must
// hold exactly the produced set in between and nothing at the end.
func TestTstConcurrent(t *testing.T) {
	var tt Tst[int]

	const producers = 4
	const perProducer = 250
	const total = producers * perProducer

	key := func(p, i int) string { return fmt.Sprintf("w%d-k%04d", p, i) }

	var wg sync.WaitGroup
	var done atomic.Bool

	// Observers exercise the read-lock operations and the snapshot
	// iterators concurrently with the writers below, so the race
	// detector sees reads and writes overlapping.
	for range 2 {
		wg.Go(func() {
			for !done.Load() {
				_ = tt.Len()
				_ = tt.Length()
				_ = tt.IsEmpty()
				_, _ = tt.Search("w0-k0000")
				_ = tt.Contains("w1-k0001")
				_ = tt.LongestPrefixOf("w2-k0002-extra")
				_ = tt.KeysWithPrefix("w3")
				_ = tt.KeysThatMatch("w.-k00..")
				for range tt.All() {
					break // one snapshot is taken even for a single visit
				}
			}
		})
	}

	// Producers insert disjoint key ranges.
	var prodWg sync.WaitGroup
	for p := range producers {
		prodWg.Go(func() {
			for i := range perProducer {
				tt.Insert(key(p, i), p*perProducer+i)
			}
		})
	}
	prodWg.Wait()

	if got := tt.Len(); got != total {
		t.Fatalf("Expected %d keys after the concurrent inserts, got %d", total, got)
	}
	for p := range producers {
		for i := range perProducer {
			if v, ok := tt.Search(key(p, i)); !ok || v != p*perProducer+i {
				t.Fatalf("Search(%q)=(%d, %v), expected (%d, true)", key(p, i), v, ok, p*perProducer+i)
			}
		}
	}

	// Consumers delete the disjoint ranges concurrently.
	for p := range producers {
		prodWg.Go(func() {
			for i := range perProducer {
				tt.Delete(key(p, i))
			}
		})
	}
	prodWg.Wait()

	done.Store(true)
	wg.Wait()

	if !tt.IsEmpty() || tt.Length() != 0 {
		t.Errorf("Expected empty trie after the concurrent drain, got length %d", tt.Length())
	}
	checkInvariants(t, &tt)
}

// TestConcurrentCompound mixes compound Lock+Nl sections with plain
// locked operations: writers do an atomic read-modify-write under Lock
// while other goroutines insert and iterate.  Race-detector target
// (`make race`); the value accounting is exact — a torn read-modify-
// write would lose increments.
func TestConcurrentCompound(t *testing.T) {
	var tt Tst[int]

	const writers = 4
	const rounds = 200
	const keySpace = 50

	var wg sync.WaitGroup
	// Compound writers: atomically increment xNNN's value under one lock
	// hold (NlSearch then NlInsert).
	for range writers {
		wg.Go(func() {
			for r := range rounds {
				k := fmt.Sprintf("x%03d", r%keySpace)
				tt.Lock()
				v, _ := tt.NlSearch(k)
				tt.NlInsert(k, v+1)
				tt.Unlock()
			}
		})
	}
	// Plain writers on a disjoint key space, plus snapshot iterators.
	for w := range 2 {
		wg.Go(func() {
			for r := range rounds {
				tt.Insert(fmt.Sprintf("y%d-%03d", w, r%keySpace), r)
				for range tt.All() {
					break
				}
			}
		})
	}
	wg.Wait()

	if tt.Len() != 3*keySpace { // 50 x-keys + 2*50 y-keys
		t.Fatalf("Expected length %d, got %d", 3*keySpace, tt.Len())
	}
	// Each x-key was incremented writers*(rounds/keySpace) times,
	// starting from absent (NlSearch reports the zero value).
	want := writers * (rounds / keySpace)
	for i := range keySpace {
		if v, ok := tt.Search(fmt.Sprintf("x%03d", i)); !ok || v != want {
			t.Errorf("x%03d: expected (%d, true), got (%d, %v) — lost increments", i, want, v, ok)
		}
	}
	checkInvariants(t, &tt)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that value-level marshalers are honored through the trie.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

func TestMarshalJSON(t *testing.T) {
	// Exact object output, keys ascending regardless of insert order.
	var tt Tst[int]
	for i, key := range []string{"shells", "she", "sea", "by"} {
		tt.Insert(key, i)
	}
	b, err := json.Marshal(&tt)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"by":3,"sea":2,"she":1,"shells":0}` {
		t.Errorf("Expected ascending-key object, got %s", b)
	}

	// Values use their normal JSON encoding, including a value type
	// with its own MarshalJSON.
	var custom Tst[upperString]
	custom.Insert("k", upperString("ab"))
	if b, err := json.Marshal(&custom); err != nil || string(b) != `{"k":"AB"}` {
		t.Errorf("Expected value-level MarshalJSON to be honored, got (%s, %v)", b, err)
	}

	// An empty trie is {}.
	var empty Tst[int]
	if b, err := json.Marshal(&empty); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for an empty trie, got (%s, %v)", b, err)
	}

	// A zero-value trie is a tolerated read: {}.
	var zero Tst[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for a zero-value trie, got (%s, %v)", b, err)
	}

	// A direct call on a nil trie encodes as {}; json.Marshal on a nil
	// *Tst never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTst *Tst[int]
	if b, err := nilTst.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} from a direct nil-trie call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTst); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil trie, got (%s, %v)", b, err)
	}

	// A value the json package cannot encode surfaces its error.
	var bad Tst[chan int]
	bad.Insert("k", make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a trie of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded pairs become the trie's contents.
	var tt Tst[int]
	if err := json.Unmarshal([]byte(`{"she":1,"sea":6,"by":4}`), &tt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if tt.Length() != 3 {
		t.Fatalf("Expected Length()=3, got %d", tt.Length())
	}
	for key, want := range map[string]int{"she": 1, "sea": 6, "by": 4} {
		if got, ok := tt.Search(key); !ok || got != want {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", key, want, got, ok)
		}
	}
	checkInvariants(t, &tt)

	// Unmarshaling replaces the current contents rather than merging.
	if err := json.Unmarshal([]byte(`{"new":9}`), &tt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if tt.Length() != 1 || tt.Contains("she") {
		t.Errorf("Expected the contents to be replaced, Length()=%d Contains(\"she\")=%v", tt.Length(), tt.Contains("she"))
	}
	if got, ok := tt.Search("new"); !ok || got != 9 {
		t.Errorf("Expected Search(\"new\")=(9, true), got (%d, %v)", got, ok)
	}

	// The zero value unmarshals directly — there are no constructors.
	var zero Tst[int]
	if err := json.Unmarshal([]byte(`{"go":1}`), &zero); err != nil {
		t.Fatalf("json.Unmarshal into the zero value: %v", err)
	}
	if got, ok := zero.Search("go"); !ok || got != 1 {
		t.Errorf("Expected Search(\"go\")=(1, true) on the zero value, got (%d, %v)", got, ok)
	}

	// Value-level UnmarshalJSON is honored.
	var custom Tst[upperString]
	if err := json.Unmarshal([]byte(`{"k":"ab"}`), &custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, ok := custom.Search("k"); !ok || got != upperString("ab") {
		t.Errorf("Expected Search(\"k\")=(\"ab\", true), got (%q, %v)", got, ok)
	}

	// {} and null clear the trie and store nothing.
	for _, data := range []string{"{}", "null"} {
		if err := json.Unmarshal([]byte(data), &tt); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !tt.IsEmpty() || tt.Length() != 0 || tt.root != nil {
			t.Errorf("Expected %s to clear the trie, Length()=%d root=%v", data, tt.Length(), tt.root != nil)
		}
	}

	// The empty key is dropped, matching Insert's rejection of it.
	var ek Tst[int]
	if err := json.Unmarshal([]byte(`{"":7,"a":1}`), &ek); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ek.Length() != 1 || !ek.Contains("a") {
		t.Errorf("Expected the empty key to be dropped and \"a\" kept, Length()=%d", ek.Length())
	}
	checkInvariants(t, &ek)

	// Decode errors leave the trie untouched.
	var keep Tst[int]
	keep.Insert("she", 1)
	keep.Insert("sea", 6)
	for _, data := range []string{
		`{`,                   // malformed
		`[1,2]`,               // not an object
		`{"she":"x"}`,         // wrong value type
		`{"she":1,"sea":"y"}`, // partially wrong value type
		`{"she":1`,            // truncated
	} {
		if err := json.Unmarshal([]byte(data), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", data)
		}
	}
	if keep.Length() != 2 {
		t.Fatalf("Decode errors changed Length(): expected 2, got %d", keep.Length())
	}
	for key, want := range map[string]int{"she": 1, "sea": 6} {
		if got, ok := keep.Search(key); !ok || got != want {
			t.Errorf("After decode errors: expected Search(%q)=(%d, true), got (%d, %v)", key, want, got, ok)
		}
	}
	checkInvariants(t, &keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing values into a nil trie panics with the package's
// insert-contract wording, while {} and null — which store nothing —
// are tolerated even on a nil trie.  The zero value never panics: it is
// fully usable, so storing into it works.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilTst *Tst[int]
	for _, data := range []string{"{}", "null"} {
		if err := nilTst.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil trie to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON on a nil *Tst", "tst_ts: UnmarshalJSON called on a nil Tst", func() {
		_ = nilTst.UnmarshalJSON([]byte(`{"a":1}`))
	})

	var zero Tst[int]
	if err := zero.UnmarshalJSON([]byte(`{"a":1}`)); err != nil {
		t.Errorf("Expected unmarshaling into the zero value to work, got %v", err)
	}
	if !zero.Contains("a") {
		t.Errorf("Expected the zero value to hold \"a\" after unmarshaling.")
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map[string]int reference model at fixed seed: after random
// edits the trie's JSON must decode to exactly the model, and decoding
// a random object into the trie must give the trie exactly that object's
// pairs.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "abcd"
	randKey := func() string {
		n := 1 + rng.Intn(6)
		var sb strings.Builder
		for range n {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}

	var tt Tst[int]
	model := make(map[string]int)

	for step := range 200 {
		key := randKey()
		if rng.Intn(3) == 0 {
			delete(model, key)
			tt.Delete(key)
		} else {
			value := rng.Intn(10000)
			model[key] = value
			tt.Insert(key, value)
		}

		if step%17 != 0 {
			continue
		}

		// Marshal and decode with the plain json package: the result
		// must be exactly the model.
		b, err := json.Marshal(&tt)
		if err != nil {
			t.Fatalf("step %d: json.Marshal: %v", step, err)
		}
		got := make(map[string]int)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("step %d: json.Unmarshal of the marshaled trie: %v", step, err)
		}
		if len(got) != len(model) {
			t.Fatalf("step %d: marshaled trie has %d keys, model has %d", step, len(got), len(model))
		}
		for key, want := range model {
			if v, ok := got[key]; !ok || v != want {
				t.Fatalf("step %d: marshaled trie has %q=(%d, %v), model has (%d, true)", step, key, v, ok, want)
			}
		}

		// Decode a random object into a fresh zero-value trie: the trie
		// must hold exactly the object's pairs.
		fresh := make(map[string]int, 1+rng.Intn(8))
		for range len(fresh) + rng.Intn(4) {
			fresh[randKey()] = rng.Intn(10000)
		}
		b, err = json.Marshal(fresh)
		if err != nil {
			t.Fatalf("step %d: json.Marshal of the model: %v", step, err)
		}
		var decoded Tst[int]
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("step %d: json.Unmarshal into a fresh trie: %v", step, err)
		}
		if decoded.Length() != len(fresh) {
			t.Fatalf("step %d: decoded Length()=%d, object has %d keys", step, decoded.Length(), len(fresh))
		}
		for key, want := range fresh {
			if v, ok := decoded.Search(key); !ok || v != want {
				t.Fatalf("step %d: decoded Search(%q)=(%d, %v), object has (%d, true)", step, key, v, ok, want)
			}
		}
		checkInvariants(t, &decoded)
	}
	checkInvariants(t, &tt)
}

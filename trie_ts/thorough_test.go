package trie_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: structural invariants, the fixed-seed randomized
// property test against a map reference model, snapshot-iterator
// semantics, the Lock/Nl* compound surface, and the concurrent race
// tests.

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// countNodes returns the number of live nodes in the subtrie rooted at x.
func countNodes[T any](x *trieNode[T]) int {
	if x == nil {
		return 0
	}
	n := 1
	for b := 0; b < radix; b++ {
		n += countNodes(x.children[b])
	}
	return n
}

// checkInvariants verifies the trie against the reference model: every
// model key is found by Search with the model's value, Length equals the
// model size, All yields exactly Length pairs in strictly ascending key
// order covering exactly the model keys, and the live node count is at
// least Length (a pruned trie never has fewer nodes than keys).  Call it
// after any structural change.
//
// It reads internals (root) WITHOUT the lock — single-goroutine tests
// only.
func checkInvariants(t *testing.T, tr *Trie[int], model map[string]int) {
	t.Helper()

	// Search finds every model key with the model's value.
	for k, want := range model {
		v, ok := tr.Search(k)
		if !ok || v != want {
			t.Fatalf("Search(%q)=(%d, %v), model says (%d, true)", k, v, ok, want)
		}
	}

	if tr.Length() != len(model) {
		t.Fatalf("Length()=%d, model has %d keys", tr.Length(), len(model))
	}
	if tr.IsEmpty() != (len(model) == 0) {
		t.Fatalf("IsEmpty()=%v, model has %d keys", tr.IsEmpty(), len(model))
	}

	// All yields ascending keys, exactly Length of them, covering the model.
	var keys []string
	for k, v := range tr.All() {
		if len(keys) > 0 && keys[len(keys)-1] >= k {
			t.Fatalf("All yielded keys out of ascending order: %q then %q", keys[len(keys)-1], k)
		}
		want, ok := model[k]
		if !ok {
			t.Fatalf("All yielded %q, which is not in the model", k)
		}
		if v != want {
			t.Fatalf("All yielded (%q, %d), model says %d", k, v, want)
		}
		keys = append(keys, k)
	}
	if len(keys) != tr.Length() {
		t.Fatalf("All yielded %d pairs, Length()=%d", len(keys), tr.Length())
	}

	// A pruned trie never has fewer live nodes than keys.
	if nodes := countNodes(tr.root); nodes < tr.Length() {
		t.Fatalf("node count %d < Length()=%d", nodes, tr.Length())
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic helper
// -------------------------------------------------------------------------------------------------------

// expectPanic runs f and verifies that it panics with a message
// containing wantSub.
func expectPanic(t *testing.T, wantSub string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected a panic containing %q, got none.", wantSub)
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, wantSub) {
			t.Errorf("Expected a panic containing %q, got %v", wantSub, r)
		}
	}()
	f()
}

// TestNilInsertPanicsThorough exercises the panic helper against the
// package's only panic.
func TestNilInsertPanicsThorough(t *testing.T) {
	var nilTrie *Trie[int]
	expectPanic(t, "Insert", func() { nilTrie.Insert("k", 1) })
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// matchPattern reports whether key matches pattern byte for byte, with
// '.' matching any one byte — the obviously-correct reference for
// KeysThatMatch.
func matchPattern(key, pattern string) bool {
	if len(key) != len(pattern) {
		return false
	}
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '.' && pattern[i] != key[i] {
			return false
		}
	}
	return true
}

// sortedModelKeys returns the model's keys in ascending order.
func sortedModelKeys(model map[string]int) []string {
	keys := make([]string, 0, len(model))
	for k := range model {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// longestPrefixModel is the reference for LongestPrefixOf: the longest
// model key that is a prefix of query.
func longestPrefixModel(model map[string]int, query string) string {
	longest := ""
	for k := range model {
		if strings.HasPrefix(query, k) && len(k) > len(longest) {
			longest = k
		}
	}
	return longest
}

func TestTrieRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "abc" // small alphabet so collisions and shared prefixes are common
	var tr Trie[int]
	model := make(map[string]int)

	randomKey := func() string {
		n := rng.Intn(9) // lengths 0..8, including the empty-string key
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}

	verify := func(step int) {
		checkInvariants(t, &tr, model)

		// KeysWithPrefix against the model, with a random short prefix.
		prefix := randomKey()
		var want []string
		for _, k := range sortedModelKeys(model) {
			if strings.HasPrefix(k, prefix) {
				want = append(want, k)
			}
		}
		got := tr.KeysWithPrefix(prefix)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("step %d: KeysWithPrefix(%q)=%v, model says %v", step, prefix, got, want)
		}

		// KeysThatMatch against the model: take a random model key (or a
		// random string) and poke a wildcard into a random position.
		base := randomKey()
		if keys := sortedModelKeys(model); len(keys) > 0 && rng.Intn(2) == 0 {
			base = keys[rng.Intn(len(keys))]
		}
		pattern := base
		if len(pattern) > 0 {
			b := []byte(pattern)
			b[rng.Intn(len(b))] = '.'
			pattern = string(b)
		}
		want = want[:0]
		for _, k := range sortedModelKeys(model) {
			if matchPattern(k, pattern) {
				want = append(want, k)
			}
		}
		got = tr.KeysThatMatch(pattern)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("step %d: KeysThatMatch(%q)=%v, model says %v", step, pattern, got, want)
		}

		// LongestPrefixOf against the model.
		query := randomKey()
		if got, want := tr.LongestPrefixOf(query), longestPrefixModel(model, query); got != want {
			t.Fatalf("step %d: LongestPrefixOf(%q)=%q, model says %q", step, query, got, want)
		}
	}

	for step := range 800 {
		key := randomKey()
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4: // Insert
			value := rng.Intn(10000)
			got := tr.Insert(key, value)
			_, existed := model[key]
			if got != !existed {
				t.Fatalf("step %d: Insert(%q)=%v, model says the key existed=%v", step, key, got, existed)
			}
			model[key] = value
		case 5, 6, 7: // Search / Contains
			v, ok := tr.Search(key)
			want, exists := model[key]
			if ok != exists || (ok && v != want) {
				t.Fatalf("step %d: Search(%q)=(%d, %v), model says (%d, %v)", step, key, v, ok, want, exists)
			}
			if tr.Contains(key) != exists {
				t.Fatalf("step %d: Contains(%q)=%v, model says %v", step, key, tr.Contains(key), exists)
			}
		case 8, 9: // Delete
			got := tr.Delete(key)
			_, existed := model[key]
			if got != existed {
				t.Fatalf("step %d: Delete(%q)=%v, model says %v", step, key, got, existed)
			}
			delete(model, key)
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)

	// Delete everything; the trie must end up empty and fully pruned.
	for _, k := range sortedModelKeys(model) {
		if !tr.Delete(k) {
			t.Fatalf("Delete(%q) failed during the final drain", k)
		}
	}
	if !tr.IsEmpty() || tr.Length() != 0 || tr.root != nil {
		t.Fatalf("Expected an empty, fully pruned trie after draining; IsEmpty=%v Length=%d root=%v",
			tr.IsEmpty(), tr.Length(), tr.root)
	}
}

// -------------------------------------------------------------------------------------------------------
// Snapshot iterators
// -------------------------------------------------------------------------------------------------------

// TestIteratorSnapshotSemantics is the opposite of the plain trie's
// live-walk contract: All operates on a snapshot collected when it is
// called — later modifications, even deleting every key, are not
// observed — and mutating the trie from inside the loop is safe.
// KeysWithPrefix and KeysThatMatch return eager slices, so they are
// snapshots by construction.
func TestIteratorSnapshotSemantics(t *testing.T) {
	var tr Trie[int]
	keys := []string{"sea", "sells", "she", "shells", "shore", "the"}
	for i, k := range keys {
		tr.Insert(k, i)
	}

	all := tr.All()                      // snapshot collected now
	prefixed := tr.KeysWithPrefix("sh")  // eager slice, copied now
	matched := tr.KeysThatMatch("s....") // eager slice, copied now

	// Empty the trie after the snapshots were taken.
	for _, k := range keys {
		tr.Delete(k)
	}
	if !tr.IsEmpty() {
		t.Fatalf("Expected an empty trie after deleting every key.")
	}

	// The All iterator must still yield the 6 pairs captured at call time.
	n := 0
	for range all {
		n++
	}
	if n != len(keys) {
		t.Errorf("All should yield the %d pairs captured at call time, got %d", len(keys), n)
	}
	if len(prefixed) != 3 {
		t.Errorf("KeysWithPrefix should keep the 3 keys captured at call time, got %v", prefixed)
	}
	if len(matched) != 2 { // "sells" and "shore"
		t.Errorf("KeysThatMatch should keep the keys captured at call time, got %v", matched)
	}

	// Mutating the trie from inside the loop body is safe: the loop sees
	// the snapshot.
	for i, k := range keys {
		tr.Insert(k, i)
	}
	visited := 0
	for k := range tr.All() {
		visited++
		tr.Delete(k) // delete each yielded key inside the loop
	}
	if visited != len(keys) {
		t.Errorf("Expected %d visits while deleting during iteration, got %d", len(keys), visited)
	}
	if !tr.IsEmpty() {
		t.Errorf("Expected an empty trie after deleting every yielded key.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Lock + Nl* compound operations
// -------------------------------------------------------------------------------------------------------

// TestLockNlCompound verifies the Lock/Unlock + Nl* escape hatch for
// compound operations: a search followed by a delete (or a bulk insert)
// runs atomically under one lock hold.
func TestLockNlCompound(t *testing.T) {
	var tr Trie[int]
	for i := range 40 {
		tr.Insert(fmt.Sprintf("c%03d", i), i)
	}

	tr.Lock()
	if v, found := tr.NlSearch("c021"); found {
		if v != 21 {
			t.Errorf("NlSearch returned %d, want 21", v)
		}
		if !tr.NlDelete("c021") {
			t.Errorf("NlDelete inside the held lock should succeed")
		}
	} else {
		t.Errorf("NlSearch should have found c021")
	}
	if tr.NlLen() != 39 || tr.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty should report 39/false, got %d/%v", tr.NlLen(), tr.NlIsEmpty())
	}
	// A bulk insert under the same single lock hold.
	for i := 100; i < 150; i++ {
		tr.NlInsert(fmt.Sprintf("c%03d", i), i)
	}
	tr.Unlock()

	if tr.Len() != 89 {
		t.Fatalf("Expected length 89 after the compound section, got %d", tr.Len())
	}
	if _, found := tr.Search("c021"); found {
		t.Errorf("c021 should be gone")
	}
	for i := 100; i < 150; i++ {
		if v, found := tr.Search(fmt.Sprintf("c%03d", i)); !found || v != i {
			t.Errorf("Expected to find c%03d with value %d, got %v found=%v", i, i, v, found)
		}
	}

	model := make(map[string]int)
	for i := range 40 {
		if i != 21 {
			model[fmt.Sprintf("c%03d", i)] = i
		}
	}
	for i := 100; i < 150; i++ {
		model[fmt.Sprintf("c%03d", i)] = i
	}
	checkInvariants(t, &tr, model)
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestTrieConcurrent runs writers inserting disjoint key ranges against
// one shared trie while observers hammer the read operations and the
// snapshot iterators.  It is primarily a test for the race detector
// (`make race`); the final state must be exactly the union of the
// ranges.
func TestTrieConcurrent(t *testing.T) {
	var tr Trie[int]
	const writers = 8
	const perWriter = 200
	const total = writers * perWriter

	key := func(w, i int) string { return fmt.Sprintf("w%d-%04d", w, i) }

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWriter {
				tr.Insert(key(w, i), w*perWriter+i)
			}
		}(w)
	}

	// Observers run until the writers finish.  Searches may hit or miss
	// (writers are in flight); every snapshot must be a consistent,
	// ascending view of at most the total number of keys.
	stop := make(chan struct{})
	var observers sync.WaitGroup
	for range 4 {
		observers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = tr.Search(key(3, 42))
				_ = tr.Contains(key(3, 42))
				_ = tr.IsEmpty()
				_ = tr.Len()
				_ = tr.Length()
				_ = tr.LongestPrefixOf(key(3, 42))
				n := 0
				var prev string
				for k, v := range tr.All() { // snapshot; safe alongside the writers
					if n > 0 && prev >= k {
						t.Errorf("All snapshot out of ascending order: %q then %q", prev, k)
						return
					}
					prev = k
					_ = v
					n++
				}
				if n > total {
					t.Errorf("All yielded %d pairs, more than the %d ever inserted", n, total)
					return
				}
				if got := tr.KeysWithPrefix("w3-"); len(got) > perWriter {
					t.Errorf("KeysWithPrefix(\"w3-\") returned %d keys, more than the %d ever inserted there", len(got), perWriter)
					return
				}
				if got := tr.KeysThatMatch("w3-...."); len(got) > perWriter {
					t.Errorf("KeysThatMatch(\"w3-....\") returned %d keys, more than the %d ever inserted there", len(got), perWriter)
					return
				}
			}
		})
	}

	wg.Wait()
	close(stop)
	observers.Wait()

	if tr.Len() != total {
		t.Fatalf("Expected length %d, got %d", total, tr.Len())
	}
	for w := range writers {
		for i := range perWriter {
			v, found := tr.Search(key(w, i))
			if !found {
				t.Fatalf("Expected to find %q after all writers finished", key(w, i))
			}
			if v != w*perWriter+i {
				t.Fatalf("%q stored %d, want %d", key(w, i), v, w*perWriter+i)
			}
		}
	}
}

// TestTrieConcurrentDelete fills a trie and then deletes disjoint key
// ranges from concurrent goroutines while observers search and iterate.
// After the wait the trie must be empty and every key not-found.
// Race-detector target (`make race`).
func TestTrieConcurrentDelete(t *testing.T) {
	var tr Trie[int]
	const deleters = 8
	const perDeleter = 100
	key := func(d, i int) string { return fmt.Sprintf("d%d-%04d", d, i) }

	for d := range deleters {
		for i := range perDeleter {
			tr.Insert(key(d, i), d*perDeleter+i)
		}
	}

	var wg sync.WaitGroup
	for d := range deleters {
		wg.Add(1)
		go func(d int) {
			defer wg.Done()
			for i := range perDeleter {
				if !tr.Delete(key(d, i)) {
					t.Errorf("Expected to delete %q", key(d, i))
					return
				}
			}
		}(d)
	}

	stop := make(chan struct{})
	var observers sync.WaitGroup
	for range 4 {
		observers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = tr.Search(key(3, 42)) // found or not: both legal mid-flight
				_ = tr.Contains(key(3, 42))
				_ = tr.LongestPrefixOf(key(3, 42))
				for range tr.All() { // snapshot; safe alongside the deleters
				}
				_ = tr.KeysWithPrefix("d3-")
			}
		})
	}

	wg.Wait()
	close(stop)
	observers.Wait()

	if !tr.IsEmpty() || tr.Len() != 0 {
		t.Fatalf("Expected an empty trie after all deleters finished, got length %d", tr.Len())
	}
	if tr.root != nil { // single goroutine again: the trie fully pruned itself
		t.Fatalf("Expected a fully pruned trie (root == nil) after the concurrent drain")
	}
	for d := range deleters {
		for i := range perDeleter {
			if _, found := tr.Search(key(d, i)); found {
				t.Fatalf("%q should be gone", key(d, i))
			}
		}
	}
}

// TestTrieConcurrentCompound mixes compound Lock+Nl sections with plain
// locked operations: writers do atomic read-modify-write counter bumps
// under Lock while other goroutines insert disjoint keys and drain the
// snapshot iterators.  A torn compound would surface as a lost
// increment in the final tally.  Race-detector target (`make race`).
func TestTrieConcurrentCompound(t *testing.T) {
	var tr Trie[int]

	const writers = 4
	const rounds = 200

	var wg sync.WaitGroup
	// Compound writers: atomically bump xNNN's counter (NlSearch +
	// NlInsert under one lock hold).  Each x-key sees exactly
	// writers*(rounds/50) increments.
	for range writers {
		wg.Go(func() {
			for r := range rounds {
				k := fmt.Sprintf("x%03d", r%50)
				tr.Lock()
				v, found := tr.NlSearch(k)
				if found {
					tr.NlInsert(k, v+1)
				} else {
					tr.NlInsert(k, 1)
				}
				tr.Unlock()
			}
		})
	}
	// Plain writers on a disjoint key space, plus snapshot iterators.
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := range rounds {
				tr.Insert(fmt.Sprintf("y%d-%03d", w, r%50), r)
				for range tr.All() {
				}
			}
		}(w)
	}
	wg.Wait()

	if tr.Len() != 150 { // 50 x-keys + 2*50 y-keys
		t.Fatalf("Expected length 150, got %d", tr.Len())
	}
	for i := range 50 {
		want := writers * (rounds / 50)
		if v, found := tr.Search(fmt.Sprintf("x%03d", i)); !found || v != want {
			t.Errorf("x%03d: expected counter %d, got (%d, %v) — a torn compound lost increments", i, want, v, found)
		}
	}
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
	// Exact object output, members in ascending key order regardless of
	// insertion order — the trie's natural iteration order.
	var tr Trie[int]
	tr.Insert("shells", 3)
	tr.Insert("she", 0)
	tr.Insert("sea", 2)
	tr.Insert("", 9) // the empty-string key at the root sorts first
	b, err := json.Marshal(&tr)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"":9,"sea":2,"she":0,"shells":3}` {
		t.Errorf(`Expected {"":9,"sea":2,"she":0,"shells":3}, got %s`, b)
	}

	// Struct values use their normal JSON encoding.
	var items Trie[struct {
		S string
	}]
	items.Insert("a", struct{ S string }{"x"})
	if b, err := json.Marshal(&items); err != nil || string(b) != `{"a":{"S":"x"}}` {
		t.Errorf(`Expected {"a":{"S":"x"}}, got (%s, %v)`, b, err)
	}

	// A value type with its own MarshalJSON is honored.
	var upper Trie[upperString]
	upper.Insert("k", "v")
	if b, err := json.Marshal(&upper); err != nil || string(b) != `{"k":"V"}` {
		t.Errorf(`Expected {"k":"V"}, got (%s, %v)`, b, err)
	}

	// An empty trie marshals as {}.
	var empty Trie[int]
	if b, err := json.Marshal(&empty); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for an empty trie, got (%s, %v)", b, err)
	}

	// A direct call on a nil trie encodes as {}; json.Marshal on a nil
	// *Trie never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTrie *Trie[int]
	if b, err := nilTrie.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} from a direct nil-trie call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTrie); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil trie, got (%s, %v)", b, err)
	}

	// A value that cannot be encoded (a channel) surfaces the json
	// package's error.
	var bad Trie[chan int]
	bad.Insert("c", make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a trie of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded pairs replace the contents; lookup finds every one.
	var tr Trie[int]
	if err := json.Unmarshal([]byte(`{"sea":2,"she":0,"shells":3}`), &tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, &tr, map[string]int{"sea": 2, "she": 0, "shells": 3})

	// Round trip: marshal, unmarshal into a second trie, marshal again —
	// the byte output is identical because the key order is canonical.
	var tr2 Trie[int]
	b1, err := json.Marshal(&tr)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := json.Unmarshal(b1, &tr2); err != nil {
		t.Fatalf("json.Unmarshal round trip: %v", err)
	}
	b2, err := json.Marshal(&tr2)
	if err != nil {
		t.Fatalf("json.Marshal round trip: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("Round trip changed the encoding: %s then %s", b1, b2)
	}

	// Unmarshaling replaces the old contents, it does not merge.
	if err := json.Unmarshal([]byte(`{"by":4}`), &tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, &tr, map[string]int{"by": 4})

	// A value type with its own UnmarshalJSON is honored.
	var upper Trie[upperString]
	if err := json.Unmarshal([]byte(`{"k":"v"}`), &upper); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := upper.Search("k"); !ok || v != "v" {
		t.Errorf(`Expected Search("k")=("v", true) after unmarshal, got (%q, %v)`, v, ok)
	}

	// An empty object and null clear the trie.
	for _, data := range []string{"{}", "null"} {
		if err := json.Unmarshal([]byte(data), &tr); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !tr.IsEmpty() {
			t.Errorf("Expected an empty trie after unmarshaling %s.", data)
		}
	}
}

// TestUnmarshalJSONErrors verifies that a decode error — malformed JSON,
// a non-object document, or a wrong value type — is returned with the
// trie untouched.
func TestUnmarshalJSONErrors(t *testing.T) {
	var tr Trie[int]
	tr.Insert("keep", 1)

	for _, data := range []string{
		`{bad`,        // malformed
		`[1,2]`,       // an array, not an object
		`"str"`,       // a scalar, not an object
		`{"k":"nah"}`, // wrong value type for int
	} {
		if err := json.Unmarshal([]byte(data), &tr); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", data)
		}
	}
	checkInvariants(t, &tr, map[string]int{"keep": 1})
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing values into a nil trie panics with a message naming
// the method, while {} and null — which store nothing — are tolerated
// everywhere.  The zero value is ready to use, so unmarshaling into one
// works without any constructor.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Trie[int]
	if err := zero.UnmarshalJSON([]byte(`{"go":1}`)); err != nil {
		t.Errorf("Expected a zero-value trie to accept unmarshal, got %v", err)
	}
	if v, ok := zero.Search("go"); !ok || v != 1 {
		t.Errorf(`Expected Search("go")=(1, true) on the zero-value trie, got (%d, %v)`, v, ok)
	}

	var nilTrie *Trie[int]
	for _, data := range []string{"{}", "null"} {
		if err := nilTrie.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil trie to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"go":1}`))
	})
	expectPanic(t, "nil Trie", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"go":1}`))
	})
}

// TestJSONStructField marshals and unmarshals a Trie nested in a struct
// through the encoding/json package.  Unlike the constructor-based
// structures, a nil *Trie field works here: the json package allocates
// a zero-value trie itself, and the zero value is ready to use.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string       `json:"title"`
		Tags  *Trie[int]   `json:"tags"`
		Meta  Trie[string] `json:"meta"`
	}

	doc := Doc{Title: "x", Tags: &Trie[int]{}}
	doc.Tags.Insert("a", 1)
	doc.Tags.Insert("b", 2)
	doc.Meta.Insert("k", "v")

	b, err := json.Marshal(&doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"x","tags":{"a":1,"b":2},"meta":{"k":"v"}}` {
		t.Errorf("Unexpected struct encoding: %s", b)
	}

	var back Doc
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back.Tags == nil {
		t.Fatalf("Expected the json package to allocate the *Trie field.")
	}
	checkInvariants(t, back.Tags, map[string]int{"a": 1, "b": 2})
	if v, ok := back.Meta.Search("k"); !ok || v != "v" {
		t.Errorf(`Expected Meta to hold "k"->"v", got (%q, %v)`, v, ok)
	}

	// A nil *Trie field also decodes: the allocated zero value is ready
	// to use.
	var fresh Doc
	if err := json.Unmarshal([]byte(`{"title":"y","tags":{"z":26},"meta":{}}`), &fresh); err != nil {
		t.Fatalf("json.Unmarshal into a nil *Trie field: %v", err)
	}
	if v, ok := fresh.Tags.Search("z"); !ok || v != 26 {
		t.Errorf(`Expected Tags to hold "z"->26, got (%d, %v)`, v, ok)
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map reference model at fixed seed: the trie's JSON is the
// model's JSON, and unmarshaling the model's JSON rebuilds the model.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run

	const alphabet = "abc"
	var tr Trie[int]
	model := make(map[string]int)

	randomKey := func() string {
		n := rng.Intn(7) // lengths 0..6, including the empty-string key
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}

	for step := range 300 {
		key := randomKey()
		value := rng.Intn(10000)
		tr.Insert(key, value)
		model[key] = value

		if step%41 == 0 || step == 299 {
			// Marshal the trie and the model; decode both into maps and
			// compare — the documents must carry the same pairs.
			tb, err := json.Marshal(&tr)
			if err != nil {
				t.Fatalf("step %d: json.Marshal(trie): %v", step, err)
			}
			mb, err := json.Marshal(model)
			if err != nil {
				t.Fatalf("step %d: json.Marshal(model): %v", step, err)
			}
			var fromTrie, fromModel map[string]int
			if err := json.Unmarshal(tb, &fromTrie); err != nil {
				t.Fatalf("step %d: decoding the trie's JSON: %v", step, err)
			}
			if err := json.Unmarshal(mb, &fromModel); err != nil {
				t.Fatalf("step %d: decoding the model's JSON: %v", step, err)
			}
			if len(fromTrie) != len(fromModel) {
				t.Fatalf("step %d: trie JSON has %d pairs, model has %d", step, len(fromTrie), len(fromModel))
			}
			for k, v := range fromModel {
				if got, ok := fromTrie[k]; !ok || got != v {
					t.Fatalf("step %d: trie JSON has %q->%d (%v), model says %d", step, k, got, ok, v)
				}
			}

			// Unmarshal the model's JSON into a fresh trie; it must match
			// the model exactly.
			var rebuilt Trie[int]
			if err := json.Unmarshal(mb, &rebuilt); err != nil {
				t.Fatalf("step %d: json.Unmarshal(model JSON): %v", step, err)
			}
			checkInvariants(t, &rebuilt, model)
		}
	}
}

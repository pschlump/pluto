package trie

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
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
	// Exact object output; the json package emits members in sorted key
	// order, which is the trie's natural iteration order.
	var tr Trie[int]
	tr.Insert("she", 0)
	tr.Insert("by", 1)
	tr.Insert("sea", 2)
	b, err := json.Marshal(&tr)
	if err != nil {
		t.Fatalf("json.Marshal(&tr): %v", err)
	}
	if string(b) != `{"by":1,"sea":2,"she":0}` {
		t.Errorf(`Expected {"by":1,"sea":2,"she":0}, got %s`, b)
	}

	// The empty-string key is just another object member.
	var emptyKey Trie[int]
	emptyKey.Insert("", 42)
	if b, err := json.Marshal(&emptyKey); err != nil || string(b) != `{"":42}` {
		t.Errorf(`Expected {"":42}, got (%s, %v)`, b, err)
	}

	// An empty trie encodes as {}.
	if b, err := json.Marshal(&Trie[int]{}); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for an empty trie, got (%s, %v)", b, err)
	}

	// A zero-value trie is a tolerated read: {}.
	var zero Trie[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for a zero-value trie, got (%s, %v)", b, err)
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

	// Value-level marshalers are honored.
	var custom Trie[upperString]
	custom.Insert("k1", "x")
	custom.Insert("k2", "y")
	if b, err := json.Marshal(&custom); err != nil || string(b) != `{"k1":"X","k2":"Y"}` {
		t.Errorf(`Expected {"k1":"X","k2":"Y"}, got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	var bad Trie[chan int]
	bad.Insert("c", make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a trie of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded pairs land under their keys; iteration is ascending.
	var tr Trie[int]
	if err := json.Unmarshal([]byte(`{"she":0,"by":1,"sea":2}`), &tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []string
	for k, v := range tr.All() {
		got = append(got, fmt.Sprintf("%s:%d", k, v))
	}
	if fmt.Sprint(got) != "[by:1 sea:2 she:0]" {
		t.Errorf("Expected [by:1 sea:2 she:0], got %v", got)
	}

	// A round trip rebuilds a structurally sound trie.
	var items Trie[int]
	items.Insert("a", 1)
	items.Insert("b", 2)
	items.Insert("c", 3)
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Trie[int]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, &again, map[string]int{"a": 1, "b": 2, "c": 3})
	if v, ok := again.Search("b"); !ok || v != 2 {
		t.Errorf("Expected Search to work after unmarshal, got (%d, %v)", v, ok)
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`{"z":7}`), &tr); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if tr.Length() != 1 || tr.Contains("by") {
		t.Errorf("Expected replacement, got Length()=%d, Contains(\"by\")=%v", tr.Length(), tr.Contains("by"))
	}

	// An empty object and null clear the trie, fully pruning it.
	var full Trie[int]
	full.Insert("z", 1)
	if err := json.Unmarshal([]byte("{}"), &full); err != nil {
		t.Fatalf("json.Unmarshal({}): %v", err)
	}
	if !full.IsEmpty() || full.root != nil {
		t.Errorf("Expected {} to clear the trie.")
	}
	full.Insert("z", 1)
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() || full.root != nil {
		t.Errorf("Expected null to clear the trie.")
	}

	// Value-level unmarshalers are honored.
	var custom Trie[upperString]
	if err := json.Unmarshal([]byte(`{"k1":"X","k2":"Y"}`), &custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := custom.Search("k2"); !ok || string(v) != "Y" {
		t.Errorf("Expected Search(\"k2\")=(Y, true), got (%q, %v)", v, ok)
	}

	// Decode errors are returned and leave the trie untouched.
	var keep Trie[int]
	keep.Insert("keep", 9)
	for _, badData := range []string{"{", `{"a":`, `["x"]`, "7", `{"a":"x"}`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if v, ok := keep.Search("keep"); !ok || v != 9 || keep.Length() != 1 {
			t.Errorf("Trie changed after the error on %s.", badData)
		}
	}
	checkInvariants(t, &keep, map[string]int{"keep": 9})
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing values into a nil trie panics with a message naming
// the method, while {} and null — which store nothing — are tolerated
// everywhere.  Unlike the constructor-built structures, a zero-value
// trie is ready to use and accepts elements directly.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Trie[int]
	for _, data := range []string{"{}", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value trie to be tolerated, got %v", data, err)
		}
	}

	// The zero value stores elements without any constructor.
	if err := zero.UnmarshalJSON([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("Expected a zero-value trie to accept elements, got %v", err)
	}
	if v, ok := zero.Search("a"); !ok || v != 1 {
		t.Errorf("Expected Search(\"a\")=(1, true) on a zero-value trie, got (%d, %v)", v, ok)
	}

	var nilTrie *Trie[int]
	for _, data := range []string{"{}", "null"} {
		if err := nilTrie.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil trie to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"a":1}`))
	})
	expectPanic(t, "nil Trie", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"a":1}`))
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run

	const alphabet = "abc"
	var tr Trie[int]
	model := make(map[string]int)

	randomKey := func() string {
		n := rng.Intn(9)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteByte(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}

	for step := range 500 {
		switch rng.Intn(4) {
		case 0, 1: // Insert
			key, value := randomKey(), rng.Intn(10000)
			tr.Insert(key, value)
			model[key] = value
		case 2: // Delete
			key := randomKey()
			tr.Delete(key)
			delete(model, key)
		case 3: // JSON round trip through the model
			b, err := json.Marshal(&tr)
			if err != nil {
				t.Fatalf("step %d: json.Marshal: %v", step, err)
			}
			var want []byte
			if len(model) == 0 {
				want = []byte("{}")
			} else {
				var err error
				if want, err = json.Marshal(model); err != nil {
					t.Fatalf("step %d: json.Marshal(model): %v", step, err)
				}
			}
			if string(b) != string(want) {
				t.Fatalf("step %d: marshaled %s, model says %s", step, b, want)
			}
			var back Trie[int]
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("step %d: json.Unmarshal: %v", step, err)
			}
			checkInvariants(t, &back, model)
		}
	}
	checkInvariants(t, &tr, model)
}

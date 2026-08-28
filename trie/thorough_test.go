package trie

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
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

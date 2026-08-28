package tst

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

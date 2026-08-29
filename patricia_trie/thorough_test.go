package patricia_trie

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

// countNodes returns the number of leaf and internal nodes in the
// subtree rooted at x.
func countNodes[T any](x *patriciaNode[T]) (leaves, internal int) {
	if x == nil {
		return 0, 0
	}
	if x.bit < 0 {
		return 1, 0
	}
	l0, i0 := countNodes(x.child[0])
	l1, i1 := countNodes(x.child[1])
	return l0 + l1, i0 + i1 + 1
}

// checkStructure verifies the Patricia shape of the subtree rooted at
// x: internal nodes have two non-nil children (path compression — no
// one-child internal nodes), and critical bit indexes strictly increase
// along every root-to-leaf path (minBit is the parent's bit index).
// It returns the leaf and internal node counts.
func checkStructure[T any](t *testing.T, x *patriciaNode[T], minBit int) (leaves, internal int) {
	t.Helper()
	if x == nil {
		t.Fatalf("nil node linked into the trie")
	}
	if x.bit < 0 {
		return 1, 0
	}
	if x.bit <= minBit {
		t.Fatalf("bit indexes not strictly increasing: node bit %d after parent bit %d", x.bit, minBit)
	}
	if x.child[0] == nil || x.child[1] == nil {
		t.Fatalf("unary internal node at bit %d — path compression violated", x.bit)
	}
	l0, i0 := checkStructure(t, x.child[0], x.bit)
	l1, i1 := checkStructure(t, x.child[1], x.bit)
	return l0 + l1, i0 + i1 + 1
}

// checkInvariants verifies the trie against the reference model: every
// model key is found by Search with the model's value, Length equals
// the model size, All yields exactly Length pairs in strictly ascending
// key order and Backward in strictly descending order, both covering
// exactly the model keys, and the structure is a proper Patricia trie —
// no unary internal nodes, strictly increasing bit indexes, and exactly
// Length leaves with Length-1 internal nodes (root == nil iff empty).
// Call it after any structural change.
func checkInvariants(t *testing.T, pt *PatriciaTrie[int], model map[string]int) {
	t.Helper()

	// Search finds every model key with the model's value.
	for k, want := range model {
		v, ok := pt.Search(k)
		if !ok || v != want {
			t.Fatalf("Search(%q)=(%d, %v), model says (%d, true)", k, v, ok, want)
		}
	}

	if pt.Length() != len(model) {
		t.Fatalf("Length()=%d, model has %d keys", pt.Length(), len(model))
	}
	if pt.IsEmpty() != (len(model) == 0) {
		t.Fatalf("IsEmpty()=%v, model has %d keys", pt.IsEmpty(), len(model))
	}

	// All yields ascending keys, exactly Length of them, covering the model.
	var keys []string
	for k, v := range pt.All() {
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
	if len(keys) != pt.Length() {
		t.Fatalf("All yielded %d pairs, Length()=%d", len(keys), pt.Length())
	}

	// Backward yields the same keys in strictly descending order.
	var backKeys []string
	for k := range pt.Backward() {
		if len(backKeys) > 0 && backKeys[len(backKeys)-1] <= k {
			t.Fatalf("Backward yielded keys out of descending order: %q then %q", backKeys[len(backKeys)-1], k)
		}
		backKeys = append(backKeys, k)
	}
	slices.Reverse(backKeys)
	if slices.Compare(keys, backKeys) != 0 {
		t.Fatalf("Backward keys %q are not the reverse of All keys %q", backKeys, keys)
	}

	// Structural invariants: node counts and shape.
	if pt.root == nil {
		if pt.Length() != 0 {
			t.Fatalf("root == nil but Length()=%d", pt.Length())
		}
		return
	}
	leaves, internal := checkStructure(t, pt.root, -1)
	if leaves != pt.Length() {
		t.Fatalf("leaf count %d != Length()=%d", leaves, pt.Length())
	}
	if internal != pt.Length()-1 {
		t.Fatalf("internal node count %d != Length()-1=%d", internal, pt.Length()-1)
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
	var nilTrie *PatriciaTrie[int]
	expectPanic(t, "Insert", func() { nilTrie.Insert("k", 1) })
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// sortedModelKeys returns the model's keys in ascending order.
func sortedModelKeys(model map[string]int) []string {
	keys := make([]string, 0, len(model))
	for k := range model {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestPatriciaTrieRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const alphabet = "abc" // small alphabet so collisions and shared prefixes are common
	var pt PatriciaTrie[int]
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
		checkInvariants(t, &pt, model)

		// KeysWithPrefix against the model, with a random short prefix.
		prefix := randomKey()
		var want []string
		for _, k := range sortedModelKeys(model) {
			if strings.HasPrefix(k, prefix) {
				want = append(want, k)
			}
		}
		got := pt.KeysWithPrefix(prefix)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("step %d: KeysWithPrefix(%q)=%v, model says %v", step, prefix, got, want)
		}
	}

	for step := range 800 {
		key := randomKey()
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4: // Insert
			value := rng.Intn(10000)
			got := pt.Insert(key, value)
			_, existed := model[key]
			if got != !existed {
				t.Fatalf("step %d: Insert(%q)=%v, model says the key existed=%v", step, key, got, existed)
			}
			model[key] = value
		case 5, 6, 7: // Search / Contains
			v, ok := pt.Search(key)
			want, exists := model[key]
			if ok != exists || (ok && v != want) {
				t.Fatalf("step %d: Search(%q)=(%d, %v), model says (%d, %v)", step, key, v, ok, want, exists)
			}
			if pt.Contains(key) != exists {
				t.Fatalf("step %d: Contains(%q)=%v, model says %v", step, key, pt.Contains(key), exists)
			}
		case 8, 9: // Delete
			got := pt.Delete(key)
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

	// Delete everything; the trie must end up empty and fully collapsed.
	for _, k := range sortedModelKeys(model) {
		if !pt.Delete(k) {
			t.Fatalf("Delete(%q) failed during the final drain", k)
		}
	}
	if !pt.IsEmpty() || pt.Length() != 0 || pt.root != nil {
		t.Fatalf("Expected an empty, fully collapsed trie after draining; IsEmpty=%v Length=%d root=%v",
			pt.IsEmpty(), pt.Length(), pt.root)
	}
}

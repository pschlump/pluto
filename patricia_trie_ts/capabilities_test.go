/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package patricia_trie_ts

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Tests for the set-convenience capabilities (capabilities.go): the
// go-set translation for the string-keyed tries.  The reference model
// is a map[string]int; Go's native string < is byte-wise, so
// slices.Sort over the model keys is the exact ascending-keys oracle —
// no approximating comparator anywhere.

func TestCapabilitiesKeys(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	for _, k := range []string{"she", "shells", "sea", "by", "the", "shore"} {
		tr.Insert(k, len(k))
	}
	got := tr.Keys()
	want := []string{"by", "sea", "she", "shells", "shore", "the"}
	if !slices.Equal(got, want) {
		t.Fatalf("Keys()=%q, want %q", got, want)
	}

	// Byte order, not rune order: the first byte of "é" (0xC3) sorts
	// after 'z', and the empty string sorts first.
	tr2 := &PatriciaTrie[int]{}
	for _, k := range []string{"z", "é", "", "a"} {
		tr2.Insert(k, 0)
	}
	if got, want := tr2.Keys(), []string{"", "a", "z", "é"}; !slices.Equal(got, want) {
		t.Fatalf("Keys()=%q, want %q (byte order)", got, want)
	}
}

func TestCapabilitiesValues(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	for _, k := range []string{"she", "shells", "sea"} {
		tr.Insert(k, len(k))
	}
	got := tr.Values()
	want := []int{3, 3, 6} // values of "sea", "she", "shells" — key order
	if !slices.Equal(got, want) {
		t.Fatalf("Values()=%v, want %v", got, want)
	}

	// Keys and Values are index-aligned.
	keys := tr.Keys()
	for i := range keys {
		if v, ok := tr.Search(keys[i]); !ok || v != got[i] {
			t.Fatalf("Keys()[%d]=%q does not match Values()[%d]=%d", i, keys[i], i, got[i])
		}
	}
}

func TestCapabilitiesStringDeterminism(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	for _, k := range []string{"she", "shells", "sea"} {
		tr.Insert(k, 0)
	}
	want := "[sea she shells]" // values are not rendered
	for i := 0; i < 3; i++ {
		if got := tr.String(); got != want {
			t.Fatalf("String()=%q, want %q", got, want)
		}
	}
	if got := (&PatriciaTrie[int]{}).String(); got != "[]" {
		t.Fatalf("empty trie String()=%q, want []", got)
	}
}

func TestCapabilitiesStringFunc(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	for _, k := range []string{"b", "a"} {
		tr.Insert(k, 0)
	}
	got := tr.StringFunc(func(key string) string {
		return fmt.Sprintf("<%s>", strings.ToUpper(key))
	})
	if want := "[<A> <B>]"; got != want {
		t.Fatalf("StringFunc()=%q, want %q", got, want)
	}
	if got := tr.StringFunc(nil); got != "[]" {
		t.Fatalf("nil format StringFunc()=%q, want []", got)
	}
}

func TestCapabilitiesClone(t *testing.T) {
	model := map[string]int{"she": 3, "shells": 6, "sea": 3, "": 0}
	tr := &PatriciaTrie[int]{}
	for k, v := range model {
		tr.Insert(k, v)
	}
	dup := tr.Clone()

	checkInvariants(t, dup, model) // the copy satisfies every invariant
	if !tr.Equal(dup) || !dup.Equal(tr) {
		t.Fatal("clone is not Equal to the original (keys)")
	}

	// Deep copy: mutating either side leaves the other untouched.
	dup.Insert("extra", 1)
	dup.Delete("she")
	if tr.Length() != len(model) {
		t.Fatalf("original Length()=%d after mutating the clone, want %d", tr.Length(), len(model))
	}
	if !tr.Contains("she") || tr.Contains("extra") {
		t.Fatal("mutating the clone changed the original")
	}
	tr.Delete("sea")
	if !dup.Contains("sea") {
		t.Fatal("mutating the original changed the clone")
	}

	// A nil trie clones to nil; an empty trie clones to an empty trie.
	var nilTrie *PatriciaTrie[int]
	if dup := nilTrie.Clone(); dup != nil {
		t.Fatalf("nil Clone()=%v, want nil", dup)
	}
	if dup := (&PatriciaTrie[int]{}).Clone(); dup.Length() != 0 {
		t.Fatalf("empty Clone() has Length %d, want 0", dup.Length())
	}
}

func TestCapabilitiesEqual(t *testing.T) {
	a := &PatriciaTrie[int]{}
	b := &PatriciaTrie[int]{}
	for _, k := range []string{"she", "shells", "sea"} {
		a.Insert(k, 1)
	}
	for _, k := range []string{"sea", "shells", "she"} { // different order
		b.Insert(k, 2) // different values
	}
	if !a.Equal(b) || !b.Equal(a) {
		t.Fatal("Equal is keys-only: same keys, different values must be Equal")
	}

	// Different key sets are not equal, either way.
	b.Delete("she")
	b.Insert("shore", 2)
	if a.Equal(b) || b.Equal(a) {
		t.Fatal("tries with different key sets must not be Equal")
	}

	// A key that is only a prefix is still a different key set.
	c := &PatriciaTrie[int]{}
	c.Insert("she", 1)
	c.Insert("shells", 1)
	d := &PatriciaTrie[int]{}
	d.Insert("she", 1)
	d.Insert("shel", 1)
	if c.Equal(d) || d.Equal(c) {
		t.Fatal("prefix-vs-key difference must not be Equal")
	}

	// Empty tries, nil tries and zero values are all equal.
	var nilTrie *PatriciaTrie[int]
	if !a.Equal(a) || !nilTrie.Equal(&PatriciaTrie[int]{}) || !(&PatriciaTrie[int]{}).Equal(nilTrie) {
		t.Fatal("nil/empty tries must be Equal")
	}
	if a.Equal(nilTrie) || nilTrie.Equal(a) {
		t.Fatal("a non-empty trie must not Equal a nil trie")
	}
}

func TestCapabilitiesContainsBatches(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	for _, k := range []string{"she", "shells", "sea"} {
		tr.Insert(k, 0)
	}
	if !tr.ContainsSlice([]string{"she", "sea"}) {
		t.Fatal("ContainsSlice must hold for stored keys")
	}
	if tr.ContainsSlice([]string{"she", "shell"}) {
		t.Fatal("ContainsSlice must fail for a prefix that is not a key")
	}
	if !tr.ContainsSlice(nil) || !tr.ContainsSlice([]string{}) {
		t.Fatal("an empty batch is vacuously contained")
	}
	if !tr.ContainsAny([]string{"shell", "shells", "x"}) {
		t.Fatal("ContainsAny must hold when one key is stored")
	}
	if tr.ContainsAny([]string{"shell", "x"}) {
		t.Fatal("ContainsAny must fail when no key is stored")
	}
	if tr.ContainsAny(nil) {
		t.Fatal("an empty batch contains nothing")
	}

	// The empty string is a real key at the root.
	tr.Insert("", 0)
	if !tr.ContainsSlice([]string{""}) {
		t.Fatal("the empty string is a stored key")
	}

	var nilTrie *PatriciaTrie[int]
	if nilTrie.ContainsSlice([]string{"she"}) {
		t.Fatal("a nil trie contains nothing")
	}
	if !nilTrie.ContainsSlice(nil) {
		t.Fatal("an empty batch is vacuously contained, even on nil")
	}
	if nilTrie.ContainsAny([]string{"she"}) {
		t.Fatal("a nil trie contains nothing")
	}
}

func TestCapabilitiesDeleteBatches(t *testing.T) {
	tr := &PatriciaTrie[int]{}
	model := map[string]int{}
	for _, k := range []string{"she", "shells", "sea", "by", "the"} {
		tr.Insert(k, len(k))
		model[k] = len(k)
	}
	changed := tr.DeleteSlice([]string{"she", "nope", "the", "the"})
	if !changed {
		t.Fatal("DeleteSlice must report true when at least one key was present")
	}
	delete(model, "she")
	delete(model, "the")
	checkInvariants(t, tr, model)

	if tr.DeleteSlice([]string{"nope"}) {
		t.Fatal("DeleteSlice must report false when nothing was present")
	}
	if tr.DeleteSlice(nil) {
		t.Fatal("DeleteSlice(nil) must report false")
	}
	if !slices.Equal(tr.Keys(), []string{"by", "sea", "shells"}) {
		t.Fatalf("Keys()=%q after the batch delete", tr.Keys())
	}

	var nilTrie *PatriciaTrie[int]
	if nilTrie.DeleteSlice([]string{"she"}) {
		t.Fatal("DeleteSlice on a nil trie must report false")
	}
}

func TestCapabilitiesNilAndZeroValue(t *testing.T) {
	var nilTrie *PatriciaTrie[int]
	zero := &PatriciaTrie[int]{}
	for _, tr := range []*PatriciaTrie[int]{nilTrie, zero} {
		where := "zero value"
		if tr == nil {
			where = "nil trie"
		}
		if keys := tr.Keys(); keys != nil {
			t.Fatalf("%s: Keys()=%q, want nil", where, keys)
		}
		if values := tr.Values(); values != nil {
			t.Fatalf("%s: Values()=%v, want nil", where, values)
		}
		if got := tr.String(); got != "[]" {
			t.Fatalf("%s: String()=%q, want []", where, got)
		}
		if got := tr.StringFunc(func(string) string { return "x" }); got != "[]" {
			t.Fatalf("%s: StringFunc()=%q, want []", where, got)
		}
		if !tr.Equal(&PatriciaTrie[int]{}) {
			t.Fatalf("%s: Equal(empty) must hold", where)
		}
		if tr.Length() != 0 {
			t.Fatalf("%s: Length()=%d, want 0", where, tr.Length())
		}
	}
	if dup := nilTrie.Clone(); dup != nil {
		t.Fatalf("nil trie: Clone()=%v, want nil", dup)
	}
	if dup := zero.Clone(); dup == nil || dup.Length() != 0 || dup.String() != "[]" {
		t.Fatalf("zero value: Clone()=%v, want a fresh empty trie", dup)
	}
}

func TestCapabilitiesRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 42))
	tr := &PatriciaTrie[int]{}
	model := map[string]int{}
	alphabet := []string{"a", "b", "c", "ab", "abc", "ba", "", "z", "az", "za", "aa", "zz", "é", "aé", "éa"}

	step := func() {
		k := alphabet[rng.IntN(len(alphabet))]
		switch rng.IntN(3) {
		case 0, 1:
			v := rng.IntN(1000)
			tr.Insert(k, v)
			model[k] = v
		case 2:
			if tr.Delete(k) {
				delete(model, k)
			} else if _, ok := model[k]; ok {
				t.Fatalf("Delete(%q)=false, model says present", k)
			}
		}
	}
	for i := 0; i < 800; i++ {
		step()
		if i%37 == 0 {
			checkInvariants(t, tr, model)
			wantKeys := slices.Sorted(maps.Keys(model))
			if got := tr.Keys(); !slices.Equal(got, wantKeys) {
				t.Fatalf("step %d: Keys()=%q, model says %q", i, got, wantKeys)
			}
			values := tr.Values()
			if len(values) != len(wantKeys) {
				t.Fatalf("step %d: len(Values)=%d, want %d", i, len(values), len(wantKeys))
			}
			for j, k := range wantKeys {
				if v, ok := tr.Search(k); !ok || v != values[j] {
					t.Fatalf("step %d: Values[%d] does not match key %q", i, j, k)
				}
			}
		}
	}
	checkInvariants(t, tr, model)

	// Clone and Equal against an independently built trie over the same
	// keys with different values — keys-only equality must hold.
	other := &PatriciaTrie[int]{}
	for k := range model {
		other.Insert(k, -1)
	}
	if !tr.Equal(other) || !other.Equal(tr) {
		t.Fatal("Equal must be keys-only against an independently built trie")
	}
	dup := tr.Clone()
	checkInvariants(t, dup, model)
	if !dup.Equal(tr) {
		t.Fatal("clone must be Equal to the original")
	}

	// Random batches against the model.
	allKeys := slices.Sorted(maps.Keys(model))
	rng.Shuffle(len(allKeys), func(i, j int) { allKeys[i], allKeys[j] = allKeys[j], allKeys[i] })
	half := len(allKeys) / 2
	if !tr.ContainsSlice(allKeys[:half]) || tr.ContainsSlice(append([]string{"no-such-key"}, allKeys[:half]...)) {
		t.Fatal("ContainsSlice disagrees with the model")
	}
	if !tr.ContainsAny(allKeys[len(allKeys)-1:]) {
		t.Fatal("ContainsAny must find a stored key")
	}
	if !tr.DeleteSlice(allKeys[:half]) {
		t.Fatal("DeleteSlice must report true for stored keys")
	}
	for _, k := range allKeys[:half] {
		delete(model, k)
	}
	checkInvariants(t, tr, model)
	if want := slices.Sorted(maps.Keys(model)); !slices.Equal(tr.Keys(), want) {
		t.Fatalf("Keys()=%q after batch delete, model says %q", tr.Keys(), want)
	}
}

// ---------------------------------------------------------------------------
// Concurrency (the _ts additions) — `go test -race` is the real
// assertion: no data race, no deadlock, and atomic batches.
// ---------------------------------------------------------------------------

// TestCapabilitiesConcurrent runs the new methods from several goroutines
// at once — `go test -race` is the real assertion here (no data race and
// no deadlock), the final-state check just verifies the writes all
// landed.
func TestCapabilitiesConcurrent(t *testing.T) {
	tree := &PatriciaTrie[int]{}
	tree.Insert("seed", 0) // a little initial content

	const workers = 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				k := fmt.Sprintf("k%06d", base+i*workers)
				switch i % 8 {
				case 0, 1, 2:
					tree.Insert(k, i)
				case 3:
					tree.Insert(k, i) // duplicate insert: value replace
					tree.Insert(k+"x", i)
				case 4:
					tree.Delete(k)
					tree.DeleteSlice([]string{k + "x"})
				case 5:
					tree.Delete(k + "y")
				case 6:
					_ = tree.Contains(k)
					_ = tree.ContainsSlice([]string{k, k + "x"})
					_ = tree.ContainsAny([]string{k, k + "z"})
				case 7:
					_ = tree.Len()
					_ = tree.IsEmpty()
					_ = tree.String()
					_ = tree.Keys()
					_ = tree.Values()
					_ = tree.Clone().Len()
				}
			}
		}(w * 100000)
	}
	wg.Wait()

	if tree.Len() == 0 {
		t.Errorf("expected residual content after the concurrent mix")
	}
	for _, v := range tree.All() { // snapshot iterator: safe after the wait
		_ = v
	}
}

// TestCapabilitiesBatchAtomicity verifies that a batch delete holds the
// write lock for the whole batch: a concurrent reader observes either
// none or all of the batch, never a partial application.
func TestCapabilitiesBatchAtomicity(t *testing.T) {
	tree := &PatriciaTrie[int]{}
	const batch = 5000
	keys := make([]string, batch)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%06d", i)
		tree.Insert(keys[i], i)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tree.DeleteSlice(keys)
	}()

	observed := map[int]int{}
	for running := true; running; {
		n := tree.Len() // read lock; blocks while the batch holds the write lock
		observed[n]++
		select {
		case <-done:
			running = false
		default:
		}
	}
	if observed[0] == 0 {
		t.Errorf("the final size 0 was never observed (observations: %v)", observed)
	}
	for n := range observed {
		if n != 0 && n != batch {
			t.Errorf("observed partial batch state len=%d (observations: %v)", n, observed)
		}
	}
}

// TestCapabilitiesEqualNoDeadlock compares two tries from two goroutines
// in opposite directions at once — Equal must never hold both locks.
func TestCapabilitiesEqualNoDeadlock(t *testing.T) {
	a := &PatriciaTrie[int]{}
	b := &PatriciaTrie[int]{}
	for i := 0; i < 500; i++ {
		k := fmt.Sprintf("k%06d", i)
		a.Insert(k, i)
		b.Insert(k, -i) // different values: Equal is keys-only anyway
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.Equal(b) }()
	go func() { defer wg.Done(); _ = b.Equal(a) }()
	wg.Wait()
}

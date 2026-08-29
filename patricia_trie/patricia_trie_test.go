package patricia_trie

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Zero value, Insert, Search, Contains
// -------------------------------------------------------------------------------------------------------

// TestZeroValue verifies that the zero value is an empty trie ready to
// use: every read reports empty/not-found, and Insert works without any
// constructor.
func TestZeroValue(t *testing.T) {
	var pt PatriciaTrie[int]

	if !pt.IsEmpty() {
		t.Errorf("Expected IsEmpty()=true on a zero-value trie.")
	}
	if pt.Length() != 0 || pt.Len() != 0 {
		t.Errorf("Expected Length()=Len()=0 on a zero-value trie, got %d/%d", pt.Length(), pt.Len())
	}
	if v, ok := pt.Search("a"); ok || v != 0 {
		t.Errorf("Expected Search on a zero-value trie to return (0, false), got (%d, %v)", v, ok)
	}
	if pt.Contains("a") {
		t.Errorf("Expected Contains on a zero-value trie to return false.")
	}
	if pt.Delete("a") {
		t.Errorf("Expected Delete on a zero-value trie to return false.")
	}
	if got := pt.KeysWithPrefix("a"); got != nil {
		t.Errorf("Expected KeysWithPrefix on a zero-value trie to return nil, got %v", got)
	}
	for range pt.All() {
		t.Errorf("Expected All on a zero-value trie to visit nothing.")
	}
	for range pt.Backward() {
		t.Errorf("Expected Backward on a zero-value trie to visit nothing.")
	}

	// The zero value accepts Insert directly.
	if !pt.Insert("go", 1) {
		t.Errorf("Expected Insert on a zero-value trie to return true.")
	}
	if pt.Length() != 1 {
		t.Errorf("Expected Length()=1 after one insert, got %d", pt.Length())
	}
}

func TestInsertSearchContains(t *testing.T) {
	var pt PatriciaTrie[int]

	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore"}
	for i, k := range keys {
		if !pt.Insert(k, i) {
			t.Errorf("Expected Insert(%q) to return true for a new key.", k)
		}
	}
	if pt.Length() != len(keys) {
		t.Errorf("Expected Length()=%d, got %d", len(keys), pt.Length())
	}
	if pt.IsEmpty() {
		t.Errorf("Expected IsEmpty()=false on a populated trie.")
	}
	if pt.Len() != pt.Length() {
		t.Errorf("Expected Len()=%d to alias Length()=%d.", pt.Len(), pt.Length())
	}

	for i, k := range keys {
		v, ok := pt.Search(k)
		if !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", k, i, v, ok)
		}
		if !pt.Contains(k) {
			t.Errorf("Expected Contains(%q)=true.", k)
		}
	}

	// Absent keys: not stored, including proper prefixes and extensions
	// of stored keys.
	for _, k := range []string{"s", "sh", "shell", "shellshock", "a", ""} {
		if _, ok := pt.Search(k); ok {
			t.Errorf("Expected Search(%q) to return ok=false.", k)
		}
		if pt.Contains(k) {
			t.Errorf("Expected Contains(%q)=false.", k)
		}
	}
	checkInvariants(t, &pt, map[string]int{
		"she": 0, "sells": 1, "sea": 2, "shells": 3, "by": 4, "the": 5, "shore": 6,
	})
}

// TestInsertDuplicateReplaces verifies the duplicates-replace
// convention: re-inserting a key replaces its value, returns false, and
// does not change the length.
func TestInsertDuplicateReplaces(t *testing.T) {
	var pt PatriciaTrie[int]

	if !pt.Insert("key", 1) {
		t.Errorf("Expected first Insert to return true.")
	}
	if pt.Insert("key", 2) {
		t.Errorf("Expected duplicate Insert to return false (replaced).")
	}
	if pt.Length() != 1 {
		t.Errorf("Expected Length()=1 after a duplicate insert, got %d", pt.Length())
	}
	if v, ok := pt.Search("key"); !ok || v != 2 {
		t.Errorf("Expected the replacement value 2, got (%d, %v)", v, ok)
	}
}

// TestEmptyStringKey verifies that "" is an ordinary valid key.
func TestEmptyStringKey(t *testing.T) {
	var pt PatriciaTrie[int]

	if !pt.Insert("", 42) {
		t.Errorf("Expected Insert(\"\") to return true.")
	}
	if v, ok := pt.Search(""); !ok || v != 42 {
		t.Errorf("Expected Search(\"\")=(42, true), got (%d, %v)", v, ok)
	}
	if !pt.Contains("") {
		t.Errorf("Expected Contains(\"\")=true.")
	}
	if pt.Insert("", 43) {
		t.Errorf("Expected a duplicate Insert(\"\") to return false.")
	}
	// "" is a prefix of every key, so it shows up in KeysWithPrefix("")
	// and sorts first.
	pt.Insert("a", 1)
	got := pt.KeysWithPrefix("")
	if len(got) != 2 || got[0] != "" || got[1] != "a" {
		t.Errorf("Expected KeysWithPrefix(\"\")=[\"\" \"a\"], got %v", got)
	}
	if !pt.Delete("") {
		t.Errorf("Expected Delete(\"\") to return true.")
	}
	if pt.Contains("") {
		t.Errorf("Expected Contains(\"\")=false after Delete.")
	}
	if pt.Length() != 1 {
		t.Errorf("Expected Length()=1 after deleting the empty key, got %d", pt.Length())
	}
}

// TestUTF8Keys verifies that arbitrary UTF-8 strings work as keys — the
// trie is bitwise over the key bytes, so multi-byte runes are just bit
// paths.
func TestUTF8Keys(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"héllo", "日本語", "\x00\x01", "emoji ✓"}
	for i, k := range keys {
		pt.Insert(k, i)
	}
	for i, k := range keys {
		if v, ok := pt.Search(k); !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", k, i, v, ok)
		}
	}
	if v, ok := pt.Search("héll"); ok {
		t.Errorf("Expected a partial-rune prefix %q to be absent, got (%d, %v)", "héll", v, ok)
	}
}

// TestNulByteKeys verifies the prefix-free encoding: keys that differ
// only by trailing or embedded NUL bytes are distinct, and a key is
// never confused with a zero-padded extension of itself.
func TestNulByteKeys(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"a", "a\x00", "a\x00\x00", "\x00", "\x00a"}
	for i, k := range keys {
		if !pt.Insert(k, i) {
			t.Errorf("Expected Insert(%q) to return true for a new key.", k)
		}
	}
	if pt.Length() != len(keys) {
		t.Errorf("Expected Length()=%d, got %d", len(keys), pt.Length())
	}
	for i, k := range keys {
		if v, ok := pt.Search(k); !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", k, i, v, ok)
		}
	}
	// Ascending order: "" would sort before "a", which sorts before its
	// NUL-padded extensions.
	got := pt.KeysWithPrefix("a")
	want := []string{"a", "a\x00", "a\x00\x00"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"a\")=%q, got %q", want, got)
	}
	checkInvariants(t, &pt, map[string]int{
		"a": 0, "a\x00": 1, "a\x00\x00": 2, "\x00": 3, "\x00a": 4,
	})
}

// -------------------------------------------------------------------------------------------------------
// Delete, including branch collapse
// -------------------------------------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	var pt PatriciaTrie[int]
	pt.Insert("she", 0)
	pt.Insert("shells", 1)
	pt.Insert("shore", 2)

	// Deleting a missing key reports false and changes nothing — even a
	// key whose path is a prefix of a stored key.
	if pt.Delete("sh") {
		t.Errorf("Expected Delete(\"sh\")=false (not a key).")
	}
	if pt.Delete("shellshock") {
		t.Errorf("Expected Delete(\"shellshock\")=false.")
	}
	if pt.Delete("missing") {
		t.Errorf("Expected Delete(\"missing\")=false.")
	}
	if pt.Length() != 3 {
		t.Errorf("Failed deletes changed Length(): expected 3, got %d", pt.Length())
	}

	// Deleting "she" must not disturb "shells"/"shore".
	if !pt.Delete("she") {
		t.Errorf("Expected Delete(\"she\")=true.")
	}
	if pt.Contains("she") {
		t.Errorf("Expected \"she\" to be gone.")
	}
	if !pt.Contains("shells") || !pt.Contains("shore") {
		t.Errorf("Deleting \"she\" disturbed keys that share its prefix.")
	}
	if pt.Length() != 2 {
		t.Errorf("Expected Length()=2, got %d", pt.Length())
	}

	// Re-inserting a deleted key works — its leaf and branch are gone.
	if !pt.Delete("shells") {
		t.Errorf("Expected Delete(\"shells\")=true.")
	}
	if !pt.Insert("shells", 10) {
		t.Errorf("Expected Insert(\"shells\") after delete to return true (added anew).")
	}
	if v, _ := pt.Search("shells"); v != 10 {
		t.Errorf("Expected Search(\"shells\")=10, got %d", v)
	}
	checkInvariants(t, &pt, map[string]int{"shore": 2, "shells": 10})
}

// TestDeleteRestoresZeroShape verifies that deleting every key
// collapses the trie back to no nodes at all: a Patricia trie has no
// dead nodes — every internal node branches, so the last delete removes
// the last leaf directly.
func TestDeleteRestoresZeroShape(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore", ""}
	for i, k := range keys {
		pt.Insert(k, i)
	}
	if leaves, internal := countNodes(pt.root); leaves != len(keys) || internal != len(keys)-1 {
		t.Fatalf("Expected %d leaves and %d internal nodes, got %d/%d",
			len(keys), len(keys)-1, leaves, internal)
	}
	for _, k := range keys {
		if !pt.Delete(k) {
			t.Fatalf("Expected Delete(%q)=true.", k)
		}
	}
	if pt.root != nil {
		t.Errorf("Expected the trie to be fully collapsed (root == nil).")
	}
	if !pt.IsEmpty() || pt.Length() != 0 {
		t.Errorf("Expected an empty trie after deleting the last key.")
	}
}

// -------------------------------------------------------------------------------------------------------
// KeysWithPrefix
// -------------------------------------------------------------------------------------------------------

func TestKeysWithPrefix(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"sea", "sells", "she", "shells", "shore", "the"}
	for i, k := range keys {
		pt.Insert(k, i)
	}

	// Results come out in ascending order regardless of insertion order.
	got := pt.KeysWithPrefix("sh")
	want := []string{"she", "shells", "shore"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"sh\")=%v, got %v", want, got)
	}

	// A key itself is a prefix of itself.
	if got := pt.KeysWithPrefix("sea"); len(got) != 1 || got[0] != "sea" {
		t.Errorf("Expected KeysWithPrefix(\"sea\")=[\"sea\"], got %v", got)
	}

	// A key that is also a prefix of another key matches both.
	got = pt.KeysWithPrefix("she")
	want = []string{"she", "shells"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"she\")=%v, got %v", want, got)
	}

	// Every key matches the empty prefix, ascending.
	got = pt.KeysWithPrefix("")
	want = []string{"sea", "sells", "she", "shells", "shore", "the"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"\")=%v, got %v", want, got)
	}

	// No match returns nil.
	if got := pt.KeysWithPrefix("x"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"x\")=nil, got %v", got)
	}
	if got := pt.KeysWithPrefix("shellshock"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"shellshock\")=nil, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// All / Backward iterators
// -------------------------------------------------------------------------------------------------------

func TestAllIterator(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"the", "she", "shore", "sells", "sea", "shells", "by"}
	for i, k := range keys {
		pt.Insert(k, i)
	}

	// Ascending key order, independent of insertion order.
	var gotKeys []string
	gotVals := make(map[string]int)
	for k, v := range pt.All() {
		gotKeys = append(gotKeys, k)
		gotVals[k] = v
	}
	want := []string{"by", "sea", "sells", "she", "shells", "shore", "the"}
	if strings.Join(gotKeys, ",") != strings.Join(want, ",") {
		t.Errorf("Expected All to yield %v, got %v", want, gotKeys)
	}
	for i, k := range keys {
		if gotVals[k] != i {
			t.Errorf("Expected All to pair %q with %d, got %d", k, i, gotVals[k])
		}
	}

	// A single-variable range yields the key.
	var first string
	for k := range pt.All() {
		first = k
		break
	}
	if first != "by" {
		t.Errorf("Expected a single-variable range to yield the first key \"by\", got %q", first)
	}

	// Breaking out of the loop stops the walk early.
	count := 0
	for range pt.All() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("Expected the early break to stop after 3 pairs, got %d", count)
	}
}

func TestBackwardIterator(t *testing.T) {
	var pt PatriciaTrie[int]
	keys := []string{"the", "she", "shore", "sells", "sea", "shells", "by"}
	for i, k := range keys {
		pt.Insert(k, i)
	}

	var gotKeys []string
	for k := range pt.Backward() {
		gotKeys = append(gotKeys, k)
	}
	want := []string{"the", "shore", "shells", "she", "sells", "sea", "by"}
	if strings.Join(gotKeys, ",") != strings.Join(want, ",") {
		t.Errorf("Expected Backward to yield %v, got %v", want, gotKeys)
	}

	// Breaking out of the loop stops the walk early.
	count := 0
	for range pt.Backward() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("Expected the early break to stop after 2 pairs, got %d", count)
	}
}

// -------------------------------------------------------------------------------------------------------
// Truncate
// -------------------------------------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	var pt PatriciaTrie[int]
	pt.Insert("she", 0)
	pt.Insert("shells", 1)

	pt.Truncate()
	if !pt.IsEmpty() || pt.Length() != 0 || pt.root != nil {
		t.Errorf("Expected an empty trie after Truncate, got IsEmpty=%v Length=%d", pt.IsEmpty(), pt.Length())
	}
	if got := pt.KeysWithPrefix(""); got != nil {
		t.Errorf("Expected KeysWithPrefix after Truncate to return nil, got %v", got)
	}

	// The trie remains usable after Truncate.
	if !pt.Insert("sea", 2) {
		t.Errorf("Expected Insert after Truncate to return true.")
	}
	if v, ok := pt.Search("sea"); !ok || v != 2 {
		t.Errorf("Expected Search(\"sea\")=(2, true) after Truncate, got (%d, %v)", v, ok)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil receiver: tolerated everywhere except Insert
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *PatriciaTrie behaves as an
// empty trie for every operation except Insert.
func TestNilTolerated(t *testing.T) {
	var nilTrie *PatriciaTrie[int]

	if !nilTrie.IsEmpty() {
		t.Errorf("Expected IsEmpty()=true on a nil trie.")
	}
	if nilTrie.Length() != 0 || nilTrie.Len() != 0 {
		t.Errorf("Expected Length()=Len()=0 on a nil trie.")
	}
	if v, ok := nilTrie.Search("a"); ok || v != 0 {
		t.Errorf("Expected Search on a nil trie to return (0, false), got (%d, %v)", v, ok)
	}
	if nilTrie.Contains("a") {
		t.Errorf("Expected Contains on a nil trie to return false.")
	}
	if nilTrie.Delete("a") {
		t.Errorf("Expected Delete on a nil trie to return false.")
	}
	if got := nilTrie.KeysWithPrefix(""); got != nil {
		t.Errorf("Expected KeysWithPrefix on a nil trie to return nil, got %v", got)
	}
	for range nilTrie.All() {
		t.Errorf("Expected All on a nil trie to visit nothing.")
	}
	for range nilTrie.Backward() {
		t.Errorf("Expected Backward on a nil trie to visit nothing.")
	}
	nilTrie.Truncate() // must not panic
	nilTrie.Dump(&strings.Builder{})
}

// TestNilInsertPanics verifies the package's only panic: Insert on a
// nil *PatriciaTrie.
func TestNilInsertPanics(t *testing.T) {
	var nilTrie *PatriciaTrie[int]
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected Insert on a nil trie to panic, it did not.")
			return
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "Insert") || !strings.Contains(msg, "nil") {
			t.Errorf("Unexpected panic message: %v", r)
		}
	}()
	nilTrie.Insert("a", 1)
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkTrieSize = 1000

func benchmarkKey(j int) string {
	return fmt.Sprintf("%06d", j)
}

func BenchmarkInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var pt PatriciaTrie[int]
		for j := range benchmarkTrieSize {
			pt.Insert(benchmarkKey(j), j)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	var pt PatriciaTrie[int]
	for j := range benchmarkTrieSize {
		pt.Insert(benchmarkKey(j), j)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pt.Search(benchmarkKey(i % benchmarkTrieSize))
	}
}

func BenchmarkDelete(b *testing.B) {
	var pt PatriciaTrie[int]
	for j := range benchmarkTrieSize {
		pt.Insert(benchmarkKey(j), j)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range benchmarkTrieSize {
			pt.Delete(benchmarkKey(j))
		}
		for j := range benchmarkTrieSize {
			pt.Insert(benchmarkKey(j), j)
		}
	}
}

package trie_ts

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
	var tr Trie[int]

	if !tr.IsEmpty() {
		t.Errorf("Expected IsEmpty()=true on a zero-value trie.")
	}
	if tr.Length() != 0 || tr.Len() != 0 {
		t.Errorf("Expected Length()=Len()=0 on a zero-value trie, got %d/%d", tr.Length(), tr.Len())
	}
	if v, ok := tr.Search("a"); ok || v != 0 {
		t.Errorf("Expected Search on a zero-value trie to return (0, false), got (%d, %v)", v, ok)
	}
	if tr.Contains("a") {
		t.Errorf("Expected Contains on a zero-value trie to return false.")
	}
	if tr.Delete("a") {
		t.Errorf("Expected Delete on a zero-value trie to return false.")
	}
	if got := tr.LongestPrefixOf("abc"); got != "" {
		t.Errorf("Expected LongestPrefixOf on a zero-value trie to return \"\", got %q", got)
	}
	if got := tr.KeysWithPrefix("a"); got != nil {
		t.Errorf("Expected KeysWithPrefix on a zero-value trie to return nil, got %v", got)
	}
	if got := tr.KeysThatMatch("."); got != nil {
		t.Errorf("Expected KeysThatMatch on a zero-value trie to return nil, got %v", got)
	}
	for range tr.All() {
		t.Errorf("Expected All on a zero-value trie to visit nothing.")
	}

	// The zero value accepts Insert directly.
	if !tr.Insert("go", 1) {
		t.Errorf("Expected Insert on a zero-value trie to return true.")
	}
	if tr.Length() != 1 {
		t.Errorf("Expected Length()=1 after one insert, got %d", tr.Length())
	}
}

func TestInsertSearchContains(t *testing.T) {
	var tr Trie[int]

	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore"}
	for i, k := range keys {
		if !tr.Insert(k, i) {
			t.Errorf("Expected Insert(%q) to return true for a new key.", k)
		}
	}
	if tr.Length() != len(keys) {
		t.Errorf("Expected Length()=%d, got %d", len(keys), tr.Length())
	}
	if tr.IsEmpty() {
		t.Errorf("Expected IsEmpty()=false on a populated trie.")
	}
	if tr.Len() != tr.Length() {
		t.Errorf("Expected Len()=%d to alias Length()=%d.", tr.Len(), tr.Length())
	}

	for i, k := range keys {
		v, ok := tr.Search(k)
		if !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", k, i, v, ok)
		}
		if !tr.Contains(k) {
			t.Errorf("Expected Contains(%q)=true.", k)
		}
	}

	// Absent keys: not stored, including proper prefixes and extensions
	// of stored keys.
	for _, k := range []string{"s", "sh", "shell", "shellshock", "a", ""} {
		if _, ok := tr.Search(k); ok {
			t.Errorf("Expected Search(%q) to return ok=false.", k)
		}
		if tr.Contains(k) {
			t.Errorf("Expected Contains(%q)=false.", k)
		}
	}
}

// TestInsertDuplicateReplaces verifies the duplicates-replace
// convention: re-inserting a key replaces its value, returns false, and
// does not change the length.
func TestInsertDuplicateReplaces(t *testing.T) {
	var tr Trie[int]

	if !tr.Insert("key", 1) {
		t.Errorf("Expected first Insert to return true.")
	}
	if tr.Insert("key", 2) {
		t.Errorf("Expected duplicate Insert to return false (replaced).")
	}
	if tr.Length() != 1 {
		t.Errorf("Expected Length()=1 after a duplicate insert, got %d", tr.Length())
	}
	if v, ok := tr.Search("key"); !ok || v != 2 {
		t.Errorf("Expected the replacement value 2, got (%d, %v)", v, ok)
	}
}

// TestEmptyStringKey verifies the algs4 convention: "" is a valid key,
// stored at the root.
func TestEmptyStringKey(t *testing.T) {
	var tr Trie[int]

	if !tr.Insert("", 42) {
		t.Errorf("Expected Insert(\"\") to return true.")
	}
	if v, ok := tr.Search(""); !ok || v != 42 {
		t.Errorf("Expected Search(\"\")=(42, true), got (%d, %v)", v, ok)
	}
	if !tr.Contains("") {
		t.Errorf("Expected Contains(\"\")=true.")
	}
	if tr.Insert("", 43) {
		t.Errorf("Expected a duplicate Insert(\"\") to return false.")
	}
	if got := tr.LongestPrefixOf("anything"); got != "" {
		t.Errorf("Expected LongestPrefixOf to return \"\" (the empty key), got %q", got)
	}
	// "" is a prefix of every key, so it shows up in KeysWithPrefix("").
	tr.Insert("a", 1)
	got := tr.KeysWithPrefix("")
	if len(got) != 2 || got[0] != "" || got[1] != "a" {
		t.Errorf("Expected KeysWithPrefix(\"\")=[\"\" \"a\"], got %v", got)
	}
	if !tr.Delete("") {
		t.Errorf("Expected Delete(\"\") to return true.")
	}
	if tr.Contains("") {
		t.Errorf("Expected Contains(\"\")=false after Delete.")
	}
	if tr.Length() != 1 {
		t.Errorf("Expected Length()=1 after deleting the empty key, got %d", tr.Length())
	}
}

// TestUTF8Keys verifies that arbitrary UTF-8 strings work as keys —
// the trie is byte-oriented, so multi-byte runes are just byte paths.
func TestUTF8Keys(t *testing.T) {
	var tr Trie[int]
	keys := []string{"héllo", "日本語", "\x00\x01", "emoji ✓"}
	for i, k := range keys {
		tr.Insert(k, i)
	}
	for i, k := range keys {
		if v, ok := tr.Search(k); !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", k, i, v, ok)
		}
	}
	if v, ok := tr.Search("héll"); ok {
		t.Errorf("Expected a partial-rune prefix %q to be absent, got (%d, %v)", "héll", v, ok)
	}
}

// -------------------------------------------------------------------------------------------------------
// Delete, including pruning
// -------------------------------------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	var tr Trie[int]
	tr.Insert("she", 0)
	tr.Insert("shells", 1)
	tr.Insert("shore", 2)

	// Deleting a missing key reports false and changes nothing — even a
	// key whose path is a prefix of a stored key.
	if tr.Delete("sh") {
		t.Errorf("Expected Delete(\"sh\")=false (not a key).")
	}
	if tr.Delete("shellshock") {
		t.Errorf("Expected Delete(\"shellshock\")=false (path dies out).")
	}
	if tr.Delete("missing") {
		t.Errorf("Expected Delete(\"missing\")=false.")
	}
	if tr.Length() != 3 {
		t.Errorf("Failed deletes changed Length(): expected 3, got %d", tr.Length())
	}

	// Deleting "she" must not disturb "shells"/"shore", which share its
	// path; no node of theirs may be pruned.
	if !tr.Delete("she") {
		t.Errorf("Expected Delete(\"she\")=true.")
	}
	if tr.Contains("she") {
		t.Errorf("Expected \"she\" to be gone.")
	}
	if !tr.Contains("shells") || !tr.Contains("shore") {
		t.Errorf("Deleting \"she\" disturbed keys that share its path.")
	}
	if tr.Length() != 2 {
		t.Errorf("Expected Length()=2, got %d", tr.Length())
	}

	// Re-inserting a deleted key works — its nodes were pruned away.
	if !tr.Delete("shells") {
		t.Errorf("Expected Delete(\"shells\")=true.")
	}
	if !tr.Insert("shells", 10) {
		t.Errorf("Expected Insert(\"shells\") after delete to return true (added anew).")
	}
	if v, _ := tr.Search("shells"); v != 10 {
		t.Errorf("Expected Search(\"shells\")=10, got %d", v)
	}
	checkInvariants(t, &tr, map[string]int{"shore": 2, "shells": 10})
}

// TestDeletePrunesNodes verifies that delete prunes now-childless
// value-less nodes: deleting every key collapses the trie back to no
// nodes at all.  (Single-goroutine test: reading root without the lock
// is fine here.)
func TestDeletePrunesNodes(t *testing.T) {
	var tr Trie[int]
	tr.Insert("abcdef", 1)
	if nodes := countNodes(tr.root); nodes != 7 { // root + 6 bytes
		t.Fatalf("Expected 7 nodes after one insert, got %d", nodes)
	}
	if !tr.Delete("abcdef") {
		t.Fatalf("Expected Delete(\"abcdef\")=true.")
	}
	if tr.root != nil {
		t.Errorf("Expected the trie to be fully pruned (root == nil), got %d nodes", countNodes(tr.root))
	}
	if !tr.IsEmpty() || tr.Length() != 0 {
		t.Errorf("Expected an empty trie after deleting the last key.")
	}
}

// -------------------------------------------------------------------------------------------------------
// LongestPrefixOf
// -------------------------------------------------------------------------------------------------------

func TestLongestPrefixOf(t *testing.T) {
	var tr Trie[int]
	tr.Insert("she", 0)
	tr.Insert("shell", 1)
	tr.Insert("shells", 2)
	tr.Insert("by", 3)

	cases := []struct {
		query, want string
	}{
		{"shellsort", "shells"}, // longest key that is a prefix
		{"shell", "shell"},      // an exact match is its own prefix
		{"shells", "shells"},    //
		{"shelter", "she"},      // diverges after "she"
		{"by", "by"},            //
		{"byte", "by"},          //
		{"sh", ""},              // a non-key prefix of keys is not an answer
		{"x", ""},               // no key is a prefix
		{"", ""},                // empty query, no empty key stored
	}
	for _, tc := range cases {
		if got := tr.LongestPrefixOf(tc.query); got != tc.want {
			t.Errorf("Expected LongestPrefixOf(%q)=%q, got %q", tc.query, tc.want, got)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// KeysWithPrefix
// -------------------------------------------------------------------------------------------------------

func TestKeysWithPrefix(t *testing.T) {
	var tr Trie[int]
	keys := []string{"sea", "sells", "she", "shells", "shore", "the"}
	for i, k := range keys {
		tr.Insert(k, i)
	}

	// Results come out in ascending order regardless of insertion order.
	got := tr.KeysWithPrefix("sh")
	want := []string{"she", "shells", "shore"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"sh\")=%v, got %v", want, got)
	}

	// A key itself is a prefix of itself.
	if got := tr.KeysWithPrefix("sea"); len(got) != 1 || got[0] != "sea" {
		t.Errorf("Expected KeysWithPrefix(\"sea\")=[\"sea\"], got %v", got)
	}

	// Every key matches the empty prefix, ascending.
	got = tr.KeysWithPrefix("")
	want = []string{"sea", "sells", "she", "shells", "shore", "the"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysWithPrefix(\"\")=%v, got %v", want, got)
	}

	// No match returns nil.
	if got := tr.KeysWithPrefix("x"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"x\")=nil, got %v", got)
	}
	if got := tr.KeysWithPrefix("shellshock"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"shellshock\")=nil (path dies out), got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// KeysThatMatch
// -------------------------------------------------------------------------------------------------------

func TestKeysThatMatch(t *testing.T) {
	var tr Trie[int]
	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore"}
	for i, k := range keys {
		tr.Insert(k, i)
	}

	// '.' matches any one byte; the pattern length fixes the key length.
	got := tr.KeysThatMatch(".he")
	want := []string{"she", "the"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysThatMatch(\".he\")=%v, got %v", want, got)
	}

	got = tr.KeysThatMatch("s..")
	want = []string{"sea", "she"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysThatMatch(\"s..\")=%v, got %v", want, got)
	}

	// A literal-only pattern is an exact match.
	if got := tr.KeysThatMatch("sea"); len(got) != 1 || got[0] != "sea" {
		t.Errorf("Expected KeysThatMatch(\"sea\")=[\"sea\"], got %v", got)
	}

	// Lengths must match exactly: "..." does not match "sells".
	got = tr.KeysThatMatch("s....")
	want = []string{"sells", "shore"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Expected KeysThatMatch(\"s....\")=%v, got %v", want, got)
	}

	// No match returns nil.
	if got := tr.KeysThatMatch("x.."); got != nil {
		t.Errorf("Expected KeysThatMatch(\"x..\")=nil, got %v", got)
	}
	if got := tr.KeysThatMatch("........"); got != nil {
		t.Errorf("Expected KeysThatMatch with an over-long pattern to return nil, got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// All iterator
// -------------------------------------------------------------------------------------------------------

func TestAllIterator(t *testing.T) {
	var tr Trie[int]
	keys := []string{"the", "she", "shore", "sells", "sea", "shells", "by"}
	for i, k := range keys {
		tr.Insert(k, i)
	}

	// Ascending key order, independent of insertion order.
	var gotKeys []string
	gotVals := make(map[string]int)
	for k, v := range tr.All() {
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
	for k := range tr.All() {
		first = k
		break
	}
	if first != "by" {
		t.Errorf("Expected a single-variable range to yield the first key \"by\", got %q", first)
	}

	// Breaking out of the loop stops the walk early.
	count := 0
	for range tr.All() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("Expected the early break to stop after 3 pairs, got %d", count)
	}
}

// -------------------------------------------------------------------------------------------------------
// Nil receiver: tolerated everywhere except Insert
// -------------------------------------------------------------------------------------------------------

// TestNilTolerated verifies that a nil *Trie behaves as an empty trie
// for every operation except Insert, and that Lock/Unlock on a nil trie
// are no-ops.
func TestNilTolerated(t *testing.T) {
	var nilTrie *Trie[int]

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
	if got := nilTrie.LongestPrefixOf("abc"); got != "" {
		t.Errorf("Expected LongestPrefixOf on a nil trie to return \"\", got %q", got)
	}
	if got := nilTrie.KeysWithPrefix(""); got != nil {
		t.Errorf("Expected KeysWithPrefix on a nil trie to return nil, got %v", got)
	}
	if got := nilTrie.KeysThatMatch("."); got != nil {
		t.Errorf("Expected KeysThatMatch on a nil trie to return nil, got %v", got)
	}
	for range nilTrie.All() {
		t.Errorf("Expected All on a nil trie to visit nothing.")
	}

	// Locking a nil trie is a no-op, not a panic.
	nilTrie.Lock()
	nilTrie.Unlock()
}

// TestNilInsertPanics verifies the package's only panic: Insert on a
// nil *Trie.
func TestNilInsertPanics(t *testing.T) {
	var nilTrie *Trie[int]
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected Insert on a nil trie to panic, it did not.")
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
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
		var tr Trie[int]
		for j := range benchmarkTrieSize {
			tr.Insert(benchmarkKey(j), j)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	var tr Trie[int]
	for j := range benchmarkTrieSize {
		tr.Insert(benchmarkKey(j), j)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Search(benchmarkKey(i % benchmarkTrieSize))
	}
}

func BenchmarkDelete(b *testing.B) {
	var tr Trie[int]
	for j := range benchmarkTrieSize {
		tr.Insert(benchmarkKey(j), j)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range benchmarkTrieSize {
			tr.Delete(benchmarkKey(j))
		}
		for j := range benchmarkTrieSize {
			tr.Insert(benchmarkKey(j), j)
		}
	}
}

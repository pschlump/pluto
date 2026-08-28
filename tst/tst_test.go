package tst

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Insert, Search, Contains
// -------------------------------------------------------------------------------------------------------

// TestInsertSearch covers the algs4 §5.5 TST trace keys.
func TestInsertSearch(t *testing.T) {
	var tt Tst[int]
	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore"}
	for i, key := range keys {
		if !tt.Insert(key, i) {
			t.Errorf("Expected first Insert(%q) to return true.", key)
		}
	}
	if tt.Insert("sea", 99) {
		t.Errorf("Expected re-Insert of %q to return false (replace).", "sea")
	}
	if tt.Length() != 7 {
		t.Errorf("Expected Length()=7, got %d", tt.Length())
	}
	if tt.Len() != 7 {
		t.Errorf("Expected Len()=7, got %d", tt.Len())
	}
	if tt.IsEmpty() {
		t.Errorf("Expected IsEmpty()=false.")
	}

	for i, key := range []string{"she", "sells", "sea", "shells", "by", "the", "shore"} {
		if key == "sea" {
			continue // checked below — its value was replaced with 99
		}
		v, ok := tt.Search(key)
		if !ok || v != i {
			t.Errorf("Expected Search(%q)=(%d, true), got (%d, %v)", key, i, v, ok)
		}
	}
	if v, ok := tt.Search("sea"); !ok || v != 99 {
		t.Errorf("Expected the replaced value: Search(\"sea\")=(99, true), got (%d, %v)", v, ok)
	}

	for _, key := range []string{"s", "sh", "seas", "shel", "shells2", "xyz"} {
		if _, ok := tt.Search(key); ok {
			t.Errorf("Expected Search(%q) to report not-found.", key)
		}
		if tt.Contains(key) {
			t.Errorf("Expected Contains(%q) to return false.", key)
		}
	}
	if !tt.Contains("shells") {
		t.Errorf("Expected Contains(\"shells\") to return true.")
	}
}

// TestByteOrientedKeys verifies that ordering and search are by key
// bytes: multibyte UTF-8 keys and embedded non-printable bytes work and
// sort in byte order.
func TestByteOrientedKeys(t *testing.T) {
	var tt Tst[string]
	keys := []string{"é", "e", "z\x00z", "日本", "a\xff"}
	for _, key := range keys {
		tt.Insert(key, key)
	}
	sorted := slices.Clone(keys)
	slices.Sort(sorted)
	i := 0
	for key, value := range tt.All() {
		if key != sorted[i] || value != sorted[i] {
			t.Errorf("All()[%d]: expected %q, got (%q, %q)", i, sorted[i], key, value)
		}
		i++
	}
	if i != len(keys) {
		t.Errorf("Expected All() to visit %d keys, visited %d", len(keys), i)
	}
	for _, key := range keys {
		if !tt.Contains(key) {
			t.Errorf("Expected Contains(%q) to return true.", key)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// The empty key is rejected
// -------------------------------------------------------------------------------------------------------

func TestEmptyKeyRejected(t *testing.T) {
	var tt Tst[int]
	tt.Insert("a", 1)

	if tt.Insert("", 42) {
		t.Errorf("Expected Insert(\"\") to return false.")
	}
	if tt.Length() != 1 {
		t.Errorf("Insert(\"\") changed Length(): expected 1, got %d", tt.Length())
	}
	if _, ok := tt.Search(""); ok {
		t.Errorf("Expected Search(\"\") to report not-found.")
	}
	if tt.Contains("") {
		t.Errorf("Expected Contains(\"\") to return false.")
	}
	if tt.Delete("") {
		t.Errorf("Expected Delete(\"\") to return false.")
	}
	if got := tt.LongestPrefixOf(""); got != "" {
		t.Errorf("Expected LongestPrefixOf(\"\")=\"\", got %q", got)
	}
	if got := tt.KeysWithPrefix(""); len(got) != 1 || got[0] != "a" {
		t.Errorf("Expected KeysWithPrefix(\"\")=[\"a\"], got %v", got)
	}
}

// -------------------------------------------------------------------------------------------------------
// Delete
// -------------------------------------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	var tt Tst[int]
	keys := []string{"she", "sells", "sea", "shells", "by", "the", "shore"}
	for i, key := range keys {
		tt.Insert(key, i)
	}
	checkInvariants(t, &tt)

	// Missing keys report false and change nothing.
	for _, key := range []string{"s", "se", "seas", "shell", "x", "she2"} {
		if tt.Delete(key) {
			t.Errorf("Expected Delete(%q) of a missing key to return false.", key)
		}
	}
	if tt.Length() != 7 {
		t.Fatalf("Deletes of missing keys changed Length(): expected 7, got %d", tt.Length())
	}
	checkInvariants(t, &tt)

	if !tt.Delete("sells") {
		t.Errorf("Expected Delete(\"sells\") to return true.")
	}
	if tt.Contains("sells") || tt.Contains("she") != true {
		t.Errorf("Delete(\"sells\") disturbed the trie: sells gone, she kept.")
	}
	if tt.Delete("sells") {
		t.Errorf("Expected a second Delete(\"sells\") to return false.")
	}
	if tt.Length() != 6 {
		t.Errorf("Expected Length()=6 after one delete, got %d", tt.Length())
	}
	checkInvariants(t, &tt)

	// Deleting every key must restore the trie to its zero shape: the
	// branch-pruning in deleteNode unlinks every node.
	for _, key := range keys {
		if key == "sells" {
			continue
		}
		if !tt.Delete(key) {
			t.Errorf("Expected Delete(%q) to return true.", key)
		}
	}
	if !tt.IsEmpty() || tt.Length() != 0 {
		t.Errorf("Expected an empty trie after deleting all keys, Length()=%d", tt.Length())
	}
	if tt.root != nil {
		t.Errorf("Expected root=nil after deleting all keys (branches not pruned).")
	}
	checkInvariants(t, &tt)
}

// TestDeletePrefixOfOtherKeys verifies that deleting a key that is a
// strict prefix of other keys ("she" while "shells" remains) keeps the
// other keys intact, and that the reverse order works too.
func TestDeletePrefixOfOtherKeys(t *testing.T) {
	var tt Tst[int]
	tt.Insert("she", 1)
	tt.Insert("shells", 2)
	tt.Insert("shell", 3)

	if !tt.Delete("she") {
		t.Errorf("Expected Delete(\"she\") to return true.")
	}
	for _, key := range []string{"shell", "shells"} {
		if !tt.Contains(key) {
			t.Errorf("Delete(\"she\") lost %q.", key)
		}
	}
	checkInvariants(t, &tt)

	if !tt.Delete("shells") {
		t.Errorf("Expected Delete(\"shells\") to return true.")
	}
	if !tt.Contains("shell") {
		t.Errorf("Delete(\"shells\") lost \"shell\".")
	}
	if !tt.Delete("shell") {
		t.Errorf("Expected Delete(\"shell\") to return true.")
	}
	if tt.root != nil {
		t.Errorf("Expected root=nil after deleting all keys.")
	}
}

// -------------------------------------------------------------------------------------------------------
// LongestPrefixOf
// -------------------------------------------------------------------------------------------------------

func TestLongestPrefixOf(t *testing.T) {
	var tt Tst[int]
	for i, key := range []string{"she", "shell", "shells", "sea"} {
		tt.Insert(key, i)
	}
	cases := []struct{ query, want string }{
		{"shellsort", "shells"},
		{"shell", "shell"},
		{"shells", "shells"},
		{"she", "she"},
		{"shelters", "she"},
		{"seashore", "sea"},
		{"xyz", ""},
		{"s", ""},
		{"sh", ""},
	}
	for _, c := range cases {
		if got := tt.LongestPrefixOf(c.query); got != c.want {
			t.Errorf("Expected LongestPrefixOf(%q)=%q, got %q", c.query, c.want, got)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// KeysWithPrefix, KeysThatMatch
// -------------------------------------------------------------------------------------------------------

func TestKeysWithPrefix(t *testing.T) {
	var tt Tst[int]
	for i, key := range []string{"she", "shells", "sea", "shore", "the", "by"} {
		tt.Insert(key, i)
	}

	got := tt.KeysWithPrefix("sh")
	want := []string{"she", "shells", "shore"}
	if !slices.Equal(got, want) {
		t.Errorf("Expected KeysWithPrefix(\"sh\")=%v, got %v", want, got)
	}
	if !slices.IsSorted(got) {
		t.Errorf("Expected KeysWithPrefix results in ascending order, got %v", got)
	}

	got = tt.KeysWithPrefix("she")
	want = []string{"she", "shells"}
	if !slices.Equal(got, want) {
		t.Errorf("Expected KeysWithPrefix(\"she\")=%v (the prefix itself is a key), got %v", want, got)
	}

	if got := tt.KeysWithPrefix("xyz"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"xyz\")=nil, got %v", got)
	}
	if got := tt.KeysWithPrefix("sea2"); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"sea2\")=nil, got %v", got)
	}

	// The empty prefix matches every key, ascending.
	got = tt.KeysWithPrefix("")
	want = []string{"by", "sea", "she", "shells", "shore", "the"}
	if !slices.Equal(got, want) {
		t.Errorf("Expected KeysWithPrefix(\"\")=%v, got %v", want, got)
	}

	// Prefix queries on an empty trie report nil.
	var empty Tst[int]
	if got := empty.KeysWithPrefix(""); got != nil {
		t.Errorf("Expected KeysWithPrefix(\"\") on an empty trie to return nil, got %v", got)
	}
}

func TestKeysThatMatch(t *testing.T) {
	var tt Tst[int]
	for i, key := range []string{"she", "the", "sea", "by"} {
		tt.Insert(key, i)
	}

	cases := []struct {
		pattern string
		want    []string
	}{
		{".he", []string{"she", "the"}},
		{"s..", []string{"sea", "she"}},
		{"...", []string{"sea", "she", "the"}},
		{"..", []string{"by"}},
		{"s.e", []string{"she"}},
		{"the", []string{"the"}},
		{"....", nil},
		{"x..", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := tt.KeysThatMatch(c.pattern)
		if !slices.Equal(got, c.want) {
			t.Errorf("Expected KeysThatMatch(%q)=%v, got %v", c.pattern, c.want, got)
		}
		if got != nil && !slices.IsSorted(got) {
			t.Errorf("Expected KeysThatMatch(%q) results in ascending order, got %v", c.pattern, got)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// All — ascending iteration over the live trie
// -------------------------------------------------------------------------------------------------------

func TestAll(t *testing.T) {
	var tt Tst[int]
	for i, key := range []string{"she", "sells", "sea", "shells", "by", "the", "shore"} {
		tt.Insert(key, i)
	}

	var keys []string
	var values []int
	for key, value := range tt.All() {
		keys = append(keys, key)
		values = append(values, value)
	}
	want := []string{"by", "sea", "she", "shells", "shore", "sells", "the"}
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Errorf("Expected All() to visit %v in ascending order, got %v", want, keys)
	}
	for i, key := range keys {
		if v, _ := tt.Search(key); v != values[i] {
			t.Errorf("All() value mismatch for %q: iterator gave %d, Search gives %d", key, values[i], v)
		}
	}

	// Early break stops the walk.
	count := 0
	for range tt.All() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("Expected an early break after 3 keys, got %d", count)
	}

	// An empty trie visits nothing.
	var empty Tst[int]
	for range empty.All() {
		t.Errorf("Expected All() on an empty trie to visit nothing.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value, nil tolerance, and the one panic
// -------------------------------------------------------------------------------------------------------

// TestZeroValue verifies that the zero value is fully usable, including
// Insert, and behaves as an empty trie for every other operation.
func TestZeroValue(t *testing.T) {
	var tt Tst[int]

	if !tt.IsEmpty() || tt.Length() != 0 || tt.Len() != 0 {
		t.Errorf("Expected the zero value to report empty/0/0.")
	}
	if _, ok := tt.Search("a"); ok {
		t.Errorf("Expected Search on the zero value to report not-found.")
	}
	if tt.Contains("a") || tt.Delete("a") {
		t.Errorf("Expected Contains/Delete on the zero value to return false.")
	}
	if got := tt.LongestPrefixOf("abc"); got != "" {
		t.Errorf("Expected LongestPrefixOf on the zero value to return \"\".")
	}
	if got := tt.KeysWithPrefix(""); got != nil {
		t.Errorf("Expected KeysWithPrefix on the zero value to return nil.")
	}
	if got := tt.KeysThatMatch("..."); got != nil {
		t.Errorf("Expected KeysThatMatch on the zero value to return nil.")
	}
	for range tt.All() {
		t.Errorf("Expected All on the zero value to visit nothing.")
	}

	// Insert on the zero value works — no constructor needed.
	if !tt.Insert("go", 1) {
		t.Errorf("Expected Insert on the zero value to return true.")
	}
	if v, ok := tt.Search("go"); !ok || v != 1 {
		t.Errorf("Expected Search(\"go\")=(1, true) after inserting into the zero value, got (%d, %v)", v, ok)
	}
	checkInvariants(t, &tt)
}

// TestNilTolerated verifies that a nil *Tst behaves as an empty trie
// for every operation except Insert.
func TestNilTolerated(t *testing.T) {
	var nilTst *Tst[int]

	if _, ok := nilTst.Search("a"); ok {
		t.Errorf("Expected Search on a nil trie to report not-found.")
	}
	if nilTst.Contains("a") || nilTst.Delete("a") {
		t.Errorf("Expected Contains/Delete on a nil trie to return false.")
	}
	if !nilTst.IsEmpty() || nilTst.Length() != 0 || nilTst.Len() != 0 {
		t.Errorf("Expected a nil trie to report empty/0/0.")
	}
	if got := nilTst.LongestPrefixOf("abc"); got != "" {
		t.Errorf("Expected LongestPrefixOf on a nil trie to return \"\".")
	}
	if got := nilTst.KeysWithPrefix("a"); got != nil {
		t.Errorf("Expected KeysWithPrefix on a nil trie to return nil.")
	}
	if got := nilTst.KeysThatMatch("."); got != nil {
		t.Errorf("Expected KeysThatMatch on a nil trie to return nil.")
	}
	for range nilTst.All() {
		t.Errorf("Expected All on a nil trie to visit nothing.")
	}
}

// TestNilInsertPanics verifies the package's only panic: Insert on a
// nil *Tst.
func TestNilInsertPanics(t *testing.T) {
	var nilTst *Tst[int]
	expectPanic(t, "Insert on a nil *Tst", "tst: Insert called on a nil Tst", func() {
		nilTst.Insert("a", 1)
	})
}

// TestPanicMessageDocumentsTheFix pins the panic message: it names the
// method and the nil receiver.
func TestPanicMessageDocumentsTheFix(t *testing.T) {
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
	var nilTst *Tst[int]
	nilTst.Insert("a", 1)
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

// benchKeys returns n deterministic distinct keys of the form "keyNNNNN".
func benchKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%05d", i)
	}
	return keys
}

func BenchmarkInsert(b *testing.B) {
	keys := benchKeys(1000)
	for b.Loop() {
		var tt Tst[int]
		for i, key := range keys {
			tt.Insert(key, i)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	keys := benchKeys(1000)
	var tt Tst[int]
	for i, key := range keys {
		tt.Insert(key, i)
	}
	b.ResetTimer()
	for b.Loop() {
		for _, key := range keys {
			tt.Search(key)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	keys := benchKeys(1000)
	for b.Loop() {
		b.StopTimer()
		var tt Tst[int]
		for i, key := range keys {
			tt.Insert(key, i)
		}
		b.StartTimer()
		for _, key := range keys {
			tt.Delete(key)
		}
	}
}

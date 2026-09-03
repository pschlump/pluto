package patricia_trie

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

		// LongestPrefixOf against the model.
		query := randomKey()
		wKey, wOK := longestPrefixModel(model, query)
		gKey, _, gOK := pt.LongestPrefixOf(query)
		if gKey != wKey || gOK != wOK {
			t.Fatalf("step %d: LongestPrefixOf(%q)=(%q, %v), model says (%q, %v)",
				step, query, gKey, gOK, wKey, wOK)
		}

		// KeysThatMatch against the model, with a wildcard-poked glob.
		pattern := randomGlob(rng, model, randomKey)
		want = want[:0]
		for _, k := range sortedModelKeys(model) {
			if refStringMatchLen(pattern, k) {
				want = append(want, k)
			}
		}
		var gotKeys []string
		for k := range pt.KeysThatMatch(pattern) {
			gotKeys = append(gotKeys, k)
		}
		if strings.Join(gotKeys, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("step %d: KeysThatMatch(%q)=%v, model says %v", step, pattern, gotKeys, want)
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

// -------------------------------------------------------------------------------------------------------
// LongestPrefixOf oracle
// -------------------------------------------------------------------------------------------------------

// longestPrefixModel is the reference for LongestPrefixOf: the longest
// model key that is a prefix of query, and whether any exists.
func longestPrefixModel(model map[string]int, query string) (string, bool) {
	longest := ""
	found := false
	for k := range model {
		if strings.HasPrefix(query, k) && (!found || len(k) > len(longest)) {
			longest, found = k, true
		}
	}
	return longest, found
}

// -------------------------------------------------------------------------------------------------------
// Glob reference: a second, independent port of stringmatchlen
// -------------------------------------------------------------------------------------------------------

// refStringMatchLen is the oracle for globMatch: a second port of
// Redis's stringmatchlen (note/redis/src/util.c), written as an
// element-at-a-time recursion rather than the C's cursor loop, and
// without the skipLongerMatches early-out or the nesting cutoff — the
// plain exponential semantics.  Cross-checking the two catches porting
// slips in either; inputs are kept short so the exponential search
// stays cheap.  It is case-sensitive, matching the package.
//
// One C quirk must be preserved positionally: an empty string matches
// only the empty pattern — the C loop never runs on entry, so its
// trailing-'*' strip (which fires when the walk CONSUMES the last byte)
// never gets a chance.  Hence "*" does not match "".
func refStringMatchLen(pattern, s string) bool {
	if len(s) == 0 {
		return len(pattern) == 0
	}
	if len(pattern) == 0 {
		return false
	}
	rest := pattern // the pattern remaining after the first element
	matched := true // did the first element match s[0]?
	switch pattern[0] {
	case '*':
		for len(pattern) > 1 && pattern[1] == '*' {
			pattern = pattern[1:] // collapse runs of '*'
		}
		if len(pattern) == 1 {
			return true
		}
		for i := 0; i < len(s); i++ { // non-empty suffixes only, as in the C
			if refStringMatchLen(pattern[1:], s[i:]) {
				return true
			}
		}
		return false
	case '?':
		rest = pattern[1:]
	case '[':
		rest = pattern[1:]
		negated := strings.HasPrefix(rest, "^")
		if negated {
			rest = rest[1:]
		}
		matched = false
		for len(rest) > 0 {
			if rest[0] == ']' {
				rest = rest[1:]
				break
			}
			switch {
			case strings.HasPrefix(rest, `\`) && len(rest) >= 2:
				if rest[1] == s[0] {
					matched = true
				}
				rest = rest[2:]
			case len(rest) >= 3 && rest[1] == '-':
				lo, hi := rest[0], rest[2]
				if lo > hi {
					lo, hi = hi, lo // reversed bounds swap, as in the C
				}
				if s[0] >= lo && s[0] <= hi {
					matched = true
				}
				rest = rest[3:] // the range end may itself be ']'
			default:
				if rest[0] == s[0] {
					matched = true
				}
				rest = rest[1:]
			}
		}
		// An unterminated class simply runs to the end of the pattern.
		if negated {
			matched = !matched
		}
	case '\\':
		if len(pattern) >= 2 {
			matched = pattern[1] == s[0]
			rest = pattern[2:]
		} else {
			matched = s[0] == '\\' // trailing lone backslash is literal
			rest = pattern[1:]
		}
	default:
		matched = pattern[0] == s[0]
		rest = pattern[1:]
	}
	if !matched {
		return false
	}
	if len(s) == 1 {
		// Consuming s[0] emptied the string — the C's end-of-loop strip:
		// only '*'s may remain.
		for len(rest) > 0 && rest[0] == '*' {
			rest = rest[1:]
		}
		return len(rest) == 0
	}
	return refStringMatchLen(rest, s[1:])
}

// randomGlob builds a random glob pattern over the test alphabet by
// poking wildcards into a base key (a random string half the time, a
// stored model key half the time, so both hits and misses are common).
func randomGlob(rng *rand.Rand, model map[string]int, randomKey func() string) string {
	base := randomKey()
	if keys := sortedModelKeys(model); len(keys) > 0 && rng.Intn(2) == 0 {
		base = keys[rng.Intn(len(keys))]
	}
	for range 1 + rng.Intn(3) {
		if len(base) == 0 {
			break
		}
		i := rng.Intn(len(base))
		var wild string
		switch rng.Intn(8) {
		case 0, 1:
			wild = "*"
		case 2, 3:
			wild = "?"
		case 4:
			wild = "[ab]"
		case 5:
			wild = "[^a]"
		case 6:
			wild = "[a-c]"
		case 7:
			wild = `\*`
		}
		base = base[:i] + wild + base[i+1:]
	}
	switch rng.Intn(4) {
	case 0:
		base = "*" + base
	case 1:
		base = base + "*"
	case 2:
		base = "*" + base + "*"
	}
	if rng.Intn(16) == 0 {
		base = "" // occasionally the empty pattern
	}
	return base
}

// TestGlobMatchVectors pins globMatch — and its oracle — against
// behaviors hand-traced from the C stringmatchlen, including the edge
// cases: escaping, unterminated classes, empty classes, reversed
// ranges, and the C quirk that '*' does not match an empty string (the
// C loop never runs when the string is empty).
func TestGlobMatchVectors(t *testing.T) {
	vectors := []struct {
		pattern, s string
		want       bool
	}{
		// Literals and lengths.
		{"", "", true},
		{"", "a", false},
		{"a", "", false},
		{"a", "a", true},
		{"a", "b", false},
		{"a", "ab", false},
		{"ab", "a", false},
		{"A", "a", false}, // case-sensitive

		// '?' matches exactly one byte.
		{"?", "a", true},
		{"?", "", false},
		{"?", "ab", false},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"h?llo", "hallo", true},

		// '*'.
		{"*", "abc", true},
		{"*", "", false}, // C quirk: the loop needs a non-empty string
		{"**", "abc", true},
		{"a*", "a", true},
		{"a*", "abxyz", true},
		{"*a", "a", true},
		{"*a", "ba", true},
		{"*a", "b", false},
		{"a*b", "aXb", true},
		{"a*b", "ab", true},
		{"a*b", "aX", false},
		{"*a*", "banana", true},
		{"*a*", "bbbb", false},
		{"*a*b*", "xaYb", true},

		// Backslash escaping.
		{`a\*b`, "a*b", true},
		{`a\*b`, "aXb", false},
		{`a\?`, "a?", true},
		{`\`, `\`, true}, // trailing lone backslash is a literal one
		{`\`, "x", false},
		{`\a`, "a", true}, // any byte may be escaped, special or not

		// Character classes.
		{"[abc]", "b", true},
		{"[abc]", "d", false},
		{"[a-c]", "b", true},
		{"[a-c]", "d", false},
		{"[c-a]", "b", true}, // reversed bounds swap
		{"[^abc]", "d", true},
		{"[^abc]", "a", false},
		{"x[0-9]y", "x5y", true},
		{"[a-z]*", "hello", true},
		{`[\]]`, "]", true}, // an escaped ']' is a member
		{`[\]]`, "x", false},
		{"[[]", "[", true}, // a '[' inside a class is literal
		{"[]", "]", false}, // ']' first closes at once: empty class
		{"[]", "[", false},
		{"[a^b]", "^", true}, // '^' is literal when not first

		// Unterminated classes run to the end of the pattern.
		{"[b", "b", true},
		{"[b", "x", false},
		{"[", "[", false}, // a bare '[' is an empty class: matches nothing
		{"[^", "x", true}, // an unterminated '[^' is negated-empty: any byte
		{"a[", "a", false},
		{"a[", "ab", false},
		{"[ab", "a", true},
		{"[ab", "x", false},
		{"[^ab", "x", true},
		{"[^ab", "a", false},

		// Quirk: in "[a-]" the '-' makes a range whose end is ']'.
		{"[a-]", "a", true},
		{"[a-]", "^", true},
		{"[a-]", "b", false},

		// Binary-safe bytes are ordinary.
		{"[\x00-\x01]", "\x00", true},
		{"[\x80-\xff]", "\xfe", true},
	}
	for _, v := range vectors {
		if got := globMatch(v.pattern, v.s); got != v.want {
			t.Errorf("globMatch(%q, %q)=%v, want %v", v.pattern, v.s, got, v.want)
		}
		if got := refStringMatchLen(v.pattern, v.s); got != v.want {
			t.Errorf("refStringMatchLen(%q, %q)=%v, want %v (oracle is wrong)", v.pattern, v.s, got, v.want)
		}
	}
}

// TestGlobMatchRandomized cross-checks globMatch against the independent
// oracle on random patterns and strings over an alphabet of plain bytes
// and every special, so escapes, classes, and star runs all appear.
func TestGlobMatchRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	alphabet := []string{"a", "b", "c", "*", "?", "[", "]", "^", "-", `\`, "[ab]", "[^a]", "[a-c]", `\*`}
	random := func(maxLen int) string {
		var sb strings.Builder
		for range rng.Intn(maxLen + 1) {
			sb.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		return sb.String()
	}
	for range 20000 {
		pattern, s := random(6), random(6)
		if got, want := globMatch(pattern, s), refStringMatchLen(pattern, s); got != want {
			t.Fatalf("globMatch(%q, %q)=%v, oracle says %v", pattern, s, got, want)
		}
	}
}

// TestGlobLiteralPrefix pins the pruning-prefix extraction.
func TestGlobLiteralPrefix(t *testing.T) {
	for pat, want := range map[string]string{
		"user:*": "user:",
		"h?i":    "h",
		"*x":     "",
		"[a]x":   "",
		"a[b":    "a",
		`ab\*`:   "ab*",
		`ab\?c`:  "ab?c",
		"ab\\":   `ab\`, // trailing lone backslash is a literal one
		"plain":  "plain",
		"":       "",
		"?":      "",
		`x\[y`:   "x[y",
		`x\]y`:   "x]y",
		`x\^y`:   "x^y",
	} {
		if got := globLiteralPrefix(pat); got != want {
			t.Errorf("globLiteralPrefix(%q)=%q, want %q", pat, got, want)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Delete stress: prefix-shared keys and path re-compression
// -------------------------------------------------------------------------------------------------------

// TestDeletePrefixSharedKeys is the classic crit-bit Delete bug farm:
// dense keys with heavy prefix sharing (including NUL-padded chains and
// the empty key), deleted in a fixed-seed shuffled order, with the
// structural invariants — no unary internal nodes, strictly increasing
// bit indexes, exact node counts — verified after every single delete.
func TestDeletePrefixSharedKeys(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	// Every string over {a, b} of length 0..4, plus NUL-suffixed
	// variants — keys that are prefixes of one another, differ only in
	// trailing NULs, and share long bit paths.
	var keys []string
	var gen func(prefix string, depth int)
	gen = func(prefix string, depth int) {
		keys = append(keys, prefix)
		if depth == 4 {
			return
		}
		gen(prefix+"a", depth+1)
		gen(prefix+"b", depth+1)
	}
	gen("", 0)
	for _, k := range append([]string{}, keys...) {
		keys = append(keys, k+"\x00", k+"\x00\x00")
	}

	var pt PatriciaTrie[int]
	model := make(map[string]int, len(keys))
	for i, k := range keys {
		if !pt.Insert(k, i) {
			t.Fatalf("Expected Insert(%q) to return true.", k)
		}
		model[k] = i
	}
	checkInvariants(t, &pt, model)

	// Delete in shuffled order; after each delete the remaining keys are
	// all present, the deleted one is gone, and the shape is a proper
	// Patricia trie (checkInvariants rebuilds the model view).
	order := sortedModelKeys(model)
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	for i, k := range order {
		if !pt.Delete(k) {
			t.Fatalf("Delete(%q) failed at step %d.", k, i)
		}
		delete(model, k)
		if i%7 == 0 || i >= len(order)-8 {
			checkInvariants(t, &pt, model) // no unary internal nodes, counts exact
		}
	}

	if !pt.IsEmpty() || pt.Length() != 0 || pt.root != nil {
		t.Fatalf("Expected a fully collapsed trie after the drain; IsEmpty=%v Length=%d root=%v",
			pt.IsEmpty(), pt.Length(), pt.root)
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
	var pt PatriciaTrie[int]
	pt.Insert("she", 0)
	pt.Insert("by", 1)
	pt.Insert("sea", 2)
	b, err := json.Marshal(&pt)
	if err != nil {
		t.Fatalf("json.Marshal(&pt): %v", err)
	}
	if string(b) != `{"by":1,"sea":2,"she":0}` {
		t.Errorf(`Expected {"by":1,"sea":2,"she":0}, got %s`, b)
	}

	// The empty-string key is just another object member.
	var emptyKey PatriciaTrie[int]
	emptyKey.Insert("", 42)
	if b, err := json.Marshal(&emptyKey); err != nil || string(b) != `{"":42}` {
		t.Errorf(`Expected {"":42}, got (%s, %v)`, b, err)
	}

	// An empty trie encodes as {}.
	if b, err := json.Marshal(&PatriciaTrie[int]{}); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for an empty trie, got (%s, %v)", b, err)
	}

	// A zero-value trie is a tolerated read: {}.
	var zero PatriciaTrie[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} for a zero-value trie, got (%s, %v)", b, err)
	}

	// A direct call on a nil trie encodes as {}; json.Marshal on a nil
	// *PatriciaTrie never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilTrie *PatriciaTrie[int]
	if b, err := nilTrie.MarshalJSON(); err != nil || string(b) != "{}" {
		t.Errorf("Expected {} from a direct nil-trie call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTrie); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil trie, got (%s, %v)", b, err)
	}

	// Value-level marshalers are honored.
	var custom PatriciaTrie[upperString]
	custom.Insert("k1", "x")
	custom.Insert("k2", "y")
	if b, err := json.Marshal(&custom); err != nil || string(b) != `{"k1":"X","k2":"Y"}` {
		t.Errorf(`Expected {"k1":"X","k2":"Y"}, got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	var bad PatriciaTrie[chan int]
	bad.Insert("c", make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a trie of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded pairs land under their keys; iteration is ascending.
	var pt PatriciaTrie[int]
	if err := json.Unmarshal([]byte(`{"she":0,"by":1,"sea":2}`), &pt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []string
	for k, v := range pt.All() {
		got = append(got, fmt.Sprintf("%s:%d", k, v))
	}
	if fmt.Sprint(got) != "[by:1 sea:2 she:0]" {
		t.Errorf("Expected [by:1 sea:2 she:0], got %v", got)
	}

	// A round trip rebuilds a structurally sound trie.
	var items PatriciaTrie[int]
	items.Insert("a", 1)
	items.Insert("b", 2)
	items.Insert("c", 3)
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again PatriciaTrie[int]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, &again, map[string]int{"a": 1, "b": 2, "c": 3})
	if v, ok := again.Search("b"); !ok || v != 2 {
		t.Errorf("Expected Search to work after unmarshal, got (%d, %v)", v, ok)
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte(`{"z":7}`), &pt); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if pt.Length() != 1 || pt.Contains("by") {
		t.Errorf("Expected replacement, got Length()=%d, Contains(\"by\")=%v", pt.Length(), pt.Contains("by"))
	}

	// An empty object and null clear the trie, fully collapsing it.
	var full PatriciaTrie[int]
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
	var custom PatriciaTrie[upperString]
	if err := json.Unmarshal([]byte(`{"k1":"X","k2":"Y"}`), &custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, ok := custom.Search("k2"); !ok || string(v) != "Y" {
		t.Errorf("Expected Search(\"k2\")=(Y, true), got (%q, %v)", v, ok)
	}

	// Decode errors are returned and leave the trie untouched.
	var keep PatriciaTrie[int]
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
	var zero PatriciaTrie[int]
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

	var nilTrie *PatriciaTrie[int]
	for _, data := range []string{"{}", "null"} {
		if err := nilTrie.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil trie to be tolerated, got %v", data, err)
		}
	}
	expectPanic(t, "UnmarshalJSON", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"a":1}`))
	})
	expectPanic(t, "nil PatriciaTrie", func() {
		_ = nilTrie.UnmarshalJSON([]byte(`{"a":1}`))
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a map reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260903)) // fixed seed: deterministic run

	const alphabet = "abc"
	var pt PatriciaTrie[int]
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
			pt.Insert(key, value)
			model[key] = value
		case 2: // Delete
			key := randomKey()
			pt.Delete(key)
			delete(model, key)
		case 3: // JSON round trip through the model
			b, err := json.Marshal(&pt)
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
			var back PatriciaTrie[int]
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("step %d: json.Unmarshal: %v", step, err)
			}
			checkInvariants(t, &back, model)
		}
	}
	checkInvariants(t, &pt, model)
}

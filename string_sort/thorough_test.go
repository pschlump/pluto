package string_sort

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// randomKeys returns n random keys over the first alphabet byte
// values, each of length exactly fixedLen (when fixedLen >= 0) or of
// uniform random length in [0, maxLen].
func randomKeys(rng *rand.Rand, n, alphabet, maxLen, fixedLen int) []string {
	keys := make([]string, n)
	for i := range keys {
		l := 0
		if fixedLen >= 0 {
			l = fixedLen
		} else if maxLen > 0 {
			l = rng.IntN(maxLen + 1)
		}
		b := make([]byte, l)
		for j := range b {
			b[j] = byte(rng.IntN(alphabet))
		}
		keys[i] = string(b)
	}
	return keys
}

// checkAgainstOracle compares the sorted data against slices.Sort of
// the original input.  Strings are values, so the sorted sequence of a
// multiset is unique: equality against the oracle checks sortedness
// and permutation in one.
func checkAgainstOracle(t *testing.T, name string, data, original []string) {
	t.Helper()
	oracle := slices.Clone(original)
	slices.Sort(oracle)
	if !slices.Equal(data, oracle) {
		t.Fatalf("%s: result %q is not the sorted permutation of the input", name, firstFew(data))
	}
}

// firstFew renders at most 5 keys of a failed result to keep failure
// messages readable.
func firstFew(data []string) []string {
	if len(data) > 5 {
		return data[:5]
	}
	return data
}

// TestLSDRandomizedModel sorts random fixed-width keys (small
// alphabets so duplicate keys are common) and cross-checks against
// slices.Sort.  Fixed PCG seed for reproducibility.
func TestLSDRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	alphabets := []int{2, 4, 26}

	for range 500 {
		w := 1 + rng.IntN(6)
		n := rng.IntN(400)
		original := randomKeys(rng, n, alphabets[rng.IntN(3)], 0, w)
		data := slices.Clone(original)
		LSDSort(data, w)
		checkAgainstOracle(t, "LSD round", data, original)
	}
}

// TestMSDRandomizedModel sorts random variable-width keys over small
// and full-byte alphabets and cross-checks against slices.Sort.
func TestMSDRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	alphabets := []int{2, 26, 256}

	for range 500 {
		n := rng.IntN(400)
		original := randomKeys(rng, n, alphabets[rng.IntN(3)], 14, -1)
		data := slices.Clone(original)
		MSDSort(data)
		checkAgainstOracle(t, "MSD round", data, original)
	}
}

// TestQuick3RandomizedModel is TestMSDRandomizedModel for
// Quick3String.
func TestQuick3RandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	alphabets := []int{2, 26, 256}

	for range 500 {
		n := rng.IntN(400)
		original := randomKeys(rng, n, alphabets[rng.IntN(3)], 14, -1)
		data := slices.Clone(original)
		Quick3String(data)
		checkAgainstOracle(t, "Quick3 round", data, original)
	}
}

// TestEqualPrefixHeavy is the backlog's MSD worst case: keys sharing
// long prefixes with tiny random tails, down to the all-equal extreme.
func TestEqualPrefixHeavy(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for range 400 {
		n := rng.IntN(300)
		prefixLen := rng.IntN(12)
		prefix := make([]byte, prefixLen)
		for i := range prefix {
			prefix[i] = byte('a' + rng.IntN(3))
		}
		original := make([]string, n)
		for i := range original {
			tail := make([]byte, rng.IntN(4)) // 0..3 — zero-length tails make all-equal sets
			for j := range tail {
				tail[j] = byte('a' + rng.IntN(3))
			}
			original[i] = string(prefix) + string(tail)
		}

		msd := slices.Clone(original)
		MSDSort(msd)
		checkAgainstOracle(t, "MSD equal-prefix round", msd, original)

		q3 := slices.Clone(original)
		Quick3String(q3)
		checkAgainstOracle(t, "Quick3 equal-prefix round", q3, original)
	}
}

// TestPrefixChains sorts keys where each is a prefix of the next —
// the sentinel-ordering path in its purest form.
func TestPrefixChains(t *testing.T) {
	sorted := []string{""}
	for i := 1; i <= 24; i++ {
		sorted = append(sorted, strings.Repeat("a", i))
	}
	for i := 2; i <= 12; i++ {
		sorted = append(sorted, strings.Repeat("ab", i))
	}
	sorted = append(sorted, "b", "b")

	rng := rand.New(rand.NewPCG(42, 7))
	for range 10 {
		data := slices.Clone(sorted)
		rng.Shuffle(len(data), func(i, j int) { data[i], data[j] = data[j], data[i] })
		original := slices.Clone(data)

		MSDSort(data)
		if !slices.Equal(data, sorted) {
			t.Fatalf("MSD prefix chains: got %q", firstFew(data))
		}

		data = slices.Clone(original)
		Quick3String(data)
		if !slices.Equal(data, sorted) {
			t.Fatalf("Quick3 prefix chains: got %q", firstFew(data))
		}
	}
}

// TestSortedAdversarial hammers the large adversarial shapes:
// already-sorted and reverse-sorted input (the pivot story Quick3's
// median-of-3 exists for) and all-equal keys.
func TestSortedAdversarial(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const n = 100_000
	base := randomKeys(rng, n, 26, 0, 8)

	sorted := slices.Clone(base)
	slices.Sort(sorted)

	reverse := make([]string, n)
	for i := range reverse {
		reverse[i] = sorted[n-1-i]
	}

	equal := make([]string, 2000)
	for i := range equal {
		equal[i] = strings.Repeat("k", 500)
	}

	shapes := []struct {
		name string
		keys []string
	}{
		{"sorted", sorted},
		{"reverse", reverse},
		{"all-equal", equal},
	}
	sorters := []struct {
		name string
		sort func([]string)
	}{
		{"MSDSort", MSDSort},
		{"Quick3String", Quick3String},
	}

	for _, shape := range shapes {
		for _, sorter := range sorters {
			data := slices.Clone(shape.keys)
			sorter.sort(data)
			checkAgainstOracle(t, sorter.name+" on "+shape.name, data, shape.keys)
		}
	}
}

// TestDeepCommonPrefix pushes the recursion depth: thousands of keys
// sharing a 2000-byte prefix with tiny tails.  MSDSort recurses once
// per shared byte and Quick3String walks the equal band one byte at a
// time, so both must survive ~2000-deep recursion (Go grows the
// stack); the sort must still be correct.
func TestDeepCommonPrefix(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const n = 2000

	original := make([]string, n)
	for i := range original {
		key := make([]byte, 2000+rng.IntN(3))
		for j := range key {
			key[j] = 'p'
		}
		for j := 2000; j < len(key); j++ {
			key[j] = byte('a' + rng.IntN(4))
		}
		original[i] = string(key)
	}

	msd := slices.Clone(original)
	MSDSort(msd)
	checkAgainstOracle(t, "MSD deep common prefix", msd, original)

	q3 := slices.Clone(original)
	Quick3String(q3)
	checkAgainstOracle(t, "Quick3 deep common prefix", q3, original)
}

// TestLSDMaxWidth sorts with w equal to the full common key length —
// the usual way LSD is used (all keys the same length, w = that
// length).
func TestLSDMaxWidth(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	for _, w := range []int{1, 2, 4, 8, 16} {
		original := randomKeys(rng, 1000, 26, 0, w)
		data := slices.Clone(original)
		LSDSort(data, w)
		checkAgainstOracle(t, "LSD full-width", data, original)
	}
}

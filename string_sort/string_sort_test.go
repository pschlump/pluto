package string_sort

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"slices"
	"strings"
	"testing"
)

// The §5.1 three-letter word set is the algs4 MSD example (figure 5.1
// in the book sorts exactly these); shells.txt ("she sells sea shells
// by the sea shore") is the book's running string-sort fixture.
var (
	msdWords  = []string{"dab", "add", "cab", "fad", "fee", "bad", "dad", "ebb", "ace"}
	msdSorted = []string{"ace", "add", "bad", "cab", "dab", "dad", "ebb", "fad", "fee"}

	shellsWords  = []string{"she", "sells", "sea", "shells", "by", "the", "sea", "shore"}
	shellsSorted = []string{"by", "sea", "sea", "sells", "she", "shells", "shore", "the"}
)

// expectPanicContaining runs fn expecting a panic whose message
// mentions substr.
func expectPanicContaining(t *testing.T, name, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected a panic", name)
			return
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, substr) {
			t.Errorf("%s: panic message %v does not contain %q", name, r, substr)
		}
	}()
	fn()
}

func TestLSDKnownVector(t *testing.T) {
	data := slices.Clone(msdWords)
	LSDSort(data, 3)
	if !slices.Equal(data, msdSorted) {
		t.Fatalf("LSD known vector: got %v, want %v", data, msdSorted)
	}
}

func TestMSDKnownVector(t *testing.T) {
	data := slices.Clone(msdWords)
	MSDSort(data)
	if !slices.Equal(data, msdSorted) {
		t.Fatalf("MSD known vector (3-letter words): got %v, want %v", data, msdSorted)
	}

	data = slices.Clone(shellsWords)
	MSDSort(data)
	if !slices.Equal(data, shellsSorted) {
		t.Fatalf("MSD known vector (shells): got %v, want %v", data, shellsSorted)
	}
}

func TestQuick3KnownVector(t *testing.T) {
	data := slices.Clone(msdWords)
	Quick3String(data)
	if !slices.Equal(data, msdSorted) {
		t.Fatalf("Quick3 known vector (3-letter words): got %v, want %v", data, msdSorted)
	}

	data = slices.Clone(shellsWords)
	Quick3String(data)
	if !slices.Equal(data, shellsSorted) {
		t.Fatalf("Quick3 known vector (shells): got %v, want %v", data, shellsSorted)
	}
}

func TestEmptyAndSmallInputs(t *testing.T) {
	for _, sorter := range []struct {
		name string
		sort func([]string)
	}{
		{"MSDSort", MSDSort},
		{"Quick3String", Quick3String},
	} {
		sorter.sort(nil)
		sorter.sort([]string{})
		one := []string{"only"}
		sorter.sort(one)
		if !slices.Equal(one, []string{"only"}) {
			t.Fatalf("%s: single-element slice was disturbed: %v", sorter.name, one)
		}
		two := []string{"b", "a"}
		sorter.sort(two)
		if !slices.Equal(two, []string{"a", "b"}) {
			t.Fatalf("%s: two-element slice not sorted: %v", sorter.name, two)
		}
	}

	LSDSort(nil, 0)
	LSDSort([]string{}, 0)
	one := []string{"only"}
	LSDSort(one, 1)
	if !slices.Equal(one, []string{"only"}) {
		t.Fatalf("LSDSort: single-element slice was disturbed: %v", one)
	}
}

func TestLSDWZero(t *testing.T) {
	// Zero passes: the slice is untouched (a no-op, not a sort).
	data := []string{"b", "a", "c"}
	LSDSort(data, 0)
	if !slices.Equal(data, []string{"b", "a", "c"}) {
		t.Fatalf("LSDSort(w=0) disturbed the slice: %v", data)
	}
}

func TestLSDStabilityWitness(t *testing.T) {
	// Keys that tie on the first w bytes keep their input order — the
	// counting-sort passes are stable, which is what makes the
	// least-significant-first composition work at all.
	data := []string{"bb", "ba", "ab", "aa"}
	LSDSort(data, 1)
	want := []string{"ab", "aa", "bb", "ba"}
	if !slices.Equal(data, want) {
		t.Fatalf("LSD stability witness: got %v, want %v", data, want)
	}
}

func TestLSDLongerThanW(t *testing.T) {
	// Only the first w bytes are significant; beyond the wth byte the
	// keys are never examined.
	data := []string{"bxz", "ayz"}
	LSDSort(data, 1)
	want := []string{"ayz", "bxz"}
	if !slices.Equal(data, want) {
		t.Fatalf("LSD first-w-bytes contract: got %v, want %v", data, want)
	}
}

func TestEmptyStringKeys(t *testing.T) {
	// The empty string is a prefix of everything and sorts first —
	// the end-of-key sentinel path.
	want := []string{"", "", "\x00", "a", "ab"}
	for _, sorter := range []struct {
		name string
		sort func([]string)
	}{
		{"MSDSort", MSDSort},
		{"Quick3String", Quick3String},
	} {
		data := []string{"", "a", "", "ab", "\x00"}
		sorter.sort(data)
		if !slices.Equal(data, want) {
			t.Fatalf("%s: empty-string keys: got %q, want %q", sorter.name, data, want)
		}
	}
}

func TestBinarySafeKeys(t *testing.T) {
	want := []string{"\x00\x00", "\x00\xff", "\x7f", "\xff", "\xff\x00"}
	for _, sorter := range []struct {
		name string
		sort func([]string)
	}{
		{"MSDSort", MSDSort},
		{"Quick3String", Quick3String},
	} {
		data := []string{"\xff\x00", "\x00\xff", "\xff", "\x00\x00", "\x7f"}
		sorter.sort(data)
		if !slices.Equal(data, want) {
			t.Fatalf("%s: binary-safe keys: got %q, want %q", sorter.name, data, want)
		}
	}

	// Fixed-width binary keys through LSD.
	data := []string{"\xff\x00", "\x00\xff", "\xff\xff", "\x00\x00"}
	LSDSort(data, 2)
	if !slices.Equal(data, []string{"\x00\x00", "\x00\xff", "\xff\x00", "\xff\xff"}) {
		t.Fatalf("LSD binary-safe keys: got %q", data)
	}
}

func TestPanics(t *testing.T) {
	expectPanicContaining(t, "negative w", "LSDSort", func() {
		LSDSort([]string{"a"}, -1)
	})
	expectPanicContaining(t, "negative w on empty slice", "LSDSort", func() {
		LSDSort(nil, -1)
	})
	expectPanicContaining(t, "short key", "LSDSort", func() {
		LSDSort([]string{"abc", "a", "xy"}, 2)
	})
	expectPanicContaining(t, "w beyond every key", "LSDSort", func() {
		LSDSort([]string{"ab"}, 3)
	})
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/pschlump/pluto/multimap"
)

// TestMultimapValuesOfMatchesGet checks ValuesOf against Get for every key of
// random multimaps (duplicate keys and values, natural and Func indexes), plus
// absent keys.
func TestMultimapValuesOfMatchesGet(t *testing.T) { // also checks ValueCount
	random := rand.New(rand.NewSource(42))
	intEqual := func(a, b int) bool { return a == b }
	weakHash := func(value int) uint64 { return uint64(value % 8) }
	for _, size := range []int{0, 1, 2, 17, 300} {
		entries := make([]multimap.Association[int, int], size)
		for position := range entries {
			entries[position] = multimap.Association[int, int]{Key: random.Intn(size/3 + 1), Value: random.Intn(5)}
		}
		for name, value := range map[string]*Multimap[int, int]{
			"natural": NewMultimap(entries),
			"func":    NewMultimapFunc(intEqual, weakHash, intEqual, entries),
		} {
			for key := -1; key <= size/3+2; key++ {
				want, _ := value.Get(key)
				got := slices.Collect(value.ValuesOf(key))
				if !slices.Equal(got, want) {
					t.Fatalf("%s size %d key %d: ValuesOf %v, Get %v", name, size, key, got, want)
				}
				if count := value.ValueCount(key); count != len(want) {
					t.Fatalf("%s size %d key %d: ValueCount %d, len(Get) %d", name, size, key, count, len(want))
				}
			}
		}
	}
}

func TestMultimapValuesOfEarlyStopAndEmpty(t *testing.T) {
	value := NewMultimap([]multimap.Association[string, int]{
		{Key: "a", Value: 1}, {Key: "b", Value: 9}, {Key: "a", Value: 2}, {Key: "a", Value: 2},
	})
	if got := slices.Collect(value.ValuesOf("a")); !slices.Equal(got, []int{1, 2, 2}) {
		t.Fatalf("ValuesOf(a) = %v, want input order with duplicates", got)
	}
	seen := 0
	for range value.ValuesOf("a") {
		seen++
		if seen == 2 {
			break
		}
	}
	if seen != 2 {
		t.Fatalf("early stop saw %d values", seen)
	}
	if got := value.ValueCount("a"); got != 3 {
		t.Fatalf("ValueCount(a) = %d, want 3 (duplicates count)", got)
	}
	var nilMultimap *Multimap[string, int]
	for _, empty := range []*Multimap[string, int]{nilMultimap, {}, value} {
		for range empty.ValuesOf("missing") {
			t.Fatal("absent key, nil or zero-value multimap yielded a value")
		}
		if got := empty.ValueCount("missing"); got != 0 {
			t.Fatalf("ValueCount of an absent key, nil or zero-value multimap = %d", got)
		}
	}
}

// TestMultimapValuesOfAllocationFree pins the reason ValuesOf exists: reading
// a key's values must not allocate, unlike Get's copy.
func TestMultimapValuesOfAllocationFree(t *testing.T) {
	entries := make([]multimap.Association[int, int], 0, 64)
	for position := range 64 {
		entries = append(entries, multimap.Association[int, int]{Key: position % 4, Value: position})
	}
	value := NewMultimap(entries)
	sum := 0
	allocations := testing.AllocsPerRun(1000, func() {
		for item := range value.ValuesOf(2) {
			sum += item
		}
	})
	if allocations != 0 {
		t.Fatalf("ValuesOf allocated %v times per call", allocations)
	}
	if sum == 0 {
		t.Fatal("ValuesOf yielded nothing")
	}
	if allocations := testing.AllocsPerRun(1000, func() { sum += value.ValueCount(2) }); allocations != 0 {
		t.Fatalf("ValueCount allocated %v times per call", allocations)
	}
}

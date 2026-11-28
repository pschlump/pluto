/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"cmp"
	"iter"
	"math/rand"
	"slices"
	"testing"
)

// perKeyReader is the slice of a multimap these tests exercise, captured as
// closures so one driver covers all three variants.
type perKeyReader struct {
	put        func(key, value int)
	putAll     func(key int, values []int)
	remove     func(key, value int)
	removeAll  func(key int)
	get        func(key int) ([]int, bool)
	valueCount func(key int) int
	valuesOf   func(key int) iter.Seq[int]
}

func listReader(m *ListMultimap[int, int]) perKeyReader {
	return perKeyReader{
		put: func(k, v int) { m.Put(k, v) }, putAll: func(k int, vs []int) { m.PutAll(k, vs) },
		remove: func(k, v int) { m.Remove(k, v) }, removeAll: func(k int) { m.RemoveAll(k) },
		get: m.Get, valueCount: m.ValueCount, valuesOf: m.ValuesOf,
	}
}

func setReader(m *SetMultimap[int, int]) perKeyReader {
	return perKeyReader{
		put: func(k, v int) { m.Put(k, v) }, putAll: func(k int, vs []int) { m.PutAll(k, vs) },
		remove: func(k, v int) { m.Remove(k, v) }, removeAll: func(k int) { m.RemoveAll(k) },
		get: m.Get, valueCount: m.ValueCount, valuesOf: m.ValuesOf,
	}
}

func sortedReader(m *SortedSetMultimap[int, int]) perKeyReader {
	return perKeyReader{
		put: func(k, v int) { m.Put(k, v) }, putAll: func(k int, vs []int) { m.PutAll(k, vs) },
		remove: func(k, v int) { m.Remove(k, v) }, removeAll: func(k int) { m.RemoveAll(k) },
		get: m.Get, valueCount: m.ValueCount, valuesOf: m.ValuesOf,
	}
}

// TestPerKeyReadersMatchGet drives random mutations and, after every step,
// checks ValueCount == len(Get) and ValuesOf == Get (same order) for every key,
// including absent ones, for all three variants with natural and clustered
// (weak) hashes.
func TestPerKeyReadersMatchGet(t *testing.T) {
	intEqual := func(a, b int) bool { return a == b }
	weakHash := func(value int) uint64 { return uint64(value % 4) }
	variants := map[string]func() perKeyReader{
		"list":      func() perKeyReader { return listReader(NewListMultimap[int, int]()) },
		"list-func": func() perKeyReader { return listReader(NewListMultimapFunc(intEqual, weakHash, intEqual)) },
		"set":       func() perKeyReader { return setReader(NewSetMultimap[int, int]()) },
		"set-func":  func() perKeyReader { return setReader(NewSetMultimapFunc(intEqual, weakHash, intEqual, weakHash)) },
		"sorted":    func() perKeyReader { return sortedReader(NewSortedSetMultimap[int, int]()) },
		"sorted-func": func() perKeyReader {
			return sortedReader(NewSortedSetMultimapFunc(intEqual, weakHash, cmp.Compare[int]))
		},
	}
	for name, build := range variants {
		random := rand.New(rand.NewSource(42))
		reader := build()
		for step := range 3000 {
			key, value := random.Intn(12), random.Intn(20)
			switch random.Intn(10) {
			case 0, 1, 2, 3:
				reader.put(key, value)
			case 4:
				reader.putAll(key, []int{value, value, random.Intn(20)})
			case 5, 6:
				reader.remove(key, value)
			case 7:
				reader.removeAll(key)
			}
			for probe := -1; probe <= 13; probe++ {
				want, _ := reader.get(probe)
				if count := reader.valueCount(probe); count != len(want) {
					t.Fatalf("%s step %d key %d: ValueCount %d, len(Get) %d", name, step, probe, count, len(want))
				}
				if got := slices.Collect(reader.valuesOf(probe)); !slices.Equal(got, want) {
					t.Fatalf("%s step %d key %d: ValuesOf %v, Get %v", name, step, probe, got, want)
				}
			}
		}
	}
}

func TestPerKeyReadersEdgeCases(t *testing.T) {
	list := NewListMultimap[string, int]()
	list.PutAll("a", []int{3, 1, 3})
	if got := list.ValueCount("a"); got != 3 {
		t.Fatalf("list ValueCount(a) = %d, want 3 (duplicates count)", got)
	}
	set := NewSetMultimap[string, int]()
	set.PutAll("a", []int{3, 1, 3})
	if got := set.ValueCount("a"); got != 2 {
		t.Fatalf("set ValueCount(a) = %d, want 2 (distinct)", got)
	}
	sorted := NewSortedSetMultimap[string, int]()
	sorted.PutAll("a", []int{3, 1, 3, 2})
	if got := slices.Collect(sorted.ValuesOf("a")); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("sorted ValuesOf(a) = %v, want sorted distinct", got)
	}
	seen := 0
	for range list.ValuesOf("a") {
		if seen++; seen == 2 {
			break
		}
	}
	if seen != 2 {
		t.Fatalf("early stop saw %d values", seen)
	}
	var nilList *ListMultimap[string, int]
	var nilSet *SetMultimap[string, int]
	var nilSorted *SortedSetMultimap[string, int]
	if nilList.ValueCount("a") != 0 || nilSet.ValueCount("a") != 0 || nilSorted.ValueCount("a") != 0 {
		t.Fatal("nil multimap ValueCount not 0")
	}
	if (&ListMultimap[string, int]{}).ValueCount("a") != 0 || (&SetMultimap[string, int]{}).ValueCount("a") != 0 ||
		(&SortedSetMultimap[string, int]{}).ValueCount("a") != 0 {
		t.Fatal("zero-value multimap ValueCount not 0")
	}
	for _, seq := range []iter.Seq[int]{
		nilList.ValuesOf("a"), nilSet.ValuesOf("a"), nilSorted.ValuesOf("a"),
		(&ListMultimap[string, int]{}).ValuesOf("a"), (&SetMultimap[string, int]{}).ValuesOf("a"),
		(&SortedSetMultimap[string, int]{}).ValuesOf("a"), list.ValuesOf("missing"),
	} {
		for range seq {
			t.Fatal("nil, zero-value, or absent key yielded a value")
		}
	}
}

// TestPerKeyReadersAllocationFree pins the reason both methods exist: reading
// a key's values must not allocate, unlike Get's copy.
func TestPerKeyReadersAllocationFree(t *testing.T) {
	list, set, sorted := NewListMultimap[int, int](), NewSetMultimap[int, int](), NewSortedSetMultimap[int, int]()
	for value := range 64 {
		list.Put(value%4, value)
		set.Put(value%4, value)
		sorted.Put(value%4, value)
	}
	sum := 0
	checks := map[string]func(){
		"list ValuesOf": func() {
			for v := range list.ValuesOf(2) {
				sum += v
			}
		},
		"set ValuesOf": func() {
			for v := range set.ValuesOf(2) {
				sum += v
			}
		},
		"sorted ValuesOf": func() {
			for v := range sorted.ValuesOf(2) {
				sum += v
			}
		},
		"list ValueCount":   func() { sum += list.ValueCount(2) },
		"set ValueCount":    func() { sum += set.ValueCount(2) },
		"sorted ValueCount": func() { sum += sorted.ValueCount(2) },
	}
	for name, check := range checks {
		if allocations := testing.AllocsPerRun(1000, check); allocations != 0 {
			t.Errorf("%s allocated %v times per call", name, allocations)
		}
	}
	if sum == 0 {
		t.Fatal("readers returned nothing")
	}
}

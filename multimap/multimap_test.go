/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"hash/fnv"
	"iter"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type intMultimap interface {
	Put(int, int) bool
	PutAll(int, []int) int
	Get(int) ([]int, bool)
	Remove(int, int) bool
	RemoveAll(int) []int
	ReplaceValues(int, []int) []int
	ContainsKey(int) bool
	ContainsEntry(int, int) bool
	ContainsValue(int) bool
	Len() int
	KeyLen() int
	Entries() iter.Seq2[int, int]
	Keys() iter.Seq[int]
	Values() iter.Seq[int]
	KeySet() iter.Seq[int]
}

func TestListSemantics(t *testing.T) {
	set := NewListMultimap[int, int]()
	set.Put(1, 10)
	set.Put(1, 10)
	set.Put(1, 20)
	set.Put(2, 30)
	if values, _ := set.Get(1); !reflect.DeepEqual(values, []int{10, 10, 20}) {
		t.Fatal(values)
	}
	if set.Len() != 4 || set.KeyLen() != 2 || !set.ContainsEntry(1, 10) || !set.ContainsValue(30) {
		t.Fatal("list membership or size mismatch")
	}
	if !set.Remove(1, 10) {
		t.Fatal("remove failed")
	}
	if values, _ := set.Get(1); !reflect.DeepEqual(values, []int{10, 20}) {
		t.Fatal(values)
	}
	old := set.ReplaceValues(1, []int{40, 50, 50})
	if !reflect.DeepEqual(old, []int{10, 20}) {
		t.Fatal(old)
	}
	old = set.ReplaceValues(2, nil)
	if !reflect.DeepEqual(old, []int{30}) || set.ContainsKey(2) {
		t.Fatal("empty replacement retained key")
	}
	checkInvariant(t, set)
	set.RemoveAll(1)
	checkInvariant(t, set)
}

func TestSetAndSortedSemantics(t *testing.T) {
	set := NewSetMultimap[int, int]()
	if !set.Put(1, 3) || set.Put(1, 3) || set.PutAll(1, []int{2, 2, 4}) != 2 {
		t.Fatal("set duplicate handling failed")
	}
	if set.Len() != 3 || set.KeyLen() != 1 {
		t.Fatal(set.Len(), set.KeyLen())
	}
	set.Remove(1, 2)
	set.Remove(1, 3)
	set.Remove(1, 4)
	checkInvariant(t, set)

	sorted := NewSortedSetMultimap[int, int]()
	sorted.PutAll(1, []int{4, 2, 4, 3, 1})
	if values, _ := sorted.Get(1); !reflect.DeepEqual(values, []int{1, 2, 3, 4}) {
		t.Fatal(values)
	}
	if old := sorted.ReplaceValues(1, []int{8, 7, 8}); !reflect.DeepEqual(old, []int{1, 2, 3, 4}) {
		t.Fatal(old)
	}
	if values, _ := sorted.Get(1); !reflect.DeepEqual(values, []int{7, 8}) {
		t.Fatal(values)
	}
	checkInvariant(t, sorted)
}

func TestListSeededModel(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	actual := NewListMultimap[int, int]()
	model := make(map[int][]int)
	for step := 0; step < 2000; step++ {
		key, value := random.Intn(12), random.Intn(30)
		switch random.Intn(5) {
		case 0:
			actual.Put(key, value)
			model[key] = append(model[key], value)
		case 1:
			values := []int{random.Intn(30), random.Intn(30)}
			if got := actual.PutAll(key, values); got != 2 {
				t.Fatal(got)
			}
			model[key] = append(model[key], values...)
		case 2:
			got := actual.Remove(key, value)
			index := slicesIndex(model[key], value)
			if got != (index >= 0) {
				t.Fatalf("step %d Remove=%v, want %v", step, got, index >= 0)
			}
			if index >= 0 {
				model[key] = append(model[key][:index], model[key][index+1:]...)
			}
		case 3:
			if !reflect.DeepEqual(actual.RemoveAll(key), model[key]) {
				t.Fatalf("step %d RemoveAll mismatch", step)
			}
			delete(model, key)
		case 4:
			values := []int{random.Intn(30), value}
			old := actual.ReplaceValues(key, values)
			if !reflect.DeepEqual(old, model[key]) {
				t.Fatalf("step %d replacement return mismatch", step)
			}
			model[key] = values
		}
		if len(model[key]) == 0 {
			delete(model, key)
		}
		if step%20 == 0 {
			for expectedKey := 0; expectedKey < 12; expectedKey++ {
				got, found := actual.Get(expectedKey)
				want := model[expectedKey]
				if found != (len(want) > 0) || !reflect.DeepEqual(got, want) {
					t.Fatalf("step %d key %d got %v/%v want %v", step, expectedKey, got, found, want)
				}
			}
			checkInvariant(t, actual)
		}
	}
}

func TestSetSeededModels(t *testing.T) {
	for _, sortedValues := range []bool{false, true} {
		random := rand.New(rand.NewSource(7))
		var actual intMultimap
		if sortedValues {
			actual = NewSortedSetMultimap[int, int]()
		} else {
			actual = NewSetMultimap[int, int]()
		}
		model := make(map[int]map[int]bool)
		for step := 0; step < 2000; step++ {
			key, value := random.Intn(12), random.Intn(30)
			switch random.Intn(4) {
			case 0:
				got := actual.Put(key, value)
				_, exists := model[key][value]
				if got != !exists {
					t.Fatal("Put duplicate result mismatch")
				}
				if model[key] == nil {
					model[key] = make(map[int]bool)
				}
				model[key][value] = true
			case 1:
				values := []int{value, random.Intn(30)}
				added := actual.PutAll(key, values)
				expectedAdded := 0
				if model[key] == nil {
					model[key] = make(map[int]bool)
				}
				for _, item := range values {
					if !model[key][item] {
						expectedAdded++
						model[key][item] = true
					}
				}
				if added != expectedAdded {
					t.Fatal("PutAll count mismatch")
				}
			case 2:
				got := actual.Remove(key, value)
				if got != model[key][value] {
					t.Fatal("Remove result mismatch")
				}
				delete(model[key], value)
			case 3:
				old := actual.RemoveAll(key)
				if len(old) != len(model[key]) {
					t.Fatal("RemoveAll count mismatch")
				}
				delete(model, key)
			}
			if len(model[key]) == 0 {
				delete(model, key)
			}
			if step%20 == 0 {
				for expectedKey := 0; expectedKey < 12; expectedKey++ {
					got, found := actual.Get(expectedKey)
					wantMap := model[expectedKey]
					if found != (len(wantMap) > 0) || len(got) != len(wantMap) {
						t.Fatalf("step %d key %d mismatch", step, expectedKey)
					}
					if sortedValues && !sort.IntsAreSorted(got) {
						t.Fatal("sorted values out of order")
					}
					for _, item := range got {
						if !wantMap[item] {
							t.Fatal("unexpected value")
						}
					}
				}
				checkInvariant(t, actual)
			}
		}
	}
}

func checkInvariant(t *testing.T, set intMultimap) {
	t.Helper()
	keys := 0
	total := 0
	for key := range set.KeySet() {
		keys++
		values, found := set.Get(key)
		if !found || len(values) == 0 {
			t.Fatal("empty or missing key")
		}
		total += len(values)
	}
	if keys != set.KeyLen() || total != set.Len() {
		t.Fatalf("invariant sizes keys=%d/%d total=%d/%d", keys, set.KeyLen(), total, set.Len())
	}
	entries := 0
	for key, value := range set.Entries() {
		entries++
		if !set.ContainsEntry(key, value) {
			t.Fatal("iterated entry is absent")
		}
	}
	if entries != total {
		t.Fatal("flattened entry count mismatch")
	}
}

func TestSnapshotsAndUtilities(t *testing.T) {
	set := NewListMultimap[int, int]()
	set.PutAll(1, []int{2, 3})
	snapshot := set.AsMap()
	values, _ := snapshot.Get(1)
	values[0] = 99
	actual, _ := set.Get(1)
	actual[1] = 88
	again, _ := set.Get(1)
	if !reflect.DeepEqual(again, []int{2, 3}) {
		t.Fatal("Get or AsMap aliases source")
	}
	for entry := range snapshot.Entries() {
		entry.Values[0] = 77
	}
	unchanged, _ := snapshot.Get(1)
	if !reflect.DeepEqual(unchanged, []int{2, 3}) {
		t.Fatal("snapshot Entries aliases snapshot")
	}
	if set.KeyMultiset().Count(1) != 2 {
		t.Fatal("key multiplicity mismatch")
	}

	indexed := Index([]string{"aa", "ab", "b"}, func(value string) string { return value[:1] })
	if values, _ := indexed.Get("a"); !reflect.DeepEqual(values, []string{"aa", "ab"}) {
		t.Fatal(values)
	}
	inverted := InvertFrom[string, string](indexed, nil)
	if inverted.Len() != 3 || !inverted.ContainsEntry("aa", "a") {
		t.Fatal("inversion failed")
	}
	fromMap := ForMap(map[string]int{"a": 1, "b": 2})
	if fromMap.Len() != 2 || fromMap.KeyLen() != 2 {
		t.Fatal("ForMap failed")
	}
}

func TestCustomFunctions(t *testing.T) {
	type key struct {
		name  string
		extra []int
	}
	keyEq := func(a, b key) bool { return strings.EqualFold(a.name, b.name) }
	keyHash := func(value key) uint64 {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(strings.ToLower(value.name)))
		return hash.Sum64()
	}
	list := NewListMultimapFunc[key, string](keyEq, keyHash, strings.EqualFold)
	list.Put(key{name: "A", extra: []int{7}}, "One")
	if !list.ContainsEntry(key{name: "a"}, "one") {
		t.Fatal("custom list functions failed")
	}
	for stored := range list.KeySet() {
		if len(stored.extra) != 1 || stored.extra[0] != 7 {
			t.Fatal("custom key payload was not preserved")
		}
	}
	set := NewSetMultimapFunc[key, string](keyEq, keyHash, strings.EqualFold, func(value string) uint64 {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(strings.ToLower(value)))
		return hash.Sum64()
	})
	set.Put(key{name: "A"}, "One")
	if set.Put(key{name: "a"}, "one") || set.Len() != 1 {
		t.Fatal("custom set functions failed")
	}
	sorted := NewSortedSetMultimapFunc[key, int](keyEq, keyHash, func(a, b int) int { return b - a })
	sorted.PutAll(key{name: "A"}, []int{1, 3, 2})
	if values, _ := sorted.Get(key{name: "a"}); !reflect.DeepEqual(values, []int{3, 2, 1}) {
		t.Fatal(values)
	}
}

func slicesIndex(values []int, value int) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return -1
}

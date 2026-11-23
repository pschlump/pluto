/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestInsertionOrderUpdateDeleteAndReinsert(t *testing.T) {
	m := New[string, int]()
	m.Put("b", 2)
	m.Put("a", 1)
	m.Put("c", 3)
	m.Put("a", 10)
	if got := slices.Collect(m.Keys()); !reflect.DeepEqual(got, []string{"b", "a", "c"}) {
		t.Fatal(got)
	}
	if old, found := m.Delete("a"); old != 10 || !found {
		t.Fatal(old, found)
	}
	m.Put("a", 11)
	want := []Entry[string, int]{{"b", 2}, {"c", 3}, {"a", 11}}
	if got := m.EntriesSlice(); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	clone := m.Clone()
	clone.Put("b", 20)
	if value, _ := m.Get("b"); value != 2 {
		t.Fatal("clone mutated source")
	}
	if m.Equal(clone, func(a, b int) bool { return a == b }) {
		t.Fatal("different values compared equal")
	}
	clone.Put("b", 2)
	if !m.Equal(clone, func(a, b int) bool { return a == b }) {
		t.Fatal("equal maps compared unequal")
	}
}

func TestSeededMapModel(t *testing.T) {
	random := rand.New(rand.NewSource(93))
	actual := New[int, int]()
	model := make(map[int]int)
	order := []int{}
	for step := 0; step < 5000; step++ {
		key, value := random.Intn(100), random.Intn(1000)
		switch random.Intn(3) {
		case 0:
			old, found := actual.Put(key, value)
			want, existed := model[key]
			if old != want || found != existed {
				t.Fatalf("step %d Put=%d/%v want=%d/%v", step, old, found, want, existed)
			}
			if !existed {
				order = append(order, key)
			}
			model[key] = value
		case 1:
			old, found := actual.Delete(key)
			want, existed := model[key]
			if old != want || found != existed {
				t.Fatalf("step %d Delete=%d/%v want=%d/%v", step, old, found, want, existed)
			}
			if existed {
				delete(model, key)
				for index, candidate := range order {
					if candidate == key {
						order = append(order[:index], order[index+1:]...)
						break
					}
				}
			}
		case 2:
			got, found := actual.Get(key)
			want, existed := model[key]
			if got != want || found != existed {
				t.Fatalf("step %d Get=%d/%v want=%d/%v", step, got, found, want, existed)
			}
		}
		if step%25 == 0 {
			if actual.Len() != len(model) || !reflect.DeepEqual(slices.Collect(actual.Keys()), order) {
				t.Fatalf("step %d order or length mismatch", step)
			}
		}
	}
}

func TestCustomEqualityHashAndCollisions(t *testing.T) {
	m := NewFunc[string, int](func(a, b string) bool { return strings.EqualFold(a, b) }, func(string) uint64 { return 1 })
	m.Put("One", 1)
	m.Put("ONE", 2)
	m.Put("Two", 3)
	if m.Len() != 2 || !reflect.DeepEqual(slices.Collect(m.Keys()), []string{"One", "Two"}) {
		t.Fatal(m.EntriesSlice())
	}
	if value, found := m.Get("one"); value != 2 || !found {
		t.Fatal(value, found)
	}
}

func TestJSONRoundTripAndAtomicFailure(t *testing.T) {
	m := FromEntries([]Entry[string, int]{{"a", 1}, {"b", 2}})
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	decoded := New[string, int]()
	if err = json.Unmarshal(data, decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.EntriesSlice(), m.EntriesSlice()) {
		t.Fatal(decoded.EntriesSlice())
	}
	before := m.EntriesSlice()
	if err = json.Unmarshal([]byte(`[{"key":"x"}],`), m); err == nil {
		t.Fatal("malformed JSON succeeded")
	}
	if !reflect.DeepEqual(m.EntriesSlice(), before) {
		t.Fatal("failed decode changed map")
	}
}

func TestEarlyStopAndZeroValue(t *testing.T) {
	m := FromEntries([]Entry[int, int]{{1, 10}, {2, 20}, {3, 30}})
	count := 0
	for range m.Values() {
		count++
		break
	}
	if count != 1 {
		t.Fatal(count)
	}
	var zero Map[int, int]
	if zero.Len() != 0 || zero.ContainsKey(1) || len(zero.EntriesSlice()) != 0 {
		t.Fatal("zero value is not empty")
	}
}

func ExampleMap() {
	m := New[string, int]()
	m.Put("second", 2)
	m.Put("first", 1)
	m.Put("second", 20)
	for key, value := range m.All() {
		fmt.Println(key, value)
	}
	// Output:
	// second 20
	// first 1
}

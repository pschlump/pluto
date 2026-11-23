/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLinkedHashMultisetOrderAndParity(t *testing.T) {
	m := NewLinkedHashMultiset[string]()
	m.Add("b", 2)
	m.Add("a", 1)
	m.Add("c", 3)
	m.Add("a", 2)
	want := []Entry[string]{{"b", 2}, {"a", 3}, {"c", 3}}
	if got := m.EntriesSlice(); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if got := m.Slice(); !reflect.DeepEqual(got, []string{"b", "b", "a", "a", "a", "c", "c", "c"}) {
		t.Fatal(got)
	}
	m.DeleteAll("a")
	m.Add("a", 1)
	if got := m.EntriesSlice(); !reflect.DeepEqual(got, []Entry[string]{{"b", 2}, {"c", 3}, {"a", 1}}) {
		t.Fatal(got)
	}
	if got := m.HighestCountFirst(); !reflect.DeepEqual(got, []Entry[string]{{"c", 3}, {"b", 2}, {"a", 1}}) {
		t.Fatal(got)
	}
}

func TestLinkedHashMultisetAlgebraAndJSON(t *testing.T) {
	left := FromSliceLinked([]int{2, 1, 2, 3})
	right := FromSliceLinked([]int{3, 2, 4})
	var result LinkedHashMultiset[int]
	result.Sum(left, right)
	if got := result.EntriesSlice(); !reflect.DeepEqual(got, []Entry[int]{{2, 3}, {1, 1}, {3, 2}, {4, 1}}) {
		t.Fatal(got)
	}
	result.Intersection(left, right)
	if got := result.EntriesSlice(); !reflect.DeepEqual(got, []Entry[int]{{2, 1}, {3, 1}}) {
		t.Fatal(got)
	}
	data, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewLinkedHashMultiset[int]()
	if err = json.Unmarshal(data, decoded); err != nil {
		t.Fatal(err)
	}
	if !left.Equal(decoded) || !reflect.DeepEqual(left.EntriesSlice(), decoded.EntriesSlice()) {
		t.Fatal(decoded.EntriesSlice())
	}
}

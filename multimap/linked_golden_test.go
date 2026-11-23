/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestGuavaLinkedMultimapGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_linked_multimap.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version            int                        `json:"version"`
		ListKeys           []string                   `json:"listKeys"`
		InitialListEntries []Association[string, int] `json:"initialListEntries"`
		ListEntries        []Association[string, int] `json:"listEntries"`
		SetKeys            []string                   `json:"setKeys"`
		InitialSetEntries  []Association[string, int] `json:"initialSetEntries"`
		SetEntries         []Association[string, int] `json:"setEntries"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	list := NewLinkedListMultimap[string, int]()
	list.Put("b", 2)
	list.Put("a", 1)
	list.Put("b", 3)
	list.Put("a", 1)
	initialListEntries := list.EntriesSlice()
	list.RemoveAll("b")
	list.Put("b", 4)
	set := NewLinkedHashMultimap[string, int]()
	set.PutAll("b", []int{3, 1, 3, 2})
	set.Put("a", 9)
	set.Put("b", 1)
	initialSetEntries := set.EntriesSlice()
	set.Remove("b", 1)
	set.Put("b", 1)
	if golden.Version != 1 ||
		!reflect.DeepEqual(slices.Collect(list.KeySet()), golden.ListKeys) ||
		!reflect.DeepEqual(initialListEntries, golden.InitialListEntries) ||
		!reflect.DeepEqual(list.EntriesSlice(), golden.ListEntries) ||
		!reflect.DeepEqual(slices.Collect(set.KeySet()), golden.SetKeys) ||
		!reflect.DeepEqual(initialSetEntries, golden.InitialSetEntries) ||
		!reflect.DeepEqual(set.EntriesSlice(), golden.SetEntries) {
		t.Fatalf("golden mismatch: %+v\nlist=%v\nset=%v", golden, list.EntriesSlice(), set.EntriesSlice())
	}
}

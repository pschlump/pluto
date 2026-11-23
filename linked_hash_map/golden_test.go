/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestGuavaLinkedMapGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_linked_map.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version int      `json:"version"`
		Keys    []string `json:"keys"`
		Values  []int    `json:"values"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	m := New[string, int]()
	m.Put("b", 2)
	m.Put("a", 1)
	m.Put("c", 3)
	m.Put("a", 10)
	m.Delete("a")
	m.Put("a", 11)
	if golden.Version != 1 || !reflect.DeepEqual(slices.Collect(m.Keys()), golden.Keys) || !reflect.DeepEqual(slices.Collect(m.Values()), golden.Values) {
		t.Fatalf("golden mismatch: %+v, entries=%v", golden, m.EntriesSlice())
	}
}

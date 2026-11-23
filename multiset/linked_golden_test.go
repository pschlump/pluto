/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestGuavaLinkedMultisetGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_linked_multiset.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version int             `json:"version"`
		Entries []Entry[string] `json:"entries"`
		All     []string        `json:"all"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	m := NewLinkedHashMultiset[string]()
	m.Add("b", 2)
	m.Add("a", 1)
	m.Add("c", 3)
	m.Add("a", 2)
	m.SetCount("a", 0)
	m.Add("a", 1)
	if golden.Version != 1 || !reflect.DeepEqual(m.EntriesSlice(), golden.Entries) || !reflect.DeepEqual(m.Slice(), golden.All) {
		t.Fatalf("golden mismatch: %+v, entries=%v all=%v", golden, m.EntriesSlice(), m.Slice())
	}
}

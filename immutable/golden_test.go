/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/pschlump/pluto/bimap"
	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/multimap"
	"github.com/pschlump/pluto/multiset"
	"github.com/pschlump/pluto/table"
)

type immutableTrace struct {
	Version   int                               `json:"version"`
	List      []int                             `json:"list"`
	Set       []string                          `json:"set"`
	Map       []Entry[string, int]              `json:"map"`
	SortedSet []int                             `json:"sortedSet"`
	SortedMap []Entry[string, int]              `json:"sortedMap"`
	Multiset  []string                          `json:"multiset"`
	Multimap  []Entry[string, int]              `json:"multimap"`
	BiMap     []Entry[string, int]              `json:"bimap"`
	Table     []table.Cell[string, string, int] `json:"table"`
}

type listGolden struct {
	Elements []int `json:"elements"`
	Size     int   `json:"size"`
	At       int   `json:"at"`
	SubList  []int `json:"subList"`
	Reverse  []int `json:"reverse"`
}

type setGolden struct {
	Elements  []string `json:"elements"`
	ContainsA bool     `json:"containsA"`
	ContainsZ bool     `json:"containsZ"`
}

type mapGolden struct {
	Entries []Entry[string, int] `json:"entries"`
	Size    int                  `json:"size"`
	GetA    int                  `json:"getA"`
	GetZ    any                  `json:"getZ"`
}

type sortedSetGolden struct {
	Elements []int `json:"elements"`
	SubRange []int `json:"subRange"`
	Higher   int   `json:"higher"`
	Lower    int   `json:"lower"`
	Reverse  []int `json:"reverse"`
}

type sortedMapGolden struct {
	Entries []Entry[string, int] `json:"entries"`
	Head    []Entry[string, int] `json:"head"`
	GetZ    any                  `json:"getZ"`
}

type multisetGolden struct {
	Entries []multiset.Entry[string] `json:"entries"`
	Size    int                      `json:"size"`
	CountB  int                      `json:"countB"`
	All     []string                 `json:"all"`
}

type multimapGolden struct {
	Entries []Entry[string, int] `json:"entries"`
	Size    int                  `json:"size"`
	KeyLen  int                  `json:"keyLen"`
	GetA    []int                `json:"getA"`
}

type bimapGolden struct {
	Entries    []Entry[string, int] `json:"entries"`
	GetA       int                  `json:"getA"`
	InverseOne string               `json:"inverseOne"`
}

type tableGolden struct {
	Cells []table.Cell[string, string, int] `json:"cells"`
	Size  int                               `json:"size"`
	GetAY int                               `json:"getAY"`
}

type immutableGolden struct {
	Version     int             `json:"version"`
	GuavaCommit string          `json:"guavaCommit"`
	List        listGolden      `json:"list"`
	Set         setGolden       `json:"set"`
	Map         mapGolden       `json:"map"`
	SortedSet   sortedSetGolden `json:"sortedSet"`
	SortedMap   sortedMapGolden `json:"sortedMap"`
	Multiset    multisetGolden  `json:"multiset"`
	Multimap    multimapGolden  `json:"multimap"`
	BiMap       bimapGolden     `json:"bimap"`
	Table       tableGolden     `json:"table"`
}

func TestGuavaImmutableGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/immutable.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace immutableTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	actual := immutableGolden{Version: 1, GuavaCommit: guava_compare.GuavaCommit}

	list := NewList(trace.List)
	actual.List = listGolden{Elements: list.Slice(), Size: list.Len(), SubList: list.SubList(1, 3).Slice(), Reverse: collect(list.Reverse())}
	actual.List.At, _ = list.At(1)

	set := NewSet(trace.Set)
	actual.Set = setGolden{Elements: set.Slice(), ContainsA: set.Contains("a"), ContainsZ: set.Contains("z")}

	mapValue, err := NewMap(trace.Map)
	if err != nil {
		t.Fatal(err)
	}
	actual.Map = mapGolden{Entries: mapValue.EntriesSlice(), Size: mapValue.Len()}
	actual.Map.GetA, _ = mapValue.Get("a")
	if _, found := mapValue.Get("z"); !found {
		actual.Map.GetZ = nil
	}

	sortedSet := NewSortedSet(trace.SortedSet)
	actual.SortedSet = sortedSetGolden{Elements: sortedSet.Slice(), SubRange: sortedSet.SubRange(2, true, 4, true).Slice(), Reverse: collect(sortedSet.Reverse())}
	actual.SortedSet.Higher, _ = sortedSet.Higher(2)
	actual.SortedSet.Lower, _ = sortedSet.Lower(4)

	sortedMap, err := NewSortedMap(trace.SortedMap)
	if err != nil {
		t.Fatal(err)
	}
	actual.SortedMap = sortedMapGolden{Entries: sortedMap.EntriesSlice(), Head: sortedMap.Head("c", false).EntriesSlice()}
	if _, found := sortedMap.Get("z"); !found {
		actual.SortedMap.GetZ = nil
	}

	multi := NewMultiset(trace.Multiset)
	actual.Multiset = multisetGolden{Entries: multi.EntriesSlice(), Size: multi.Len(), CountB: multi.Count("b"), All: multi.Slice()}

	multimapValue := NewMultimap(multimapEntries(trace.Multimap))
	actual.Multimap = multimapGolden{Size: multimapValue.Len(), KeyLen: multimapValue.KeyLen()}
	actual.Multimap.GetA, _ = multimapValue.Get("a")
	for entry := range multimapValue.Entries() {
		actual.Multimap.Entries = append(actual.Multimap.Entries, Entry[string, int]{Key: entry.Key, Value: entry.Value})
	}

	bimapValue, err := NewBiMap(bimapEntries(trace.BiMap))
	if err != nil {
		t.Fatal(err)
	}
	actual.BiMap = bimapGolden{}
	for entry := range bimapValue.Entries() {
		actual.BiMap.Entries = append(actual.BiMap.Entries, Entry[string, int]{Key: entry.Key, Value: entry.Value})
	}
	actual.BiMap.GetA, _ = bimapValue.GetValue("a")
	actual.BiMap.InverseOne, _ = bimapValue.Inverse().GetValue(1)

	tableValue := table.NewTreeTable[string, string, int]()
	for _, cell := range trace.Table {
		tableValue.Put(cell.Row, cell.Column, cell.Value)
	}
	frozenTable := FreezeTreeTable(tableValue)
	actual.Table.Size = frozenTable.Len()
	actual.Table.GetAY, _ = frozenTable.Get("a", "y")
	for cell := range frozenTable.Cells() {
		actual.Table.Cells = append(actual.Table.Cells, cell)
	}

	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/immutable.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func multimapEntries(entries []Entry[string, int]) []multimap.Association[string, int] {
	result := make([]multimap.Association[string, int], len(entries))
	for index, entry := range entries {
		result[index] = multimap.Association[string, int]{Key: entry.Key, Value: entry.Value}
	}
	return result
}

func bimapEntries(entries []Entry[string, int]) []bimap.Association[string, int] {
	result := make([]bimap.Association[string, int], len(entries))
	for index, entry := range entries {
		result[index] = bimap.Association[string, int]{Key: entry.Key, Value: entry.Value}
	}
	return result
}

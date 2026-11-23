/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"cmp"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/pschlump/pluto/bimap"
	"github.com/pschlump/pluto/multimap"
	"github.com/pschlump/pluto/multiset"
	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/range_set"
	"github.com/pschlump/pluto/ranges"
	"github.com/pschlump/pluto/table"
)

func TestMultisetSnapshotAndBuilder(t *testing.T) {
	source := []string{"a", "b", "a", "c", "a"}
	value := NewMultiset(source)
	source[0] = "changed"
	if value.Len() != 5 || value.DistinctLen() != 3 || value.Count("a") != 3 {
		t.Fatal("multiset counts", value.EntriesSlice())
	}
	if got := collect(value.Elements()); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatal(got)
	}
	if got := value.Slice(); !reflect.DeepEqual(got, []string{"a", "a", "a", "b", "c"}) {
		t.Fatal(got)
	}
	custom := NewMultisetFunc(lowerEqual, lowerHash, []string{"A", "a", "b"})
	if custom.Count("a") != 2 || custom.Count("B") != 1 {
		t.Fatal("custom multiset equality")
	}
	builder := NewMultisetBuilder[string]()
	builder.AddAll([]string{"a", "a"})
	first := builder.Build()
	builder.Add("b")
	second := builder.Build()
	if first.Len() != 2 || second.Len() != 3 || !first.Equal(NewMultiset([]string{"a", "a"})) {
		t.Fatal("multiset builder reuse")
	}
	encoded, err := json.Marshal(custom)
	if err != nil || string(encoded) != `[{"value":"A","count":2},{"value":"b","count":1}]` {
		t.Fatal(string(encoded), err)
	}
	if CopyOfMultiset(custom) != custom {
		t.Fatal("CopyOfMultiset copied")
	}
}

func TestFreezeMultisets(t *testing.T) {
	hash := multiset.NewHashMultisetFunc(lowerEqual, lowerHash)
	hash.Add("A", 2)
	hash.Add("b", 1)
	frozenHash := FreezeHashMultiset(hash)
	hash.SetCount("A", 0)
	if frozenHash.Count("a") != 2 || frozenHash.Count("B") != 1 {
		t.Fatal("hash snapshot changed with source")
	}
	tree := multiset.NewTreeMultisetFunc(func(a, b string) int { return cmp.Compare(b, a) })
	tree.Add("a", 2)
	tree.Add("c", 1)
	tree.Add("b", 3)
	frozenTree := FreezeTreeMultiset(tree)
	tree.DeleteAll("b")
	if got := collect(frozenTree.Elements()); !reflect.DeepEqual(got, []string{"c", "b", "a"}) {
		t.Fatal(got)
	}
	if frozenTree.Count("b") != 3 || frozenTree.Count("a") != 2 {
		t.Fatal("tree snapshot counts")
	}
	if got := FreezeHashMultiset(&multiset.HashMultiset[string]{}); got.Len() != 0 {
		t.Fatal("zero-value hash freeze")
	}
}

func TestMultimapSnapshotsAndBuilder(t *testing.T) {
	entries := []multimap.Association[string, string]{{Key: "a", Value: "x"}, {Key: "b", Value: "y"}, {Key: "a", Value: "x"}, {Key: "a", Value: "z"}}
	value := NewMultimap(entries)
	entries[0].Value = "changed"
	if value.Len() != 4 || value.KeyLen() != 2 || !value.ContainsEntry("a", "x") || !value.ContainsValue("z") {
		t.Fatal("multimap membership")
	}
	got, found := value.Get("a")
	if !found || !reflect.DeepEqual(got, []string{"x", "x", "z"}) {
		t.Fatal(got)
	}
	got[0] = "changed"
	again, _ := value.Get("a")
	if again[0] != "x" {
		t.Fatal("Get returned internal storage")
	}
	if got := collect(value.Keys()); !reflect.DeepEqual(got, []string{"a", "a", "a", "b"}) {
		t.Fatal(got)
	}
	if got := collect(value.KeySet()); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
	for key, values := range value.AsMap() {
		values[0] = "changed"
		after, _ := value.Get(key)
		if after[0] == "changed" {
			t.Fatal("AsMap returned internal storage")
		}
	}
	custom := NewMultimapFunc(lowerEqual, lowerHash, lowerEqual, []multimap.Association[string, string]{{Key: "A", Value: "X"}, {Key: "a", Value: "x"}})
	if custom.KeyLen() != 1 || custom.Len() != 2 || !custom.ContainsEntry("a", "x") {
		t.Fatal("custom multimap equality")
	}
	builder := NewMultimapBuilder[string, string]()
	builder.Put("a", "x")
	first := builder.Build()
	builder.Put("a", "y")
	second := builder.Build()
	if first.Len() != 1 || second.Len() != 2 {
		t.Fatal("multimap builder reuse")
	}
	encoded, err := json.Marshal(second)
	if err != nil || string(encoded) != `[{"key":"a","value":"x"},{"key":"a","value":"y"}]` {
		t.Fatal(string(encoded), err)
	}
	if CopyOfMultimap(second) != second {
		t.Fatal("CopyOfMultimap copied")
	}
}

func TestFreezeMultimaps(t *testing.T) {
	list := multimap.NewListMultimapFunc(lowerEqual, lowerHash, lowerEqual)
	list.Put("A", "X")
	frozenList := FreezeListMultimap(list)
	list.Put("A", "Y")
	if frozenList.Len() != 1 || !frozenList.ContainsEntry("a", "x") {
		t.Fatal("list multimap snapshot")
	}
	set := multimap.NewSetMultimapFunc(lowerEqual, lowerHash, lowerEqual, lowerHash)
	set.Put("A", "X")
	set.Put("a", "x")
	frozenSet := FreezeSetMultimap(set)
	if frozenSet.Len() != 1 {
		t.Fatal("set multimap snapshot")
	}
	sorted := multimap.NewSortedSetMultimapFunc(lowerEqual, lowerHash, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	sorted.Put("A", "X")
	frozenSorted := FreezeSortedSetMultimap(sorted)
	if frozenSorted.Len() != 1 || !frozenSorted.ContainsEntry("a", "x") {
		t.Fatal("sorted multimap snapshot")
	}
}

func TestBiMapInverseDuplicatesAndBuilder(t *testing.T) {
	entries := []bimap.Association[string, int]{{Key: "a", Value: 1}, {Key: "b", Value: 2}}
	value, err := NewBiMap(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Value = 99
	if got, found := value.GetValue("a"); !found || got != 1 {
		t.Fatal(got, found)
	}
	if got, found := value.GetKey(2); !found || got != "b" {
		t.Fatal(got, found)
	}
	inverse := value.Inverse()
	if got, found := inverse.GetValue(1); !found || got != "a" || inverse.Inverse() != value {
		t.Fatal("inverse identity or lookup", got, found)
	}
	if _, err := NewBiMap([]bimap.Association[string, int]{{Key: "a", Value: 1}, {Key: "a", Value: 2}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatal("duplicate bimap key", err)
	}
	if _, err := NewBiMap([]bimap.Association[string, int]{{Key: "a", Value: 1}, {Key: "b", Value: 1}}); !errors.Is(err, ErrDuplicateValue) {
		t.Fatal("duplicate bimap value", err)
	}
	builder := NewBiMapBuilder[string, int]()
	builder.Put("a", 1)
	first, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	builder.Put("b", 2)
	second, err := builder.Build()
	if err != nil || first.Len() != 1 || second.Len() != 2 {
		t.Fatal("bimap builder reuse", err)
	}
	encoded, err := json.Marshal(second)
	if err != nil || string(encoded) != `[{"key":"a","value":1},{"key":"b","value":2}]` {
		t.Fatal(string(encoded), err)
	}
	if CopyOfBiMap(second) != second {
		t.Fatal("CopyOfBiMap copied")
	}
}

func TestFreezeBiMapAndTables(t *testing.T) {
	source := bimap.NewHashBiMapFunc(lowerEqual, lowerHash, lowerEqual, lowerHash)
	if err := source.Put("A", "X"); err != nil {
		t.Fatal(err)
	}
	frozen := FreezeHashBiMap(source)
	source.ForcePut("a", "Y")
	if got, found := frozen.GetValue("a"); !found || got != "X" {
		t.Fatal("bimap snapshot", got, found)
	}
	if got, found := frozen.GetKey("x"); !found || got != "A" {
		t.Fatal("custom inverse snapshot", got, found)
	}

	hash := table.NewHashTable[string, string, int]()
	hash.Put("a", "x", 1)
	frozenTable := FreezeHashTable(hash)
	hash.Put("a", "x", 2)
	if got, found := frozenTable.Get("a", "x"); !found || got != 1 {
		t.Fatal("table snapshot", got, found)
	}
	if CopyOfTable(frozenTable) != frozenTable {
		t.Fatal("CopyOfTable copied")
	}
	tree := table.NewTreeTable[string, string, int]()
	tree.Put("a", "x", 1)
	if got, found := FreezeTreeTable(tree).Get("a", "x"); !found || got != 1 {
		t.Fatal("tree table snapshot")
	}
	array := table.NewArrayTable[string, string, int]([]string{"a"}, []string{"x"})
	_, _, _ = array.Put("a", "x", 1)
	if got, found := FreezeArrayTable(array).Get("a", "x"); !found || got != 1 {
		t.Fatal("array table snapshot")
	}
}

func TestRangeImmutableSnapshots(t *testing.T) {
	set := range_set.NewDiscreteRangeSet[int](ranges.SignedDomain[int]())
	set.Add(ranges.Closed(1, 3))
	set.Add(ranges.Closed(4, 5))
	frozenSet := FreezeRangeSet(set)
	set.Remove(ranges.Closed(2, 4))
	if !frozenSet.Contains(2) || frozenSet.Len() != 1 || CopyOfRangeSet(frozenSet) != frozenSet {
		t.Fatal("range set snapshot changed with source")
	}
	if got := frozenSet.SubRangeSet(ranges.Closed(2, 4)); !got.Contains(3) || got.Contains(1) {
		t.Fatal("immutable subrange")
	}
	if complement := frozenSet.Complement(); complement.Contains(2) || !complement.Contains(0) {
		t.Fatal("immutable complement")
	}

	mapped := range_map.NewRangeMap[int, string]()
	mapped.Put(ranges.Closed(1, 5), "a")
	frozenMap := FreezeRangeMap(mapped)
	mapped.Put(ranges.Closed(2, 3), "b")
	if value, found := frozenMap.Get(2); !found || value != "a" || CopyOfRangeMap(frozenMap) != frozenMap {
		t.Fatal("range map snapshot changed with source")
	}
	if got := frozenMap.SubRangeMap(ranges.Closed(2, 3)); got.Len() != 1 {
		t.Fatal("immutable range-map subrange")
	}
	if frozenMap.Equal(FreezeRangeMap(mapped.Clone()), func(a, b string) bool { return a == b }) {
		t.Fatal("different range maps are equal")
	}
}

func TestPhaseOneImmutableConcurrentReads(t *testing.T) {
	multi := NewMultiset([]int{1, 1, 2})
	multimapValue := NewMultimap([]multimap.Association[int, int]{{Key: 1, Value: 2}})
	bimapValue, err := NewBiMap([]bimap.Association[int, int]{{Key: 1, Value: 2}})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 1000 {
				multi.Count(1)
				multimapValue.Get(1)
				bimapValue.GetValue(1)
				bimapValue.Inverse().GetValue(2)
				for range multi.All() {
				}
			}
		}()
	}
	wait.Wait()
}

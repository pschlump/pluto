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
	"slices"
	"strings"
	"testing"
)

func TestSortedSetNavigationRangesAndBuilder(t *testing.T) {
	source := []int{3, 1, 5, 3}
	set := NewSortedSet(source)
	source[0] = 99
	if got := set.Slice(); !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatal(got)
	}
	for probe := 0; probe <= 6; probe++ {
		lower, lowerFound := set.Lower(probe)
		floor, floorFound := set.Floor(probe)
		ceil, ceilFound := set.Ceil(probe)
		higher, higherFound := set.Higher(probe)
		wantLower, wantFloor, wantCeil, wantHigher := 0, 0, 0, 0
		wantLowerFound, wantFloorFound, wantCeilFound, wantHigherFound := false, false, false, false
		for _, value := range []int{1, 3, 5} {
			if value < probe {
				wantLower, wantLowerFound = value, true
			}
			if value <= probe {
				wantFloor, wantFloorFound = value, true
			}
			if value >= probe && (!wantCeilFound || value < wantCeil) {
				wantCeil, wantCeilFound = value, true
			}
			if value > probe && (!wantHigherFound || value < wantHigher) {
				wantHigher, wantHigherFound = value, true
			}
		}
		if lower != wantLower || lowerFound != wantLowerFound || floor != wantFloor || floorFound != wantFloorFound || ceil != wantCeil || ceilFound != wantCeilFound || higher != wantHigher || higherFound != wantHigherFound {
			t.Fatalf("navigation at %d: lower=%d/%v floor=%d/%v ceil=%d/%v higher=%d/%v", probe, lower, lowerFound, floor, floorFound, ceil, ceilFound, higher, higherFound)
		}
		if got, want := set.Rank(probe), len(set.Head(probe, false).Slice()); got != want {
			t.Fatal("Rank differs from head size", probe, got, want)
		}
	}
	if got := set.SubRange(1, false, 5, false).Slice(); !reflect.DeepEqual(got, []int{3}) {
		t.Fatal(got)
	}
	if got := set.SubRange(3, false, 3, true).Slice(); len(got) != 0 {
		t.Fatal(got)
	}
	if got := set.Head(0, true).Slice(); len(got) != 0 {
		t.Fatal(got)
	}
	if got := set.Head(3, false).Slice(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatal(got)
	}
	if got := set.Tail(5, false).Slice(); len(got) != 0 {
		t.Fatal(got)
	}
	if got := collect(set.Reverse()); !reflect.DeepEqual(got, []int{5, 3, 1}) {
		t.Fatal(got)
	}
	builder := NewSortedSetBuilder[int]()
	builder.AddAll([]int{3, 1})
	first := builder.Build()
	builder.AddAll([]int{2, 1})
	second := builder.Build()
	if !reflect.DeepEqual(first.Slice(), []int{1, 3}) || !reflect.DeepEqual(second.Slice(), []int{1, 2, 3}) {
		t.Fatal("sorted set builder reuse failed")
	}
	if CopyOfSortedSet(second) != second {
		t.Fatal("CopyOfSortedSet copied")
	}
}

func TestSortedSetEmptySubviewDoesNotRetainArray(t *testing.T) {
	values := make([]int, 1024)
	for index := range values {
		values[index] = index
	}
	set := NewSortedSet(values)
	empty := set.SubRange(512, true, 512, false)
	if empty.Len() != 0 || cap(empty.values) != 0 {
		t.Fatal("empty sorted subview retained backing array", cap(empty.values))
	}
}

func TestSortedSetCustomOrderAndJSON(t *testing.T) {
	reverse := func(a, b string) int { return cmp.Compare(b, a) }
	set := NewSortedSetFunc(reverse, []string{"a", "c", "b", "a"})
	if got := set.Slice(); !reflect.DeepEqual(got, []string{"c", "b", "a"}) {
		t.Fatal(got)
	}
	if !set.Contains("b") || set.Contains("d") {
		t.Fatal("custom membership failed")
	}
	encoded, err := json.Marshal(set)
	if err != nil || string(encoded) != `["c","b","a"]` {
		t.Fatal(string(encoded), err)
	}
}

func TestSortedMapNavigationRangesAndBuilder(t *testing.T) {
	entries := []Entry[int, string]{{Key: 3, Value: "c"}, {Key: 1, Value: "a"}, {Key: 5, Value: "e"}}
	value, err := NewSortedMap(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Value = "changed"
	if got, found := value.Get(3); !found || got != "c" {
		t.Fatal(got, found)
	}
	if _, err := NewSortedMap([]Entry[int, string]{{Key: 1}, {Key: 1}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatal("sorted duplicate accepted", err)
	}
	for probe := 0; probe <= 6; probe++ {
		lower, lowerFound := value.Lower(probe)
		floor, floorFound := value.Floor(probe)
		ceil, ceilFound := value.Ceil(probe)
		higher, higherFound := value.Higher(probe)
		keys := []int{}
		if lowerFound {
			keys = append(keys, lower.Key)
		}
		if floorFound {
			keys = append(keys, floor.Key)
		}
		if ceilFound {
			keys = append(keys, ceil.Key)
		}
		if higherFound {
			keys = append(keys, higher.Key)
		}
		switch probe {
		case 0:
			if lowerFound || floorFound || ceil.Key != 1 || higher.Key != 1 {
				t.Fatal("navigation below minimum", probe, keys)
			}
		case 1, 3, 5:
			if lower.Key >= probe || floor.Key != probe || ceil.Key != probe || higherFound && higher.Key <= probe || probe == 5 && higherFound {
				t.Fatal("navigation at stored key", probe, keys)
			}
		case 2, 4:
			if lower.Key != probe-1 || floor.Key != probe-1 || ceil.Key != probe+1 || higher.Key != probe+1 {
				t.Fatal("navigation between keys", probe, keys)
			}
		case 6:
			if lower.Key != 5 || floor.Key != 5 || ceilFound || higherFound {
				t.Fatal("navigation above maximum", keys)
			}
		}
	}
	if got := collect(value.SubRange(1, false, 5, false).Keys()); !reflect.DeepEqual(got, []int{3}) {
		t.Fatal(got)
	}
	if got := collect(value.Head(0, true).Keys()); len(got) != 0 {
		t.Fatal(got)
	}
	if got := collect(value.Tail(5, false).Keys()); len(got) != 0 {
		t.Fatal(got)
	}
	if got := collect(value.Keys()); !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatal(got)
	}
	reverse := []int{}
	for entry := range value.Reverse() {
		reverse = append(reverse, entry.Key)
	}
	if !slices.Equal(reverse, []int{5, 3, 1}) {
		t.Fatal(reverse)
	}
	builder := NewSortedMapBuilder[int, string]()
	builder.Put(2, "b")
	first, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	builder.Put(1, "a")
	second, err := builder.Build()
	if err != nil || first.Len() != 1 || second.Len() != 2 {
		t.Fatal("sorted map builder reuse", err)
	}
	if CopyOfSortedMap(second) != second {
		t.Fatal("CopyOfSortedMap copied")
	}
	encoded, err := json.Marshal(second)
	if err != nil || string(encoded) != `[{"key":1,"value":"a"},{"key":2,"value":"b"}]` {
		t.Fatal(string(encoded), err)
	}
}

func TestSortedMapCustomDuplicate(t *testing.T) {
	compare := func(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) }
	if _, err := NewSortedMapFunc(compare, []Entry[string, int]{{Key: "a"}, {Key: "A"}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatal("custom duplicate accepted", err)
	}
}

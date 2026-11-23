/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func lowerEqual(a, b string) bool { return strings.EqualFold(a, b) }
func lowerHash(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(strings.ToLower(value)))
	return hash.Sum64()
}

func collect[T any](sequence func(func(T) bool)) []T {
	values := []T{}
	for value := range sequence {
		values = append(values, value)
	}
	return values
}

func TestListDefensiveCopySublistAndBuilder(t *testing.T) {
	source := []int{1, 2, 3, 4}
	list := NewList(source)
	source[0] = 99
	if got := list.Slice(); !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Fatal(got)
	}
	if got, found := list.At(2); !found || got != 3 {
		t.Fatal(got, found)
	}
	if _, found := list.At(-1); found {
		t.Fatal("negative index found")
	}
	sub := list.SubList(1, 3)
	if got := sub.Slice(); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatal(got)
	}
	if got := collect(list.Reverse()); !reflect.DeepEqual(got, []int{4, 3, 2, 1}) {
		t.Fatal(got)
	}
	builder := NewListBuilder[int]()
	builder.AddAll(source)
	first := builder.Build()
	builder.Add(5)
	second := builder.Build()
	if first.Len() != 4 || second.Len() != 5 || !first.Equal(NewList([]int{99, 2, 3, 4})) {
		t.Fatal("builder reuse mutated an earlier build")
	}
	firstSlice := first.Slice()
	firstSlice[0] = -1
	if value, _ := first.At(0); value != 99 {
		t.Fatal("Slice aliases internal storage")
	}
	if CopyOfList(first) != first {
		t.Fatal("CopyOfList copied an immutable value")
	}
}

func TestListSubviewRetentionAndNonComparableEquality(t *testing.T) {
	values := make([]int, 1024)
	list := NewList(values)
	small := list.SubList(0, 1)
	if cap(small.values) >= cap(list.values) {
		t.Fatal("small sublist retained the large backing array")
	}
	large := list.SubList(0, 800)
	if &large.values[0] != &list.values[0] {
		t.Fatal("large sublist copied instead of using an O(1) view")
	}
	type value struct{ data []int }
	left := NewList([]value{{data: []int{1}}})
	right := NewList([]value{{data: []int{2}}})
	if !left.EqualFunc(right, func(a, b value) bool { return len(a.data) == len(b.data) }) {
		t.Fatal("EqualFunc failed for non-comparable values")
	}
}

func TestSetOrderCustomEqualityAndBuilder(t *testing.T) {
	source := []string{"A", "b", "A", "B", "c"}
	set := NewSet(source)
	source[0] = "changed"
	if got := set.Slice(); !reflect.DeepEqual(got, []string{"A", "b", "B", "c"}) {
		t.Fatal(got)
	}
	custom := NewSetFunc(lowerEqual, lowerHash, []string{"A", "a", "b"})
	if custom.Len() != 2 || !custom.Contains("a") || !custom.Contains("B") {
		t.Fatal("custom set equality failed")
	}
	builder := NewSetBuilderFunc[string](lowerEqual, lowerHash)
	builder.Add("A")
	first := builder.Build()
	builder.Add("a")
	builder.Add("b")
	second := builder.Build()
	if first.Len() != 1 || second.Len() != 2 || !first.Equal(NewSetFunc(lowerEqual, lowerHash, []string{"a"})) {
		t.Fatal("set builder reuse failed")
	}
	encoded, err := json.Marshal(custom)
	if err != nil || string(encoded) != `["A","b"]` {
		t.Fatal(string(encoded), err)
	}
	if CopyOfSet(custom) != custom {
		t.Fatal("CopyOfSet copied an immutable value")
	}
}

func TestMapDuplicateOrderAndBuilder(t *testing.T) {
	entries := []Entry[string, int]{{Key: "a", Value: 0}, {Key: "b", Value: 2}}
	value, err := NewMap(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Value = 99
	if got, found := value.Get("a"); !found || got != 0 {
		t.Fatal("source mutation or zero-value lookup", got, found)
	}
	if got := collect(value.Keys()); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
	if got := collect(value.Values()); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatal(got)
	}
	if _, err := NewMap([]Entry[string, int]{{Key: "a"}, {Key: "a"}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatal("duplicate key was accepted", err)
	}
	if _, err := NewMapFunc(lowerEqual, lowerHash, []Entry[string, int]{{Key: "a"}, {Key: "A"}}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatal("custom duplicate key was accepted", err)
	}
	builder := NewMapBuilder[string, int]()
	builder.Put("a", 1)
	first, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	builder.Put("b", 2)
	second, err := builder.Build()
	if err != nil || first.Len() != 1 || second.Len() != 2 {
		t.Fatal("map builder reuse failed", err)
	}
	if !first.Equal(newMapUnchecked(Entry[string, int]{Key: "a", Value: 1})) {
		t.Fatal("map equality failed")
	}
	encoded, err := json.Marshal(second)
	if err != nil || string(encoded) != `[{"key":"a","value":1},{"key":"b","value":2}]` {
		t.Fatal(string(encoded), err)
	}
	if CopyOfMap(second) != second {
		t.Fatal("CopyOfMap copied an immutable value")
	}
}

func newMapUnchecked[K comparable, V any](entries ...Entry[K, V]) *Map[K, V] {
	value, err := NewMap(entries)
	if err != nil {
		panic(err)
	}
	return value
}

func TestEmptyValuesMarshalAsArrays(t *testing.T) {
	emptyMap, err := NewMap[string, int](nil)
	if err != nil {
		t.Fatal(err)
	}
	emptySortedMap, err := NewSortedMap[string, int](nil)
	if err != nil {
		t.Fatal(err)
	}
	emptyBiMap, err := NewBiMap[string, int](nil)
	if err != nil {
		t.Fatal(err)
	}
	values := []any{
		NewList[int](nil), NewSet[int](nil), emptyMap, NewSortedSet[int](nil), emptySortedMap,
		NewMultiset[int](nil), NewMultimap[int, int](nil), emptyBiMap,
	}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != "[]" {
			t.Fatalf("empty %T encoded as %s, %v", value, encoded, err)
		}
	}
}

func TestImmutableConcurrentReads(t *testing.T) {
	list := NewList([]int{1, 2, 3})
	set := NewSet([]int{1, 2, 3})
	value := newMapUnchecked(Entry[string, int]{Key: "a", Value: 1})
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 1000 {
				list.At(2)
				list.SubList(1, 3).Len()
				set.Contains(2)
				for range list.All() {
				}
				value.Get("a")
				for range value.Entries() {
				}
			}
		}()
	}
	wait.Wait()
}

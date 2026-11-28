/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"slices"
	"sync"
	"testing"
)

func TestTwinPerKeyReaders(t *testing.T) {
	list, set, sorted := NewListMultimap[string, int](), NewSetMultimap[string, int](), NewSortedSetMultimap[string, int]()
	for _, value := range []int{3, 1, 3, 2} {
		list.Put("a", value)
		set.Put("a", value)
		sorted.Put("a", value)
	}
	for name, got := range map[string][2]any{
		"list":   {list.ValueCount("a"), slices.Collect(list.ValuesOf("a"))},
		"set":    {set.ValueCount("a"), len(slices.Collect(set.ValuesOf("a")))},
		"sorted": {sorted.ValueCount("a"), slices.Collect(sorted.ValuesOf("a"))},
	} {
		switch name {
		case "list":
			if got[0] != 4 || !slices.Equal(got[1].([]int), []int{3, 1, 3, 2}) {
				t.Fatalf("list: %v", got)
			}
		case "set":
			if got[0] != 3 || got[1] != 3 {
				t.Fatalf("set: %v", got)
			}
		case "sorted":
			if got[0] != 3 || !slices.Equal(got[1].([]int), []int{1, 2, 3}) {
				t.Fatalf("sorted: %v", got)
			}
		}
	}
	var nilList *ListMultimap[string, int]
	var nilSet *SetMultimap[string, int]
	var nilSorted *SortedSetMultimap[string, int]
	if nilList.ValueCount("a")+nilSet.ValueCount("a")+nilSorted.ValueCount("a") != 0 {
		t.Fatal("nil twin ValueCount not 0")
	}
	for range nilList.ValuesOf("a") {
		t.Fatal("nil twin yielded")
	}
	for range nilSet.ValuesOf("a") {
		t.Fatal("nil twin yielded")
	}
	for range nilSorted.ValuesOf("a") {
		t.Fatal("nil twin yielded")
	}
}

// TestTwinPerKeyReadersConcurrent reads ValueCount/ValuesOf while writers
// mutate the same key; run under -race. Every snapshot must be a consistent
// set of the key's values (each value from the writers' range, no tearing).
func TestTwinPerKeyReadersConcurrent(t *testing.T) {
	list, set, sorted := NewListMultimap[int, int](), NewSetMultimap[int, int](), NewSortedSetMultimap[int, int]()
	var wait sync.WaitGroup
	for writer := range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for step := range 2000 {
				value := writer*10000 + step
				list.Put(step%3, value)
				set.Put(step%3, value)
				sorted.Put(step%3, value)
				if step%5 == 0 {
					list.Remove(step%3, value)
					set.Remove(step%3, value)
					sorted.Remove(step%3, value)
				}
			}
		}()
	}
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for step := range 2000 {
				key := step % 3
				_ = list.ValueCount(key) + set.ValueCount(key) + sorted.ValueCount(key)
				for value := range list.ValuesOf(key) {
					if value < 0 || value >= 40000 {
						panic("torn list value")
					}
				}
				for range set.ValuesOf(key) {
				}
				previous := -1
				for value := range sorted.ValuesOf(key) {
					if value <= previous {
						panic("sorted snapshot out of order")
					}
					previous = value
				}
			}
		}()
	}
	wait.Wait()
}

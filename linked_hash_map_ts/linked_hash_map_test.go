/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map_ts

import (
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
)

func TestConcurrentUpdatesAndSnapshots(t *testing.T) {
	m := New[int, int]()
	const workers = 16
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := 0; index < 1000; index++ {
				key := (worker + index) % 100
				m.Put(key, index)
				m.Get(key)
				if index%7 == 0 {
					m.Delete(key)
				}
				if index%31 == 0 {
					for range m.All() {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	sequence := m.Entries()
	m.Put(1000, 1)
	count := 0
	for range sequence {
		count++
	}
	if count != m.Len()-1 {
		t.Fatal("iterator was not a snapshot", count, m.Len())
	}
}

func TestParityAndOppositeEqual(t *testing.T) {
	left := FromEntries([]Entry[int, int]{{Key: 2, Value: 20}, {Key: 1, Value: 10}})
	right := FromEntries([]Entry[int, int]{{Key: 1, Value: 10}, {Key: 2, Value: 20}})
	if !left.Equal(right, func(a, b int) bool { return a == b }) || !right.Equal(left, func(a, b int) bool { return a == b }) {
		t.Fatal("content equality failed")
	}
	if got := slices.Collect(left.Keys()); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatal(got)
	}
	data, err := left.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err = right.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left.EntriesSlice(), right.EntriesSlice()) {
		t.Fatal("JSON order mismatch")
	}
}

func ExampleMap() {
	m := New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	for entry := range m.Entries() {
		fmt.Println(entry.Key, entry.Value)
	}
	// Output:
	// a 1
	// b 2
}

func TestNilAndEmptyEqual(t *testing.T) {
	var nilMap *Map[int, int]
	empty := New[int, int]()
	equal := func(a, b int) bool { return a == b }
	if !nilMap.Equal(empty, equal) || !empty.Equal(nilMap, equal) || !nilMap.Equal(nil, equal) {
		t.Fatal("nil and empty maps should have equal content")
	}
	if err := empty.UnmarshalJSON([]byte(`[]`)); err != nil {
		t.Fatal(err)
	}
}

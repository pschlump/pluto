/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"reflect"
	"slices"
	"sync"
	"testing"
)

func TestLinkedMultimapOrderingAndConcurrentSnapshots(t *testing.T) {
	m := NewLinkedHashMultimap[int, int]()
	m.PutAll(2, []int{4, 3, 4})
	m.Put(1, 9)
	if got := slices.Collect(m.KeySet()); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatal(got)
	}
	var wg sync.WaitGroup
	for worker := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range 400 {
				key, value := index%20, (worker+index)%30
				m.Put(key, value)
				m.Get(key)
				if index%7 == 0 {
					m.Remove(key, value)
				}
				if index%29 == 0 {
					for range m.Entries() {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	snapshot := m.EntriesSlice()
	iterator := m.Entries()
	m.Put(1000, 1000)
	count := 0
	for range iterator {
		count++
	}
	if count != len(snapshot) {
		t.Fatal("iterator was not a snapshot")
	}
}

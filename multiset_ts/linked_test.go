/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset_ts

import (
	"reflect"
	"sync"
	"testing"

	"github.com/pschlump/pluto/multiset"
)

func TestLinkedHashMultisetConcurrentAndSnapshot(t *testing.T) {
	m := NewLinkedHashMultiset[int]()
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range 500 {
				m.Add(index%50, 1)
				if index%7 == 0 {
					m.Remove(index%50, 1)
				}
				if index%31 == 0 {
					for range m.Entries() {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	snapshot := m.Entries()
	before := m.DistinctLen()
	m.Add(1000, 1)
	count := 0
	for range snapshot {
		count++
	}
	if count != before {
		t.Fatal("iterator was not a snapshot")
	}
}

func TestLinkedHashMultisetOppositeAlgebra(t *testing.T) {
	left := FromSliceLinked([]int{1, 2, 2})
	right := FromSliceLinked([]int{2, 3})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			var destination LinkedHashMultiset[int]
			destination.Sum(left, right)
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			var destination LinkedHashMultiset[int]
			destination.Sum(right, left)
		}
	}()
	wg.Wait()
	if got := left.EntriesSlice(); !reflect.DeepEqual(got, []multiset.Entry[int]{{Value: 1, Count: 1}, {Value: 2, Count: 2}}) {
		t.Fatal(got)
	}
}

func TestLinkedHashMultisetNilOperands(t *testing.T) {
	var nilSet *LinkedHashMultiset[int]
	empty := NewLinkedHashMultiset[int]()
	if nilSet.Add(1, 0) != 0 || nilSet.SetCount(1, 0) != 0 || nilSet.Remove(1, 1) != 0 {
		t.Fatal("nil non-storing mutation mismatch")
	}
	if !nilSet.Equal(empty) || !empty.Equal(nilSet) || !nilSet.ContainsOccurrences(empty) || !empty.ContainsOccurrences(nilSet) {
		t.Fatal("nil/empty equality or containment mismatch")
	}
	empty.Sum(nilSet, nilSet)
	empty.Union(nilSet, nilSet)
	empty.Intersection(nilSet, nilSet)
	empty.Minus(nilSet, nilSet)
	nilSet.Sum(empty, empty)
	if !empty.IsEmpty() {
		t.Fatal("nil algebra produced values")
	}
}

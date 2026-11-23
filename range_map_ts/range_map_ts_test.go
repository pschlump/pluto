/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_map_ts

import (
	"sync"
	"testing"

	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/ranges"
)

func TestRangeMapConcurrentSnapshotsAndEquality(t *testing.T) {
	value := NewRangeMap[int, int]()
	value.Put(ranges.Closed(1, 3), 1)
	iterator := value.Entries()
	value.Put(ranges.Closed(5, 7), 2)
	if got := entriesToSlice(iterator); len(got) != 1 {
		t.Fatal("iterator not snapshot", got)
	}
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 500; iteration++ {
				interval := ranges.Closed(worker, worker+iteration%4)
				value.Put(interval, worker)
				value.Get(iteration)
				value.RangeContaining(iteration)
				value.EntriesSlice()
				value.Entries()
				if iteration%3 == 0 {
					value.Remove(interval)
				}
			}
		}(worker)
	}
	wait.Wait()
	clone := value.Clone()
	if !value.Equal(clone, func(a, b int) bool { return a == b }) {
		t.Fatal("clone differs")
	}
	clone.Clear()
	if value.IsEmpty() {
		t.Fatal("clone clear changed source")
	}
	left, right := NewRangeMap[int, int](), NewRangeMap[int, int]()
	left.Put(ranges.Closed(1, 2), 1)
	right.Put(ranges.Closed(1, 2), 1)
	equal := func(a, b int) bool { return a == b }
	wait.Add(2)
	go func() {
		defer wait.Done()
		for range 500 {
			left.Equal(right, equal)
		}
	}()
	go func() {
		defer wait.Done()
		for range 500 {
			right.Equal(left, equal)
		}
	}()
	wait.Wait()
}

func entriesToSlice(sequence func(func(range_map.Entry[int, int]) bool)) []range_map.Entry[int, int] {
	result := []range_map.Entry[int, int]{}
	for value := range sequence {
		result = append(result, value)
	}
	return result
}

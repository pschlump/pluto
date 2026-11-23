/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_set_ts

import (
	"sync"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeSetConcurrentSnapshotsAndEquality(t *testing.T) {
	set := NewRangeSet[int]()
	set.Add(ranges.Closed(1, 3))
	iterator := set.All()
	set.Add(ranges.Closed(5, 7))
	if got := iteratorToSlice(iterator); len(got) != 1 {
		t.Fatal("iterator not snapshot", got)
	}
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 500; iteration++ {
				value := ranges.Closed(worker, worker+iteration%4)
				set.Add(value)
				set.Contains(iteration)
				set.RangeContaining(iteration)
				set.Slice()
				set.All()
				if iteration%3 == 0 {
					set.Remove(value)
				}
			}
		}(worker)
	}
	wait.Wait()
	clone := set.Clone()
	if !set.Equal(clone) {
		t.Fatal("clone differs")
	}
	clone.Clear()
	if set.IsEmpty() {
		t.Fatal("clone clear changed source")
	}
	left, right := NewRangeSet[int](), NewRangeSet[int]()
	left.Add(ranges.Closed(1, 2))
	right.Add(ranges.Closed(1, 2))
	wait.Add(2)
	go func() {
		defer wait.Done()
		for range 500 {
			left.Equal(right)
		}
	}()
	go func() {
		defer wait.Done()
		for range 500 {
			right.Equal(left)
		}
	}()
	wait.Wait()
}

func iteratorToSlice(sequence func(func(ranges.Range[int]) bool)) []ranges.Range[int] {
	result := []ranges.Range[int]{}
	for value := range sequence {
		result = append(result, value)
	}
	return result
}

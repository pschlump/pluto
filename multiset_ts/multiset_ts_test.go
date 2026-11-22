/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset_ts

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestConcurrentAccess(t *testing.T) {
	set := NewHashMultiset[int]()
	const writerCount = 8
	const additions = 1000
	var readers sync.WaitGroup
	var writers sync.WaitGroup
	done := make(chan struct{})
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = set.Len()
				_ = set.DistinctLen()
			}
		}
	}()
	for writer := 0; writer < writerCount; writer++ {
		writers.Add(1)
		go func(value int) {
			defer writers.Done()
			for range additions {
				set.Add(value, 1)
			}
		}(writer)
	}
	writers.Wait()
	close(done)
	readers.Wait()
	if set.DistinctLen() != writerCount {
		t.Fatal(set.DistinctLen())
	}
	for value := 0; value < writerCount; value++ {
		if set.Count(value) != additions {
			t.Fatalf("Count(%d)=%d", value, set.Count(value))
		}
	}
}

func TestOppositeDirectionOperations(t *testing.T) {
	a := FromSlice([]int{1, 1, 2})
	b := FromSlice([]int{1, 2, 2, 3})
	done := make(chan struct{})
	go func() {
		for range 100 {
			if !a.ContainsOccurrences(a) || !b.ContainsOccurrences(b) {
				panic("self containment failed")
			}
			_ = a.Equal(b)
			_ = b.Equal(a)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opposite-direction operations deadlocked")
	}
}

func TestSnapshotIteration(t *testing.T) {
	set := FromSlice([]int{1, 1, 2, 3})
	entries := set.EntriesSlice()
	all := set.All()
	set.Clear()
	seen := 0
	for range all {
		seen++
	}
	if seen != 4 || len(entries) != 3 || set.Len() != 0 {
		t.Fatal("iterator did not retain an independent snapshot")
	}
}

func TestBatchOverflowIsAtomic(t *testing.T) {
	set := NewHashMultiset[int]()
	maximum := int(^uint(0) >> 1)
	set.SetCount(1, maximum)
	defer func() {
		if recover() == nil {
			t.Fatal("expected overflow panic")
		}
		if set.Len() != maximum || set.Count(2) != 0 {
			t.Fatal("failed batch changed the multiset")
		}
	}()
	set.AddSlice([]int{2})
}

func TestJSONAndClone(t *testing.T) {
	set := FromSlice([]int{1, 1, 2})
	encoded, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewHashMultiset[int]()
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(set) {
		t.Fatal("JSON round trip changed counts")
	}
	clone := set.Clone()
	clone.Add(2, 3)
	if set.Count(2) != 1 || clone.Count(2) != 4 || !clone.ContainsOccurrences(set) {
		t.Fatal("clone is not independent")
	}
}

func TestZeroCountReadOnlyValues(t *testing.T) {
	var set HashMultiset[int]
	if set.Add(1, 0) != 0 || set.SetCount(1, 0) != 0 || set.Len() != 0 {
		t.Fatal("zero-count mutations changed zero value")
	}
}

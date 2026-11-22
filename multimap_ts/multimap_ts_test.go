/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"iter"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/pschlump/pluto/multimap"
)

type twinMultimap interface {
	Put(int, int) bool
	PutAll(int, []int) int
	Get(int) ([]int, bool)
	Remove(int, int) bool
	ReplaceValues(int, []int) []int
	Len() int
	KeyLen() int
	Entries() iter.Seq2[int, int]
}

func TestTwinParity(t *testing.T) {
	for _, test := range []struct {
		name string
		new  func() twinMultimap
	}{
		{name: "list", new: func() twinMultimap { return NewListMultimap[int, int]() }},
		{name: "set", new: func() twinMultimap { return NewSetMultimap[int, int]() }},
		{name: "sorted", new: func() twinMultimap { return NewSortedSetMultimap[int, int]() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := test.new()
			set.Put(1, 2)
			set.Put(1, 2)
			set.PutAll(1, []int{3, 1})
			if test.name == "list" {
				if values, _ := set.Get(1); !reflect.DeepEqual(values, []int{2, 2, 3, 1}) {
					t.Fatal(values)
				}
			} else {
				values, _ := set.Get(1)
				if test.name == "set" {
					sort.Ints(values)
				}
				if !sort.IntsAreSorted(values) || len(values) != 3 {
					t.Fatal(values)
				}
			}
			set.Remove(1, 2)
			set.ReplaceValues(2, []int{8, 9})
			if set.Len() == 0 || set.KeyLen() != 2 {
				t.Fatal(set.Len(), set.KeyLen())
			}
		})
	}
}

func TestConcurrentWritersAndReaders(t *testing.T) {
	set := NewListMultimap[int, int]()
	const writerCount = 8
	const valuesPerWriter = 500
	var writers sync.WaitGroup
	var readers sync.WaitGroup
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
				_, _ = set.Get(3)
			}
		}
	}()
	for writer := 0; writer < writerCount; writer++ {
		writers.Add(1)
		go func(key int) {
			defer writers.Done()
			for value := 0; value < valuesPerWriter; value++ {
				set.Put(key, value)
			}
		}(writer)
	}
	writers.Wait()
	close(done)
	readers.Wait()
	if set.Len() != writerCount*valuesPerWriter || set.KeyLen() != writerCount {
		t.Fatal(set.Len(), set.KeyLen())
	}
}

func TestBatchAtomicity(t *testing.T) {
	set := NewListMultimap[int, int]()
	set.Put(1, 0)
	done := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-done:
				return
			default:
				values, _ := set.Get(1)
				if len(values) != 1 && len(values) != 100 && len(values) != 200 {
					t.Errorf("partial batch visible: %d", len(values))
					return
				}
			}
		}
	}()
	hundred := make([]int, 100)
	twoHundred := make([]int, 200)
	for range 100 {
		set.ReplaceValues(1, hundred)
		set.PutAll(1, twoHundred[:100])
		set.ReplaceValues(1, []int{0})
	}
	close(done)
	readers.Wait()
}

func TestSnapshotIteration(t *testing.T) {
	set := NewSetMultimap[int, int]()
	set.PutAll(1, []int{1, 2, 3})
	entries := set.Entries()
	set.Clear()
	count := 0
	for range entries {
		count++
	}
	if count != 3 || set.Len() != 0 {
		t.Fatal("snapshot iteration failed")
	}
}

func TestOppositeInversionsDoNotDeadlock(t *testing.T) {
	a := NewListMultimap[int, int]()
	b := NewListMultimap[int, int]()
	a.PutAll(1, []int{2, 3})
	b.PutAll(2, []int{3, 4})
	done := make(chan struct{})
	go func() {
		for range 5 {
			InvertFrom[int, int](a, b)
			InvertFrom[int, int](b, a)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opposite inversions deadlocked")
	}
}

var _ multimap.Reader[int, int] = (*ListMultimap[int, int])(nil)
var _ multimap.Reader[int, int] = (*SetMultimap[int, int])(nil)
var _ multimap.Reader[int, int] = (*SortedSetMultimap[int, int])(nil)

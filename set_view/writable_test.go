/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package set_view

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/pschlump/pluto/hash_set"
	"github.com/pschlump/pluto/hash_set_ts"
)

func TestFilteredMutableView(t *testing.T) {
	set := hash_set.FromSlice([]int{1, 2, 3, 4})
	view := Filter[int](set, func(value int) bool { return value%2 == 0 })
	if got := slices.Collect(view.All()); len(got) != 2 || view.Len() != 2 {
		t.Fatal(got)
	}
	if _, err := view.Insert(5); !errors.Is(err, ErrOutsideView) || set.Contains(5) {
		t.Fatal("rejected write reached parent", err)
	}
	if _, err := view.Insert(6); err != nil || !set.Contains(6) {
		t.Fatal("view insert failed", err)
	}
	set.Insert(8)
	if !view.Contains(8) || view.Delete(3) || !set.Contains(3) {
		t.Fatal("filter visibility or hidden delete failed")
	}
	view.Truncate()
	got := set.Slice()
	if len(got) != 2 {
		t.Fatal(got)
	}
	for _, value := range got {
		if value%2 == 0 {
			t.Fatal("visible value survived truncate")
		}
	}
}

func TestWrapAndThreadSafeView(t *testing.T) {
	plain := hash_set.NewHashSet[int]()
	wrapped := Wrap[int](plain)
	_, _ = wrapped.Insert(1)
	wrapped.Truncate()
	if plain.Len() != 0 {
		t.Fatal("wrapped truncate failed")
	}
	set := hash_set_ts.NewHashSet[int]()
	view := Filter[int](set, func(value int) bool { return value >= 0 })
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range 500 {
				_, _ = view.Insert(index % 100)
				view.Contains(index % 100)
				if index%7 == 0 {
					view.Delete(index % 100)
				}
			}
		}()
	}
	wg.Wait()
	for value := range view.All() {
		if value < 0 {
			t.Fatal("filtered value exposed")
		}
	}
}

func ExampleFilter() {
	set := hash_set.FromSlice([]int{1, 2, 3, 4})
	evens := Filter[int](set, func(value int) bool { return value%2 == 0 })
	_, _ = evens.Insert(6)
	values := slices.Collect(evens.All())
	slices.Sort(values)
	fmt.Println(values)
	// Output: [2 4 6]
}

func TestWrappedViewMatchesSet(t *testing.T) {
	set := hash_set.FromSlice([]int{1, 2})
	view := Wrap[int](set)
	if got := slices.Collect(view.All()); len(got) != 2 || reflect.DeepEqual(got, []int{}) {
		t.Fatal(got)
	}
}

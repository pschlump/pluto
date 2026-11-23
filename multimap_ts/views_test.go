/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"sync"
	"testing"
)

func TestConcurrentViewWrites(t *testing.T) {
	m := NewLinkedListMultimap[int, int]()
	view := m.GetView(1)
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range 200 {
				view.Add(worker*1000 + index)
				if index%5 == 0 {
					view.Remove(worker*1000 + index)
				}
				view.Len()
			}
		}()
	}
	wg.Wait()
	if got, want := m.Len(), view.Len(); got != want {
		t.Fatal(got, want)
	}
	view.Clear()
	if m.ContainsKey(1) {
		t.Fatal("view clear retained key")
	}
}

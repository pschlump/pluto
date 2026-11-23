/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"sync"
	"testing"
)

func TestConcurrentRowView(t *testing.T) {
	table := NewHashTable[int, int, int]()
	row := table.RowView(1)
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range 200 {
				column := worker*1000 + index
				_, _, _ = row.Put(column, column)
				row.Get(column)
				if index%5 == 0 {
					_, _, _ = row.Delete(column)
				}
			}
		}()
	}
	wg.Wait()
	if row.Len() != table.Len() {
		t.Fatal(row.Len(), table.Len())
	}
	if err := row.Clear(); err != nil || table.Len() != 0 {
		t.Fatal("row clear failed", err)
	}
}

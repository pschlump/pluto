/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pschlump/pluto/table"
)

func TestConcurrentHashTableAndSnapshots(t *testing.T) {
	actual := NewHashTable[int, int, int]()
	const writers = 8
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		wait.Add(1)
		go func(row int) {
			defer wait.Done()
			for column := 0; column < 100; column++ {
				actual.Put(row%4, column, row*100+column)
				if column%3 == 0 {
					actual.Delete(row%4, column)
				}
			}
		}(index)
	}
	wait.Wait()
	cells := actual.Cells()
	rows := actual.Rows()
	clone := actual.Clone()
	actual.Clear()
	if count := len(snapshotCells(cells)); count == 0 {
		t.Fatal("cell snapshot was not taken before Clear")
	}
	rowCount := 0
	for _, cells := range rows {
		rowCount++
		if len(cells) == 0 {
			t.Fatal("sparse snapshot contains empty row")
		}
	}
	if rowCount == 0 || clone.Len() == 0 || actual.Len() != 0 {
		t.Fatal("snapshot or clone failed")
	}
}

func TestTreeAndArrayTwinParity(t *testing.T) {
	tree := NewTreeTable[int, string, int]()
	tree.Put(2, "b", 2)
	tree.Put(1, "a", 1)
	if got := snapshotCells(tree.Cells()); len(got) != 2 || got[0].Row != 1 {
		t.Fatal("tree twin ordering failed", got)
	}
	if value, found := tree.Transpose().Get("a", 1); !found || value != 1 {
		t.Fatal("tree twin transpose failed")
	}
	array := NewArrayTable[int, string, int]([]int{1, 2}, []string{"a", "b"})
	if _, _, err := array.Put(1, "a", 0); err != nil {
		t.Fatal(err)
	}
	if !array.ContainsRow(2) || array.Len() != 1 {
		t.Fatal("array twin fixed universe failed")
	}
	array.PutAt(1, 1, 2)
	if value, found, err := array.Delete(1, "a"); err != nil || !found || value != 0 {
		t.Fatal("array twin deletion failed", value, found, err)
	}
	if value, found := array.Transpose().GetAt(1, 1); !found || value != 2 {
		t.Fatal("array twin transpose failed")
	}
}

func TestAtomicReplacementAndJSON(t *testing.T) {
	actual := NewHashTable[int, string, int]()
	actual.Put(1, "old", 1)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for index := 0; index < 100; index++ {
			actual.Replace([]table.Cell[int, string, int]{{Row: 2, Column: "new", Value: index}})
		}
	}()
	go func() {
		defer wait.Done()
		for range 100 {
			cells := snapshotCells(actual.Cells())
			if len(cells) == 1 && cells[0].Column != "old" && cells[0].Column != "new" {
				t.Error("partial replacement visible")
			}
		}
	}()
	wait.Wait()
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewHashTable[int, string, int]()
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Len() != 1 || !decoded.Contains(2, "new") {
		t.Fatal("hash twin JSON round trip failed")
	}
	array := NewArrayTable[int, string, int]([]int{1}, []string{"a"})
	if err := json.Unmarshal([]byte(`[{"row":2,"column":"a","value":1}]`), array); err == nil || array.Len() != 0 {
		t.Fatal("invalid array JSON changed table", err)
	}
}

func TestSnapshotsDoNotHoldLockDuringIteration(t *testing.T) {
	actual := NewTreeTable[int, int, int]()
	actual.Put(1, 1, 1)
	done := make(chan struct{})
	go func() {
		for range actual.Cells() {
			actual.Put(2, 2, 2)
		}
		for range actual.Rows() {
			actual.Delete(2, 2)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot iterator held lock during caller code")
	}
}

func TestConcurrentArrayAndTreeWriters(t *testing.T) {
	tree := NewTreeTable[int, int, string]()
	array := NewArrayTable[int, int, string]([]int{0, 1, 2, 3}, []int{0, 1, 2, 3})
	var wait sync.WaitGroup
	for index := 0; index < 4; index++ {
		wait.Add(1)
		go func(key int) {
			defer wait.Done()
			value := fmt.Sprintf("value-%d", key)
			tree.Put(key, key, value)
			if _, _, err := array.Put(key, key, value); err != nil {
				t.Error(err)
			}
		}(index)
	}
	wait.Wait()
	if tree.Len() != 4 || array.Len() != 4 {
		t.Fatal("concurrent writes lost", tree.Len(), array.Len())
	}
}

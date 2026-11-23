/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"errors"
	"fmt"
	"testing"
)

func TestHashTableLiveViews(t *testing.T) {
	table := NewHashTable[string, string, int]()
	row := table.RowView("r")
	column := table.ColumnView("c")
	cell := table.CellView("r", "c")
	if _, _, err := row.Put("c", 1); err != nil {
		t.Fatal(err)
	}
	if value, found := table.Get("r", "c"); value != 1 || !found || !cell.Exists() {
		t.Fatal("row write not visible", value, found)
	}
	if _, _, err := column.Put("other", 2); err != nil {
		t.Fatal(err)
	}
	if value, _ := cell.Get(); value != 1 {
		t.Fatal(value)
	}
	table.Put("r", "c", 3)
	if value, _ := cell.Get(); value != 3 || row.Len() != 1 || column.Len() != 2 {
		t.Fatal("parent update not visible")
	}
	if err := row.Clear(); err != nil {
		t.Fatal(err)
	}
	if table.ContainsRow("r") || column.Len() != 1 {
		t.Fatal("row clear failed")
	}
	if _, _, err := column.Put("r", 4); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cell.Delete(); err != nil || table.Len() != 1 {
		t.Fatal("cell delete failed", err)
	}
}

func TestTreeAndArrayTableViews(t *testing.T) {
	tree := NewTreeTable[string, int, int]()
	row := tree.RowView("a")
	_, _, _ = row.Put(2, 2)
	_, _, _ = row.Put(1, 1)
	got := row.Cells()
	if len(got) != 2 || got[0].Row != "a" || got[0].Column != 1 || got[0].Value != 1 || got[1].Column != 2 || got[1].Value != 2 {
		t.Fatal(got)
	}
	array := NewArrayTable[string, int, int]([]string{"a"}, []int{1, 2})
	arrayRow := array.RowView("a")
	_, _, _ = arrayRow.Put(2, 20)
	if value, found := array.Get("a", 2); value != 20 || !found {
		t.Fatal(value, found)
	}
	if _, _, err := array.RowView("missing").Put(1, 1); !errors.Is(err, ErrOutsideUniverse) {
		t.Fatal("out-of-universe view write succeeded", err)
	}
	if err := arrayRow.Clear(); err != nil || array.Len() != 0 {
		t.Fatal("array row clear failed", err)
	}
}

func ExampleRowView() {
	table := NewHashTable[string, string, int]()
	row := table.RowView("row")
	_, _, _ = row.Put("a", 1)
	_, _, _ = row.Put("b", 2)
	fmt.Println(row.Len(), table.Len())
	// Output: 2 2
}

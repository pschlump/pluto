/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"cmp"
	"encoding/json"
	"errors"
	"hash/fnv"
	"iter"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func sortedCells[R cmp.Ordered, C cmp.Ordered, V any](sequence iter.Seq[Cell[R, C, V]]) []Cell[R, C, V] {
	cells := collectCells(sequence)
	slices.SortFunc(cells, func(a, b Cell[R, C, V]) int {
		if order := cmp.Compare(a.Row, b.Row); order != 0 {
			return order
		}
		return cmp.Compare(a.Column, b.Column)
	})
	return cells
}

func TestHashTableSeededModel(t *testing.T) {
	random := rand.New(rand.NewSource(1729))
	actual := NewHashTable[int, int, string]()
	model := make(map[int]map[int]string)
	for step := 0; step < 4000; step++ {
		row, column := random.Intn(30), random.Intn(20)
		value := string(rune('a' + random.Intn(8)))
		switch random.Intn(4) {
		case 0:
			old, found := actual.Put(row, column, value)
			want, expected := model[row][column]
			if old != want || found != expected {
				t.Fatalf("step %d Put old=%q/%v want=%q/%v", step, old, found, want, expected)
			}
			if model[row] == nil {
				model[row] = make(map[int]string)
			}
			model[row][column] = value
		case 1:
			old, found := actual.Delete(row, column)
			want, expected := model[row][column]
			if old != want || found != expected {
				t.Fatalf("step %d Delete old=%q/%v want=%q/%v", step, old, found, want, expected)
			}
			if expected {
				delete(model[row], column)
				if len(model[row]) == 0 {
					delete(model, row)
				}
			}
		case 2:
			got, found := actual.Get(row, column)
			want, expected := model[row][column]
			if got != want || found != expected {
				t.Fatalf("step %d Get got=%q/%v want=%q/%v", step, got, found, want, expected)
			}
		case 3:
			if got, want := actual.ContainsRow(row), model[row] != nil; got != want {
				t.Fatalf("step %d ContainsRow=%v want=%v", step, got, want)
			}
		}
		if step%40 == 0 {
			checkHashModel(t, actual, model)
		}
	}
}

func checkHashModel(t *testing.T, actual *HashTable[int, int, string], model map[int]map[int]string) {
	t.Helper()
	wantCells := []Cell[int, int, string]{}
	for row, columns := range model {
		for column, value := range columns {
			wantCells = append(wantCells, Cell[int, int, string]{Row: row, Column: column, Value: value})
		}
	}
	slices.SortFunc(wantCells, func(a, b Cell[int, int, string]) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}
		return a.Column - b.Column
	})
	if got := sortedCells(actual.Cells()); !reflect.DeepEqual(got, wantCells) {
		t.Fatalf("cells mismatch\ngot:  %#v\nwant: %#v", got, wantCells)
	}
	if actual.Len() != len(wantCells) {
		t.Fatalf("Len=%d want=%d", actual.Len(), len(wantCells))
	}
	for row := range model {
		if !actual.ContainsRow(row) {
			t.Fatal("missing model row")
		}
	}
}

func TestTreeTableOrderingAndTranspose(t *testing.T) {
	actual := NewTreeTable[string, int, int]()
	actual.Put("b", 2, 22)
	actual.Put("a", 3, 13)
	actual.Put("a", 1, 11)
	actual.Put("c", 1, 31)
	cells := collectCells(actual.Cells())
	want := []Cell[string, int, int]{{"a", 1, 11}, {"a", 3, 13}, {"b", 2, 22}, {"c", 1, 31}}
	if !reflect.DeepEqual(cells, want) {
		t.Fatalf("ordered cells=%#v", cells)
	}
	if got := actual.Column(1); !reflect.DeepEqual(got, []Cell[string, int, int]{{"a", 1, 11}, {"c", 1, 31}}) {
		t.Fatalf("ordered column=%#v", got)
	}
	transposed := actual.Transpose()
	wantTransposed := []Cell[int, string, int]{{1, "a", 11}, {1, "c", 31}, {2, "b", 22}, {3, "a", 13}}
	if got := collectCells(transposed.Cells()); !reflect.DeepEqual(got, wantTransposed) {
		t.Fatalf("transpose=%#v", got)
	}
	if got := collectCells(transposed.Transpose().Cells()); !reflect.DeepEqual(got, want) {
		t.Fatalf("transpose round trip=%#v", got)
	}
	actual.Delete("a", 1)
	actual.Delete("a", 3)
	if actual.ContainsRow("a") || len(actual.Row("a")) != 0 {
		t.Fatal("empty tree row retained")
	}
}

func TestTreeTableCustomOrdering(t *testing.T) {
	compare := func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }
	actual := NewTreeTableFunc[string, string, int](compare, compare)
	actual.Put("B", "Y", 1)
	actual.Put("a", "X", 2)
	if _, found := actual.Get("A", "x"); !found {
		t.Fatal("custom comparison lookup failed")
	}
	keys := []string{}
	for key := range actual.RowKeys() {
		keys = append(keys, key)
	}
	if !reflect.DeepEqual(keys, []string{"a", "B"}) {
		t.Fatal(keys)
	}
	frozen := actual.Freeze()
	if value, found := frozen.Get("A", "x"); !found || value != 2 {
		t.Fatal("ordered immutable lookup failed")
	}
	if cells := frozen.Row("a"); len(cells) != 1 || cells[0].Value != 2 {
		t.Fatal("ordered immutable row lookup failed", cells)
	}
}

func TestArrayTableDenseModelAndZeroValues(t *testing.T) {
	random := rand.New(rand.NewSource(99))
	actual := NewArrayTable[int, int, int]([]int{0, 1, 2, 1}, []int{0, 1, 2})
	model := [3][3]int{}
	present := [3][3]bool{}
	if !actual.ContainsRow(1) || !actual.ContainsColumn(2) || actual.ContainsRow(3) {
		t.Fatal("dense universe membership failed")
	}
	for step := 0; step < 2000; step++ {
		row, column := random.Intn(3), random.Intn(3)
		switch random.Intn(3) {
		case 0:
			value := random.Intn(3)
			old, found, err := actual.Put(row, column, value)
			if err != nil || old != model[row][column] || found != present[row][column] {
				t.Fatal("dense Put mismatch", err)
			}
			model[row][column], present[row][column] = value, true
		case 1:
			old, found, err := actual.Delete(row, column)
			if err != nil || old != model[row][column] || found != present[row][column] {
				t.Fatal("dense Delete mismatch", err)
			}
			model[row][column], present[row][column] = 0, false
		case 2:
			got, found := actual.GetAt(row, column)
			if got != model[row][column] || found != present[row][column] {
				t.Fatal("dense GetAt mismatch")
			}
		}
	}
	count := 0
	for cell := range actual.Cells() {
		count++
		if !present[cell.Row][cell.Column] || model[cell.Row][cell.Column] != cell.Value {
			t.Fatal("dense cell mismatch")
		}
	}
	if count != actual.Len() {
		t.Fatal("dense length mismatch")
	}
	actual.PutAt(0, 0, 0)
	if value, found := actual.GetAt(0, 0); !found || value != 0 {
		t.Fatal("zero value became absent")
	}
	actual.DeleteAt(0, 0)
	if _, found := actual.GetAt(0, 0); found {
		t.Fatal("deleted zero value remains")
	}
	if _, _, err := actual.Put(9, 0, 1); !errors.Is(err, ErrOutsideUniverse) {
		t.Fatal("out-of-universe Put error", err)
	}
}

func TestArrayUniversesSnapshotsAndClone(t *testing.T) {
	actual := NewArrayTable[string, string, int]([]string{"a", "b"}, []string{"x", "y"})
	_, _, _ = actual.Put("a", "x", 1)
	rows := actual.Rows()
	columns := actual.Columns()
	actual.Clear()
	rowCount, columnCount := 0, 0
	for _, cells := range rows {
		rowCount++
		if len(cells) > 1 {
			t.Fatal("unexpected cells")
		}
	}
	for range columns {
		columnCount++
	}
	if rowCount != 2 || columnCount != 2 {
		t.Fatal("dense snapshots lost empty universe keys", rowCount, columnCount)
	}
	_, _, _ = actual.Put("b", "y", 2)
	clone := actual.Clone()
	transposed := actual.Transpose()
	actual.Clear()
	if got, found := clone.Get("b", "y"); !found || got != 2 {
		t.Fatal("clone aliases source")
	}
	if got, found := transposed.Get("y", "b"); !found || got != 2 {
		t.Fatal("transpose aliases source")
	}
	if !clone.ContainsRow("a") || !clone.ContainsRow("b") {
		t.Fatal("fixed empty row semantics failed")
	}
}

func TestCustomHashAndImmutableSnapshot(t *testing.T) {
	type key struct {
		name string
		data []int
	}
	equal := func(a, b key) bool { return strings.EqualFold(a.name, b.name) }
	hash := func(value key) uint64 {
		digest := fnv.New64a()
		_, _ = digest.Write([]byte(strings.ToLower(value.name)))
		return digest.Sum64()
	}
	actual := NewHashTableFunc[key, key, int](equal, hash, equal, hash)
	row, column := key{name: "A", data: []int{1}}, key{name: "X", data: []int{2}}
	actual.Put(row, column, 0)
	if value, found := actual.Get(key{name: "a"}, key{name: "x"}); !found || value != 0 {
		t.Fatal("custom equality or zero value failed")
	}
	frozen := actual.Freeze()
	rows := frozen.Rows()
	actual.Put(key{name: "B"}, key{name: "Y"}, 2)
	if frozen.Len() != 1 || frozen.ContainsRow(key{name: "b"}) {
		t.Fatal("immutable snapshot changed with source")
	}
	for _, cells := range rows {
		cells[0].Value = 99
	}
	if got, _ := frozen.Get(row, column); got != 0 {
		t.Fatal("returned row aliases immutable cells")
	}
	if got, found := frozen.Transpose().Get(column, row); !found || got != 0 {
		t.Fatal("immutable transpose failed")
	}
}

func TestJSONRoundTripsAndValidation(t *testing.T) {
	hash := NewHashTable[int, string, int]()
	hash.Put(1, "x", 0)
	hash.Put(2, "y", 2)
	encoded, err := json.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewHashTable[int, string, int]()
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortedCells(hash.Cells()), sortedCells(decoded.Cells())) {
		t.Fatal("hash JSON round trip failed")
	}
	tree := NewTreeTable[int, string, int]()
	if err := json.Unmarshal(encoded, tree); err != nil {
		t.Fatal(err)
	}
	if got := collectCells(tree.Cells()); len(got) != 2 || got[0].Row != 1 || got[0].Value != 0 {
		t.Fatal("tree JSON round trip failed", got)
	}
	dense := NewArrayTable[int, string, int]([]int{1, 2}, []string{"x", "y"})
	if err := json.Unmarshal(encoded, dense); err != nil {
		t.Fatal(err)
	}
	before := collectCells(dense.Cells())
	invalid := []byte(`[{"row":9,"column":"x","value":3}]`)
	if err := json.Unmarshal(invalid, dense); !errors.Is(err, ErrOutsideUniverse) {
		t.Fatal("dense JSON accepted unknown key", err)
	}
	if got := collectCells(dense.Cells()); !reflect.DeepEqual(got, before) {
		t.Fatal("failed dense JSON changed table")
	}
	frozenEncoded, err := json.Marshal(dense.Freeze())
	if err != nil {
		t.Fatal(err)
	}
	var frozenCells []Cell[int, string, int]
	if err := json.Unmarshal(frozenEncoded, &frozenCells); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortedCells(cellSequence(frozenCells)), sortedCells(dense.Cells())) {
		t.Fatal("immutable JSON differs", string(frozenEncoded))
	}
}

func TestIterationEarlyStop(t *testing.T) {
	actual := NewTreeTable[int, int, int]()
	for index := 0; index < 10; index++ {
		actual.Put(index, index, index)
	}
	count := 0
	for range actual.Cells() {
		count++
		break
	}
	if count != 1 {
		t.Fatal(count)
	}
}

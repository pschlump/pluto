/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"encoding/json"
	"iter"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type tableGolden struct {
	Version     int                 `json:"version"`
	GuavaCommit string              `json:"guavaCommit"`
	Variants    tableGoldenVariants `json:"variants"`
}

type tableGoldenVariants struct {
	Hash  tableGoldenVariant `json:"hash"`
	Tree  tableGoldenVariant `json:"tree"`
	Array tableGoldenVariant `json:"array"`
}

type tableGoldenVariant struct {
	Results []any            `json:"results"`
	Final   tableGoldenFinal `json:"final"`
}

type tableGoldenFinal struct {
	Size       int                            `json:"size"`
	Cells      []Cell[string, string, string] `json:"cells"`
	RowKeys    []string                       `json:"rowKeys"`
	ColumnKeys []string                       `json:"columnKeys"`
}

type tableTrace struct {
	Operations []struct {
		Op     string `json:"op"`
		Row    string `json:"row"`
		Column string `json:"column"`
		Value  string `json:"value"`
	} `json:"operations"`
}

type goldenAdapter struct {
	put            func(string, string, string) (string, bool, error)
	get            func(string, string) (string, bool)
	remove         func(string, string) (string, bool, error)
	containsRow    func(string) bool
	containsColumn func(string) bool
	len            func() int
	cells          func() iter.Seq[Cell[string, string, string]]
	row            func(string) []Cell[string, string, string]
	column         func(string) []Cell[string, string, string]
	rowKeys        func() iter.Seq[string]
	columnKeys     func() iter.Seq[string]
}

func TestGuavaTableGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/table.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace tableTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	hash := NewHashTable[string, string, string]()
	tree := NewTreeTable[string, string, string]()
	array := NewArrayTable[string, string, string]([]string{"a", "b", "c"}, []string{"x", "y", "z"})
	actual := tableGolden{
		Version: 1, GuavaCommit: guava_compare.GuavaCommit,
		Variants: tableGoldenVariants{
			Hash: runGolden(trace, goldenAdapter{
				put: func(row, column, value string) (string, bool, error) {
					old, found := hash.Put(row, column, value)
					return old, found, nil
				},
				get: hash.Get,
				remove: func(row, column string) (string, bool, error) {
					old, found := hash.Delete(row, column)
					return old, found, nil
				},
				containsRow: hash.ContainsRow, containsColumn: hash.ContainsColumn, len: hash.Len,
				cells: hash.Cells, row: hash.Row, column: hash.Column, rowKeys: hash.RowKeys, columnKeys: hash.ColumnKeys,
			}),
			Tree: runGolden(trace, goldenAdapter{
				put: func(row, column, value string) (string, bool, error) {
					old, found := tree.Put(row, column, value)
					return old, found, nil
				},
				get: tree.Get,
				remove: func(row, column string) (string, bool, error) {
					old, found := tree.Delete(row, column)
					return old, found, nil
				},
				containsRow: tree.ContainsRow, containsColumn: tree.ContainsColumn, len: tree.Len,
				cells: tree.Cells, row: tree.Row, column: tree.Column, rowKeys: tree.RowKeys, columnKeys: tree.ColumnKeys,
			}),
			Array: runGolden(trace, goldenAdapter{
				put: array.Put, get: array.Get, remove: array.Delete,
				containsRow: array.ContainsRow, containsColumn: array.ContainsColumn, len: array.Len,
				cells: array.Cells, row: array.Row, column: array.Column, rowKeys: array.RowKeys, columnKeys: array.ColumnKeys,
			}),
		},
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/table.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func runGolden(trace tableTrace, adapter goldenAdapter) tableGoldenVariant {
	results := make([]any, 0, len(trace.Operations))
	for _, operation := range trace.Operations {
		switch operation.Op {
		case "put":
			old, found, err := adapter.put(operation.Row, operation.Column, operation.Value)
			if err != nil {
				results = append(results, "error")
			} else {
				results = append(results, nullableTableString(old, found))
			}
		case "get":
			value, found := adapter.get(operation.Row, operation.Column)
			results = append(results, nullableTableString(value, found))
		case "remove":
			old, found, err := adapter.remove(operation.Row, operation.Column)
			if err != nil {
				results = append(results, "error")
			} else {
				results = append(results, nullableTableString(old, found))
			}
		case "containsRow":
			results = append(results, adapter.containsRow(operation.Row))
		case "containsColumn":
			results = append(results, adapter.containsColumn(operation.Column))
		case "size":
			results = append(results, adapter.len())
		case "row":
			results = append(results, canonicalCells(adapter.row(operation.Row)))
		case "column":
			results = append(results, canonicalCells(adapter.column(operation.Column)))
		}
	}
	return tableGoldenVariant{
		Results: results,
		Final: tableGoldenFinal{
			Size: adapter.len(), Cells: canonicalCells(collectCells(adapter.cells())),
			RowKeys: sortedStrings(adapter.rowKeys()), ColumnKeys: sortedStrings(adapter.columnKeys()),
		},
	}
}

func nullableTableString(value string, found bool) any {
	if !found {
		return nil
	}
	return value
}

func canonicalCells(cells []Cell[string, string, string]) []Cell[string, string, string] {
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Row != cells[j].Row {
			return cells[i].Row < cells[j].Row
		}
		return cells[i].Column < cells[j].Column
	})
	return cells
}

func sortedStrings(sequence iter.Seq[string]) []string {
	values := []string{}
	for value := range sequence {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

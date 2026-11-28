/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

var tableBenchmarkResult int
var tableMemoryFlatValues []string
var tableMemoryFlatPresent []bool

func benchmarkTableStream(b *testing.B, apply func(guava_compare.TableOperation) int) {
	b.Helper()
	corpus := readTableCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		tableBenchmarkResult = apply(corpus.Operations[index%len(corpus.Operations)])
	}
}

func BenchmarkHashTable(b *testing.B) {
	actual := NewHashTable[string, string, string]()
	benchmarkTableStream(b, func(operation guava_compare.TableOperation) int {
		switch operation.Op {
		case "put":
			_, found := actual.Put(operation.Row, operation.Column, operation.Value)
			return tableBoolResult(found)
		case "remove":
			_, found := actual.Delete(operation.Row, operation.Column)
			return tableBoolResult(found)
		default:
			_, found := actual.Get(operation.Row, operation.Column)
			return tableBoolResult(found)
		}
	})
}

func BenchmarkTreeTable(b *testing.B) {
	actual := NewTreeTable[string, string, string]()
	benchmarkTableStream(b, func(operation guava_compare.TableOperation) int {
		switch operation.Op {
		case "put":
			_, found := actual.Put(operation.Row, operation.Column, operation.Value)
			return tableBoolResult(found)
		case "remove":
			_, found := actual.Delete(operation.Row, operation.Column)
			return tableBoolResult(found)
		default:
			_, found := actual.Get(operation.Row, operation.Column)
			return tableBoolResult(found)
		}
	})
}

func BenchmarkNestedMap(b *testing.B) {
	actual := make(map[string]map[string]string)
	benchmarkTableStream(b, func(operation guava_compare.TableOperation) int {
		columns := actual[operation.Row]
		switch operation.Op {
		case "put":
			_, found := columns[operation.Column]
			if columns == nil {
				columns = make(map[string]string)
				actual[operation.Row] = columns
			}
			columns[operation.Column] = operation.Value
			return tableBoolResult(found)
		case "remove":
			_, found := columns[operation.Column]
			delete(columns, operation.Column)
			if len(columns) == 0 {
				delete(actual, operation.Row)
			}
			return tableBoolResult(found)
		default:
			_, found := columns[operation.Column]
			return tableBoolResult(found)
		}
	})
}

func BenchmarkArrayTable(b *testing.B) {
	corpus := readTableCorpus(b)
	actual := NewArrayTable[string, string, string](tableRowKeys(corpus.Rows), tableColumnKeys(corpus.Columns))
	benchmarkTableStream(b, func(operation guava_compare.TableOperation) int {
		switch operation.Op {
		case "put":
			_, found, _ := actual.Put(operation.Row, operation.Column, operation.Value)
			return tableBoolResult(found)
		case "remove":
			// Matches TableBenchmark.applyArray, which removes with the
			// index-based arrayTable.set(rowIndex, columnIndex, null).
			_, found := actual.DeleteAt(operation.RowIndex, operation.ColumnIndex)
			return tableBoolResult(found)
		default:
			_, found := actual.Get(operation.Row, operation.Column)
			return tableBoolResult(found)
		}
	})
}

func BenchmarkFlatMatrix(b *testing.B) {
	corpus := readTableCorpus(b)
	values := make([]string, corpus.Rows*corpus.Columns)
	present := make([]bool, len(values))
	benchmarkTableStream(b, func(operation guava_compare.TableOperation) int {
		position := operation.RowIndex*corpus.Columns + operation.ColumnIndex
		found := present[position]
		switch operation.Op {
		case "put":
			values[position] = operation.Value
			present[position] = true
		case "remove":
			values[position] = ""
			present[position] = false
		}
		return tableBoolResult(found)
	})
}

func BenchmarkHashTableRow(b *testing.B) {
	actual := fullHashTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Row("row-032"))
	}
}

func BenchmarkHashTableColumn(b *testing.B) {
	actual := fullHashTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Column("column-032"))
	}
}

func BenchmarkTreeTableRow(b *testing.B) {
	actual := NewTreeTable[string, string, string]()
	fillTable(actual.Put, 64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Row("row-032"))
	}
}

func BenchmarkTreeTableColumn(b *testing.B) {
	actual := NewTreeTable[string, string, string]()
	fillTable(actual.Put, 64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Column("column-032"))
	}
}

func BenchmarkArrayTableRow(b *testing.B) {
	actual := fullArrayTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Row("row-032"))
	}
}

func BenchmarkArrayTableColumn(b *testing.B) {
	actual := fullArrayTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = len(actual.Column("column-032"))
	}
}

func BenchmarkHashTableTranspose(b *testing.B) {
	actual := fullHashTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = actual.Transpose().Len()
	}
}

func BenchmarkTreeTableTranspose(b *testing.B) {
	actual := NewTreeTable[string, string, string]()
	fillTable(actual.Put, 64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = actual.Transpose().Len()
	}
}

func BenchmarkArrayTableTranspose(b *testing.B) {
	actual := fullArrayTable(64, 64)
	b.ResetTimer()
	for range b.N {
		tableBenchmarkResult = actual.Transpose().Len()
	}
}

func BenchmarkTableMemory(b *testing.B) {
	for _, implementation := range []string{"hash", "tree", "array", "nested", "flat"} {
		b.Run(implementation, func(b *testing.B) {
			for range b.N {
				runtime.GC()
				var base runtime.MemStats
				runtime.ReadMemStats(&base)
				var keep any
				var flatValues []string
				var flatPresent []bool
				switch implementation {
				case "hash":
					keep = fullHashTable(64, 64)
				case "tree":
					actual := NewTreeTable[string, string, string]()
					fillTable(actual.Put, 64, 64)
					keep = actual
				case "array":
					keep = fullArrayTable(64, 64)
				case "nested":
					actual := make(map[string]map[string]string)
					for row := 0; row < 64; row++ {
						columns := make(map[string]string)
						actual[fmt.Sprintf("row-%03d", row)] = columns
						for column := 0; column < 64; column++ {
							columns[fmt.Sprintf("column-%03d", column)] = "value"
						}
					}
					keep = actual
				case "flat":
					flatValues = make([]string, 64*64)
					flatPresent = make([]bool, len(flatValues))
					value := fmt.Sprintf("value-%d", 0)
					for index := range flatValues {
						flatValues[index] = value
						flatPresent[index] = true
					}
					tableMemoryFlatValues = flatValues
					tableMemoryFlatPresent = flatPresent
				}
				runtime.GC()
				var populated runtime.MemStats
				runtime.ReadMemStats(&populated)
				overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
				b.ReportMetric(float64(overhead), "retained-bytes")
				b.ReportMetric(float64(overhead)/(64*64), "retained-bytes/cell")
				runtime.KeepAlive(keep)
				runtime.KeepAlive(flatValues)
				runtime.KeepAlive(flatPresent)
				for _, value := range flatValues {
					tableBenchmarkResult += len(value)
				}
				tableBenchmarkResult += len(flatPresent)
				tableMemoryFlatValues = nil
				tableMemoryFlatPresent = nil
			}
		})
	}
}

func fullHashTable(rows, columns int) *HashTable[string, string, string] {
	actual := NewHashTable[string, string, string]()
	fillTable(actual.Put, rows, columns)
	return actual
}

func fullArrayTable(rows, columns int) *ArrayTable[string, string, string] {
	rowKeys, columnKeys := tableRowKeys(rows), tableColumnKeys(columns)
	actual := NewArrayTable[string, string, string](rowKeys, columnKeys)
	for _, row := range rowKeys {
		for _, column := range columnKeys {
			_, _, _ = actual.Put(row, column, "value")
		}
	}
	return actual
}

func fillTable(put func(string, string, string) (string, bool), rows, columns int) {
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			put(fmt.Sprintf("row-%03d", row), fmt.Sprintf("column-%03d", column), "value")
		}
	}
}

func tableRowKeys(count int) []string {
	keys := make([]string, count)
	for index := range keys {
		keys[index] = fmt.Sprintf("row-%03d", index)
	}
	return keys
}

func tableColumnKeys(count int) []string {
	keys := make([]string, count)
	for index := range keys {
		keys[index] = fmt.Sprintf("column-%03d", index)
	}
	return keys
}

func tableBoolResult(found bool) int {
	if found {
		return 1
	}
	return 0
}

func readTableCorpus(b *testing.B) guava_compare.TableCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadTableCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "table.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}

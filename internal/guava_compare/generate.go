/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package guava_compare

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
)

const (
	BenchmarkSeed                      int64 = 1729
	BenchmarkCapacity                        = 4096
	BenchmarkCorpusSize                      = 10000
	BenchmarkInputDistribution               = "uniform keys; 45% add, 25% remove, 20% count, 10% setCount"
	BenchmarkJMHSettings                     = "throughput; 2x1s warmup; 3x1s measurement; 1 fork; 1 thread"
	MultimapBenchmarkCapacity                = 256
	MultimapBenchmarkValuesPerKey            = 16
	MultimapBenchmarkCorpusSize              = 10000
	MultimapBenchmarkInputDistribution       = "256 keys x 16 values; 45% put, 20% remove, 15% get, 10% putAll, 10% containsEntry"
	BimapBenchmarkCapacity                   = 4096
	BimapBenchmarkInputDistribution          = "4096 keys/values; 20% put, 10% forcePut, 10% deleteKey, 10% deleteValue, 25% getValue, 25% getKey"
	TableBenchmarkRows                       = 64
	TableBenchmarkColumns                    = 64
	TableBenchmarkInputDistribution          = "64 rows x 64 columns; 40% put, 20% remove, 40% get"
)

type Corpus struct {
	Version           int         `json:"version"`
	Fixture           string      `json:"fixture"`
	Seed              int64       `json:"seed"`
	Capacity          int         `json:"capacity"`
	InputDistribution string      `json:"inputDistribution"`
	Operations        []Operation `json:"operations"`
}

type MultimapOperation struct {
	Op     string   `json:"op"`
	Key    string   `json:"key"`
	Value  string   `json:"value,omitempty"`
	Values []string `json:"values,omitempty"`
}

type MultimapCorpus struct {
	Version           int                 `json:"version"`
	Seed              int64               `json:"seed"`
	Capacity          int                 `json:"capacity"`
	ValuesPerKey      int                 `json:"valuesPerKey"`
	InputDistribution string              `json:"inputDistribution"`
	Operations        []MultimapOperation `json:"operations"`
}

type TableOperation struct {
	Op          string `json:"op"`
	Row         string `json:"row"`
	Column      string `json:"column"`
	Value       string `json:"value,omitempty"`
	RowIndex    int    `json:"rowIndex"`
	ColumnIndex int    `json:"columnIndex"`
}

type TableCorpus struct {
	Version           int              `json:"version"`
	Seed              int64            `json:"seed"`
	Rows              int              `json:"rows"`
	Columns           int              `json:"columns"`
	InputDistribution string           `json:"inputDistribution"`
	Operations        []TableOperation `json:"operations"`
}

func GenerateTrace(seed int64, operationCount int) Trace {
	return Trace{
		Version:    TraceVersion,
		Fixture:    FixtureName,
		Seed:       seed,
		Operations: generateOperations(seed, operationCount, 32, true),
	}
}

func GenerateCorpus(seed int64, operationCount, capacity int) Corpus {
	return Corpus{
		Version:           TraceVersion,
		Fixture:           FixtureName,
		Seed:              seed,
		Capacity:          capacity,
		InputDistribution: BenchmarkInputDistribution,
		Operations:        generateOperations(seed, operationCount, capacity, false),
	}
}

func ReadCorpus(path string) (Corpus, error) {
	file, err := os.Open(path)
	if err != nil {
		return Corpus{}, err
	}
	defer func() { _ = file.Close() }()

	var corpus Corpus
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return Corpus{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return Corpus{}, err
	}
	return corpus, nil
}

func WriteCorpus(path string, corpus Corpus) error {
	return writeJSON(path, corpus, true)
}

func GenerateMultimapCorpus(seed int64, operationCount, capacity, valuesPerKey int) MultimapCorpus {
	random := rand.New(rand.NewSource(seed))
	operations := make([]MultimapOperation, operationCount)
	for index := range operations {
		key := fmt.Sprintf("key-%04d", random.Intn(capacity))
		value := fmt.Sprintf("value-%02d", random.Intn(valuesPerKey))
		operation := MultimapOperation{Key: key, Value: value}
		choice := random.Intn(100)
		switch {
		case choice < 45:
			operation.Op = "put"
		case choice < 65:
			operation.Op = "remove"
		case choice < 80:
			operation.Op = "get"
		case choice < 90:
			operation.Op = "putAll"
			operation.Values = []string{value, fmt.Sprintf("value-%02d", random.Intn(valuesPerKey))}
		default:
			operation.Op = "containsEntry"
		}
		operations[index] = operation
	}
	return MultimapCorpus{
		Version:           TraceVersion,
		Seed:              seed,
		Capacity:          capacity,
		ValuesPerKey:      valuesPerKey,
		InputDistribution: MultimapBenchmarkInputDistribution,
		Operations:        operations,
	}
}

func ReadMultimapCorpus(path string) (MultimapCorpus, error) {
	file, err := os.Open(path)
	if err != nil {
		return MultimapCorpus{}, err
	}
	defer func() { _ = file.Close() }()
	var corpus MultimapCorpus
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return MultimapCorpus{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return MultimapCorpus{}, err
	}
	return corpus, nil
}

func WriteMultimapCorpus(path string, corpus MultimapCorpus) error {
	return writeJSON(path, corpus, true)
}

func GenerateBimapCorpus(seed int64, operationCount, capacity int) MultimapCorpus {
	random := rand.New(rand.NewSource(seed))
	operations := make([]MultimapOperation, operationCount)
	for index := range operations {
		key := fmt.Sprintf("key-%04d", random.Intn(capacity))
		value := fmt.Sprintf("value-%04d", random.Intn(capacity))
		operation := MultimapOperation{Key: key, Value: value}
		choice := random.Intn(100)
		switch {
		case choice < 20:
			operation.Op = "put"
		case choice < 30:
			operation.Op = "forcePut"
		case choice < 40:
			operation.Op = "deleteKey"
		case choice < 50:
			operation.Op = "deleteValue"
		case choice < 75:
			operation.Op = "getValue"
		default:
			operation.Op = "getKey"
		}
		operations[index] = operation
	}
	return MultimapCorpus{
		Version:           TraceVersion,
		Seed:              seed,
		Capacity:          capacity,
		ValuesPerKey:      1,
		InputDistribution: BimapBenchmarkInputDistribution,
		Operations:        operations,
	}
}

func GenerateTableCorpus(seed int64, operationCount, rows, columns int) TableCorpus {
	random := rand.New(rand.NewSource(seed))
	operations := make([]TableOperation, operationCount)
	for index := range operations {
		rowIndex, columnIndex := random.Intn(rows), random.Intn(columns)
		operation := TableOperation{
			Row: fmt.Sprintf("row-%03d", rowIndex), Column: fmt.Sprintf("column-%03d", columnIndex),
			Value: fmt.Sprintf("value-%04d", index), RowIndex: rowIndex, ColumnIndex: columnIndex,
		}
		choice := random.Intn(100)
		switch {
		case choice < 40:
			operation.Op = "put"
		case choice < 60:
			operation.Op = "remove"
		default:
			operation.Op = "get"
		}
		operations[index] = operation
	}
	return TableCorpus{
		Version: TraceVersion, Seed: seed, Rows: rows, Columns: columns,
		InputDistribution: TableBenchmarkInputDistribution, Operations: operations,
	}
}

func ReadTableCorpus(path string) (TableCorpus, error) {
	file, err := os.Open(path)
	if err != nil {
		return TableCorpus{}, err
	}
	defer func() { _ = file.Close() }()
	var corpus TableCorpus
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return TableCorpus{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return TableCorpus{}, err
	}
	return corpus, nil
}

func WriteTableCorpus(path string, corpus TableCorpus) error {
	return writeJSON(path, corpus, true)
}

func generateOperations(seed int64, operationCount, capacity int, queries bool) []Operation {
	random := rand.New(rand.NewSource(seed))
	operations := make([]Operation, operationCount)
	for index := range operations {
		operation := Operation{Value: fmt.Sprintf("key-%04d", random.Intn(capacity))}
		choice := random.Intn(100)
		switch {
		case choice < 45:
			operation.Op = "add"
			operation.Count = random.Intn(4) + 1
		case choice < 70:
			operation.Op = "remove"
			operation.Count = random.Intn(4) + 1
		case choice < 90:
			operation.Op = "count"
		default:
			operation.Op = "setCount"
			operation.Count = random.Intn(5)
		}
		if queries && index%11 == 10 {
			operation = Operation{Op: "size"}
		} else if queries && index%17 == 16 {
			operation = Operation{Op: "distinctSize"}
		}
		operations[index] = operation
	}
	return operations
}

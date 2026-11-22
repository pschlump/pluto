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
	BenchmarkSeed              int64 = 1729
	BenchmarkCapacity                = 4096
	BenchmarkCorpusSize              = 10000
	BenchmarkInputDistribution       = "uniform keys; 45% add, 25% remove, 20% count, 10% setCount"
	BenchmarkJMHSettings             = "throughput; 2x1s warmup; 3x1s measurement; 1 fork; 1 thread"
)

type Corpus struct {
	Version           int         `json:"version"`
	Fixture           string      `json:"fixture"`
	Seed              int64       `json:"seed"`
	Capacity          int         `json:"capacity"`
	InputDistribution string      `json:"inputDistribution"`
	Operations        []Operation `json:"operations"`
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

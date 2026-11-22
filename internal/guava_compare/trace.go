/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package guava_compare provides opt-in Guava differential and benchmark tools.
package guava_compare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	TraceVersion = 1
	FixtureName  = "counter"
	GuavaCommit  = "36e3995499d6c3b74b1207d362fa9e19915d2589"
)

type Operation struct {
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
	Count int    `json:"count,omitempty"`
}

type Trace struct {
	Version    int         `json:"version"`
	Fixture    string      `json:"fixture"`
	Seed       int64       `json:"seed"`
	Operations []Operation `json:"operations"`
}

type Entry struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type FinalState struct {
	Size         int     `json:"size"`
	DistinctSize int     `json:"distinctSize"`
	Entries      []Entry `json:"entries"`
}

type TraceResult struct {
	Version     int        `json:"version"`
	Fixture     string     `json:"fixture"`
	GuavaCommit string     `json:"guavaCommit"`
	Results     []int      `json:"results"`
	Final       FinalState `json:"final"`
}

func ReadTrace(path string) (Trace, error) {
	file, err := os.Open(path)
	if err != nil {
		return Trace{}, err
	}
	defer func() { _ = file.Close() }()

	var trace Trace
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&trace); err != nil {
		return Trace{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return Trace{}, err
	}
	return trace, nil
}

func WriteTrace(path string, trace Trace) error {
	return writeJSON(path, trace, true)
}

func WriteTraceResult(path string, result TraceResult) error {
	return writeJSON(path, result, false)
}

func ExecuteTrace(trace Trace) (TraceResult, error) {
	if trace.Version != TraceVersion {
		return TraceResult{}, fmt.Errorf("unsupported trace version %d", trace.Version)
	}
	if trace.Fixture != FixtureName {
		return TraceResult{}, fmt.Errorf("unsupported fixture %q", trace.Fixture)
	}

	counter := counter{counts: make(map[string]int)}
	results := make([]int, 0, len(trace.Operations))
	for index, operation := range trace.Operations {
		result, err := counter.apply(operation)
		if err != nil {
			return TraceResult{}, fmt.Errorf("operation %d: %w", index, err)
		}
		results = append(results, result)
	}

	entries := make([]Entry, 0, len(counter.counts))
	for value, count := range counter.counts {
		entries = append(entries, Entry{Value: value, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Value < entries[j].Value
	})
	return TraceResult{
		Version:     TraceVersion,
		Fixture:     FixtureName,
		GuavaCommit: GuavaCommit,
		Results:     results,
		Final: FinalState{
			Size:         counter.size,
			DistinctSize: len(counter.counts),
			Entries:      entries,
		},
	}, nil
}

type counter struct {
	counts map[string]int
	size   int
}

func (c *counter) apply(operation Operation) (int, error) {
	count := c.counts[operation.Value]
	switch operation.Op {
	case "add":
		if operation.Count <= 0 {
			return 0, errors.New("add count must be positive")
		}
		count += operation.Count
		c.counts[operation.Value] = count
		c.size += operation.Count
		return count, nil
	case "remove":
		if operation.Count <= 0 {
			return 0, errors.New("remove count must be positive")
		}
		removed := operation.Count
		if removed > count {
			removed = count
		}
		count -= removed
		c.size -= removed
		if count == 0 {
			delete(c.counts, operation.Value)
		} else {
			c.counts[operation.Value] = count
		}
		return count, nil
	case "setCount":
		if operation.Count < 0 {
			return 0, errors.New("setCount count must not be negative")
		}
		c.size += operation.Count - count
		if operation.Count == 0 {
			delete(c.counts, operation.Value)
		} else {
			c.counts[operation.Value] = operation.Count
		}
		return operation.Count, nil
	case "count":
		return count, nil
	case "size":
		return c.size, nil
	case "distinctSize":
		return len(c.counts), nil
	default:
		return 0, fmt.Errorf("unknown operation %q", operation.Op)
	}
}

func writeJSON(path string, value any, indent bool) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if indent {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

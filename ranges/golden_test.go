/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package ranges_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/range_set"
	"github.com/pschlump/pluto/ranges"
)

type rangeOperation struct {
	Op        string `json:"op"`
	Low       int    `json:"low"`
	High      int    `json:"high"`
	LowerOpen bool   `json:"lowerOpen"`
	UpperOpen bool   `json:"upperOpen"`
	Point     int    `json:"point"`
	Value     string `json:"value"`
}

type rangeCollectionTrace struct {
	Version       int              `json:"version"`
	Canonical     []rangeOperation `json:"canonical"`
	Contiguous    []rangeOperation `json:"contiguous"`
	SetOperations []rangeOperation `json:"setOperations"`
	MapOperations []rangeOperation `json:"mapOperations"`
}

type rangeCollectionGolden struct {
	Version     int      `json:"version"`
	GuavaCommit string   `json:"guavaCommit"`
	Canonical   []string `json:"canonical"`
	Contiguous  [][]int  `json:"contiguous"`
	Set         struct {
		Results []any    `json:"results"`
		Final   []string `json:"final"`
	} `json:"set"`
	Map struct {
		Results []any    `json:"results"`
		Final   []string `json:"final"`
	} `json:"map"`
}

func traceRange(operation rangeOperation) ranges.Range[int] {
	switch {
	case operation.LowerOpen && operation.UpperOpen:
		return ranges.Open(operation.Low, operation.High)
	case operation.LowerOpen:
		return ranges.OpenClosed(operation.Low, operation.High)
	case operation.UpperOpen:
		return ranges.ClosedOpen(operation.Low, operation.High)
	default:
		return ranges.Closed(operation.Low, operation.High)
	}
}

func formatRange(value ranges.Range[int]) string {
	result := ""
	if lower, bound, has := value.LowerBound(); has {
		if bound == ranges.OpenBound {
			result = "("
		} else {
			result = "["
		}
		result += fmt.Sprint(lower)
	} else {
		result = "(-inf"
	}
	result += ".."
	if upper, bound, has := value.UpperBound(); has {
		result += fmt.Sprint(upper)
		if bound == ranges.OpenBound {
			result += ")"
		} else {
			result += "]"
		}
	} else {
		result += "+inf)"
	}
	return result
}

func TestGuavaRangeCollectionGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/range_collections.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace rangeCollectionTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	actual := rangeCollectionGolden{Version: 1, GuavaCommit: guava_compare.GuavaCommit}
	for _, operation := range trace.Canonical {
		actual.Canonical = append(actual.Canonical, formatRange(traceRange(operation).Canonical(ranges.SignedDomain[int]())))
	}
	for _, operation := range trace.Contiguous {
		values := []int{}
		for value := range ranges.NewContiguousSet(traceRange(operation), ranges.SignedDomain[int]()).All() {
			values = append(values, value)
		}
		actual.Contiguous = append(actual.Contiguous, values)
	}
	set := range_set.NewRangeSet[int]()
	for _, operation := range trace.SetOperations {
		switch operation.Op {
		case "add":
			set.Add(traceRange(operation))
			actual.Set.Results = append(actual.Set.Results, set.Len())
		case "remove":
			set.Remove(traceRange(operation))
			actual.Set.Results = append(actual.Set.Results, set.Len())
		case "contains":
			actual.Set.Results = append(actual.Set.Results, set.Contains(operation.Point))
		case "size":
			actual.Set.Results = append(actual.Set.Results, set.Len())
		}
	}
	for _, value := range set.Slice() {
		actual.Set.Final = append(actual.Set.Final, formatRange(value))
	}
	mapped := range_map.NewRangeMap[int, string]()
	for _, operation := range trace.MapOperations {
		switch operation.Op {
		case "put":
			mapped.Put(traceRange(operation), operation.Value)
			actual.Map.Results = append(actual.Map.Results, mapped.Len())
		case "remove":
			mapped.Remove(traceRange(operation))
			actual.Map.Results = append(actual.Map.Results, mapped.Len())
		case "get":
			value, found := mapped.Get(operation.Point)
			if found {
				actual.Map.Results = append(actual.Map.Results, value)
			} else {
				actual.Map.Results = append(actual.Map.Results, nil)
			}
		case "size":
			actual.Map.Results = append(actual.Map.Results, mapped.Len())
		}
	}
	for _, entry := range mapped.EntriesSlice() {
		actual.Map.Final = append(actual.Map.Final, formatRange(entry.Range)+"="+entry.Value)
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/range_collections.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

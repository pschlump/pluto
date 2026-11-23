/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type minMaxTrace struct {
	Version      int   `json:"version"`
	MaximumSizes []int `json:"maximumSizes"`
	Operations   []struct {
		Op    string `json:"op"`
		Value int    `json:"value"`
	} `json:"operations"`
}

type minMaxGolden struct {
	Version     int    `json:"version"`
	GuavaCommit string `json:"guavaCommit"`
	Variants    []struct {
		MaximumSize int   `json:"maximumSize"`
		Results     []any `json:"results"`
		Final       []int `json:"final"`
	} `json:"variants"`
}

func TestGuavaMinMaxPriorityQueueGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/min_max_pq.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace minMaxTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	actual := minMaxGolden{Version: 1, GuavaCommit: guava_compare.GuavaCommit}
	for _, maximumSize := range trace.MaximumSizes {
		queue := NewMinMaxPriorityQueue[int]()
		if maximumSize >= 0 {
			queue = NewBoundedMinMaxPriorityQueue[int](maximumSize)
		}
		variant := struct {
			MaximumSize int   `json:"maximumSize"`
			Results     []any `json:"results"`
			Final       []int `json:"final"`
		}{MaximumSize: maximumSize, Results: []any{}}
		for _, operation := range trace.Operations {
			switch operation.Op {
			case "offer":
				queue.Push(operation.Value)
				variant.Results = append(variant.Results, queue.Len())
			case "poll":
				value, found := queue.PopMin()
				variant.Results = append(variant.Results, nullableMinMaxInt(value, found))
			case "pollLast":
				value, found := queue.PopMax()
				variant.Results = append(variant.Results, nullableMinMaxInt(value, found))
			case "peek":
				value, found := queue.PeekMin()
				variant.Results = append(variant.Results, nullableMinMaxInt(value, found))
			case "peekLast":
				value, found := queue.PeekMax()
				variant.Results = append(variant.Results, nullableMinMaxInt(value, found))
			case "size":
				variant.Results = append(variant.Results, queue.Len())
			default:
				t.Fatalf("unknown operation %q", operation.Op)
			}
		}
		variant.Final = queue.Slice()
		slices.Sort(variant.Final)
		actual.Variants = append(actual.Variants, variant)
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/min_max_pq.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func nullableMinMaxInt(value int, found bool) any {
	if !found {
		return nil
	}
	return value
}

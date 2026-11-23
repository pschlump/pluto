/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type queueTrace struct {
	Version    int   `json:"version"`
	Capacities []int `json:"capacities"`
	Operations []struct {
		Op     string   `json:"op"`
		Value  string   `json:"value"`
		Values []string `json:"values"`
	} `json:"operations"`
}

type queueGolden struct {
	Version     int    `json:"version"`
	GuavaCommit string `json:"guavaCommit"`
	Variants    []struct {
		Capacity int      `json:"capacity"`
		Results  []any    `json:"results"`
		Final    []string `json:"final"`
	} `json:"variants"`
}

func TestGuavaEvictingQueueGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/evicting_queue.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace queueTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	actual := queueGolden{Version: 1, GuavaCommit: guava_compare.GuavaCommit}
	for _, capacity := range trace.Capacities {
		queue := NewQueue[string](capacity)
		variant := struct {
			Capacity int      `json:"capacity"`
			Results  []any    `json:"results"`
			Final    []string `json:"final"`
		}{Capacity: capacity, Results: []any{}}
		for _, operation := range trace.Operations {
			switch operation.Op {
			case "add":
				queue.Push(operation.Value)
				variant.Results = append(variant.Results, true)
			case "addAll":
				queue.PushAll(operation.Values)
				variant.Results = append(variant.Results, len(operation.Values) > 0 && capacity > 0)
			case "poll":
				value, found := queue.Pop()
				variant.Results = append(variant.Results, nullableQueueString(value, found))
			case "peek":
				value, found := queue.Peek()
				variant.Results = append(variant.Results, nullableQueueString(value, found))
			case "remainingCapacity":
				variant.Results = append(variant.Results, queue.RemainingCapacity())
			case "size":
				variant.Results = append(variant.Results, queue.Len())
			default:
				t.Fatalf("unknown operation %q", operation.Op)
			}
		}
		variant.Final = queue.Slice()
		actual.Variants = append(actual.Variants, variant)
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/evicting_queue.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func nullableQueueString(value string, found bool) any {
	if !found {
		return nil
	}
	return value
}

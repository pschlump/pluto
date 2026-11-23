/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type cacheTrace struct {
	Version    int `json:"version"`
	Operations []struct {
		Op    string `json:"op"`
		Key   string `json:"key"`
		Value string `json:"value"`
		Nanos int64  `json:"nanos"`
	} `json:"operations"`
}

type cacheGolden struct {
	Version     int    `json:"version"`
	GuavaCommit string `json:"guavaCommit"`
	Results     []any  `json:"results"`
	FinalSize   int    `json:"finalSize"`
	Stats       struct {
		Hits      uint64 `json:"hits"`
		Misses    uint64 `json:"misses"`
		Loads     uint64 `json:"loads"`
		Successes uint64 `json:"successes"`
		Failures  uint64 `json:"failures"`
		Evictions uint64 `json:"evictions"`
	} `json:"stats"`
}

func TestGuavaCacheGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/cache.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace cacheTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	value := NewLoading(func(_ context.Context, key string) (string, error) {
		if key == "bad" {
			return "", errors.New("load")
		}
		return key + "!", nil
	}, WithExpireAfterAccess[string, string](10), WithClock[string, string](clock.Now))
	actual := cacheGolden{Version: 1, GuavaCommit: guava_compare.GuavaCommit}
	for _, operation := range trace.Operations {
		switch operation.Op {
		case "put":
			value.Put(operation.Key, operation.Value)
			actual.Results = append(actual.Results, nil)
		case "getIfPresent":
			item, found := value.GetIfPresent(operation.Key)
			if found {
				actual.Results = append(actual.Results, item)
			} else {
				actual.Results = append(actual.Results, nil)
			}
		case "get":
			item, err := value.Get(context.Background(), operation.Key)
			if err != nil {
				actual.Results = append(actual.Results, "error")
			} else {
				actual.Results = append(actual.Results, item)
			}
		case "size":
			actual.Results = append(actual.Results, value.Len())
		case "advance":
			clock.Advance(time.Duration(operation.Nanos))
			actual.Results = append(actual.Results, nil)
		case "invalidate":
			value.Invalidate(operation.Key)
			actual.Results = append(actual.Results, nil)
		case "invalidateAll":
			value.InvalidateAll()
			actual.Results = append(actual.Results, nil)
		}
	}
	actual.FinalSize = value.Len()
	stats := value.Stats()
	actual.Stats.Hits, actual.Stats.Misses, actual.Stats.Loads = stats.Hits, stats.Misses, stats.Loads
	actual.Stats.Successes, actual.Stats.Failures, actual.Stats.Evictions = stats.LoadSuccesses, stats.LoadFailures, stats.Evictions
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/cache.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

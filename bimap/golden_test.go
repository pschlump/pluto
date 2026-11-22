/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type bimapGolden struct {
	Version     int              `json:"version"`
	GuavaCommit string           `json:"guavaCommit"`
	Results     []any            `json:"results"`
	Final       bimapGoldenFinal `json:"final"`
}

type bimapGoldenFinal struct {
	Size    int                           `json:"size"`
	Entries []Association[string, string] `json:"entries"`
}

func TestGuavaBimapGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/bimap.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace struct {
		Operations []struct {
			Op    string `json:"op"`
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	set := NewHashBiMap[string, string]()
	results := make([]any, 0, len(trace.Operations))
	for _, operation := range trace.Operations {
		switch operation.Op {
		case "put":
			results = append(results, set.Put(operation.Key, operation.Value) == nil)
		case "forcePut":
			set.ForcePut(operation.Key, operation.Value)
			results = append(results, true)
		case "getValue":
			value, found := set.GetValue(operation.Key)
			results = append(results, nullableString(value, found))
		case "getKey":
			key, found := set.GetKey(operation.Value)
			results = append(results, nullableString(key, found))
		case "deleteKey":
			value, found := set.DeleteKey(operation.Key)
			results = append(results, nullableString(value, found))
		case "deleteValue":
			key, found := set.DeleteValue(operation.Value)
			results = append(results, nullableString(key, found))
		case "size":
			results = append(results, set.Len())
		}
	}
	entries := set.EntriesSlice()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	actual := bimapGolden{
		Version: 1, GuavaCommit: guava_compare.GuavaCommit, Results: results,
		Final: bimapGoldenFinal{Size: set.Len(), Entries: entries},
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile("testdata/bimap.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func nullableString(value string, found bool) any {
	if !found {
		return nil
	}
	return value
}

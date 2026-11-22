/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"encoding/json"
	"iter"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/pschlump/pluto/internal/guava_compare"
)

type stringMultimap interface {
	Put(string, string) bool
	PutAll(string, []string) int
	Get(string) ([]string, bool)
	Remove(string, string) bool
	RemoveAll(string) []string
	ReplaceValues(string, []string) []string
	ContainsEntry(string, string) bool
	Len() int
	KeyLen() int
	KeySet() iter.Seq[string]
}

type multimapTrace struct {
	Version    int `json:"version"`
	Operations []struct {
		Op     string   `json:"op"`
		Key    string   `json:"key"`
		Value  string   `json:"value"`
		Values []string `json:"values"`
	} `json:"operations"`
}

type multimapGolden struct {
	Version     int                      `json:"version"`
	GuavaCommit string                   `json:"guavaCommit"`
	Fixtures    map[string]fixtureGolden `json:"fixtures"`
}

type fixtureGolden struct {
	Results []any         `json:"results"`
	Final   multimapFinal `json:"final"`
}

type multimapFinal struct {
	Size    int                `json:"size"`
	KeySize int                `json:"keySize"`
	AsMap   []multimapMapEntry `json:"asMap"`
}

type multimapMapEntry struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func TestGuavaMultimapGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/multimap.trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace multimapTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/multimap.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	actual := multimapGolden{
		Version:     1,
		GuavaCommit: guava_compare.GuavaCommit,
		Fixtures: map[string]fixtureGolden{
			"list":      replayGolden(NewListMultimap[string, string](), trace, true),
			"set":       replayGolden(NewSetMultimap[string, string](), trace, false),
			"sortedSet": replayGolden(NewSortedSetMultimap[string, string](), trace, false),
		},
	}
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if !reflect.DeepEqual(encoded, expected) {
		t.Fatalf("golden mismatch\nactual:   %s\nexpected: %s", encoded, expected)
	}
}

func replayGolden(set stringMultimap, trace multimapTrace, listOrder bool) fixtureGolden {
	results := make([]any, 0, len(trace.Operations))
	for _, operation := range trace.Operations {
		switch operation.Op {
		case "put":
			results = append(results, set.Put(operation.Key, operation.Value))
		case "putAll":
			results = append(results, set.PutAll(operation.Key, operation.Values))
		case "get":
			values, _ := set.Get(operation.Key)
			results = append(results, canonicalValues(values, listOrder))
		case "containsEntry":
			results = append(results, set.ContainsEntry(operation.Key, operation.Value))
		case "remove":
			results = append(results, set.Remove(operation.Key, operation.Value))
		case "replaceValues":
			results = append(results, canonicalValues(set.ReplaceValues(operation.Key, operation.Values), listOrder))
		case "removeAll":
			results = append(results, canonicalValues(set.RemoveAll(operation.Key), listOrder))
		case "size":
			results = append(results, set.Len())
		case "keySize":
			results = append(results, set.KeyLen())
		default:
			panic("unknown golden operation")
		}
	}
	keys := make([]string, 0, set.KeyLen())
	for key := range set.KeySet() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]multimapMapEntry, 0, len(keys))
	for _, key := range keys {
		values, _ := set.Get(key)
		entries = append(entries, multimapMapEntry{Key: key, Values: canonicalValues(values, listOrder)})
	}
	return fixtureGolden{Results: results, Final: multimapFinal{Size: set.Len(), KeySize: set.KeyLen(), AsMap: entries}}
}

func canonicalValues(values []string, preserveOrder bool) []string {
	result := append([]string{}, values...)
	if !preserveOrder {
		sort.Strings(result)
	}
	return result
}

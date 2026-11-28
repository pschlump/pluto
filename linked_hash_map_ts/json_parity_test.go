/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map_ts

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/pschlump/pluto/linked_hash_map"
)

func TestUnmarshalJSONContracts(t *testing.T) {
	folded := NewFunc[string, int](strings.EqualFold, func(key string) uint64 { return uint64(len(key)) })
	folded.Put("old", 1)
	if err := folded.UnmarshalJSON([]byte(`[{"key":"B","value":2},{"key":"a","value":1},{"key":"b","value":3}]`)); err != nil {
		t.Fatal(err)
	}
	want := []Entry[string, int]{{Key: "B", Value: 3}, {Key: "a", Value: 1}}
	if got := folded.EntriesSlice(); !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded %v, want %v (custom equality retained, first key kept)", got, want)
	}
	if err := folded.UnmarshalJSON([]byte(`[{"key":"x"}],`)); err == nil {
		t.Fatal("malformed JSON succeeded")
	}
	if got := folded.EntriesSlice(); !reflect.DeepEqual(got, want) {
		t.Fatal("failed decode changed the map")
	}

	var zero Map[int, int]
	if err := zero.UnmarshalJSON([]byte(`[]`)); err != nil || zero.Len() != 0 {
		t.Fatal("empty decode into a zero twin must be a no-op", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("non-empty decode into a zero twin must panic")
		}
	}()
	_ = zero.UnmarshalJSON([]byte(`[{"key":1,"value":1}]`))
}

func TestTwinPlainParity(t *testing.T) {
	random := rand.New(rand.NewSource(4242))
	twin, plain := New[int, int](), linked_hash_map.New[int, int]()
	for step := range 5000 {
		key, value := random.Intn(64), random.Intn(1000)
		switch random.Intn(5) {
		case 0, 1:
			a, b := twin.Put(key, value)
			c, d := plain.Put(key, value)
			if a != c || b != d {
				t.Fatalf("step %d Put", step)
			}
		case 2:
			a, b := twin.Delete(key)
			c, d := plain.Delete(key)
			if a != c || b != d {
				t.Fatalf("step %d Delete", step)
			}
		case 3:
			a, b := twin.Get(key)
			c, d := plain.Get(key)
			if a != c || b != d {
				t.Fatalf("step %d Get", step)
			}
		default:
			data, err := plain.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if err = twin.UnmarshalJSON(data); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(twin.EntriesSlice(), plain.EntriesSlice()) {
			t.Fatalf("step %d: entries diverged", step)
		}
	}
}

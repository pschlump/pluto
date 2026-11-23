/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package intern

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGuavaStrongInterningGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_intern.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version       int      `json:"version"`
		FirstRetained bool     `json:"firstRetained"`
		Values        []string `json:"values"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	interner := New[string]()
	canonical := interner.Intern(strings.Clone("pluto"))
	values := []string{}
	for index := range 100 {
		value := interner.Intern(strings.Clone("pluto"))
		if value != canonical {
			t.Fatal("canonical string value changed")
		}
		if index < 3 {
			values = append(values, value)
		}
	}
	if golden.Version != 1 || !golden.FirstRetained || !reflect.DeepEqual(values, golden.Values) || interner.Len() != 1 {
		t.Fatalf("golden mismatch: %+v, values=%v len=%d", golden, values, interner.Len())
	}
}

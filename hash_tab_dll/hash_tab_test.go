package hash_tab

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"os"
	"testing"

	"github.com/pschlump/HashStr"
	"github.com/pschlump/pluto/comparable"
)

// TestData is an Inteface Matcing data type for the Nodes that supports the Comparable
// interface.  This means that it has a Compare fucntion.

type TestData struct {
	S string
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*TestData)(nil)
var _ Hashable = (*TestData)(nil)
var _ comparable.Equality = (*TestData)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa TestData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(TestData); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else if bb, ok := x.(*TestData); ok {
		if aa.S < bb.S {
			return -1
		} else if aa.S > bb.S {
			return 1
		}
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return 0
}

func (aa TestData) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(TestData); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else if bb, ok := x.(*TestData); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return false
}

func (aa TestData) HashKey(x interface{}) (rv int) {
	if v, ok := x.(*TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		return
	}
	if v, ok := x.(TestData); ok {
		rv = HashStr.HashStr([]byte(v.S))
		return
	}
	return
}

func TestTest(t *testing.T) {

	ht := NewHashTab[TestData](7)

	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after decleration, failed to get one.")
	}

	for i := 0; i < 40; i++ {
		ht.Insert(&TestData{S: fmt.Sprintf("%4d", i)})
	}

	if db8 {
		ht.Dump(os.Stdout)
	}

	// Check setup of hash tab
	if ht.IsEmpty() {
		t.Errorf("Expected to not be empty hash-tab, failed.")
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Search - find
	it := ht.Search(&TestData{S: "   8"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}

	// Delete
	found := ht.DeleteFound(it) // func (tt *HashTab[T]) Delete(find *T) (found bool) {
	if !found {
		t.Errorf("Expected to delete it, did not")
	}
	// Len
	if ht.Len() != 39 {
		t.Errorf("Expected length of 39, got %d", ht.Len())
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "   8"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

	// Insert
	ht.Insert(&TestData{S: "abcd"})

	// Len
	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Search - find
	it = ht.Search(&TestData{S: "abcd"})
	if it == nil {
		t.Errorf("Expected to find it, did not")
	}

	// Truncate
	ht.Truncate()

	// Len
	if ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}

	// Search - do not find
	it = ht.Search(&TestData{S: "abcd"})
	if it != nil {
		t.Errorf("Expected to NOT find it, did not")
	}

}

const db8 = false

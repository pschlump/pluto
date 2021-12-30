package dllts_test

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"sync"
	"testing"

	"github.com/pschlump/godebug"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dllts"
)

type TestDemo struct {
	S string
}

func NewTestDemo() *TestDemo {
	return &TestDemo{}
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Equality = (*TestDemo)(nil)

//
func (aa TestDemo) IsEqual(x comparable.Equality) bool {
	if bb, ok := x.(TestDemo); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else if bb, ok := x.(*TestDemo); ok {
		if aa.S == bb.S {
			return true
		}
		return false
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	return false
}

func TestDllGoroutines(t *testing.T) {

	var Dll1 dllts.Dll[TestDemo]

	if db7 {
		fmt.Printf("AT: %s\n", godebug.LF())
	}

	var wg sync.WaitGroup
	used := make(map[string]int)
	var used_lock sync.RWMutex

	for i := 0; i < 40000; i++ {
		if db7 {
			fmt.Printf("In Loop at %d AT: %s\n", i, godebug.LF())
		}
		go func(n int) {
			wg.Add(1)
			defer wg.Done()
			if db7 {
				fmt.Printf("In Push()\n")
			}
			Dll1.Push(&TestDemo{S: fmt.Sprintf("%04d", n)})
		}(i)
		go func(n int) {
			wg.Add(1)
			defer wg.Done()
			done := false
			for !done {
				if db7 {
					fmt.Printf("In POP(), length = %d\n", Dll1.Length())
				}
				x, err := Dll1.Pop()
				if err == nil {
					used_lock.Lock()
					used[x.S]++
					used_lock.Unlock()
					done = true
				}
			}
		}(i)
	}

	wg.Wait()

	if Dll1.Length() != 0 {
		t.Errorf("Length should be 0 if push/pop is thread safe. Got %d instead!", Dll1.Length())
	}
	for k, v := range used {
		if v > 1 {
			t.Errorf("For key %+v error of %d\n", k, v)
		}
	}
}

var db7 = false

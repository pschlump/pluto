package dll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import "testing"

func TestDll(t *testing.T) {
	type TestDemo struct {
		S string
	}

	var Dll1 Dll[TestDemo]

	if !Dll1.IsEmpty() {
		t.Errorf ( "Expected empty stack after decleration, failed to get one." )
	}

	Dll1.AppendTailSLL ( &TestDemo{S:"hi"} )

	if Dll1.IsEmpty() {
		t.Errorf ( "Expected non-empty stack after 1st push, failed to get one." )
	}

	_, err := Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd empty stack error after 1 pop" )
	}
	_, err = Dll1.Pop()
	if err == nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}

	Dll1.AppendTailSLL ( &TestDemo{S:"hi2"} )
	Dll1.AppendTailSLL ( &TestDemo{S:"hi3"} )

	got := Dll1.Length() 
	expect := 2
	if got != expect {
		t.Errorf ( "Expected length of %d got %d", expect, got )
	}

	ss, err := Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi2" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

	ss, err = Dll1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi3" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

}



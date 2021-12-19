package queue

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import "testing"

func TestStack(t *testing.T) {
	type TestDemo struct {
		S string
	}

	var Que1 Queue[TestDemo]

	if !Que1.IsEmpty() {
		t.Errorf ( "Expected empty stack after decleration, failed to get one." )
	}

	Que1.Push ( TestDemo{S:"hi"} )

	if Que1.IsEmpty() {
		t.Errorf ( "Expected non-empty stack after 1st push, failed to get one." )
	}

	err := Que1.Pop()
	if err != nil {
		t.Errorf ( "Unexpectd empty stack error after 1 pop" )
	}
	err = Que1.Pop()
	if err == nil {
		t.Errorf ( "Unexpectd lack of error after pop on empty stack" )
	}

	Que1.Push ( TestDemo{S:"hi2"} )
	Que1.Push ( TestDemo{S:"hi3"} )

	got := Que1.Length() 
	expect := 2
	if got != expect {
		t.Errorf ( "Expected length of %d got %d", expect, got )
	}

	ss, err := Que1.Peek()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi2" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

	_ = Que1.Pop()
	ss, err = Que1.Peek()
	if err != nil {
		t.Errorf ( "Unexpectd error on non-empty stack" )
	}
	if ss.S != "hi3" {
		t.Errorf ( "Expected %s got %s", "hi3", ss.S )
	}

}



package queue_dll_ts

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import "testing"

func TestQueue001(t *testing.T) {
	type TestDemo struct {
		S string
	}

	var Que1 Queue[TestDemo]

	if !Que1.IsEmpty() {
		t.Errorf("Expected empty queue after decleration, failed to get one.")
	}

	Que1.Push(TestDemo{S: "hi"})

	if Que1.IsEmpty() {
		t.Errorf("Expected non-empty queue after 1st push, failed to get one.")
	}

	err := Que1.Pop()
	if err != nil {
		t.Errorf("Unexpectd empty queue error after 1 pop")
	}
	err = Que1.Pop()
	if err == nil {
		t.Errorf("Unexpectd lack of error after pop on empty queue")
	}

	Que1.Push(TestDemo{S: "hi2"})
	Que1.Push(TestDemo{S: "hi3"})

	got := Que1.Length()
	expect := 2
	if got != expect {
		t.Errorf("Expected length of %d got %d", expect, got)
	}

	ss, err := Que1.Peek()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty queue")
	}
	if ss.S != "hi2" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	_ = Que1.Pop()
	ss, err = Que1.Peek()
	if err != nil {
		t.Errorf("Unexpectd error on non-empty queue")
	}
	if ss.S != "hi3" {
		t.Errorf("Expected %s got %s", "hi3", ss.S)
	}

	Que1.Truncate()
	if !Que1.IsEmpty() {
		t.Errorf("Expected non-empty queue after Truncate, failed to get one.")
	}

}

/* vim: set noai ts=4 sw=4: */

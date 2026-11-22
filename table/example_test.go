/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_test

import (
	"fmt"

	"github.com/pschlump/pluto/table"
)

func ExampleTreeTable() {
	grades := table.NewTreeTable[string, string, int]()
	grades.Put("alice", "math", 92)
	grades.Put("alice", "art", 96)
	grades.Put("bob", "math", 88)
	for cell := range grades.Cells() {
		fmt.Println(cell.Row, cell.Column, cell.Value)
	}
	transposed := grades.Transpose()
	fmt.Println(transposed.Get("math", "bob"))
	// Output:
	// alice art 96
	// alice math 92
	// bob math 88
	// 88 true
}

func ExampleArrayTable() {
	board := table.NewArrayTable[string, string, int](
		[]string{"top", "bottom"}, []string{"left", "right"})
	_, _, _ = board.Put("top", "left", 0)
	value, present := board.GetAt(0, 0)
	fmt.Println(value, present, board.ContainsRow("bottom"))
	board.DeleteAt(0, 0)
	_, present = board.GetAt(0, 0)
	fmt.Println(present, board.ContainsRow("top"))
	// Output:
	// 0 true true
	// false true
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable_test

import (
	"fmt"

	"github.com/pschlump/pluto/immutable"
)

func ExampleList() {
	builder := immutable.NewListBuilder[string]()
	builder.AddAll([]string{"a", "b"})
	first := builder.Build()
	builder.Add("c")
	fmt.Println(first.Slice(), builder.Build().Slice())
	// Output: [a b] [a b c]
}

func ExampleMap() {
	value, err := immutable.NewMap([]immutable.Entry[string, int]{{Key: "a", Value: 1}, {Key: "b", Value: 2}})
	if err != nil {
		panic(err)
	}
	fmt.Println(value.Get("b"))
	// Output: 2 true
}

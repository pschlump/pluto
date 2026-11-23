package iter_util_test

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/iter_util"
)

func ExamplePartition() {
	values := slices.Values([]int{1, 2, 3, 4, 5})
	for part := range iter_util.Partition(values, 2) {
		fmt.Println(part)
	}
	// Output:
	// [1 2]
	// [3 4]
	// [5]
}

func ExamplePeeking() {
	cursor := iter_util.NewPeeking(slices.Values([]string{"a", "b"}))
	defer cursor.Close()
	peeked, _ := cursor.Peek()
	again, _ := cursor.Peek()
	next, _ := cursor.Next()
	fmt.Println(peeked, again, next)
	// Output: a a a
}

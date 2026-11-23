package ordering_test

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/ordering"
)

func ExampleOrdering_LeastOf() {
	order := ordering.Natural[int]()
	fmt.Println(order.LeastOf(slices.Values([]int{8, 3, 5, 1, 9}), 3))
	// Output: [1 3 5]
}

func ExampleByKey() {
	type word struct {
		Text string
		Len  int
	}
	words := []word{{"aaa", 3}, {"z", 1}, {"bb", 2}}
	byLength := ordering.ByKey(func(value word) int { return value.Len }, ordering.Natural[int]())
	fmt.Println(byLength.SortedCopy(words))
	// Output: [{z 1} {bb 2} {aaa 3}]
}

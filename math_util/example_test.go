package math_util_test

import (
	"fmt"

	"github.com/pschlump/pluto/math_util"
)

func ExampleCheckedAdd() {
	value, ok := math_util.CheckedAdd(int8(120), 10)
	fmt.Println(value, ok)
	fmt.Println(math_util.SaturatingAdd(int8(120), 10))
	// Output:
	// 0 false
	// 127
}

func ExampleDivide() {
	value, err := math_util.Divide(7, 2, math_util.RoundHalfEven)
	fmt.Println(value, err)
	// Output: 4 <nil>
}

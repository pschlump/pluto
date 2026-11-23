package bloom_filter_test

import (
	"fmt"

	"github.com/pschlump/pluto/bloom_filter"
	"github.com/pschlump/pluto/hashing"
)

func Example() {
	filter := bloom_filter.New(1000, 0.01, hashing.StringFunnel)
	filter.Put("pluto")
	fmt.Println(filter.MightContain("pluto"))
	fmt.Println(filter.MightContain("guava"))
	// Output:
	// true
	// false
}

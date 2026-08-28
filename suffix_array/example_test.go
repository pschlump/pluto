/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package suffix_array_test

import (
	"fmt"

	"github.com/pschlump/pluto/suffix_array"
)

// The suffix array of "banana": the six suffixes in ascending order,
// each with its text offset and its LCP with the previous sorted
// suffix.  Iterating i from 0 to Len()-1 visits the sorted order, so
// the output is deterministic.
func Example() {
	sa := suffix_array.NewSuffixArray("banana")
	for i := 0; i < sa.Len(); i++ {
		off, _ := sa.Index(i)
		lcp, _ := sa.LCP(i)
		suf, _ := sa.Select(i)
		fmt.Printf("i=%d offset=%d lcp=%d suffix=%q\n", i, off, lcp, suf)
	}
	// Output:
	// i=0 offset=5 lcp=0 suffix="a"
	// i=1 offset=3 lcp=1 suffix="ana"
	// i=2 offset=1 lcp=3 suffix="anana"
	// i=3 offset=0 lcp=0 suffix="banana"
	// i=4 offset=4 lcp=0 suffix="na"
	// i=5 offset=2 lcp=2 suffix="nana"
}

// The algs4 §6.3 longest-repeated-substring example: "acaag" appears
// at offsets 1 and 9 of "aacaagtttacaagc".
func ExampleSuffixArray_LongestRepeatedSubstring() {
	sa := suffix_array.NewSuffixArray("aacaagtttacaagc")
	fmt.Println(sa.LongestRepeatedSubstring())
	// Output:
	// acaag
}

// Rank answers the inverse question: where does the suffix starting at
// a given text offset land in the sorted order?  "banana" itself
// (offset 0) is the 4th smallest of its suffixes, at sorted position 3.
func ExampleSuffixArray_Rank() {
	sa := suffix_array.NewSuffixArray("banana")
	r, _ := sa.Rank(0)
	fmt.Println(r)
	r, _ = sa.Rank(5) // "a" is the smallest suffix
	fmt.Println(r)
	// Output:
	// 3
	// 0
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package run_length_test

import (
	"bytes"
	"fmt"

	"github.com/pschlump/pluto/run_length"
)

func Example() {
	// A fax-like black/white strip: 40 zero bits, then 16 ones, then
	// 8 zeros — 8 bytes in, 3 count bytes out.
	data := append(append(bytes.Repeat([]byte{0x00}, 5), bytes.Repeat([]byte{0xFF}, 2)...), 0x00)

	blob := run_length.Compress(data)
	fmt.Printf("%v\n", blob)
	fmt.Println(bytes.Equal(run_length.Expand(blob), data))
	// Output:
	// [40 16 8]
	// true
}

func ExampleCompress() {
	blob := run_length.Compress([]byte{0x00, 0x0F})
	fmt.Printf("%v\n", blob)
	// Output:
	// [12 4]
}

func ExampleExpand() {
	data := run_length.Expand([]byte{3, 5})
	fmt.Printf("% x\n", data)
	// Output:
	// 1f
}

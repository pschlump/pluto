/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package lzw_test

import (
	"fmt"

	"github.com/pschlump/pluto/lzw"
)

// Compress and expand the algs4 §5.5 textbook example: the 24-byte
// input packs down to 20 bytes of 9-bit codewords and expands back to
// the original text.
func Example() {
	data := []byte("TOBEORNOTTOBEORTOBEORNOT")
	compressed := lzw.Compress(data)
	fmt.Printf("%d bytes compress to %d bytes\n", len(data), len(compressed))
	fmt.Printf("expanded: %s\n", lzw.Expand(compressed))
	// Output:
	// 24 bytes compress to 20 bytes
	// expanded: TOBEORNOTTOBEORTOBEORNOT
}

// Repetitive input is where LZW shines: a kilobyte of "ABABAB..."
// compresses to a handful of bytes.
func ExampleCompress() {
	data := []byte("ABABABABABABABABABABABABABABABABABABABABABABABABABABABABABABABAB")
	compressed := lzw.Compress(data)
	fmt.Printf("%d bytes compress to %d bytes\n", len(data), len(compressed))
	// Output:
	// 64 bytes compress to 18 bytes
}

// Expand returns nil for malformed input instead of panicking.
func ExampleExpand() {
	fmt.Printf("%q\n", lzw.Expand(lzw.Compress([]byte("hello, hello, hello"))))
	fmt.Println(lzw.Expand(nil) == nil) // not even one codeword: malformed
	// Output:
	// "hello, hello, hello"
	// true
}

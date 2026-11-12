/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/deflate"
)

func ExampleCompress() {
	data := []byte(strings.Repeat("Call me Ishmael. Some years ago—never mind how long precisely—", 20))

	compressed := deflate.Compress(data)
	fmt.Println(len(data), "bytes compress to", len(compressed))

	back, n := deflate.Expand(compressed)
	fmt.Println("round trip equal:", string(back) == string(data), "-", n, "bytes consumed")
	// Output:
	// 1320 bytes compress to 77
	// round trip equal: true - 77 bytes consumed
}

func ExampleExpand_malformed() {
	// BFINAL=1 with the reserved BTYPE 11.
	out, n := deflate.Expand([]byte{0x07})
	fmt.Println(out == nil, n == 0)
	// Output:
	// true true
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package gzip_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/gzip"
)

func ExampleCompress() {
	data := []byte(strings.Repeat("the gzip member format wraps DEFLATE with a CRC-32 and a size check\n", 50))

	gz := gzip.Compress(data)
	fmt.Println(len(data), "bytes gzip to", len(gz))

	back := gzip.Expand(gz)
	fmt.Println("round trip equal:", string(back) == string(data))
	// Output:
	// 3400 bytes gzip to 112
	// round trip equal: true
}

func ExampleExpand_multiMember() {
	a := gzip.Compress([]byte("first "))
	b := gzip.Compress([]byte("second"))
	joined := append(append([]byte{}, a...), b...)
	fmt.Println(string(gzip.Expand(joined)))
	// Output:
	// first second
}

func ExampleExpand_malformed() {
	// Trailing junk after the member is not another member.
	bad := append(gzip.Compress([]byte("x")), 0x01)
	fmt.Println(gzip.Expand(bad) == nil)
	// Output:
	// true
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package zlib_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pschlump/pluto/zlib"
)

func ExampleCompress() {
	data := []byte(strings.Repeat("zlib: a 2-byte header, DEFLATE, an Adler-32. ", 40))

	z := zlib.Compress(data)
	fmt.Printf("header: % x\n", z[:2])
	fmt.Println(len(data), "bytes zlib to", len(z))

	back := zlib.Expand(z)
	fmt.Println("round trip equal:", bytes.Equal(back, data))
	// Output:
	// header: 78 9c
	// 1800 bytes zlib to 68
	// round trip equal: true
}

func ExampleExpand_malformed() {
	z := zlib.Compress([]byte("content"))
	z[len(z)-1] ^= 0x01 // corrupt the Adler-32
	fmt.Println(zlib.Expand(z) == nil)
	// Output:
	// true
}

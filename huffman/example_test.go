/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pschlump/pluto/huffman"
)

func Example() {
	data := []byte(strings.Repeat("abracadabra", 100))
	blob := huffman.Compress(data)
	fmt.Printf("%d -> %d bytes\n", len(data), len(blob))

	back := huffman.Expand(blob)
	fmt.Println(bytes.Equal(back, data))

	// Short inputs expand: the trie and count header (~14 bytes) costs
	// more than a few symbols' codewords save.
	tiny := huffman.Compress([]byte("ABRACADABRA"))
	fmt.Printf("11 -> %d bytes\n", len(tiny))
	fmt.Println(string(huffman.Expand(tiny)))

	// Output:
	// 1100 -> 302 bytes
	// true
	// 11 -> 17 bytes
	// ABRACADABRA
}

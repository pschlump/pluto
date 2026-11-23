package hashing_test

import (
	"fmt"

	"github.com/pschlump/pluto/hashing"
)

func Example_hashFunction() {
	hash := hashing.Murmur3_128().HashString("pluto")
	fmt.Println(hash)
	// Output: 8114ff5dfeca2df89346f3d888ee40a8
}

func ExampleHashObject() {
	type record struct {
		Name string
		ID   int64
	}
	funnel := func(value record, sink hashing.PrimitiveSink) {
		sink.PutUint32(uint32(len(value.Name))).PutString(value.Name).PutInt64(value.ID)
	}
	hash := hashing.HashObject(record{"example", 42}, funnel, hashing.Murmur3_128())
	fmt.Println(hash.Len())
	// Output: 16
}

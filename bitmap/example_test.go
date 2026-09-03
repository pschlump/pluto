/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import "fmt"

// SETBIT grows a nil buffer zero-filled and returns the previous bit;
// GETBIT reads MSB-first (bit 7 is the last bit of byte 0).
func ExampleSetBit() {
	buf, old, err := SetBit(nil, 7, 1)
	fmt.Println(old, err, GetBit(buf, 7), GetBit(buf, 0))
	// Output: 0 <nil> 1 0
}

// BITCOUNT over byte and bit ranges, with negative indexes from the end
// in the range's unit.
func ExampleBitCount() {
	buf := []byte("foobar")
	fmt.Println(BitCount(buf, 0, -1, ByteUnit))
	fmt.Println(BitCount(buf, 1, -2, ByteUnit)) // "ooba"
	fmt.Println(BitCount(buf, 10, 14, BitUnit)) // 5 bits of the first 'o'
	// Output:
	// 26
	// 18
	// 4
}

// BITPOS: without an explicit end a search for 0 may answer the first
// bit past the buffer (Redis treats the right side as zero-padded).
func ExampleBitPos() {
	buf := []byte{0xFF, 0xF0, 0x00}
	pos, found := BitPos(buf, 0, 0, 0, ByteUnit, false) // first 0
	fmt.Println(pos, found)
	pos, found = BitPos(buf, 0, 0, 1, ByteUnit, true) // first 0 in bytes 0..1
	fmt.Println(pos, found)
	pos, found = BitPos(buf, 1, 8, 0, BitUnit, false) // first 1 from bit 8 on
	fmt.Println(pos, found)
	// Output:
	// 12 true
	// 12 true
	// 8 true
}

// BITOP combines sources of different lengths; shorter sources read as
// zero, so AND clears the tail.
func ExampleBitOpAnd() {
	a := []byte{0x0F, 0xFF, 0xFF}
	b := []byte{0xF0}
	fmt.Printf("% x\n", BitOpOr(a, b))
	fmt.Printf("% x\n", BitOpAnd(a, b))
	fmt.Printf("% x\n", BitOpXor(a, b))
	fmt.Printf("% x\n", BitOpNot(a))
	// Output:
	// ff ff ff
	// 00 00 00
	// ff ff ff
	// f0 00 00
}

// BITFIELD: ops run left-to-right; SET reports the previous value,
// INCRBY the new one, and OverflowFail marks the op failed instead of
// writing.
func ExampleExecuteFieldOps() {
	_, results, failed, err := ExecuteFieldOps(nil, []FieldOp{
		{Kind: FieldSet, Bits: 8, Offset: 0, Value: 255},
		{Kind: FieldIncrBy, Bits: 8, Offset: 0, Value: 100}, // wraps to 99
		{Kind: FieldIncrBy, Signed: true, Bits: 8, Offset: 0, Value: 100, Overflow: OverflowSat},
		{Kind: FieldIncrBy, Bits: 8, Offset: 0, Value: 200, Overflow: OverflowFail}, // 127+200 > 255
		{Kind: FieldGet, Bits: 8, Offset: 0},
	})
	fmt.Println(results, failed, err)
	// Output: [0 99 127 0 127] [false false false true false] <nil>
}

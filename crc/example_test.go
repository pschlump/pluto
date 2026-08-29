/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package crc_test

import (
	"fmt"

	"github.com/pschlump/pluto/crc"
)

func ExampleChecksum32() {
	// The published check value of CRC-32/IEEE for "123456789".
	fmt.Printf("0x%08X\n", crc.Checksum32([]byte("123456789"), crc.IEEETable))
	// Output:
	// 0xCBF43926
}

func ExampleChecksum64() {
	// The published check value of CRC-64/XZ (the reflected ECMA
	// polynomial with all-ones init/xorout — what hash/crc64 computes)
	// for "123456789".
	fmt.Printf("0x%016X\n", crc.Checksum64([]byte("123456789"), crc.ECMATable))
	// Output:
	// 0x995DC9BBDF1939FA
}

// A checksum can be built incrementally: Update32 folds each chunk into
// the running CRC, and the result equals the one-shot checksum.
func ExampleUpdate32() {
	header := []byte("JRNL")
	payload := []byte("payload bytes")
	running := crc.Update32(0, crc.CastagnoliTable, header)
	running = crc.Update32(running, crc.CastagnoliTable, payload)
	oneShot := crc.Checksum32(append(header, payload...), crc.CastagnoliTable)
	fmt.Println(running == oneShot)
	// Output:
	// true
}

// MakeTable32 builds a table for any 32-bit polynomial; here the
// Koopman polynomial reproduces its published check value.
func ExampleMakeTable32() {
	tab := crc.MakeTable32(crc.Koopman)
	fmt.Printf("0x%08X\n", crc.Checksum32([]byte("123456789"), tab))
	// Output:
	// 0x2D3DD0AE
}

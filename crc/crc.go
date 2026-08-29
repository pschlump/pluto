/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package crc implements table-driven 32-bit and 64-bit CRC checksums
// using the reflected (right-shifting) algorithm — the same conventions
// as the standard library's hash/crc32 and hash/crc64, so this package
// returns identical results for every polynomial the two share.
//
// A Table32 or Table64 is precomputed from a polynomial with
// MakeTable32 / MakeTable64; Checksum32 / Checksum64 compute the CRC of
// a byte slice in one shot, and Update32 / Update64 fold more data into
// an in-progress CRC so a checksum can be built incrementally:
//
//	crc := crc.Update32(0, crc.CastagnoliTable, header)
//	crc = crc.Update32(crc, crc.CastagnoliTable, payload)
//
// The exported polynomial constants (IEEE, Castagnoli, Koopman, ECMA,
// ISO) carry the same numeric values as the standard library, and the
// prebuilt package tables (IEEETable, CastagnoliTable, KoopmanTable,
// ECMATable, ISOTable) are built once at package init, ready to use.
//
// The package panics in exactly one situation, a programmer error with
// no sane answer:
//
//	Update / Checksum with a nil table — the message names the method
//	and the fix (build a table with MakeTable32 / MakeTable64 or use
//	one of the prebuilt package tables).
//
// Every function is pure — it operates only on its arguments and the
// read-only table — so the package is safe for concurrent use without
// any locking, including sharing one table across goroutines.
package crc

// The 32-bit polynomials, in the reflected representation the algorithm
// uses; the values match hash/crc32.
const (
	IEEE       uint32 = 0xedb88320 // CRC-32 (Ethernet, zip, PNG, …)
	Castagnoli uint32 = 0x82f63b78 // CRC-32C (iSCSI, ext4, …)
	Koopman    uint32 = 0xeb31d82e // CRC-32K
)

// The 64-bit polynomials, in the reflected representation the algorithm
// uses; the values match hash/crc64.
const (
	ECMA uint64 = 0xC96C5795D7870F42 // CRC-64/XZ
	ISO  uint64 = 0xD800000000000000 // CRC-64/ISO (HDLC)
)

// Prebuilt tables for the exported polynomials, made once at package
// init.  Tables are read-only after construction and safe to share
// across goroutines.
var (
	IEEETable       = MakeTable32(IEEE)
	CastagnoliTable = MakeTable32(Castagnoli)
	KoopmanTable    = MakeTable32(Koopman)
	ECMATable       = MakeTable64(ECMA)
	ISOTable        = MakeTable64(ISO)
)

// Table32 is a 256-entry lookup table for a 32-bit CRC.
type Table32 [256]uint32

// Table64 is a 256-entry lookup table for a 64-bit CRC.
type Table64 [256]uint64

// MakeTable32 returns a newly allocated table for the given 32-bit
// polynomial (in reflected representation, e.g. IEEE).
// Complexity is O(1) — a fixed 256 × 8 loop.
func MakeTable32(poly uint32) *Table32 {
	t := new(Table32)
	for i := 0; i < 256; i++ {
		c := uint32(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = (c >> 1) ^ poly
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}

// MakeTable64 returns a newly allocated table for the given 64-bit
// polynomial (in reflected representation, e.g. ECMA).
// Complexity is O(1) — a fixed 256 × 8 loop.
func MakeTable64(poly uint64) *Table64 {
	t := new(Table64)
	for i := 0; i < 256; i++ {
		c := uint64(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = (c >> 1) ^ poly
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}

// Update32 returns the result of adding the bytes of data to the
// in-progress CRC value crc.  Start an incremental checksum with
// Update32(0, tab, firstChunk); Checksum32(data, tab) is exactly
// Update32(0, tab, data).
//
// Update32 panics on a nil table: build one with MakeTable32 or use a
// prebuilt package table such as IEEETable.
// Complexity is O(len(data)).
func Update32(crc uint32, tab *Table32, data []byte) uint32 {
	if tab == nil {
		panic("crc: Update32 called with a nil table: build one with MakeTable32(poly) or use a prebuilt package table (IEEETable, CastagnoliTable, KoopmanTable)")
	}
	return update32(crc, tab, data)
}

// Checksum32 returns the CRC-32 of data using the given table; it is
// exactly Update32(0, tab, data).
//
// Checksum32 panics on a nil table: build one with MakeTable32 or use
// a prebuilt package table such as IEEETable.
// Complexity is O(len(data)).
func Checksum32(data []byte, tab *Table32) uint32 {
	if tab == nil {
		panic("crc: Checksum32 called with a nil table: build one with MakeTable32(poly) or use a prebuilt package table (IEEETable, CastagnoliTable, KoopmanTable)")
	}
	return update32(0, tab, data)
}

// Update64 returns the result of adding the bytes of data to the
// in-progress CRC value crc.  Start an incremental checksum with
// Update64(0, tab, firstChunk); Checksum64(data, tab) is exactly
// Update64(0, tab, data).
//
// Update64 panics on a nil table: build one with MakeTable64 or use a
// prebuilt package table such as ECMATable.
// Complexity is O(len(data)).
func Update64(crc uint64, tab *Table64, data []byte) uint64 {
	if tab == nil {
		panic("crc: Update64 called with a nil table: build one with MakeTable64(poly) or use a prebuilt package table (ECMATable, ISOTable)")
	}
	return update64(crc, tab, data)
}

// Checksum64 returns the CRC-64 of data using the given table; it is
// exactly Update64(0, tab, data).
//
// Checksum64 panics on a nil table: build one with MakeTable64 or use
// a prebuilt package table such as ECMATable.
// Complexity is O(len(data)).
func Checksum64(data []byte, tab *Table64) uint64 {
	if tab == nil {
		panic("crc: Checksum64 called with a nil table: build one with MakeTable64(poly) or use a prebuilt package table (ECMATable, ISOTable)")
	}
	return update64(0, tab, data)
}

// update32 is the table loop shared by Update32 and Checksum32; the
// nil-table check has already happened in the caller.
func update32(crc uint32, tab *Table32, data []byte) uint32 {
	crc = ^crc
	for _, b := range data {
		crc = tab[byte(crc)^b] ^ (crc >> 8)
	}
	return ^crc
}

// update64 is the table loop shared by Update64 and Checksum64; the
// nil-table check has already happened in the caller.
func update64(crc uint64, tab *Table64, data []byte) uint64 {
	crc = ^crc
	for _, b := range data {
		crc = tab[byte(crc)^b] ^ (crc >> 8)
	}
	return ^crc
}

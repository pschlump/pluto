/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package deflate

// The RFC 1951 §3.2.5 length and distance code tables, and the §3.2.7
// code-length permutation.

// Length codes 257..285: base match length and number of extra bits.
var lengthBase = [29]uint16{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258}
var lengthExtra = [29]uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}

// Distance codes 0..29: base distance and number of extra bits.
var distBase = [30]uint16{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577}
var distExtra = [30]uint8{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13}

// clOrder is the permuted order in which the 19 code-length code
// lengths are transmitted (§3.2.7); it puts the frequently used
// symbols first so the header can be truncated early.
var clOrder = [19]uint8{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

// lengthCodeOf maps a match length (3..258) to its length-code index
// 0..28 (symbol 257+index).  A linear search down the 29 bases — the
// encoder-side cost is trivial next to matching.
func lengthCodeOf(length int) int {
	i := 28
	for lengthBase[i] > uint16(length) {
		i--
	}
	return i
}

// distCodeOf maps a distance (1..32768) to distance code 0..29.
func distCodeOf(dist int) int {
	i := 29
	for distBase[i] > uint16(dist) {
		i--
	}
	return i
}

// fixedLitLengths and fixedDistLengths are the §3.2.6 fixed codes:
// literals 0-143 and 280-287 are 8 bits, 144-255 are 9 bits, 256-279
// are 7 bits; all 30 distances are 5 bits (codes 30 and 31 exist in
// the fixed tree but never appear in a stream).
var (
	fixedLitLengths  = fixedLitLengthsInit()
	fixedDistLengths = func() [30]uint8 {
		var a [30]uint8
		for i := range a {
			a[i] = 5
		}
		return a
	}()
	fixedLitCodes  = canonicalCodes(fixedLitLengths[:])
	fixedDistCodes = canonicalCodes(fixedDistLengths[:])
)

func fixedLitLengthsInit() [288]uint8 {
	var a [288]uint8
	for s := range a {
		switch {
		case s <= 143:
			a[s] = 8
		case s <= 255:
			a[s] = 9
		case s <= 279:
			a[s] = 7
		default:
			a[s] = 8
		}
	}
	return a
}

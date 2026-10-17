/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package huffman

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

// benchRegimes are the four entropy regimes of the README's measured
// table, 1 MiB each.
func benchRegimes() map[string][]byte {
	rng := rand.New(rand.NewPCG(42, 7))
	const n = 1 << 20

	text := make([]byte, n) // English-like: heavy letters + space
	for i := range text {
		switch rng.IntN(100) {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18:
			text[i] = ' '
		case 19, 20, 21, 22, 23, 24, 25, 26, 27:
			text[i] = 'e'
		case 28, 29, 30, 31:
			text[i] = 't'
		case 32, 33, 34, 35:
			text[i] = 'a'
		case 36, 37, 38:
			text[i] = 'o'
		case 39, 40:
			text[i] = 'i'
		case 41, 42:
			text[i] = 'n'
		case 43, 44:
			text[i] = 's'
		case 45:
			text[i] = 'h'
		case 46:
			text[i] = 'r'
		case 47:
			text[i] = 'd'
		case 48:
			text[i] = 'l'
		default:
			text[i] = byte('a' + rng.IntN(26))
		}
	}

	runs := make([]byte, 0, n) // long runs over six symbols
	for len(runs) < n {
		sym := byte(rng.IntN(6))
		run := 50 + rng.IntN(150)
		runs = append(runs, bytes.Repeat([]byte{sym}, run)...)
	}

	uniform := make([]byte, n) // all 256 symbols equally
	for i := range uniform {
		uniform[i] = byte(rng.IntN(256))
	}

	return map[string][]byte{
		"text":    text,
		"runs":    runs[:n],
		"uniform": uniform,
		"single":  bytes.Repeat([]byte{'a'}, n),
	}
}

func BenchmarkCompress(b *testing.B) {
	for name, data := range benchRegimes() {
		b.Run(name, func(b *testing.B) {
			blob := Compress(data)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				Compress(data)
			}
			// after the loop: ResetTimer deletes user-reported metrics
			b.ReportMetric(float64(len(blob))/float64(len(data)), "B/byte")
		})
	}
}

func BenchmarkExpand(b *testing.B) {
	for name, data := range benchRegimes() {
		blob := Compress(data)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				Expand(blob)
			}
		})
	}
}

package hashing

import (
	"strconv"
	"testing"
)

var benchmarkHash HashCode
var benchmarkBucket int

func benchmarkInput(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*131 + size)
	}
	return data
}

func BenchmarkMurmur3_128(b *testing.B) {
	for _, size := range []int{16, 64, 1024} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			data := benchmarkInput(size)
			function := Murmur3_128()
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				benchmarkHash = function.HashBytes(data)
			}
		})
	}
}

func BenchmarkMurmur3_32(b *testing.B) {
	data := benchmarkInput(1024)
	function := Murmur3_32()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		benchmarkHash = function.HashBytes(data)
	}
}

func BenchmarkSipHash24(b *testing.B) {
	data := benchmarkInput(64)
	function := SipHash24()
	b.ReportAllocs()
	for b.Loop() {
		benchmarkHash = function.HashBytes(data)
	}
}

func BenchmarkFarmHashFingerprint64(b *testing.B) {
	data := benchmarkInput(1024)
	function := FarmHashFingerprint64()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		benchmarkHash = function.HashBytes(data)
	}
}

func BenchmarkFingerprint2011(b *testing.B) {
	data := benchmarkInput(1024)
	function := Fingerprint2011()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		benchmarkHash = function.HashBytes(data)
	}
}

func BenchmarkMurmur3_128Streaming(b *testing.B) {
	for _, chunk := range []int{1, 7, 64} {
		b.Run(strconv.Itoa(chunk), func(b *testing.B) {
			data := benchmarkInput(1024)
			function := Murmur3_128()
			b.ReportAllocs()
			for b.Loop() {
				h := function.NewHasher()
				for offset := 0; offset < len(data); offset += chunk {
					h.PutBytes(data[offset:min(offset+chunk, len(data))])
				}
				benchmarkHash = h.Hash()
			}
		})
	}
}

func BenchmarkConsistentHash(b *testing.B) {
	code := Murmur3_128().HashString("bucket-key")
	b.ReportAllocs()
	for b.Loop() {
		benchmarkBucket = ConsistentHash(code, 1024)
	}
}

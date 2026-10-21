package fft

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// benchSignal returns a power-of-2 signal of small integers.
func benchSignal(n int) []complex128 {
	// fixed seed: deterministic setup
	rng := rand.New(rand.NewSource(42))
	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(float64(rng.Intn(21)-10), 0)
	}
	return x
}

func BenchmarkFFT1024(b *testing.B) { benchmarkFFTAt(b, 1024) }
func BenchmarkFFT4096(b *testing.B) { benchmarkFFTAt(b, 4096) }
func BenchmarkFFT16384(b *testing.B) {
	benchmarkFFTAt(b, 16384)
}

func benchmarkFFTAt(b *testing.B, n int) {
	x := benchSignal(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FFT(x)
	}
}

func BenchmarkIFFT1024(b *testing.B) {
	x := benchSignal(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IFFT(x)
	}
}

func BenchmarkConvolve1024(b *testing.B) {
	// 1024+1024 → padded to 4096.
	x, y := benchSignal(1024), benchSignal(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Convolve(x, y)
	}
}

// BenchmarkNaiveDFT256 is the O(n²) definitional transform at n=256.
// Measured (M4 Max): naive DFT at 256 costs 486µs — the same as FFT at
// 16384 points (1.43ms is 3× that, at 64× the points), which is the
// n log n vs n² gap made concrete.  The recursive FFT allocates ~2.2
// slices per point (2302 allocs at n=1024) — the faithful algs4 shape;
// an iterative in-place FFT would trade that away.
func BenchmarkNaiveDFT256(b *testing.B) {
	x := benchSignal(256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dftNaive(x)
	}
}

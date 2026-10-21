/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package fft computes the discrete Fourier transform and its inverse
// via the radix-2 Cooley-Tukey fast Fourier transform (Sedgewick &
// Wayne, Algorithms 4th ed. §9, FFT.java), plus circular and linear
// convolution built on it.
//
// These are stateless functions over []complex128 — the geo package
// precedent — so they are safe for concurrent use and need no _ts twin.
//
//	FFT(x)        — discrete Fourier transform.	O(n log n)
//	IFFT(x)       — inverse transform.		O(n log n)
//	CConvolve(x, y) — circular convolution.	O(n log n)
//	Convolve(x, y)  — linear convolution.	O(n log n)
//
// FFT and IFFT require a power-of-2 length and panic otherwise, naming
// the function (the algs4 IllegalArgumentException contract).  Empty
// input transforms to empty output.  Inputs are never modified; every
// result is a fresh slice.
package fft

import (
	"fmt"
	"math"
)

// FFT computes the discrete Fourier transform of x,
//
//	X[k] = sum_t x[t] * e^{-2πikt/n},
//
// by the radix-2 Cooley-Tukey divide-and-conquer: the transform of the
// even-indexed and odd-indexed halves, combined by twiddle factors.  A
// nil or empty x transforms to an empty result; a non-power-of-2
// length panics.
// Complexity is O(n log n) time, O(n) space (the recursion's scratch
// is one half-size buffer per level).
func FFT(x []complex128) []complex128 {
	if len(x) == 0 {
		return nil
	}
	if !powerOfTwo(len(x)) {
		panic(fmt.Sprintf("fft: FFT called with a slice of length %d — the length must be a power of 2", len(x)))
	}
	return fftRec(x)
}

// fftRec is the recursive Cooley-Tukey core over an already-validated
// power-of-2 slice (the algs4 fft, including its single-scratch-buffer
// reuse: the even half's buffer is refilled with the odd half after
// the even transform returns).
func fftRec(x []complex128) []complex128 {
	n := len(x)
	if n == 1 {
		return []complex128{x[0]}
	}

	half := make([]complex128, n/2)
	for k := 0; k < n/2; k++ {
		half[k] = x[2*k]
	}
	evenFFT := fftRec(half)
	for k := 0; k < n/2; k++ { // algs4 reuses the array for the odd half
		half[k] = x[2*k+1]
	}
	oddFFT := fftRec(half)

	y := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		theta := -2 * float64(k) * math.Pi / float64(n)
		wk := complex(math.Cos(theta), math.Sin(theta))
		t := wk * oddFFT[k]
		y[k] = evenFFT[k] + t
		y[k+n/2] = evenFFT[k] - t
	}
	return y
}

// IFFT computes the inverse discrete Fourier transform of x —
// conjugate, forward transform, conjugate, scale by 1/n (the algs4
// construction, which shares the one forward FFT).  A nil or empty x
// transforms to an empty result; a non-power-of-2 length panics.
// Complexity is O(n log n) time, O(n) space.
func IFFT(x []complex128) []complex128 {
	if len(x) == 0 {
		return nil
	}
	if !powerOfTwo(len(x)) {
		panic(fmt.Sprintf("fft: IFFT called with a slice of length %d — the length must be a power of 2", len(x)))
	}
	n := len(x)
	y := make([]complex128, n)
	for i, v := range x {
		y[i] = complex(real(v), -imag(v)) // conjugate
	}
	y = fftRec(y)
	scale := complex(1/float64(n), 0)
	for i, v := range y {
		v = complex(real(v), -imag(v)) // conjugate again
		y[i] = v * scale
	}
	return y
}

// CConvolve computes the circular convolution of x and y,
//
//	z[t] = sum_k x[k] * y[(t-k) mod n],
//
// as IFFT(FFT(x) · FFT(y)) — the convolution theorem.  Both slices
// must have the same power-of-2 length; unequal or non-power-of-2
// lengths panic.
// Complexity is O(n log n) time, O(n) space.
func CConvolve(x, y []complex128) []complex128 {
	if len(x) != len(y) {
		panic(fmt.Sprintf("fft: CConvolve called with slices of length %d and %d — both must have the same power-of-2 length", len(x), len(y)))
	}
	if len(x) == 0 {
		return nil
	}
	if !powerOfTwo(len(x)) {
		panic(fmt.Sprintf("fft: CConvolve called with slices of length %d — the length must be a power of 2", len(x)))
	}
	a := fftRec(x)
	b := fftRec(y)
	c := make([]complex128, len(a))
	for i := range a {
		c[i] = a[i] * b[i]
	}
	return IFFT(c)
}

// Convolve computes the linear convolution of x and y — the
// polynomial product, length len(x)+len(y)-1 — by zero-padding both to
// the smallest power of 2 at least len(x)+len(y)-1 and taking the
// circular convolution there (no wrap-around at that padding).  Either
// slice empty yields an empty result.
//
// This fixes algs4's convolve, which requires equal power-of-2 input
// lengths, always pads to exactly twice that, and returns the padded
// length with trailing zeros; the deviation is documented in the
// README.
// Complexity is O(n log n) time, O(n) space, where n is the padded
// length.
func Convolve(x, y []complex128) []complex128 {
	n1, n2 := len(x), len(y)
	if n1 == 0 || n2 == 0 {
		return nil
	}
	pad := 1
	for pad < n1+n2-1 {
		pad *= 2
	}
	a := make([]complex128, pad)
	b := make([]complex128, pad)
	copy(a, x)
	copy(b, y)
	z := CConvolve(a, b)
	return z[:n1+n2-1]
}

// powerOfTwo reports whether n > 0 is a power of two.
func powerOfTwo(n int) bool {
	return n&(n-1) == 0
}

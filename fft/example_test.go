package fft_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math"

	"github.com/pschlump/pluto/fft"
)

// round3 rounds a complex value to 3 decimals so example output is
// stable (the +0 turns a rounded -0 back into +0).
func round3(c complex128) complex128 {
	return complex(math.Round(real(c)*1e3)/1e3+0, math.Round(imag(c)*1e3)/1e3+0)
}

// ExampleFFT transforms a small ramp signal and recovers it with IFFT
// (the algs4 FFT test).
func ExampleFFT() {
	x := []complex128{0, 1, 2, 3, 4, 5, 6, 7}
	fmt.Println("x:   ", x)
	fmt.Println("FFT: ", roundAll(fft.FFT(x)))
	fmt.Println("back:", roundAll(fft.IFFT(fft.FFT(x))))
	// Output:
	// x:    [(0+0i) (1+0i) (2+0i) (3+0i) (4+0i) (5+0i) (6+0i) (7+0i)]
	// FFT:  [(28+0i) (-4+9.657i) (-4+4i) (-4+1.657i) (-4+0i) (-4-1.657i) (-4-4i) (-4-9.657i)]
	// back: [(0+0i) (1+0i) (2+0i) (3+0i) (4+0i) (5+0i) (6+0i) (7+0i)]
}

func roundAll(x []complex128) []complex128 {
	y := make([]complex128, len(x))
	for i, c := range x {
		y[i] = round3(c)
	}
	return y
}

// ExampleConvolve multiplies two polynomials: the coefficient vector of
// the product is the linear convolution of the coefficient vectors.
func ExampleConvolve() {
	// (1 + 2x + 3x²) * (4 + x) = 4 + 9x + 14x² + 3x³.
	product := fft.Convolve(
		[]complex128{1, 2, 3},
		[]complex128{4, 1},
	)
	for _, c := range product {
		fmt.Printf("%.0f ", real(c))
	}
	fmt.Println()
	// Output: 4 9 14 3
}

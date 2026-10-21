package fft

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/cmplx"
	"testing"
)

// tol is the comparison tolerance: integer-valued test signals of
// magnitude <= 20 and length <= 256 keep both the FFT and the O(n²)
// oracle within ~1e-12 of each other, so 1e-9 is a comfortable margin.
const tol = 1e-9

// dftNaive is the definitional O(n²) discrete Fourier transform — the
// oracle, independent of any fast algorithm.
func dftNaive(x []complex128) []complex128 {
	n := len(x)
	y := make([]complex128, n)
	for k := 0; k < n; k++ {
		var sum complex128
		for t := 0; t < n; t++ {
			theta := -2 * math.Pi * float64(k) * float64(t) / float64(n)
			sum += x[t] * complex(math.Cos(theta), math.Sin(theta))
		}
		y[k] = sum
	}
	return y
}

// assertEq fails unless every element of got matches want within tol.
func assertEq(t *testing.T, label string, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", label, len(got), len(want))
	}
	for i := range want {
		if cmplx.Abs(got[i]-want[i]) > tol {
			t.Fatalf("%s: element %d is %v, want %v (within %g)", label, i, got[i], want[i], tol)
		}
	}
}

func TestFFTKnownAnswers(t *testing.T) {
	// The 1-point transform is the identity.
	assertEq(t, "FFT of [1]", FFT([]complex128{1}), []complex128{1})

	// A constant signal is all DC: [nc, 0, ..., 0].
	for _, n := range []int{2, 8, 64} {
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(3, 0)
		}
		want := make([]complex128, n)
		want[0] = complex(3*float64(n), 0)
		assertEq(t, "FFT of constant signal", FFT(x), want)
	}

	// An impulse at 0 transforms to all ones.
	for _, n := range []int{2, 8, 64} {
		x := make([]complex128, n)
		x[0] = 1
		want := make([]complex128, n)
		for i := range want {
			want[i] = 1
		}
		assertEq(t, "FFT of impulse", FFT(x), want)
	}

	// A pure complex exponential at bin k0 transforms to n at bin k0
	// (and only there).
	n, k0 := 16, 5
	x := make([]complex128, n)
	for t := 0; t < n; t++ {
		theta := 2 * math.Pi * float64(k0) * float64(t) / float64(n)
		x[t] = complex(math.Cos(theta), math.Sin(theta))
	}
	want := make([]complex128, n)
	want[k0] = complex(float64(n), 0)
	assertEq(t, "FFT of pure tone", FFT(x), want)
}

func TestFFTRoundTrip(t *testing.T) {
	// algs4's own test: IFFT(FFT(x)) recovers x.
	for _, n := range []int{1, 2, 4, 8, 64, 256} {
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(float64(i%7)-3, float64(i%5)-2)
		}
		assertEq(t, "IFFT(FFT(x))", IFFT(FFT(x)), x)
	}
}

func TestFFTEmpty(t *testing.T) {
	if got := FFT(nil); got != nil {
		t.Errorf("FFT(nil) = %v, want nil", got)
	}
	if got := IFFT([]complex128{}); got != nil {
		t.Errorf("IFFT(empty) = %v, want nil", got)
	}
	if got := CConvolve(nil, nil); got != nil {
		t.Errorf("CConvolve(nil, nil) = %v, want nil", got)
	}
	if got := Convolve(nil, []complex128{1}); got != nil {
		t.Errorf("Convolve(nil, x) = %v, want nil", got)
	}
}

// TestFFTRandomizedDFT cross-checks FFT and IFFT against the
// definitional O(n²) transform on fixed-seed random integer signals
// (integers keep the comparison tight).
func TestFFTRandomizedDFT(t *testing.T) {
	rng := newRand()
	for step := 0; step < 200; step++ {
		n := 1 << uint(rng.Intn(7)) // 1..64, a power of 2
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(float64(rng.Intn(41)-20), float64(rng.Intn(41)-20))
		}
		assertEq(t, "FFT vs DFT", FFT(x), dftNaive(x))

		// IFFT is the inverse: transform then invert recovers x, and
		// IFFT itself matches the definitional inverse.
		assertEq(t, "IFFT round trip", IFFT(FFT(x)), x)
	}
}

func TestFFTInputNotMutated(t *testing.T) {
	x := []complex128{1, 2, 3, 4, -1, 0, 2i, 1i}
	backup := append([]complex128(nil), x...)
	FFT(x)
	IFFT(x)
	CConvolve(x, x)
	Convolve(x, x[:3])
	for i := range x {
		if x[i] != backup[i] {
			t.Fatalf("Input was mutated at %d: %v, want %v", i, x[i], backup[i])
		}
	}
}

// TestCConvolveNaive cross-checks the circular convolution against its
// definitional sum.
func TestCConvolveNaive(t *testing.T) {
	rng := newRand()
	for step := 0; step < 100; step++ {
		n := 1 << uint(1+rng.Intn(6)) // 2..64
		x := make([]complex128, n)
		y := make([]complex128, n)
		for i := range x {
			x[i] = complex(float64(rng.Intn(21)-10), 0)
			y[i] = complex(float64(rng.Intn(21)-10), 0)
		}
		want := make([]complex128, n)
		for t0 := 0; t0 < n; t0++ {
			var sum complex128
			for k := 0; k < n; k++ {
				sum += x[k] * y[(t0-k+n)%n]
			}
			want[t0] = sum
		}
		assertEq(t, "CConvolve vs naive", CConvolve(x, y), want)
	}
}

// TestConvolveNaive cross-checks the linear convolution against its
// definitional sum, on arbitrary (non-power-of-2) lengths — the fix
// over algs4's equal-length convolve.
func TestConvolveNaive(t *testing.T) {
	rng := newRand()
	for step := 0; step < 200; step++ {
		n1 := 1 + rng.Intn(12)
		n2 := 1 + rng.Intn(12)
		x := make([]complex128, n1)
		y := make([]complex128, n2)
		for i := range x {
			x[i] = complex(float64(rng.Intn(21)-10), float64(rng.Intn(11)-5))
		}
		for i := range y {
			y[i] = complex(float64(rng.Intn(21)-10), 0)
		}
		want := make([]complex128, n1+n2-1)
		for t0 := range want {
			lo := t0 - n2 + 1
			if lo < 0 {
				lo = 0
			}
			var sum complex128
			for k := lo; k <= t0 && k < n1; k++ {
				sum += x[k] * y[t0-k]
			}
			want[t0] = sum
		}
		got := Convolve(x, y)
		if len(got) != n1+n2-1 {
			t.Fatalf("Convolve length %d, want %d", len(got), n1+n2-1)
		}
		assertEq(t, "Convolve vs naive", got, want)
	}
}

// TestConvolveIdentity: convolving with the unit impulse [1] returns
// the signal unchanged.
func TestConvolveIdentity(t *testing.T) {
	x := []complex128{2, -3i, 4 + 1i, 0, 7}
	assertEq(t, "Convolve with impulse", Convolve(x, []complex128{1}), x)
	assertEq(t, "Convolve with impulse (reversed)", Convolve([]complex128{1}, x), x)
}

func TestFFTPanics(t *testing.T) {
	expectPanic(t, "FFT called with a slice of length 3", func() { FFT([]complex128{1, 2, 3}) })
	expectPanic(t, "IFFT called with a slice of length 6", func() { IFFT(make([]complex128, 6)) })
	expectPanic(t, "slices of length 2 and 4", func() {
		CConvolve([]complex128{1, 2}, []complex128{1, 2, 3, 4})
	})
	expectPanic(t, "length must be a power of 2", func() {
		CConvolve([]complex128{1, 2, 3}, []complex128{4, 5, 6})
	})
}

package stats_test

import (
	"math"
	"testing"

	"github.com/pschlump/pluto/stats"
)

func TestPairedRegression(t *testing.T) {
	var accumulator stats.PairedAccumulator
	for x := 0.0; x < 10; x++ {
		accumulator.Add(x, 3*x+2)
	}
	result := accumulator.Snapshot()
	if result.Count() != 10 {
		t.Fatalf("count = %d", result.Count())
	}
	requireClose(t, result.PearsonCorrelation(), 1, 1e-15)
	fit := result.LeastSquaresFit()
	if fit.IsHorizontal() || fit.IsVertical() {
		t.Fatal("regular fit reported degenerate")
	}
	slope, ok := fit.Slope()
	if !ok {
		t.Fatal("regular fit has no slope")
	}
	requireClose(t, slope, 3, 1e-14)
	y, ok := fit.Transform(4)
	if !ok {
		t.Fatal("regular transform failed")
	}
	requireClose(t, y, 14, 1e-14)
	inverse := fit.Inverse()
	x, ok := inverse.Transform(y)
	if !ok {
		t.Fatal("inverse transform failed")
	}
	requireClose(t, x, 4, 1e-14)
}

func TestPairedDegenerateAndNonFinite(t *testing.T) {
	var horizontal stats.PairedAccumulator
	horizontal.AddAll([]float64{1, 2, 3}, []float64{7, 7, 7})
	fit := horizontal.Snapshot().LeastSquaresFit()
	if !fit.IsHorizontal() {
		t.Fatal("constant y is not horizontal")
	}
	y, ok := fit.Transform(99)
	if !ok {
		t.Fatal("horizontal transform failed")
	}
	requireClose(t, y, 7, 0)
	if !fit.Inverse().IsVertical() {
		t.Fatal("horizontal inverse is not vertical")
	}
	if !math.IsNaN(horizontal.Snapshot().PearsonCorrelation()) {
		t.Fatal("constant correlation is not NaN")
	}

	var vertical stats.PairedAccumulator
	vertical.AddAll([]float64{4, 4, 4}, []float64{1, 2, 3})
	if !vertical.Snapshot().LeastSquaresFit().IsVertical() {
		t.Fatal("constant x is not vertical")
	}
	if _, ok := vertical.Snapshot().LeastSquaresFit().Transform(1); ok {
		t.Fatal("vertical transform succeeded")
	}
	var constant stats.PairedAccumulator
	constant.AddAll([]float64{4, 4}, []float64{7, 7})
	constantSlope, _ := constant.Snapshot().LeastSquaresFit().Slope()
	if !math.IsNaN(constantSlope) {
		t.Fatal("constant fit is not NaN")
	}
	var insufficient stats.PairedAccumulator
	insufficient.Add(1, 2)
	if !math.IsNaN(insufficient.Snapshot().PearsonCorrelation()) {
		t.Fatal("singleton correlation is not NaN")
	}
	var nonFinite stats.PairedAccumulator
	nonFinite.AddAll([]float64{1, 2}, []float64{3, math.Inf(1)})
	if !math.IsNaN(nonFinite.Snapshot().PearsonCorrelation()) {
		t.Fatal("non-finite correlation is not NaN")
	}
}

func TestPairedMerge(t *testing.T) {
	var onePass, left, right stats.PairedAccumulator
	for index := 1; index <= 100; index++ {
		x := float64(index) + 1e9
		y := -2*x + 17 + float64(index%3)
		onePass.Add(x, y)
		if index%2 == 0 {
			left.Add(x, y)
		} else {
			right.Add(x, y)
		}
	}
	left.Merge(right.Snapshot())
	requireClose(t, left.Snapshot().PearsonCorrelation(), onePass.Snapshot().PearsonCorrelation(), 2e-4)
	leftSlope, _ := left.Snapshot().LeastSquaresFit().Slope()
	oneSlope, _ := onePass.Snapshot().LeastSquaresFit().Slope()
	requireClose(t, leftSlope, oneSlope, 3e-4)
}

func TestMappingAndNaNTransformation(t *testing.T) {
	vertical := stats.Mapping(2, 3).And(2, 9)
	if !vertical.IsVertical() || !vertical.Inverse().IsHorizontal() {
		t.Fatal("vertical mapping/inverse mismatch")
	}
	nan := stats.NaNTransformation()
	if nan.IsVertical() || nan.IsHorizontal() {
		t.Fatal("NaN transformation reported degenerate")
	}
	nanSlope, _ := nan.Slope()
	if !math.IsNaN(nanSlope) {
		t.Fatal("NaN slope mismatch")
	}
	if value, ok := nan.Transform(2); !ok || !math.IsNaN(value) {
		t.Fatal("NaN transform mismatch")
	}
}

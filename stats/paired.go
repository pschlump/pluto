/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stats

import "math"

// PairedAccumulator collects online statistics for corresponding x/y values.
type PairedAccumulator struct {
	x       Accumulator
	y       Accumulator
	product float64
}

// PairedStats is an immutable paired-statistics snapshot.
type PairedStats struct {
	X       Stats
	Y       Stats
	product float64
}

// Add adds one pair.
func (accumulator *PairedAccumulator) Add(x, y float64) {
	accumulator.x.Add(x)
	if finite(x) && finite(y) {
		if accumulator.x.count > 1 {
			accumulator.product += (x - accumulator.x.mean) * (y - accumulator.y.mean)
		}
	} else {
		accumulator.product = math.NaN()
	}
	accumulator.y.Add(y)
}

// AddAll adds corresponding pairs and panics if the slice lengths differ.
func (accumulator *PairedAccumulator) AddAll(x, y []float64) {
	if len(x) != len(y) {
		panic("stats: mismatched pair lengths")
	}
	for index := range x {
		accumulator.Add(x[index], y[index])
	}
}

// Merge incorporates an immutable paired snapshot.
func (accumulator *PairedAccumulator) Merge(other PairedStats) {
	if other.X.count == 0 {
		return
	}
	if accumulator.x.count == 0 {
		*accumulator = PairedAccumulator{x: Accumulator(other.X), y: Accumulator(other.Y), product: other.product}
		return
	}
	oldXMean, oldYMean := accumulator.x.mean, accumulator.y.mean
	accumulator.x.Merge(other.X)
	accumulator.product += other.product + (other.X.mean-oldXMean)*(other.Y.mean-oldYMean)*float64(other.X.count)
	accumulator.y.Merge(other.Y)
}

// Snapshot returns an independent immutable value.
func (accumulator *PairedAccumulator) Snapshot() PairedStats {
	return PairedStats{X: accumulator.x.Snapshot(), Y: accumulator.y.Snapshot(), product: accumulator.product}
}

// Count returns the number of pairs.
func (stats PairedStats) Count() uint64 { return stats.X.count }

// PearsonCorrelation returns NaN for fewer than two pairs, non-finite input,
// or zero variance in either variable.
func (stats PairedStats) PearsonCorrelation() float64 {
	if stats.X.count < 2 || math.IsNaN(stats.product) || stats.X.m2 <= 0 || stats.Y.m2 <= 0 {
		return math.NaN()
	}
	product := stats.X.m2 * stats.Y.m2
	if product <= 0 {
		product = math.SmallestNonzeroFloat64
	}
	correlation := stats.product / math.Sqrt(product)
	if math.IsNaN(correlation) {
		return correlation
	}
	return max(-1, min(1, correlation))
}

// LeastSquaresFit returns ordinary least-squares y-on-x regression. It returns
// horizontal/vertical transformations for one constant variable and a NaN
// transformation for insufficient, non-finite, or doubly constant data.
func (stats PairedStats) LeastSquaresFit() LinearTransformation {
	if stats.X.count < 2 || math.IsNaN(stats.product) {
		return NaNTransformation()
	}
	if stats.X.m2 > 0 {
		if stats.Y.m2 > 0 {
			return Mapping(stats.X.mean, stats.Y.mean).WithSlope(stats.product / stats.X.m2)
		}
		return Horizontal(stats.Y.mean)
	}
	if stats.Y.m2 > 0 {
		return Vertical(stats.X.mean)
	}
	return NaNTransformation()
}

type transformationKind uint8

const (
	transformationRegular transformationKind = iota
	transformationVertical
	transformationNaN
)

// LinearTransformation is an immutable y=slope*x+intercept, horizontal,
// vertical, or NaN transformation.
type LinearTransformation struct {
	kind      transformationKind
	slope     float64
	intercept float64
	constant  float64
}

// LinearTransformationBuilder finishes a mapping through one point.
type LinearTransformationBuilder struct {
	x float64
	y float64
}

// Mapping starts a transformation through (x,y); both values must be finite.
func Mapping(x, y float64) LinearTransformationBuilder {
	if !finite(x) || !finite(y) {
		panic("stats: non-finite mapping point")
	}
	return LinearTransformationBuilder{x: x, y: y}
}

// And completes a transformation through a second finite point.
func (builder LinearTransformationBuilder) And(x, y float64) LinearTransformation {
	if !finite(x) || !finite(y) {
		panic("stats: non-finite mapping point")
	}
	if x == builder.x {
		if y == builder.y {
			panic("stats: identical mapping points")
		}
		return Vertical(x)
	}
	return builder.WithSlope((y - builder.y) / (x - builder.x))
}

// WithSlope completes a transformation with a non-NaN slope. Infinite slopes
// produce vertical transformations.
func (builder LinearTransformationBuilder) WithSlope(slope float64) LinearTransformation {
	if math.IsNaN(slope) {
		panic("stats: NaN slope")
	}
	if math.IsInf(slope, 0) {
		return Vertical(builder.x)
	}
	return LinearTransformation{kind: transformationRegular, slope: slope, intercept: builder.y - builder.x*slope}
}

// Horizontal returns the transformation y=constant.
func Horizontal(constant float64) LinearTransformation {
	if !finite(constant) {
		panic("stats: non-finite horizontal transformation")
	}
	return LinearTransformation{kind: transformationRegular, slope: 0, intercept: constant}
}

// Vertical returns the transformation x=constant.
func Vertical(constant float64) LinearTransformation {
	if !finite(constant) {
		panic("stats: non-finite vertical transformation")
	}
	return LinearTransformation{kind: transformationVertical, constant: constant}
}

// NaNTransformation returns a transformation whose results are NaN.
func NaNTransformation() LinearTransformation {
	return LinearTransformation{kind: transformationNaN, slope: math.NaN(), intercept: math.NaN(), constant: math.NaN()}
}

// IsVertical reports whether the transformation is vertical.
func (transformation LinearTransformation) IsVertical() bool {
	return transformation.kind == transformationVertical
}

// IsHorizontal reports whether the transformation is horizontal.
func (transformation LinearTransformation) IsHorizontal() bool {
	return transformation.kind == transformationRegular && transformation.slope == 0
}

// Slope returns the slope and false for vertical transformations. NaN
// transformations return NaN and true.
func (transformation LinearTransformation) Slope() (float64, bool) {
	if transformation.kind == transformationVertical {
		return 0, false
	}
	return transformation.slope, true
}

// Transform maps x to y and returns false for vertical transformations. NaN
// transformations return NaN and true.
func (transformation LinearTransformation) Transform(x float64) (float64, bool) {
	if transformation.kind == transformationVertical {
		return 0, false
	}
	if transformation.kind == transformationNaN {
		return math.NaN(), true
	}
	return x*transformation.slope + transformation.intercept, true
}

// Inverse swaps x and y. Horizontal and vertical transformations are mutual
// inverses; a NaN transformation is its own inverse.
func (transformation LinearTransformation) Inverse() LinearTransformation {
	switch transformation.kind {
	case transformationVertical:
		return Horizontal(transformation.constant)
	case transformationNaN:
		return transformation
	default:
		if transformation.slope == 0 {
			return Vertical(transformation.intercept)
		}
		return LinearTransformation{
			kind:      transformationRegular,
			slope:     1 / transformation.slope,
			intercept: -transformation.intercept / transformation.slope,
		}
	}
}

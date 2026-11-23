/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set_ts

// UnionOf returns an independent ordered union using operand snapshots.
func UnionOf[T any](left, right *TreeSet[T]) *TreeSet[T] {
	result := &TreeSet[T]{}
	result.Union(left, right)
	return result
}
func IntersectionOf[T any](left, right *TreeSet[T]) *TreeSet[T] {
	result := &TreeSet[T]{}
	result.Intersect(left, right)
	return result
}
func DifferenceOf[T any](left, right *TreeSet[T]) *TreeSet[T] {
	result := &TreeSet[T]{}
	result.Minus(left, right)
	return result
}
func SymmetricDifferenceOf[T any](left, right *TreeSet[T]) *TreeSet[T] {
	result := &TreeSet[T]{}
	result.SymmetricDifference(left, right)
	return result
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package hash_set_ts

// UnionOf returns an independent set union using operand snapshots.
func UnionOf[T any](left, right *HashSet[T]) *HashSet[T] {
	result := &HashSet[T]{}
	result.Union(left, right)
	return result
}
func IntersectionOf[T any](left, right *HashSet[T]) *HashSet[T] {
	result := &HashSet[T]{}
	result.Intersect(left, right)
	return result
}
func DifferenceOf[T any](left, right *HashSet[T]) *HashSet[T] {
	result := &HashSet[T]{}
	result.Minus(left, right)
	return result
}
func SymmetricDifferenceOf[T any](left, right *HashSet[T]) *HashSet[T] {
	result := &HashSet[T]{}
	result.SymmetricDifference(left, right)
	return result
}

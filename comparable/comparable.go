package comparable

type Comparable interface {
	// Compare will return -1 (or a value less than 0) if a.Compare(b) has a < b,
	// 0 if the two are considered to be equal, and
	// +1 (or a value larger than 0) if a.Compare(b) has a > b.
	// For int this can be implemented as "a - b"
	Compare(b Comparable) int // Compare(b interface{}) int
}

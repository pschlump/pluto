/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package ranges

import "iter"

// ContiguousSet is a lazy finite set of discrete values in a canonical range.
// Unbounded endpoints use the domain's finite extrema.
type ContiguousSet[T any] struct {
	canonical Range[T]
	domain    DiscreteDomain[T]
	lower     T
	upper     T
	empty     bool
}

func NewContiguousSet[T any](value Range[T], domain DiscreteDomain[T]) *ContiguousSet[T] {
	canonical := value.Canonical(domain)
	result := &ContiguousSet[T]{canonical: canonical, domain: domain, empty: canonical.IsEmpty()}
	if result.empty {
		return result
	}
	var ok bool
	if canonical.hasLower {
		result.lower = canonical.lower
	} else {
		result.lower, ok = domain.Min()
		if !ok {
			panic("ranges: unbounded ContiguousSet requires a domain minimum")
		}
	}
	if canonical.hasUpper {
		result.upper = canonical.upper
		if canonical.upperOpen {
			result.upper, ok = domain.Prev(result.upper)
			if !ok {
				result.empty = true
				return result
			}
		}
	} else {
		result.upper, ok = domain.Max()
		if !ok {
			panic("ranges: unbounded ContiguousSet requires a domain maximum")
		}
	}
	if canonical.compare(result.lower, result.upper) > 0 {
		result.empty = true
	}
	return result
}

func (s *ContiguousSet[T]) IsEmpty() bool { return s == nil || s.empty }

// Len returns the exact size, or false when it exceeds uint64.
func (s *ContiguousSet[T]) Len() (uint64, bool) {
	if s == nil || s.empty {
		return 0, true
	}
	distance, ok := s.domain.Distance(s.lower, s.upper)
	if !ok || distance == ^uint64(0) {
		return 0, false
	}
	return distance + 1, true
}

func (s *ContiguousSet[T]) Contains(value T) bool {
	return s != nil && !s.empty && s.canonical.Contains(value)
}

// Rank returns the zero-based position of a member.
func (s *ContiguousSet[T]) Rank(value T) (uint64, bool) {
	if !s.Contains(value) {
		return 0, false
	}
	return s.domain.Distance(s.lower, value)
}

// All lazily iterates in ascending discrete order without materializing values.
func (s *ContiguousSet[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if s == nil || s.empty {
			return
		}
		for value := s.lower; ; {
			if !yield(value) {
				return
			}
			if s.canonical.compare(value, s.upper) == 0 {
				return
			}
			next, ok := s.domain.Next(value)
			if !ok {
				panic("ranges: ContiguousSet exceeded domain")
			}
			value = next
		}
	}
}

// Range returns the canonical range used by the set.
func (s *ContiguousSet[T]) Range() Range[T] {
	if s == nil {
		return Range[T]{}
	}
	return s.canonical
}

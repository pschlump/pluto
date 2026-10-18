/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set

import "encoding/json"

// MarshalJSON encodes the set as a JSON array of its elements in slot
// order (which varies from process to process — sort the decoded array
// when a fixed order matters).  A nil or empty set encodes as [].  (One
// stdlib wrinkle, the same for every pluto container: json.Marshal of a
// nil POINTER renders "null" before this method runs — the [] form is
// what the method itself produces, e.g. on a nil element of a slice.)
// Element values are encoded with their own encoding/json
// representation; element types that cannot be marshaled fail with the
// error from the stdlib, as any slice of T would.
// Complexity is O(n).
func (ss *HashSet[T]) MarshalJSON() ([]byte, error) {
	if ss == nil {
		return []byte("[]"), nil
	}
	return ss.tab.MarshalJSON()
}

// UnmarshalJSON replaces the contents of the set with the elements of
// the decoded JSON array — the previous elements are discarded, the
// stored equality/hash functions are KEPT, so the set stays usable with
// whatever constructor pair built it (a decoded element equal to an
// earlier one collapses, exactly as repeated Insert calls do).
//
// A malformed array leaves the set untouched; nil and an empty array
// empty the set.  The insert contract applies exactly when there is an
// element to store: UnmarshalJSON of a non-empty array into a nil set
// panics ("called on a nil set"), and into a zero-value set panics with
// the standard no-equality/hash-functions message — the same contract
// Insert applies, spelled for JSON.
// Complexity is O(k) average, k = number of decoded elements.
func (ss *HashSet[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err // malformed input: the set is untouched
	}
	if len(items) > 0 {
		ss.insertContractCheck("UnmarshalJSON")
	}
	if ss == nil || ss.tab == nil {
		return nil // nothing to store (or a zero-value set): nothing to do
	}
	ss.tab.Truncate()
	for _, v := range items {
		ss.tab.Insert(v)
	}
	return nil
}

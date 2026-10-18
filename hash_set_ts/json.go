/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_set_ts

import "encoding/json"

// MarshalJSON encodes the set as a JSON array of its elements in slot
// order (which varies from process to process — sort the decoded array
// when a fixed order matters).  A nil or empty set encodes as [].  (As
// everywhere, json.Marshal of a nil POINTER renders "null" before this
// method runs; [] is what the method itself produces.)  The elements
// are snapshotted under the read lock.
// Complexity is O(n).
func (t *HashSet[T]) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.MarshalJSON()
}

// UnmarshalJSON replaces the contents of the set with the elements of
// the decoded JSON array under one write lock — concurrent readers
// observe the whole replacement at once.  The previous elements are
// discarded and the stored equality/hash functions are KEPT, so the set
// stays usable with whatever constructor pair built it.  A malformed
// array leaves the set untouched; nil and an empty array empty the set.
// The insert contract applies exactly when there is an element to
// store: unmarshaling a non-empty array into a nil set panics ("called
// on a nil set"), and into a zero-value set panics with the standard
// no-equality/hash-functions message — hash_set_ts spelled, like every
// panic in this package.
// Complexity is O(k) average, k = number of decoded elements.
func (t *HashSet[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err // malformed input: the set is untouched
	}
	if len(items) > 0 && t == nil {
		panic("hash_set_ts: UnmarshalJSON called on a nil set (a nil set cannot store an element)")
	}
	if t == nil {
		return nil // nothing to store
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	if len(items) > 0 {
		t.insertContract("UnmarshalJSON")
	}
	return t.inner.UnmarshalJSON(data)
}

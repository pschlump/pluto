/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set_ts

import (
	"encoding/json"

	"github.com/pschlump/pluto/tree_set"
)

// The JSON round trip with the twin's lock discipline: MarshalJSON
// snapshots under the read lock; UnmarshalJSON decodes with NO lock
// held (a decode error must leave the set untouched) and then replaces
// the contents under one hold of the write lock — atomic to concurrent
// readers.  The insert contract of the plain package applies to
// storing, with this package's constructors in the message.

// MarshalJSON encodes the set as a JSON array of its elements in
// ascending order, under one hold of the read lock — a consistent
// snapshot.  A nil set or a zero-value set encodes as an empty array
// when called directly.  (The encoding/json caveat applies as everywhere
// in pluto: json.Marshal on a nil *TreeSet writes `null` before
// MarshalJSON is ever called.)
// Complexity is O(n).
func (t *TreeSet[T]) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.inner.MarshalJSON()
}

// UnmarshalJSON replaces the set's contents with the elements decoded
// from a JSON array, ordered by the set's existing comparison function
// (duplicates collapse as inserting them in order would).  The whole
// replacement runs under one hold of the write lock; a decode error
// leaves the set untouched.  Storing into a nil set or a zero-value set
// (no comparison function) panics with the standard insert-family
// message; unmarshaling an empty array anywhere is tolerated and clears
// the set.
// Complexity is O(k·log₂ k), k = number of decoded elements.
func (t *TreeSet[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err // decode failed: the set is untouched, no lock was ever held
	}
	if len(items) == 0 {
		if t != nil {
			t.lock.Lock()
			t.inner = tree_set.TreeSet[T]{} // clears, keeps the twin's cmp
			t.lock.Unlock()
		}
		return nil
	}
	if t == nil {
		t.writeContractCheck("UnmarshalJSON") // reports the nil-set contract and panics
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.cmp == nil {
		t.writeContractCheck("UnmarshalJSON") // the zero-value contract, under the lock
	}
	t.inner = *tree_set.FromSliceFunc(t.cmp, items) // replace, keeping the comparator
	return nil
}

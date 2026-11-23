/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import "encoding/json"

// Immutable values support JSON marshal only; use constructors or builders to
// parse replacements rather than mutating a published value through UnmarshalJSON.
func (l *List[T]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(l.Slice())
}

func (s *Set[T]) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s.Slice())
}

func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(m.EntriesSlice())
}

func (s *SortedSet[T]) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(s.Slice())
}

func (m *SortedMap[K, V]) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(m.EntriesSlice())
}

func (m *Multiset[T]) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(m.EntriesSlice())
}

func (m *Multimap[K, V]) MarshalJSON() ([]byte, error) {
	entries := []Entry[K, V]{}
	for association := range m.Entries() {
		entries = append(entries, Entry[K, V]{Key: association.Key, Value: association.Value})
	}
	return json.Marshal(entries)
}

func (m *BiMap[K, V]) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(m.EntriesSlice())
}

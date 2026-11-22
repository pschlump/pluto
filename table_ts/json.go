/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table_ts

import (
	"encoding/json"

	"github.com/pschlump/pluto/table"
)

func decodeCells[R any, C any, V any](data []byte) ([]table.Cell[R, C, V], error) {
	var cells []table.Cell[R, C, V]
	if err := json.Unmarshal(data, &cells); err != nil {
		return nil, err
	}
	return cells, nil
}

func (t *HashTable[R, C, V]) MarshalJSON() ([]byte, error) {
	if t == nil || t.plain == nil {
		return []byte("[]"), nil
	}
	t.lock.RLock()
	plain := t.plain.Clone()
	t.lock.RUnlock()
	return json.Marshal(plain)
}

func (t *HashTable[R, C, V]) UnmarshalJSON(data []byte) error {
	cells, err := decodeCells[R, C, V](data)
	if err != nil {
		return err
	}
	t.writeContract("UnmarshalJSON")
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Replace(cells)
	return nil
}

func (t *TreeTable[R, C, V]) MarshalJSON() ([]byte, error) {
	if t == nil || t.plain == nil {
		return []byte("[]"), nil
	}
	t.lock.RLock()
	plain := t.plain.Clone()
	t.lock.RUnlock()
	return json.Marshal(plain)
}

func (t *TreeTable[R, C, V]) UnmarshalJSON(data []byte) error {
	cells, err := decodeCells[R, C, V](data)
	if err != nil {
		return err
	}
	t.writeContract("UnmarshalJSON")
	t.lock.Lock()
	defer t.lock.Unlock()
	t.plain.Replace(cells)
	return nil
}

func (t *ArrayTable[R, C, V]) MarshalJSON() ([]byte, error) {
	if t == nil || t.plain == nil {
		return []byte("[]"), nil
	}
	t.lock.RLock()
	plain := t.plain.Clone()
	t.lock.RUnlock()
	return json.Marshal(plain)
}

func (t *ArrayTable[R, C, V]) UnmarshalJSON(data []byte) error {
	cells, err := decodeCells[R, C, V](data)
	if err != nil {
		return err
	}
	t.writeContract("UnmarshalJSON")
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.plain.Replace(cells)
}

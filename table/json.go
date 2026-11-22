/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"encoding/json"
	"iter"
)

func collectCells[R any, C any, V any](sequence iter.Seq[Cell[R, C, V]]) []Cell[R, C, V] {
	cells := []Cell[R, C, V]{}
	for cell := range sequence {
		cells = append(cells, cell)
	}
	return cells
}

func (t *HashTable[R, C, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(collectCells(t.Cells()))
}

func (t *HashTable[R, C, V]) UnmarshalJSON(data []byte) error {
	var cells []Cell[R, C, V]
	if err := json.Unmarshal(data, &cells); err != nil {
		return err
	}
	if len(cells) > 0 {
		t.writeContract("UnmarshalJSON")
	}
	if t == nil || t.rows == nil {
		return nil
	}
	next := NewHashTableFunc[R, C, V](t.rowEqual, t.rowHash, t.colEqual, t.colHash)
	for _, cell := range cells {
		next.Put(cell.Row, cell.Column, cell.Value)
	}
	*t = *next
	return nil
}

func (t *TreeTable[R, C, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(collectCells(t.Cells()))
}

func (t *TreeTable[R, C, V]) UnmarshalJSON(data []byte) error {
	var cells []Cell[R, C, V]
	if err := json.Unmarshal(data, &cells); err != nil {
		return err
	}
	if len(cells) > 0 {
		t.writeContract("UnmarshalJSON")
	}
	if t == nil || t.rows == nil {
		return nil
	}
	next := NewTreeTableFunc[R, C, V](t.rowCompare, t.colCompare)
	for _, cell := range cells {
		next.Put(cell.Row, cell.Column, cell.Value)
	}
	*t = *next
	return nil
}

func (t *ArrayTable[R, C, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(collectCells(t.Cells()))
}

func (t *ArrayTable[R, C, V]) UnmarshalJSON(data []byte) error {
	var cells []Cell[R, C, V]
	if err := json.Unmarshal(data, &cells); err != nil {
		return err
	}
	if len(cells) > 0 {
		t.writeContract("UnmarshalJSON")
	}
	if t == nil || t.rows == nil {
		return nil
	}
	next := NewArrayTableFunc[R, C, V](t.rowKeys, t.colKeys, t.rowEqual, t.rowHash, t.colEqual, t.colHash)
	for _, cell := range cells {
		if _, _, err := next.Put(cell.Row, cell.Column, cell.Value); err != nil {
			return err
		}
	}
	*t = *next
	return nil
}

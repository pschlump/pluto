/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

// Replace atomically substitutes cells from the caller's perspective. Duplicate
// coordinates are applied in order. The input is not retained.
func (t *HashTable[R, C, V]) Replace(cells []Cell[R, C, V]) {
	if len(cells) > 0 {
		t.writeContract("Replace")
	}
	if t == nil || t.rows == nil {
		return
	}
	next := NewHashTableFunc[R, C, V](t.rowEqual, t.rowHash, t.colEqual, t.colHash)
	for _, cell := range cells {
		next.Put(cell.Row, cell.Column, cell.Value)
	}
	*t = *next
}

// Replace substitutes all cells while retaining row and column comparators.
func (t *TreeTable[R, C, V]) Replace(cells []Cell[R, C, V]) {
	if len(cells) > 0 {
		t.writeContract("Replace")
	}
	if t == nil || t.rows == nil {
		return
	}
	next := NewTreeTableFunc[R, C, V](t.rowCompare, t.colCompare)
	for _, cell := range cells {
		next.Put(cell.Row, cell.Column, cell.Value)
	}
	*t = *next
}

// Replace validates every cell against the fixed universe before substitution.
// An invalid cell leaves the receiver unchanged.
func (t *ArrayTable[R, C, V]) Replace(cells []Cell[R, C, V]) error {
	if len(cells) > 0 {
		t.writeContract("Replace")
	}
	if t == nil || t.rowIndex == nil {
		return nil
	}
	next := t.sameUniverse()
	for _, cell := range cells {
		if _, _, err := next.Put(cell.Row, cell.Column, cell.Value); err != nil {
			return err
		}
	}
	*t = *next
	return nil
}

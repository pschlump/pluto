/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"cmp"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

// Tests for the private indexes (hashIndex, treeIndex), the bulk paths built
// on them (Transpose, Clone, Columns), snapshot independence and allocation
// counts. Every randomized test uses a fixed seed.

type modelCell struct {
	row, column int
}

// modelCells returns the model's cells in row-major ascending order.
func modelCells(model map[modelCell]string) []Cell[int, int, string] {
	cells := make([]Cell[int, int, string], 0, len(model))
	for key, value := range model {
		cells = append(cells, Cell[int, int, string]{Row: key.row, Column: key.column, Value: value})
	}
	slices.SortFunc(cells, compareCellsRowMajor)
	return cells
}

func compareCellsRowMajor(a, b Cell[int, int, string]) int {
	if order := cmp.Compare(a.Row, b.Row); order != 0 {
		return order
	}
	return cmp.Compare(a.Column, b.Column)
}

func sortedCopy(cells []Cell[int, int, string], compare func(a, b Cell[int, int, string]) int) []Cell[int, int, string] {
	result := slices.Clone(cells)
	slices.SortFunc(result, compare)
	return result
}

func modelRow(model map[modelCell]string, row int) []Cell[int, int, string] {
	var cells []Cell[int, int, string]
	for _, cell := range modelCells(model) {
		if cell.Row == row {
			cells = append(cells, cell)
		}
	}
	return cells
}

func modelColumn(model map[modelCell]string, column int) []Cell[int, int, string] {
	var cells []Cell[int, int, string]
	for _, cell := range modelCells(model) {
		if cell.Column == column {
			cells = append(cells, cell)
		}
	}
	return cells
}

func transposeModel(model map[modelCell]string) map[modelCell]string {
	result := make(map[modelCell]string, len(model))
	for key, value := range model {
		result[modelCell{row: key.column, column: key.row}] = value
	}
	return result
}

func sameCells(t *testing.T, label string, got, want []Cell[int, int, string]) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d cells, want %d\ngot  %v\nwant %v", label, len(got), len(want), got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("%s: cell %d = %v, want %v", label, index, got[index], want[index])
		}
	}
}

// sparseTable is the surface shared by HashTable and TreeTable in these tests.
type sparseTable interface {
	Put(int, int, string) (string, bool)
	Get(int, int) (string, bool)
	Delete(int, int) (string, bool)
	Len() int
	ContainsRow(int) bool
	ContainsColumn(int) bool
	Row(int) []Cell[int, int, string]
	Column(int) []Cell[int, int, string]
}

// collidingHash maps every int onto 8 hash values, including 0 (the empty-slot
// marker that fixHash must remap), so probe chains are long and clusters wrap.
func collidingHash(value int) uint64 { return uint64(value & 7) }

func intEqual(a, b int) bool { return a == b }

// checkSparseAgainstModel compares every read path of a sparse table with the
// model. ordered selects exact row-major order for TreeTable; hash tables are
// compared after sorting.
func checkSparseAgainstModel(t *testing.T, label string, actual sparseTable, cells func() []Cell[int, int, string], rows func(func(int, []Cell[int, int, string]) bool), columns func(func(int, []Cell[int, int, string]) bool), model map[modelCell]string, ordered bool, keySpace int) {
	t.Helper()
	if actual.Len() != len(model) {
		t.Fatalf("%s: Len %d, want %d", label, actual.Len(), len(model))
	}
	want := modelCells(model)
	got := cells()
	if !ordered {
		got = sortedCopy(got, compareCellsRowMajor)
	}
	sameCells(t, label+" Cells", got, want)
	for key := 0; key < keySpace; key++ {
		row := actual.Row(key)
		if !ordered {
			row = sortedCopy(row, compareCellsRowMajor)
		}
		sameCells(t, fmt.Sprintf("%s Row(%d)", label, key), row, modelRow(model, key))
		column := actual.Column(key)
		if !ordered {
			column = sortedCopy(column, compareCellsRowMajor)
		}
		sameCells(t, fmt.Sprintf("%s Column(%d)", label, key), column, modelColumn(model, key))
		if actual.ContainsRow(key) != (len(modelRow(model, key)) > 0) {
			t.Fatalf("%s: ContainsRow(%d) wrong", label, key)
		}
		if actual.ContainsColumn(key) != (len(modelColumn(model, key)) > 0) {
			t.Fatalf("%s: ContainsColumn(%d) wrong", label, key)
		}
		for column := 0; column < keySpace; column++ {
			value, found := actual.Get(key, column)
			wantValue, wantFound := model[modelCell{row: key, column: column}]
			if found != wantFound || value != wantValue {
				t.Fatalf("%s: Get(%d,%d) = %q,%v want %q,%v", label, key, column, value, found, wantValue, wantFound)
			}
		}
	}
	var grouped []Cell[int, int, string]
	previous := -1
	rows(func(row int, cells []Cell[int, int, string]) bool {
		if ordered && row <= previous {
			t.Fatalf("%s: Rows out of order at %d", label, row)
		}
		previous = row
		for _, cell := range cells {
			if cell.Row != row {
				t.Fatalf("%s: Rows group %d holds %v", label, row, cell)
			}
		}
		grouped = append(grouped, cells...)
		return true
	})
	sameCells(t, label+" Rows", sortedCopy(grouped, compareCellsRowMajor), want)
	grouped, previous = nil, -1
	columns(func(column int, cells []Cell[int, int, string]) bool {
		if ordered && column <= previous {
			t.Fatalf("%s: Columns out of order at %d", label, column)
		}
		previous = column
		for index, cell := range cells {
			if cell.Column != column {
				t.Fatalf("%s: Columns group %d holds %v", label, column, cell)
			}
			if ordered && index > 0 && cells[index-1].Row >= cell.Row {
				t.Fatalf("%s: Columns group %d rows out of order", label, column)
			}
		}
		grouped = append(grouped, cells...)
		return true
	})
	sameCells(t, label+" Columns", sortedCopy(grouped, compareCellsRowMajor), want)
}

// TestSparseTablesRandomizedModel drives HashTable (natural and colliding
// custom hash) and TreeTable through a seeded put/delete stream and, at
// checkpoints, compares every read path, Transpose and Clone with the model.
func TestSparseTablesRandomizedModel(t *testing.T) {
	const keySpace = 24
	rng := rand.New(rand.NewSource(20261127))
	natural := NewHashTable[int, int, string]()
	colliding := NewHashTableFunc[int, int, string](intEqual, collidingHash, intEqual, collidingHash)
	tree := NewTreeTable[int, int, string]()
	model := map[modelCell]string{}
	for step := 0; step < 6000; step++ {
		row, column := rng.Intn(keySpace), rng.Intn(keySpace)
		key := modelCell{row: row, column: column}
		wantOld, wantFound := model[key]
		if rng.Intn(10) < 6 {
			value := fmt.Sprintf("v%d", step)
			for _, table := range []sparseTable{natural, colliding, tree} {
				if old, found := table.Put(row, column, value); old != wantOld || found != wantFound {
					t.Fatalf("step %d: Put(%d,%d) = %q,%v want %q,%v", step, row, column, old, found, wantOld, wantFound)
				}
			}
			model[key] = value
		} else {
			for _, table := range []sparseTable{natural, colliding, tree} {
				if old, found := table.Delete(row, column); old != wantOld || found != wantFound {
					t.Fatalf("step %d: Delete(%d,%d) = %q,%v want %q,%v", step, row, column, old, found, wantOld, wantFound)
				}
			}
			delete(model, key)
		}
		if step%500 != 499 {
			continue
		}
		for label, table := range map[string]*HashTable[int, int, string]{"natural": natural, "colliding": colliding} {
			checkSparseAgainstModel(t, label, table, func() []Cell[int, int, string] { return collectCells(table.Cells()) },
				func(f func(int, []Cell[int, int, string]) bool) { table.Rows()(f) },
				func(f func(int, []Cell[int, int, string]) bool) { table.Columns()(f) }, model, false, keySpace)
			transposed := table.Transpose()
			checkSparseAgainstModel(t, label+" transpose", transposed, func() []Cell[int, int, string] { return collectCells(transposed.Cells()) },
				func(f func(int, []Cell[int, int, string]) bool) { transposed.Rows()(f) },
				func(f func(int, []Cell[int, int, string]) bool) { transposed.Columns()(f) }, transposeModel(model), false, keySpace)
			clone := table.Clone()
			checkSparseAgainstModel(t, label+" clone", clone, func() []Cell[int, int, string] { return collectCells(clone.Cells()) },
				func(f func(int, []Cell[int, int, string]) bool) { clone.Rows()(f) },
				func(f func(int, []Cell[int, int, string]) bool) { clone.Columns()(f) }, model, false, keySpace)
		}
		checkSparseAgainstModel(t, "tree", tree, func() []Cell[int, int, string] { return collectCells(tree.Cells()) },
			func(f func(int, []Cell[int, int, string]) bool) { tree.Rows()(f) },
			func(f func(int, []Cell[int, int, string]) bool) { tree.Columns()(f) }, model, true, keySpace)
		transposed := tree.Transpose()
		checkSparseAgainstModel(t, "tree transpose", transposed, func() []Cell[int, int, string] { return collectCells(transposed.Cells()) },
			func(f func(int, []Cell[int, int, string]) bool) { transposed.Rows()(f) },
			func(f func(int, []Cell[int, int, string]) bool) { transposed.Columns()(f) }, transposeModel(model), true, keySpace)
		clone := tree.Clone()
		checkSparseAgainstModel(t, "tree clone", clone, func() []Cell[int, int, string] { return collectCells(clone.Cells()) },
			func(f func(int, []Cell[int, int, string]) bool) { clone.Rows()(f) },
			func(f func(int, []Cell[int, int, string]) bool) { clone.Columns()(f) }, model, true, keySpace)
		checkTreeTableShape(t, tree)
		checkTreeTableShape(t, transposed)
		checkTreeTableShape(t, clone)
	}
}

// TestSparseBulkResultsStayMutable keeps mutating Transpose and Clone results
// (bulk-built: slab trees, presized hash rows, verbatim slot copies) against
// their own models, so later writes and deletes on bulk-built structure are
// covered, not only the first read.
func TestSparseBulkResultsStayMutable(t *testing.T) {
	const keySpace = 20
	rng := rand.New(rand.NewSource(7))
	sourceModel := map[modelCell]string{}
	hash := NewHashTableFunc[int, int, string](intEqual, collidingHash, intEqual, collidingHash)
	tree := NewTreeTable[int, int, string]()
	for len(sourceModel) < 250 {
		row, column := rng.Intn(keySpace), rng.Intn(keySpace)
		value := fmt.Sprint(row*100 + column)
		hash.Put(row, column, value)
		tree.Put(row, column, value)
		sourceModel[modelCell{row: row, column: column}] = value
	}
	type subject struct {
		label   string
		table   sparseTable
		model   map[modelCell]string
		ordered bool
	}
	hashTranspose, hashClone := hash.Transpose(), hash.Clone()
	treeTranspose, treeClone := tree.Transpose(), tree.Clone()
	subjects := []subject{
		{label: "hash transpose", table: hashTranspose, model: transposeModel(sourceModel)},
		{label: "hash clone", table: hashClone, model: transposeModel(transposeModel(sourceModel))},
		{label: "tree transpose", table: treeTranspose, model: transposeModel(sourceModel), ordered: true},
		{label: "tree clone", table: treeClone, model: transposeModel(transposeModel(sourceModel)), ordered: true},
	}
	for step := 0; step < 3000; step++ {
		row, column := rng.Intn(keySpace), rng.Intn(keySpace)
		put := rng.Intn(2) == 0
		value := fmt.Sprintf("s%d", step)
		for _, subject := range subjects {
			key := modelCell{row: row, column: column}
			wantOld, wantFound := subject.model[key]
			var old string
			var found bool
			if put {
				old, found = subject.table.Put(row, column, value)
				subject.model[key] = value
			} else {
				old, found = subject.table.Delete(row, column)
				delete(subject.model, key)
			}
			if old != wantOld || found != wantFound {
				t.Fatalf("%s step %d: got %q,%v want %q,%v", subject.label, step, old, found, wantOld, wantFound)
			}
		}
	}
	checkSparseAgainstModel(t, "hash transpose", hashTranspose, func() []Cell[int, int, string] { return collectCells(hashTranspose.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { hashTranspose.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { hashTranspose.Columns()(f) }, subjects[0].model, false, keySpace)
	checkSparseAgainstModel(t, "hash clone", hashClone, func() []Cell[int, int, string] { return collectCells(hashClone.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { hashClone.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { hashClone.Columns()(f) }, subjects[1].model, false, keySpace)
	checkSparseAgainstModel(t, "tree transpose", treeTranspose, func() []Cell[int, int, string] { return collectCells(treeTranspose.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { treeTranspose.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { treeTranspose.Columns()(f) }, subjects[2].model, true, keySpace)
	checkSparseAgainstModel(t, "tree clone", treeClone, func() []Cell[int, int, string] { return collectCells(treeClone.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { treeClone.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { treeClone.Columns()(f) }, subjects[3].model, true, keySpace)
	checkTreeTableShape(t, treeTranspose)
	checkTreeTableShape(t, treeClone)
	// The sources were never touched by the subjects' writes.
	checkSparseAgainstModel(t, "hash source", hash, func() []Cell[int, int, string] { return collectCells(hash.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { hash.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { hash.Columns()(f) }, sourceModel, false, keySpace)
	checkSparseAgainstModel(t, "tree source", tree, func() []Cell[int, int, string] { return collectCells(tree.Cells()) },
		func(f func(int, []Cell[int, int, string]) bool) { tree.Rows()(f) },
		func(f func(int, []Cell[int, int, string]) bool) { tree.Columns()(f) }, sourceModel, true, keySpace)
}

// checkTreeTableShape validates the AVL invariants of the row index and of
// every row's column index.
func checkTreeTableShape[R any, C any, V any](t *testing.T, table *TreeTable[R, C, V]) {
	t.Helper()
	checkTreeIndex(t, table.rows)
	cells := 0
	for _, columns := range table.rows.all() {
		if columns.count == 0 {
			t.Fatal("empty row retained in TreeTable")
		}
		checkTreeIndex(t, columns)
		cells += columns.count
	}
	if cells != table.cellCount {
		t.Fatalf("cellCount %d, rows hold %d", table.cellCount, cells)
	}
}

// checkTreeIndex verifies strict key order, cached heights, |balance| <= 1 and
// count.
func checkTreeIndex[K any, V any](t *testing.T, index *treeIndex[K, V]) {
	t.Helper()
	var walk func(node *treeNode[K, V]) (int32, int)
	walk = func(node *treeNode[K, V]) (int32, int) {
		if node == nil {
			return 0, 0
		}
		if node.left != nil && index.compare(node.left.key, node.key) >= 0 {
			t.Fatal("tree order violated on the left")
		}
		if node.right != nil && index.compare(node.right.key, node.key) <= 0 {
			t.Fatal("tree order violated on the right")
		}
		leftHeight, leftCount := walk(node.left)
		rightHeight, rightCount := walk(node.right)
		if balance := leftHeight - rightHeight; balance > 1 || balance < -1 {
			t.Fatalf("AVL balance %d", balance)
		}
		if node.height != max(leftHeight, rightHeight)+1 {
			t.Fatalf("cached height %d, actual %d", node.height, max(leftHeight, rightHeight)+1)
		}
		return node.height, leftCount + rightCount + 1
	}
	if _, count := walk(index.root); count != index.count {
		t.Fatalf("count %d, nodes %d", index.count, count)
	}
	// In-order iteration must be strictly ascending (whole-tree order).
	first := true
	var previous K
	for key := range index.all() {
		if !first && index.compare(previous, key) >= 0 {
			t.Fatal("in-order iteration not strictly ascending")
		}
		first, previous = false, key
	}
}

// TestTreeIndexRandomizedModel exercises treeIndex alone against a map, with
// every structural invariant checked after each operation.
func TestTreeIndexRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	index := newTreeIndex[int, int](cmp.Compare[int])
	model := map[int]int{}
	for step := 0; step < 5000; step++ {
		key := rng.Intn(200)
		wantOld, wantFound := model[key]
		var old int
		var found bool
		if rng.Intn(3) != 0 {
			old, found = index.put(key, step)
			model[key] = step
		} else {
			old, found = index.delete(key)
			delete(model, key)
		}
		if old != wantOld || found != wantFound {
			t.Fatalf("step %d key %d: got %d,%v want %d,%v", step, key, old, found, wantOld, wantFound)
		}
		checkTreeIndex(t, index)
		if value, found := index.get(key); found != (model[key] == value && func() bool { _, ok := model[key]; return ok }()) {
			t.Fatalf("step %d: get(%d) = %d,%v", step, key, value, found)
		}
	}
	keys := make([]int, 0, len(model))
	for key := range model {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	values := make([]int, len(keys))
	for index, key := range keys {
		values[index] = model[key]
	}
	built := buildTreeIndex(cmp.Compare[int], keys, values)
	checkTreeIndex(t, &built)
	cloned := built.clone()
	checkTreeIndex(t, cloned)
	position := 0
	for key, value := range cloned.all() {
		if key != keys[position] || value != values[position] {
			t.Fatalf("clone entry %d = %d:%d", position, key, value)
		}
		position++
	}
	// Deleting everything from a slab-built tree keeps the invariants.
	for _, key := range keys {
		if _, found := cloned.delete(key); !found {
			t.Fatalf("clone lost %d", key)
		}
		checkTreeIndex(t, cloned)
	}
	if cloned.count != 0 || cloned.root != nil || built.count != len(keys) {
		t.Fatal("clone/builder not independent")
	}
}

// TestHashIndexRandomizedModel exercises hashIndex alone with a hash that has
// 8 values (including 0), so backward-shift deletion runs across wrapped
// clusters, and checks the probe invariant after every operation.
func TestHashIndexRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	index := newHashIndex[int, int](intEqual, collidingHash)
	model := map[int]int{}
	for step := 0; step < 8000; step++ {
		key := rng.Intn(150)
		wantOld, wantFound := model[key]
		var old int
		var found bool
		if rng.Intn(5) < 3 {
			old, found = index.put(key, step)
			model[key] = step
		} else {
			old, found = index.delete(key)
			delete(model, key)
		}
		if old != wantOld || found != wantFound {
			t.Fatalf("step %d key %d: got %d,%v want %d,%v", step, key, old, found, wantOld, wantFound)
		}
		checkHashIndex(t, index, model)
	}
	cloned := index.clone()
	checkHashIndex(t, cloned, model)
	cloned.put(-1, -1)
	if _, found := index.get(-1); found {
		t.Fatal("clone shares slots with its source")
	}
	index.clear()
	if index.len() != 0 || index.slots != nil {
		t.Fatal("clear left entries")
	}
	if _, found := index.get(3); found {
		t.Fatal("cleared index still finds keys")
	}
}

func checkHashIndex(t *testing.T, index *hashIndex[int, int], model map[int]int) {
	t.Helper()
	if index.count != len(model) {
		t.Fatalf("count %d, want %d", index.count, len(model))
	}
	if index.slots != nil && (index.count+1)*4 > len(index.slots)*3+4 {
		t.Fatalf("load %d/%d above 3/4", index.count, len(index.slots))
	}
	mask := len(index.slots) - 1
	seen := 0
	for position, slot := range index.slots {
		if slot.hash == 0 {
			if slot.key != 0 || slot.value != 0 {
				t.Fatalf("empty slot %d retains %d:%d", position, slot.key, slot.value)
			}
			continue
		}
		seen++
		if slot.hash != fixHash(collidingHash(slot.key)) {
			t.Fatalf("slot %d caches hash %d for key %d", position, slot.hash, slot.key)
		}
		// Linear-probing invariant: no empty slot between home and position.
		for probe := index.home(slot.hash); probe != position; probe = (probe + 1) & mask {
			if index.slots[probe].hash == 0 {
				t.Fatalf("key %d at %d unreachable: empty slot %d on its probe chain", slot.key, position, probe)
			}
		}
		if want, ok := model[slot.key]; !ok || want != slot.value {
			t.Fatalf("slot %d holds %d:%d, model %d,%v", position, slot.key, slot.value, want, ok)
		}
	}
	if seen != len(model) {
		t.Fatalf("%d occupied slots, model %d", seen, len(model))
	}
	for key, value := range model {
		if got, found := index.get(key); !found || got != value {
			t.Fatalf("get(%d) = %d,%v want %d", key, got, found, value)
		}
	}
}

// TestHashIndexReplacesWholeEntry pins that an equal key replaces the stored
// key as well as the value (the hash_grow Insert semantics the index replaced),
// for both sparse variants' column level.
func TestHashIndexReplacesWholeEntry(t *testing.T) {
	type key struct {
		id  int
		tag string
	}
	equal := func(a, b key) bool { return a.id == b.id }
	hash := func(k key) uint64 { return uint64(k.id) }
	table := NewHashTableFunc[int, key, string](intEqual, func(v int) uint64 { return uint64(v) }, equal, hash)
	table.Put(1, key{id: 5, tag: "old"}, "a")
	table.Put(1, key{id: 5, tag: "new"}, "b")
	if row := table.Row(1); len(row) != 1 || row[0].Column.tag != "new" || row[0].Value != "b" {
		t.Fatalf("hash row = %v", row)
	}
	tree := NewTreeTableFunc[int, key, string](cmp.Compare[int], func(a, b key) int { return cmp.Compare(a.id, b.id) })
	tree.Put(1, key{id: 5, tag: "old"}, "a")
	tree.Put(1, key{id: 5, tag: "new"}, "b")
	if row := tree.Row(1); len(row) != 1 || row[0].Column.tag != "new" || row[0].Value != "b" {
		t.Fatalf("tree row = %v", row)
	}
}

// TestArrayTableRandomizedModel compares both ArrayTable constructors (the
// built-in-map universe and the custom-function universe with colliding
// hashes) with a dense model, including Row, Column, Transpose and Clone.
func TestArrayTableRandomizedModel(t *testing.T) {
	rowKeys := []int{4, 9, 1, 4, 7, 0, 12} // 4 repeats: first occurrence wins
	columnKeys := []int{3, 8, 5, 3, 11, 2} // 3 repeats
	distinctRows := []int{4, 9, 1, 7, 0, 12}
	distinctColumns := []int{3, 8, 5, 11, 2}
	natural := NewArrayTable[int, int, string](rowKeys, columnKeys)
	custom := NewArrayTableFunc[int, int, string](rowKeys, columnKeys, intEqual, collidingHash, intEqual, collidingHash)
	model := map[modelCell]string{}
	rng := rand.New(rand.NewSource(5150))
	for step := 0; step < 3000; step++ {
		row, column := rng.Intn(14), rng.Intn(13) // includes out-of-universe keys
		inUniverse := slices.Contains(distinctRows, row) && slices.Contains(distinctColumns, column)
		key := modelCell{row: row, column: column}
		wantOld, wantFound := model[key]
		for _, table := range []*ArrayTable[int, int, string]{natural, custom} {
			var old string
			var found bool
			var err error
			if step%3 == 0 {
				old, found, err = table.Delete(row, column)
			} else {
				old, found, err = table.Put(row, column, fmt.Sprint(step))
			}
			if (err != nil) != !inUniverse || old != wantOld || found != wantFound {
				t.Fatalf("step %d (%d,%d): %q,%v,%v want %q,%v in=%v", step, row, column, old, found, err, wantOld, wantFound, inUniverse)
			}
		}
		if inUniverse {
			if step%3 == 0 {
				delete(model, key)
			} else {
				model[key] = fmt.Sprint(step)
			}
		}
		if step%300 != 299 {
			continue
		}
		for label, table := range map[string]*ArrayTable[int, int, string]{"natural": natural, "custom": custom} {
			checkArrayAgainstModel(t, label, table, distinctRows, distinctColumns, model)
			checkArrayAgainstModel(t, label+" clone", table.Clone(), distinctRows, distinctColumns, model)
			checkArrayAgainstModel(t, label+" transpose", table.Transpose(), distinctColumns, distinctRows, transposeModel(model))
			checkArrayAgainstModel(t, label+" transpose twice", table.Transpose().Transpose(), distinctRows, distinctColumns, model)
		}
	}
}

func checkArrayAgainstModel(t *testing.T, label string, table *ArrayTable[int, int, string], rows, columns []int, model map[modelCell]string) {
	t.Helper()
	if table.Len() != len(model) {
		t.Fatalf("%s: Len %d want %d", label, table.Len(), len(model))
	}
	if got := slices.Collect(table.RowKeys()); !slices.Equal(got, rows) {
		t.Fatalf("%s: RowKeys %v want %v", label, got, rows)
	}
	if got := slices.Collect(table.ColumnKeys()); !slices.Equal(got, columns) {
		t.Fatalf("%s: ColumnKeys %v want %v", label, got, columns)
	}
	var want []Cell[int, int, string]
	for _, row := range rows {
		for _, column := range columns {
			if value, ok := model[modelCell{row: row, column: column}]; ok {
				want = append(want, Cell[int, int, string]{Row: row, Column: column, Value: value})
			}
		}
	}
	sameCells(t, label+" Cells", collectCells(table.Cells()), want)
	for rowIndex, row := range rows {
		var wantRow []Cell[int, int, string]
		for columnIndex, column := range columns {
			value, ok := model[modelCell{row: row, column: column}]
			if ok {
				wantRow = append(wantRow, Cell[int, int, string]{Row: row, Column: column, Value: value})
			}
			got, found := table.GetAt(rowIndex, columnIndex)
			if found != ok || got != value {
				t.Fatalf("%s: GetAt(%d,%d) = %q,%v", label, rowIndex, columnIndex, got, found)
			}
		}
		sameCells(t, fmt.Sprintf("%s Row(%d)", label, row), table.Row(row), wantRow)
	}
	for _, column := range columns {
		var wantColumn []Cell[int, int, string]
		for _, row := range rows {
			if value, ok := model[modelCell{row: row, column: column}]; ok {
				wantColumn = append(wantColumn, Cell[int, int, string]{Row: row, Column: column, Value: value})
			}
		}
		sameCells(t, fmt.Sprintf("%s Column(%d)", label, column), table.Column(column), wantColumn)
	}
	if !table.ContainsRow(rows[0]) || table.ContainsRow(-1) || !table.ContainsColumn(columns[0]) || table.ContainsColumn(-1) {
		t.Fatalf("%s: universe membership wrong", label)
	}
}

// TestSnapshotIndependence proves that Row, Column, Rows, Columns, Cells,
// Transpose and Clone results are copies: writing through them never reaches
// the table, and writing to the table never reaches them.
func TestSnapshotIndependence(t *testing.T) {
	keys := []string{"a", "b", "c"}
	newHash := func() snapshotSubject {
		table := NewHashTable[string, string, string]()
		return snapshotSubject{table: hashSubject{table}}
	}
	newTree := func() snapshotSubject {
		table := NewTreeTable[string, string, string]()
		return snapshotSubject{table: treeSubject{table}}
	}
	newArray := func() snapshotSubject {
		table := NewArrayTable[string, string, string](keys, keys)
		return snapshotSubject{table: arraySubject{table}}
	}
	for name, constructor := range map[string]func() snapshotSubject{"hash": newHash, "tree": newTree, "array": newArray} {
		subject := constructor()
		for _, row := range keys {
			for _, column := range keys {
				subject.table.put(row, column, row+column)
			}
		}
		row := subject.table.row("b")
		column := subject.table.column("b")
		for index := range row {
			row[index].Value = "scribbled"
			row[index].Column = "zz"
		}
		for index := range column {
			column[index].Value = "scribbled"
		}
		_ = append(row, Cell[string, string, string]{Row: "b", Column: "appended", Value: "x"})
		for _, r := range keys {
			for _, c := range keys {
				if value, _ := subject.table.get(r, c); value != r+c {
					t.Fatalf("%s: writing a snapshot changed (%s,%s) to %q", name, r, c, value)
				}
			}
		}
		rowBefore := subject.table.row("a")
		transposed, cloned := subject.table.transpose(), subject.table.clone()
		subject.table.put("a", "a", "changed")
		subject.table.put("a", "b", "changed")
		for _, cell := range rowBefore {
			if cell.Value == "changed" {
				t.Fatalf("%s: table write reached an earlier Row snapshot", name)
			}
		}
		if value, _ := transposed.get("a", "a"); value != "aa" {
			t.Fatalf("%s: table write reached Transpose result: %q", name, value)
		}
		if value, _ := transposed.get("b", "a"); value != "ab" {
			t.Fatalf("%s: Transpose (b,a) = %q, want ab", name, value)
		}
		if value, _ := cloned.get("a", "b"); value != "ab" {
			t.Fatalf("%s: table write reached Clone result: %q", name, value)
		}
		transposed.put("c", "c", "from-transpose")
		cloned.put("c", "c", "from-clone")
		cloned.del("b", "b")
		if value, _ := subject.table.get("c", "c"); value != "cc" {
			t.Fatalf("%s: Transpose/Clone write reached the table: %q", name, value)
		}
		if _, found := subject.table.get("b", "b"); !found {
			t.Fatalf("%s: Clone delete reached the table", name)
		}
		if value, _ := transposed.get("c", "c"); value != "from-transpose" {
			t.Fatalf("%s: clone write reached the transpose: %q", name, value)
		}
	}
}

type snapshotOps interface {
	put(row, column, value string)
	get(row, column string) (string, bool)
	del(row, column string)
	row(row string) []Cell[string, string, string]
	column(column string) []Cell[string, string, string]
	transpose() snapshotOps
	clone() snapshotOps
}

type snapshotSubject struct{ table snapshotOps }

type hashSubject struct {
	*HashTable[string, string, string]
}

func (s hashSubject) put(r, c, v string)                             { s.Put(r, c, v) }
func (s hashSubject) get(r, c string) (string, bool)                 { return s.Get(r, c) }
func (s hashSubject) del(r, c string)                                { s.Delete(r, c) }
func (s hashSubject) row(r string) []Cell[string, string, string]    { return s.Row(r) }
func (s hashSubject) column(c string) []Cell[string, string, string] { return s.Column(c) }
func (s hashSubject) transpose() snapshotOps                         { return hashSubject{s.Transpose()} }
func (s hashSubject) clone() snapshotOps                             { return hashSubject{s.Clone()} }

type treeSubject struct {
	*TreeTable[string, string, string]
}

func (s treeSubject) put(r, c, v string)                             { s.Put(r, c, v) }
func (s treeSubject) get(r, c string) (string, bool)                 { return s.Get(r, c) }
func (s treeSubject) del(r, c string)                                { s.Delete(r, c) }
func (s treeSubject) row(r string) []Cell[string, string, string]    { return s.Row(r) }
func (s treeSubject) column(c string) []Cell[string, string, string] { return s.Column(c) }
func (s treeSubject) transpose() snapshotOps                         { return treeSubject{s.Transpose()} }
func (s treeSubject) clone() snapshotOps                             { return treeSubject{s.Clone()} }

type arraySubject struct {
	*ArrayTable[string, string, string]
}

func (s arraySubject) put(r, c, v string)                             { _, _, _ = s.Put(r, c, v) }
func (s arraySubject) get(r, c string) (string, bool)                 { return s.Get(r, c) }
func (s arraySubject) del(r, c string)                                { _, _, _ = s.Delete(r, c) }
func (s arraySubject) row(r string) []Cell[string, string, string]    { return s.Row(r) }
func (s arraySubject) column(c string) []Cell[string, string, string] { return s.Column(c) }
func (s arraySubject) transpose() snapshotOps                         { return arraySubject{s.Transpose()} }
func (s arraySubject) clone() snapshotOps                             { return arraySubject{s.Clone()} }

// TestIteratorSnapshotsIndependent covers Rows and Columns groups: each yield
// is a fresh copy, so scribbling on one cannot change the table or a later
// iteration.
func TestIteratorSnapshotsIndependent(t *testing.T) {
	hash := NewHashTable[int, int, string]()
	tree := NewTreeTable[int, int, string]()
	array := NewArrayTable[int, int, string]([]int{0, 1, 2}, []int{0, 1, 2})
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			value := fmt.Sprint(row, column)
			hash.Put(row, column, value)
			tree.Put(row, column, value)
			_, _, _ = array.Put(row, column, value)
		}
	}
	scribble := func(_ int, cells []Cell[int, int, string]) bool {
		for index := range cells {
			cells[index].Value = "x"
		}
		return true
	}
	hash.Rows()(scribble)
	hash.Columns()(scribble)
	tree.Rows()(scribble)
	tree.Columns()(scribble)
	array.Rows()(scribble)
	array.Columns()(scribble)
	columns := tree.Columns()
	columns(scribble)
	columns(func(_ int, cells []Cell[int, int, string]) bool {
		for _, cell := range cells {
			if cell.Value == "x" {
				t.Fatal("a Columns iterator re-yielded a scribbled group")
			}
		}
		return true
	})
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			want := fmt.Sprint(row, column)
			for label, get := range map[string]func(int, int) (string, bool){"hash": hash.Get, "tree": tree.Get, "array": array.Get} {
				if value, _ := get(row, column); value != want {
					t.Fatalf("%s (%d,%d) = %q", label, row, column, value)
				}
			}
		}
	}
}

// TestSnapshotAllocations pins the one-allocation copies: Row and Column
// allocate exactly their result, and the dense Transpose allocates only the
// new table and its matrix (the universe is shared).
func TestSnapshotAllocations(t *testing.T) {
	hash := fullHashTable(16, 16)
	tree := NewTreeTable[string, string, string]()
	fillTable(tree.Put, 16, 16)
	array := fullArrayTable(16, 16)
	cases := []struct {
		name string
		max  float64
		run  func()
	}{
		{"HashTable.Row", 1, func() { _ = hash.Row("row-003") }},
		{"HashTable.Column", 1, func() { _ = hash.Column("column-003") }},
		{"TreeTable.Row", 1, func() { _ = tree.Row("row-003") }},
		{"TreeTable.Column", 1, func() { _ = tree.Column("column-003") }},
		{"ArrayTable.Row", 1, func() { _ = array.Row("row-003") }},
		{"ArrayTable.Column", 1, func() { _ = array.Column("column-003") }},
		{"ArrayTable.Transpose", 3, func() { _ = array.Transpose() }},
		{"ArrayTable.Clone", 3, func() { _ = array.Clone() }},
		{"HashTable.Get", 0, func() { _, _ = hash.Get("row-003", "column-004") }},
		{"TreeTable.Get", 0, func() { _, _ = tree.Get("row-003", "column-004") }},
		{"TreeTable.Put replace", 0, func() { tree.Put("row-003", "column-004", "value") }},
		{"HashTable.Put replace", 0, func() { hash.Put("row-003", "column-004", "value") }},
	}
	for _, c := range cases {
		if allocs := testing.AllocsPerRun(50, c.run); allocs > c.max {
			t.Errorf("%s: %.1f allocations, want at most %.0f", c.name, allocs, c.max)
		}
	}
	// A transposed tree builds one node slab per row instead of one node per
	// cell: far fewer allocations than cells.
	if allocs := testing.AllocsPerRun(10, func() { _ = tree.Transpose() }); allocs > 3*16+16 {
		t.Errorf("TreeTable.Transpose: %.0f allocations for 256 cells", allocs)
	}
}

// TestColumnCapacityHint pins the preallocation bound for column snapshots.
func TestColumnCapacityHint(t *testing.T) {
	if got := columnCapacityHint(10, 4); got != 4 {
		t.Fatalf("hint(10,4) = %d", got)
	}
	if got := columnCapacityHint(3, 40); got != 3 {
		t.Fatalf("hint(3,40) = %d", got)
	}
	if got := columnCapacityHint(1<<20, 1<<20); got != maxColumnPrealloc {
		t.Fatalf("hint(big) = %d", got)
	}
	// A sparse column in a table with many rows still returns exactly its
	// cells.
	table := NewHashTable[int, int, int]()
	for row := 0; row < 3000; row++ {
		table.Put(row, row%1000, row)
	}
	if got := len(table.Column(7)); got != 3 {
		t.Fatalf("sparse column has %d cells", got)
	}
}

// TestTransposeWork pins the bulk-transpose work bounds that the results alone
// cannot show: the sorted-merge regrouping of TreeTable.Transpose (about one
// column comparison per cell when rows share columns, and no tree lookups for
// columns the previous row also had), and the presized destination rows of
// HashTable.Transpose (no growth allocations).
func TestTransposeWork(t *testing.T) {
	comparisons := 0
	counting := func(a, b int) int {
		comparisons++
		return cmp.Compare(a, b)
	}
	const n = 64
	full := NewTreeTableFunc[int, int, int](cmp.Compare[int], counting)
	for row := 0; row < n; row++ {
		for column := 0; column < n; column++ {
			full.Put(row, column, row*n+column)
		}
	}
	comparisons = 0
	full.Transpose()
	if limit := n * n * 13 / 10; comparisons > limit {
		t.Errorf("full TreeTable transpose: %d column comparisons for %d cells, want <= %d", comparisons, n*n, limit)
	}
	// Rows that drop different columns force the cursor to skip. A third of
	// each row's columns are absent from the previous row and cost a
	// temporary-index lookup (about log2(64) comparisons), so the budget is
	// five comparisons per cell; a cursor that cannot skip sends nearly every
	// cell to the lookup and exceeds it.
	ragged := NewTreeTableFunc[int, int, int](cmp.Compare[int], counting)
	cells := 0
	for row := 0; row < n; row++ {
		for column := 0; column < n; column++ {
			if (row+column)%3 != 0 {
				ragged.Put(row, column, row*n+column)
				cells++
			}
		}
	}
	comparisons = 0
	transposed := ragged.Transpose()
	t.Logf("comparisons: ragged %d for %d cells", comparisons, cells)
	if limit := cells * 5; comparisons > limit {
		t.Errorf("ragged TreeTable transpose: %d column comparisons for %d cells, want <= %d", comparisons, cells, limit)
	}
	checkTreeTableShape(t, transposed)

	hash := NewHashTable[int, int, int]()
	for row := 0; row < n; row++ {
		for column := 0; column < n; column++ {
			hash.Put(row, column, row)
		}
	}
	// Result struct, outer index struct and its slot arrays (growth to 128
	// slots), then one index struct and one slot array per destination row.
	allocs := testing.AllocsPerRun(10, func() { hash.Transpose() })
	t.Logf("hash transpose allocations: %.0f", allocs)
	if allocs > 2*n+8 {
		t.Errorf("HashTable transpose: %.0f allocations for %d destination rows, want <= %d", allocs, n, 2*n+8)
	}
}

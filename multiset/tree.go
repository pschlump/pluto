/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"cmp"
	"fmt"
	"iter"
	"sort"
)

type treeNode[T any] struct {
	value  T
	count  int
	height int
	left   *treeNode[T]
	right  *treeNode[T]
}

// TreeMultiset is an AVL-backed sorted multiset. Range methods return
// independent count-preserving snapshots, not live views.
type TreeMultiset[T any] struct {
	root     *treeNode[T]
	distinct int
	size     int
	cmp      func(a, b T) int
}

// NewTreeMultiset creates an empty multiset in natural order.
func NewTreeMultiset[T cmp.Ordered]() *TreeMultiset[T] {
	return NewTreeMultisetFunc[T](compareOrdered[T])
}

// NewTreeMultisetFunc creates an empty multiset with a custom comparator.
func NewTreeMultisetFunc[T any](compare func(a, b T) int) *TreeMultiset[T] {
	if compare == nil {
		panic("multiset: NewTreeMultisetFunc called with a nil comparison function")
	}
	return &TreeMultiset[T]{cmp: compare}
}

// Compare returns the stored comparison function, or nil for a zero value.
func (m *TreeMultiset[T]) Compare() func(a, b T) int {
	if m == nil {
		return nil
	}
	return m.cmp
}

func compareOrdered[T cmp.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// FromTreeSlice creates a sorted multiset containing one occurrence per value.
func FromTreeSlice[T cmp.Ordered](values []T) *TreeMultiset[T] {
	result := NewTreeMultiset[T]()
	result.AddSlice(values)
	return result
}

// FromTreeSliceFunc is FromTreeSlice with a custom comparator.
func FromTreeSliceFunc[T any](compare func(a, b T) int, values []T) *TreeMultiset[T] {
	result := NewTreeMultisetFunc(compare)
	result.AddSlice(values)
	return result
}

func (m *TreeMultiset[T]) writeContract(caller string) {
	if m == nil || m.cmp == nil {
		panic(fmt.Sprintf("multiset: %s on a nil or zero-value TreeMultiset (use NewTreeMultiset or NewTreeMultisetFunc)", caller))
	}
}

func nodeHeight[T any](node *treeNode[T]) int {
	if node == nil {
		return 0
	}
	return node.height
}

func (m *TreeMultiset[T]) findNode(value T) *treeNode[T] {
	if m == nil || m.cmp == nil {
		return nil
	}
	node := m.root
	for node != nil {
		comparison := m.cmp(value, node.value)
		if comparison < 0 {
			node = node.left
		} else if comparison > 0 {
			node = node.right
		} else {
			return node
		}
	}
	return nil
}

// Count returns the number of occurrences of value.
func (m *TreeMultiset[T]) Count(value T) int {
	node := m.findNode(value)
	if node == nil {
		return 0
	}
	return node.count
}

// Add adds occurrences of value and returns its resulting count.
func (m *TreeMultiset[T]) Add(value T, occurrences int) int {
	checkedCount("Add", occurrences)
	old := m.Count(value)
	if occurrences == 0 {
		return old
	}
	m.writeContract("Add")
	checkedAddition(old, occurrences)
	checkedAddition(m.size, occurrences)
	m.root = m.insert(m.root, value, occurrences)
	m.size += occurrences
	if old == 0 {
		m.distinct++
	}
	return old + occurrences
}

// Remove removes up to occurrences of value and returns its resulting count.
func (m *TreeMultiset[T]) Remove(value T, occurrences int) int {
	checkedCount("Remove", occurrences)
	node := m.findNode(value)
	if node == nil || occurrences == 0 {
		if node == nil {
			return 0
		}
		return node.count
	}
	if occurrences >= node.count {
		old := node.count
		m.root = m.delete(m.root, value)
		m.distinct--
		m.size -= old
		return 0
	}
	node.count -= occurrences
	m.size -= occurrences
	return node.count
}

// SetCount changes value's count and returns its previous count.
func (m *TreeMultiset[T]) SetCount(value T, count int) int {
	checkedCount("SetCount", count)
	node := m.findNode(value)
	old := 0
	if node != nil {
		old = node.count
	}
	if old == count {
		return old
	}
	if count == 0 {
		m.root = m.delete(m.root, value)
		m.distinct--
		m.size -= old
		return old
	}
	m.writeContract("SetCount")
	if count > old {
		checkedAddition(m.size, count-old)
	}
	if node == nil {
		m.root = m.insert(m.root, value, count)
		m.distinct++
	} else {
		node.count = count
	}
	m.size += count - old
	return old
}

// DeleteAll removes every occurrence of value and returns its previous count.
func (m *TreeMultiset[T]) DeleteAll(value T) int {
	return m.SetCount(value, 0)
}

// Contains reports whether value has a positive count.
func (m *TreeMultiset[T]) Contains(value T) bool {
	return m.findNode(value) != nil
}

// Len returns the total number of occurrences.
func (m *TreeMultiset[T]) Len() int {
	if m == nil {
		return 0
	}
	return m.size
}

// DistinctLen returns the number of values with positive counts.
func (m *TreeMultiset[T]) DistinctLen() int {
	if m == nil {
		return 0
	}
	return m.distinct
}

// IsEmpty reports whether the multiset has no occurrences.
func (m *TreeMultiset[T]) IsEmpty() bool {
	return m.Len() == 0
}

// Clear removes all values while retaining the comparator.
func (m *TreeMultiset[T]) Clear() {
	if m == nil {
		return
	}
	m.root = nil
	m.distinct = 0
	m.size = 0
}

func newTreeNode[T any](value T, count int) *treeNode[T] {
	return &treeNode[T]{value: value, count: count, height: 1}
}

func updateHeight[T any](node *treeNode[T]) {
	node.height = 1 + max(nodeHeight(node.left), nodeHeight(node.right))
}

func rotateRight[T any](node *treeNode[T]) *treeNode[T] {
	left := node.left
	node.left = left.right
	left.right = node
	updateHeight(node)
	updateHeight(left)
	return left
}

func rotateLeft[T any](node *treeNode[T]) *treeNode[T] {
	right := node.right
	node.right = right.left
	right.left = node
	updateHeight(node)
	updateHeight(right)
	return right
}

func balance[T any](node *treeNode[T]) *treeNode[T] {
	updateHeight(node)
	difference := nodeHeight(node.left) - nodeHeight(node.right)
	if difference > 1 {
		if nodeHeight(node.left.left) < nodeHeight(node.left.right) {
			node.left = rotateLeft(node.left)
		}
		return rotateRight(node)
	}
	if difference < -1 {
		if nodeHeight(node.right.right) < nodeHeight(node.right.left) {
			node.right = rotateRight(node.right)
		}
		return rotateLeft(node)
	}
	return node
}

func (m *TreeMultiset[T]) insert(node *treeNode[T], value T, count int) *treeNode[T] {
	if node == nil {
		return newTreeNode(value, count)
	}
	comparison := m.cmp(value, node.value)
	if comparison < 0 {
		node.left = m.insert(node.left, value, count)
	} else if comparison > 0 {
		node.right = m.insert(node.right, value, count)
	} else {
		node.count += count
		return node
	}
	return balance(node)
}

func (m *TreeMultiset[T]) deleteMin(node *treeNode[T]) (*treeNode[T], *treeNode[T]) {
	if node.left == nil {
		return node.right, node
	}
	var minimum *treeNode[T]
	node.left, minimum = m.deleteMin(node.left)
	return balance(node), minimum
}

func (m *TreeMultiset[T]) delete(node *treeNode[T], value T) *treeNode[T] {
	if node == nil {
		return nil
	}
	comparison := m.cmp(value, node.value)
	if comparison < 0 {
		node.left = m.delete(node.left, value)
	} else if comparison > 0 {
		node.right = m.delete(node.right, value)
	} else {
		if node.left == nil {
			return node.right
		}
		if node.right == nil {
			return node.left
		}
		var successor *treeNode[T]
		node.right, successor = m.deleteMin(node.right)
		node.value = successor.value
		node.count = successor.count
	}
	return balance(node)
}

// FindMin returns the smallest distinct value.
func (m *TreeMultiset[T]) FindMin() (T, bool) {
	if m == nil || m.root == nil {
		var zero T
		return zero, false
	}
	node := m.root
	for node.left != nil {
		node = node.left
	}
	return node.value, true
}

// FindMax returns the largest distinct value.
func (m *TreeMultiset[T]) FindMax() (T, bool) {
	if m == nil || m.root == nil {
		var zero T
		return zero, false
	}
	node := m.root
	for node.right != nil {
		node = node.right
	}
	return node.value, true
}

// Floor returns the largest distinct value no greater than value.
func (m *TreeMultiset[T]) Floor(value T) (T, bool) {
	var result T
	found := false
	if m == nil || m.cmp == nil {
		return result, false
	}
	for node := m.root; node != nil; {
		comparison := m.cmp(value, node.value)
		if comparison < 0 {
			node = node.left
		} else {
			result, found = node.value, true
			if comparison == 0 {
				break
			}
			node = node.right
		}
	}
	return result, found
}

// Ceil returns the smallest distinct value no less than value.
func (m *TreeMultiset[T]) Ceil(value T) (T, bool) {
	var result T
	found := false
	if m == nil || m.cmp == nil {
		return result, false
	}
	for node := m.root; node != nil; {
		comparison := m.cmp(value, node.value)
		if comparison > 0 {
			node = node.right
		} else {
			result, found = node.value, true
			if comparison == 0 {
				break
			}
			node = node.left
		}
	}
	return result, found
}

func walkEntries[T any](node *treeNode[T], yield func(Entry[T]) bool) bool {
	if node == nil {
		return true
	}
	if !walkEntries(node.left, yield) {
		return false
	}
	if !yield(Entry[T]{Value: node.value, Count: node.count}) {
		return false
	}
	return walkEntries(node.right, yield)
}

func walkEntriesBackward[T any](node *treeNode[T], yield func(Entry[T]) bool) bool {
	if node == nil {
		return true
	}
	if !walkEntriesBackward(node.right, yield) {
		return false
	}
	if !yield(Entry[T]{Value: node.value, Count: node.count}) {
		return false
	}
	return walkEntriesBackward(node.left, yield)
}

// Entries iterates over distinct entries in ascending order.
func (m *TreeMultiset[T]) Entries() iter.Seq[Entry[T]] {
	return func(yield func(Entry[T]) bool) {
		if m != nil {
			walkEntries(m.root, yield)
		}
	}
}

// EntriesBackward iterates over distinct entries in descending order.
func (m *TreeMultiset[T]) EntriesBackward() iter.Seq[Entry[T]] {
	return func(yield func(Entry[T]) bool) {
		if m != nil {
			walkEntriesBackward(m.root, yield)
		}
	}
}

// Elements iterates over distinct values in ascending order.
func (m *TreeMultiset[T]) Elements() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

// All iterates over every occurrence in ascending order.
func (m *TreeMultiset[T]) All() iter.Seq[T] {
	return occurrences(m.Entries())
}

// Backward iterates over every occurrence in descending order.
func (m *TreeMultiset[T]) Backward() iter.Seq[T] {
	return occurrences(m.EntriesBackward())
}

func occurrences[T any](entries iter.Seq[Entry[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range entries {
			for range entry.Count {
				if !yield(entry.Value) {
					return
				}
			}
		}
	}
}

// EntriesSlice returns a fresh ascending entry slice.
func (m *TreeMultiset[T]) EntriesSlice() []Entry[T] {
	if m == nil {
		return nil
	}
	entries := make([]Entry[T], 0, m.DistinctLen())
	for entry := range m.Entries() {
		entries = append(entries, entry)
	}
	return entries
}

// Slice returns a fresh ascending slice containing every occurrence.
func (m *TreeMultiset[T]) Slice() []T {
	if m == nil {
		return nil
	}
	values := make([]T, 0, m.Len())
	for value := range m.All() {
		values = append(values, value)
	}
	return values
}

// HighestCountFirst returns entries by decreasing count, breaking ties in
// comparator ascending order.
func (m *TreeMultiset[T]) HighestCountFirst() []Entry[T] {
	entries := m.EntriesSlice()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	return entries
}

// Clone returns an independent copy with the same comparator and tree shape.
func (m *TreeMultiset[T]) Clone() *TreeMultiset[T] {
	if m == nil {
		return nil
	}
	return &TreeMultiset[T]{
		root:     cloneTree(m.root),
		distinct: m.distinct,
		size:     m.size,
		cmp:      m.cmp,
	}
}

func cloneTree[T any](node *treeNode[T]) *treeNode[T] {
	if node == nil {
		return nil
	}
	clone := *node
	clone.left = cloneTree(node.left)
	clone.right = cloneTree(node.right)
	return &clone
}

// Equal reports count-aware content equality.
func (m *TreeMultiset[T]) Equal(other *TreeMultiset[T]) bool {
	if m.Len() != other.Len() || m.DistinctLen() != other.DistinctLen() {
		return false
	}
	return m.ContainsOccurrences(other)
}

// ContainsOccurrences reports whether every count in other is present in m.
func (m *TreeMultiset[T]) ContainsOccurrences(other *TreeMultiset[T]) bool {
	if other == nil {
		return true
	}
	for entry := range other.Entries() {
		if m.Count(entry.Value) < entry.Count {
			return false
		}
	}
	return true
}

// AddSlice atomically adds one occurrence of each value.
func (m *TreeMultiset[T]) AddSlice(values []T) {
	if len(values) == 0 {
		return
	}
	m.writeContract("AddSlice")
	next := m.Clone()
	for _, value := range values {
		next.Add(value, 1)
	}
	*m = *next
}

// AddIter drains seq before adding one occurrence of each value.
func (m *TreeMultiset[T]) AddIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	var values []T
	for value := range seq {
		values = append(values, value)
	}
	m.AddSlice(values)
}

// RemoveSlice removes up to one occurrence of each value.
func (m *TreeMultiset[T]) RemoveSlice(values []T) {
	for _, value := range values {
		m.Remove(value, 1)
	}
}

// RemoveIter drains seq before removing one occurrence of each value.
func (m *TreeMultiset[T]) RemoveIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	var values []T
	for value := range seq {
		values = append(values, value)
	}
	m.RemoveSlice(values)
}

func (m *TreeMultiset[T]) rangeEntries(minimum *T, includeMinimum bool, maximum *T, includeMaximum bool) iter.Seq[Entry[T]] {
	return func(yield func(Entry[T]) bool) {
		if m == nil || m.cmp == nil {
			return
		}
		var walk func(*treeNode[T]) bool
		walk = func(node *treeNode[T]) bool {
			if node == nil {
				return true
			}
			aboveMinimum := minimum == nil || m.cmp(node.value, *minimum) > 0 || (includeMinimum && m.cmp(node.value, *minimum) == 0)
			belowMaximum := maximum == nil || m.cmp(node.value, *maximum) < 0 || (includeMaximum && m.cmp(node.value, *maximum) == 0)
			if aboveMinimum && !walk(node.left) {
				return false
			}
			if aboveMinimum && belowMaximum && !yield(Entry[T]{Value: node.value, Count: node.count}) {
				return false
			}
			if belowMaximum {
				return walk(node.right)
			}
			return true
		}
		walk(m.root)
	}
}

func buildBalanced[T any](entries []Entry[T]) *treeNode[T] {
	if len(entries) == 0 {
		return nil
	}
	middle := len(entries) / 2
	node := newTreeNode(entries[middle].Value, entries[middle].Count)
	node.left = buildBalanced(entries[:middle])
	node.right = buildBalanced(entries[middle+1:])
	updateHeight(node)
	return node
}

func treeFromEntries[T any](compare func(T, T) int, entries []Entry[T]) *TreeMultiset[T] {
	result := &TreeMultiset[T]{cmp: compare, distinct: len(entries)}
	result.root = buildBalanced(entries)
	for _, entry := range entries {
		result.size = checkedAddition(result.size, entry.Count)
	}
	return result
}

// SubRange returns an independent snapshot with lo and hi bounds. Reversed
// bounds panic; equal endpoints are included only when both bounds are closed.
func (m *TreeMultiset[T]) SubRange(lo T, includeLo bool, hi T, includeHi bool) *TreeMultiset[T] {
	if m == nil || m.cmp == nil {
		return &TreeMultiset[T]{}
	}
	if m.cmp(lo, hi) > 0 {
		panic("multiset: SubRange lower bound exceeds upper bound")
	}
	entries := make([]Entry[T], 0)
	for entry := range m.rangeEntries(&lo, includeLo, &hi, includeHi) {
		entries = append(entries, entry)
	}
	return treeFromEntries(m.cmp, entries)
}

// HeadRange returns an independent snapshot of values below hi.
func (m *TreeMultiset[T]) HeadRange(hi T, includeHi bool) *TreeMultiset[T] {
	if m == nil || m.cmp == nil {
		return &TreeMultiset[T]{}
	}
	entries := make([]Entry[T], 0)
	for entry := range m.rangeEntries(nil, false, &hi, includeHi) {
		entries = append(entries, entry)
	}
	return treeFromEntries(m.cmp, entries)
}

// TailRange returns an independent snapshot of values above lo.
func (m *TreeMultiset[T]) TailRange(lo T, includeLo bool) *TreeMultiset[T] {
	if m == nil || m.cmp == nil {
		return &TreeMultiset[T]{}
	}
	entries := make([]Entry[T], 0)
	for entry := range m.rangeEntries(&lo, includeLo, nil, false) {
		entries = append(entries, entry)
	}
	return treeFromEntries(m.cmp, entries)
}

func (m *TreeMultiset[T]) adopt(yy, zz *TreeMultiset[T]) func(T, T) int {
	if m != nil && m.cmp != nil {
		return m.cmp
	}
	if yy != nil && yy.cmp != nil {
		return yy.cmp
	}
	if zz != nil {
		return zz.cmp
	}
	return nil
}

// Union sets m to the count-wise maximum of yy and zz.
func (m *TreeMultiset[T]) Union(yy, zz *TreeMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	compare := m.adopt(yy, zz)
	next := &TreeMultiset[T]{cmp: compare}
	for _, entry := range a {
		next.SetCount(entry.Value, entry.Count)
	}
	for entry := range zz.Entries() {
		if entry.Count > next.Count(entry.Value) {
			next.SetCount(entry.Value, entry.Count)
		}
	}
	*m = *next
}

// Sum sets m to the count-wise sum of yy and zz.
func (m *TreeMultiset[T]) Sum(yy, zz *TreeMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	compare := m.adopt(yy, zz)
	next := &TreeMultiset[T]{cmp: compare}
	for _, entry := range a {
		next.SetCount(entry.Value, entry.Count)
	}
	for entry := range zz.Entries() {
		next.Add(entry.Value, entry.Count)
	}
	*m = *next
}

// Intersection sets m to the count-wise minimum of yy and zz.
func (m *TreeMultiset[T]) Intersection(yy, zz *TreeMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	compare := m.adopt(yy, zz)
	next := &TreeMultiset[T]{cmp: compare}
	for _, entry := range a {
		count := min(entry.Count, zz.Count(entry.Value))
		if count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}

// Minus sets m to yy's counts minus zz's counts, saturating at zero.
func (m *TreeMultiset[T]) Minus(yy, zz *TreeMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	compare := m.adopt(yy, zz)
	next := &TreeMultiset[T]{cmp: compare}
	for _, entry := range a {
		count := entry.Count - zz.Count(entry.Value)
		if count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}

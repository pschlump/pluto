/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package multimap implements list, set, and sorted-set multimaps. Keys map to
// collections of values; a key is never stored with an empty collection.
// Get, range materialization, and AsMap return snapshots, not live views.
package multimap

import (
	"fmt"
	"iter"
	"slices"

	"github.com/pschlump/pluto/multiset"
)

type collectionOps[V any, C any] struct {
	unique        bool
	newCollection func() C
	add           func(C, V) (C, bool)
	remove        func(C, V) (C, bool)
	contains      func(C, V) bool
	length        func(C) int
	values        func(C) []V
	replace       func([]V) C
	// addAll, when non-nil, adds a batch to a collection and returns the
	// number of values actually added. It must not mutate the collection
	// unless it returns, so a panicking caller callback leaves C unchanged.
	// When nil, PutAll adds value by value with add.
	addAll func(C, []V) (C, int)
}

const maxAssociations = int(^uint(0) >> 1)

// core is the shared list/set/sorted-set multimap. The key index is the
// package's own open-addressing table, whose payload is the collection
// itself: every operation hashes the key once, probes once, and updates the
// collection in place through the slot (no Search-then-Insert re-probe).
type core[K any, V any, C any] struct {
	tab     *table[K, C]
	eq      func(K, K) bool
	hash    func(K) uint64
	ops     collectionOps[V, C]
	valueEq func(V, V) bool
	size    int
}

func newCore[K any, V any, C any](eq func(K, K) bool, hash func(K) uint64, ops collectionOps[V, C], valueEq func(V, V) bool) core[K, V, C] {
	if eq == nil || hash == nil {
		panic("multimap: key equality and hash functions must not be nil")
	}
	return core[K, V, C]{
		tab:     newTable[K, C](eq, hash),
		eq:      eq,
		hash:    hash,
		ops:     ops,
		valueEq: valueEq,
	}
}

func (c *core[K, V, C]) writeContract(caller string) {
	if c == nil || c.tab == nil {
		panic(fmt.Sprintf("multimap: %s on a nil or zero-value multimap; use a constructor", caller))
	}
}

// collection returns a pointer to key's stored collection, or nil.
func (c *core[K, V, C]) collection(key K) *C {
	if c == nil {
		return nil
	}
	return c.tab.lookup(key)
}

func (c *core[K, V, C]) Get(key K) ([]V, bool) {
	collection := c.collection(key)
	if collection == nil {
		return nil, false
	}
	return c.ops.values(*collection), true
}

// ValueCount returns len of key's collection without copying it.
func (c *core[K, V, C]) ValueCount(key K) int {
	collection := c.collection(key)
	if collection == nil {
		return 0
	}
	return c.ops.length(*collection)
}

func (c *core[K, V, C]) Put(key K, value V) bool {
	c.writeContract("Put")
	index, h, found := c.tab.find(key)
	if !found {
		if c.size == maxAssociations {
			panic("multimap: association count overflow")
		}
		collection, _ := c.ops.add(c.ops.newCollection(), value)
		c.tab.insertAt(index, h, key, collection)
		c.size++
		return true
	}
	collection := &c.tab.slots[index].val
	if c.size == maxAssociations && (!c.ops.unique || !c.ops.contains(*collection, value)) {
		panic("multimap: association count overflow")
	}
	next, added := c.ops.add(*collection, value)
	if !added {
		return false
	}
	*collection = next
	c.tab.slots[index].key = key // an adding write stores the latest key representative
	c.size++
	return true
}

// PutAll adds values to key's collection and returns the number added. When
// the batch cannot overflow the association count, it is added in place:
// a new key's collection is built before the key is installed, and an
// existing key's collection is extended without a copy. Otherwise the exact
// copy-and-rebuild path checks overflow before installing anything.
func (c *core[K, V, C]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	c.writeContract("PutAll")
	if c.size > maxAssociations-len(values) {
		return c.putAllRebuild(key, values)
	}
	index, h, found := c.tab.find(key)
	if !found {
		collection := c.ops.newCollection()
		added := 0
		if c.ops.addAll != nil {
			collection, added = c.ops.addAll(collection, values)
		} else {
			for _, value := range values {
				var ok bool
				if collection, ok = c.ops.add(collection, value); ok {
					added++
				}
			}
		}
		if added == 0 {
			return 0
		}
		c.tab.insertAt(index, h, key, collection)
		c.size += added
		return added
	}
	s := &c.tab.slots[index]
	added := 0
	if c.ops.addAll != nil {
		var next C
		next, added = c.ops.addAll(s.val, values)
		s.val = next
		c.size += added
	} else {
		// A batch that adds nothing leaves the collection untouched (the
		// rebuild semantics: no representative is replaced), so first look
		// for one absent value; usually the first value already is.
		if c.ops.unique && !slices.ContainsFunc(values, func(value V) bool { return !c.ops.contains(s.val, value) }) {
			return 0
		}
		// Value-by-value: the size follows every successful add, so a
		// panicking caller callback leaves a consistent (partial) batch.
		for _, value := range values {
			next, ok := c.ops.add(s.val, value)
			s.val = next
			if ok {
				added++
				c.size++
			}
		}
	}
	if added > 0 {
		s.key = key
	}
	return added
}

// putAllRebuild is the near-overflow PutAll: it builds the combined
// collection first and installs it only after the exact overflow check.
func (c *core[K, V, C]) putAllRebuild(key K, values []V) int {
	index, h, found := c.tab.find(key)
	var old C
	if found {
		old = c.tab.slots[index].val
	}
	combined := c.ops.values(old)
	combined = append(combined, values...)
	next := c.ops.replace(combined)
	added := c.ops.length(next) - c.ops.length(old)
	if added == 0 {
		return 0
	}
	checkedSize(c.size, added)
	if found {
		c.tab.slots[index].key = key
		c.tab.slots[index].val = next
	} else {
		c.tab.insertAt(index, h, key, next)
	}
	c.size += added
	return added
}

func (c *core[K, V, C]) Remove(key K, value V) bool {
	if c == nil || c.tab == nil || c.tab.length == 0 {
		return false
	}
	index, _, found := c.tab.find(key)
	if !found {
		return false
	}
	collection := &c.tab.slots[index].val
	next, removed := c.ops.remove(*collection, value)
	if !removed {
		return false
	}
	if c.ops.length(next) == 0 {
		c.tab.deleteAt(index)
	} else {
		*collection = next
	}
	c.size--
	return true
}

func (c *core[K, V, C]) RemoveAll(key K) []V {
	if c == nil || c.tab == nil || c.tab.length == 0 {
		return nil
	}
	index, _, found := c.tab.find(key)
	if !found {
		return nil
	}
	values := c.ops.values(c.tab.slots[index].val)
	c.tab.deleteAt(index)
	c.size -= len(values)
	return values
}

func (c *core[K, V, C]) ReplaceValues(key K, values []V) []V {
	c.writeContract("ReplaceValues")
	index, h, found := c.tab.find(key)
	var oldCollection C
	if found {
		oldCollection = c.tab.slots[index].val
	}
	old := c.ops.values(oldCollection)
	next := c.ops.replace(values)
	delta := c.ops.length(next) - len(old)
	if delta > 0 {
		checkedSize(c.size, delta)
	}
	switch {
	case c.ops.length(next) == 0:
		if found {
			c.tab.deleteAt(index)
		}
	case found:
		c.tab.slots[index].key = key
		c.tab.slots[index].val = next
	default:
		c.tab.insertAt(index, h, key, next)
	}
	c.size += delta
	return old
}

func (c *core[K, V, C]) ContainsKey(key K) bool {
	return c.collection(key) != nil
}

func (c *core[K, V, C]) ContainsEntry(key K, value V) bool {
	collection := c.collection(key)
	return collection != nil && c.ops.contains(*collection, value)
}

func (c *core[K, V, C]) ContainsValue(value V) bool {
	if c == nil || c.tab == nil {
		return false
	}
	for _, collection := range c.tab.all() {
		if c.ops.contains(collection, value) {
			return true
		}
	}
	return false
}

func (c *core[K, V, C]) Len() int {
	if c == nil {
		return 0
	}
	return c.size
}

func (c *core[K, V, C]) KeyLen() int {
	if c == nil || c.tab == nil {
		return 0
	}
	return c.tab.length
}

func (c *core[K, V, C]) IsEmpty() bool {
	return c.Len() == 0
}

func (c *core[K, V, C]) Clear() {
	if c == nil || c.tab == nil {
		return
	}
	c.tab.clear()
	c.size = 0
}

func (c *core[K, V, C]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if c == nil || c.tab == nil {
			return
		}
		for key, collection := range c.tab.all() {
			for _, value := range c.ops.values(collection) {
				if !yield(key, value) {
					return
				}
			}
		}
	}
}

func (c *core[K, V, C]) EntriesSlice() []Association[K, V] {
	if c == nil {
		return nil
	}
	entries := make([]Association[K, V], 0, c.Len())
	for key, value := range c.Entries() {
		entries = append(entries, Association[K, V]{Key: key, Value: value})
	}
	return entries
}

func (c *core[K, V, C]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for key := range c.Entries() {
			if !yield(key) {
				return
			}
		}
	}
}

func (c *core[K, V, C]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, value := range c.Entries() {
			if !yield(value) {
				return
			}
		}
	}
}

func (c *core[K, V, C]) KeySet() iter.Seq[K] {
	return func(yield func(K) bool) {
		if c == nil || c.tab == nil {
			return
		}
		for key := range c.tab.all() {
			if !yield(key) {
				return
			}
		}
	}
}

func (c *core[K, V, C]) KeyMultiset() *multiset.HashMultiset[K] {
	if c == nil || c.tab == nil {
		return &multiset.HashMultiset[K]{}
	}
	result := multiset.NewHashMultisetFunc(c.eq, c.hash)
	for key, collection := range c.tab.all() {
		result.Add(key, c.ops.length(collection))
	}
	return result
}

func checkedSize(size, delta int) {
	if delta > 0 && size > maxAssociations-delta {
		panic("multimap: association count overflow")
	}
}

// Association is one key/value pair in a multimap snapshot.
type Association[K any, V any] struct {
	Key   K
	Value V
}

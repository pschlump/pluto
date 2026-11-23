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

	"github.com/pschlump/pluto/hash_grow"
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
}

type keyEntry[K any, V any, C any] struct {
	Key        K
	Collection C
}

type core[K any, V any, C any] struct {
	tab     *hash_grow.HashTab[keyEntry[K, V, C]]
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
		tab: hash_grow.NewHashTabFunc(
			func(a, b keyEntry[K, V, C]) bool { return eq(a.Key, b.Key) },
			func(a keyEntry[K, V, C]) uint64 { return hash(a.Key) },
			16,
			0,
		),
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

func (c *core[K, V, C]) entry(key K) (keyEntry[K, V, C], bool) {
	if c == nil || c.tab == nil {
		var zero keyEntry[K, V, C]
		return zero, false
	}
	return c.tab.Search(keyEntry[K, V, C]{Key: key})
}

func (c *core[K, V, C]) Get(key K) ([]V, bool) {
	entry, found := c.entry(key)
	if !found {
		return nil, false
	}
	return c.ops.values(entry.Collection), true
}

func (c *core[K, V, C]) Put(key K, value V) bool {
	c.writeContract("Put")
	entry, found := c.entry(key)
	collection := entry.Collection
	if c.size == int(^uint(0)>>1) && (!found || !c.ops.unique || !c.ops.contains(collection, value)) {
		panic("multimap: association count overflow")
	}
	if !found {
		collection = c.ops.newCollection()
	}
	collection, added := c.ops.add(collection, value)
	if !added {
		return false
	}
	entry = keyEntry[K, V, C]{Key: key, Collection: collection}
	c.tab.Insert(entry)
	c.size++
	return true
}

func (c *core[K, V, C]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	c.writeContract("PutAll")
	entry, _ := c.entry(key)
	combined := c.ops.values(entry.Collection)
	combined = append(combined, values...)
	next := c.ops.replace(combined)
	added := c.ops.length(next) - c.ops.length(entry.Collection)
	if added == 0 {
		return 0
	}
	checkedSize(c.size, added)
	entry = keyEntry[K, V, C]{Key: key, Collection: next}
	c.tab.Insert(entry)
	c.size += added
	return added
}

func (c *core[K, V, C]) Remove(key K, value V) bool {
	entry, found := c.entry(key)
	if !found {
		return false
	}
	collection, removed := c.ops.remove(entry.Collection, value)
	if !removed {
		return false
	}
	if c.ops.length(collection) == 0 {
		c.tab.Delete(keyEntry[K, V, C]{Key: key})
	} else {
		entry.Collection = collection
		c.tab.Insert(entry)
	}
	c.size--
	return true
}

func (c *core[K, V, C]) RemoveAll(key K) []V {
	entry, found := c.entry(key)
	if !found {
		return nil
	}
	values := c.ops.values(entry.Collection)
	c.tab.Delete(keyEntry[K, V, C]{Key: key})
	c.size -= len(values)
	return values
}

func (c *core[K, V, C]) ReplaceValues(key K, values []V) []V {
	c.writeContract("ReplaceValues")
	entry, found := c.entry(key)
	old := c.ops.values(entry.Collection)
	next := c.ops.replace(values)
	delta := c.ops.length(next) - len(old)
	if delta > 0 {
		checkedSize(c.size, delta)
	}
	if c.ops.length(next) == 0 {
		if found {
			c.tab.Delete(keyEntry[K, V, C]{Key: key})
		}
	} else {
		c.tab.Insert(keyEntry[K, V, C]{Key: key, Collection: next})
	}
	c.size += delta
	return old
}

func (c *core[K, V, C]) ContainsKey(key K) bool {
	_, found := c.entry(key)
	return found
}

func (c *core[K, V, C]) ContainsEntry(key K, value V) bool {
	entry, found := c.entry(key)
	return found && c.ops.contains(entry.Collection, value)
}

func (c *core[K, V, C]) ContainsValue(value V) bool {
	if c == nil || c.tab == nil {
		return false
	}
	for entry := range c.tab.Values() {
		if c.ops.contains(entry.Collection, value) {
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
	return c.tab.Len()
}

func (c *core[K, V, C]) IsEmpty() bool {
	return c.Len() == 0
}

func (c *core[K, V, C]) Clear() {
	if c == nil || c.tab == nil {
		return
	}
	c.tab.Truncate()
	c.size = 0
}

func (c *core[K, V, C]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if c == nil || c.tab == nil {
			return
		}
		for entry := range c.tab.Values() {
			for _, value := range c.ops.values(entry.Collection) {
				if !yield(entry.Key, value) {
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
		for entry := range c.tab.Values() {
			if !yield(entry.Key) {
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
	for entry := range c.tab.Values() {
		result.Add(entry.Key, c.ops.length(entry.Collection))
	}
	return result
}

func checkedSize(size, delta int) {
	if delta > 0 && size > int(^uint(0)>>1)-delta {
		panic("multimap: association count overflow")
	}
}

// Association is one key/value pair in a multimap snapshot.
type Association[K any, V any] struct {
	Key   K
	Value V
}

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/linked_hash_map"
)

type linkedAssociation[K any, V any] struct {
	key              K
	associationValue V
	prev, next       *linkedAssociation[K, V]
	keyPrev, keyNext *linkedAssociation[K, V]
}

type linkedKeyEntry[K any, V any] struct {
	head, tail *linkedAssociation[K, V]
	values     *linked_hash_map.Map[V, *linkedAssociation[K, V]]
	count      int
}

type linkedCore[K any, V any] struct {
	keys      *linked_hash_map.Map[K, *linkedKeyEntry[K, V]]
	eq        func(K, K) bool
	hash      func(K) uint64
	valueEq   func(V, V) bool
	valueHash func(V) uint64
	unique    bool
	head      *linkedAssociation[K, V]
	tail      *linkedAssociation[K, V]
	size      int
}

func newLinkedCore[K any, V any](eq func(K, K) bool, hash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64, unique bool) linkedCore[K, V] {
	if eq == nil || hash == nil || valueEq == nil || (unique && valueHash == nil) {
		panic("multimap: linked multimap requires non-nil equality/hash functions")
	}
	return linkedCore[K, V]{
		keys: linked_hash_map.NewFunc[K, *linkedKeyEntry[K, V]](eq, hash),
		eq:   eq, hash: hash, valueEq: valueEq, valueHash: valueHash, unique: unique,
	}
}

func (c *linkedCore[K, V]) writeContract(caller string) {
	if c == nil || c.keys == nil {
		panic(fmt.Sprintf("multimap: %s on a nil or zero-value linked multimap; use a constructor", caller))
	}
}

func (c *linkedCore[K, V]) entry(key K) (*linkedKeyEntry[K, V], bool) {
	if c == nil || c.keys == nil {
		return nil, false
	}
	return c.keys.Get(key)
}

func (c *linkedCore[K, V]) Get(key K) ([]V, bool) {
	entry, found := c.entry(key)
	if !found {
		return nil, false
	}
	values := make([]V, 0, entry.count)
	for node := entry.head; node != nil; node = node.keyNext {
		values = append(values, node.associationValue)
	}
	return values, true
}

func (c *linkedCore[K, V]) Put(key K, value V) bool {
	c.writeContract("Put")
	entry, found := c.entry(key)
	if found && c.unique && entry.values.ContainsKey(value) {
		return false
	}
	if c.size == int(^uint(0)>>1) {
		panic("multimap: association count overflow")
	}
	if !found {
		entry = &linkedKeyEntry[K, V]{}
		if c.unique {
			entry.values = linked_hash_map.NewFunc[V, *linkedAssociation[K, V]](c.valueEq, c.valueHash)
		}
		c.keys.Put(key, entry)
	}
	node := &linkedAssociation[K, V]{key: key, associationValue: value, prev: c.tail, keyPrev: entry.tail}
	if c.tail == nil {
		c.head = node
	} else {
		c.tail.next = node
	}
	c.tail = node
	if entry.tail == nil {
		entry.head = node
	} else {
		entry.tail.keyNext = node
	}
	entry.tail = node
	entry.count++
	if c.unique {
		entry.values.Put(value, node)
	}
	c.size++
	return true
}

func (c *linkedCore[K, V]) addedCount(entry *linkedKeyEntry[K, V], values []V) int {
	if !c.unique {
		return len(values)
	}
	seen := linked_hash_map.NewFunc[V, struct{}](c.valueEq, c.valueHash)
	added := 0
	for _, value := range values {
		if entry != nil && entry.values.ContainsKey(value) || seen.ContainsKey(value) {
			continue
		}
		seen.Put(value, struct{}{})
		added++
	}
	return added
}

func (c *linkedCore[K, V]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	c.writeContract("PutAll")
	entry, _ := c.entry(key)
	added := c.addedCount(entry, values)
	checkedSize(c.size, added)
	for _, value := range values {
		c.Put(key, value)
	}
	return added
}

func (c *linkedCore[K, V]) unlink(entry *linkedKeyEntry[K, V], node *linkedAssociation[K, V]) {
	if node.prev == nil {
		c.head = node.next
	} else {
		node.prev.next = node.next
	}
	if node.next == nil {
		c.tail = node.prev
	} else {
		node.next.prev = node.prev
	}
	if node.keyPrev == nil {
		entry.head = node.keyNext
	} else {
		node.keyPrev.keyNext = node.keyNext
	}
	if node.keyNext == nil {
		entry.tail = node.keyPrev
	} else {
		node.keyNext.keyPrev = node.keyPrev
	}
	entry.count--
	c.size--
	node.prev, node.next, node.keyPrev, node.keyNext = nil, nil, nil, nil
}

func (c *linkedCore[K, V]) Remove(key K, value V) bool {
	entry, found := c.entry(key)
	if !found {
		return false
	}
	var node *linkedAssociation[K, V]
	if c.unique {
		node, _ = entry.values.Get(value)
	} else {
		for candidate := entry.head; candidate != nil; candidate = candidate.keyNext {
			if c.valueEq(candidate.associationValue, value) {
				node = candidate
				break
			}
		}
	}
	if node == nil {
		return false
	}
	c.unlink(entry, node)
	if c.unique {
		entry.values.Delete(value)
	}
	if entry.count == 0 {
		c.keys.Delete(key)
	}
	return true
}

func (c *linkedCore[K, V]) RemoveAll(key K) []V {
	entry, found := c.entry(key)
	if !found {
		return nil
	}
	values := make([]V, 0, entry.count)
	for node := entry.head; node != nil; {
		next := node.keyNext
		values = append(values, node.associationValue)
		c.unlink(entry, node)
		node = next
	}
	c.keys.Delete(key)
	return values
}

func (c *linkedCore[K, V]) ReplaceValues(key K, values []V) []V {
	c.writeContract("ReplaceValues")
	entry, _ := c.entry(key)
	oldCount := 0
	if entry != nil {
		oldCount = entry.count
	}
	checkedSize(c.size-oldCount, c.addedCount(nil, values))
	old := c.RemoveAll(key)
	c.PutAll(key, values)
	return old
}

func (c *linkedCore[K, V]) ContainsKey(key K) bool {
	_, found := c.entry(key)
	return found
}

func (c *linkedCore[K, V]) ContainsEntry(key K, value V) bool {
	entry, found := c.entry(key)
	if !found {
		return false
	}
	if c.unique {
		return entry.values.ContainsKey(value)
	}
	for node := entry.head; node != nil; node = node.keyNext {
		if c.valueEq(node.associationValue, value) {
			return true
		}
	}
	return false
}

func (c *linkedCore[K, V]) ContainsValue(value V) bool {
	if c == nil || c.keys == nil {
		return false
	}
	for node := c.head; node != nil; node = node.next {
		if c.valueEq(node.associationValue, value) {
			return true
		}
	}
	return false
}

func (c *linkedCore[K, V]) Len() int {
	if c == nil {
		return 0
	}
	return c.size
}

func (c *linkedCore[K, V]) KeyLen() int {
	if c == nil || c.keys == nil {
		return 0
	}
	return c.keys.Len()
}

func (c *linkedCore[K, V]) IsEmpty() bool { return c.Len() == 0 }

func (c *linkedCore[K, V]) Clear() {
	if c == nil || c.keys == nil {
		return
	}
	c.keys.Clear()
	c.head, c.tail, c.size = nil, nil, 0
}

func (c *linkedCore[K, V]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if c == nil || c.keys == nil {
			return
		}
		for node := c.head; node != nil; node = node.next {
			if !yield(node.key, node.associationValue) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) EntriesSlice() []Association[K, V] {
	if c == nil {
		return nil
	}
	entries := make([]Association[K, V], 0, c.size)
	for key, value := range c.Entries() {
		entries = append(entries, Association[K, V]{Key: key, Value: value})
	}
	return entries
}

func (c *linkedCore[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for key := range c.Entries() {
			if !yield(key) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, value := range c.Entries() {
			if !yield(value) {
				return
			}
		}
	}
}

func (c *linkedCore[K, V]) KeySet() iter.Seq[K] {
	if c == nil || c.keys == nil {
		return func(func(K) bool) {}
	}
	return c.keys.Keys()
}

func (c *linkedCore[K, V]) AsMap() MapSnapshot[K, V] {
	if c == nil || c.keys == nil {
		return MapSnapshot[K, V]{}
	}
	result := MapSnapshot[K, V]{eq: c.eq, size: c.size}
	for key := range c.KeySet() {
		values, _ := c.Get(key)
		result.entries = append(result.entries, MapSnapshotEntry[K, V]{Key: key, Values: values})
	}
	return result
}
